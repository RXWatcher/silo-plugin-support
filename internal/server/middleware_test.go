package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireUserBlocksMissingIdentity(t *testing.T) {
	called := false
	h := requireUser(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("inner handler must not run when identity is missing")
	}
}

func TestRequireUserAllowsKnownIdentity(t *testing.T) {
	called := false
	h := requireUser(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Continuum-User-Id", "42")
	rec := httptest.NewRecorder()
	h(rec, req)
	if !called {
		t.Fatal("inner handler must run for authenticated user")
	}
}

func TestRequireAdminBlocksNonAdmin(t *testing.T) {
	called := false
	h := requireAdmin(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Continuum-User-Id", "42")
	req.Header.Set("X-Continuum-User-Role", "user")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if called {
		t.Fatal("inner handler must not run for non-admin")
	}
}

func TestRequireAdminAllowsAdmin(t *testing.T) {
	called := false
	h := requireAdmin(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec := httptest.NewRecorder()
	h(rec, req)
	if !called {
		t.Fatal("inner handler must run for admin")
	}
}

func TestSecurityHeadersApplyOnEveryResponse(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := securityHeaders(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}
