package geoip

import (
	"context"
	"net/http"
	"testing"
)

type fakeSource struct {
	id        int64
	country   string
	failErr   error
	callCount int
}

func (f *fakeSource) ID() int64    { return f.id }
func (f *fakeSource) Kind() string { return "fake" }
func (f *fakeSource) Resolve(_ context.Context, _ string, _ *http.Request) (string, error) {
	f.callCount++
	return f.country, f.failErr
}

type recordingStatus struct {
	updates []statusUpdate
}

type statusUpdate struct {
	sourceID int64
	used     bool
	status   string
}

func (r *recordingStatus) MarkUsed(id int64)             { r.updates = append(r.updates, statusUpdate{id, true, "ok"}) }
func (r *recordingStatus) MarkStatus(id int64, s string) { r.updates = append(r.updates, statusUpdate{id, false, s}) }

func TestChainReturnsFirstNonEmpty(t *testing.T) {
	a := &fakeSource{id: 1, country: ""}
	b := &fakeSource{id: 2, country: "GB"}
	c := &fakeSource{id: 3, country: "US"}
	rec := &recordingStatus{}
	chain := NewChain([]Source{a, b, c}, rec)

	got, srcID, err := chain.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "GB" || srcID != 2 {
		t.Fatalf("got (%q, %d), want (GB, 2)", got, srcID)
	}
	if c.callCount != 0 {
		t.Fatalf("third source should not have been called once we got a hit")
	}
}

func TestChainAllMissReturnsEmpty(t *testing.T) {
	a := &fakeSource{country: ""}
	b := &fakeSource{country: ""}
	chain := NewChain([]Source{a, b}, &recordingStatus{})
	got, srcID, err := chain.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil || got != "" || srcID != 0 {
		t.Fatalf("got (%q, %d, %v), want ('', 0, nil)", got, srcID, err)
	}
}

func TestChainErrorOnSourceMovesOn(t *testing.T) {
	a := &fakeSource{country: "", failErr: context.Canceled}
	b := &fakeSource{id: 2, country: "GB"}
	chain := NewChain([]Source{a, b}, &recordingStatus{})
	got, srcID, _ := chain.Resolve(context.Background(), "192.0.2.1", nil)
	if got != "GB" || srcID != 2 {
		t.Fatalf("got (%q, %d), want (GB, 2)", got, srcID)
	}
}
