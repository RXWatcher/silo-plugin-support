package server

import (
	"encoding/json"
	"net/http"

	"github.com/hashicorp/go-hclog"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func writeInternal(w http.ResponseWriter, r *http.Request, d Deps, code string, err error) {
	if d.Logger != nil {
		d.Logger.Error("internal error", "code", code, "err", err, "path", r.URL.Path)
	} else {
		hclog.L().Error("internal error", "code", code, "err", err, "path", r.URL.Path)
	}
	writeErr(w, http.StatusInternalServerError, code, "internal error")
}
