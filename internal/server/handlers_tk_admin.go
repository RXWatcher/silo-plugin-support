package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
	"github.com/RXWatcher/continuum-plugin-support/internal/tickets"
)

func tkAdminStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// Admin SPA shells.
func hTKAdminQueuePage(d Deps) http.HandlerFunc      { return adminSPAHandler(d, "admin-tickets-queue") }
func hTKAdminDetailPage(d Deps) http.HandlerFunc     { return adminSPAHandler(d, "admin-tickets-detail") }
func hTKAdminCategoriesPage(d Deps) http.HandlerFunc { return adminSPAHandler(d, "admin-tickets-categories") }

// --- Queue ---------------------------------------------------------

func hTKAdminQueue(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.TKTicketListFilter{
			Status:        r.URL.Query().Get("status"),
			StatusGroup:   r.URL.Query().Get("statusGroup"),
			CategoryID:    parseInt64(r.URL.Query().Get("categoryId")),
			AssigneeID:    r.URL.Query().Get("assignee"),
			CallerAdminID: r.Header.Get("X-Continuum-User-Id"),
			Search:        r.URL.Query().Get("q"),
			Limit:         parseLimit(r.URL.Query().Get("limit"), 200),
			Offset:        parseInt(r.URL.Query().Get("offset")),
		}
		out, err := tkAdminStore(d).TKListTickets(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "tk_queue_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func hTKAdminDetail(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err)
			return
		}
		if err := st.TKLoadTicketAux(r.Context(), &t); err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err)
			return
		}
		entries, err := st.TKListEntries(r.Context(), t.ID, false)
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ticket": t, "entries": entries})
	}
}

func hTKAdminReply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err)
			return
		}
		if t.Status == "closed" {
			writeErr(w, http.StatusConflict, "tk_closed", "ticket is closed")
			return
		}
		var req struct{ Body string `json:"body"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Body) == "" {
			writeErr(w, http.StatusBadRequest, "tk_empty_body", "reply body cannot be empty")
			return
		}
		adminID := r.Header.Get("X-Continuum-User-Id")
		entry, err := st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "reply", AuthorID: adminID, AuthorRole: "admin", Body: req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err)
			return
		}
		if t.Status == "open" {
			if err := tickets.AllowTransition(t.Status, "in_progress", tickets.TriggerAdminReply, timeNow()); err == nil {
				updated, _ := st.TKUpdateTicketStatus(r.Context(), t.ID, "in_progress", nil, nil)
				tkEnrichForEvent(r.Context(), d, &updated)
				tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
					"from": t.Status, "to": "in_progress", "by": adminID,
				})
				t = updated
			}
		}
		tkEnrichForEvent(r.Context(), d, &t)
		tkPublishEvent(d, "ticket_replied", t, map[string]any{
			"author_role": "admin", "author_id": adminID, "excerpt": excerpt(req.Body, 280),
		})
		writeJSON(w, http.StatusOK, map[string]any{"entry": entry, "ticket": t})
	}
}

func hTKAdminNote(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_note_failed", err)
			return
		}
		var req struct{ Body string `json:"body"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Body) == "" {
			writeErr(w, http.StatusBadRequest, "tk_empty_body", "note body cannot be empty")
			return
		}
		entry, err := st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "internal_note",
			AuthorID: r.Header.Get("X-Continuum-User-Id"), AuthorRole: "admin", Body: req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_note_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, entry)
	}
}

func hTKAdminStatus(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_status_failed", err)
			return
		}
		var req struct{ To string `json:"to"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if err := tickets.AllowTransition(t.Status, req.To, tickets.TriggerAdminStatus, timeNow()); err != nil {
			writeErr(w, http.StatusConflict, "tk_bad_transition", err.Error())
			return
		}
		adminID := r.Header.Get("X-Continuum-User-Id")
		updated, err := st.TKUpdateTicketStatus(r.Context(), t.ID, req.To, nil, nil)
		if err != nil {
			writeInternal(w, r, d, "tk_status_failed", err)
			return
		}
		_, _ = st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "status_change", AuthorID: adminID, AuthorRole: "admin",
			Body: "Status changed: " + t.Status + " → " + req.To,
		})
		tkEnrichForEvent(r.Context(), d, &updated)
		tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
			"from": t.Status, "to": req.To, "by": adminID,
		})
		if req.To == "resolved" {
			tkPublishEvent(d, "ticket_resolved", updated, map[string]any{"by": adminID})
		}
		if req.To == "closed" {
			tkPublishEvent(d, "ticket_closed", updated, map[string]any{"by": adminID})
		}
		writeJSON(w, http.StatusOK, updated)
	}
}

func hTKAdminAssign(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_assign_failed", err)
			return
		}
		var req struct{ AdminID *string `json:"adminId"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		updated, err := st.TKAssignTicket(r.Context(), t.ID, req.AdminID)
		if err != nil {
			writeInternal(w, r, d, "tk_assign_failed", err)
			return
		}
		tkEnrichForEvent(r.Context(), d, &updated)
		tkPublishEvent(d, "ticket_assigned", updated, map[string]any{
			"from_admin_id": t.AssignedAdminID, "to_admin_id": req.AdminID,
		})
		writeJSON(w, http.StatusOK, updated)
	}
}

