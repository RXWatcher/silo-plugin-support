package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
	"github.com/RXWatcher/silo-plugin-support/internal/tickets"
)

// tkCustomerActionMinInterval is the minimum spacing between
// customer-originated ticket actions (create / reply). Mirrors the
// speedtest module's per-customer guard.
const tkCustomerActionMinInterval = 20 * time.Second

// tkRateLimit enforces a per-customer minimum interval between ticket
// create/reply actions. Returns true if the request was rate limited
// (and an error response has already been written).
func tkRateLimit(w http.ResponseWriter, r *http.Request, d Deps, customerID string) bool {
	st := tkCustomerStore(d)
	if st == nil {
		return false
	}
	last, err := st.TKLastCustomerActionAt(r.Context(), customerID)
	if err != nil {
		writeInternal(w, r, d, "tk_rate_check_failed", err)
		return true
	}
	if !last.IsZero() {
		if since := time.Since(last); since < tkCustomerActionMinInterval {
			retryIn := int((tkCustomerActionMinInterval - since).Seconds()) + 1
			writeErr(w, http.StatusTooManyRequests, "tk_rate_limited",
				"please wait "+ts(retryIn)+" before submitting another message")
			return true
		}
	}
	return false
}

// tkValidateBody enforces the configured min/max length on a free-text
// entry body (counted in runes, not bytes, so multibyte input is judged
// by character count). It returns false and writes an error response if
// the body is too short or too long. minChars <= 0 disables the minimum;
// it is only applied on initial ticket creation where a meaningful
// description is required — replies/notes pass minChars = 0.
func tkValidateBody(w http.ResponseWriter, d Deps, body string, applyMin bool) bool {
	_, minBody, maxBody, _, _ := d.tkLimits()
	n := utf8.RuneCountInString(strings.TrimSpace(body))
	if applyMin && minBody > 0 && n < minBody {
		writeErr(w, http.StatusBadRequest, "tk_body_too_short",
			"message must be at least "+strconv.Itoa(minBody)+" characters")
		return false
	}
	if maxBody > 0 && n > maxBody {
		writeErr(w, http.StatusRequestEntityTooLarge, "tk_body_too_long",
			"message must be "+strconv.Itoa(maxBody)+" characters or fewer")
		return false
	}
	return true
}

// validCustomerEmail validates that s parses as a single RFC 5322
// address. The address is treated purely as untrusted display data;
// it is never used as an identity key or notification target.
func validCustomerEmail(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s
}

func tkCustomerStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// tkEnrichForEvent fills Category and Subcategory on the ticket so the event
// payload includes them. Best-effort — a failed aux load publishes a partial
// payload rather than blocking the request.
func tkEnrichForEvent(ctx context.Context, d Deps, t *store.TKTicket) {
	st := tkCustomerStore(d)
	if st == nil || t == nil {
		return
	}
	_ = st.TKLoadTicketAux(ctx, t)
}

// SPA shell handlers.
func hTKListPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-list", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: r.Header.Get("X-Silo-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hTKDetailPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-detail", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: r.Header.Get("X-Silo-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hTKNewPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-new", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: r.Header.Get("X-Silo-User-Role") == "admin",
		}, http.StatusOK)
	}
}

// --- Categories (form rendering) -----------------------------------

type tkCategoriesResponse struct {
	Categories    []store.TKCategory                `json:"categories"`
	Subcategories map[int64][]store.TKSubcategory   `json:"subcategories"`
	Fields        map[int64][]store.TKCategoryField `json:"fields"`
}

