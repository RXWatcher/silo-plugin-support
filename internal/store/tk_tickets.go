package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TKNextTrackingNumber atomically increments the sequence row and
// returns the new tracking number in the SUP-N format.
func (s *Store) TKNextTrackingNumber(ctx context.Context) (string, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_ticket_sequence SET next_n = next_n + 1
		WHERE id = 1
		RETURNING next_n - 1`).Scan(&n)
	if err != nil {
		return "", fmt.Errorf("tk next tracking number: %w", err)
	}
	return fmt.Sprintf("SUP-%d", n), nil
}

func (s *Store) TKCreateTicket(ctx context.Context, tx pgx.Tx, in TKTicket) (TKTicket, error) {
	var out TKTicket
	err := tx.QueryRow(ctx, `
		INSERT INTO tk_tickets (
		  tracking_number, customer_id, customer_email,
		  category_id, subcategory_id, subject, status)
		VALUES ($1,$2,$3,$4,$5,$6,'open')
		RETURNING id, tracking_number, customer_id, customer_email,
		          category_id, subcategory_id, subject, status,
		          assigned_admin_id, created_at, updated_at,
		          waiting_since, resolved_at`,
		in.TrackingNumber, in.CustomerID, in.CustomerEmail,
		in.CategoryID, in.SubcategoryID, in.Subject).
		Scan(&out.ID, &out.TrackingNumber, &out.CustomerID, &out.CustomerEmail,
			&out.CategoryID, &out.SubcategoryID, &out.Subject, &out.Status,
			&out.AssignedAdminID, &out.CreatedAt, &out.UpdatedAt,
			&out.WaitingSince, &out.ResolvedAt)
	if err != nil {
		return TKTicket{}, fmt.Errorf("insert tk_ticket: %w", err)
	}
	return out, nil
}

func (s *Store) TKGetTicketByTracking(ctx context.Context, tn string) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		SELECT id, tracking_number, customer_id, customer_email,
		       category_id, subcategory_id, subject, status,
		       assigned_admin_id, created_at, updated_at,
		       waiting_since, resolved_at
		FROM tk_tickets WHERE tracking_number = $1`, tn).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	if err != nil {
		return TKTicket{}, fmt.Errorf("get tk_ticket: %w", err)
	}
	return t, nil
}

func (s *Store) TKGetTicketByID(ctx context.Context, id int64) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		SELECT id, tracking_number, customer_id, customer_email,
		       category_id, subcategory_id, subject, status,
		       assigned_admin_id, created_at, updated_at,
		       waiting_since, resolved_at
		FROM tk_tickets WHERE id = $1`, id).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	return t, err
}

// TKCountOpenTicketsForCustomer returns how many non-terminal
// (open / in_progress / waiting_customer) tickets a customer currently
// has. Used to cap how many concurrent tickets one customer can open.
func (s *Store) TKCountOpenTicketsForCustomer(ctx context.Context, customerID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tk_tickets
		WHERE customer_id = $1
		  AND status IN ('open','in_progress','waiting_customer')`, customerID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count open tickets: %w", err)
	}
	return n, nil
}

