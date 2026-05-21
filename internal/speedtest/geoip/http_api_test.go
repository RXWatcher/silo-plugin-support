package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPAPITextFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "192.0.2.1") {
			t.Fatalf("path missing ip: %s", r.URL.Path)
		}
		fmt.Fprintln(w, "gb")
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`{"url_pattern": %q, "format": "text"}`, srv.URL+"/{ip}/country/")
	src, err := NewHTTPAPISource(1, json.RawMessage(cfg), nil)
	if err != nil { t.Fatal(err) }
	got, err := src.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil { t.Fatal(err) }
	if got != "GB" {
		t.Fatalf("got %q, want GB", got)
	}
}

func TestHTTPAPIJSONFormatWithPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"country_code": "de", "city": "Berlin"}`)
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`{"url_pattern": %q, "format": "json", "json_path": "country_code"}`, srv.URL+"/{ip}")
	src, err := NewHTTPAPISource(1, json.RawMessage(cfg), nil)
	if err != nil { t.Fatal(err) }
	got, _ := src.Resolve(context.Background(), "203.0.113.7", nil)
	if got != "DE" {
		t.Fatalf("got %q, want DE", got)
	}
}

func TestHTTPAPICacheHits(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		fmt.Fprintln(w, "fr")
	}))
	defer srv.Close()

	cache := newCountryCache()
	cfg := fmt.Sprintf(`{"url_pattern": %q, "format": "text"}`, srv.URL+"/{ip}")
	src, _ := NewHTTPAPISource(1, json.RawMessage(cfg), cache)
	src.Resolve(context.Background(), "203.0.113.8", nil)
	src.Resolve(context.Background(), "203.0.113.8", nil)
	src.Resolve(context.Background(), "203.0.113.8", nil)
	if calls != 1 {
		t.Fatalf("upstream called %d times, want 1 (cache hit)", calls)
	}
}

func TestHTTPAPICacheExpiresAfter30Days(t *testing.T) {
	cache := newCountryCache()
	cache.set("203.0.113.9", "ES", time.Now().Add(-31*24*time.Hour))
	if v := cache.get("203.0.113.9"); v != "" {
		t.Fatalf("expired entry returned %q, want empty", v)
	}
}
