package tickets

import (
	"context"
	"time"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

// CronStore is the slice of *store.Store the cron needs.
type CronStore interface {
	TKResolvedAtIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error)
	TKWaitingIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error)
	TKGetTicketByID(ctx context.Context, id int64) (store.TKTicket, error)
	TKUpdateTicketStatus(ctx context.Context, id int64, newStatus string, waitingSince, resolvedAt *interface{}) (store.TKTicket, error)
	TKInsertEntryNoTx(ctx context.Context, e store.TKEntry) (store.TKEntry, error)
}

// Emitter mirrors the server's tk event publisher.
type Emitter interface {
	PublishTicketEvent(ctx context.Context, name string, t store.TKTicket, extra map[string]any)
}

// Cron bundles the per-day auto-close behaviour.
type Cron struct {
	Store             CronStore
	Emitter           Emitter
	Enabled           bool
	ResolvedAfterDays int
	WaitingAfterDays  int
}

// CloseIdle runs both passes. No-op when Enabled=false; per-pass day=0 skips that pass.
func (c *Cron) CloseIdle(ctx context.Context) error {
	if !c.Enabled {
		return nil
	}
	if c.ResolvedAfterDays > 0 {
		ids, err := c.Store.TKResolvedAtIdleSince(ctx, c.ResolvedAfterDays, 200)
		if err != nil {
			return err
		}
		for _, id := range ids {
			c.closeOne(ctx, id, "resolved_idle")
		}
	}
	if c.WaitingAfterDays > 0 {
		ids, err := c.Store.TKWaitingIdleSince(ctx, c.WaitingAfterDays, 200)
		if err != nil {
			return err
		}
		for _, id := range ids {
			c.closeOne(ctx, id, "waiting_idle")
		}
	}
	return nil
}

func (c *Cron) closeOne(ctx context.Context, id int64, reason string) {
	updated, err := c.Store.TKUpdateTicketStatus(ctx, id, "closed", nil, nil)
	if err != nil {
		return
	}
	_, _ = c.Store.TKInsertEntryNoTx(ctx, store.TKEntry{
		TicketID: id, Kind: "system", AuthorID: "system", AuthorRole: "system",
		Body: "Ticket auto-closed by cron (" + reason + ")",
	})
	c.Emitter.PublishTicketEvent(ctx, "ticket_closed", updated, map[string]any{
		"by":     "system",
		"reason": reason,
	})
}

var _ = time.Time{}
