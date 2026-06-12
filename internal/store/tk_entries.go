package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TKInsertEntry writes an append-only entry on a ticket. Used inside
// transactions (initial entry + ticket create) and outside (admin
// replies, internal notes, status_change auto-entries).
func (s *Store) TKInsertEntry(ctx context.Context, tx pgx.Tx, e TKEntry) (TKEntry, error) {
	var out TKEntry
	err := tx.QueryRow(ctx, `
		INSERT INTO tk_ticket_entries (ticket_id, kind, author_id, author_role, body)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, ticket_id, kind, author_id, author_role, body, created_at`,
		e.TicketID, e.Kind, e.AuthorID, e.AuthorRole, e.Body).
		Scan(&out.ID, &out.TicketID, &out.Kind, &out.AuthorID, &out.AuthorRole, &out.Body, &out.CreatedAt)
	if err != nil {
		return TKEntry{}, fmt.Errorf("insert tk_ticket_entry: %w", err)
	}
	return out, nil
}

// TKInsertEntryNoTx wraps an entry insert in its own transaction.
func (s *Store) TKInsertEntryNoTx(ctx context.Context, e TKEntry) (TKEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TKEntry{}, err
	}
	defer tx.Rollback(ctx)
	saved, err := s.TKInsertEntry(ctx, tx, e)
	if err != nil {
		return TKEntry{}, err
	}
	if err := s.TKTouchTicket(ctx, tx, e.TicketID); err != nil {
		return TKEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TKEntry{}, err
	}
	return saved, nil
}

// TKListEntries returns ticket entries oldest-first. customerView
// filters out internal_note rows for the customer detail endpoint.
func (s *Store) TKListEntries(ctx context.Context, ticketID int64, customerView bool) ([]TKEntry, error) {
	q := `SELECT id, ticket_id, kind, author_id, author_role, body, created_at
	      FROM tk_ticket_entries WHERE ticket_id = $1`
	if customerView {
		q += ` AND kind != 'internal_note'`
	}
	q += ` ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list tk_ticket_entries: %w", err)
	}
	defer rows.Close()
	out := []TKEntry{}
	for rows.Next() {
		var e TKEntry
		if err := rows.Scan(&e.ID, &e.TicketID, &e.Kind, &e.AuthorID, &e.AuthorRole, &e.Body, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		atts, err := s.TKListEntryAttachments(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Attachments = atts
	}
	return out, nil
}

// TKLastCustomerActionAt returns the most recent time this customer
// created a ticket or authored a ticket entry, or zero time if never.
// Used by the per-customer create/reply rate limit.
func (s *Store) TKLastCustomerActionAt(ctx context.Context, customerID string) (time.Time, error) {
	var t *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT MAX(at) FROM (
			SELECT created_at AS at FROM tk_tickets WHERE customer_id = $1
			UNION ALL
			SELECT created_at AS at FROM tk_ticket_entries
			WHERE author_role = 'customer' AND author_id = $1
		) actions`, customerID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) || t == nil {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("tk last customer action at: %w", err)
	}
	return *t, nil
}

func (s *Store) TKGetEntry(ctx context.Context, id int64) (TKEntry, error) {
	var e TKEntry
	err := s.pool.QueryRow(ctx, `
		SELECT id, ticket_id, kind, author_id, author_role, body, created_at
		FROM tk_ticket_entries WHERE id = $1`, id).
		Scan(&e.ID, &e.TicketID, &e.Kind, &e.AuthorID, &e.AuthorRole, &e.Body, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKEntry{}, ErrNotFound
	}
	return e, err
}
