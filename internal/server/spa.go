package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"

	pluginrt "github.com/ContinuumApp/continuum-plugin-support/internal/runtime"
)

//go:embed public/dist/*
var publicSPA embed.FS

type supportBootstrap struct {
	Mode    string                 `json:"mode"`
	Theme   string                 `json:"theme"`
	Modules pluginrt.ModuleToggles `json:"modules"`
	UserID  string                 `json:"userId"`
	IsAdmin bool                   `json:"isAdmin"`
}

func hPublicAsset() http.HandlerFunc {
	dist, err := fs.Sub(publicSPA, "public/dist")
	if err != nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, nil)
		}
	}
	handler := http.StripPrefix("/", http.FileServer(http.FS(dist)))
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

func writeSPA(w http.ResponseWriter, r *http.Request, bs supportBootstrap, status int) {
	if bs.Theme == "" {
		bs.Theme = "midnight-cinema"
	}
	index, err := publicSPA.ReadFile("public/dist/index.html")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "spa_unavailable", "support app has not been built")
		return
	}
	rawBootstrap, err := json.Marshal(bs)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "bootstrap_failed", "failed to render bootstrap")
		return
	}
	index = bytes.Replace(index, []byte("%SUPPORT_BOOTSTRAP%"), rawBootstrap, 1)
	index = bytes.Replace(index, []byte(`<html lang="en">`),
		[]byte(`<html lang="en" data-theme="`+html.EscapeString(bs.Theme)+`">`), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(index)
}

func adminTheme(r *http.Request) string {
	theme := r.URL.Query().Get("theme")
	if theme == "" {
		theme = r.Header.Get("X-Continuum-Theme")
	}
	if theme == "" {
		theme = r.Header.Get("X-Continuum-User-Theme")
	}
	if theme == "" {
		theme = "default"
	}
	return html.EscapeString(theme)
}