func hTKCategoriesForCustomer(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st := tkCustomerStore(d)
		cats, err := st.TKListCategories(r.Context(), true)
		if err != nil {
			writeInternal(w, r, d, "tk_categories_failed", err)
			return
		}
		out := tkCategoriesResponse{
			Categories:    cats,
			Subcategories: map[int64][]store.TKSubcategory{},
			Fields:        map[int64][]store.TKCategoryField{},
		}
		for _, c := range cats {
			subs, err := st.TKListSubcategories(r.Context(), c.ID, true)
			if err == nil {
				out.Subcategories[c.ID] = subs
			}
			fields, err := st.TKListCategoryFields(r.Context(), c.ID)
			if err == nil {
				out.Fields[c.ID] = fields
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// --- Customer ticket list ------------------------------------------

func hTKCustomerList(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.TKTicketListFilter{
			CustomerID:  r.Header.Get("X-Silo-User-Id"),
			StatusGroup: r.URL.Query().Get("statusGroup"),
			Limit:       parseLimit(r.URL.Query().Get("limit"), 100),
			Offset:      parseInt(r.URL.Query().Get("offset")),
		}
		out, err := tkCustomerStore(d).TKListTickets(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "tk_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// --- Customer ticket detail (excludes internal notes) --------------

type tkDetailResponse struct {
	Ticket  store.TKTicket  `json:"ticket"`
	Entries []store.TKEntry `json:"entries"`
}

func hTKCustomerDetail(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkCustomerStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err)
			return
		}
		if t.CustomerID != r.Header.Get("X-Silo-User-Id") {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err := st.TKLoadTicketAux(r.Context(), &t); err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err)
			return
		}
		entries, err := st.TKListEntries(r.Context(), t.ID, true)
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, tkDetailResponse{Ticket: t, Entries: entries})
	}
}

// --- Customer creates a ticket -------------------------------------

type tkCreateRequest struct {
	CategoryID    int64             `json:"categoryId"`
	SubcategoryID *int64            `json:"subcategoryId,omitempty"`
	Subject       string            `json:"subject"`
	Body          string            `json:"body"`
	FieldValues   map[string]string `json:"fieldValues,omitempty"`
	CustomerEmail string            `json:"customerEmail"`
}

func hTKCustomerCreate(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Body) == "" || req.CategoryID == 0 {
			writeErr(w, http.StatusBadRequest, "tk_bad_ticket", "subject, body, and categoryId are required")
			return
		}
		if !tkValidateBody(w, d, req.Body, true) {
			return
		}

		customerID := r.Header.Get("X-Silo-User-Id")
		if tkRateLimit(w, r, d, customerID) {
			return
		}

		// Per-customer open-ticket cap: reject creating a new ticket when
		// the customer already has the configured number of non-terminal
		// tickets. Curbs ticket-flooding beyond the per-action rate limit.
		if maxOpen, _, _, _, _ := d.tkLimits(); maxOpen > 0 {
			open, err := tkCustomerStore(d).TKCountOpenTicketsForCustomer(r.Context(), customerID)
			if err != nil {
				writeInternal(w, r, d, "tk_open_count_failed", err)
				return
			}
			if open >= maxOpen {
				writeErr(w, http.StatusTooManyRequests, "tk_too_many_open",
					"you have too many open tickets; please wait for an existing ticket to be resolved")
				return
			}
		}

		// Prefer a host-provided email (trusted, tied to the authenticated
		// identity) when available. Fall back to the client-supplied value
		// only as untrusted display data, and only if it is well-formed.
		// The customer email is never used as an identity key or
		// notification target.
		customerEmail := strings.TrimSpace(r.Header.Get("X-Silo-User-Email"))
		if customerEmail == "" || !validCustomerEmail(customerEmail) {
			clientEmail := strings.TrimSpace(req.CustomerEmail)
			if clientEmail == "" {
				writeErr(w, http.StatusBadRequest, "tk_bad_email", "customerEmail is required")
				return
			}
			if !validCustomerEmail(clientEmail) {
				writeErr(w, http.StatusBadRequest, "tk_bad_email", "customerEmail is not a valid email address")
				return
			}
			customerEmail = clientEmail
		}

		st := tkCustomerStore(d)

		fields, err := st.TKListCategoryFields(r.Context(), req.CategoryID)
		if err != nil {
			writeInternal(w, r, d, "tk_categories_failed", err)
			return
		}
		for _, f := range fields {
			if f.Required && strings.TrimSpace(req.FieldValues[f.Key]) == "" {
				writeErr(w, http.StatusBadRequest, "tk_missing_field", "required field missing: "+f.Key)
				return
			}
		}

		tn, err := st.TKNextTrackingNumber(r.Context())
		if err != nil {
			writeInternal(w, r, d, "tk_tn_failed", err)
			return
		}

		tx, err := st.TKBegin(r.Context())
		if err != nil {
			writeInternal(w, r, d, "tk_tx_failed", err)
			return
		}
		defer tx.Rollback(r.Context())

		saved, err := st.TKCreateTicket(r.Context(), tx, store.TKTicket{
			TrackingNumber: tn,
			CustomerID:     customerID,
			CustomerEmail:  customerEmail,
			CategoryID:     req.CategoryID,
			SubcategoryID:  req.SubcategoryID,
			Subject:        req.Subject,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_create_failed", err)
			return
		}

		_, err = st.TKInsertEntry(r.Context(), tx, store.TKEntry{
			TicketID:   saved.ID,
			Kind:       "initial",
			AuthorID:   saved.CustomerID,
			AuthorRole: "customer",
			Body:       req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_entry_failed", err)
			return
		}

		for _, f := range fields {
			if v, ok := req.FieldValues[f.Key]; ok && v != "" {
				if err := st.TKInsertFieldValue(r.Context(), tx, saved.ID, f.ID, v); err != nil {
					writeInternal(w, r, d, "tk_field_value_failed", err)
					return
				}
			}
		}

		if err := tx.Commit(r.Context()); err != nil {
			writeInternal(w, r, d, "tk_commit_failed", err)
			return
		}

		tkEnrichForEvent(r.Context(), d, &saved)
		tkPublishEvent(d, "ticket_submitted", saved, nil)
		writeJSON(w, http.StatusOK, saved)
	}
}

