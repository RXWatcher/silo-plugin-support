package server

import (
	"context"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

// kbPublishEvent assembles the base payload + extra keys and hands
// off to Deps.EventPublisher. No-ops when EventPublisher is nil
// (test contexts). Best-effort: a publish failure is logged but
// never propagated to the HTTP response.
func kbPublishEvent(d Deps, name string, a store.KBArticle, extra map[string]any) {
	if d.EventPublisher == nil {
		return
	}
	payload := map[string]any{
		"article_id":    a.ID,
		"slug":          a.Slug,
		"title":         a.Title,
		"category_slug": "",
		"category_name": "",
		"tags":          kbTagSlugs(a.Tags),
		"deep_link":     "/kb/" + a.Slug,
	}
	if a.Category != nil {
		payload["category_slug"] = a.Category.Slug
		payload["category_name"] = a.Category.Name
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := d.EventPublisher.PublishEvent(context.Background(),
		"plugin.silo.support."+name, payload); err != nil && d.Logger != nil {
		d.Logger.Warn("kb event publish failed", "event", name, "err", err)
	}
}

func kbTagSlugs(tags []store.KBTag) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, t.Slug)
	}
	return out
}
