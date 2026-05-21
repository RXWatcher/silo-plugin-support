package server

import (
	"context"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// tkPublishEvent assembles the base ticket payload + extra keys and
// hands off to Deps.EventPublisher. No-ops when EventPublisher is nil.
func tkPublishEvent(d Deps, name string, t store.TKTicket, extra map[string]any) {
	if d.EventPublisher == nil {
		return
	}
	payload := map[string]any{
		"ticket_id":         t.ID,
		"tracking_number":   t.TrackingNumber,
		"subject":           t.Subject,
		"status":            t.Status,
		"customer_id":       t.CustomerID,
		"customer_email":    t.CustomerEmail,
		"assigned_admin_id": t.AssignedAdminID,
		"deep_link":         "/tickets/" + t.TrackingNumber,
	}
	if t.Category != nil {
		payload["category"] = map[string]any{
			"id": t.Category.ID, "slug": t.Category.Slug, "name": t.Category.Name,
		}
	}
	if t.Subcategory != nil {
		payload["subcategory"] = map[string]any{
			"id": t.Subcategory.ID, "slug": t.Subcategory.Slug, "name": t.Subcategory.Name,
		}
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := d.EventPublisher.PublishEvent(context.Background(),
		"plugin.continuum.support."+name, payload); err != nil && d.Logger != nil {
		d.Logger.Warn("ticket event publish failed", "event", name, "err", err)
	}
}
