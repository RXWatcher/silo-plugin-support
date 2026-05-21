package server

import (
	"context"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

// stPublishEvent assembles the base speedtest payload + extra keys
// and hands off to Deps.EventPublisher. No-ops when EventPublisher
// is nil (test contexts). Best-effort.
func stPublishEvent(d Deps, name string, r store.STResult, extra map[string]any) {
	if d.EventPublisher == nil {
		return
	}
	payload := map[string]any{
		"customer_id":    r.CustomerID,
		"endpoint_id":    r.EndpointID,
		"endpoint_label": r.EndpointLabel,
		"download_mbps":  r.DownloadMbps,
		"upload_mbps":    r.UploadMbps,
		"ping_ms":        r.PingMs,
		"jitter_ms":      r.JitterMs,
		"auto_strategy":  r.AutoStrategy,
		"ran_at":         r.RanAt,
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := d.EventPublisher.PublishEvent(context.Background(),
		"plugin.continuum.support."+name, payload); err != nil && d.Logger != nil {
		d.Logger.Warn("speedtest event publish failed", "event", name, "err", err)
	}
}