func hTKAdminRunCron(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := &tickets.Cron{
			Store:             tkAdminStore(d),
			Emitter:           tkEventEmitter{d: d},
			Enabled:           d.TKAutoCloseEnabled,
			ResolvedAfterDays: d.TKResolvedCloseAfterDays,
			WaitingAfterDays:  d.TKWaitingCloseAfterDays,
		}
		if err := c.CloseIdle(r.Context()); err != nil {
			writeInternal(w, r, d, "tk_cron_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

type tkEventEmitter struct{ d Deps }

func (e tkEventEmitter) PublishTicketEvent(_ context.Context, name string, t store.TKTicket, extra map[string]any) {
	tkPublishEvent(e.d, name, t, extra)
}

// --- Categories admin ----------------------------------------------

type tkCategoryRequest struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Active    bool   `json:"active"`
}

func hTKAdminListCategoriesAdmin(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cats, err := tkAdminStore(d).TKListCategories(r.Context(), false)
		if err != nil { writeInternal(w, r, d, "tk_categories_list_failed", err); return }
		writeJSON(w, http.StatusOK, cats)
	}
}

func hTKAdminCreateCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if req.Slug == "" || req.Name == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_cat", "slug and name required"); return
		}
		saved, err := tkAdminStore(d).TKCreateCategory(r.Context(), store.TKCategory{
			Slug: req.Slug, Name: req.Name, SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil { writeInternal(w, r, d, "tk_cat_create_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminUpdateCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		var req tkCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		saved, err := tkAdminStore(d).TKUpdateCategory(r.Context(), store.TKCategory{
			ID: id, Name: req.Name, SortOrder: req.SortOrder, Active: req.Active,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "category not found"); return
		}
		if err != nil { writeInternal(w, r, d, "tk_cat_update_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminDeleteCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		if err := tkAdminStore(d).TKDeleteCategory(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "tk_not_found", "category not found"); return
			}
			writeErr(w, http.StatusConflict, "tk_cat_in_use", "category is in use"); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// --- Subcategories admin ------------------------------------------

type tkSubcategoryRequest struct {
	CategoryID int64  `json:"categoryId"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	SortOrder  int    `json:"sortOrder"`
	Active     bool   `json:"active"`
}

func hTKAdminListSubcategories(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID := parseInt64(r.URL.Query().Get("categoryId"))
		if categoryID == 0 {
			writeErr(w, http.StatusBadRequest, "bad_query", "categoryId is required"); return
		}
		subs, err := tkAdminStore(d).TKListSubcategories(r.Context(), categoryID, false)
		if err != nil { writeInternal(w, r, d, "tk_subs_list_failed", err); return }
		writeJSON(w, http.StatusOK, subs)
	}
}

func hTKAdminCreateSubcategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkSubcategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if req.CategoryID == 0 || req.Slug == "" || req.Name == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_sub", "categoryId, slug, name required"); return
		}
		saved, err := tkAdminStore(d).TKCreateSubcategory(r.Context(), store.TKSubcategory{
			CategoryID: req.CategoryID, Slug: req.Slug, Name: req.Name,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil { writeInternal(w, r, d, "tk_sub_create_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminUpdateSubcategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		var req tkSubcategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		saved, err := tkAdminStore(d).TKUpdateSubcategory(r.Context(), store.TKSubcategory{
			ID: id, Name: req.Name, SortOrder: req.SortOrder, Active: req.Active,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "subcategory not found"); return
		}
		if err != nil { writeInternal(w, r, d, "tk_sub_update_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminDeleteSubcategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		if err := tkAdminStore(d).TKDeleteSubcategory(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "tk_not_found", "subcategory not found"); return
			}
			writeErr(w, http.StatusConflict, "tk_sub_in_use", "subcategory is in use"); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// --- Category fields admin ----------------------------------------

type tkFieldRequest struct {
	CategoryID int64  `json:"categoryId"`
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Required   bool   `json:"required"`
	SortOrder  int    `json:"sortOrder"`
}

func hTKAdminListFields(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID := parseInt64(r.URL.Query().Get("categoryId"))
		fields, err := tkAdminStore(d).TKListCategoryFields(r.Context(), categoryID)
		if err != nil { writeInternal(w, r, d, "tk_fields_list_failed", err); return }
		writeJSON(w, http.StatusOK, fields)
	}
}

func hTKAdminCreateField(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkFieldRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		validKinds := map[string]bool{"text": true, "textarea": true, "number": true, "url": true}
		if !validKinds[req.Kind] || req.CategoryID == 0 || req.Key == "" || req.Label == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_field", "categoryId, key, label, and a valid kind required"); return
		}
		saved, err := tkAdminStore(d).TKCreateCategoryField(r.Context(), store.TKCategoryField{
			CategoryID: req.CategoryID, Key: req.Key, Label: req.Label, Kind: req.Kind,
			Required: req.Required, SortOrder: req.SortOrder,
		})
		if err != nil { writeInternal(w, r, d, "tk_field_create_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminUpdateField(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		var req tkFieldRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		saved, err := tkAdminStore(d).TKUpdateCategoryField(r.Context(), store.TKCategoryField{
			ID: id, Label: req.Label, Kind: req.Kind, Required: req.Required, SortOrder: req.SortOrder,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "field not found"); return
		}
		if err != nil { writeInternal(w, r, d, "tk_field_update_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminDeleteField(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		if err := tkAdminStore(d).TKDeleteCategoryField(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "tk_not_found", "field not found"); return
			}
			writeInternal(w, r, d, "tk_field_delete_failed", err); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
