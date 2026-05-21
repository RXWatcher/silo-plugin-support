package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// KBArticleListFilter narrows what KBListArticles returns. Zero
// values are wildcards.
type KBArticleListFilter struct {
	Status      string  // "draft" | "published" | "" (all)
	CategoryID  int64   // 0 = any
	TagSlug     string  // "" = any
	TitleQuery  string  // "" = no title filter (LIKE)
	Limit       int     // 0 = 100
	Offset      int
}

// KBListArticles returns paginated summaries matching the filter.
func (s *Store) KBListArticles(ctx context.Context, f KBArticleListFilter) ([]KBArticleSummary, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	args := []any{}
	where := "WHERE 1=1"
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND a.status = $%d", len(args))
	}
	if f.CategoryID > 0 {
		args = append(args, f.CategoryID)
		where += fmt.Sprintf(" AND a.category_id = $%d", len(args))
	}
	if f.TitleQuery != "" {
		args = append(args, "%"+f.TitleQuery+"%")
		where += fmt.Sprintf(" AND a.title ILIKE $%d", len(args))
	}
	if f.TagSlug != "" {
		args = append(args, f.TagSlug)
		where += fmt.Sprintf(`
		 AND EXISTS (
		   SELECT 1 FROM kb_article_tags at
		   JOIN kb_tags t ON t.id = at.tag_id
		   WHERE at.article_id = a.id AND t.slug = $%d)`, len(args))
	}
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`
		SELECT a.id, a.slug, a.title, a.summary, a.category_id,
		       c.name AS category_name, a.status, a.published_at, a.updated_at,
		       COALESCE(
		         (SELECT array_agg(t.slug ORDER BY t.label)
		          FROM kb_article_tags at JOIN kb_tags t ON t.id = at.tag_id
		          WHERE at.article_id = a.id),
		         ARRAY[]::text[]) AS tag_slugs
		FROM kb_articles a
		JOIN kb_categories c ON c.id = a.category_id
		%s
		ORDER BY a.updated_at DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list kb_articles: %w", err)
	}
	defer rows.Close()
	out := []KBArticleSummary{}
	for rows.Next() {
		var sum KBArticleSummary
		if err := rows.Scan(&sum.ID, &sum.Slug, &sum.Title, &sum.Summary,
			&sum.CategoryID, &sum.CategoryName, &sum.Status, &sum.PublishedAt,
			&sum.UpdatedAt, &sum.Tags); err != nil {
			return nil, fmt.Errorf("scan kb_article: %w", err)
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// KBGetArticleBySlug returns a full article including tags +
// category. Returns ErrNotFound if absent. publishedOnly excludes
// drafts (customer path uses true; admin uses false).
func (s *Store) KBGetArticleBySlug(ctx context.Context, slug string, publishedOnly bool) (KBArticle, error) {
	q := `SELECT id, slug, title, summary, body_html, body_text, category_id,
	             status, publish_at, published_at, last_edited_by,
	             created_at, updated_at
	      FROM kb_articles WHERE slug = $1`
	if publishedOnly {
		q += ` AND status = 'published'`
	}
	var a KBArticle
	err := s.pool.QueryRow(ctx, q, slug).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBArticle{}, ErrNotFound
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("get kb_article by slug: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

// KBGetArticleByID — admin variant; returns drafts too.
func (s *Store) KBGetArticleByID(ctx context.Context, id int64) (KBArticle, error) {
	var a KBArticle
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, title, summary, body_html, body_text, category_id,
		       status, publish_at, published_at, last_edited_by,
		       created_at, updated_at
		FROM kb_articles WHERE id = $1`, id).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBArticle{}, ErrNotFound
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("get kb_article by id: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

// kbLoadArticleAux fills .Tags and .Category from secondary queries.
// Avoids the JSON-aggregation gymnastics on the main query for clarity.
func (s *Store) kbLoadArticleAux(ctx context.Context, a *KBArticle) error {
	tagRows, err := s.pool.Query(ctx,
		`SELECT t.id, t.slug, t.label, t.created_at
		 FROM kb_tags t JOIN kb_article_tags at ON at.tag_id = t.id
		 WHERE at.article_id = $1 ORDER BY lower(t.label)`, a.ID)
	if err != nil {
		return fmt.Errorf("load tags: %w", err)
	}
	defer tagRows.Close()
	a.Tags = []KBTag{}
	for tagRows.Next() {
		var t KBTag
		if err := tagRows.Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt); err != nil {
			return fmt.Errorf("scan tag: %w", err)
		}
		a.Tags = append(a.Tags, t)
	}
	if err := tagRows.Err(); err != nil {
		return err
	}
	cat, err := s.KBGetCategory(ctx, a.CategoryID)
	if err == nil {
		a.Category = &cat
	}
	return nil
}

