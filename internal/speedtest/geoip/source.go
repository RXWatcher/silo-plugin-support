// Package geoip implements GeoIP source kinds + the chain walker
// the auto-strategy resolver consults.
package geoip

import (
	"context"
	"net/http"
)

// Source is a single GeoIP lookup strategy. Resolve returns:
//   - country: ISO 3166-1 alpha-2 (uppercased) or "" if the source
//     can't answer for this IP / request
//   - err:     a transient/operational error (network, file read).
//     The chain walker treats "" + err as "miss" and moves on; the
//     error is propagated to status logging via the StatusSink.
type Source interface {
	ID() int64
	Kind() string
	Resolve(ctx context.Context, ip string, r *http.Request) (country string, err error)
}

// StatusSink lets the chain record per-source success / failure so the
// admin UI can surface "ok / used 12 min ago" or "error: dns timeout".
type StatusSink interface {
	MarkUsed(sourceID int64)
	MarkStatus(sourceID int64, status string) // "ok" on success, "error: ..." on failure
}
