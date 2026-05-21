package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (s *Store) STListGeoIPSources(ctx context.Context, activeOnly bool) ([]STGeoIPSource, error) {
	q := `SELECT id, label, kind, config, sort_order, active,
	             last_status, last_used_at, last_refreshed_at,
	             created_at, updated_at
	      FROM st_geoip_sources`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, id`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list st_geoip_sources: %w", err)
	}
	defer rows.Close()
	out := []STGeoIPSource{}
	for rows.Next() {
		var src STGeoIPSource
		var cfg []byte
		if err := rows.Scan(&src.ID, &src.Label, &src.Kind, &cfg,
			&src.SortOrder, &src.Active, &src.LastStatus,
			&src.LastUsedAt, &src.LastRefreshedAt,
			&src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan st_geoip_source: %w", err)
		}
		src.Config = json.RawMessage(cfg)
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) STGetGeoIPSource(ctx context.Context, id int64) (STGeoIPSource, error) {
	var src STGeoIPSource
	var cfg []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, label, kind, config, sort_order, active,
		       last_status, last_used_at, last_refreshed_at,
		       created_at, updated_at
		FROM st_geoip_sources WHERE id = $1`, id).
		Scan(&src.ID, &src.Label, &src.Kind, &cfg,
			&src.SortOrder, &src.Active, &src.LastStatus,
			&src.LastUsedAt, &src.LastRefreshedAt,
			&src.CreatedAt, &src.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STGeoIPSource{}, ErrNotFound
	}
	if err != nil {
		return STGeoIPSource{}, fmt.Errorf("get st_geoip_source: %w", err)
	}
	src.Config = json.RawMessage(cfg)
	return src, nil
}

func (s *Store) STCreateGeoIPSource(ctx context.Context, in STGeoIPSource) (STGeoIPSource, error) {
	var out STGeoIPSource
	var cfg []byte
	err := s.pool.QueryRow(ctx, `
		INSERT INTO st_geoip_sources (label, kind, config, sort_order, active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, label, kind, config, sort_order, active,
		          last_status, last_used_at, last_refreshed_at,
		          created_at, updated_at`,
		in.Label, in.Kind, []byte(in.Config), in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.Kind, &cfg,
			&out.SortOrder, &out.Active, &out.LastStatus,
			&out.LastUsedAt, &out.LastRefreshedAt,
			&out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return STGeoIPSource{}, fmt.Errorf("insert st_geoip_source: %w", err)
	}
	out.Config = json.RawMessage(cfg)
	return out, nil
}

func (s *Store) STUpdateGeoIPSource(ctx context.Context, in STGeoIPSource) (STGeoIPSource, error) {
	var out STGeoIPSource
	var cfg []byte
	err := s.pool.QueryRow(ctx, `
		UPDATE st_geoip_sources SET
		  label = $2, config = $3, sort_order = $4, active = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING id, label, kind, config, sort_order, active,
		          last_status, last_used_at, last_refreshed_at,
		          created_at, updated_at`,
		in.ID, in.Label, []byte(in.Config), in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.Kind, &cfg,
			&out.SortOrder, &out.Active, &out.LastStatus,
			&out.LastUsedAt, &out.LastRefreshedAt,
			&out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STGeoIPSource{}, ErrNotFound
	}
	if err != nil {
		return STGeoIPSource{}, fmt.Errorf("update st_geoip_source: %w", err)
	}
	out.Config = json.RawMessage(cfg)
	return out, nil
}

func (s *Store) STDeleteGeoIPSource(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM st_geoip_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete st_geoip_source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) STMarkGeoIPSourceUsed(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE st_geoip_sources SET last_used_at = NOW(), last_status = 'ok' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark geoip source used: %w", err)
	}
	return nil
}

func (s *Store) STMarkGeoIPSourceStatus(ctx context.Context, id int64, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE st_geoip_sources SET last_status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("mark geoip source status: %w", err)
	}
	return nil
}

func (s *Store) STMarkGeoIPSourceRefreshed(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE st_geoip_sources SET last_refreshed_at = NOW(), last_status = 'ok' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark geoip source refreshed: %w", err)
	}
	return nil
}
