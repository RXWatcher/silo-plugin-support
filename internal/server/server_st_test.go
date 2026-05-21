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

	"github.com/RXWatcher/continuum-plugin-support/internal/migrate"
	"github.com/RXWatcher/continuum-plugin-support/internal/speedtest"
	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

func stTestDeps(t *testing.T) (Deps, *store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN unset; skipping speedtest integration test")
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
	resolver := speedtest.NewResolver(st, nil, "latency")
	d := Deps{
		ConfigStore:         st,
		STAutoResolver:      resolver,
		STClientIPStorage:   "truncated",
		STSlowThresholdMbps: 5,
	}
	return d, st, func() { pool.Close() }
}

func TestSTCustomerEndpointsRequiresAuth(t *testing.T) {
	d, _, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/endpoints", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestSTAdminRoutesRejectNonAdmin(t *testing.T) {
	d, _, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)
	for _, path := range []string{
		"/admin/speedtest",
		"/api/admin/speedtest/endpoints",
		"/api/admin/speedtest/geoip",
		"/api/admin/speedtest/results",
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

func TestSTEndpointCRUDRoundTrip(t *testing.T) {
	d, _, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)

	body := `{"label":"London","url":"https://lon/","country":"GB","sortOrder":0,"active":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/speedtest/endpoints", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create endpoint status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var ep store.STEndpoint
	if err := json.Unmarshal(rec.Body.Bytes(), &ep); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/endpoints", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("customer list status = %d", rec.Code)
	}

	rbody := fmt.Sprintf(`{"endpointId":%d,"endpointLabel":"London","downloadMbps":142.3,"uploadMbps":18.7,"pingMs":28,"jitterMs":2.1}`, ep.ID)
	req = httptest.NewRequest(http.MethodPost, "/api/customer/speedtest/results", bytes.NewBufferString(rbody))
	req.Header.Set("X-Continuum-User-Id", "9")
	req.RemoteAddr = "192.0.2.50:1234"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save result status = %d, body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer/speedtest/results", bytes.NewBufferString(rbody))
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate-limit 429, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/results", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d", rec.Code)
	}
}

func TestSTAutoLatencyReturnsCandidates(t *testing.T) {
	d, st, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)

	_, err := st.STCreateEndpoint(context.Background(), store.STEndpoint{
		Label: "London", URL: "https://lon/", Country: "GB", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/auto", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("auto status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out speedtest.AutoResolution
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Strategy != "latency" || len(out.Candidates) == 0 {
		t.Fatalf("unexpected resolution: %+v", out)
	}
}
