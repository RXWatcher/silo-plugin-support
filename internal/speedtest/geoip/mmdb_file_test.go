package geoip

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMMDBFileEmptyPathReturnsEmpty(t *testing.T) {
	src, err := NewMMDBFileSource(1, json.RawMessage(`{"path": ""}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil || got != "" {
		t.Fatalf("got (%q, %v), want ('', nil)", got, err)
	}
}

func TestMMDBFileMissingPathReturnsStatError(t *testing.T) {
	src, _ := NewMMDBFileSource(1, json.RawMessage(`{"path": "/nonexistent/file.mmdb"}`))
	got, err := src.Resolve(context.Background(), "192.0.2.1", nil)
	if err == nil {
		t.Fatalf("expected stat error, got (%q, nil)", got)
	}
}

func TestMMDBFileCorruptFileSurfacesOpenError(t *testing.T) {
	// Create a temp file with garbage bytes — geoip2.Open should reject.
	tmp := filepath.Join(t.TempDir(), "broken.mmdb")
	if err := os.WriteFile(tmp, []byte("not a real mmdb file\x00\x01\x02"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := `{"path": "` + tmp + `"}`
	src, _ := NewMMDBFileSource(1, json.RawMessage(cfg))
	// First call surfaces the open error.
	_, err1 := src.Resolve(context.Background(), "192.0.2.1", nil)
	if err1 == nil {
		t.Fatalf("first call: expected open error, got nil")
	}
	// Second call must surface the SAME error — not silently return empty.
	_, err2 := src.Resolve(context.Background(), "203.0.113.1", nil)
	if err2 == nil {
		t.Fatalf("second call: expected open error to persist, got nil")
	}
}
