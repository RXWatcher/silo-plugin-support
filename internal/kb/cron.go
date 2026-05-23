package kb

import (
	"context"
	"time"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

// CronStore is the subset of store.Store the cron needs. Captured
// as an interface so the cron is unit-testable with a fake.
type CronStore interface {
	KBPendingPublishes(ctx context.Context, now time.Time, limit int) ([]int64, error)
	KBPublishArticle(ctx context.Context, id int64, by string) (store.KBArticle, error)
	KBPublishedArticleIDs(ctx context.Context) ([]int64, error)
	KBVoteWindow24h(ctx context.Context, id int64) (store.KBVoteWindow, error)
	KBGetArticleByID(ctx context.Context, id int64) (store.KBArticle, error)
}

// EventEmitter is the subset of the server's event publisher the cron
// needs. Keeps cron decoupled from the server package.
type EventEmitter interface {
	PublishKBArticleEvent(ctx context.Context, name string, a store.KBArticle, extra map[string]any)
}

// Cron bundles the daily-run KB tasks.
type Cron struct {
	Store              CronStore
	Publisher          EventEmitter
	UnhelpfulThreshold float64 // helpful_ratio below this triggers an event (default 0.5)
	UnhelpfulMinVotes  int     // require at least N votes in the 24h window (default 5)
}

// PublishDue flips every draft article whose publish_at has elapsed
// to 'published' and emits kb_article_published.
func (c *Cron) PublishDue(ctx context.Context) error {
	ids, err := c.Store.KBPendingPublishes(ctx, time.Now(), 100)
	if err != nil {
		return err
	}
	for _, id := range ids {
		a, err := c.Store.KBPublishArticle(ctx, id, "system")
		if err != nil {
			continue
		}
		c.Publisher.PublishKBArticleEvent(ctx, "kb_article_published", a, map[string]any{
			"by": "system",
		})
	}
	return nil
}

// UnhelpfulSweep emits kb_article_unhelpful for any published article
// whose last-24h helpful ratio is below threshold and that has at
// least MinVotes votes in the window.
func (c *Cron) UnhelpfulSweep(ctx context.Context) error {
	if c.UnhelpfulThreshold == 0 {
		c.UnhelpfulThreshold = 0.5
	}
	if c.UnhelpfulMinVotes == 0 {
		c.UnhelpfulMinVotes = 5
	}
	ids, err := c.Store.KBPublishedArticleIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		win, err := c.Store.KBVoteWindow24h(ctx, id)
		if err != nil {
			continue
		}
		total := win.Helpful + win.NotHelpful
		if total < c.UnhelpfulMinVotes {
			continue
		}
		if win.HelpfulRatio >= c.UnhelpfulThreshold {
			continue
		}
		a, err := c.Store.KBGetArticleByID(ctx, id)
		if err != nil {
			continue
		}
		c.Publisher.PublishKBArticleEvent(ctx, "kb_article_unhelpful", a, map[string]any{
			"helpful_ratio_24h": win.HelpfulRatio,
			"threshold":         c.UnhelpfulThreshold,
			"votes_24h":         total,
		})
	}
	return nil
}
