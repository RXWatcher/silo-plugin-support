package geoip

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// realMMDBPath returns the path to a real .mmdb test fixture. If the
// MMDB_FIXTURE env var is set, it's used directly. Otherwise we try
// internal/speedtest/geoip/testdata/sample.mmdb (which an operator
// can drop in manually). When neither is available, t.Skip — CI
// without network access still passes.
func realMMDBPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("MMDB_FIXTURE"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	candidate := filepath.Join("testdata", "sample.mmdb")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	t.Skip("no real mmdb fixture available (set MMDB_FIXTURE or put one at testdata/sample.mmdb)")
	return ""
}

func TestMMDBAutoNeedsRefreshBeforeFirstLoad(t *testing.T) {
	src, err := NewMMDBAutoSource(1, json.RawMessage(`{"url_pattern": "https://example.invalid/{YYYY-MM}.mmdb.gz", "refresh_days": 25}`), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !src.NeedsRefresh() {
		t.Fatal("a fresh source must NeedsRefresh = true")
	}
}

func TestMMDBAutoLocalPathIsStable(t *testing.T) {
	dir := t.TempDir()
	src, _ := NewMMDBAutoSource(42, json.RawMessage(`{"url_pattern": "x", "refresh_days": 25}`), dir)
	want := filepath.Join(dir, "42.mmdb")
	if got := src.LocalPath(); got != want {
		t.Fatalf("LocalPath = %q, want %q", got, want)
	}
}

// TestMMDBAutoRefreshAtomicSwap downloads a real mmdb (via the fixture
// helper) through a local httptest server, asserts the file lands at
// LocalPath after the swap, and that the source can resolve an IP.
func TestMMDBAutoRefreshAtomicSwap(t *testing.T) {
	fixturePath := realMMDBPath(t)
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	// Serve the fixture as plain bytes (not gzipped — URL has no .gz suffix).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(fixtureBytes)
	}))
	defer srv.Close()

	dir := t.TempDir()
	src, err := NewMMDBAutoSource(1,
		json.RawMessage(`{"url_pattern": "`+srv.URL+`/{YYYY-MM}.mmdb", "refresh_days": 25}`),
		dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := src.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	// File must land at LocalPath.
	if _, err := os.Stat(src.LocalPath()); err != nil {
		t.Fatalf("LocalPath after refresh: %v", err)
	}
	// Source resolves now (result depends on the DB; we just need no error).
	_, err = src.Resolve(ctx, "8.8.8.8", nil)
	if err != nil {
		t.Fatalf("Resolve after refresh: %v", err)
	}
}

// TestMMDBAutoMissingURLPrimaryFallsBackToPrev verifies the "previous
// month" fallback fires when the primary URL returns 404. Uses two
// httptest endpoints — the primary always 404s, the fallback serves
// the fixture.
func TestMMDBAutoMissingURLPrimaryFallsBackToPrev(t *testing.T) {
	fixturePath := realMMDBPath(t)
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	prevMonth := time.Now().UTC().AddDate(0, -1, 0).Format("2006-01")
	currMonth := time.Now().UTC().Format("2006-01")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/"+currMonth+".mmdb":
			http.NotFound(w, r)
		case r.URL.Path == "/"+prevMonth+".mmdb":
			_, _ = w.Write(fixtureBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	src, _ := NewMMDBAutoSource(1,
		json.RawMessage(`{"url_pattern": "`+srv.URL+`/{YYYY-MM}.mmdb", "refresh_days": 25}`),
		dir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := src.Refresh(ctx); err != nil {
		t.Fatalf("Refresh (expected to use prev-month fallback): %v", err)
	}
	if _, err := os.Stat(src.LocalPath()); err != nil {
		t.Fatalf("LocalPath after fallback refresh: %v", err)
	}
}
