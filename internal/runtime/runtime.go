// Package runtime is the SDK-facing plugin runtime: it owns the
// GetManifest / Configure RPC and hands a normalized Config to
// main.go's onConfig callback.
package runtime

import (
	"context"
	"fmt"
	"sync"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/continuum/plugin/v1"
	"github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/runtimedefault"
)

// Config is the union of manifest-supplied and DB-persisted plugin
// settings. DatabaseURL is manifest-only; everything else round-trips
// through app_config.data as JSONB.
type Config struct {
	DatabaseURL string        `json:"-"`
	Modules     ModuleToggles `json:"modules"`
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
	return Config{Modules: ModuleToggles{}}
}

// NormalizeAppConfig validates a Config and returns it. Validation
// is minimal at shell time — module-specific validation lives in
// module specs.
func NormalizeAppConfig(cfg Config) (Config, error) {
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
