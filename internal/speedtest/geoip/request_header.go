package geoip

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type RequestHeaderConfig struct {
	Header string `json:"header"`
}

type RequestHeaderSource struct {
	id  int64
	cfg RequestHeaderConfig
}

func NewRequestHeaderSource(id int64, rawCfg json.RawMessage) (*RequestHeaderSource, error) {
	var cfg RequestHeaderConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	return &RequestHeaderSource{id: id, cfg: cfg}, nil
}

func (s *RequestHeaderSource) ID() int64    { return s.id }
func (s *RequestHeaderSource) Kind() string { return "request_header" }

func (s *RequestHeaderSource) Resolve(_ context.Context, _ string, r *http.Request) (string, error) {
	if r == nil || s.cfg.Header == "" {
		return "", nil
	}
	v := strings.ToUpper(strings.TrimSpace(r.Header.Get(s.cfg.Header)))
	// CF and most CDNs use "XX" for unknown — treat as a miss.
	if v == "" || v == "XX" {
		return "", nil
	}
	return v, nil
}
