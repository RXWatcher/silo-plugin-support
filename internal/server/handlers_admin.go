package server

import (
	"context"
	"encoding/json"
	"net/http"

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
)

func hAdminPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modules := currentModules(r.Context(), d)
		writeSPA(w, r, supportBootstrap{
			Mode:    "admin-home",
			Theme:   adminTheme(r),
			Modules: modules,
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: true,
		}, http.StatusOK)
	}
}

func hGetConfig(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.ConfigStore == nil {
			writeErr(w, http.StatusServiceUnavailable, "config_store_unavailable", "config storage is not configured")
			return
		}
		cfg, err := d.ConfigStore.GetConfig(r.Context())
		if err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, redactConfig(cfg))
	}
}

func hPatchConfig(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.ConfigStore == nil {
			writeErr(w, http.StatusServiceUnavailable, "config_store_unavailable", "config storage is not configured")
			return
		}
		cur, err := d.ConfigStore.GetConfig(r.Context())
		if err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		var req pluginrt.Config
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		cur.Modules = req.Modules
		next, err := pluginrt.NormalizeAppConfig(cur)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "config_failed", err.Error())
			return
		}
		if err := d.ConfigStore.UpdateConfig(r.Context(), next); err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		fresh, err := d.ConfigStore.GetConfig(r.Context())
		if err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, redactConfig(fresh))
	}
}

func currentModules(ctx context.Context, d Deps) pluginrt.ModuleToggles {
	if d.ConfigStore == nil {
		return pluginrt.DefaultAppConfig().Modules
	}
	cfg, err := d.ConfigStore.GetConfig(ctx)
	if err != nil {
		if d.Logger != nil {
			d.Logger.Warn("currentModules: GetConfig failed, returning defaults", "err", err)
		}
		return pluginrt.DefaultAppConfig().Modules
	}
	return cfg.Modules
}

// redactConfig returns a copy of cfg with secret-bearing fields masked
// before the config is serialized back to an admin client. DatabaseURL
// already carries json:"-" so it never serializes, but we mask it here
// too as defense in depth and to cover any future secret fields.
func redactConfig(cfg pluginrt.Config) pluginrt.Config {
	out := cfg
	if out.DatabaseURL != "" {
		out.DatabaseURL = redactedPlaceholder
	}
	return out
}

const redactedPlaceholder = "***redacted***"
