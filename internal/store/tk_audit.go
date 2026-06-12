package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// TKInsertAudit appends an immutable audit record for a support action
// that touches customer PII. detail is optional action-specific context
// (e.g. {"from":"open","to":"resolved"}); nil is stored as an empty
// object. Audit writes are best-effort from the caller's perspective —
// they must never block the underlying action — but the store returns
// any error so callers can log it.
func (s *Store) TKInsertAudit(ctx context.Context, e TKAuditEntry) error {
	role := e.ActorRole
	if role == "" {
		role = "admin"
	}
	detail := e.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO tk_audit_log (ticket_id, actor_id, actor_role, action, detail)
		VALUES ($1, $2, $3, $4, $5)`,
		e.TicketID, e.ActorID, role, e.Action, raw)
	if err != nil {
		return fmt.Errorf("insert tk_audit_log: %w", err)
	}
	return nil
}

// TKListAudit returns the audit trail for a ticket, oldest first.
func (s *Store) TKListAudit(ctx context.Context, ticketID int64) ([]TKAuditEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ticket_id, actor_id, actor_role, action, detail, created_at
		FROM tk_audit_log WHERE ticket_id = $1
		ORDER BY created_at, id`, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list tk_audit_log: %w", err)
	}
	defer rows.Close()
	out := []TKAuditEntry{}
	for rows.Next() {
		var e TKAuditEntry
		var raw []byte
		if err := rows.Scan(&e.ID, &e.TicketID, &e.ActorID, &e.ActorRole, &e.Action, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &e.Detail)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
