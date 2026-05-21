package geoip

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestHeaderReadsConfiguredHeader(t *testing.T) {
	src, err := NewRequestHeaderSource(1, json.RawMessage(`{"header": "CF-IPCountry"}`))
	if err != nil { t.Fatal(err) }
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("CF-IPCountry", "gb")
	got, err := src.Resolve(context.Background(), "192.0.2.1", r)
	if err != nil { t.Fatal(err) }
	if got != "GB" {
		t.Fatalf("got %q, want GB", got)
	}
}

func TestRequestHeaderMissingHeaderReturnsEmpty(t *testing.T) {
	src, _ := NewRequestHeaderSource(1, json.RawMessage(`{"header": "CF-IPCountry"}`))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	got, _ := src.Resolve(context.Background(), "192.0.2.1", r)
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestRequestHeaderXXIsTreatedAsEmpty(t *testing.T) {
	// Cloudflare uses "XX" for unknown country.
	src, _ := NewRequestHeaderSource(1, json.RawMessage(`{"header": "CF-IPCountry"}`))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("CF-IPCountry", "XX")
	got, _ := src.Resolve(context.Background(), "192.0.2.1", r)
	if got != "" {
		t.Fatalf("got %q, want empty for XX", got)
	}
}
