package speedtest

import (
	"context"
	"net/http"
	"testing"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

type fakeEPStore struct{ endpoints []store.STEndpoint }

func (f *fakeEPStore) STListEndpoints(_ context.Context, _ bool) ([]store.STEndpoint, error) {
	return f.endpoints, nil
}

type fakeGeoIP struct {
	country string
	srcID   int64
}

func (f *fakeGeoIP) Resolve(_ context.Context, _ string, _ *http.Request) (string, int64, error) {
	return f.country, f.srcID, nil
}

func endpoints() []store.STEndpoint {
	return []store.STEndpoint{
		{ID: 1, Label: "London",    URL: "https://lon/", Country: "GB", Active: true, SortOrder: 0},
		{ID: 2, Label: "Frankfurt", URL: "https://fra/", Country: "DE", Active: true, SortOrder: 1},
		{ID: 3, Label: "Disabled",  URL: "https://x/",   Country: "FR", Active: false, SortOrder: 2},
	}
}

func TestResolveGeoIPPicksMatchingCountry(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{country: "DE", srcID: 7}, "geoip")
	out, err := r.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil { t.Fatal(err) }
	if out.Strategy != "geoip" || out.Endpoint == nil || out.Endpoint.ID != 2 {
		t.Fatalf("got %+v, want geoip strategy + endpoint 2", out)
	}
	if out.GeoIP.Country != "DE" || out.GeoIP.SourceID != 7 {
		t.Fatalf("got GeoIP %+v, want {DE, 7}", out.GeoIP)
	}
}

func TestResolveGeoIPNoMatchFallsThroughToFirstActive(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{country: "JP"}, "geoip")
	out, _ := r.Resolve(context.Background(), "192.0.2.1", nil)
	if out.Strategy != "fallback" || out.Endpoint == nil || out.Endpoint.ID != 1 {
		t.Fatalf("got %+v, want fallback strategy + endpoint 1", out)
	}
}

func TestResolveLatencyReturnsActiveCandidates(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{}, "latency")
	out, _ := r.Resolve(context.Background(), "192.0.2.1", nil)
	if out.Strategy != "latency" {
		t.Fatalf("got strategy %q, want latency", out.Strategy)
	}
	if len(out.Candidates) != 2 || out.Endpoint != nil {
		t.Fatalf("got candidates %v + endpoint %v; want 2 candidates, nil endpoint", out.Candidates, out.Endpoint)
	}
}

func TestResolveGeoIPEmptyResolverFallsThrough(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{country: ""}, "geoip")
	out, _ := r.Resolve(context.Background(), "192.0.2.1", nil)
	if out.Strategy != "fallback" || out.Endpoint == nil || out.Endpoint.ID != 1 {
		t.Fatalf("got %+v, want fallback to first active", out)
	}
}
