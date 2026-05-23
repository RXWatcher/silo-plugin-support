package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/RXWatcher/silo-plugin-support/internal/speedtest"
	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

func stCustomerStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

func hSTSpeedtestPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode:    "speedtest",
			Theme:   adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: r.Header.Get("X-Silo-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hSTCustomerEndpoints(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eps, err := stCustomerStore(d).STListEndpoints(r.Context(), true)
		if err != nil {
			writeInternal(w, r, d, "st_endpoints_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, eps)
	}
}

func hSTCustomerAuto(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.STAutoResolver == nil {
			writeErr(w, http.StatusServiceUnavailable, "st_unconfigured", "speedtest resolver not configured")
			return
		}
		ip := clientIP(r)
		out, err := d.STAutoResolver.Resolve(r.Context(), ip, r)
		if err != nil {
			writeInternal(w, r, d, "st_auto_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type stResultRequest struct {
	EndpointID    int64   `json:"endpointId,omitempty"`
	EndpointLabel string  `json:"endpointLabel"`
	AutoStrategy  string  `json:"autoStrategy,omitempty"`
	DownloadMbps  float64 `json:"downloadMbps"`
	UploadMbps    float64 `json:"uploadMbps"`
	PingMs        float64 `json:"pingMs"`
	JitterMs      float64 `json:"jitterMs"`
}

func hSTCustomerSaveResult(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerID := r.Header.Get("X-Silo-User-Id")

		// 60s per-customer rate limit.
		last, err := stCustomerStore(d).STLastResultAt(r.Context(), customerID)
		if err != nil {
			writeInternal(w, r, d, "st_rate_check_failed", err)
			return
		}
		if !last.IsZero() && time.Since(last) < 60*time.Second {
			retryIn := 60 - int(time.Since(last).Seconds())
			writeErr(w, http.StatusTooManyRequests, "st_rate_limited",
				"please wait "+ts(retryIn)+" before running another test")
			return
		}

		var req stResultRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}

		ip := clientIP(r)
		truncated := speedtest.TruncateIP(ip, d.STClientIPStorage)

		var epID *int64
		if req.EndpointID > 0 {
			id := req.EndpointID
			epID = &id
		}

		saved, err := stCustomerStore(d).STInsertResult(r.Context(), store.STResult{
			CustomerID:    customerID,
			EndpointID:    epID,
			EndpointLabel: req.EndpointLabel,
			AutoStrategy:  req.AutoStrategy,
			DownloadMbps:  req.DownloadMbps,
			UploadMbps:    req.UploadMbps,
			PingMs:        req.PingMs,
			JitterMs:      req.JitterMs,
			ClientIP:      truncated,
			UserAgent:     r.UserAgent(),
		})
		if err != nil {
			writeInternal(w, r, d, "st_save_failed", err)
			return
		}

		stPublishEvent(d, "speedtest_run", saved, nil)
		if d.STSlowThresholdMbps > 0 && saved.DownloadMbps < d.STSlowThresholdMbps {
			stPublishEvent(d, "speedtest_slow", saved, map[string]any{
				"threshold_mbps": d.STSlowThresholdMbps,
				"slow_by_mbps":   d.STSlowThresholdMbps - saved.DownloadMbps,
			})
		}

		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTCustomerHistory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hist, err := stCustomerStore(d).STCustomerHistory(r.Context(),
			r.Header.Get("X-Silo-User-Id"), 20)
		if err != nil {
			writeInternal(w, r, d, "st_history_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, hist)
	}
}

// clientIP returns the best-guess client IP, preferring X-Forwarded-For's
// first entry when present (Silo runs behind a reverse proxy).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func ts(n int) string {
	if n <= 1 {
		return "1 second"
	}
	return itoa(n) + " seconds"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
