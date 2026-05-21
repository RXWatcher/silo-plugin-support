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

func tkTestDeps(t *testing.T) (Deps, *store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN unset; skipping tickets integration test")
	}
	ctx := context.Background()
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	d := Deps{
		ConfigStore:              st,
		TKAutoCloseEnabled:       true,
		TKResolvedCloseAfterDays: 7,
		TKWaitingCloseAfterDays:  14,
	}
	return d, st, func() { pool.Close() }
}

func TestTKCustomerListRequiresAuth(t *testing.T) {
	d, _, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer/tickets", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTKAdminRoutesRejectNonAdmin(t *testing.T) {
	d, _, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	for _, path := range []string{"/admin/tickets", "/api/admin/tickets", "/api/admin/categories"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Continuum-User-Id", "42")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("path %s status = %d, want 403", path, rec.Code)
		}
	}
}

func TestTKCustomerCreateAndDetailRoundTrip(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	ctx := context.Background()

	cat, _ := st.TKCreateCategory(ctx, store.TKCategory{Slug: "test", Name: "Test", Active: true})

	body := fmt.Sprintf(`{"categoryId":%d,"subject":"hello","body":"world","customerEmail":"a@b"}`, cat.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/customer/tickets", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var created store.TKTicket
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.TrackingNumber == "" {
		t.Fatalf("missing tracking number: %+v", created)
	}

	// Detail (owner)
	req = httptest.NewRequest(http.MethodGet, "/api/customer/tickets/"+created.TrackingNumber, nil)
	req.Header.Set("X-Continuum-User-Id", "42")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// Cross-customer → 404 (don't leak existence)
	req = httptest.NewRequest(http.MethodGet, "/api/customer/tickets/"+created.TrackingNumber, nil)
	req.Header.Set("X-Continuum-User-Id", "99")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-customer detail status = %d, want 404", rec.Code)
	}
}

func TestTKAdminLifecycle(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	ctx := context.Background()
	cat, _ := st.TKCreateCategory(ctx, store.TKCategory{Slug: "tx", Name: "TX", Active: true})

	body := fmt.Sprintf(`{"categoryId":%d,"subject":"lifecycle","body":"start","customerEmail":"c@d"}`, cat.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/customer/tickets", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "100")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var created store.TKTicket
	json.Unmarshal(rec.Body.Bytes(), &created)
	tn := created.TrackingNumber

	req = httptest.NewRequest(http.MethodPost, "/api/admin/tickets/"+tn+"/reply",
		bytes.NewBufferString(`{"body":"hi"}`))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin reply: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/tickets/"+tn+"/status",
		bytes.NewBufferString(`{"to":"resolved"}`))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin resolve: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer/tickets/"+tn+"/reopen", nil)
	req.Header.Set("X-Continuum-User-Id", "100")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen: %d %s", rec.Code, rec.Body.String())
	}
}

func TestTKAttachmentTooLargeReturns413(t *testing.T) {
	d, _, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)

	big := bytes.Repeat([]byte("x"), 11*1024*1024)
	body := &bytes.Buffer{}
	body.Write(big)

	req := httptest.NewRequest(http.MethodPost, "/api/tickets/entries/1/attachments", body)
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=BOUNDARY")
	req.ContentLength = int64(body.Len())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}
