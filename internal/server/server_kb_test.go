package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ContinuumApp/continuum-plugin-support/internal/migrate"
	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// kbTestDeps spins a real Postgres-backed Store. Skips the calling
// test if PG_DSN is unset.
func kbTestDeps(t *testing.T) (Deps, *store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN unset; skipping KB integration test")
	}
	ctx := context.Background()
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	st := store.New(pool)
	cleanup := func() { pool.Close() }
	return Deps{ConfigStore: st}, st, cleanup
}

func TestKBCustomerListRequiresAuth(t *testing.T) {
	d, _, cleanup := kbTestDeps(t)
	defer cleanup()
	h := New(d)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer/kb/articles", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestKBAdminRoutesRejectNonAdmin(t *testing.T) {
	d, _, cleanup := kbTestDeps(t)
	defer cleanup()
	h := New(d)
	for _, path := range []string{
		"/admin/kb",
		"/api/admin/kb/articles",
		"/api/admin/kb/categories",
		"/api/admin/kb/tags",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Continuum-User-Id", "42")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("path %s status = %d, want 403", path, rec.Code)
		}
	}
}

func TestKBArticleCRUDRoundTrip(t *testing.T) {
	d, st, cleanup := kbTestDeps(t)
	defer cleanup()
	h := New(d)
	ctx := context.Background()

	cat, err := st.KBCreateCategory(ctx, "tests", "Tests", 0)
	if err != nil {
		t.Fatalf("seed category: %v", err)
	}

	body := fmt.Sprintf(`{"title":"Hello","categoryId":%d,"bodyHtml":"<p>hi</p>","status":"draft","tagLabels":["beta"]}`, cat.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/kb/articles", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var created store.KBArticle
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Slug != "hello" || created.Status != "draft" {
		t.Fatalf("unexpected article: %+v", created)
	}

	// Publish.
	req = httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/admin/kb/articles/%d/publish", created.ID), nil)
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d", rec.Code)
	}

	// Customer detail returns it.
	req = httptest.NewRequest(http.MethodGet, "/api/customer/kb/articles/hello", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("customer detail status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// Customer vote.
	req = httptest.NewRequest(http.MethodPost, "/api/customer/kb/articles/hello/vote",
		bytes.NewBufferString(`{"vote":"up"}`))
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("vote status = %d, body=%s", rec.Code, rec.Body.String())
	}
}
