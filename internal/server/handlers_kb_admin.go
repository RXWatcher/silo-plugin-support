package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/silo-plugin-support/internal/htmlx"
	"github.com/RXWatcher/silo-plugin-support/internal/kb"
	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

// Admin SPA shell handlers.
func hKBAdminListPage(d Deps) http.HandlerFunc       { return adminSPAHandler(d, "admin-kb-list") }
func hKBAdminEditPage(d Deps) http.HandlerFunc       { return adminSPAHandler(d, "admin-kb-edit") }
func hKBAdminCategoriesPage(d Deps) http.HandlerFunc { return adminSPAHandler(d, "admin-kb-categories") }
func hKBAdminTagsPage(d Deps) http.HandlerFunc       { return adminSPAHandler(d, "admin-kb-tags") }

func adminSPAHandler(d Deps, mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode:    mode,
			Theme:   adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: true,
		}, http.StatusOK)
	}
}

// --- Article CRUD ----------------------------------------------------

type kbArticleRequest struct {
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	BodyHTML   string   `json:"bodyHtml"`
	CategoryID int64    `json:"categoryId"`
	Status     string   `json:"status"`
	PublishAt  *string  `json:"publishAt"`
	TagLabels  []string `json:"tagLabels"`
}

func hKBAdminListArticles(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.KBArticleListFilter{
			Status:     r.URL.Query().Get("status"),
			CategoryID: parseInt64(r.URL.Query().Get("categoryId")),
			TagSlug:    r.URL.Query().Get("tag"),
			TitleQuery: r.URL.Query().Get("q"),
			Limit:      parseLimit(r.URL.Query().Get("limit"), 200),
			Offset:     parseInt(r.URL.Query().Get("offset")),
		}
		out, err := kbAdminStore(d).KBListArticles(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "kb_admin_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func hKBAdminGetArticle(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid article id")
			return
		}
		a, err := kbAdminStore(d).KBGetArticleByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "kb_admin_get_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

func hKBAdminCreateArticle(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { kbWriteArticle(w, r, d, 0) }
}

func hKBAdminUpdateArticle(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid article id")
			return
		}
		kbWriteArticle(w, r, d, id)
	}
}

func kbWriteArticle(w http.ResponseWriter, r *http.Request, d Deps, id int64) {
	var req kbArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
		return
	}
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "bad_title", "title is required")
		return
	}
	if req.CategoryID == 0 {
		writeErr(w, http.StatusBadRequest, "bad_category", "categoryId is required")
		return
	}
	if req.Status != "draft" && req.Status != "published" {
		req.Status = "draft"
	}

	base := req.Slug
	if base == "" {
		base = kb.Slugify(req.Title)
	} else {
		base = kb.Slugify(base)
	}
	if base == "" {
		writeErr(w, http.StatusBadRequest, "bad_slug", "title or slug produced empty slug")
		return
	}
	slug, err := kb.UniqueSlug(base, func(s string) (bool, error) {
		return kbAdminStore(d).KBArticleSlugExists(r.Context(), s, id)
	})
	if err != nil {
		writeInternal(w, r, d, "kb_slug_failed", err)
		return
	}

	bodyHTML := htmlx.Sanitize(req.BodyHTML)
	bodyText := htmlx.ExtractText(bodyHTML)

	tagIDs := []int64{}
	for _, label := range req.TagLabels {
		if label == "" {
			continue
		}
		tagSlug := kb.Slugify(label)
		if tagSlug == "" {
			continue
		}
		existing, gerr := kbAdminStore(d).KBGetTagBySlug(r.Context(), tagSlug)
		if errors.Is(gerr, store.ErrNotFound) {
			t, cerr := kbAdminStore(d).KBCreateTag(r.Context(), tagSlug, label)
			if cerr != nil {
				writeInternal(w, r, d, "kb_tag_create_failed", cerr)
				return
			}
			tagIDs = append(tagIDs, t.ID)
		} else if gerr != nil {
			writeInternal(w, r, d, "kb_tag_lookup_failed", gerr)
			return
		} else {
			tagIDs = append(tagIDs, existing.ID)
		}
	}

	in := store.KBArticle{
		ID:           id,
		Slug:         slug,
		Title:        req.Title,
		Summary:      req.Summary,
		BodyHTML:     bodyHTML,
		BodyText:     bodyText,
		CategoryID:   req.CategoryID,
		Status:       req.Status,
		LastEditedBy: r.Header.Get("X-Silo-User-Id"),
	}
	if req.PublishAt != nil && *req.PublishAt != "" {
		ts, perr := parseTime(*req.PublishAt)
		if perr != nil {
			writeErr(w, http.StatusBadRequest, "bad_publish_at", "publishAt must be an RFC3339 timestamp")
			return
		}
		in.PublishAt = &ts
	}
	if in.Status == "published" {
		now := timeNow()
		in.PublishedAt = &now
	}

	saved, err := kbAdminStore(d).KBSaveArticle(r.Context(), in, tagIDs)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
		return
	}
	if err != nil {
		writeInternal(w, r, d, "kb_save_failed", err)
		return
	}

	if imgIDs := kb.ExtractImageIDs(saved.BodyHTML); len(imgIDs) > 0 {
		_ = kbAdminStore(d).KBAdoptOrphanImages(r.Context(), saved.ID, imgIDs)
	}

	switch {
	case id == 0 && saved.Status == "published":
		// First-publish on create.
		kbPublishEvent(d, "kb_article_published", saved, map[string]any{
			"by": saved.LastEditedBy,
		})
	case id != 0 && saved.Status == "published":
		// Save against an already-published article — spec contracts
		// kb_article_updated for the notifications plugin to route.
		kbPublishEvent(d, "kb_article_updated", saved, map[string]any{
			"changed_by": saved.LastEditedBy,
		})
	}

	writeJSON(w, http.StatusOK, saved)
}

