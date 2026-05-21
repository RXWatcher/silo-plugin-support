package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// STInsertResult writes a row and returns it.
func (s *Store) STInsertResult(ctx context.Context, in STResult) (STResult, error) {
	var out STResult
	var ipBytes any
	if in.ClientIP != "" {
		ipBytes = in.ClientIP
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO st_results (
		  customer_id, endpoint_id, endpoint_label, auto_strategy,
		  download_mbps, upload_mbps, ping_ms, jitter_ms,
		  client_ip, user_agent
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::inet,$10)
		RETURNING id, customer_id, endpoint_id, endpoint_label, auto_strategy,
		          download_mbps, upload_mbps, ping_ms, jitter_ms,
		          host(client_ip), user_agent, ran_at`,
		in.CustomerID, in.EndpointID, in.EndpointLabel, in.AutoStrategy,
		in.DownloadMbps, in.UploadMbps, in.PingMs, in.JitterMs,
		ipBytes, in.UserAgent).
		Scan(&out.ID, &out.CustomerID, &out.EndpointID, &out.EndpointLabel, &out.AutoStrategy,
			&out.DownloadMbps, &out.UploadMbps, &out.PingMs, &out.JitterMs,
			&out.ClientIP, &out.UserAgent, &out.RanAt)
	if err != nil {
		return STResult{}, fmt.Errorf("insert st_result: %w", err)
	}
	return out, nil
}

// STCustomerHistory returns the calling customer's last N results.
func (s *Store) STCustomerHistory(ctx context.Context, customerID string, limit int) ([]STResult, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.stListResults(ctx, `WHERE customer_id = $1`, []any{customerID}, "ran_at DESC", limit, 0)
}

// STLastResultAt returns when this customer last ran a test, or
// zero time if never. Used by the 60s rate-limit check.
func (s *Store) STLastResultAt(ctx context.Context, customerID string) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT ran_at FROM st_results WHERE customer_id = $1 ORDER BY ran_at DESC LIMIT 1`,
		customerID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("last st_result at: %w", err)
	}
	return t, nil
}

// STListResults serves the admin filtered listing.
func (s *Store) STListResults(ctx context.Context, f STResultFilter) ([]STResult, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	args := []any{}
	clauses := []string{}
	if f.CustomerID != "" {
		args = append(args, f.CustomerID)
		clauses = append(clauses, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.EndpointID > 0 {
		args = append(args, f.EndpointID)
		clauses = append(clauses, fmt.Sprintf("endpoint_id = $%d", len(args)))
	}
	if f.AutoStrategy != "" {
		args = append(args, f.AutoStrategy)
		clauses = append(clauses, fmt.Sprintf("auto_strategy = $%d", len(args)))
	}
	if !f.Since.IsZero() {
		args = append(args, f.Since)
		clauses = append(clauses, fmt.Sprintf("ran_at >= $%d", len(args)))
	}
	if f.SlowOnly && f.SlowThresh > 0 {
		args = append(args, f.SlowThresh)
		clauses = append(clauses, fmt.Sprintf("download_mbps < $%d", len(args)))
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	return s.stListResults(ctx, where, args, "ran_at DESC", f.Limit, f.Offset)
}

func (s *Store) stListResults(ctx context.Context, where string, args []any, orderBy string, limit, offset int) ([]STResult, error) {
	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, customer_id, endpoint_id, endpoint_label, auto_strategy,
		       download_mbps, upload_mbps, ping_ms, jitter_ms,
		       COALESCE(host(client_ip), ''), user_agent, ran_at
		FROM st_results
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, where, orderBy, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list st_results: %w", err)
	}
	defer rows.Close()
	out := []STResult{}
	for rows.Next() {
		var r STResult
		if err := rows.Scan(&r.ID, &r.CustomerID, &r.EndpointID, &r.EndpointLabel, &r.AutoStrategy,
			&r.DownloadMbps, &r.UploadMbps, &r.PingMs, &r.JitterMs,
			&r.ClientIP, &r.UserAgent, &r.RanAt); err != nil {
			return nil, fmt.Errorf("scan st_result: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// STDashboardAggregatesData returns the four aggregate slices the
// admin dashboard page renders. Window is fixed at 30 days.
func (s *Store) STDashboardAggregatesData(ctx context.Context, slowThresh float64) (STDashboardAggregates, error) {
	out := STDashboardAggregates{
		PerEndpoint: []STEndpointAggregate{},
		PerDay:      []STDailyCount{},
		SlowTop10:   []STResult{},
		CountryHits: []STCountryCount{},
	}

	rows, err := s.pool.Query(ctx, `
		SELECT endpoint_id, endpoint_label,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY download_mbps)::float8,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY upload_mbps)::float8,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY ping_ms)::float8,
		       COUNT(*)
		FROM st_results
		WHERE ran_at > NOW() - INTERVAL '30 days'
		GROUP BY endpoint_id, endpoint_label
		ORDER BY COUNT(*) DESC`)
	if err != nil {
		return out, fmt.Errorf("per-endpoint medians: %w", err)
	}
	for rows.Next() {
		var a STEndpointAggregate
		if err := rows.Scan(&a.EndpointID, &a.Label, &a.MedianDownload, &a.MedianUpload, &a.MedianPing, &a.ResultCount); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan per-endpoint: %w", err)
		}
		out.PerEndpoint = append(out.PerEndpoint, a)
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `
		SELECT to_char(date_trunc('day', ran_at), 'YYYY-MM-DD'), COUNT(*)
		FROM st_results
		WHERE ran_at > NOW() - INTERVAL '30 days'
		GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return out, fmt.Errorf("per-day counts: %w", err)
	}
	for rows.Next() {
		var d STDailyCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan per-day: %w", err)
		}
		out.PerDay = append(out.PerDay, d)
	}
	rows.Close()

	if slowThresh > 0 {
		results, err := s.stListResults(ctx,
			`WHERE ran_at > NOW() - INTERVAL '7 days' AND download_mbps < $1`,
			[]any{slowThresh},
			`download_mbps ASC`, 10, 0)
		if err != nil {
			return out, fmt.Errorf("slow top-10: %w", err)
		}
		out.SlowTop10 = results
	}

	// countryHits left empty in v1 — see plan's self-review for the
	// st_results resolved-country column follow-up.

	return out, nil
}
