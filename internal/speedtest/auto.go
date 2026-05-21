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

type Resolver struct {
	store    EndpointLister
	geoip    GeoIPResolver
	strategy string
}

func NewResolver(store EndpointLister, geoip GeoIPResolver, strategy string) *Resolver {
	if strategy != "latency" && strategy != "geoip" {
		strategy = "latency"
	}
	return &Resolver{store: store, geoip: geoip, strategy: strategy}
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
					return AutoResolution{
						Strategy: "geoip",
						Endpoint: &ep,
						GeoIP:    AutoGeoIPHint{Country: country, SourceID: srcID},
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
