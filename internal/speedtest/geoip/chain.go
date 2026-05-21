package geoip

import (
	"context"
	"fmt"
	"net/http"
)

// Chain walks an ordered list of Sources, returning the country from
// the first source that returns a non-empty result. Sources that
// error are marked in the status sink and skipped.
type Chain struct {
	sources []Source
	status  StatusSink
}

func NewChain(sources []Source, status StatusSink) *Chain {
	return &Chain{sources: sources, status: status}
}

// Resolve returns (country, sourceID, err). country may be "" if no
// source answered. err is non-nil only for context cancellation —
// individual source failures are absorbed and logged via the status
// sink so a flaky source can't block the chain.
func (c *Chain) Resolve(ctx context.Context, ip string, r *http.Request) (string, int64, error) {
	for _, src := range c.sources {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		country, err := src.Resolve(ctx, ip, r)
		if err != nil {
			c.status.MarkStatus(src.ID(), fmt.Sprintf("error: %s", err))
			continue
		}
		if country == "" {
			continue
		}
		c.status.MarkUsed(src.ID())
		return country, src.ID(), nil
	}
	return "", 0, nil
}