func hKBAdminDeleteArticle(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid article id")
			return
		}
		if err := kbAdminStore(d).KBDeleteArticle(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
				return
			}
			writeInternal(w, r, d, "kb_delete_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hKBAdminPublishArticle(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid article id")
			return
		}
		saved, err := kbAdminStore(d).KBPublishArticle(r.Context(), id, r.Header.Get("X-Silo-User-Id"))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusConflict, "kb_not_draft", err.Error())
			return
		}
		kbPublishEvent(d, "kb_article_published", saved, map[string]any{"by": saved.LastEditedBy})
		writeJSON(w, http.StatusOK, saved)
	}
}

func hKBAdminUnpublishArticle(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid article id")
			return
		}
		saved, err := kbAdminStore(d).KBUnpublishArticle(r.Context(), id, r.Header.Get("X-Silo-User-Id"))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusConflict, "kb_not_published", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hKBAdminEngagement(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid article id")
			return
		}
		votes, err := kbAdminStore(d).KBVoteAggregateFor(r.Context(), id)
		if err != nil {
			writeInternal(w, r, d, "kb_engagement_failed", err)
			return
		}
		views, err := kbAdminStore(d).KBViewAggregate30d(r.Context(), id)
		if err != nil {
			writeInternal(w, r, d, "kb_engagement_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"votes": votes, "views": views})
	}
}

// --- Categories admin ------------------------------------------------

type kbCategoryRequest struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Active    bool   `json:"active"`
}

func hKBAdminListCategories(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all, err := kbAdminStore(d).KBListCategories(r.Context(), false)
		if err != nil {
			writeInternal(w, r, d, "kb_categories_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, all)
	}
}

func hKBAdminCreateCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req kbCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if req.Name == "" {
			writeErr(w, http.StatusBadRequest, "bad_name", "name is required")
			return
		}
		base := req.Slug
		if base == "" {
			base = kb.Slugify(req.Name)
		} else {
			base = kb.Slugify(base)
		}
		if base == "" {
			writeErr(w, http.StatusBadRequest, "bad_slug", "name produced empty slug")
			return
		}
		slug, err := kb.UniqueSlug(base, func(s string) (bool, error) {
			return kbAdminStore(d).KBCategorySlugExists(r.Context(), s)
		})
		if err != nil {
			writeInternal(w, r, d, "kb_slug_failed", err)
			return
		}
		c, err := kbAdminStore(d).KBCreateCategory(r.Context(), slug, req.Name, req.SortOrder)
		if err != nil {
			writeInternal(w, r, d, "kb_category_create_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

func hKBAdminUpdateCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid category id")
			return
		}
		var req kbCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if req.Name == "" {
			writeErr(w, http.StatusBadRequest, "bad_name", "name is required")
			return
		}
		c, err := kbAdminStore(d).KBUpdateCategory(r.Context(), id, req.Name, req.SortOrder, req.Active)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "category not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "kb_category_update_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

func hKBAdminDeleteCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid category id")
			return
		}
		if err := kbAdminStore(d).KBDeleteCategory(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "kb_not_found", "category not found")
				return
			}
			writeErr(w, http.StatusConflict, "kb_category_in_use", "category is in use by one or more articles")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// --- Tags admin ------------------------------------------------------