// --- Customer reply ------------------------------------------------

type tkReplyRequest struct {
	Body string `json:"body"`
}

func hTKCustomerReply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkCustomerStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err)
			return
		}
		if t.CustomerID != r.Header.Get("X-Silo-User-Id") {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if t.Status == "closed" {
			writeErr(w, http.StatusConflict, "tk_closed", "ticket is closed; please open a new one")
			return
		}
		var req tkReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Body) == "" {
			writeErr(w, http.StatusBadRequest, "tk_empty_body", "reply body cannot be empty")
			return
		}
		if !tkValidateBody(w, d, req.Body, false) {
			return
		}

		if tkRateLimit(w, r, d, t.CustomerID) {
			return
		}

		entry, err := st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID:   t.ID,
			Kind:       "reply",
			AuthorID:   t.CustomerID,
			AuthorRole: "customer",
			Body:       req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err)
			return
		}

		if t.Status == "waiting_customer" {
			if err := tickets.AllowTransition(t.Status, "in_progress", tickets.TriggerCustomerReply, timeNow()); err == nil {
				updated, _ := st.TKUpdateTicketStatus(r.Context(), t.ID, "in_progress", nil, nil)
				tkEnrichForEvent(r.Context(), d, &updated)
				tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
					"from": t.Status, "to": "in_progress", "by": "customer",
				})
				t = updated
			}
		}

		tkEnrichForEvent(r.Context(), d, &t)
		tkPublishEvent(d, "ticket_replied", t, map[string]any{
			"author_role": "customer", "author_id": t.CustomerID,
			"excerpt": excerpt(req.Body, 280),
		})

		writeJSON(w, http.StatusOK, map[string]any{"entry": entry, "ticket": t})
	}
}

// --- Customer reopen -----------------------------------------------

func hTKCustomerReopen(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkCustomerStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_reopen_failed", err)
			return
		}
		if t.CustomerID != r.Header.Get("X-Silo-User-Id") {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err := tickets.AllowTransition(t.Status, "in_progress", tickets.TriggerCustomerReopen, timeNow()); err != nil {
			writeErr(w, http.StatusConflict, "tk_reopen_denied", err.Error())
			return
		}
		if t.ResolvedAt == nil {
			writeErr(w, http.StatusConflict, "tk_reopen_denied", "ticket has no resolved_at")
			return
		}
		if err := tickets.AllowReopen(*t.ResolvedAt); err != nil {
			writeErr(w, http.StatusConflict, "tk_reopen_window", err.Error())
			return
		}
		updated, err := st.TKUpdateTicketStatus(r.Context(), t.ID, "in_progress", nil, nil)
		if err != nil {
			writeInternal(w, r, d, "tk_reopen_failed", err)
			return
		}
		_, _ = st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "system", AuthorID: "system", AuthorRole: "system",
			Body: "Reopened by customer",
		})
		tkEnrichForEvent(r.Context(), d, &updated)
		tkPublishEvent(d, "ticket_reopened", updated, map[string]any{"by": "customer"})
		tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
			"from": "resolved", "to": "in_progress", "by": "customer",
		})
		writeJSON(w, http.StatusOK, updated)
	}
}

// excerpt is a tiny helper for event payloads.
func excerpt(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
