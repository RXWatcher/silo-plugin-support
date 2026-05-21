package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// STListEndpoints returns every endpoint, ordered by sort_order then
// label. activeOnly skips soft-deleted rows.
func (s *Store) STListEndpoints(ctx context.Context, activeOnly bool) ([]STEndpoint, error) {
	q := `SELECT id, label, url, country, region, sort_order, active, created_at, updated_at
	      FROM st_endpoints`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(label)`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list st_endpoints: %w", err)
	}
	defer rows.Close()
	out := []STEndpoint{}
	for rows.Next() {
		var e STEndpoint
		if err := rows.Scan(&e.ID, &e.Label, &e.URL, &e.Country, &e.Region,
			&e.SortOrder, &e.Active, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan st_endpoint: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) STGetEndpoint(ctx context.Context, id int64) (STEndpoint, error) {
	var e STEndpoint
	err := s.pool.QueryRow(ctx, `
		SELECT id, label, url, country, region, sort_order, active, created_at, updated_at
		FROM st_endpoints WHERE id = $1`, id).
		Scan(&e.ID, &e.Label, &e.URL, &e.Country, &e.Region,
			&e.SortOrder, &e.Active, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STEndpoint{}, ErrNotFound
	}
	if err != nil {
		return STEndpoint{}, fmt.Errorf("get st_endpoint: %w", err)
	}
	return e, nil
}

func (s *Store) STCreateEndpoint(ctx context.Context, in STEndpoint) (STEndpoint, error) {
	var out STEndpoint
	err := s.pool.QueryRow(ctx, `
		INSERT INTO st_endpoints (label, url, country, region, sort_order, active)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, label, url, country, region, sort_order, active, created_at, updated_at`,
		in.Label, in.URL, in.Country, in.Region, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.URL, &out.Country, &out.Region,
			&out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return STEndpoint{}, fmt.Errorf("insert st_endpoint: %w", err)
	}
	return out, nil
}

func (s *Store) STUpdateEndpoint(ctx context.Context, in STEndpoint) (STEndpoint, error) {
	var out STEndpoint
	err := s.pool.QueryRow(ctx, `
		UPDATE st_endpoints SET
		  label = $2, url = $3, country = $4, region = $5,
		  sort_order = $6, active = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING id, label, url, country, region, sort_order, active, created_at, updated_at`,
		in.ID, in.Label, in.URL, in.Country, in.Region, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.URL, &out.Country, &out.Region,
			&out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STEndpoint{}, ErrNotFound
	}
	if err != nil {
		return STEndpoint{}, fmt.Errorf("update st_endpoint: %w", err)
	}
	return out, nil
}

func (s *Store) STDeleteEndpoint(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM st_endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete st_endpoint: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
