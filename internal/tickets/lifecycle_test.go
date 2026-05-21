package tickets

import (
	"strings"
	"testing"
	"time"
)

func TestAllowedTransitions(t *testing.T) {
	cases := []struct {
		from, to string
		trigger  Trigger
	}{
		{"open", "in_progress", TriggerAdminReply},
		{"in_progress", "waiting_customer", TriggerAdminStatus},
		{"waiting_customer", "in_progress", TriggerCustomerReply},
		{"in_progress", "resolved", TriggerAdminStatus},
		{"waiting_customer", "resolved", TriggerAdminStatus},
		{"resolved", "in_progress", TriggerCustomerReopen},
		{"resolved", "closed", TriggerCronIdle},
		{"waiting_customer", "closed", TriggerCronIdle},
		{"open", "closed", TriggerAdminStatus},
		{"in_progress", "closed", TriggerAdminStatus},
	}
	for _, c := range cases {
		if err := AllowTransition(c.from, c.to, c.trigger, time.Now()); err != nil {
			t.Errorf("AllowTransition(%s -> %s via %v) = %v, want nil", c.from, c.to, c.trigger, err)
		}
	}
}

func TestForbiddenTransitionsAreRejected(t *testing.T) {
	cases := []struct{ from, to string }{
		{"closed", "in_progress"},
		{"closed", "open"},
		{"open", "resolved"},
		{"open", "waiting_customer"},
		{"waiting_customer", "open"},
		{"resolved", "waiting_customer"},
	}
	for _, c := range cases {
		err := AllowTransition(c.from, c.to, TriggerAdminStatus, time.Now())
		if err == nil {
			t.Errorf("AllowTransition(%s -> %s) accepted; want rejected", c.from, c.to)
		}
	}
}

func TestReopenWindowEnforced(t *testing.T) {
	recent := time.Now().Add(-6 * 24 * time.Hour)
	old := time.Now().Add(-8 * 24 * time.Hour)

	if err := AllowReopen(recent); err != nil {
		t.Errorf("AllowReopen(6d ago) = %v, want nil", err)
	}
	err := AllowReopen(old)
	if err == nil {
		t.Errorf("AllowReopen(8d ago) accepted; want rejected")
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("AllowReopen error %q should mention 7-day window", err)
	}
}

func TestUnknownStatusRejected(t *testing.T) {
	if err := AllowTransition("open", "frobnicated", TriggerAdminStatus, time.Now()); err == nil {
		t.Errorf("unknown target accepted")
	}
}
