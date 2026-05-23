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
		if r.Header.Get("X-Silo-User-Id") == "" {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "log in to continue")
			return
		}
		next(w, r)
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Silo-User-Id") == "" {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "admin login required")
			return
		}
		if r.Header.Get("X-Silo-User-Role") != "admin" {
			writeErr(w, http.StatusForbidden, "forbidden", "admin access required")
			return
		}
		next(w, r)
	}
}

// limitBody caps inbound request bodies. The wrapped handler sees a
// MaxBytesReader; reading past max returns an error which http
// surfaces as a 413 if the handler writes nothing else first.
func limitBody(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}
