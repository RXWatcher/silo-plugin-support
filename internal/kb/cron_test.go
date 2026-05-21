package kb

import (
	"context"
	"testing"
	"time"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// fakeStore is a hand-rolled minimal stand-in covering only the
// methods Cron touches.
type fakeStore struct {
	pending      []int64
	publishedIDs map[int64]bool
	voteWin      map[int64]store.KBVoteWindow
}

func (f *fakeStore) KBPendingPublishes(_ context.Context, _ time.Time, _ int) ([]int64, error) {
	return f.pending, nil
}
func (f *fakeStore) KBPublishArticle(_ context.Context, id int64, _ string) (store.KBArticle, error) {
	f.publishedIDs[id] = true
	return store.KBArticle{ID: id, Slug: "x", Status: "published"}, nil
}
func (f *fakeStore) KBPublishedArticleIDs(_ context.Context) ([]int64, error) {
	out := []int64{}
	for id := range f.publishedIDs {
		out = append(out, id)
	}
	return out, nil
}
func (f *fakeStore) KBVoteWindow24h(_ context.Context, id int64) (store.KBVoteWindow, error) {
	return f.voteWin[id], nil
}
func (f *fakeStore) KBGetArticleByID(_ context.Context, id int64) (store.KBArticle, error) {
	return store.KBArticle{ID: id, Slug: "x", Status: "published"}, nil
}

type fakePublisher struct{ events []string }

func (f *fakePublisher) PublishKBArticleEvent(_ context.Context, name string, _ store.KBArticle, _ map[string]any) {
	f.events = append(f.events, name)
}

func TestCronPublishDuePublishesAndEmits(t *testing.T) {
	s := &fakeStore{
		pending:      []int64{1, 2},
		publishedIDs: map[int64]bool{},
	}
	p := &fakePublisher{}
	c := &Cron{Store: s, Publisher: p, UnhelpfulThreshold: 0.5, UnhelpfulMinVotes: 5}
	if err := c.PublishDue(context.Background()); err != nil {
		t.Fatalf("PublishDue: %v", err)
	}
	if !s.publishedIDs[1] || !s.publishedIDs[2] {
		t.Fatalf("expected both pending articles published; got %v", s.publishedIDs)
	}
	if len(p.events) != 2 || p.events[0] != "kb_article_published" {
		t.Fatalf("expected 2 publish events; got %v", p.events)
	}
}

func TestCronUnhelpfulSweepEmitsOnlyBelowThresholdWithEnoughVotes(t *testing.T) {
	s := &fakeStore{
		publishedIDs: map[int64]bool{1: true, 2: true, 3: true},
		voteWin: map[int64]store.KBVoteWindow{
			1: {Helpful: 1, NotHelpful: 9, HelpfulRatio: 0.10}, // emit
			2: {Helpful: 4, NotHelpful: 6, HelpfulRatio: 0.40}, // emit
			3: {Helpful: 2, NotHelpful: 1, HelpfulRatio: 0.66}, // skip (above)
		},
	}
	p := &fakePublisher{}
	c := &Cron{Store: s, Publisher: p, UnhelpfulThreshold: 0.5, UnhelpfulMinVotes: 5}
	if err := c.UnhelpfulSweep(context.Background()); err != nil {
		t.Fatalf("UnhelpfulSweep: %v", err)
	}
	got := map[string]int{}
	for _, e := range p.events {
		got[e]++
	}
	if got["kb_article_unhelpful"] != 2 {
		t.Fatalf("expected 2 unhelpful events; got %v", got)
	}
}

func TestCronUnhelpfulSweepSkipsWhenBelowMinVotes(t *testing.T) {
	s := &fakeStore{
		publishedIDs: map[int64]bool{1: true},
		voteWin: map[int64]store.KBVoteWindow{
			1: {Helpful: 0, NotHelpful: 2, HelpfulRatio: 0.0}, // bad ratio but only 2 votes
		},
	}
	p := &fakePublisher{}
	c := &Cron{Store: s, Publisher: p, UnhelpfulThreshold: 0.5, UnhelpfulMinVotes: 5}
	_ = c.UnhelpfulSweep(context.Background())
	if len(p.events) != 0 {
		t.Fatalf("expected no events when below min votes; got %v", p.events)
	}
}