// KBSaveArticle creates if id==0, else updates. tagIDs is the full
// desired set (the writer rewrites kb_article_tags inside a tx).
// Returns the row as it stands after the write.
func (s *Store) KBSaveArticle(ctx context.Context, in KBArticle, tagIDs []int64) (KBArticle, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return KBArticle{}, fmt.Errorf("save kb_article begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if in.ID == 0 {
		err = tx.QueryRow(ctx, `
			INSERT INTO kb_articles
			  (slug, title, summary, body_html, body_text, category_id,
			   status, publish_at, published_at, last_edited_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			RETURNING id, created_at, updated_at`,
			in.Slug, in.Title, in.Summary, in.BodyHTML, in.BodyText, in.CategoryID,
			in.Status, in.PublishAt, in.PublishedAt, in.LastEditedBy).
			Scan(&in.ID, &in.CreatedAt, &in.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE kb_articles SET
			  slug = $2, title = $3, summary = $4, body_html = $5, body_text = $6,
			  category_id = $7, status = $8, publish_at = $9, published_at = $10,
			  last_edited_by = $11, updated_at = NOW()
			WHERE id = $1
			RETURNING created_at, updated_at`,
			in.ID, in.Slug, in.Title, in.Summary, in.BodyHTML, in.BodyText, in.CategoryID,
			in.Status, in.PublishAt, in.PublishedAt, in.LastEditedBy).
			Scan(&in.CreatedAt, &in.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return KBArticle{}, ErrNotFound
		}
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("upsert kb_article: %w", err)
	}

	// Rewrite tag set.
	if _, err := tx.Exec(ctx,
		`DELETE FROM kb_article_tags WHERE article_id = $1`, in.ID); err != nil {
		return KBArticle{}, fmt.Errorf("clear kb_article_tags: %w", err)
	}
	for _, tID := range tagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO kb_article_tags (article_id, tag_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`, in.ID, tID); err != nil {
			return KBArticle{}, fmt.Errorf("insert kb_article_tag: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return KBArticle{}, fmt.Errorf("save kb_article commit: %w", err)
	}
	return s.KBGetArticleByID(ctx, in.ID)
}

// KBDeleteArticle hard-deletes. Returns ErrNotFound if absent.
func (s *Store) KBDeleteArticle(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_articles WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete kb_article: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// KBPublishArticle flips status to 'published', sets published_at,
// clears publish_at. Returns the post-write row. Errors if not draft.
func (s *Store) KBPublishArticle(ctx context.Context, id int64, by string) (KBArticle, error) {
	var a KBArticle
	err := s.pool.QueryRow(ctx, `
		UPDATE kb_articles
		SET status = 'published',
		    published_at = NOW(),
		    publish_at = NULL,
		    last_edited_by = $2,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'draft'
		RETURNING id, slug, title, summary, body_html, body_text, category_id,
		          status, publish_at, published_at, last_edited_by,
		          created_at, updated_at`, id, by).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the article doesn't exist or it's already published.
		// Distinguish for the handler.
		exists, _ := s.KBArticleExists(ctx, id)
		if !exists {
			return KBArticle{}, ErrNotFound
		}
		return KBArticle{}, fmt.Errorf("publish kb_article: not in draft state")
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("publish kb_article: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

// KBUnpublishArticle reverts a published article to draft. Preserves
// published_at for history.
func (s *Store) KBUnpublishArticle(ctx context.Context, id int64, by string) (KBArticle, error) {
	var a KBArticle
	err := s.pool.QueryRow(ctx, `
		UPDATE kb_articles
		SET status = 'draft',
		    last_edited_by = $2,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'published'
		RETURNING id, slug, title, summary, body_html, body_text, category_id,
		          status, publish_at, published_at, last_edited_by,
		          created_at, updated_at`, id, by).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		exists, _ := s.KBArticleExists(ctx, id)
		if !exists {
			return KBArticle{}, ErrNotFound
		}
		return KBArticle{}, fmt.Errorf("unpublish kb_article: not in published state")
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("unpublish kb_article: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

func (s *Store) KBArticleExists(ctx context.Context, id int64) (bool, error) {
	var x int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM kb_articles WHERE id = $1`, id).Scan(&x)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check kb_article exists: %w", err)
	}
	return true, nil
}

// KBArticleSlugExists for the slug-collision suffix loop.
func (s *Store) KBArticleSlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT 1 FROM kb_articles WHERE slug = $1 AND ($2 = 0 OR id != $2)`,
		slug, excludeID).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check kb_article slug: %w", err)
	}
	return true, nil
}

// KBPendingPublishes returns drafts whose publish_at has elapsed.
// Used by the daily cron pass.
func (s *Store) KBPendingPublishes(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM kb_articles
		WHERE status = 'draft' AND publish_at IS NOT NULL AND publish_at <= $1
		ORDER BY publish_at
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("kb pending publishes: %w", err)
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
