package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// hKBBrowsePage renders the customer SPA shell in browse mode.
func hKBBrowsePage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode:    "kb-browse",
			Theme:   adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}

// hKBDetailPage renders the customer SPA shell in detail mode.
// The slug travels in the URL; the SPA fetches the article via the
// JSON API after first paint.
func hKBDetailPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode:    "kb-detail",
			Theme:   adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}

// hKBCustomerList returns published articles, filterable by category
// + tag. Used by the browse page to populate per-category sections.
func hKBCustomerList(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.KBArticleListFilter{
			Status:  "published",
			TagSlug: r.URL.Query().Get("tag"),
		}
		if cat := r.URL.Query().Get("category"); cat != "" {
			c, err := kbCustomerStore(d).KBGetCategory(r.Context(), parseInt64(cat))
			if err == nil {
				f.CategoryID = c.ID
			}
		}
		f.Limit = parseLimit(r.URL.Query().Get("limit"), 100)
		f.Offset = parseInt(r.URL.Query().Get("offset"))
		out, err := kbCustomerStore(d).KBListArticles(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "kb_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// hKBCustomerDetail returns a single published article + records a
// view row (deduped per 24h).
func hKBCustomerDetail(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		article, err := kbCustomerStore(d).KBGetArticleBySlug(r.Context(), slug, true)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "kb_detail_failed", err)
			return
		}
		// Record view (best-effort — failure doesn't break the page).
		_, _ = kbCustomerStore(d).KBRecordView(r.Context(), article.ID,
			r.Header.Get("X-Continuum-User-Id"))
		writeJSON(w, http.StatusOK, article)
	}
}

// hKBCustomerRelated returns up to 3 related articles by slug.
func hKBCustomerRelated(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		article, err := kbCustomerStore(d).KBGetArticleBySlug(r.Context(), slug, true)
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusOK, []store.KBArticleSummary{})
			return
		}
		if err != nil {
			writeInternal(w, r, d, "kb_related_failed", err)
			return
		}
		related, err := kbCustomerStore(d).KBRelatedArticles(r.Context(), article.ID, 3)
		if err != nil {
			writeInternal(w, r, d, "kb_related_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, related)
	}
}

// hKBCustomerSearch runs the FTS query.
func hKBCustomerSearch(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			writeJSON(w, http.StatusOK, []store.KBSearchHit{})
			return
		}
		hits, err := kbCustomerStore(d).KBSearchArticles(r.Context(), q, 20)
		if err != nil {
			writeInternal(w, r, d, "kb_search_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, hits)
	}
}

// hKBCustomerVote upserts a vote for the calling customer.
func hKBCustomerVote(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		article, err := kbCustomerStore(d).KBGetArticleBySlug(r.Context(), slug, true)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "kb_vote_failed", err)
			return
		}
		var body struct {
			Vote string `json:"vote"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if body.Vote != "up" && body.Vote != "down" {
			writeErr(w, http.StatusBadRequest, "bad_vote", "vote must be 'up' or 'down'")
			return
		}
		if err := kbCustomerStore(d).KBUpsertVote(r.Context(), article.ID,
			r.Header.Get("X-Continuum-User-Id"), body.Vote); err != nil {
			writeInternal(w, r, d, "kb_vote_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"vote": body.Vote})
	}
}

// kbCustomerStore unwraps Deps.ConfigStore into the concrete *store.Store
// so KB-specific methods are reachable. The shell's ConfigStore
// interface only exposes config-CRUD; KB needs the wider surface, so
// it bypasses the interface here.
//
// In production main.go always wires a *store.Store. Tests that don't
// supply one are tests for non-KB handlers and never reach here.
func kbCustomerStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// parseLimit / parseInt / parseInt64 — tiny helpers to keep handlers
// readable. parseLimit clamps to [1, max].
func parseLimit(s string, max int) int {
	n := parseInt(s)
	if n <= 0 || n > max {
		return max
	}
	return n
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func parseInt64(s string) int64 {
	return int64(parseInt(s))
}
