package server

import (
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

const kbImageMaxBytes = 5 << 20 // 5 MB

var kbAllowedImageMIMEs = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// hKBAdminUploadImage accepts a multipart upload, validates size +
// MIME, persists, returns the served URL + id.
func hKBAdminUploadImage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Pre-flight size guard — anything claiming > 5 MB → 413
		// before we start reading. The shell's body cap is 12 MB,
		// so any Content-Length over 5 MB lands here as a 413, not
		// the shell's 413.
		if r.ContentLength > kbImageMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large",
				"image must be 5 MB or smaller")
			return
		}
		if err := r.ParseMultipartForm(kbImageMaxBytes); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_multipart", "could not parse multipart body")
			return
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "missing_image", "image field is required")
			return
		}
		defer file.Close()

		// Hard re-check post-read in case Content-Length lied.
		body, err := io.ReadAll(io.LimitReader(file, kbImageMaxBytes+1))
		if err != nil {
			writeInternal(w, r, d, "image_read_failed", err)
			return
		}
		if int64(len(body)) > kbImageMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large",
				"image must be 5 MB or smaller")
			return
		}

		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = http.DetectContentType(body)
		}
		if !kbAllowedImageMIMEs[mime] {
			writeErr(w, http.StatusUnsupportedMediaType, "bad_mime",
				"image must be PNG, JPEG, GIF, or WEBP")
			return
		}

		sum := sha256.Sum256(body)

		var articleID *int64
		if v := r.URL.Query().Get("articleId"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				articleID = &n
			}
		}

		id, err := kbAdminStore(d).KBInsertImage(r.Context(), store.KBImage{
			ArticleID: articleID,
			Filename:  header.Filename,
			MIME:      mime,
			Bytes:     int64(len(body)),
			Content:   body,
			SHA256:    sum[:],
		})
		if err != nil {
			writeInternal(w, r, d, "image_insert_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":  id,
			"url": "/api/kb/images/" + strconv.FormatInt(id, 10),
		})
	}
}

// hKBImageServe streams the image bytes. Available to both customers
// and admins (requireUser middleware — the admin-uploaded URL has to
// resolve for any logged-in customer reading the article).
func hKBImageServe(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}
		img, err := kbAdminStore(d).KBGetImage(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeInternal(w, r, d, "image_get_failed", err)
			return
		}
		w.Header().Set("Content-Type", img.MIME)
		w.Header().Set("Content-Length", strconv.FormatInt(img.Bytes, 10))
		w.Header().Set("Cache-Control", "private, max-age=3600")
		_, _ = w.Write(img.Content)
	}
}

