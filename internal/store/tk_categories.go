package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// --- Categories ----------------------------------------------------------

func (s *Store) TKListCategories(ctx context.Context, activeOnly bool) ([]TKCategory, error) {
	q := `SELECT id, slug, name, sort_order, active, created_at, updated_at FROM tk_categories`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(name)`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list tk_categories: %w", err)
	}
	defer rows.Close()
	out := []TKCategory{}
	for rows.Next() {
		var c TKCategory
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) TKGetCategory(ctx context.Context, id int64) (TKCategory, error) {
	var c TKCategory
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, sort_order, active, created_at, updated_at
		FROM tk_categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategory{}, ErrNotFound
	}
	if err != nil {
		return TKCategory{}, fmt.Errorf("get tk_category: %w", err)
	}
	return c, nil
}

func (s *Store) TKCreateCategory(ctx context.Context, in TKCategory) (TKCategory, error) {
	var out TKCategory
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_categories (slug, name, sort_order, active)
		VALUES ($1,$2,$3,$4)
		RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		in.Slug, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return TKCategory{}, fmt.Errorf("insert tk_category: %w", err)
	}
	return out, nil
}

func (s *Store) TKUpdateCategory(ctx context.Context, in TKCategory) (TKCategory, error) {
	var out TKCategory
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_categories SET
		  name = $2, sort_order = $3, active = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		in.ID, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategory{}, ErrNotFound
	}
	if err != nil {
		return TKCategory{}, fmt.Errorf("update tk_category: %w", err)
	}
	return out, nil
}

func (s *Store) TKDeleteCategory(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tk_categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tk_category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TKCategorySlugExists(ctx context.Context, slug string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM tk_categories WHERE slug = $1`, slug).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// --- Subcategories -------------------------------------------------------

func (s *Store) TKListSubcategories(ctx context.Context, categoryID int64, activeOnly bool) ([]TKSubcategory, error) {
	q := `SELECT id, category_id, slug, name, sort_order, active, created_at, updated_at
	      FROM tk_subcategories WHERE category_id = $1`
	if activeOnly {
		q += ` AND active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(name)`
	rows, err := s.pool.Query(ctx, q, categoryID)
	if err != nil {
		return nil, fmt.Errorf("list tk_subcategories: %w", err)
	}
	defer rows.Close()
	out := []TKSubcategory{}
	for rows.Next() {
		var x TKSubcategory
		if err := rows.Scan(&x.ID, &x.CategoryID, &x.Slug, &x.Name, &x.SortOrder, &x.Active, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) TKGetSubcategory(ctx context.Context, id int64) (TKSubcategory, error) {
	var x TKSubcategory
	err := s.pool.QueryRow(ctx, `
		SELECT id, category_id, slug, name, sort_order, active, created_at, updated_at
		FROM tk_subcategories WHERE id = $1`, id).
		Scan(&x.ID, &x.CategoryID, &x.Slug, &x.Name, &x.SortOrder, &x.Active, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKSubcategory{}, ErrNotFound
	}
	if err != nil {
		return TKSubcategory{}, fmt.Errorf("get tk_subcategory: %w", err)
	}
	return x, nil
}

func (s *Store) TKCreateSubcategory(ctx context.Context, in TKSubcategory) (TKSubcategory, error) {
	var out TKSubcategory
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_subcategories (category_id, slug, name, sort_order, active)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, category_id, slug, name, sort_order, active, created_at, updated_at`,
		in.CategoryID, in.Slug, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.CategoryID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return TKSubcategory{}, fmt.Errorf("insert tk_subcategory: %w", err)
	}
	return out, nil
}

func (s *Store) TKUpdateSubcategory(ctx context.Context, in TKSubcategory) (TKSubcategory, error) {
	var out TKSubcategory
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_subcategories SET
		  name = $2, sort_order = $3, active = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, category_id, slug, name, sort_order, active, created_at, updated_at`,
		in.ID, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.CategoryID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKSubcategory{}, ErrNotFound
	}
	if err != nil {
		return TKSubcategory{}, fmt.Errorf("update tk_subcategory: %w", err)
	}
	return out, nil
}

func (s *Store) TKDeleteSubcategory(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tk_subcategories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tk_subcategory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Category fields -----------------------------------------------------

func (s *Store) TKListCategoryFields(ctx context.Context, categoryID int64) ([]TKCategoryField, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, category_id, key, label, kind, required, sort_order
		FROM tk_category_fields WHERE category_id = $1
		ORDER BY sort_order, key`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("list tk_category_fields: %w", err)
	}
	defer rows.Close()
	out := []TKCategoryField{}
	for rows.Next() {
		var f TKCategoryField
		if err := rows.Scan(&f.ID, &f.CategoryID, &f.Key, &f.Label, &f.Kind, &f.Required, &f.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) TKGetCategoryField(ctx context.Context, id int64) (TKCategoryField, error) {
	var f TKCategoryField
	err := s.pool.QueryRow(ctx, `
		SELECT id, category_id, key, label, kind, required, sort_order
		FROM tk_category_fields WHERE id = $1`, id).
		Scan(&f.ID, &f.CategoryID, &f.Key, &f.Label, &f.Kind, &f.Required, &f.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategoryField{}, ErrNotFound
	}
	return f, err
}

func (s *Store) TKCreateCategoryField(ctx context.Context, in TKCategoryField) (TKCategoryField, error) {
	var out TKCategoryField
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_category_fields (category_id, key, label, kind, required, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, category_id, key, label, kind, required, sort_order`,
		in.CategoryID, in.Key, in.Label, in.Kind, in.Required, in.SortOrder).
		Scan(&out.ID, &out.CategoryID, &out.Key, &out.Label, &out.Kind, &out.Required, &out.SortOrder)
	if err != nil {
		return TKCategoryField{}, fmt.Errorf("insert tk_category_field: %w", err)
	}
	return out, nil
}

func (s *Store) TKUpdateCategoryField(ctx context.Context, in TKCategoryField) (TKCategoryField, error) {
	var out TKCategoryField
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_category_fields SET
		  label = $2, kind = $3, required = $4, sort_order = $5
		WHERE id = $1
		RETURNING id, category_id, key, label, kind, required, sort_order`,
		in.ID, in.Label, in.Kind, in.Required, in.SortOrder).
		Scan(&out.ID, &out.CategoryID, &out.Key, &out.Label, &out.Kind, &out.Required, &out.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategoryField{}, ErrNotFound
	}
	return out, err
}

func (s *Store) TKDeleteCategoryField(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tk_category_fields WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tk_category_field: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
