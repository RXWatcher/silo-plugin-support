package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// KBListTags returns every tag, ordered by label, plus a usage
// count via a LEFT JOIN onto kb_article_tags. The handler exposes
// the count to the admin tags admin page.
func (s *Store) KBListTags(ctx context.Context) ([]KBTagWithCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.slug, t.label, t.created_at,
		       COUNT(at.article_id) AS use_count
		FROM kb_tags t
		LEFT JOIN kb_article_tags at ON at.tag_id = t.id
		GROUP BY t.id
		ORDER BY lower(t.label)`)
	if err != nil {
		return nil, fmt.Errorf("list kb_tags: %w", err)
	}
	defer rows.Close()
	out := []KBTagWithCount{}
	for rows.Next() {
		var t KBTagWithCount
		if err := rows.Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt, &t.UseCount); err != nil {
			return nil, fmt.Errorf("scan kb_tag: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// KBGetTagBySlug finds a tag by its slug; returns ErrNotFound if absent.
func (s *Store) KBGetTagBySlug(ctx context.Context, slug string) (KBTag, error) {
	var t KBTag
	err := s.pool.QueryRow(ctx,
		`SELECT id, slug, label, created_at FROM kb_tags WHERE slug = $1`, slug).
		Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBTag{}, ErrNotFound
	}
	if err != nil {
		return KBTag{}, fmt.Errorf("get kb_tag by slug: %w", err)
	}
	return t, nil
}

// KBCreateTag inserts. Caller has already computed the slug + ensured
// it's free.
func (s *Store) KBCreateTag(ctx context.Context, slug, label string) (KBTag, error) {
	var t KBTag
	err := s.pool.QueryRow(ctx,
		`INSERT INTO kb_tags (slug, label) VALUES ($1, $2)
		 RETURNING id, slug, label, created_at`, slug, label).
		Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt)
	if err != nil {
		return KBTag{}, fmt.Errorf("insert kb_tag: %w", err)
	}
	return t, nil
}

// KBRenameTag updates label only (slug is stable to keep article-tag
// references intact across renames).
func (s *Store) KBRenameTag(ctx context.Context, id int64, label string) (KBTag, error) {
	var t KBTag
	err := s.pool.QueryRow(ctx,
		`UPDATE kb_tags SET label = $1 WHERE id = $2
		 RETURNING id, slug, label, created_at`, label, id).
		Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBTag{}, ErrNotFound
	}
	if err != nil {
		return KBTag{}, fmt.Errorf("rename kb_tag: %w", err)
	}
	return t, nil
}

// KBDeleteTag hard-deletes. Returns ErrNotFound if absent. The
// kb_article_tags FK uses ON DELETE RESTRICT, so this fails with a
// Postgres FK violation when the tag is still in use — the handler
// maps that to 409.
func (s *Store) KBDeleteTag(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete kb_tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// KBMergeTags moves every kb_article_tags row from `from` into `into`,
// then deletes `from`. Runs in a transaction. If an article already
// has both, the duplicate `from` row is dropped silently (so the
// merge is idempotent against partial pre-merge state).
func (s *Store) KBMergeTags(ctx context.Context, from, into int64) error {
	if from == into {
		return fmt.Errorf("merge kb_tags: from == into")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("merge kb_tags begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO kb_article_tags (article_id, tag_id)
		 SELECT article_id, $2 FROM kb_article_tags WHERE tag_id = $1
		 ON CONFLICT DO NOTHING`, from, into); err != nil {
		return fmt.Errorf("merge kb_tags reassign: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM kb_article_tags WHERE tag_id = $1`, from); err != nil {
		return fmt.Errorf("merge kb_tags clear: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM kb_tags WHERE id = $1`, from); err != nil {
		return fmt.Errorf("merge kb_tags delete: %w", err)
	}
	return tx.Commit(ctx)
}

// KBTagWithCount augments KBTag with a usage count for the admin list.
type KBTagWithCount struct {
	KBTag
	UseCount int `json:"useCount"`
}
