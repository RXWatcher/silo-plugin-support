package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (s *Store) TKInsertAttachment(ctx context.Context, a TKAttachment) (TKAttachmentMeta, error) {
	var out TKAttachmentMeta
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_attachments (entry_id, filename, mime, bytes, content_bytea, sha256)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, filename, mime, bytes, created_at`,
		a.EntryID, a.Filename, a.MIME, a.Bytes, a.Content, a.SHA256).
		Scan(&out.ID, &out.Filename, &out.MIME, &out.Bytes, &out.CreatedAt)
	if err != nil {
		return TKAttachmentMeta{}, fmt.Errorf("insert tk_attachment: %w", err)
	}
	return out, nil
}

func (s *Store) TKGetAttachment(ctx context.Context, id int64) (TKAttachment, error) {
	var a TKAttachment
	err := s.pool.QueryRow(ctx, `
		SELECT id, entry_id, filename, mime, bytes, content_bytea, sha256, created_at
		FROM tk_attachments WHERE id = $1`, id).
		Scan(&a.ID, &a.EntryID, &a.Filename, &a.MIME, &a.Bytes, &a.Content, &a.SHA256, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKAttachment{}, ErrNotFound
	}
	return a, err
}

// TKCustomerAttachmentBytes returns the total number of bytes this
// customer is currently storing across attachments on all of their
// tickets. Used to enforce a per-customer storage quota.
func (s *Store) TKCustomerAttachmentBytes(ctx context.Context, customerID string) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(a.bytes), 0)
		FROM tk_attachments a
		JOIN tk_ticket_entries e ON e.id = a.entry_id
		JOIN tk_tickets t       ON t.id = e.ticket_id
		WHERE t.customer_id = $1`, customerID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum customer attachment bytes: %w", err)
	}
	return total, nil
}

// TKTicketAttachmentCount returns how many attachments exist across all
// entries of a ticket. Used to cap attachments per ticket.
func (s *Store) TKTicketAttachmentCount(ctx context.Context, ticketID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM tk_attachments a
		JOIN tk_ticket_entries e ON e.id = a.entry_id
		WHERE e.ticket_id = $1`, ticketID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count ticket attachments: %w", err)
	}
	return n, nil
}

func (s *Store) TKListEntryAttachments(ctx context.Context, entryID int64) ([]TKAttachmentMeta, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, filename, mime, bytes, created_at
		FROM tk_attachments WHERE entry_id = $1
		ORDER BY id`, entryID)
	if err != nil {
		return nil, fmt.Errorf("list entry attachments: %w", err)
	}
	defer rows.Close()
	out := []TKAttachmentMeta{}
	for rows.Next() {
		var m TKAttachmentMeta
		if err := rows.Scan(&m.ID, &m.Filename, &m.MIME, &m.Bytes, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

var _ = pgx.ErrNoRows
