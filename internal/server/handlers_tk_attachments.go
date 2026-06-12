package server

import (
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

const tkAttachmentMaxBytes = 10 << 20 // 10 MB

// tkAllowedAttachmentMIMEs is the upload allowlist. Anything outside
// this set is rejected at upload time, and even an allowed type is
// only ever served back with Content-Disposition: attachment (never
// inline) to prevent stored XSS via attacker-controlled content.
var tkAllowedAttachmentMIMEs = map[string]bool{
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/webp":      true,
	"application/pdf": true,
	"text/plain":      true,
}

func hTKUploadAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryID, err := strconv.ParseInt(chi.URLParam(r, "entry_id"), 10, 64)
		if err != nil || entryID <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid entry id"); return
		}
		st := tkCustomerStore(d)
		entry, err := st.TKGetEntry(r.Context(), entryID)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "entry not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_entry_get_failed", err); return
		}
		if r.Header.Get("X-Silo-User-Role") != "admin" {
			userID := r.Header.Get("X-Silo-User-Id")
			ticket, err := st.TKGetTicketByID(r.Context(), entry.TicketID)
			if err != nil {
				writeInternal(w, r, d, "tk_ticket_get_failed", err); return
			}
			if ticket.CustomerID != userID {
				writeErr(w, http.StatusForbidden, "tk_forbidden", "not your ticket"); return
			}
			// A customer may only attach to entries they authored — not to
			// admin replies or internal notes on their own ticket.
			if entry.AuthorRole != "customer" || entry.AuthorID != userID {
				writeErr(w, http.StatusForbidden, "tk_forbidden", "you can only attach files to your own messages"); return
			}
		}
		if r.ContentLength > tkAttachmentMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "tk_too_large", "attachment must be 10 MB or smaller"); return
		}
		if err := r.ParseMultipartForm(tkAttachmentMaxBytes); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_multipart", "could not parse multipart body"); return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "missing_file", "file field required"); return
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, tkAttachmentMaxBytes+1))
		if err != nil {
			writeInternal(w, r, d, "tk_attachment_read_failed", err); return
		}
		if int64(len(body)) > tkAttachmentMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "tk_too_large", "attachment must be 10 MB or smaller"); return
		}
		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = http.DetectContentType(body)
		}
		// Normalize away any parameters (e.g. "text/plain; charset=utf-8")
		// before checking the allowlist.
		if i := strings.IndexByte(mime, ';'); i >= 0 {
			mime = strings.TrimSpace(mime[:i])
		}
		mime = strings.ToLower(mime)
		if !tkAllowedAttachmentMIMEs[mime] {
			writeErr(w, http.StatusUnsupportedMediaType, "tk_bad_mime",
				"attachment must be PNG, JPEG, GIF, WEBP, PDF, or plain text"); return
		}
		sum := sha256.Sum256(body)
		meta, err := st.TKInsertAttachment(r.Context(), store.TKAttachment{
			EntryID: entry.ID, Filename: header.Filename, MIME: mime,
			Bytes: int64(len(body)), Content: body, SHA256: sum[:],
		})
		if err != nil {
			writeInternal(w, r, d, "tk_attachment_insert_failed", err); return
		}
		writeJSON(w, http.StatusOK, meta)
	}
}

func hTKServeAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r); return
		}
		st := tkCustomerStore(d)
		att, err := st.TKGetAttachment(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_attachment_get_failed", err); return
		}
		if r.Header.Get("X-Silo-User-Role") != "admin" {
			entry, err := st.TKGetEntry(r.Context(), att.EntryID)
			if err != nil {
				writeInternal(w, r, d, "tk_entry_get_failed", err); return
			}
			ticket, err := st.TKGetTicketByID(r.Context(), entry.TicketID)
			if err != nil {
				writeInternal(w, r, d, "tk_ticket_get_failed", err); return
			}
			if ticket.CustomerID != r.Header.Get("X-Silo-User-Id") {
				http.NotFound(w, r); return
			}
		}
		// Defense in depth: only serve a stored Content-Type if it is
		// still on the allowlist; otherwise fall back to a generic binary
		// type. Content is ALWAYS served as an attachment (never inline)
		// so the browser downloads it instead of rendering it in our
		// origin — this neutralizes stored XSS via HTML/SVG/script bodies.
		ct := strings.ToLower(strings.TrimSpace(att.MIME))
		if i := strings.IndexByte(ct, ';'); i >= 0 {
			ct = strings.TrimSpace(ct[:i])
		}
		if !tkAllowedAttachmentMIMEs[ct] {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", strconv.FormatInt(att.Bytes, 10))
		w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(att.Filename)+"\"")
		_, _ = w.Write(att.Content)
	}
}

// sanitizeFilename strips characters that could break out of the quoted
// Content-Disposition filename value or inject additional header content
// (CR/LF, double quotes, backslashes, control bytes). Falls back to a
// safe default if nothing usable remains.
func sanitizeFilename(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "attachment"
	}
	return cleaned
}
