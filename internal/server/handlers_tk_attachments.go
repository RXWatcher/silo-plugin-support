package server

import (
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

const tkAttachmentMaxBytes = 10 << 20 // 10 MB

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
			ticket, err := st.TKGetTicketByID(r.Context(), entry.TicketID)
			if err != nil {
				writeInternal(w, r, d, "tk_ticket_get_failed", err); return
			}
			if ticket.CustomerID != r.Header.Get("X-Silo-User-Id") {
				writeErr(w, http.StatusForbidden, "tk_forbidden", "not your ticket"); return
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
		w.Header().Set("Content-Type", att.MIME)
		w.Header().Set("Content-Length", strconv.FormatInt(att.Bytes, 10))
		w.Header().Set("Content-Disposition", "inline; filename=\""+att.Filename+"\"")
		_, _ = w.Write(att.Content)
	}
}
