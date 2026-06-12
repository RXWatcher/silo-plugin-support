package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/silo-plugin-support/internal/safeurl"
	"github.com/RXWatcher/silo-plugin-support/internal/speedtest/geoip"
	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

func stAdminStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// Admin SPA shell handlers.
func hSTAdminEndpointsPage(d Deps) http.HandlerFunc  { return adminSPAHandler(d, "admin-st-endpoints") }
func hSTAdminGeoIPPage(d Deps) http.HandlerFunc      { return adminSPAHandler(d, "admin-st-geoip") }
func hSTAdminResultsPage(d Deps) http.HandlerFunc    { return adminSPAHandler(d, "admin-st-results") }
func hSTAdminDashboardsPage(d Deps) http.HandlerFunc { return adminSPAHandler(d, "admin-st-dashboards") }

// --- Endpoints CRUD --------------------------------------------------

type stEndpointRequest struct {
	Label     string `json:"label"`
	URL       string `json:"url"`
	Country   string `json:"country"`
	Region    string `json:"region"`
	SortOrder int    `json:"sortOrder"`
	Active    bool   `json:"active"`
}

func hSTAdminListEndpoints(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eps, err := stAdminStore(d).STListEndpoints(r.Context(), false)
		if err != nil {
			writeInternal(w, r, d, "st_endpoints_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, eps)
	}
}

func hSTAdminCreateEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req stEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if req.Label == "" || req.URL == "" {
			writeErr(w, http.StatusBadRequest, "bad_endpoint", "label and url are required")
			return
		}
		saved, err := stAdminStore(d).STCreateEndpoint(r.Context(), store.STEndpoint{
			Label: req.Label, URL: req.URL, Country: req.Country, Region: req.Region,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil {
			writeInternal(w, r, d, "st_endpoint_create_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminUpdateEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid endpoint id")
			return
		}
		var req stEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		saved, err := stAdminStore(d).STUpdateEndpoint(r.Context(), store.STEndpoint{
			ID: id, Label: req.Label, URL: req.URL, Country: req.Country, Region: req.Region,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "endpoint not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_endpoint_update_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminDeleteEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid endpoint id")
			return
		}
		if err := stAdminStore(d).STDeleteEndpoint(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "st_not_found", "endpoint not found")
				return
			}
			writeInternal(w, r, d, "st_endpoint_delete_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hSTAdminPingEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid endpoint id")
			return
		}
		ep, err := stAdminStore(d).STGetEndpoint(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "endpoint not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_endpoint_get_failed", err)
			return
		}
		target := ep.URL + "/empty.php"
		if err := safeurl.Validate(target); err != nil {
			writeErr(w, http.StatusBadRequest, "st_endpoint_unsafe_url", "endpoint URL is not a permitted target: "+err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, target, nil)
		client := &http.Client{Timeout: 5 * time.Second}
		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start).Milliseconds()
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "elapsed_ms": elapsed})
			return
		}
		resp.Body.Close()
		writeJSON(w, http.StatusOK, map[string]any{"ok": resp.StatusCode == http.StatusOK, "status": resp.StatusCode, "elapsed_ms": elapsed})
	}
}

// --- GeoIP sources CRUD ---------------------------------------------

type stGeoIPSourceRequest struct {
	Label     string          `json:"label"`
	Kind      string          `json:"kind"`
	Config    json.RawMessage `json:"config"`
	SortOrder int             `json:"sortOrder"`
	Active    bool            `json:"active"`
}

