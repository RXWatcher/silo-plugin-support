package tickets

import (
	"context"
	"testing"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

type fakeStore struct {
	resolved      []int64
	waiting       []int64
	closed        map[int64]string
	tickets       map[int64]store.TKTicket
	systemEntries []store.TKEntry
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		closed:        map[int64]string{},
		tickets:       map[int64]store.TKTicket{},
		systemEntries: []store.TKEntry{},
	}
}

func (f *fakeStore) TKResolvedAtIdleSince(_ context.Context, _ int, _ int) ([]int64, error) {
	return f.resolved, nil
}
func (f *fakeStore) TKWaitingIdleSince(_ context.Context, _ int, _ int) ([]int64, error) {
	return f.waiting, nil
}
func (f *fakeStore) TKGetTicketByID(_ context.Context, id int64) (store.TKTicket, error) {
	if t, ok := f.tickets[id]; ok {
		return t, nil
	}
	return store.TKTicket{ID: id, Status: "resolved", TrackingNumber: "SUP-X"}, nil
}
func (f *fakeStore) TKUpdateTicketStatus(_ context.Context, id int64, newStatus string, _, _ *interface{}) (store.TKTicket, error) {
	t := f.tickets[id]
	t.ID = id
	t.Status = newStatus
	f.tickets[id] = t
	return t, nil
}
func (f *fakeStore) TKInsertEntryNoTx(_ context.Context, e store.TKEntry) (store.TKEntry, error) {
	f.systemEntries = append(f.systemEntries, e)
	return e, nil
}

type fakeEmitter struct{ events []string }

func (f *fakeEmitter) PublishTicketEvent(_ context.Context, name string, _ store.TKTicket, _ map[string]any) {
	f.events = append(f.events, name)
}

func TestCloseIdleClosesResolvedAndWaiting(t *testing.T) {
	s := newFakeStore()
	s.resolved = []int64{1, 2}
	s.waiting = []int64{3}
	s.tickets[1] = store.TKTicket{ID: 1, Status: "resolved", TrackingNumber: "SUP-1"}
	s.tickets[2] = store.TKTicket{ID: 2, Status: "resolved", TrackingNumber: "SUP-2"}
	s.tickets[3] = store.TKTicket{ID: 3, Status: "waiting_customer", TrackingNumber: "SUP-3"}
	em := &fakeEmitter{}
	c := &Cron{Store: s, Emitter: em, Enabled: true, ResolvedAfterDays: 7, WaitingAfterDays: 14}
	if err := c.CloseIdle(context.Background()); err != nil {
		t.Fatalf("CloseIdle: %v", err)
	}
	if s.tickets[1].Status != "closed" || s.tickets[2].Status != "closed" || s.tickets[3].Status != "closed" {
		t.Fatalf("expected all 3 closed; got %+v", s.tickets)
	}
	if len(em.events) != 3 {
		t.Fatalf("expected 3 ticket_closed events; got %d", len(em.events))
	}
	for _, ev := range em.events {
		if ev != "ticket_closed" {
			t.Fatalf("unexpected event %q", ev)
		}
	}
	if len(s.systemEntries) != 3 {
		t.Fatalf("expected one system entry per close; got %d", len(s.systemEntries))
	}
}

func TestCloseIdleNoOpWhenDisabled(t *testing.T) {
	s := newFakeStore()
	s.resolved = []int64{1}
	s.waiting = []int64{2}
	s.tickets[1] = store.TKTicket{ID: 1, Status: "resolved"}
	s.tickets[2] = store.TKTicket{ID: 2, Status: "waiting_customer"}
	c := &Cron{Store: s, Emitter: &fakeEmitter{}, Enabled: false, ResolvedAfterDays: 7, WaitingAfterDays: 14}
	if err := c.CloseIdle(context.Background()); err != nil { t.Fatal(err) }
	if s.tickets[1].Status == "closed" || s.tickets[2].Status == "closed" {
		t.Fatalf("disabled cron should be a no-op; got %+v", s.tickets)
	}
}

func TestCloseIdleZeroDaysSkipsThatPass(t *testing.T) {
	s := newFakeStore()
	s.resolved = []int64{1}
	s.waiting = []int64{2}
	s.tickets[1] = store.TKTicket{ID: 1, Status: "resolved"}
	s.tickets[2] = store.TKTicket{ID: 2, Status: "waiting_customer"}
	c := &Cron{Store: s, Emitter: &fakeEmitter{}, Enabled: true, ResolvedAfterDays: 0, WaitingAfterDays: 14}
	if err := c.CloseIdle(context.Background()); err != nil { t.Fatal(err) }
	if s.tickets[1].Status == "closed" {
		t.Fatalf("resolved-pass should have been skipped (days=0)")
	}
	if s.tickets[2].Status != "closed" {
		t.Fatalf("waiting-pass should have closed ticket 2")
	}
}
