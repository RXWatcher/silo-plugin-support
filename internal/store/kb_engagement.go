package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// KBUpsertVote (article, customer, vote) — upsert by PK.
func (s *Store) KBUpsertVote(ctx context.Context, articleID int64, customerID, vote string) error {
	if vote != "up" && vote != "down" {
		return fmt.Errorf("kb upsert vote: invalid vote %q", vote)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO kb_votes (article_id, customer_id, vote)
		VALUES ($1, $2, $3)
		ON CONFLICT (article_id, customer_id) DO UPDATE
		SET vote = EXCLUDED.vote, voted_at = NOW()`,
		articleID, customerID, vote)
	if err != nil {
		return fmt.Errorf("kb upsert vote: %w", err)
	}
	return nil
}

// KBGetVote returns this customer's vote for an article, or
// ErrNotFound if they haven't voted.
func (s *Store) KBGetVote(ctx context.Context, articleID int64, customerID string) (string, error) {
	var v string
	err := s.pool.QueryRow(ctx,
		`SELECT vote FROM kb_votes WHERE article_id = $1 AND customer_id = $2`,
		articleID, customerID).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("kb get vote: %w", err)
	}
	return v, nil
}

// KBVoteAggregateFor returns the up/down counts for one article.
func (s *Store) KBVoteAggregateFor(ctx context.Context, articleID int64) (KBVoteAggregate, error) {
	var agg KBVoteAggregate
	err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE vote = 'up') AS helpful,
		  COUNT(*) FILTER (WHERE vote = 'down') AS not_helpful
		FROM kb_votes WHERE article_id = $1`, articleID).
		Scan(&agg.HelpfulCount, &agg.NotHelpfulCount)
	if err != nil {
		return KBVoteAggregate{}, fmt.Errorf("kb vote aggregate: %w", err)
	}
	return agg, nil
}

// KBRecordView inserts a view row IFF no view exists for this
// (article, customer) in the last 24 hours. Returns true if a row
// was inserted, false if deduped.
func (s *Store) KBRecordView(ctx context.Context, articleID int64, customerID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO kb_views (article_id, customer_id)
		SELECT $1, $2
		WHERE NOT EXISTS (
		  SELECT 1 FROM kb_views
		  WHERE article_id = $1 AND customer_id = $2
		    AND viewed_at > NOW() - INTERVAL '24 hours'
		)`, articleID, customerID)
	if err != nil {
		return false, fmt.Errorf("kb record view: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// KBViewAggregate30d returns total + unique counts over the last 30
// days for one article.
func (s *Store) KBViewAggregate30d(ctx context.Context, articleID int64) (KBViewAggregate, error) {
	var agg KBViewAggregate
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) AS total, COUNT(DISTINCT customer_id) AS unique_v
		FROM kb_views
		WHERE article_id = $1 AND viewed_at > NOW() - INTERVAL '30 days'`,
		articleID).Scan(&agg.TotalViews, &agg.UniqueViewers)
	if err != nil {
		return KBViewAggregate{}, fmt.Errorf("kb view aggregate: %w", err)
	}
	return agg, nil
}

// KBVoteWindow24h returns helpful + not-helpful + ratio for the
// last 24h. Used by the unhelpful-detection cron.
type KBVoteWindow struct {
	Helpful      int
	NotHelpful   int
	HelpfulRatio float64 // 0..1; -1 if total == 0
}

func (s *Store) KBVoteWindow24h(ctx context.Context, articleID int64) (KBVoteWindow, error) {
	var w KBVoteWindow
	err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE vote = 'up')   AS h,
		  COUNT(*) FILTER (WHERE vote = 'down') AS nh
		FROM kb_votes
		WHERE article_id = $1 AND voted_at > NOW() - INTERVAL '24 hours'`,
		articleID).Scan(&w.Helpful, &w.NotHelpful)
	if err != nil {
		return KBVoteWindow{}, fmt.Errorf("kb vote window: %w", err)
	}
	tot := w.Helpful + w.NotHelpful
	if tot == 0 {
		w.HelpfulRatio = -1
	} else {
		w.HelpfulRatio = float64(w.Helpful) / float64(tot)
	}
	return w, nil
}

// KBPublishedArticleIDs returns the IDs of all published articles —
// the cron iterates these for the unhelpful-detection pass.
func (s *Store) KBPublishedArticleIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM kb_articles WHERE status = 'published'`)
	if err != nil {
		return nil, fmt.Errorf("kb published ids: %w", err)
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

var _ = time.Time{} // imported for callers that build cutoff timestamps