func (s *Store) TKListTickets(ctx context.Context, f TKTicketListFilter) ([]TKTicket, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	args := []any{}
	clauses := []string{}

	if f.CustomerID != "" {
		args = append(args, f.CustomerID)
		clauses = append(clauses, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	switch f.StatusGroup {
	case "active":
		clauses = append(clauses, "status IN ('open','in_progress','waiting_customer')")
	case "closed":
		clauses = append(clauses, "status = 'closed'")
	}
	if f.CategoryID > 0 {
		args = append(args, f.CategoryID)
		clauses = append(clauses, fmt.Sprintf("category_id = $%d", len(args)))
	}
	switch f.AssigneeID {
	case "__mine__":
		args = append(args, f.CallerAdminID)
		clauses = append(clauses, fmt.Sprintf("assigned_admin_id = $%d", len(args)))
	case "__unassigned__":
		clauses = append(clauses, "assigned_admin_id IS NULL")
	default:
		if f.AssigneeID != "" {
			args = append(args, f.AssigneeID)
			clauses = append(clauses, fmt.Sprintf("assigned_admin_id = $%d", len(args)))
		}
	}
	if f.Search != "" {
		args = append(args, f.Search+"%")
		args = append(args, "%"+f.Search+"%")
		clauses = append(clauses,
			fmt.Sprintf("(tracking_number ILIKE $%d OR subject ILIKE $%d)", len(args)-1, len(args)))
	}

	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`
		SELECT id, tracking_number, customer_id, customer_email,
		       category_id, subcategory_id, subject, status,
		       assigned_admin_id, created_at, updated_at,
		       waiting_since, resolved_at
		FROM tk_tickets
		%s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tk_tickets: %w", err)
	}
	defer rows.Close()
	out := []TKTicket{}
	for rows.Next() {
		var t TKTicket
		if err := rows.Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TKUpdateTicketStatus persists the lifecycle transition + side
// effects in a single UPDATE.
func (s *Store) TKUpdateTicketStatus(ctx context.Context, id int64, newStatus string, _, _ *interface{}) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_tickets SET
		  status        = $2,
		  waiting_since = CASE WHEN $2 = 'waiting_customer' THEN NOW()
		                       WHEN $2 IN ('in_progress','open') THEN NULL
		                       ELSE waiting_since END,
		  resolved_at   = CASE WHEN $2 = 'resolved' THEN NOW()
		                       WHEN $2 = 'in_progress' THEN NULL
		                       ELSE resolved_at END,
		  updated_at    = NOW()
		WHERE id = $1
		RETURNING id, tracking_number, customer_id, customer_email,
		          category_id, subcategory_id, subject, status,
		          assigned_admin_id, created_at, updated_at,
		          waiting_since, resolved_at`, id, newStatus).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	if err != nil {
		return TKTicket{}, fmt.Errorf("update tk_ticket status: %w", err)
	}
	return t, nil
}

func (s *Store) TKAssignTicket(ctx context.Context, id int64, adminID *string) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_tickets SET assigned_admin_id = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, tracking_number, customer_id, customer_email,
		          category_id, subcategory_id, subject, status,
		          assigned_admin_id, created_at, updated_at,
		          waiting_since, resolved_at`, id, adminID).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	return t, err
}

func (s *Store) TKTouchTicket(ctx context.Context, tx pgx.Tx, id int64) error {
	_, err := tx.Exec(ctx, `UPDATE tk_tickets SET updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *Store) TKLoadTicketAux(ctx context.Context, t *TKTicket) error {
	cat, err := s.TKGetCategory(ctx, t.CategoryID)
	if err == nil {
		t.Category = &cat
	}
	if t.SubcategoryID != nil {
		sub, err := s.TKGetSubcategory(ctx, *t.SubcategoryID)
		if err == nil {
			t.Subcategory = &sub
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT f.id, f.key, f.label, fv.value
		FROM tk_ticket_field_values fv
		JOIN tk_category_fields f ON f.id = fv.field_id
		WHERE fv.ticket_id = $1
		ORDER BY f.sort_order`, t.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	t.FieldValues = []TKFieldValue{}
	for rows.Next() {
		var v TKFieldValue
		if err := rows.Scan(&v.FieldID, &v.FieldKey, &v.FieldLabel, &v.Value); err != nil {
			return err
		}
		t.FieldValues = append(t.FieldValues, v)
	}
	return rows.Err()
}

func (s *Store) TKInsertFieldValue(ctx context.Context, tx pgx.Tx, ticketID, fieldID int64, value string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO tk_ticket_field_values (ticket_id, field_id, value)
		VALUES ($1, $2, $3)`, ticketID, fieldID, value)
	return err
}

func (s *Store) TKBegin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

func (s *Store) TKResolvedAtIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM tk_tickets
		WHERE status = 'resolved'
		  AND resolved_at < NOW() - ($1 || ' days')::interval
		ORDER BY resolved_at
		LIMIT $2`, fmt.Sprintf("%d", cutoffDays), limit)
	if err != nil {
		return nil, fmt.Errorf("tk resolved idle: %w", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) TKWaitingIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM tk_tickets
		WHERE status = 'waiting_customer'
		  AND waiting_since < NOW() - ($1 || ' days')::interval
		ORDER BY waiting_since
		LIMIT $2`, fmt.Sprintf("%d", cutoffDays), limit)
	if err != nil {
		return nil, fmt.Errorf("tk waiting idle: %w", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
