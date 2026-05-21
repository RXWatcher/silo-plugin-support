package speedtest

import (
	"context"
	"net/http"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// EndpointLister is the slice of Store the resolver needs.
type EndpointLister interface {
	STListEndpoints(ctx context.Context, activeOnly bool) ([]store.STEndpoint, error)
}

// GeoIPResolver mirrors the geoip.Chain.Resolve signature without
// importing the geoip package (keeps the resolver free of that
// dependency for testability).
type GeoIPResolver interface {
	Resolve(ctx context.Context, ip string, r *http.Request) (country string, sourceID int64, err error)
}

// AutoResolution is what the /api/customer/speedtest/auto handler
// returns to the SPA.
type AutoResolution struct {
	Strategy   string             `json:"strategy"`
	Endpoint   *store.STEndpoint  `json:"endpoint,omitempty"`
	Candidates []store.STEndpoint `json:"candidates,omitempty"`
	GeoIP      AutoGeoIPHint      `json:"geoip"`
}

type AutoGeoIPHint struct {
	Country     string `json:"country,omitempty"`
	SourceID    int64  `json:"sourceId,omitempty"`
	SourceLabel string `json:"sourceLabel,omitempty"`
}

// SourceLabelLookup returns the human-readable label for a geoip source
// by ID. Returning "" is safe — the field is omitempty.
type SourceLabelLookup func(id int64) string

type Resolver struct {
	store      EndpointLister
	geoip      GeoIPResolver
	strategy   string
	labelOfSrc SourceLabelLookup // optional; nil → no label populated
}

func NewResolver(store EndpointLister, geoip GeoIPResolver, strategy string) *Resolver {
	if strategy != "latency" && strategy != "geoip" {
		strategy = "latency"
	}
	return &Resolver{store: store, geoip: geoip, strategy: strategy}
}

// WithLabelLookup wires in a store-backed label lookup. Returns the
// receiver so the call can be chained onto NewResolver.
func (r *Resolver) WithLabelLookup(f SourceLabelLookup) *Resolver {
	r.labelOfSrc = f
	return r
}

// Resolve picks the endpoint (or returns the latency candidate list)
// per the configured strategy. Filters endpoints by Active here in
// case the underlying store didn't (e.g. test fakes).
func (r *Resolver) Resolve(ctx context.Context, clientIP string, req *http.Request) (AutoResolution, error) {
	allRaw, err := r.store.STListEndpoints(ctx, true)
	if err != nil {
		return AutoResolution{}, err
	}
	all := make([]store.STEndpoint, 0, len(allRaw))
	for _, ep := range allRaw {
		if ep.Active {
			all = append(all, ep)
		}
	}

	if r.strategy == "geoip" && r.geoip != nil {
		country, srcID, _ := r.geoip.Resolve(ctx, clientIP, req)
		if country != "" {
			for _, ep := range all {
				if ep.Country == country {
					ep := ep
					hint := AutoGeoIPHint{Country: country, SourceID: srcID}
					if r.labelOfSrc != nil && srcID > 0 {
						hint.SourceLabel = r.labelOfSrc(srcID)
					}
					return AutoResolution{
						Strategy: "geoip",
						Endpoint: &ep,
						GeoIP:    hint,
					}, nil
				}
			}
		}
	}

	if r.strategy == "latency" {
		return AutoResolution{
			Strategy:   "latency",
			Candidates: all,
		}, nil
	}

	if len(all) > 0 {
		ep := all[0]
		return AutoResolution{Strategy: "fallback", Endpoint: &ep}, nil
	}
	return AutoResolution{Strategy: "fallback"}, nil
}
