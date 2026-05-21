package geoip

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

// mmdbReader wraps geoip2.Reader with a mutex so the file can be
// hot-swapped (mmdb_auto downloader replaces the underlying file +
// reopens). Both mmdb_auto and mmdb_file use one of these.
type mmdbReader struct {
	mu     sync.RWMutex
	reader *geoip2.Reader
	path   string
}

func newMMDBReader() *mmdbReader { return &mmdbReader{} }

// Open the .mmdb at path. Replaces any previously open reader.
// Returns an error if the file is missing or unparseable.
func (m *mmdbReader) Open(path string) error {
	r, err := geoip2.Open(path)
	if err != nil {
		return fmt.Errorf("open mmdb %q: %w", path, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reader != nil {
		_ = m.reader.Close()
	}
	m.reader = r
	m.path = path
	return nil
}

// Country looks up the ISO country code for ip. Returns "" if the
// reader has not been opened yet OR the lookup misses (private IP,
// reserved range). Errors are returned for parse failures only.
func (m *mmdbReader) Country(_ context.Context, ip string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.reader == nil {
		return "", nil
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("invalid ip %q", ip)
	}
	rec, err := m.reader.Country(parsed)
	if err != nil {
		return "", fmt.Errorf("mmdb lookup: %w", err)
	}
	return rec.Country.IsoCode, nil
}
