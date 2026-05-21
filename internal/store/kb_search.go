package store

import (
	"context"
	"fmt"
)

// KBSearchHit is a search result with a rank score the customer
// page uses for ordering.
type KBSearchHit struct {
	Article KBArticleSummary `json:"article"`
	Rank    float64          `json:"rank"`
	Snippet string           `json:"snippet"`
}

// KBSearchArticles runs a plainto_tsquery over published articles
// only and returns ranked hits with an ts_headline snippet of body_text.
func (s *Store) KBSearchArticles(ctx context.Context, q string, limit int) ([]KBSearchHit, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		WITH q AS (SELECT plainto_tsquery('english', $1) AS tsq)
		SELECT a.id, a.slug, a.title, a.summary, a.category_id, c.name,
		       a.status, a.published_at, a.updated_at,
		       COALESCE(
		         (SELECT array_agg(t.slug ORDER BY t.label)
		          FROM kb_article_tags at JOIN kb_tags t ON t.id = at.tag_id
		          WHERE at.article_id = a.id),
		         ARRAY[]::text[]) AS tag_slugs,
		       ts_rank(a.search_vector, q.tsq) AS rank,
		       ts_headline('english', a.body_text, q.tsq,
		                   'StartSel=<mark>, StopSel=</mark>, MaxFragments=2, MinWords=8, MaxWords=20')
		FROM kb_articles a, q
		JOIN kb_categories c ON c.id = a.category_id
		WHERE a.status = 'published' AND a.search_vector @@ q.tsq
		ORDER BY rank DESC, a.published_at DESC
		LIMIT $2`, q, limit)
	if err != nil {
		return nil, fmt.Errorf("kb search: %w", err)
	}
	defer rows.Close()
	out := []KBSearchHit{}
	for rows.Next() {
		var h KBSearchHit
		if err := rows.Scan(&h.Article.ID, &h.Article.Slug, &h.Article.Title,
			&h.Article.Summary, &h.Article.CategoryID, &h.Article.CategoryName,
			&h.Article.Status, &h.Article.PublishedAt, &h.Article.UpdatedAt,
			&h.Article.Tags, &h.Rank, &h.Snippet); err != nil {
			return nil, fmt.Errorf("scan kb search hit: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// KBRelatedArticles returns up to `limit` other published articles
// whose search vector overlaps with the source article's, biased
// toward the same category. Source article excluded from results.
func (s *Store) KBRelatedArticles(ctx context.Context, sourceID int64, limit int) ([]KBArticleSummary, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := s.pool.Query(ctx, `
		WITH src AS (
		  SELECT search_vector AS sv, category_id AS cat
		  FROM kb_articles WHERE id = $1
		)
		SELECT a.id, a.slug, a.title, a.summary, a.category_id, c.name,
		       a.status, a.published_at, a.updated_at,
		       COALESCE(
		         (SELECT array_agg(t.slug ORDER BY t.label)
		          FROM kb_article_tags at JOIN kb_tags t ON t.id = at.tag_id
		          WHERE at.article_id = a.id),
		         ARRAY[]::text[]) AS tag_slugs
		FROM kb_articles a, src
		JOIN kb_categories c ON c.id = a.category_id
		WHERE a.id != $1
		  AND a.status = 'published'
		  AND a.search_vector @@ to_tsquery('english', replace(strip(src.sv)::text, ' ', ' | '))
		ORDER BY (a.category_id = src.cat) DESC,
		         ts_rank(a.search_vector, to_tsquery('english', replace(strip(src.sv)::text, ' ', ' | '))) DESC,
		         a.published_at DESC
		LIMIT $2`, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("kb related: %w", err)
	}
	defer rows.Close()
	out := []KBArticleSummary{}
	for rows.Next() {
		var sum KBArticleSummary
		if err := rows.Scan(&sum.ID, &sum.Slug, &sum.Title, &sum.Summary,
			&sum.CategoryID, &sum.CategoryName, &sum.Status, &sum.PublishedAt,
			&sum.UpdatedAt, &sum.Tags); err != nil {
			return nil, fmt.Errorf("scan kb related: %w", err)
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}
