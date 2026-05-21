package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a row lookup misses. Distinct from
// pgx.ErrNoRows so callers don't depend on the driver layer.
var ErrNotFound = errors.New("not found")

// KBListCategories returns every category, ordered by sort_order
// then name. activeOnly skips soft-deleted rows.
func (s *Store) KBListCategories(ctx context.Context, activeOnly bool) ([]KBCategory, error) {
	q := `SELECT id, slug, name, sort_order, active, created_at, updated_at
	      FROM kb_categories`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(name)`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list kb_categories: %w", err)
	}
	defer rows.Close()
	out := []KBCategory{}
	for rows.Next() {
		var c KBCategory
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan kb_category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// KBGetCategory by id. Returns ErrNotFound if absent.
func (s *Store) KBGetCategory(ctx context.Context, id int64) (KBCategory, error) {
	var c KBCategory
	err := s.pool.QueryRow(ctx,
		`SELECT id, slug, name, sort_order, active, created_at, updated_at
		 FROM kb_categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBCategory{}, ErrNotFound
	}
	if err != nil {
		return KBCategory{}, fmt.Errorf("get kb_category: %w", err)
	}
	return c, nil
}

// KBCreateCategory inserts a new row. Caller has already computed
// the slug + ensured it's free (the handler runs the
// slug-collision suffix loop).
func (s *Store) KBCreateCategory(ctx context.Context, slug, name string, sortOrder int) (KBCategory, error) {
	var c KBCategory
	err := s.pool.QueryRow(ctx,
		`INSERT INTO kb_categories (slug, name, sort_order)
		 VALUES ($1, $2, $3)
		 RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		slug, name, sortOrder).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return KBCategory{}, fmt.Errorf("insert kb_category: %w", err)
	}
	return c, nil
}

// KBUpdateCategory: rename, reorder, toggle active. Only the
// supplied fields are written; the handler passes through whatever
// the operator changed.
func (s *Store) KBUpdateCategory(ctx context.Context, id int64, name string, sortOrder int, active bool) (KBCategory, error) {
	var c KBCategory
	err := s.pool.QueryRow(ctx,
		`UPDATE kb_categories
		 SET name = $1, sort_order = $2, active = $3, updated_at = NOW()
		 WHERE id = $4
		 RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		name, sortOrder, active, id).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBCategory{}, ErrNotFound
	}
	if err != nil {
		return KBCategory{}, fmt.Errorf("update kb_category: %w", err)
	}
	return c, nil
}

// KBDeleteCategory hard-deletes. Returns the ON-DELETE-RESTRICT
// foreign key violation as a plain error the handler maps to 409.
func (s *Store) KBDeleteCategory(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete kb_category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// KBCategorySlugExists reports whether a slug is already taken.
// Used by the handler's slug-collision suffixing loop.
func (s *Store) KBCategorySlugExists(ctx context.Context, slug string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM kb_categories WHERE slug = $1`, slug).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check kb_category slug: %w", err)
	}
	return true, nil
}
