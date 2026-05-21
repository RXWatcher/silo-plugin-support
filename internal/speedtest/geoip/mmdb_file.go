package geoip

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
)

// MMDBFileConfig is the operator-supplied path to an .mmdb file the
// plugin only reads. Updates are the operator's job (cron,
// geoipupdate, etc).
type MMDBFileConfig struct {
	Path string `json:"path"`
}

// MMDBFileSource implements Source for `kind = 'mmdb_file'`.
type MMDBFileSource struct {
	id     int64
	cfg    MMDBFileConfig
	reader *mmdbReader
	once   sync.Once
}

func NewMMDBFileSource(id int64, rawCfg json.RawMessage) (*MMDBFileSource, error) {
	var cfg MMDBFileConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	return &MMDBFileSource{id: id, cfg: cfg, reader: newMMDBReader()}, nil
}

func (m *MMDBFileSource) ID() int64    { return m.id }
func (m *MMDBFileSource) Kind() string { return "mmdb_file" }

func (m *MMDBFileSource) Resolve(ctx context.Context, ip string, _ *http.Request) (string, error) {
	if m.cfg.Path == "" {
		return "", nil
	}
	if _, err := os.Stat(m.cfg.Path); err != nil {
		return "", err
	}
	m.once.Do(func() { _ = m.reader.Open(m.cfg.Path) })
	return m.reader.Country(ctx, ip)
}
