package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type HTTPAPIConfig struct {
	URLPattern string `json:"url_pattern"`        // contains {ip}
	Format     string `json:"format"`              // "text" | "json"
	JSONPath   string `json:"json_path,omitempty"` // dot-path for json format
}

type HTTPAPISource struct {
	id    int64
	cfg   HTTPAPIConfig
	cache *countryCache
	httpc *http.Client
}

// NewHTTPAPISource. cache may be nil — a new one is created if so.
func NewHTTPAPISource(id int64, rawCfg json.RawMessage, cache *countryCache) (*HTTPAPISource, error) {
	var cfg HTTPAPIConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	if cache == nil {
		cache = newCountryCache()
	}
	return &HTTPAPISource{
		id:    id,
		cfg:   cfg,
		cache: cache,
		httpc: &http.Client{Timeout: 2 * time.Second},
	}, nil
}

func (s *HTTPAPISource) ID() int64    { return s.id }
func (s *HTTPAPISource) Kind() string { return "http_api" }

func (s *HTTPAPISource) Resolve(ctx context.Context, ip string, _ *http.Request) (string, error) {
	if s.cfg.URLPattern == "" || ip == "" {
		return "", nil
	}
	if cached := s.cache.get(ip); cached != "" {
		return cached, nil
	}
	url := strings.ReplaceAll(s.cfg.URLPattern, "{ip}", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http_api %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	country, err := s.extract(body)
	if err != nil || country == "" {
		return country, err
	}
	country = strings.ToUpper(strings.TrimSpace(country))
	if country == "XX" {
		country = ""
	}
	if country != "" {
		s.cache.set(ip, country, time.Now())
	}
	return country, nil
}

func (s *HTTPAPISource) extract(body []byte) (string, error) {
	if s.cfg.Format == "json" {
		var any any
		if err := json.Unmarshal(body, &any); err != nil {
			return "", err
		}
		return jsonPath(any, s.cfg.JSONPath), nil
	}
	return string(body), nil
}

// jsonPath walks a dot-separated path through a generic JSON tree.
// Returns "" for any miss.
func jsonPath(node any, path string) string {
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		m, ok := node.(map[string]any)
		if !ok {
			return ""
		}
		node = m[part]
	}
	if s, ok := node.(string); ok {
		return s
	}
	return ""
}

// countryCache: per-IP cache with 30-day TTL. Lost on restart;
// re-warms quickly. Mutex-protected.
type countryCache struct {
	mu sync.RWMutex
	m  map[string]cacheEntry
}

type cacheEntry struct {
	country string
	setAt   time.Time
}

const cacheTTL = 30 * 24 * time.Hour

func newCountryCache() *countryCache {
	return &countryCache{m: map[string]cacheEntry{}}
}

func (c *countryCache) get(ip string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[ip]
	if !ok || time.Since(e.setAt) > cacheTTL {
		return ""
	}
	return e.country
}

func (c *countryCache) set(ip, country string, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[ip] = cacheEntry{country: country, setAt: at}
}
