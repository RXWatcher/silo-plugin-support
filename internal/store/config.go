package store

import (
	"context"
	"encoding/json"
	"fmt"

	pluginrt "github.com/ContinuumApp/continuum-plugin-support/internal/runtime"
)

// GetConfig reads the singleton app_config row. Returns the
// in-code default if the row is empty or missing.
func (s *Store) GetConfig(ctx context.Context) (pluginrt.Config, error) {
	var data []byte
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM app_config WHERE id = 1`).Scan(&data)
	if err != nil {
		return pluginrt.DefaultAppConfig(), fmt.Errorf("read app_config: %w", err)
	}
	cfg := pluginrt.DefaultAppConfig()
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return pluginrt.DefaultAppConfig(), fmt.Errorf("parse app_config.data: %w", err)
		}
	}
	return cfg, nil
}

// UpdateConfig persists the JSONB shape of cfg into the singleton
// row. DatabaseURL is never persisted (it's manifest-only).
func (s *Store) UpdateConfig(ctx context.Context, cfg pluginrt.Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal app_config: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE app_config SET data = $1, updated_at = NOW() WHERE id = 1`, data)
	if err != nil {
		return fmt.Errorf("update app_config: %w", err)
	}
	return nil
}

// Bootstrap merges manifest-supplied cfg with whatever is already
// persisted, applies in-code defaults, normalises, and persists the
// result. Returns the canonical config that survives reinstalls.
func (s *Store) Bootstrap(ctx context.Context, cfg pluginrt.Config) (pluginrt.Config, error) {
	stored, err := s.GetConfig(ctx)
	if err != nil {
		stored = pluginrt.DefaultAppConfig()
	}
	merged := stored
	merged.DatabaseURL = cfg.DatabaseURL
	merged, err = pluginrt.NormalizeAppConfig(merged)
	if err != nil {
		return pluginrt.Config{}, err
	}
	if err := s.UpdateConfig(ctx, merged); err != nil {
		return pluginrt.Config{}, err
	}
	return merged, nil
}
