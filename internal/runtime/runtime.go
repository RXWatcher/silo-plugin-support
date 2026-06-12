// Package runtime is the SDK-facing plugin runtime: it owns the
// GetManifest / Configure RPC and hands a normalized Config to
// main.go's onConfig callback.
package runtime

import (
	"context"
	"fmt"
	"sync"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtimedefault"
)

// Config is the union of manifest-supplied and DB-persisted plugin
// settings. DatabaseURL is manifest-only; everything else round-trips
// through app_config.data as JSONB.
type Config struct {
	DatabaseURL string        `json:"-"`
	Modules     ModuleToggles `json:"modules"`

	// Speedtest module config.
	AutoStrategy      string  `json:"auto_strategy,omitempty"`
	GeoIPCacheDir     string  `json:"geoip_cache_dir,omitempty"`
	ClientIPStorage   string  `json:"client_ip_storage,omitempty"`
	SlowThresholdMbps float64 `json:"slow_threshold_mbps,omitempty"`

	// Tickets module config.
	TicketsAutoCloseEnabled        bool `json:"tickets_auto_close_enabled"`
	TicketsResolvedCloseAfterDays  int  `json:"tickets_resolved_close_after_days"`
	TicketsWaitingCloseAfterDays   int  `json:"tickets_waiting_close_after_days"`

	// Tickets spam / abuse + quota controls. Zero means "use the
	// in-code default" (see DefaultAppConfig); a negative value means
	// the limit is disabled where that is meaningful.
	TicketsMaxOpenPerCustomer      int `json:"tickets_max_open_per_customer"`
	TicketsMinBodyChars            int `json:"tickets_min_body_chars"`
	TicketsMaxBodyChars            int `json:"tickets_max_body_chars"`
	TicketsMaxAttachmentsPerTicket int `json:"tickets_max_attachments_per_ticket"`
	TicketsMaxStorageBytesPerCustomer int64 `json:"tickets_max_storage_bytes_per_customer"`
}

// ModuleToggles controls which modules are exposed in the UI. All
// default off; each module's release flips its own toggle to true in
// DefaultAppConfig and adds its routes to the manifest.
type ModuleToggles struct {
	KB        bool `json:"kb"`
	Speedtest bool `json:"speedtest"`
	Tickets   bool `json:"tickets"`
	AI        bool `json:"ai"`
}

type Server struct {
	runtimedefault.Server
	manifest *pluginv1.PluginManifest
	onConfig func(Config) error

	mu  sync.RWMutex
	cfg Config
}

func New(manifest *pluginv1.PluginManifest, onConfig func(Config) error) *Server {
	return &Server{manifest: manifest, onConfig: onConfig}
}

func (s *Server) GetManifest(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

// DefaultAppConfig returns the in-code defaults applied when no DB
// row exists yet. Each module ship flips its own toggle to true.
func DefaultAppConfig() Config {
	return Config{
		Modules: ModuleToggles{KB: true, Speedtest: true, Tickets: true},
		AutoStrategy:      "latency",
		ClientIPStorage:   "truncated",
		SlowThresholdMbps: 5,

		TicketsAutoCloseEnabled:       true,
		TicketsResolvedCloseAfterDays: 7,
		TicketsWaitingCloseAfterDays:  14,

		TicketsMaxOpenPerCustomer:         10,
		TicketsMinBodyChars:               10,
		TicketsMaxBodyChars:               20000,
		TicketsMaxAttachmentsPerTicket:    20,
		TicketsMaxStorageBytesPerCustomer: 50 << 20, // 50 MB
	}
}

// NormalizeAppConfig validates a Config and returns it.
func NormalizeAppConfig(cfg Config) (Config, error) {
	if cfg.AutoStrategy == "" {
		cfg.AutoStrategy = "latency"
	}
	if cfg.AutoStrategy != "latency" && cfg.AutoStrategy != "geoip" {
		return Config{}, fmt.Errorf("auto_strategy must be 'latency' or 'geoip'")
	}
	if cfg.ClientIPStorage == "" {
		cfg.ClientIPStorage = "truncated"
	}
	if cfg.ClientIPStorage != "truncated" && cfg.ClientIPStorage != "off" {
		return Config{}, fmt.Errorf("client_ip_storage must be 'truncated' or 'off'")
	}
	if cfg.SlowThresholdMbps < 0 {
		return Config{}, fmt.Errorf("slow_threshold_mbps must be >= 0")
	}
	if cfg.TicketsResolvedCloseAfterDays < 0 {
		return Config{}, fmt.Errorf("tickets_resolved_close_after_days must be >= 0")
	}
	if cfg.TicketsWaitingCloseAfterDays < 0 {
		return Config{}, fmt.Errorf("tickets_waiting_close_after_days must be >= 0")
	}
	// Spam / abuse + quota limits: a stored 0 (e.g. a pre-existing
	// app_config row that predates these keys) is treated as "unset" and
	// backfilled from the in-code defaults so the protections are always
	// active. Negative values are rejected.
	def := DefaultAppConfig()
	if cfg.TicketsMaxOpenPerCustomer == 0 {
		cfg.TicketsMaxOpenPerCustomer = def.TicketsMaxOpenPerCustomer
	}
	if cfg.TicketsMinBodyChars == 0 {
		cfg.TicketsMinBodyChars = def.TicketsMinBodyChars
	}
	if cfg.TicketsMaxBodyChars == 0 {
		cfg.TicketsMaxBodyChars = def.TicketsMaxBodyChars
	}
	if cfg.TicketsMaxAttachmentsPerTicket == 0 {
		cfg.TicketsMaxAttachmentsPerTicket = def.TicketsMaxAttachmentsPerTicket
	}
	if cfg.TicketsMaxStorageBytesPerCustomer == 0 {
		cfg.TicketsMaxStorageBytesPerCustomer = def.TicketsMaxStorageBytesPerCustomer
	}
	if cfg.TicketsMaxOpenPerCustomer < 0 ||
		cfg.TicketsMinBodyChars < 0 ||
		cfg.TicketsMaxBodyChars < 0 ||
		cfg.TicketsMaxAttachmentsPerTicket < 0 ||
		cfg.TicketsMaxStorageBytesPerCustomer < 0 {
		return Config{}, fmt.Errorf("tickets spam/quota limits must be >= 0")
	}
	if cfg.TicketsMaxBodyChars < cfg.TicketsMinBodyChars {
		return Config{}, fmt.Errorf("tickets_max_body_chars must be >= tickets_min_body_chars")
	}
	return cfg, nil
}

func (s *Server) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	cfg := DefaultAppConfig()
	for _, e := range req.GetConfig() {
		if e.GetValue() == nil {
			continue
		}
		m := e.GetValue().AsMap()
		switch e.GetKey() {
		case "database_url":
			cfg.DatabaseURL = stringValue(m["value"], firstString(m))
		}
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database_url is required")
	}
	var err error
	cfg, err = NormalizeAppConfig(cfg)
	if err != nil {
		return nil, err
	}
	if s.onConfig != nil {
		if err := s.onConfig(cfg); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

func stringValue(candidates ...any) string {
	for _, c := range candidates {
		if s, ok := c.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func firstString(m map[string]any) any {
	for _, v := range m {
		if _, ok := v.(string); ok {
			return v
		}
	}
	return nil
}