type kbTagRequest struct {
	Label string `json:"label"`
}

type kbTagMergeRequest struct {
	FromID int64 `json:"fromId"`
	IntoID int64 `json:"intoId"`
}

func hKBAdminListTags(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags, err := kbAdminStore(d).KBListTags(r.Context())
		if err != nil {
			writeInternal(w, r, d, "kb_tags_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, tags)
	}
}

func hKBAdminCreateTag(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req kbTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if req.Label == "" {
			writeErr(w, http.StatusBadRequest, "bad_label", "label is required")
			return
		}
		slug := kb.Slugify(req.Label)
		if slug == "" {
			writeErr(w, http.StatusBadRequest, "bad_slug", "label produced empty slug")
			return
		}
		if exists, _ := kbAdminStore(d).KBGetTagBySlug(r.Context(), slug); exists.ID != 0 {
			writeErr(w, http.StatusConflict, "kb_tag_exists", "a tag with that slug already exists")
			return
		}
		t, err := kbAdminStore(d).KBCreateTag(r.Context(), slug, req.Label)
		if err != nil {
			writeInternal(w, r, d, "kb_tag_create_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func hKBAdminRenameTag(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid tag id")
			return
		}
		var req kbTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if req.Label == "" {
			writeErr(w, http.StatusBadRequest, "bad_label", "label is required")
			return
		}
		t, err := kbAdminStore(d).KBRenameTag(r.Context(), id, req.Label)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "tag not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "kb_tag_rename_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func hKBAdminDeleteTag(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid tag id")
			return
		}
		if err := kbAdminStore(d).KBDeleteTag(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "kb_not_found", "tag not found")
				return
			}
			writeErr(w, http.StatusConflict, "kb_tag_in_use", "tag is in use by one or more articles")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hKBAdminMergeTags(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req kbTagMergeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if req.FromID <= 0 || req.IntoID <= 0 || req.FromID == req.IntoID {
			writeErr(w, http.StatusBadRequest, "bad_merge", "fromId and intoId must be distinct positive ids")
			return
		}
		if err := kbAdminStore(d).KBMergeTags(r.Context(), req.FromID, req.IntoID); err != nil {
			writeInternal(w, r, d, "kb_tag_merge_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// kbAdminStore mirrors kbCustomerStore — admin path needs the same
// concrete store. Duplicated for clarity.
func kbAdminStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// hKBAdminRunCron exposes PublishDue + UnhelpfulSweep as a single
// admin-triggered endpoint. A native scheduled_task.v1 SDK capability
// is a follow-up.
func hKBAdminRunCron(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := &kb.Cron{
			Store:     kbAdminStore(d),
			Publisher: kbEventEmitter{d: d},
		}
		if err := c.PublishDue(r.Context()); err != nil {
			writeInternal(w, r, d, "kb_cron_publish_failed", err)
			return
		}
		if err := c.UnhelpfulSweep(r.Context()); err != nil {
			writeInternal(w, r, d, "kb_cron_unhelpful_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// kbEventEmitter bridges the cron's EventEmitter interface to the
// existing kbPublishEvent helper.
type kbEventEmitter struct{ d Deps }

func (e kbEventEmitter) PublishKBArticleEvent(_ context.Context, name string, a store.KBArticle, extra map[string]any) {
	kbPublishEvent(e.d, name, a, extra)
}
