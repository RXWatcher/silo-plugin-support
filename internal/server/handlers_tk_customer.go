package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
	"github.com/ContinuumApp/continuum-plugin-support/internal/tickets"
)

func tkCustomerStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// SPA shell handlers.
func hTKListPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-list", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hTKDetailPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-detail", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hTKNewPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-new", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
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
			CustomerID:  r.Header.Get("X-Continuum-User-Id"),
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
		if t.CustomerID != r.Header.Get("X-Continuum-User-Id") {
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
		if strings.TrimSpace(req.CustomerEmail) == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_email", "customerEmail is required")
			return
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
			CustomerID:     r.Header.Get("X-Continuum-User-Id"),
			CustomerEmail:  req.CustomerEmail,
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
		if t.CustomerID != r.Header.Get("X-Continuum-User-Id") {
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
				tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
					"from": t.Status, "to": "in_progress", "by": "customer",
				})
				t = updated
			}
		}

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
		if t.CustomerID != r.Header.Get("X-Continuum-User-Id") {
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
