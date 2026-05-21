package server

import "net/http"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Continuum-User-Id") == "" {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "log in to continue")
			return
		}
		next(w, r)
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Continuum-User-Id") == "" {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "admin login required")
			return
		}
		if r.Header.Get("X-Continuum-User-Role") != "admin" {
			writeErr(w, http.StatusForbidden, "forbidden", "admin access required")
			return
		}
		next(w, r)
	}
}
