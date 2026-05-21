// Package tickets holds the lifecycle transition map, the cron pass,
// and event-payload assembly helpers — the bits that aren't store
// or HTTP handlers.
package tickets

import (
	"fmt"
	"time"
)

// Trigger names the cause of a transition.
type Trigger int

const (
	TriggerAdminReply Trigger = iota
	TriggerAdminStatus
	TriggerCustomerReply
	TriggerCustomerReopen
	TriggerCronIdle
)

// ReopenWindow is the spec-defined limit on customer reopens.
const ReopenWindow = 7 * 24 * time.Hour

// Map of allowed transitions. Anything not here is forbidden.
var allowed = map[string]map[string]map[Trigger]bool{
	"open": {
		"in_progress": {TriggerAdminReply: true},
		"closed":      {TriggerAdminStatus: true},
	},
	"in_progress": {
		"waiting_customer": {TriggerAdminStatus: true},
		"resolved":         {TriggerAdminStatus: true},
		"closed":           {TriggerAdminStatus: true},
	},
	"waiting_customer": {
		"in_progress": {TriggerCustomerReply: true},
		"resolved":    {TriggerAdminStatus: true},
		"closed":      {TriggerAdminStatus: true, TriggerCronIdle: true},
	},
	"resolved": {
		"in_progress": {TriggerCustomerReopen: true},
		"closed":      {TriggerAdminStatus: true, TriggerCronIdle: true},
	},
}

// AllowTransition validates a status change. Returns nil if allowed,
// otherwise a descriptive error suitable for the handler to map to 409.
func AllowTransition(from, to string, trigger Trigger, _ time.Time) error {
	if from == to {
		return fmt.Errorf("ticket already in status %q", to)
	}
	tos, ok := allowed[from]
	if !ok {
		return fmt.Errorf("ticket in unknown or terminal status %q", from)
	}
	triggers, ok := tos[to]
	if !ok {
		return fmt.Errorf("transition %s -> %s is not allowed", from, to)
	}
	if !triggers[trigger] {
		return fmt.Errorf("transition %s -> %s not allowed via this trigger", from, to)
	}
	return nil
}

// AllowReopen enforces the 7-day customer-reopen window.
func AllowReopen(resolvedAt time.Time) error {
	if resolvedAt.IsZero() {
		return fmt.Errorf("ticket has no resolved_at; cannot reopen")
	}
	if time.Since(resolvedAt) > ReopenWindow {
		return fmt.Errorf("reopen window (7 days) has elapsed; please open a new ticket")
	}
	return nil
}
