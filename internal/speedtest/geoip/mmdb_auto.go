package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

type MMDBAutoConfig struct {
	URLPattern  string `json:"url_pattern"` // contains {YYYY-MM}
	RefreshDays int    `json:"refresh_days,omitempty"`
}

// MMDBAutoSource wraps mmdbReader plus a background refresh
// lifecycle. The downloader is invoked from main.go (or admin
// refresh trigger) rather than per-Resolve so resolution stays
// fast and lock-free in the steady state.
type MMDBAutoSource struct {
	id       int64
	cfg      MMDBAutoConfig
	cacheDir string
	reader   *mmdbReader

	mu              sync.Mutex
	lastRefreshedAt time.Time
}

func NewMMDBAutoSource(id int64, rawCfg json.RawMessage, cacheDir string) (*MMDBAutoSource, error) {
	var cfg MMDBAutoConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	if cfg.RefreshDays <= 0 {
		cfg.RefreshDays = 25
	}
	return &MMDBAutoSource{
		id:       id,
		cfg:      cfg,
		cacheDir: cacheDir,
		reader:   newMMDBReader(),
	}, nil
}

func (m *MMDBAutoSource) ID() int64    { return m.id }
func (m *MMDBAutoSource) Kind() string { return "mmdb_auto" }

func (m *MMDBAutoSource) Resolve(ctx context.Context, ip string, _ *http.Request) (string, error) {
	return m.reader.Country(ctx, ip)
}

// LocalPath returns the cache-dir path where the downloader writes
// this source's .mmdb. Stable across restarts.
func (m *MMDBAutoSource) LocalPath() string {
	return filepath.Join(m.cacheDir, fmt.Sprintf("%d.mmdb", m.id))
}

// NeedsRefresh: true if we've never loaded OR last refresh is older
// than RefreshDays. Caller decides whether to fire the download.
func (m *MMDBAutoSource) NeedsRefresh() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastRefreshedAt.IsZero() {
		return true
	}
	return time.Since(m.lastRefreshedAt) > time.Duration(m.cfg.RefreshDays)*24*time.Hour
}

// Refresh downloads the mmdb and opens the reader. Safe to call from
// a goroutine; uses an internal lock.
func (m *MMDBAutoSource) Refresh(ctx context.Context) error {
	if err := downloadMMDB(ctx, m.cfg.URLPattern, m.LocalPath()); err != nil {
		return err
	}
	if err := m.reader.Open(m.LocalPath()); err != nil {
		return err
	}
	m.mu.Lock()
	m.lastRefreshedAt = time.Now()
	m.mu.Unlock()
	return nil
}

// LoadCached opens whatever file is already at LocalPath without
// downloading. Used on plugin start to make resolution work
// immediately if a previous run already downloaded the file.
func (m *MMDBAutoSource) LoadCached() error {
	if err := m.reader.Open(m.LocalPath()); err != nil {
		return err
	}
	return nil
}