func hSTAdminListGeoIPSources(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		srcs, err := stAdminStore(d).STListGeoIPSources(r.Context(), false)
		if err != nil {
			writeInternal(w, r, d, "st_geoip_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, srcs)
	}
}

func hSTAdminCreateGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req stGeoIPSourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		validKinds := map[string]bool{"mmdb_auto": true, "mmdb_file": true, "http_api": true, "request_header": true}
		if !validKinds[req.Kind] {
			writeErr(w, http.StatusBadRequest, "bad_kind", "kind must be mmdb_auto / mmdb_file / http_api / request_header")
			return
		}
		if req.Label == "" {
			writeErr(w, http.StatusBadRequest, "bad_label", "label is required")
			return
		}
		if len(req.Config) == 0 {
			req.Config = json.RawMessage(`{}`)
		}
		saved, err := stAdminStore(d).STCreateGeoIPSource(r.Context(), store.STGeoIPSource{
			Label: req.Label, Kind: req.Kind, Config: req.Config,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil {
			writeInternal(w, r, d, "st_geoip_create_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminUpdateGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		var req stGeoIPSourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if len(req.Config) == 0 {
			req.Config = json.RawMessage(`{}`)
		}
		// Kind is immutable; ignore req.Kind on update.
		cur, err := stAdminStore(d).STGetGeoIPSource(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_geoip_get_failed", err)
			return
		}
		cur.Label = req.Label
		cur.Config = req.Config
		cur.SortOrder = req.SortOrder
		cur.Active = req.Active
		saved, err := stAdminStore(d).STUpdateGeoIPSource(r.Context(), cur)
		if err != nil {
			writeInternal(w, r, d, "st_geoip_update_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminDeleteGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		if err := stAdminStore(d).STDeleteGeoIPSource(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
				return
			}
			writeInternal(w, r, d, "st_geoip_delete_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hSTAdminRefreshGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		row, err := stAdminStore(d).STGetGeoIPSource(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_geoip_get_failed", err)
			return
		}
		if row.Kind != "mmdb_auto" {
			writeErr(w, http.StatusBadRequest, "st_geoip_not_auto", "only mmdb_auto sources can be refreshed")
			return
		}
		cacheDir := geoipCacheDir(d)
		src, err := geoip.NewMMDBAutoSource(row.ID, row.Config, cacheDir)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "st_geoip_bad_config", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		if err := src.Refresh(ctx); err != nil {
			_ = stAdminStore(d).STMarkGeoIPSourceStatus(r.Context(), row.ID, "error: "+err.Error())
			writeInternal(w, r, d, "st_geoip_refresh_failed", err)
			return
		}
		_ = stAdminStore(d).STMarkGeoIPSourceRefreshed(r.Context(), row.ID)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hSTAdminTestGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<10))
		var req struct {
			IP string `json:"ip"`
		}
		_ = json.Unmarshal(body, &req)
		if req.IP == "" {
			req.IP = clientIP(r)
		}
		row, err := stAdminStore(d).STGetGeoIPSource(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_geoip_get_failed", err)
			return
		}
		src, err := geoip.BuildSource(row, geoipCacheDir(d))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "st_geoip_bad_config", err.Error())
			return
		}
		country, srcErr := src.Resolve(r.Context(), req.IP, r)
		writeJSON(w, http.StatusOK, map[string]any{
			"ip":      req.IP,
			"country": country,
			"error":   errString(srcErr),
		})
	}
}

func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// geoipCacheDir picks the on-disk cache dir for mmdb_auto downloads.
// Resolution order:
//  1. Explicit Deps.STGeoIPCacheDir from config
//  2. $XDG_CACHE_HOME/silo-plugin-support/geoip
//  3. $HOME/.cache/silo-plugin-support/geoip
//  4. .silo-plugin-support-cache/geoip (last-resort relative, e.g. for tests)
func geoipCacheDir(d Deps) string {
	if d.STGeoIPCacheDir != "" {
		return d.STGeoIPCacheDir
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "silo-plugin-support", "geoip")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "silo-plugin-support", "geoip")
	}
	return filepath.Join(".silo-plugin-support-cache", "geoip")
}

// --- Results + dashboards -------------------------------------------

func hSTAdminListResults(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.STResultFilter{
			CustomerID:   r.URL.Query().Get("customerId"),
			EndpointID:   parseInt64(r.URL.Query().Get("endpointId")),
			AutoStrategy: r.URL.Query().Get("autoStrategy"),
			Limit:        parseLimit(r.URL.Query().Get("limit"), 200),
			Offset:       parseInt(r.URL.Query().Get("offset")),
		}
		if since := r.URL.Query().Get("since"); since != "" {
			if ts, err := time.Parse(time.RFC3339, since); err == nil {
				f.Since = ts
			}
		}
		if r.URL.Query().Get("slowOnly") == "true" {
			f.SlowOnly = true
			f.SlowThresh = d.STSlowThresholdMbps
		}
		out, err := stAdminStore(d).STListResults(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "st_results_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func hSTAdminDashboards(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := stAdminStore(d).STDashboardAggregatesData(r.Context(), d.STSlowThresholdMbps)
		if err != nil {
			writeInternal(w, r, d, "st_dashboards_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
