# Support Plugin — KB Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Knowledge Base module to `continuum-plugin-support` — operator-authored articles with Tiptap WYSIWYG + inline images, Postgres FTS + related articles, per-customer thumbs + view tracking, scheduled publishing, slug-based URLs, events out via the existing notifications plugin.

**Architecture:** Extends the support plugin shell (no new binary). Six new tables under the `support` schema. New routes added to the existing manifest. Body stored as sanitised HTML (Tiptap-native) with a derived `body_text` column powering FTS. SPA gains 6 new bootstrap modes; admin gets a Tiptap editor + taxonomy admin.

**Tech Stack:** Go 1.26 (existing). New: `github.com/microcosm-cc/bluemonday` for server-side HTML sanitisation. Frontend new: `@tiptap/react`, `@tiptap/starter-kit`, `@tiptap/extension-image`, `@tiptap/extension-link`, `isomorphic-dompurify`. Postgres FTS via generated `tsvector` column. Everything else is the same stack as the shell.

**Spec:** [`../specs/2026-05-21-support-kb-design.md`](../specs/2026-05-21-support-kb-design.md)
**Predecessor:** Shell plan executed in commits `c67b4e2..136e43a` on `main`.

---

## File Structure

All paths relative to `/opt/continuum_plugins/continuum-plugin-support/`.

### Go side

| File | Responsibility |
|---|---|
| `internal/migrate/files/0002_kb_init.up.sql` + `.down.sql` | Create the six kb_* tables + indexes |
| `internal/store/kb_types.go` | Go types for KB rows (Article, Category, Tag, Image, Vote, ViewAggregate, etc.) |
| `internal/store/kb_categories.go` | Categories CRUD |
| `internal/store/kb_tags.go` | Tags CRUD + auto-create-by-label + merge |
| `internal/store/kb_articles.go` | Article CRUD + lifecycle + slug generation/collision |
| `internal/store/kb_search.go` | FTS query, related-articles query |
| `internal/store/kb_engagement.go` | Vote upsert + view 24h dedup + aggregates |
| `internal/store/kb_images.go` | Image insert + read |
| `internal/htmlx/htmlx.go` | HTML sanitisation (bluemonday) + plain-text extraction |
| `internal/htmlx/htmlx_test.go` | Sanitisation + text-extraction tests |
| `internal/server/handlers_kb_customer.go` | Customer KB API + SPA shell handlers |
| `internal/server/handlers_kb_admin.go` | Admin KB API + SPA shell handlers |
| `internal/server/handlers_kb_images.go` | Image upload + serve |
| `internal/server/kb_events.go` | Event publisher helpers (article_published / updated / unhelpful) |
| `internal/server/server.go` | Add KB routes to the chi router |
| `internal/server/spa.go` | Add new bootstrap modes to `supportBootstrap` |
| `internal/server/server_test.go` | Auth-gate sweeps over new KB routes |
| `internal/kb/cron.go` | PublishDue + UnhelpfulSweep |
| `internal/kb/cron_test.go` | Cron logic tests |
| `cmd/continuum-plugin-support/main.go` | Wire the cron (admin button fallback v1) |
| `cmd/continuum-plugin-support/manifest.json` | Add KB http_routes + bump version to 0.2.0 |
| `internal/runtime/runtime.go` | Flip `DefaultAppConfig().Modules.KB` to `true` |

### Web side

| File | Responsibility |
|---|---|
| `web/package.json` + `pnpm-lock.yaml` | Add tiptap + isomorphic-dompurify deps |
| `web/src/lib/modules.ts` | Flip `SHIPPED_MODULES.kb` to `true` |
| `web/src/lib/types.ts` | Extend with KB types (`KBArticle`, `KBCategory`, `KBTag`, etc.) |
| `web/src/lib/bootstrap.ts` | Extend `SupportBootstrap.mode` union + parsing |
| `web/src/api/kb.ts` | Customer KB API client |
| `web/src/api/kbAdmin.ts` | Admin KB API client |
| `web/src/components/shared/TrustedHTML.tsx` | DOMPurify-sanitised HTML renderer (copy from public-catalog) |
| `web/src/components/kb/SearchBar.tsx` + test | Debounced search input |
| `web/src/components/kb/TagChips.tsx` + test | Tag-chip rail with URL state |
| `web/src/components/kb/CategoryCard.tsx` | Per-category panel with article previews |
| `web/src/components/kb/ArticleHeader.tsx` | Title + category badge + tag chips + meta |
| `web/src/components/kb/VoteButtons.tsx` + test | Thumbs up/down (sets pressed-in for existing vote) |
| `web/src/components/kb/RelatedArticles.tsx` | "See also" panel |
| `web/src/components/admin/kb/ArticleEditor.tsx` | Tiptap WYSIWYG + image upload + schedule controls |
| `web/src/components/admin/kb/ArticleList.tsx` | Admin article table + filters |
| `web/src/components/admin/kb/CategoryAdmin.tsx` | Category CRUD UI |
| `web/src/components/admin/kb/TagAdmin.tsx` | Tag list + merge UI |
| `web/src/components/admin/kb/EngagementChart.tsx` | Views + helpful% over 30d (simple bar) |
| `web/src/pages/kb/Browse.tsx` | Customer browse page |
| `web/src/pages/kb/Detail.tsx` | Customer detail page |
| `web/src/pages/admin/kb/List.tsx` | Admin article list page |
| `web/src/pages/admin/kb/Edit.tsx` | Admin article editor page |
| `web/src/pages/admin/kb/Categories.tsx` | Admin categories page |
| `web/src/pages/admin/kb/Tags.tsx` | Admin tags page |
| `web/src/App.tsx` | Dispatch 6 new bootstrap modes |

---

## Phase A — Foundation

### Task A1: Migration `0002_kb_init`

**Files:**
- Create: `internal/migrate/files/0002_kb_init.up.sql`
- Create: `internal/migrate/files/0002_kb_init.down.sql`

- [ ] **Step 1: Write the up migration**

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > internal/migrate/files/0002_kb_init.up.sql <<'EOF'
CREATE TABLE kb_categories (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kb_tags (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    label       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kb_articles (
    id              BIGSERIAL PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    body_html       TEXT NOT NULL,
    body_text       TEXT NOT NULL,
    category_id     BIGINT NOT NULL REFERENCES kb_categories(id) ON DELETE RESTRICT,
    status          TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','published')),
    publish_at      TIMESTAMPTZ,
    published_at    TIMESTAMPTZ,
    last_edited_by  TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    search_vector   tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title,'')),     'A') ||
        setweight(to_tsvector('english', coalesce(summary,'')),   'B') ||
        setweight(to_tsvector('english', coalesce(body_text,'')), 'C')
    ) STORED
);
CREATE INDEX kb_articles_search_idx   ON kb_articles USING GIN (search_vector);
CREATE INDEX kb_articles_category_idx ON kb_articles (category_id, status, published_at DESC);
CREATE INDEX kb_articles_schedule_idx ON kb_articles (publish_at) WHERE status = 'draft';

CREATE TABLE kb_article_tags (
    article_id BIGINT NOT NULL REFERENCES kb_articles(id) ON DELETE CASCADE,
    tag_id     BIGINT NOT NULL REFERENCES kb_tags(id)     ON DELETE RESTRICT,
    PRIMARY KEY (article_id, tag_id)
);
CREATE INDEX kb_article_tags_tag_idx ON kb_article_tags(tag_id);

CREATE TABLE kb_images (
    id          BIGSERIAL PRIMARY KEY,
    article_id  BIGINT REFERENCES kb_articles(id) ON DELETE SET NULL,
    filename    TEXT NOT NULL,
    mime        TEXT NOT NULL,
    bytes       BIGINT NOT NULL,
    content     BYTEA NOT NULL,
    sha256      BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kb_votes (
    article_id  BIGINT NOT NULL REFERENCES kb_articles(id) ON DELETE CASCADE,
    customer_id TEXT   NOT NULL,
    vote        TEXT   NOT NULL CHECK (vote IN ('up','down')),
    voted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (article_id, customer_id)
);

CREATE TABLE kb_views (
    id          BIGSERIAL PRIMARY KEY,
    article_id  BIGINT NOT NULL REFERENCES kb_articles(id) ON DELETE CASCADE,
    customer_id TEXT   NOT NULL,
    viewed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX kb_views_article_idx ON kb_views (article_id, viewed_at DESC);
CREATE INDEX kb_views_dedup_idx   ON kb_views (article_id, customer_id, viewed_at DESC);
EOF
```

- [ ] **Step 2: Write the down migration**

```bash
cat > internal/migrate/files/0002_kb_init.down.sql <<'EOF'
DROP TABLE IF EXISTS kb_views;
DROP TABLE IF EXISTS kb_votes;
DROP TABLE IF EXISTS kb_images;
DROP TABLE IF EXISTS kb_article_tags;
DROP TABLE IF EXISTS kb_articles;
DROP TABLE IF EXISTS kb_tags;
DROP TABLE IF EXISTS kb_categories;
EOF
```

- [ ] **Step 3: Verify build still works**

```bash
go build ./...
```

The migration runner picks up `0002_*` automatically via the existing `//go:embed files/*.sql` glob.

- [ ] **Step 4: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/migrate/files/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(migrate): 0002 kb tables"
```

---

### Task A2: KB store types

**Files:**
- Create: `internal/store/kb_types.go`

- [ ] **Step 1: Write the types file**

```bash
cat > internal/store/kb_types.go <<'EOF'
package store

import "time"

// KBCategory mirrors a row in kb_categories.
type KBCategory struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sortOrder"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// KBTag mirrors a row in kb_tags.
type KBTag struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}

// KBArticle mirrors a row in kb_articles. Body is the sanitised
// HTML (source of truth); BodyText is the derived plaintext used by
// the FTS index — exposed for "render search snippet" cases.
type KBArticle struct {
	ID            int64      `json:"id"`
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	Summary       string     `json:"summary"`
	BodyHTML      string     `json:"bodyHtml"`
	BodyText      string     `json:"-"`
	CategoryID    int64      `json:"categoryId"`
	Status        string     `json:"status"`        // "draft" | "published"
	PublishAt     *time.Time `json:"publishAt,omitempty"`
	PublishedAt   *time.Time `json:"publishedAt,omitempty"`
	LastEditedBy  string     `json:"lastEditedBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	Tags          []KBTag    `json:"tags"`
	Category      *KBCategory `json:"category,omitempty"`
}

// KBArticleSummary is the lightweight projection returned by list
// queries (no body, no full category).
type KBArticleSummary struct {
	ID           int64      `json:"id"`
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	Summary      string     `json:"summary"`
	CategoryID   int64      `json:"categoryId"`
	CategoryName string     `json:"categoryName"`
	Status       string     `json:"status"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	Tags         []string   `json:"tags"`
}

// KBImage mirrors a row in kb_images.
type KBImage struct {
	ID        int64
	ArticleID *int64
	Filename  string
	MIME      string
	Bytes     int64
	Content   []byte
	SHA256    []byte
	CreatedAt time.Time
}

// KBVoteAggregate is the per-article rollup the admin engagement
// view consumes.
type KBVoteAggregate struct {
	HelpfulCount    int `json:"helpfulCount"`
	NotHelpfulCount int `json:"notHelpfulCount"`
}

// KBViewAggregate is the per-article rollup over the last 30 days.
type KBViewAggregate struct {
	TotalViews    int `json:"totalViews"`
	UniqueViewers int `json:"uniqueViewers"`
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/kb_types.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): kb row types"
```

---

### Task A3: HTML helpers (`internal/htmlx`)

**Files:**
- Create: `internal/htmlx/htmlx.go`
- Create: `internal/htmlx/htmlx_test.go`

Uses `github.com/microcosm-cc/bluemonday` for HTML sanitisation and `golang.org/x/net/html` for plain-text extraction. Both ship as Go modules. `bluemonday` is a fresh dep.

- [ ] **Step 1: Write the failing tests**

```bash
mkdir -p internal/htmlx
cat > internal/htmlx/htmlx_test.go <<'EOF'
package htmlx

import (
	"strings"
	"testing"
)

func TestSanitizeStripsScriptAndOnHandlers(t *testing.T) {
	dirty := `<p>hello</p><script>alert(1)</script><a href="javascript:alert(2)" onclick="bad()">x</a>`
	clean := Sanitize(dirty)
	if strings.Contains(clean, "<script>") || strings.Contains(clean, "alert") || strings.Contains(clean, "onclick") || strings.Contains(clean, "javascript:") {
		t.Fatalf("sanitize failed; got %q", clean)
	}
	if !strings.Contains(clean, "<p>hello</p>") {
		t.Fatalf("sanitize stripped safe content; got %q", clean)
	}
}

func TestSanitizeAllowsImagesAndLinks(t *testing.T) {
	dirty := `<p>see <a href="/kb/x">x</a> and <img src="/api/kb/images/3" alt="diagram"></p>`
	clean := Sanitize(dirty)
	if !strings.Contains(clean, `href="/kb/x"`) {
		t.Fatalf("sanitize dropped link; got %q", clean)
	}
	if !strings.Contains(clean, `src="/api/kb/images/3"`) {
		t.Fatalf("sanitize dropped image; got %q", clean)
	}
}

func TestExtractTextStripsTagsAndCollapsesWhitespace(t *testing.T) {
	html := `<h1>Title</h1><p>Hello   <strong>world</strong>.</p><ul><li>one</li><li>two</li></ul>`
	got := ExtractText(html)
	want := "Title Hello world. one two"
	if got != want {
		t.Fatalf("ExtractText = %q, want %q", got, want)
	}
}

func TestExtractTextOnEmptyInput(t *testing.T) {
	if got := ExtractText(""); got != "" {
		t.Fatalf("ExtractText(\"\") = %q, want empty", got)
	}
}
EOF
```

- [ ] **Step 2: Run, expect compile fail**

```bash
go test ./internal/htmlx/...
```

Expected: package missing.

- [ ] **Step 3: Write the implementation**

```bash
cat > internal/htmlx/htmlx.go <<'EOF'
// Package htmlx wraps the HTML sanitisation policy used by the KB
// module (and any future module that stores operator-authored HTML).
// One package so the policy lives in exactly one place and changes
// land for every caller atomically.
package htmlx

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var policy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// KB articles can embed plugin-served images.
	p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	// Tiptap may produce these classes for code blocks / inline code.
	p.AllowAttrs("class").OnElements("code", "pre")
	// Headings + lists are already in UGCPolicy.
	return p
}()

// Sanitize returns dirty with disallowed tags / attrs / URL schemes
// removed. Safe to call on any string (including non-HTML). Output
// is HTML-encoded plaintext if input is plaintext.
func Sanitize(dirty string) string {
	return policy.Sanitize(dirty)
}

// ExtractText reduces HTML to whitespace-collapsed plaintext for the
// FTS body_text column. Tags drop out, text nodes survive, runs of
// whitespace become a single space. Empty input yields empty.
func ExtractText(htmlSrc string) string {
	if htmlSrc == "" {
		return ""
	}
	node, err := html.Parse(strings.NewReader(htmlSrc))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(b.String()), " ")
}
EOF
```

- [ ] **Step 4: Add deps + run tests**

```bash
go get github.com/microcosm-cc/bluemonday@latest
go get golang.org/x/net/html
go mod tidy
go test ./internal/htmlx/... -v
```

Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/htmlx/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(htmlx): HTML sanitisation + plaintext extraction"
```

---

## Phase B — Store layer

### Task B1: `kb_categories.go` — CRUD

**Files:**
- Create: `internal/store/kb_categories.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/store/kb_categories.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a row lookup misses. Distinct from
// pgx.ErrNoRows so callers don't depend on the driver layer.
var ErrNotFound = errors.New("not found")

// KBListCategories returns every category, ordered by sort_order
// then name. activeOnly skips soft-deleted rows.
func (s *Store) KBListCategories(ctx context.Context, activeOnly bool) ([]KBCategory, error) {
	q := `SELECT id, slug, name, sort_order, active, created_at, updated_at
	      FROM kb_categories`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(name)`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list kb_categories: %w", err)
	}
	defer rows.Close()
	out := []KBCategory{}
	for rows.Next() {
		var c KBCategory
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan kb_category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// KBGetCategory by id. Returns ErrNotFound if absent.
func (s *Store) KBGetCategory(ctx context.Context, id int64) (KBCategory, error) {
	var c KBCategory
	err := s.pool.QueryRow(ctx,
		`SELECT id, slug, name, sort_order, active, created_at, updated_at
		 FROM kb_categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBCategory{}, ErrNotFound
	}
	if err != nil {
		return KBCategory{}, fmt.Errorf("get kb_category: %w", err)
	}
	return c, nil
}

// KBCreateCategory inserts a new row. Returns the slug-collision-
// suffixed final slug actually used (caller may have provided a
// slug that collided; the handler regenerates it with a numeric
// suffix in that case before calling here, so the typical path is
// "the slug you sent in is the slug you get back").
func (s *Store) KBCreateCategory(ctx context.Context, slug, name string, sortOrder int) (KBCategory, error) {
	var c KBCategory
	err := s.pool.QueryRow(ctx,
		`INSERT INTO kb_categories (slug, name, sort_order)
		 VALUES ($1, $2, $3)
		 RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		slug, name, sortOrder).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return KBCategory{}, fmt.Errorf("insert kb_category: %w", err)
	}
	return c, nil
}

// KBUpdateCategory: rename, reorder, toggle active. Only the
// supplied fields are written; the handler passes through whatever
// the operator changed.
func (s *Store) KBUpdateCategory(ctx context.Context, id int64, name string, sortOrder int, active bool) (KBCategory, error) {
	var c KBCategory
	err := s.pool.QueryRow(ctx,
		`UPDATE kb_categories
		 SET name = $1, sort_order = $2, active = $3, updated_at = NOW()
		 WHERE id = $4
		 RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		name, sortOrder, active, id).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBCategory{}, ErrNotFound
	}
	if err != nil {
		return KBCategory{}, fmt.Errorf("update kb_category: %w", err)
	}
	return c, nil
}

// KBDeleteCategory hard-deletes. Returns the ON-DELETE-RESTRICT
// foreign key violation as a plain error the handler maps to 409.
func (s *Store) KBDeleteCategory(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete kb_category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// KBCategorySlugExists reports whether a slug is already taken.
// Used by the handler's slug-collision suffixing loop.
func (s *Store) KBCategorySlugExists(ctx context.Context, slug string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM kb_categories WHERE slug = $1`, slug).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check kb_category slug: %w", err)
	}
	return true, nil
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/kb_categories.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): kb categories CRUD"
```

---

### Task B2: `kb_tags.go` — CRUD + merge

**Files:**
- Create: `internal/store/kb_tags.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/store/kb_tags.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// KBListTags returns every tag, ordered by label, plus a usage
// count via a LEFT JOIN onto kb_article_tags. The handler exposes
// the count to the admin tags admin page.
func (s *Store) KBListTags(ctx context.Context) ([]KBTagWithCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.slug, t.label, t.created_at,
		       COUNT(at.article_id) AS use_count
		FROM kb_tags t
		LEFT JOIN kb_article_tags at ON at.tag_id = t.id
		GROUP BY t.id
		ORDER BY lower(t.label)`)
	if err != nil {
		return nil, fmt.Errorf("list kb_tags: %w", err)
	}
	defer rows.Close()
	out := []KBTagWithCount{}
	for rows.Next() {
		var t KBTagWithCount
		if err := rows.Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt, &t.UseCount); err != nil {
			return nil, fmt.Errorf("scan kb_tag: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// KBGetTagBySlug finds a tag by its slug; returns ErrNotFound if absent.
func (s *Store) KBGetTagBySlug(ctx context.Context, slug string) (KBTag, error) {
	var t KBTag
	err := s.pool.QueryRow(ctx,
		`SELECT id, slug, label, created_at FROM kb_tags WHERE slug = $1`, slug).
		Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBTag{}, ErrNotFound
	}
	if err != nil {
		return KBTag{}, fmt.Errorf("get kb_tag by slug: %w", err)
	}
	return t, nil
}

// KBCreateTag inserts. Caller has already computed the slug + ensured
// it's free.
func (s *Store) KBCreateTag(ctx context.Context, slug, label string) (KBTag, error) {
	var t KBTag
	err := s.pool.QueryRow(ctx,
		`INSERT INTO kb_tags (slug, label) VALUES ($1, $2)
		 RETURNING id, slug, label, created_at`, slug, label).
		Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt)
	if err != nil {
		return KBTag{}, fmt.Errorf("insert kb_tag: %w", err)
	}
	return t, nil
}

// KBRenameTag updates label only (slug is stable to keep article-tag
// references intact across renames).
func (s *Store) KBRenameTag(ctx context.Context, id int64, label string) (KBTag, error) {
	var t KBTag
	err := s.pool.QueryRow(ctx,
		`UPDATE kb_tags SET label = $1 WHERE id = $2
		 RETURNING id, slug, label, created_at`, label, id).
		Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBTag{}, ErrNotFound
	}
	if err != nil {
		return KBTag{}, fmt.Errorf("rename kb_tag: %w", err)
	}
	return t, nil
}

// KBDeleteTag hard-deletes. Returns ErrNotFound if absent. The
// kb_article_tags FK uses ON DELETE RESTRICT, so this fails with a
// Postgres FK violation when the tag is still in use — the handler
// maps that to 409.
func (s *Store) KBDeleteTag(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete kb_tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// KBMergeTags moves every kb_article_tags row from `from` into `into`,
// then deletes `from`. Runs in a transaction. If an article already
// has both, the duplicate `from` row is dropped silently (so the
// merge is idempotent against partial pre-merge state).
func (s *Store) KBMergeTags(ctx context.Context, from, into int64) error {
	if from == into {
		return fmt.Errorf("merge kb_tags: from == into")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("merge kb_tags begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO kb_article_tags (article_id, tag_id)
		 SELECT article_id, $2 FROM kb_article_tags WHERE tag_id = $1
		 ON CONFLICT DO NOTHING`, from, into); err != nil {
		return fmt.Errorf("merge kb_tags reassign: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM kb_article_tags WHERE tag_id = $1`, from); err != nil {
		return fmt.Errorf("merge kb_tags clear: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM kb_tags WHERE id = $1`, from); err != nil {
		return fmt.Errorf("merge kb_tags delete: %w", err)
	}
	return tx.Commit(ctx)
}

// KBTagWithCount augments KBTag with a usage count for the admin list.
type KBTagWithCount struct {
	KBTag
	UseCount int `json:"useCount"`
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/kb_tags.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): kb tags CRUD + merge"
```

---

### Task B3: `kb_articles.go` — CRUD + lifecycle

**Files:**
- Create: `internal/store/kb_articles.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/store/kb_articles.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// KBArticleListFilter narrows what KBListArticles returns. Zero
// values are wildcards.
type KBArticleListFilter struct {
	Status      string  // "draft" | "published" | "" (all)
	CategoryID  int64   // 0 = any
	TagSlug     string  // "" = any
	TitleQuery  string  // "" = no title filter (LIKE)
	Limit       int     // 0 = 100
	Offset      int
}

// KBListArticles returns paginated summaries matching the filter.
func (s *Store) KBListArticles(ctx context.Context, f KBArticleListFilter) ([]KBArticleSummary, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	args := []any{}
	where := "WHERE 1=1"
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND a.status = $%d", len(args))
	}
	if f.CategoryID > 0 {
		args = append(args, f.CategoryID)
		where += fmt.Sprintf(" AND a.category_id = $%d", len(args))
	}
	if f.TitleQuery != "" {
		args = append(args, "%"+f.TitleQuery+"%")
		where += fmt.Sprintf(" AND a.title ILIKE $%d", len(args))
	}
	if f.TagSlug != "" {
		args = append(args, f.TagSlug)
		where += fmt.Sprintf(`
		 AND EXISTS (
		   SELECT 1 FROM kb_article_tags at
		   JOIN kb_tags t ON t.id = at.tag_id
		   WHERE at.article_id = a.id AND t.slug = $%d)`, len(args))
	}
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`
		SELECT a.id, a.slug, a.title, a.summary, a.category_id,
		       c.name AS category_name, a.status, a.published_at, a.updated_at,
		       COALESCE(
		         (SELECT array_agg(t.slug ORDER BY t.label)
		          FROM kb_article_tags at JOIN kb_tags t ON t.id = at.tag_id
		          WHERE at.article_id = a.id),
		         ARRAY[]::text[]) AS tag_slugs
		FROM kb_articles a
		JOIN kb_categories c ON c.id = a.category_id
		%s
		ORDER BY a.updated_at DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list kb_articles: %w", err)
	}
	defer rows.Close()
	out := []KBArticleSummary{}
	for rows.Next() {
		var sum KBArticleSummary
		if err := rows.Scan(&sum.ID, &sum.Slug, &sum.Title, &sum.Summary,
			&sum.CategoryID, &sum.CategoryName, &sum.Status, &sum.PublishedAt,
			&sum.UpdatedAt, &sum.Tags); err != nil {
			return nil, fmt.Errorf("scan kb_article: %w", err)
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// KBGetArticleBySlug returns a full article including tags +
// category. Returns ErrNotFound if absent. publishedOnly excludes
// drafts (customer path uses true; admin uses false).
func (s *Store) KBGetArticleBySlug(ctx context.Context, slug string, publishedOnly bool) (KBArticle, error) {
	q := `SELECT id, slug, title, summary, body_html, body_text, category_id,
	             status, publish_at, published_at, last_edited_by,
	             created_at, updated_at
	      FROM kb_articles WHERE slug = $1`
	if publishedOnly {
		q += ` AND status = 'published'`
	}
	var a KBArticle
	err := s.pool.QueryRow(ctx, q, slug).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBArticle{}, ErrNotFound
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("get kb_article by slug: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

// KBGetArticleByID — admin variant; returns drafts too.
func (s *Store) KBGetArticleByID(ctx context.Context, id int64) (KBArticle, error) {
	var a KBArticle
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, title, summary, body_html, body_text, category_id,
		       status, publish_at, published_at, last_edited_by,
		       created_at, updated_at
		FROM kb_articles WHERE id = $1`, id).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBArticle{}, ErrNotFound
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("get kb_article by id: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

// kbLoadArticleAux fills .Tags and .Category from secondary queries.
// Avoids the JSON-aggregation gymnastics on the main query for clarity.
func (s *Store) kbLoadArticleAux(ctx context.Context, a *KBArticle) error {
	tagRows, err := s.pool.Query(ctx,
		`SELECT t.id, t.slug, t.label, t.created_at
		 FROM kb_tags t JOIN kb_article_tags at ON at.tag_id = t.id
		 WHERE at.article_id = $1 ORDER BY lower(t.label)`, a.ID)
	if err != nil {
		return fmt.Errorf("load tags: %w", err)
	}
	defer tagRows.Close()
	a.Tags = []KBTag{}
	for tagRows.Next() {
		var t KBTag
		if err := tagRows.Scan(&t.ID, &t.Slug, &t.Label, &t.CreatedAt); err != nil {
			return fmt.Errorf("scan tag: %w", err)
		}
		a.Tags = append(a.Tags, t)
	}
	if err := tagRows.Err(); err != nil {
		return err
	}
	cat, err := s.KBGetCategory(ctx, a.CategoryID)
	if err == nil {
		a.Category = &cat
	}
	return nil
}

// KBSaveArticle creates if id==0, else updates. tagIDs is the full
// desired set (the writer rewrites kb_article_tags inside a tx).
// Returns the row as it stands after the write.
func (s *Store) KBSaveArticle(ctx context.Context, in KBArticle, tagIDs []int64) (KBArticle, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return KBArticle{}, fmt.Errorf("save kb_article begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if in.ID == 0 {
		err = tx.QueryRow(ctx, `
			INSERT INTO kb_articles
			  (slug, title, summary, body_html, body_text, category_id,
			   status, publish_at, published_at, last_edited_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			RETURNING id, created_at, updated_at`,
			in.Slug, in.Title, in.Summary, in.BodyHTML, in.BodyText, in.CategoryID,
			in.Status, in.PublishAt, in.PublishedAt, in.LastEditedBy).
			Scan(&in.ID, &in.CreatedAt, &in.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE kb_articles SET
			  slug = $2, title = $3, summary = $4, body_html = $5, body_text = $6,
			  category_id = $7, status = $8, publish_at = $9, published_at = $10,
			  last_edited_by = $11, updated_at = NOW()
			WHERE id = $1
			RETURNING created_at, updated_at`,
			in.ID, in.Slug, in.Title, in.Summary, in.BodyHTML, in.BodyText, in.CategoryID,
			in.Status, in.PublishAt, in.PublishedAt, in.LastEditedBy).
			Scan(&in.CreatedAt, &in.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return KBArticle{}, ErrNotFound
		}
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("upsert kb_article: %w", err)
	}

	// Rewrite tag set.
	if _, err := tx.Exec(ctx,
		`DELETE FROM kb_article_tags WHERE article_id = $1`, in.ID); err != nil {
		return KBArticle{}, fmt.Errorf("clear kb_article_tags: %w", err)
	}
	for _, tID := range tagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO kb_article_tags (article_id, tag_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`, in.ID, tID); err != nil {
			return KBArticle{}, fmt.Errorf("insert kb_article_tag: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return KBArticle{}, fmt.Errorf("save kb_article commit: %w", err)
	}
	return s.KBGetArticleByID(ctx, in.ID)
}

// KBDeleteArticle hard-deletes. Returns ErrNotFound if absent.
func (s *Store) KBDeleteArticle(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_articles WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete kb_article: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// KBPublishArticle flips status to 'published', sets published_at,
// clears publish_at. Returns the post-write row. Errors if not draft.
func (s *Store) KBPublishArticle(ctx context.Context, id int64, by string) (KBArticle, error) {
	var a KBArticle
	err := s.pool.QueryRow(ctx, `
		UPDATE kb_articles
		SET status = 'published',
		    published_at = NOW(),
		    publish_at = NULL,
		    last_edited_by = $2,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'draft'
		RETURNING id, slug, title, summary, body_html, body_text, category_id,
		          status, publish_at, published_at, last_edited_by,
		          created_at, updated_at`, id, by).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the article doesn't exist or it's already published.
		// Distinguish for the handler.
		exists, _ := s.KBArticleExists(ctx, id)
		if !exists {
			return KBArticle{}, ErrNotFound
		}
		return KBArticle{}, fmt.Errorf("publish kb_article: not in draft state")
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("publish kb_article: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

// KBUnpublishArticle reverts a published article to draft. Preserves
// published_at for history.
func (s *Store) KBUnpublishArticle(ctx context.Context, id int64, by string) (KBArticle, error) {
	var a KBArticle
	err := s.pool.QueryRow(ctx, `
		UPDATE kb_articles
		SET status = 'draft',
		    last_edited_by = $2,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'published'
		RETURNING id, slug, title, summary, body_html, body_text, category_id,
		          status, publish_at, published_at, last_edited_by,
		          created_at, updated_at`, id, by).Scan(
		&a.ID, &a.Slug, &a.Title, &a.Summary, &a.BodyHTML, &a.BodyText, &a.CategoryID,
		&a.Status, &a.PublishAt, &a.PublishedAt, &a.LastEditedBy,
		&a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		exists, _ := s.KBArticleExists(ctx, id)
		if !exists {
			return KBArticle{}, ErrNotFound
		}
		return KBArticle{}, fmt.Errorf("unpublish kb_article: not in published state")
	}
	if err != nil {
		return KBArticle{}, fmt.Errorf("unpublish kb_article: %w", err)
	}
	if err := s.kbLoadArticleAux(ctx, &a); err != nil {
		return KBArticle{}, err
	}
	return a, nil
}

func (s *Store) KBArticleExists(ctx context.Context, id int64) (bool, error) {
	var x int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM kb_articles WHERE id = $1`, id).Scan(&x)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check kb_article exists: %w", err)
	}
	return true, nil
}

// KBArticleSlugExists for the slug-collision suffix loop.
func (s *Store) KBArticleSlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT 1 FROM kb_articles WHERE slug = $1 AND ($2 = 0 OR id != $2)`,
		slug, excludeID).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check kb_article slug: %w", err)
	}
	return true, nil
}

// KBPendingPublishes returns drafts whose publish_at has elapsed.
// Used by the daily cron pass.
func (s *Store) KBPendingPublishes(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM kb_articles
		WHERE status = 'draft' AND publish_at IS NOT NULL AND publish_at <= $1
		ORDER BY publish_at
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("kb pending publishes: %w", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/kb_articles.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): kb articles CRUD + lifecycle + slug helpers"
```

---

### Task B4: `kb_search.go` — FTS + related

**Files:**
- Create: `internal/store/kb_search.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/store/kb_search.go <<'EOF'
package store

import (
	"context"
	"fmt"
)

// KBSearchHit is a search result with a rank score the customer
// page uses for ordering.
type KBSearchHit struct {
	Article  KBArticleSummary `json:"article"`
	Rank     float64          `json:"rank"`
	Snippet  string           `json:"snippet"`
}

// KBSearchArticles runs a plainto_tsquery over published articles
// only and returns ranked hits with an ts_headline snippet of body_text.
func (s *Store) KBSearchArticles(ctx context.Context, q string, limit int) ([]KBSearchHit, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		WITH q AS (SELECT plainto_tsquery('english', $1) AS tsq)
		SELECT a.id, a.slug, a.title, a.summary, a.category_id, c.name,
		       a.status, a.published_at, a.updated_at,
		       COALESCE(
		         (SELECT array_agg(t.slug ORDER BY t.label)
		          FROM kb_article_tags at JOIN kb_tags t ON t.id = at.tag_id
		          WHERE at.article_id = a.id),
		         ARRAY[]::text[]) AS tag_slugs,
		       ts_rank(a.search_vector, q.tsq) AS rank,
		       ts_headline('english', a.body_text, q.tsq,
		                   'StartSel=<mark>, StopSel=</mark>, MaxFragments=2, MinWords=8, MaxWords=20')
		FROM kb_articles a, q
		JOIN kb_categories c ON c.id = a.category_id
		WHERE a.status = 'published' AND a.search_vector @@ q.tsq
		ORDER BY rank DESC, a.published_at DESC
		LIMIT $2`, q, limit)
	if err != nil {
		return nil, fmt.Errorf("kb search: %w", err)
	}
	defer rows.Close()
	out := []KBSearchHit{}
	for rows.Next() {
		var h KBSearchHit
		if err := rows.Scan(&h.Article.ID, &h.Article.Slug, &h.Article.Title,
			&h.Article.Summary, &h.Article.CategoryID, &h.Article.CategoryName,
			&h.Article.Status, &h.Article.PublishedAt, &h.Article.UpdatedAt,
			&h.Article.Tags, &h.Rank, &h.Snippet); err != nil {
			return nil, fmt.Errorf("scan kb search hit: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// KBRelatedArticles returns up to `limit` other published articles
// whose search vector overlaps with the source article's, biased
// toward the same category. Source article excluded from results.
func (s *Store) KBRelatedArticles(ctx context.Context, sourceID int64, limit int) ([]KBArticleSummary, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := s.pool.Query(ctx, `
		WITH src AS (
		  SELECT search_vector AS sv, category_id AS cat
		  FROM kb_articles WHERE id = $1
		)
		SELECT a.id, a.slug, a.title, a.summary, a.category_id, c.name,
		       a.status, a.published_at, a.updated_at,
		       COALESCE(
		         (SELECT array_agg(t.slug ORDER BY t.label)
		          FROM kb_article_tags at JOIN kb_tags t ON t.id = at.tag_id
		          WHERE at.article_id = a.id),
		         ARRAY[]::text[]) AS tag_slugs
		FROM kb_articles a, src
		JOIN kb_categories c ON c.id = a.category_id
		WHERE a.id != $1
		  AND a.status = 'published'
		  AND a.search_vector @@ to_tsquery('english', replace(strip(src.sv)::text, ' ', ' | '))
		ORDER BY (a.category_id = src.cat) DESC,
		         ts_rank(a.search_vector, to_tsquery('english', replace(strip(src.sv)::text, ' ', ' | '))) DESC,
		         a.published_at DESC
		LIMIT $2`, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("kb related: %w", err)
	}
	defer rows.Close()
	out := []KBArticleSummary{}
	for rows.Next() {
		var sum KBArticleSummary
		if err := rows.Scan(&sum.ID, &sum.Slug, &sum.Title, &sum.Summary,
			&sum.CategoryID, &sum.CategoryName, &sum.Status, &sum.PublishedAt,
			&sum.UpdatedAt, &sum.Tags); err != nil {
			return nil, fmt.Errorf("scan kb related: %w", err)
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/kb_search.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): kb FTS search + related articles"
```

---

### Task B5: `kb_engagement.go` — votes + views

**Files:**
- Create: `internal/store/kb_engagement.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/store/kb_engagement.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// KBUpsertVote (article, customer, vote) — upsert by PK. Returns the
// final stored vote (which is `vote` unless the row was deleted by a
// concurrent admin action).
func (s *Store) KBUpsertVote(ctx context.Context, articleID int64, customerID, vote string) error {
	if vote != "up" && vote != "down" {
		return fmt.Errorf("kb upsert vote: invalid vote %q", vote)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO kb_votes (article_id, customer_id, vote)
		VALUES ($1, $2, $3)
		ON CONFLICT (article_id, customer_id) DO UPDATE
		SET vote = EXCLUDED.vote, voted_at = NOW()`,
		articleID, customerID, vote)
	if err != nil {
		return fmt.Errorf("kb upsert vote: %w", err)
	}
	return nil
}

// KBGetVote returns this customer's vote for an article, or
// ErrNotFound if they haven't voted.
func (s *Store) KBGetVote(ctx context.Context, articleID int64, customerID string) (string, error) {
	var v string
	err := s.pool.QueryRow(ctx,
		`SELECT vote FROM kb_votes WHERE article_id = $1 AND customer_id = $2`,
		articleID, customerID).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("kb get vote: %w", err)
	}
	return v, nil
}

// KBVoteAggregateFor returns the up/down counts for one article.
func (s *Store) KBVoteAggregateFor(ctx context.Context, articleID int64) (KBVoteAggregate, error) {
	var agg KBVoteAggregate
	err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE vote = 'up') AS helpful,
		  COUNT(*) FILTER (WHERE vote = 'down') AS not_helpful
		FROM kb_votes WHERE article_id = $1`, articleID).
		Scan(&agg.HelpfulCount, &agg.NotHelpfulCount)
	if err != nil {
		return KBVoteAggregate{}, fmt.Errorf("kb vote aggregate: %w", err)
	}
	return agg, nil
}

// KBRecordView inserts a view row IFF no view exists for this
// (article, customer) in the last 24 hours. Returns true if a row
// was inserted, false if deduped.
func (s *Store) KBRecordView(ctx context.Context, articleID int64, customerID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO kb_views (article_id, customer_id)
		SELECT $1, $2
		WHERE NOT EXISTS (
		  SELECT 1 FROM kb_views
		  WHERE article_id = $1 AND customer_id = $2
		    AND viewed_at > NOW() - INTERVAL '24 hours'
		)`, articleID, customerID)
	if err != nil {
		return false, fmt.Errorf("kb record view: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// KBViewAggregate30d returns total + unique counts over the last 30
// days for one article.
func (s *Store) KBViewAggregate30d(ctx context.Context, articleID int64) (KBViewAggregate, error) {
	var agg KBViewAggregate
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) AS total, COUNT(DISTINCT customer_id) AS unique_v
		FROM kb_views
		WHERE article_id = $1 AND viewed_at > NOW() - INTERVAL '30 days'`,
		articleID).Scan(&agg.TotalViews, &agg.UniqueViewers)
	if err != nil {
		return KBViewAggregate{}, fmt.Errorf("kb view aggregate: %w", err)
	}
	return agg, nil
}

// KBVoteRatio24h returns (helpful/(helpful+not_helpful), total) for
// the last 24h. Used by the unhelpful-detection cron.
type KBVoteWindow struct {
	Helpful       int
	NotHelpful    int
	HelpfulRatio  float64 // 0..1; -1 if total == 0
}

func (s *Store) KBVoteWindow24h(ctx context.Context, articleID int64) (KBVoteWindow, error) {
	var w KBVoteWindow
	err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE vote = 'up')   AS h,
		  COUNT(*) FILTER (WHERE vote = 'down') AS nh
		FROM kb_votes
		WHERE article_id = $1 AND voted_at > NOW() - INTERVAL '24 hours'`,
		articleID).Scan(&w.Helpful, &w.NotHelpful)
	if err != nil {
		return KBVoteWindow{}, fmt.Errorf("kb vote window: %w", err)
	}
	tot := w.Helpful + w.NotHelpful
	if tot == 0 {
		w.HelpfulRatio = -1
	} else {
		w.HelpfulRatio = float64(w.Helpful) / float64(tot)
	}
	return w, nil
}

// KBPublishedArticleIDs returns the IDs of all published articles —
// the cron iterates these for the unhelpful-detection pass.
func (s *Store) KBPublishedArticleIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM kb_articles WHERE status = 'published'`)
	if err != nil {
		return nil, fmt.Errorf("kb published ids: %w", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// kb_engagement.go also re-exports time for callers that build a
// cutoff timestamp.
var _ = time.Time{}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/kb_engagement.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): kb engagement (votes + views) + 24h windows"
```

---

### Task B6: `kb_images.go` — insert + read

**Files:**
- Create: `internal/store/kb_images.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/store/kb_images.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// KBInsertImage stores an image and returns its id. articleID may
// be nil for "uploaded during a new-article draft before the article
// row exists" — those get adopted on the article's first save (the
// handler walks the body_html for /api/kb/images/{id} references and
// links them).
func (s *Store) KBInsertImage(ctx context.Context, img KBImage) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO kb_images (article_id, filename, mime, bytes, content, sha256)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		img.ArticleID, img.Filename, img.MIME, img.Bytes, img.Content, img.SHA256).
		Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert kb_image: %w", err)
	}
	return id, nil
}

// KBGetImage returns the bytes + mime for the serve handler.
func (s *Store) KBGetImage(ctx context.Context, id int64) (KBImage, error) {
	var img KBImage
	err := s.pool.QueryRow(ctx, `
		SELECT id, article_id, filename, mime, bytes, content, sha256, created_at
		FROM kb_images WHERE id = $1`, id).
		Scan(&img.ID, &img.ArticleID, &img.Filename, &img.MIME, &img.Bytes,
			&img.Content, &img.SHA256, &img.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KBImage{}, ErrNotFound
	}
	if err != nil {
		return KBImage{}, fmt.Errorf("get kb_image: %w", err)
	}
	return img, nil
}

// KBAdoptOrphanImages walks the body_html for `/api/kb/images/{id}`
// references and updates each matching kb_images row's article_id
// to point at the saved article. No-op for images already linked to
// this article. Called from the article-save handler after the row
// exists.
func (s *Store) KBAdoptOrphanImages(ctx context.Context, articleID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE kb_images SET article_id = $1
		WHERE id = ANY($2) AND (article_id IS NULL OR article_id = $1)`,
		articleID, ids)
	if err != nil {
		return fmt.Errorf("adopt kb_images: %w", err)
	}
	return nil
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/kb_images.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): kb images insert / get / adopt-orphans"
```

---

## Phase C — Slug helpers + handler shared code

### Task C1: `internal/kb/slug.go` — slug derivation + collision suffix

**Files:**
- Create: `internal/kb/slug.go`
- Create: `internal/kb/slug_test.go`

- [ ] **Step 1: Write the failing tests**

```bash
mkdir -p internal/kb
cat > internal/kb/slug_test.go <<'EOF'
package kb

import "testing"

func TestSlugifyHandlesCommonCases(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"Why is my video buffering?", "why-is-my-video-buffering"},
		{"  spaces   ", "spaces"},
		{"4K Streaming!!!", "4k-streaming"},
		{"über-cool", "uber-cool"},
		{"", ""},
		{"---", ""},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniqueSlugAppendsSuffixOnCollision(t *testing.T) {
	exists := map[string]bool{"hello": true, "hello-2": true}
	check := func(s string) (bool, error) { return exists[s], nil }
	got, err := UniqueSlug("hello", check)
	if err != nil {
		t.Fatalf("UniqueSlug: %v", err)
	}
	if got != "hello-3" {
		t.Fatalf("UniqueSlug = %q, want hello-3", got)
	}
}

func TestUniqueSlugReturnsBaseOnNoCollision(t *testing.T) {
	check := func(string) (bool, error) { return false, nil }
	got, err := UniqueSlug("fresh", check)
	if err != nil || got != "fresh" {
		t.Fatalf("UniqueSlug = %q, %v; want (fresh, nil)", got, err)
	}
}
EOF
```

- [ ] **Step 2: Run, expect compile fail**

```bash
go test ./internal/kb/...
```

- [ ] **Step 3: Write the implementation**

```bash
cat > internal/kb/slug.go <<'EOF'
// Package kb holds KB-module helpers that aren't store or server
// layers — slug derivation, cron loop, event payload assembly.
package kb

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Slugify converts a title to a URL-safe lowercase-kebab slug:
//   - Unicode-normalises to NFKD and drops combining marks (so "über"
//     becomes "uber").
//   - Lowercases.
//   - Replaces any run of non-[a-z0-9] with a single "-".
//   - Trims leading/trailing "-".
//
// Empty input yields empty output (caller decides what to do).
func Slugify(s string) string {
	t := transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	normalised, _, err := transform.String(t, s)
	if err != nil {
		normalised = s
	}
	lower := strings.ToLower(normalised)
	var b strings.Builder
	prevDash := true
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// UniqueSlug returns a slug derived from base that doesn't collide
// according to exists. On collision it appends "-2", "-3", … up to
// 100 attempts. The caller passes a closure that consults the right
// table.
func UniqueSlug(base string, exists func(slug string) (bool, error)) (string, error) {
	if base == "" {
		return "", fmt.Errorf("slug: empty base")
	}
	taken, err := exists(base)
	if err != nil {
		return "", err
	}
	if !taken {
		return base, nil
	}
	for n := 2; n < 100; n++ {
		cand := fmt.Sprintf("%s-%d", base, n)
		taken, err := exists(cand)
		if err != nil {
			return "", err
		}
		if !taken {
			return cand, nil
		}
	}
	return "", fmt.Errorf("slug: too many collisions on %q", base)
}
EOF
```

- [ ] **Step 4: Add deps + run tests**

```bash
go get golang.org/x/text/runes
go get golang.org/x/text/transform
go get golang.org/x/text/unicode/norm
go mod tidy
go test ./internal/kb/... -v
```

Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/kb/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(kb): slug derivation + uniqueness helper"
```

---

### Task C2: `kb_image_refs.go` — extract image IDs from body_html

**Files:**
- Create: `internal/kb/image_refs.go`
- Create: `internal/kb/image_refs_test.go`

- [ ] **Step 1: Write the failing tests**

```bash
cat > internal/kb/image_refs_test.go <<'EOF'
package kb

import (
	"reflect"
	"testing"
)

func TestExtractImageIDsFindsAllReferences(t *testing.T) {
	html := `<p>see <img src="/api/kb/images/42" alt="x"></p>
	         <img src="/api/kb/images/7">
	         <a href="https://example.com">no match</a>
	         <img src="https://example.com/cat.png">`
	got := ExtractImageIDs(html)
	want := []int64{42, 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractImageIDs = %v, want %v", got, want)
	}
}

func TestExtractImageIDsDedupes(t *testing.T) {
	html := `<img src="/api/kb/images/3"><img src="/api/kb/images/3">`
	got := ExtractImageIDs(html)
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("ExtractImageIDs = %v, want [3]", got)
	}
}
EOF
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/kb/image_refs.go <<'EOF'
package kb

import (
	"regexp"
	"strconv"
)

var imgRefRe = regexp.MustCompile(`/api/kb/images/(\d+)`)

// ExtractImageIDs scans body_html for `/api/kb/images/{id}` references
// and returns the distinct ids in first-seen order. Used by the
// article-save handler to adopt orphan images.
func ExtractImageIDs(htmlSrc string) []int64 {
	matches := imgRefRe.FindAllStringSubmatch(htmlSrc, -1)
	if matches == nil {
		return nil
	}
	seen := map[int64]bool{}
	out := []int64{}
	for _, m := range matches {
		id, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
EOF
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/kb/... -v
```

Expected: 5 PASS (3 from C1 + 2 here).

- [ ] **Step 4: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/kb/image_refs.go internal/kb/image_refs_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(kb): extract image refs from body_html"
```

---

## Phase D — Customer handlers

### Task D1: `handlers_kb_customer.go` — list, detail, related, search, vote

**Files:**
- Create: `internal/server/handlers_kb_customer.go`

The four routes shared between customer SPA + the JSON API. Note that the SPA shell handlers (`GET /kb`, `GET /kb/{slug}`) are tiny and call `writeSPA` with the right bootstrap mode — included here so all customer KB handlers live in one file.

- [ ] **Step 1: Write the file**

```bash
cat > internal/server/handlers_kb_customer.go <<'EOF'
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
// The slug travels in the URL; the SPA resolves it via the bootstrap
// JSON path (handler bakes the article in for first-paint).
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
// + tag. Used by the browse page to populate per-category sections
// (the SPA issues this query with `category=` + `limit=5` per row).
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
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/server/...
```

(Tests get added in Phase I along with the wider server-test sweep.)

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_kb_customer.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): kb customer handlers (browse / detail / search / vote)"
```

---

## Phase E — Admin handlers

### Task E1: `handlers_kb_admin.go` — article admin (CRUD + lifecycle + engagement)

**Files:**
- Create: `internal/server/handlers_kb_admin.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/server/handlers_kb_admin.go <<'EOF'
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ContinuumApp/continuum-plugin-support/internal/htmlx"
	"github.com/ContinuumApp/continuum-plugin-support/internal/kb"
	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// hKBAdminListPage etc render the admin SPA shell with the right mode.
// The detail/edit modes pre-bake the article via writeSPA's Modules
// payload extension (not literal modules — see comment in spa.go).
func hKBAdminListPage(d Deps) http.HandlerFunc {
	return adminSPAHandler(d, "admin-kb-list")
}
func hKBAdminEditPage(d Deps) http.HandlerFunc {
	return adminSPAHandler(d, "admin-kb-edit")
}
func hKBAdminCategoriesPage(d Deps) http.HandlerFunc {
	return adminSPAHandler(d, "admin-kb-categories")
}
func hKBAdminTagsPage(d Deps) http.HandlerFunc {
	return adminSPAHandler(d, "admin-kb-tags")
}

// adminSPAHandler is the shared admin-shell render.
func adminSPAHandler(d Deps, mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode:    mode,
			Theme:   adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: true,
		}, http.StatusOK)
	}
}

// --- Article CRUD ----------------------------------------------------

// kbArticleRequest is the wire shape for create + update. Tags arrive
// as slugs; the handler resolves to ids, auto-creating any new tag.
type kbArticleRequest struct {
	Slug       string  `json:"slug"`        // optional; derived from title if empty
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	BodyHTML   string  `json:"bodyHtml"`
	CategoryID int64   `json:"categoryId"`
	Status     string  `json:"status"`      // "draft" | "published"
	PublishAt  *string `json:"publishAt"`   // RFC3339 or null
	TagLabels  []string `json:"tagLabels"`  // free-form labels
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
	return func(w http.ResponseWriter, r *http.Request) {
		kbWriteArticle(w, r, d, 0)
	}
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

	// Slug — derive from title if missing, then suffix on collision.
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

	// Sanitise body + derive plain text for FTS.
	bodyHTML := htmlx.Sanitize(req.BodyHTML)
	bodyText := htmlx.ExtractText(bodyHTML)

	// Resolve tags — auto-create new labels.
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

	// publish_at parse.
	var publishAt *string
	if req.PublishAt != nil && *req.PublishAt != "" {
		publishAt = req.PublishAt
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
		LastEditedBy: r.Header.Get("X-Continuum-User-Id"),
	}
	if publishAt != nil {
		ts, perr := parseTime(*publishAt)
		if perr != nil {
			writeErr(w, http.StatusBadRequest, "bad_publish_at",
				"publishAt must be an RFC3339 timestamp")
			return
		}
		in.PublishAt = &ts
	}
	// If status starts as published, stamp published_at.
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

	// Adopt any images referenced by the body.
	if imgIDs := kb.ExtractImageIDs(saved.BodyHTML); len(imgIDs) > 0 {
		_ = kbAdminStore(d).KBAdoptOrphanImages(r.Context(), saved.ID, imgIDs)
	}

	// Emit publish event if this was a first-publish.
	if id == 0 && saved.Status == "published" {
		kbPublishEvent(d, "kb_article_published", saved, map[string]any{
			"by": saved.LastEditedBy,
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
		saved, err := kbAdminStore(d).KBPublishArticle(r.Context(), id,
			r.Header.Get("X-Continuum-User-Id"))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "kb_not_found", "article not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusConflict, "kb_not_draft", err.Error())
			return
		}
		kbPublishEvent(d, "kb_article_published", saved, map[string]any{
			"by": saved.LastEditedBy,
		})
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
		saved, err := kbAdminStore(d).KBUnpublishArticle(r.Context(), id,
			r.Header.Get("X-Continuum-User-Id"))
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
		writeJSON(w, http.StatusOK, map[string]any{
			"votes": votes,
			"views": views,
		})
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
			// Postgres FK violation when articles still reference it.
			writeErr(w, http.StatusConflict, "kb_category_in_use",
				"category is in use by one or more articles")
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
			writeErr(w, http.StatusConflict, "kb_tag_exists",
				"a tag with that slug already exists")
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
			writeErr(w, http.StatusConflict, "kb_tag_in_use",
				"tag is in use by one or more articles")
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
// concrete store, so the helper lives here to avoid a circular file
// dependency. The two helpers are identical; duplicated for clarity.
func kbAdminStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}
EOF
```

This file also references `timeNow()`, `parseTime()`, and `kbPublishEvent()` — those are tiny helpers added next.

- [ ] **Step 2: Add the helper file**

```bash
cat > internal/server/kb_helpers.go <<'EOF'
package server

import "time"

func timeNow() time.Time { return time.Now() }

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
EOF
```

- [ ] **Step 3: Verify build (events stub still missing — Phase G)**

For now stub the event call:

```bash
cat > internal/server/kb_events.go <<'EOF'
package server

import "github.com/ContinuumApp/continuum-plugin-support/internal/store"

// kbPublishEvent is the helper Phase G replaces with the real
// host.PublishEvent call. Stubbed here so the admin handlers compile
// before the event integration lands.
func kbPublishEvent(_ Deps, _ string, _ store.KBArticle, _ map[string]any) {}
EOF
```

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_kb_admin.go internal/server/kb_helpers.go internal/server/kb_events.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): kb admin handlers (articles + categories + tags + events stub)"
```

---

## Phase F — Image upload + serve

### Task F1: `handlers_kb_images.go`

**Files:**
- Create: `internal/server/handlers_kb_images.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/server/handlers_kb_images.go <<'EOF'
package server

import (
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

const kbImageMaxBytes = 5 << 20 // 5 MB

var kbAllowedImageMIMEs = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/gif":     true,
	"image/webp":    true,
	"image/svg+xml": true,
}

// hKBAdminUploadImage accepts a multipart upload, validates size +
// MIME, persists, returns the served URL + id.
func hKBAdminUploadImage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Pre-flight size guard — anything claiming > 5 MB → 413
		// before we start reading. The shell's body cap is 12 MB,
		// so any Content-Length over 5 MB lands here as a 413, not
		// the shell's 413.
		if r.ContentLength > kbImageMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large",
				"image must be 5 MB or smaller")
			return
		}
		if err := r.ParseMultipartForm(kbImageMaxBytes); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_multipart", "could not parse multipart body")
			return
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "missing_image", "image field is required")
			return
		}
		defer file.Close()

		// Hard re-check post-read in case Content-Length lied.
		body, err := io.ReadAll(io.LimitReader(file, kbImageMaxBytes+1))
		if err != nil {
			writeInternal(w, r, d, "image_read_failed", err)
			return
		}
		if int64(len(body)) > kbImageMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large",
				"image must be 5 MB or smaller")
			return
		}

		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = http.DetectContentType(body)
		}
		if !kbAllowedImageMIMEs[mime] {
			writeErr(w, http.StatusUnsupportedMediaType, "bad_mime",
				"image must be PNG, JPEG, GIF, WEBP, or SVG")
			return
		}

		// Server-side SVG safety: SVGs can carry <script>. The
		// production fix is a dedicated SVG sanitiser; v1 simply
		// strips XML script tags by rejecting any SVG that contains
		// `<script` (case-insensitive) in the bytes.
		if mime == "image/svg+xml" {
			lower := string(body)
			if containsCI(lower, "<script") || containsCI(lower, "onerror=") || containsCI(lower, "onload=") {
				writeErr(w, http.StatusUnsupportedMediaType, "unsafe_svg",
					"SVG may not contain scripts or event handlers")
				return
			}
		}

		sum := sha256.Sum256(body)

		var articleID *int64
		if v := r.URL.Query().Get("articleId"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				articleID = &n
			}
		}

		id, err := kbAdminStore(d).KBInsertImage(r.Context(), store.KBImage{
			ArticleID: articleID,
			Filename:  header.Filename,
			MIME:      mime,
			Bytes:     int64(len(body)),
			Content:   body,
			SHA256:    sum[:],
		})
		if err != nil {
			writeInternal(w, r, d, "image_insert_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":  id,
			"url": "/api/kb/images/" + strconv.FormatInt(id, 10),
		})
	}
}

// hKBImageServe streams the image bytes. Available to both customers
// and admins (requireUser middleware — the admin-uploaded URL has to
// resolve for any logged-in customer reading the article).
func hKBImageServe(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}
		img, err := kbAdminStore(d).KBGetImage(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeInternal(w, r, d, "image_get_failed", err)
			return
		}
		w.Header().Set("Content-Type", img.MIME)
		w.Header().Set("Content-Length", strconv.FormatInt(img.Bytes, 10))
		w.Header().Set("Cache-Control", "private, max-age=3600")
		_, _ = w.Write(img.Content)
	}
}

// containsCI does a case-insensitive substring check without
// allocating two lowered strings.
func containsCI(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	hl := []byte(haystack)
	nl := []byte(needle)
	for i := 0; i < len(hl); i++ {
		hl[i] = lcByte(hl[i])
	}
	for i := 0; i < len(nl); i++ {
		nl[i] = lcByte(nl[i])
	}
	return bytesIndex(hl, nl) >= 0
}

func lcByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// bytesIndex avoids pulling in the bytes package for one call.
func bytesIndex(s, sep []byte) int {
	n := len(sep)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		match := true
		for j := 0; j < n; j++ {
			if s[i+j] != sep[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_kb_images.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): kb image upload + serve (5 MB, MIME allowlist, SVG safety)"
```

---

## Phase G — Events + cron

### Task G1: Replace event stub with real publisher

**Files:**
- Modify: `internal/server/kb_events.go`

Plugins emit events via the SDK's `runtimehost.Client.PublishEvent(ctx, name, payload)`. The current shell doesn't import the host SDK; this task pulls it in via a tiny `EventPublisher` interface on `Deps` so handlers stay testable. Production wires `runtimehost.Host().PublishEvent` from `cmd/.../main.go`.

- [ ] **Step 1: Add `EventPublisher` to `Deps`**

```bash
# Replace internal/server/server.go's Deps struct.
cd /opt/continuum_plugins/continuum-plugin-support
```

Edit `internal/server/server.go` — locate the `Deps` struct (existing) and add an EventPublisher field:

```go
type Deps struct {
    DatabaseURL    string
    Logger         hclog.Logger
    ConfigStore    ConfigStore
    EventPublisher EventPublisher  // <- ADD
}

// EventPublisher publishes plugin lifecycle events to the host bus.
// Tests can pass a fake (or leave nil — kbPublishEvent no-ops then).
type EventPublisher interface {
    PublishEvent(ctx context.Context, name string, payload map[string]any) error
}
```

(Add `"context"` to the imports if missing.)

- [ ] **Step 2: Rewrite `kb_events.go` to use the publisher**

```bash
cat > internal/server/kb_events.go <<'EOF'
package server

import (
	"context"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// kbPublishEvent assembles the base payload + extra keys and hands
// off to Deps.EventPublisher. No-ops when EventPublisher is nil
// (test contexts). Best-effort: a publish failure is logged but
// never propagated to the HTTP response.
func kbPublishEvent(d Deps, name string, a store.KBArticle, extra map[string]any) {
	if d.EventPublisher == nil {
		return
	}
	payload := map[string]any{
		"article_id":      a.ID,
		"slug":            a.Slug,
		"title":           a.Title,
		"category_slug":   "",
		"category_name":   "",
		"tags":            kbTagSlugs(a.Tags),
		"deep_link":       "/kb/" + a.Slug,
	}
	if a.Category != nil {
		payload["category_slug"] = a.Category.Slug
		payload["category_name"] = a.Category.Name
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := d.EventPublisher.PublishEvent(context.Background(),
		"plugin.continuum.support."+name, payload); err != nil && d.Logger != nil {
		d.Logger.Warn("kb event publish failed", "event", name, "err", err)
	}
}

func kbTagSlugs(tags []store.KBTag) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, t.Slug)
	}
	return out
}
EOF
```

- [ ] **Step 3: Wire the SDK publisher in main.go**

Edit `cmd/continuum-plugin-support/main.go`. Inside `applyConfig`, where the server.Deps is built, add the EventPublisher:

```go
import (
    // ... existing imports
    "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/runtimehost"
)

// inside applyConfig, before the SetHandler call:
publisher := runtimehost.Host()  // returns *runtimehost.Client; nil if host not connected

httpSrv.SetHandler(server.New(server.Deps{
    DatabaseURL:    cfg.DatabaseURL,
    Logger:         logger,
    ConfigStore:    st,
    EventPublisher: publisher,  // *runtimehost.Client implements PublishEvent
}))
```

(`*runtimehost.Client` satisfies the EventPublisher interface as defined in Step 1 because its `PublishEvent` method signature matches.)

- [ ] **Step 4: Verify build**

```bash
go build ./...
go test ./...
```

- [ ] **Step 5: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/server.go internal/server/kb_events.go cmd/continuum-plugin-support/main.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): kb event publishing via runtimehost"
```

---

### Task G2: Cron — `internal/kb/cron.go`

**Files:**
- Create: `internal/kb/cron.go`
- Create: `internal/kb/cron_test.go`

For v1, the cron is exposed as an admin button (`POST /api/admin/kb/cron/run`) that runs `PublishDue` + `UnhelpfulSweep`. The `scheduled_task.v1` SDK capability hookup is a follow-up — same fallback we documented in the tickets spec.

- [ ] **Step 1: Write the failing test**

```bash
cat > internal/kb/cron_test.go <<'EOF'
package kb

import (
	"context"
	"testing"
	"time"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// fakeStore is a hand-rolled minimal stand-in covering only the
// methods Cron touches. Keep it here so the test file is self-
// contained.
type fakeStore struct {
	pending  []int64
	publishedIDs map[int64]bool
	voteWin  map[int64]store.KBVoteWindow
	events   []string
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
EOF
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/kb/cron.go <<'EOF'
package kb

import (
	"context"
	"time"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// CronStore is the subset of store.Store the cron needs. Captured
// as an interface so the cron is unit-testable with a fake.
type CronStore interface {
	KBPendingPublishes(ctx context.Context, now time.Time, limit int) ([]int64, error)
	KBPublishArticle(ctx context.Context, id int64, by string) (store.KBArticle, error)
	KBPublishedArticleIDs(ctx context.Context) ([]int64, error)
	KBVoteWindow24h(ctx context.Context, id int64) (store.KBVoteWindow, error)
	KBGetArticleByID(ctx context.Context, id int64) (store.KBArticle, error)
}

// EventEmitter is the subset of the server's event publisher the cron
// needs. Keeps cron decoupled from the server package.
type EventEmitter interface {
	PublishKBArticleEvent(ctx context.Context, name string, a store.KBArticle, extra map[string]any)
}

// Cron bundles the daily-run KB tasks.
type Cron struct {
	Store              CronStore
	Publisher          EventEmitter
	UnhelpfulThreshold float64 // helpful_ratio below this triggers an event (default 0.5)
	UnhelpfulMinVotes  int     // require at least N votes in the 24h window (default 5)
}

// PublishDue flips every draft article whose publish_at has elapsed
// to 'published' and emits kb_article_published.
func (c *Cron) PublishDue(ctx context.Context) error {
	ids, err := c.Store.KBPendingPublishes(ctx, time.Now(), 100)
	if err != nil {
		return err
	}
	for _, id := range ids {
		a, err := c.Store.KBPublishArticle(ctx, id, "system")
		if err != nil {
			continue
		}
		c.Publisher.PublishKBArticleEvent(ctx, "kb_article_published", a, map[string]any{
			"by": "system",
		})
	}
	return nil
}

// UnhelpfulSweep emits kb_article_unhelpful for any published article
// whose last-24h helpful ratio is below threshold and that has at
// least MinVotes votes in the window.
func (c *Cron) UnhelpfulSweep(ctx context.Context) error {
	if c.UnhelpfulThreshold == 0 {
		c.UnhelpfulThreshold = 0.5
	}
	if c.UnhelpfulMinVotes == 0 {
		c.UnhelpfulMinVotes = 5
	}
	ids, err := c.Store.KBPublishedArticleIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		win, err := c.Store.KBVoteWindow24h(ctx, id)
		if err != nil {
			continue
		}
		total := win.Helpful + win.NotHelpful
		if total < c.UnhelpfulMinVotes {
			continue
		}
		if win.HelpfulRatio >= c.UnhelpfulThreshold {
			continue
		}
		a, err := c.Store.KBGetArticleByID(ctx, id)
		if err != nil {
			continue
		}
		c.Publisher.PublishKBArticleEvent(ctx, "kb_article_unhelpful", a, map[string]any{
			"helpful_ratio_24h": win.HelpfulRatio,
			"threshold":         c.UnhelpfulThreshold,
			"votes_24h":         total,
		})
	}
	return nil
}
EOF
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/kb/... -v
```

Expected: 5 PASS (slug 3 + image_refs 2 + cron 3 = 8, actually).

Wait — recount. Slug 3 + image refs 2 + cron 3 = 8.

- [ ] **Step 4: Wire admin "run cron now" endpoint**

Add to `internal/server/handlers_kb_admin.go` (append):

```go
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
```

(Add `"context"` to the imports if needed.)

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/kb/cron.go internal/kb/cron_test.go internal/server/handlers_kb_admin.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(kb): cron — publish due drafts + unhelpful sweep"
```

---

## Phase H — Routes + manifest

### Task H1: Register KB routes

**Files:**
- Modify: `internal/server/server.go`
- Modify: `cmd/continuum-plugin-support/manifest.json`

- [ ] **Step 1: Add KB routes to the chi router**

Edit `internal/server/server.go`. In `New(d Deps) http.Handler`, after the existing route registrations, add:

```go
	// KB module routes.
	r.Get("/kb",               requireUser(hKBBrowsePage(d)))
	r.Get("/kb/{slug}",        requireUser(hKBDetailPage(d)))
	r.Get("/api/customer/kb/articles",         requireUser(hKBCustomerList(d)))
	r.Get("/api/customer/kb/articles/{slug}",  requireUser(hKBCustomerDetail(d)))
	r.Get("/api/customer/kb/related/{slug}",   requireUser(hKBCustomerRelated(d)))
	r.Get("/api/customer/kb/search",           requireUser(hKBCustomerSearch(d)))
	r.Post("/api/customer/kb/articles/{slug}/vote", requireUser(hKBCustomerVote(d)))
	r.Get("/api/kb/images/{id}",               requireUser(hKBImageServe(d)))

	r.Get("/admin/kb",                requireAdmin(hKBAdminListPage(d)))
	r.Get("/admin/kb/new",            requireAdmin(hKBAdminEditPage(d)))
	r.Get("/admin/kb/{id}",           requireAdmin(hKBAdminEditPage(d)))
	r.Get("/admin/kb/categories",     requireAdmin(hKBAdminCategoriesPage(d)))
	r.Get("/admin/kb/tags",           requireAdmin(hKBAdminTagsPage(d)))

	r.Get   ("/api/admin/kb/articles",                requireAdmin(hKBAdminListArticles(d)))
	r.Post  ("/api/admin/kb/articles",                requireAdmin(hKBAdminCreateArticle(d)))
	r.Get   ("/api/admin/kb/articles/{id}",           requireAdmin(hKBAdminGetArticle(d)))
	r.Put   ("/api/admin/kb/articles/{id}",           requireAdmin(hKBAdminUpdateArticle(d)))
	r.Delete("/api/admin/kb/articles/{id}",           requireAdmin(hKBAdminDeleteArticle(d)))
	r.Post  ("/api/admin/kb/articles/{id}/publish",   requireAdmin(hKBAdminPublishArticle(d)))
	r.Post  ("/api/admin/kb/articles/{id}/unpublish", requireAdmin(hKBAdminUnpublishArticle(d)))
	r.Get   ("/api/admin/kb/articles/{id}/engagement",requireAdmin(hKBAdminEngagement(d)))

	r.Get   ("/api/admin/kb/categories",       requireAdmin(hKBAdminListCategories(d)))
	r.Post  ("/api/admin/kb/categories",       requireAdmin(hKBAdminCreateCategory(d)))
	r.Put   ("/api/admin/kb/categories/{id}",  requireAdmin(hKBAdminUpdateCategory(d)))
	r.Delete("/api/admin/kb/categories/{id}",  requireAdmin(hKBAdminDeleteCategory(d)))

	r.Get   ("/api/admin/kb/tags",             requireAdmin(hKBAdminListTags(d)))
	r.Post  ("/api/admin/kb/tags",             requireAdmin(hKBAdminCreateTag(d)))
	r.Put   ("/api/admin/kb/tags/{id}",        requireAdmin(hKBAdminRenameTag(d)))
	r.Delete("/api/admin/kb/tags/{id}",        requireAdmin(hKBAdminDeleteTag(d)))
	r.Post  ("/api/admin/kb/tags/merge",       requireAdmin(hKBAdminMergeTags(d)))

	r.Post  ("/api/admin/kb/images",           requireAdmin(hKBAdminUploadImage(d)))
	r.Post  ("/api/admin/kb/cron/run",         requireAdmin(hKBAdminRunCron(d)))
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Update manifest.json**

Edit `cmd/continuum-plugin-support/manifest.json`. Bump `version` from `0.1.0` to `0.2.0`. Append to the `http_routes` array (preserving the existing five):

```json
    { "id": "kb_browse",       "method": "GET",  "path": "/kb",                                 "access": "user" },
    { "id": "kb_detail",       "method": "GET",  "path": "/kb/*",                               "access": "user" },
    { "id": "kb_api_list",     "method": "GET",  "path": "/api/customer/kb/articles",           "access": "user" },
    { "id": "kb_api_detail",   "method": "GET",  "path": "/api/customer/kb/articles/*",         "access": "user" },
    { "id": "kb_api_related",  "method": "GET",  "path": "/api/customer/kb/related/*",          "access": "user" },
    { "id": "kb_api_search",   "method": "GET",  "path": "/api/customer/kb/search",             "access": "user" },
    { "id": "kb_api_vote",     "method": "POST", "path": "/api/customer/kb/articles/*",         "access": "user" },
    { "id": "kb_image_serve",  "method": "GET",  "path": "/api/kb/images/*",                    "access": "user" },
    { "id": "kb_admin_list",   "method": "GET",  "path": "/admin/kb",                           "access": "admin" },
    { "id": "kb_admin_edit",   "method": "GET",  "path": "/admin/kb/*",                         "access": "admin" },
    { "id": "kb_admin_api",    "method": "*",    "path": "/api/admin/kb/*",                     "access": "admin" }
```

(Wildcards keep the manifest concise; chi inside the plugin routes exactly.)

- [ ] **Step 4: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/server.go cmd/continuum-plugin-support/manifest.json
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): wire kb routes + bump manifest to 0.2.0"
```

---

## Phase I — Server tests for KB

### Task I1: KB handler sweep (auth + happy paths)

**Files:**
- Create: `internal/server/server_kb_test.go`

This test file augments `fakeConfigStore` from `server_test.go` is insufficient for KB (which needs a real `*store.Store`); these tests therefore run against a real Postgres via an env-gated `PG_DSN`. Skip when unset so CI without Postgres still passes.

- [ ] **Step 1: Write the test file**

```bash
cat > internal/server/server_kb_test.go <<'EOF'
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ContinuumApp/continuum-plugin-support/internal/migrate"
	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// kbTestDeps spins a real Postgres-backed Store. Skips the calling
// test if PG_DSN is unset.
func kbTestDeps(t *testing.T) (Deps, *store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN unset; skipping KB integration test")
	}
	ctx := context.Background()
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	st := store.New(pool)
	cleanup := func() { pool.Close() }
	return Deps{ConfigStore: st}, st, cleanup
}

func TestKBCustomerListRequiresAuth(t *testing.T) {
	d, _, cleanup := kbTestDeps(t)
	defer cleanup()
	h := New(d)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer/kb/articles", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestKBAdminRoutesRejectNonAdmin(t *testing.T) {
	d, _, cleanup := kbTestDeps(t)
	defer cleanup()
	h := New(d)

	for _, path := range []string{
		"/admin/kb",
		"/api/admin/kb/articles",
		"/api/admin/kb/categories",
		"/api/admin/kb/tags",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Continuum-User-Id", "42")
		// no admin role
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("path %s status = %d, want 403", path, rec.Code)
		}
	}
}

func TestKBArticleCRUDRoundTrip(t *testing.T) {
	d, st, cleanup := kbTestDeps(t)
	defer cleanup()
	h := New(d)

	// Seed a category.
	ctx := context.Background()
	cat, err := st.KBCreateCategory(ctx, "tests", "Tests", 0)
	if err != nil {
		t.Fatalf("seed category: %v", err)
	}

	// Create draft.
	body := fmt.Sprintf(`{"title":"Hello","categoryId":%d,"bodyHtml":"<p>hi</p>","status":"draft","tagLabels":["beta"]}`, cat.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/kb/articles", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var created store.KBArticle
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Slug != "hello" || created.Status != "draft" {
		t.Fatalf("unexpected article: %+v", created)
	}

	// Publish.
	req = httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/admin/kb/articles/%d/publish", created.ID), nil)
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d", rec.Code)
	}

	// Customer detail returns it.
	req = httptest.NewRequest(http.MethodGet, "/api/customer/kb/articles/hello", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("customer detail status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// Customer vote.
	req = httptest.NewRequest(http.MethodPost, "/api/customer/kb/articles/hello/vote",
		bytes.NewBufferString(`{"vote":"up"}`))
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("vote status = %d, body=%s", rec.Code, rec.Body.String())
	}
}
EOF
```

- [ ] **Step 2: Run tests (skipped without PG_DSN)**

```bash
go test ./internal/server/... -v -run TestKB
```

Without `PG_DSN`: all three skip cleanly.
With `PG_DSN` set to a test database: all three should pass.

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/server_kb_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "test(server): kb integration tests (PG_DSN-gated)"
```

---

## Phase J — Customer SPA

### Task J1: KB types + bootstrap-mode extensions

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/bootstrap.ts`

- [ ] **Step 1: Extend types**

Edit `web/src/lib/types.ts`. Add KB types AND extend `SupportBootstrap.mode` union:

```ts
// Existing exports preserved. Add the following:

export type KBCategory = {
  id: number;
  slug: string;
  name: string;
  sortOrder: number;
  active: boolean;
};

export type KBTag = {
  id: number;
  slug: string;
  label: string;
};

export type KBTagWithCount = KBTag & { useCount: number };

export type KBArticleSummary = {
  id: number;
  slug: string;
  title: string;
  summary: string;
  categoryId: number;
  categoryName: string;
  status: "draft" | "published";
  publishedAt?: string;
  updatedAt: string;
  tags: string[];
};

export type KBArticle = {
  id: number;
  slug: string;
  title: string;
  summary: string;
  bodyHtml: string;
  categoryId: number;
  status: "draft" | "published";
  publishAt?: string | null;
  publishedAt?: string | null;
  lastEditedBy: string;
  createdAt: string;
  updatedAt: string;
  tags: KBTag[];
  category?: KBCategory;
};

export type KBSearchHit = {
  article: KBArticleSummary;
  rank: number;
  snippet: string;
};

export type KBVoteAggregate = {
  helpfulCount: number;
  notHelpfulCount: number;
};

export type KBViewAggregate = {
  totalViews: number;
  uniqueViewers: number;
};

export type KBEngagement = {
  votes: KBVoteAggregate;
  views: KBViewAggregate;
};
```

Then change `SupportBootstrap`:

```ts
export type SupportBootstrap = {
  mode:
    | "customer-home"
    | "admin-home"
    | "kb-browse"
    | "kb-detail"
    | "admin-kb-list"
    | "admin-kb-edit"
    | "admin-kb-categories"
    | "admin-kb-tags";
  theme: string;
  modules: ModuleToggles;
  userId: string;
  isAdmin: boolean;
};
```

- [ ] **Step 2: Verify build + tests**

```bash
cd web
pnpm exec tsc -b --noEmit
pnpm test
```

(Existing 18 tests still pass — the union expansion is additive.)

- [ ] **Step 3: Commit**

```bash
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/lib/types.ts
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb types + bootstrap mode union extension"
```

---

### Task J2: api/kb.ts + TrustedHTML

**Files:**
- Create: `web/src/api/kb.ts`
- Create: `web/src/components/shared/TrustedHTML.tsx`

- [ ] **Step 1: api/kb.ts**

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > web/src/api/kb.ts <<'EOF'
import { api } from "@/lib/api";
import type { KBArticle, KBArticleSummary, KBSearchHit } from "@/lib/types";

export type KBListParams = {
  categoryId?: number;
  tag?: string;
  limit?: number;
  offset?: number;
};

export function listKBArticles(p: KBListParams = {}): Promise<KBArticleSummary[]> {
  const qs = new URLSearchParams();
  if (p.categoryId) qs.set("category", String(p.categoryId));
  if (p.tag) qs.set("tag", p.tag);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/customer/kb/articles" + (qs.toString() ? `?${qs}` : "");
  return api<KBArticleSummary[]>(path);
}

export function getKBArticle(slug: string): Promise<KBArticle> {
  return api<KBArticle>(`/api/customer/kb/articles/${encodeURIComponent(slug)}`);
}

export function getKBRelated(slug: string): Promise<KBArticleSummary[]> {
  return api<KBArticleSummary[]>(`/api/customer/kb/related/${encodeURIComponent(slug)}`);
}

export function searchKB(q: string): Promise<KBSearchHit[]> {
  return api<KBSearchHit[]>(`/api/customer/kb/search?q=${encodeURIComponent(q)}`);
}

export function voteKB(slug: string, vote: "up" | "down"): Promise<{ vote: string }> {
  return api<{ vote: string }>(`/api/customer/kb/articles/${encodeURIComponent(slug)}/vote`, {
    method: "POST",
    body: JSON.stringify({ vote }),
  });
}
EOF
```

- [ ] **Step 2: TrustedHTML — copy from public-catalog (uses DOMPurify, added as a dep in K1)**

```bash
cp /opt/continuum_plugins/continuum-plugin-public-catalog/web/src/components/shared/TrustedHTML.tsx \
   web/src/components/shared/TrustedHTML.tsx
```

(TrustedHTML imports `isomorphic-dompurify`; TypeScript will flag the missing dep until K1 installs it. Run the type check after Phase K, not now.)

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/api/kb.ts web/src/components/shared/TrustedHTML.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb customer API client + TrustedHTML"
```

---

### Task J3: SearchBar + TagChips (TDD)

**Files:**
- Create: `web/src/components/kb/SearchBar.tsx` (+ test)
- Create: `web/src/components/kb/TagChips.tsx` (+ test)

- [ ] **Step 1: SearchBar test**

```bash
mkdir -p web/src/components/kb
cat > web/src/components/kb/SearchBar.test.tsx <<'EOF'
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SearchBar } from "./SearchBar";

describe("SearchBar", () => {
  afterEach(() => cleanup());

  it("debounces calls to onQuery", async () => {
    vi.useFakeTimers();
    const onQuery = vi.fn();
    render(<SearchBar onQuery={onQuery} debounceMs={250} />);
    const input = screen.getByRole("searchbox");
    fireEvent.change(input, { target: { value: "buf" } });
    fireEvent.change(input, { target: { value: "buffe" } });
    fireEvent.change(input, { target: { value: "buffering" } });
    expect(onQuery).not.toHaveBeenCalled();
    vi.advanceTimersByTime(260);
    await waitFor(() => expect(onQuery).toHaveBeenCalledTimes(1));
    expect(onQuery).toHaveBeenCalledWith("buffering");
    vi.useRealTimers();
  });
});
EOF
```

- [ ] **Step 2: SearchBar implementation**

```bash
cat > web/src/components/kb/SearchBar.tsx <<'EOF'
import { useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";

import { Input } from "@/components/ui/input";

type Props = {
  onQuery: (q: string) => void;
  debounceMs?: number;
  initialValue?: string;
};

export function SearchBar({ onQuery, debounceMs = 250, initialValue = "" }: Props) {
  const [value, setValue] = useState(initialValue);
  const timer = useRef<number | null>(null);

  useEffect(() => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => onQuery(value), debounceMs);
    return () => {
      if (timer.current) window.clearTimeout(timer.current);
    };
  }, [value, onQuery, debounceMs]);

  return (
    <div className="relative">
      <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        type="search"
        role="searchbox"
        className="pl-8"
        placeholder="Search articles..."
        value={value}
        onChange={(e) => setValue(e.target.value)}
      />
    </div>
  );
}
EOF
```

- [ ] **Step 3: TagChips test + implementation**

```bash
cat > web/src/components/kb/TagChips.test.tsx <<'EOF'
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { TagChips } from "./TagChips";

describe("TagChips", () => {
  afterEach(() => cleanup());

  it("renders a chip per tag plus an 'all' chip", () => {
    render(<TagChips tags={["beginner","video","mobile"]} selected="" onSelect={() => {}} />);
    expect(screen.getAllByRole("button")).toHaveLength(4);
  });

  it("calls onSelect with the slug clicked, or '' for All", () => {
    const onSelect = vi.fn();
    render(<TagChips tags={["beginner","video"]} selected="" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: /^beginner$/i }));
    expect(onSelect).toHaveBeenCalledWith("beginner");
    fireEvent.click(screen.getByRole("button", { name: /^all$/i }));
    expect(onSelect).toHaveBeenLastCalledWith("");
  });

  it("marks the selected chip as pressed", () => {
    render(<TagChips tags={["beginner","video"]} selected="video" onSelect={() => {}} />);
    const video = screen.getByRole("button", { name: /^video$/i });
    expect(video).toHaveAttribute("aria-pressed", "true");
  });
});
EOF

cat > web/src/components/kb/TagChips.tsx <<'EOF'
import { Badge } from "@/components/ui/badge";

type Props = {
  tags: string[];
  selected: string;
  onSelect: (slug: string) => void;
};

export function TagChips({ tags, selected, onSelect }: Props) {
  const chips: Array<{ slug: string; label: string }> = [
    { slug: "", label: "All" },
    ...tags.map((t) => ({ slug: t, label: t })),
  ];
  return (
    <div className="flex flex-wrap gap-2">
      {chips.map((c) => {
        const pressed = c.slug === selected;
        return (
          <button
            key={c.slug || "__all__"}
            type="button"
            aria-pressed={pressed}
            onClick={() => onSelect(c.slug)}
            className="focus:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-full"
          >
            <Badge variant={pressed ? "default" : "outline"}>{c.label}</Badge>
          </button>
        );
      })}
    </div>
  );
}
EOF
```

- [ ] **Step 4: Run + commit**

```bash
cd web && pnpm test
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/kb/SearchBar.tsx web/src/components/kb/SearchBar.test.tsx web/src/components/kb/TagChips.tsx web/src/components/kb/TagChips.test.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb SearchBar + TagChips"
```

---

### Task J4: VoteButtons (TDD) + ArticleHeader + RelatedArticles

**Files:**
- Create: `web/src/components/kb/VoteButtons.tsx` (+ test)
- Create: `web/src/components/kb/ArticleHeader.tsx`
- Create: `web/src/components/kb/RelatedArticles.tsx`

- [ ] **Step 1: VoteButtons test**

```bash
cat > web/src/components/kb/VoteButtons.test.tsx <<'EOF'
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { VoteButtons } from "./VoteButtons";

describe("VoteButtons", () => {
  afterEach(() => cleanup());

  it("renders unpressed when no existing vote", () => {
    render(<VoteButtons currentVote={null} onVote={() => {}} />);
    expect(screen.getByRole("button", { name: /^helpful$/i })).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByRole("button", { name: /not helpful/i })).toHaveAttribute("aria-pressed", "false");
  });

  it("marks the matching button pressed", () => {
    render(<VoteButtons currentVote="up" onVote={() => {}} />);
    expect(screen.getByRole("button", { name: /^helpful$/i })).toHaveAttribute("aria-pressed", "true");
  });

  it("calls onVote with the clicked value", () => {
    const onVote = vi.fn();
    render(<VoteButtons currentVote={null} onVote={onVote} />);
    fireEvent.click(screen.getByRole("button", { name: /^helpful$/i }));
    expect(onVote).toHaveBeenCalledWith("up");
    fireEvent.click(screen.getByRole("button", { name: /not helpful/i }));
    expect(onVote).toHaveBeenLastCalledWith("down");
  });
});
EOF
```

- [ ] **Step 2: VoteButtons implementation**

```bash
cat > web/src/components/kb/VoteButtons.tsx <<'EOF'
import { ThumbsDown, ThumbsUp } from "lucide-react";

import { Button } from "@/components/ui/button";

type Props = {
  currentVote: "up" | "down" | null;
  onVote: (v: "up" | "down") => void;
};

export function VoteButtons({ currentVote, onVote }: Props) {
  return (
    <div className="flex items-center gap-3">
      <p className="text-sm text-muted-foreground">Was this helpful?</p>
      <Button
        type="button"
        variant={currentVote === "up" ? "default" : "outline"}
        size="sm"
        aria-pressed={currentVote === "up"}
        onClick={() => onVote("up")}
      >
        <ThumbsUp className="mr-2 h-4 w-4" /> Helpful
      </Button>
      <Button
        type="button"
        variant={currentVote === "down" ? "default" : "outline"}
        size="sm"
        aria-pressed={currentVote === "down"}
        onClick={() => onVote("down")}
      >
        <ThumbsDown className="mr-2 h-4 w-4" /> Not helpful
      </Button>
    </div>
  );
}
EOF
```

- [ ] **Step 3: ArticleHeader + RelatedArticles**

```bash
cat > web/src/components/kb/ArticleHeader.tsx <<'EOF'
import { Badge } from "@/components/ui/badge";
import type { KBArticle } from "@/lib/types";

type Props = { article: KBArticle };

export function ArticleHeader({ article }: Props) {
  return (
    <header className="space-y-2">
      <h1 className="text-3xl font-semibold leading-tight md:text-4xl">{article.title}</h1>
      <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
        {article.category && <Badge variant="secondary">{article.category.name}</Badge>}
        {article.tags.map((t) => (
          <Badge key={t.id} variant="outline">{t.label}</Badge>
        ))}
        <span className="ml-2">Updated {humanDate(article.updatedAt)}</span>
      </div>
      {article.summary && <p className="text-muted-foreground">{article.summary}</p>}
    </header>
  );
}

function humanDate(iso: string): string {
  try { return new Date(iso).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }); }
  catch { return ""; }
}
EOF

cat > web/src/components/kb/RelatedArticles.tsx <<'EOF'
import { Card, CardContent } from "@/components/ui/card";
import type { KBArticleSummary } from "@/lib/types";

type Props = { related: KBArticleSummary[] };

export function RelatedArticles({ related }: Props) {
  if (related.length === 0) return null;
  return (
    <section className="space-y-2">
      <h2 className="text-sm font-semibold uppercase tracking-[0.16em] text-muted-foreground">
        Related articles
      </h2>
      <ul className="grid gap-2">
        {related.map((a) => (
          <li key={a.id}>
            <a href={`./${encodeURIComponent(a.slug)}`} className="block">
              <Card className="transition-colors hover:border-accent/40">
                <CardContent className="space-y-1 py-3">
                  <p className="font-medium">{a.title}</p>
                  {a.summary && <p className="text-xs text-muted-foreground line-clamp-2">{a.summary}</p>}
                </CardContent>
              </Card>
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}
EOF
```

- [ ] **Step 4: Run + commit**

```bash
cd web && pnpm test
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/kb/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb VoteButtons + ArticleHeader + RelatedArticles"
```

---

### Task J5: pages/kb/Browse.tsx

**Files:**
- Create: `web/src/pages/kb/Browse.tsx`

Search snippets come from Postgres `ts_headline` and contain `<mark>...</mark>` tags. Rendering them through `<TrustedHTML>` runs them through DOMPurify on the client — defence in depth against any future change to the server-side snippet generator and avoids raw `dangerouslySetInnerHTML`.

- [ ] **Step 1: Write the page**

```bash
mkdir -p web/src/pages/kb
cat > web/src/pages/kb/Browse.tsx <<'EOF'
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { SearchBar } from "@/components/kb/SearchBar";
import { TagChips } from "@/components/kb/TagChips";
import { TopBar } from "@/components/shared/TopBar";
import { TrustedHTML } from "@/components/shared/TrustedHTML";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { listKBArticles, searchKB } from "@/api/kb";
import type { KBArticleSummary, KBSearchHit, SupportBootstrap } from "@/lib/types";

type Props = { bootstrap: SupportBootstrap };

export function KBBrowse({ bootstrap }: Props) {
  const [tag, setTag] = useState<string>(() =>
    new URLSearchParams(window.location.search).get("tag") ?? ""
  );
  const [query, setQuery] = useState("");
  const [articles, setArticles] = useState<KBArticleSummary[]>([]);
  const [hits, setHits] = useState<KBSearchHit[] | null>(null);
  const [loading, setLoading] = useState(true);

  // Mirror tag selection to ?tag= for bookmarking.
  useEffect(() => {
    const url = new URL(window.location.href);
    if (tag) url.searchParams.set("tag", tag);
    else url.searchParams.delete("tag");
    window.history.pushState({}, "", url.toString());
  }, [tag]);

  // Browse mode (no active query): load filtered list.
  useEffect(() => {
    if (query) return;
    let cancelled = false;
    setLoading(true);
    listKBArticles({ tag: tag || undefined, limit: 100 })
      .then((rows) => { if (!cancelled) setArticles(rows); })
      .catch((err) => toast.error(err instanceof Error ? err.message : "Failed to load articles"))
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [tag, query]);

  // Search mode.
  useEffect(() => {
    if (!query) { setHits(null); return; }
    let cancelled = false;
    setLoading(true);
    searchKB(query)
      .then((res) => { if (!cancelled) setHits(res); })
      .catch((err) => toast.error(err instanceof Error ? err.message : "Search failed"))
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [query]);

  const allTags = Array.from(new Set(articles.flatMap((a) => a.tags))).sort();
  const groupedByCategory = groupBy(articles, (a) => a.categoryName);

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-5xl space-y-6 px-4 py-10 md:px-8">
        <TopBar
          eyebrow="Support"
          title="Knowledge Base"
          subtitle="Search articles, FAQs, and how-tos."
        />
        <SearchBar onQuery={setQuery} />
        <TagChips tags={allTags} selected={tag} onSelect={setTag} />
        {hits !== null
          ? <SearchResults hits={hits} loading={loading} tickets={bootstrap.modules.tickets} />
          : <CategoryGroups groups={groupedByCategory} loading={loading} />}
      </div>
    </main>
  );
}

function SearchResults({ hits, loading, tickets }: { hits: KBSearchHit[]; loading: boolean; tickets: boolean }) {
  if (loading) return <p className="text-sm text-muted-foreground">Searching…</p>;
  if (hits.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
        <p className="font-medium text-foreground">No matching articles.</p>
        {tickets && (
          <p className="mt-2">
            Can't find what you need?{" "}
            <a href="../tickets/new" className="text-accent hover:underline">Open a ticket →</a>
          </p>
        )}
      </div>
    );
  }
  return (
    <ul className="grid gap-2">
      {hits.map((h) => (
        <li key={h.article.id}>
          <a href={`./kb/${encodeURIComponent(h.article.slug)}`} className="block">
            <Card className="transition-colors hover:border-accent/40">
              <CardContent className="space-y-1 py-3">
                <p className="font-medium">{h.article.title}</p>
                <TrustedHTML html={h.snippet} className="text-xs text-muted-foreground line-clamp-2" />
              </CardContent>
            </Card>
          </a>
        </li>
      ))}
    </ul>
  );
}

function CategoryGroups({ groups, loading }: { groups: Record<string, KBArticleSummary[]>; loading: boolean }) {
  if (loading) return <p className="text-sm text-muted-foreground">Loading…</p>;
  const names = Object.keys(groups).sort();
  if (names.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
        No articles published yet.
      </div>
    );
  }
  return (
    <div className="space-y-6">
      {names.map((cat) => (
        <section key={cat} className="space-y-2">
          <h2 className="text-sm font-semibold uppercase tracking-[0.16em] text-muted-foreground">{cat}</h2>
          <ul className="grid gap-2">
            {groups[cat].slice(0, 6).map((a) => (
              <li key={a.id}>
                <a href={`./kb/${encodeURIComponent(a.slug)}`} className="block">
                  <Card className="transition-colors hover:border-accent/40">
                    <CardContent className="space-y-1 py-3">
                      <p className="font-medium">{a.title}</p>
                      {a.summary && <p className="text-xs text-muted-foreground line-clamp-2">{a.summary}</p>}
                    </CardContent>
                  </Card>
                </a>
              </li>
            ))}
          </ul>
          {groups[cat].length > 6 && (
            <Button asChild variant="ghost" size="sm">
              <a href={`./kb?category=${encodeURIComponent(cat)}`}>See all in {cat} →</a>
            </Button>
          )}
        </section>
      ))}
    </div>
  );
}

function groupBy<T, K extends string>(items: T[], key: (t: T) => K): Record<K, T[]> {
  const out = {} as Record<K, T[]>;
  for (const item of items) {
    const k = key(item);
    if (!out[k]) out[k] = [];
    out[k].push(item);
  }
  return out;
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/pages/kb/Browse.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): KBBrowse page (search + tags + category groups)"
```

---

### Task J6: pages/kb/Detail.tsx

**Files:**
- Create: `web/src/pages/kb/Detail.tsx`

- [ ] **Step 1: Write the page**

```bash
cat > web/src/pages/kb/Detail.tsx <<'EOF'
import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";

import { ArticleHeader } from "@/components/kb/ArticleHeader";
import { RelatedArticles } from "@/components/kb/RelatedArticles";
import { VoteButtons } from "@/components/kb/VoteButtons";
import { TrustedHTML } from "@/components/shared/TrustedHTML";
import { getKBArticle, getKBRelated, voteKB } from "@/api/kb";
import type { KBArticle, KBArticleSummary } from "@/lib/types";

export function KBDetail() {
  const slug = decodeURIComponent(window.location.pathname.split("/kb/")[1] ?? "");
  const [article, setArticle] = useState<KBArticle | null>(null);
  const [related, setRelated] = useState<KBArticleSummary[]>([]);
  const [vote, setVote] = useState<"up" | "down" | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setError("");
    getKBArticle(slug)
      .then((a) => { if (!cancelled) setArticle(a); })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Not found"); });
    getKBRelated(slug)
      .then((r) => { if (!cancelled) setRelated(r); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [slug]);

  async function onVote(v: "up" | "down") {
    setVote(v);
    try {
      await voteKB(slug, v);
      toast.success("Thanks for the feedback.");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Vote failed");
      setVote(null);
    }
  }

  if (error) {
    return (
      <main className="mx-auto max-w-3xl px-4 py-16 text-center">
        <h1 className="mb-2 text-2xl font-semibold">Article unavailable</h1>
        <p className="text-muted-foreground">{error}</p>
        <a href="../kb" className="mt-4 inline-flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft className="h-4 w-4" /> Back to Knowledge Base
        </a>
      </main>
    );
  }

  if (!article) {
    return <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">Loading…</main>;
  }

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-6 px-4 py-10 md:px-8">
        <a href="../kb" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" /> Back to Knowledge Base
        </a>
        <ArticleHeader article={article} />
        <TrustedHTML html={article.bodyHtml} className="prose prose-invert max-w-none text-foreground" />
        <hr className="border-border" />
        <VoteButtons currentVote={vote} onVote={onVote} />
        <RelatedArticles related={related} />
      </div>
    </main>
  );
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/pages/kb/Detail.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): KBDetail page (TrustedHTML + vote + related)"
```

---

## Phase K — Admin SPA

### Task K1: Install Tiptap + DOMPurify deps

**Files:**
- Modify: `web/package.json` + `pnpm-lock.yaml`

- [ ] **Step 1: Install**

```bash
cd web
pnpm add @tiptap/react @tiptap/starter-kit @tiptap/extension-image @tiptap/extension-link isomorphic-dompurify
cd ..
```

- [ ] **Step 2: Verify**

```bash
cd web && pnpm exec tsc -b --noEmit
cd ..
```

TrustedHTML's import now resolves.

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/package.json web/pnpm-lock.yaml
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "chore(web): add tiptap + isomorphic-dompurify deps"
```

---

### Task K2: api/kbAdmin.ts

**Files:**
- Create: `web/src/api/kbAdmin.ts`

- [ ] **Step 1: Write the client**

```bash
cat > web/src/api/kbAdmin.ts <<'EOF'
import { api, absoluteURL } from "@/lib/api";
import type {
  KBArticle, KBArticleSummary, KBCategory, KBEngagement, KBTagWithCount,
} from "@/lib/types";

export type KBArticleListAdminParams = {
  status?: "draft" | "published";
  categoryId?: number;
  tag?: string;
  q?: string;
  limit?: number;
  offset?: number;
};

export function listKBArticlesAdmin(p: KBArticleListAdminParams = {}): Promise<KBArticleSummary[]> {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.categoryId) qs.set("categoryId", String(p.categoryId));
  if (p.tag) qs.set("tag", p.tag);
  if (p.q) qs.set("q", p.q);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/admin/kb/articles" + (qs.toString() ? `?${qs}` : "");
  return api<KBArticleSummary[]>(path);
}

export function getKBArticleAdmin(id: number): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}`);
}

export type KBArticleWrite = {
  slug?: string;
  title: string;
  summary: string;
  bodyHtml: string;
  categoryId: number;
  status: "draft" | "published";
  publishAt?: string | null;
  tagLabels: string[];
};

export function createKBArticle(w: KBArticleWrite): Promise<KBArticle> {
  return api<KBArticle>("/api/admin/kb/articles", { method: "POST", body: JSON.stringify(w) });
}

export function updateKBArticle(id: number, w: KBArticleWrite): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}`, { method: "PUT", body: JSON.stringify(w) });
}

export function deleteKBArticle(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/kb/articles/${id}`, { method: "DELETE" });
}

export function publishKBArticle(id: number): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}/publish`, { method: "POST" });
}

export function unpublishKBArticle(id: number): Promise<KBArticle> {
  return api<KBArticle>(`/api/admin/kb/articles/${id}/unpublish`, { method: "POST" });
}

export function getKBEngagement(id: number): Promise<KBEngagement> {
  return api<KBEngagement>(`/api/admin/kb/articles/${id}/engagement`);
}

export function listKBCategories(): Promise<KBCategory[]> {
  return api<KBCategory[]>("/api/admin/kb/categories");
}

export function createKBCategory(name: string, sortOrder: number, slug?: string): Promise<KBCategory> {
  return api<KBCategory>("/api/admin/kb/categories", {
    method: "POST",
    body: JSON.stringify({ name, sortOrder, slug: slug ?? "", active: true }),
  });
}

export function updateKBCategory(id: number, name: string, sortOrder: number, active: boolean): Promise<KBCategory> {
  return api<KBCategory>(`/api/admin/kb/categories/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name, sortOrder, active }),
  });
}

export function deleteKBCategory(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/kb/categories/${id}`, { method: "DELETE" });
}

export function listKBTags(): Promise<KBTagWithCount[]> {
  return api<KBTagWithCount[]>("/api/admin/kb/tags");
}

export function renameKBTag(id: number, label: string) {
  return api<{ id: number; slug: string; label: string }>(`/api/admin/kb/tags/${id}`, {
    method: "PUT", body: JSON.stringify({ label }),
  });
}

export function deleteKBTag(id: number) {
  return api<{ ok: boolean }>(`/api/admin/kb/tags/${id}`, { method: "DELETE" });
}

export function mergeKBTags(fromId: number, intoId: number) {
  return api<{ ok: boolean }>("/api/admin/kb/tags/merge", {
    method: "POST", body: JSON.stringify({ fromId, intoId }),
  });
}

export async function uploadKBImage(file: File, articleId?: number): Promise<{ id: number; url: string }> {
  const fd = new FormData();
  fd.append("image", file);
  const qs = articleId ? `?articleId=${articleId}` : "";
  const res = await fetch(absoluteURL(`/api/admin/kb/images${qs}`), { method: "POST", body: fd });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.error?.message ?? `Upload failed (${res.status})`);
  }
  return res.json();
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/api/kbAdmin.ts
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb admin API client"
```

---

### Task K3: ArticleEditor (Tiptap)

**Files:**
- Create: `web/src/components/admin/kb/ArticleEditor.tsx`

- [ ] **Step 1: Write the file**

```bash
mkdir -p web/src/components/admin/kb
cat > web/src/components/admin/kb/ArticleEditor.tsx <<'EOF'
import { useCallback, useEffect, useRef, useState } from "react";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import { Bold, Code, Image as ImageIcon, Italic, Link2, List, ListOrdered, Quote } from "lucide-react";

import { Button } from "@/components/ui/button";
import { uploadKBImage } from "@/api/kbAdmin";

type Props = {
  initialHTML: string;
  articleId?: number;
  onChange: (html: string) => void;
};

export function ArticleEditor({ initialHTML, articleId, onChange }: Props) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const editor = useEditor({
    extensions: [
      StarterKit.configure({ heading: { levels: [1, 2, 3] } }),
      Image,
      Link.configure({ openOnClick: false, autolink: true }),
    ],
    content: initialHTML,
    onUpdate: ({ editor }) => onChange(editor.getHTML()),
  });
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    if (editor && initialHTML && editor.getHTML() !== initialHTML) {
      editor.commands.setContent(initialHTML, false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editor]);

  const pickImage = useCallback(() => fileInputRef.current?.click(), []);

  const onImageChange = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file || !editor) return;
    setUploading(true);
    try {
      const { url } = await uploadKBImage(file, articleId);
      editor.chain().focus().setImage({ src: url, alt: file.name }).run();
    } catch (err) {
      console.error(err);
    } finally {
      setUploading(false);
    }
  }, [editor, articleId]);

  if (!editor) return null;

  return (
    <div className="space-y-2 rounded-md border border-border">
      <div className="flex flex-wrap items-center gap-1 border-b border-border bg-card px-2 py-1.5">
        <ToolbarBtn active={editor.isActive("bold")} onClick={() => editor.chain().focus().toggleBold().run()} aria-label="Bold">
          <Bold className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("italic")} onClick={() => editor.chain().focus().toggleItalic().run()} aria-label="Italic">
          <Italic className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("code")} onClick={() => editor.chain().focus().toggleCode().run()} aria-label="Inline code">
          <Code className="h-4 w-4" />
        </ToolbarBtn>
        <Separator />
        <ToolbarBtn active={editor.isActive("heading", { level: 1 })} onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()} aria-label="Heading 1">H1</ToolbarBtn>
        <ToolbarBtn active={editor.isActive("heading", { level: 2 })} onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()} aria-label="Heading 2">H2</ToolbarBtn>
        <ToolbarBtn active={editor.isActive("heading", { level: 3 })} onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()} aria-label="Heading 3">H3</ToolbarBtn>
        <Separator />
        <ToolbarBtn active={editor.isActive("bulletList")} onClick={() => editor.chain().focus().toggleBulletList().run()} aria-label="Bullet list">
          <List className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("orderedList")} onClick={() => editor.chain().focus().toggleOrderedList().run()} aria-label="Numbered list">
          <ListOrdered className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn active={editor.isActive("blockquote")} onClick={() => editor.chain().focus().toggleBlockquote().run()} aria-label="Quote">
          <Quote className="h-4 w-4" />
        </ToolbarBtn>
        <Separator />
        <ToolbarBtn
          active={editor.isActive("link")}
          onClick={() => {
            const url = window.prompt("Link URL", editor.getAttributes("link").href ?? "");
            if (url === null) return;
            if (url === "") editor.chain().focus().unsetLink().run();
            else editor.chain().focus().setLink({ href: url }).run();
          }}
          aria-label="Link"
        >
          <Link2 className="h-4 w-4" />
        </ToolbarBtn>
        <ToolbarBtn onClick={pickImage} aria-label="Image">
          <ImageIcon className="h-4 w-4" />
        </ToolbarBtn>
        {uploading && <span className="ml-2 text-xs text-muted-foreground">Uploading…</span>}
      </div>
      <EditorContent editor={editor} className="prose prose-invert max-w-none px-4 py-3 min-h-[300px]" />
      <input
        ref={fileInputRef}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp,image/svg+xml"
        className="hidden"
        onChange={onImageChange}
      />
    </div>
  );
}

function Separator() {
  return <span className="mx-1 h-5 w-px bg-border" aria-hidden />;
}

function ToolbarBtn({
  active, children, ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { active?: boolean }) {
  return (
    <Button type="button" variant={active ? "default" : "ghost"} size="sm" className="h-7 px-2" {...rest}>
      {children}
    </Button>
  );
}
EOF
```

- [ ] **Step 2: Verify + commit**

```bash
cd web && pnpm exec tsc -b --noEmit
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/admin/kb/ArticleEditor.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb ArticleEditor (Tiptap WYSIWYG + image upload)"
```

---

### Task K4: ArticleList + CategoryAdmin + TagAdmin + EngagementChart

**Files:**
- Create: `web/src/components/admin/kb/{ArticleList,CategoryAdmin,TagAdmin,EngagementChart}.tsx`

- [ ] **Step 1: ArticleList**

```bash
cat > web/src/components/admin/kb/ArticleList.tsx <<'EOF'
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { KBArticleSummary } from "@/lib/types";

type Props = { rows: KBArticleSummary[] };

export function ArticleList({ rows }: Props) {
  if (rows.length === 0) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-sm text-muted-foreground">
          No articles match the current filters.
        </CardContent>
      </Card>
    );
  }
  return (
    <table className="w-full border-collapse text-sm">
      <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
        <tr>
          <th className="py-2">Title</th>
          <th className="py-2">Category</th>
          <th className="py-2">Tags</th>
          <th className="py-2">Status</th>
          <th className="py-2">Updated</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id} className="border-t border-border">
            <td className="py-2"><a href={`./kb/${r.id}`} className="font-medium hover:underline">{r.title}</a></td>
            <td className="py-2">{r.categoryName}</td>
            <td className="py-2">
              <div className="flex flex-wrap gap-1">
                {r.tags.map((t) => <Badge key={t} variant="outline">{t}</Badge>)}
              </div>
            </td>
            <td className="py-2"><Badge variant={r.status === "published" ? "default" : "secondary"}>{r.status}</Badge></td>
            <td className="py-2 text-muted-foreground">{humanDate(r.updatedAt)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function humanDate(iso: string): string {
  try { return new Date(iso).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }); }
  catch { return ""; }
}
EOF
```

- [ ] **Step 2: CategoryAdmin**

```bash
cat > web/src/components/admin/kb/CategoryAdmin.tsx <<'EOF'
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { createKBCategory, deleteKBCategory, updateKBCategory } from "@/api/kbAdmin";
import type { KBCategory } from "@/lib/types";

type Props = { initial: KBCategory[] };

export function CategoryAdmin({ initial }: Props) {
  const [rows, setRows] = useState<KBCategory[]>(initial);
  const [newName, setNewName] = useState("");

  async function add() {
    if (!newName.trim()) return;
    try {
      const c = await createKBCategory(newName.trim(), rows.length);
      setRows((r) => [...r, c]);
      setNewName("");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Create failed"); }
  }

  async function save(c: KBCategory) {
    try {
      const updated = await updateKBCategory(c.id, c.name, c.sortOrder, c.active);
      setRows((rs) => rs.map((x) => x.id === updated.id ? updated : x));
      toast.success("Saved.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function remove(c: KBCategory) {
    if (!confirm(`Delete category "${c.name}"?`)) return;
    try {
      await deleteKBCategory(c.id);
      setRows((rs) => rs.filter((x) => x.id !== c.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle>Categories</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="flex gap-2">
          <Input placeholder="New category name" value={newName} onChange={(e) => setNewName(e.target.value)} />
          <Button onClick={add}>Add</Button>
        </div>
        <ul className="divide-y divide-border">
          {rows.map((c) => (
            <li key={c.id} className="flex items-center gap-2 py-2">
              <Input
                value={c.name}
                onChange={(e) => setRows((rs) => rs.map((x) => x.id === c.id ? { ...x, name: e.target.value } : x))}
                onBlur={() => save(c)}
                className="flex-1"
              />
              <Switch
                checked={c.active}
                onCheckedChange={(v) => {
                  setRows((rs) => rs.map((x) => x.id === c.id ? { ...x, active: v } : x));
                  save({ ...c, active: v });
                }}
              />
              <Button variant="destructive" size="sm" onClick={() => remove(c)}>Delete</Button>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
EOF
```

- [ ] **Step 3: TagAdmin**

```bash
cat > web/src/components/admin/kb/TagAdmin.tsx <<'EOF'
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { deleteKBTag, mergeKBTags, renameKBTag } from "@/api/kbAdmin";
import type { KBTagWithCount } from "@/lib/types";

type Props = { initial: KBTagWithCount[] };

export function TagAdmin({ initial }: Props) {
  const [tags, setTags] = useState<KBTagWithCount[]>(initial);
  const [mergeFrom, setMergeFrom] = useState<number | "">("");
  const [mergeInto, setMergeInto] = useState<number | "">("");

  async function rename(id: number, label: string) {
    try {
      await renameKBTag(id, label);
      setTags((ts) => ts.map((t) => t.id === id ? { ...t, label } : t));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Rename failed"); }
  }

  async function remove(t: KBTagWithCount) {
    if (!confirm(`Delete tag "${t.label}"?`)) return;
    try {
      await deleteKBTag(t.id);
      setTags((ts) => ts.filter((x) => x.id !== t.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  async function merge() {
    if (typeof mergeFrom !== "number" || typeof mergeInto !== "number" || mergeFrom === mergeInto) return;
    try {
      await mergeKBTags(mergeFrom, mergeInto);
      toast.success("Tags merged.");
      setTags((ts) => {
        const fromTag = ts.find((t) => t.id === mergeFrom);
        const fromCount = fromTag?.useCount ?? 0;
        return ts
          .filter((t) => t.id !== mergeFrom)
          .map((t) => t.id === mergeInto ? { ...t, useCount: t.useCount + fromCount } : t);
      });
      setMergeFrom(""); setMergeInto("");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Merge failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle>Tags</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <ul className="divide-y divide-border">
          {tags.map((t) => (
            <li key={t.id} className="flex items-center gap-2 py-2">
              <Input defaultValue={t.label} onBlur={(e) => e.target.value !== t.label && rename(t.id, e.target.value)} className="flex-1" />
              <Badge variant="secondary">{t.useCount} use{t.useCount === 1 ? "" : "s"}</Badge>
              <Button variant="destructive" size="sm" disabled={t.useCount > 0} onClick={() => remove(t)}>Delete</Button>
            </li>
          ))}
        </ul>
        <div className="rounded-md border border-border bg-card p-3 space-y-2">
          <p className="text-sm font-medium">Merge tags</p>
          <p className="text-xs text-muted-foreground">Move every article on <em>from</em> onto <em>into</em>, then delete <em>from</em>.</p>
          <div className="flex flex-wrap gap-2">
            <select className="rounded border border-border bg-background px-2 py-1 text-sm" value={mergeFrom} onChange={(e) => setMergeFrom(Number(e.target.value) || "")}>
              <option value="">from</option>
              {tags.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
            </select>
            <select className="rounded border border-border bg-background px-2 py-1 text-sm" value={mergeInto} onChange={(e) => setMergeInto(Number(e.target.value) || "")}>
              <option value="">into</option>
              {tags.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
            </select>
            <Button size="sm" onClick={merge} disabled={!mergeFrom || !mergeInto || mergeFrom === mergeInto}>Merge</Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
EOF
```

- [ ] **Step 4: EngagementChart**

```bash
cat > web/src/components/admin/kb/EngagementChart.tsx <<'EOF'
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { KBEngagement } from "@/lib/types";

type Props = { engagement: KBEngagement };

export function EngagementChart({ engagement }: Props) {
  const total = engagement.votes.helpfulCount + engagement.votes.notHelpfulCount;
  const helpfulPct = total === 0 ? 0 : Math.round((engagement.votes.helpfulCount / total) * 100);
  return (
    <Card>
      <CardHeader><CardTitle>Engagement (last 30 days)</CardTitle></CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-2 gap-3 text-sm">
          <Stat label="Total views" value={engagement.views.totalViews} />
          <Stat label="Unique viewers" value={engagement.views.uniqueViewers} />
          <Stat label="Helpful" value={engagement.votes.helpfulCount} />
          <Stat label="Not helpful" value={engagement.votes.notHelpfulCount} />
        </div>
        <div>
          <p className="text-xs text-muted-foreground mb-1">Helpful ratio</p>
          <div className="h-2 w-full rounded-full bg-muted">
            <div className="h-2 rounded-full bg-accent" style={{ width: `${helpfulPct}%` }} />
          </div>
          <p className="mt-1 text-xs text-muted-foreground">{helpfulPct}%</p>
        </div>
      </CardContent>
    </Card>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">{label}</p>
      <p className="text-xl font-semibold">{value}</p>
    </div>
  );
}
EOF
```

- [ ] **Step 5: Verify + commit**

```bash
cd web && pnpm exec tsc -b --noEmit
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/admin/kb/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb admin components — ArticleList / CategoryAdmin / TagAdmin / EngagementChart"
```

---

### Task K5: Admin pages — List / Edit / Categories / Tags

**Files:**
- Create: `web/src/pages/admin/kb/{List,Edit,Categories,Tags}.tsx`

- [ ] **Step 1: List + Edit + Categories + Tags**

```bash
mkdir -p web/src/pages/admin/kb
cat > web/src/pages/admin/kb/List.tsx <<'EOF'
import { useEffect, useState } from "react";
import { ArticleList } from "@/components/admin/kb/ArticleList";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { listKBArticlesAdmin } from "@/api/kbAdmin";
import type { KBArticleSummary } from "@/lib/types";

export function KBAdminList() {
  const [rows, setRows] = useState<KBArticleSummary[]>([]);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState<"" | "draft" | "published">("");
  useEffect(() => {
    let cancelled = false;
    listKBArticlesAdmin({ q: q || undefined, status: status || undefined, limit: 200 })
      .then((r) => { if (!cancelled) setRows(r); }).catch(() => {});
    return () => { cancelled = true; };
  }, [q, status]);
  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold">Knowledge Base</h2>
        <Button asChild><a href="./kb/new">New article</a></Button>
      </div>
      <div className="flex flex-wrap gap-2">
        <Input placeholder="Search title…" value={q} onChange={(e) => setQ(e.target.value)} className="max-w-sm" />
        <select value={status} onChange={(e) => setStatus(e.target.value as "" | "draft" | "published")}
                className="rounded border border-border bg-background px-2 py-1 text-sm">
          <option value="">All statuses</option>
          <option value="draft">Draft</option>
          <option value="published">Published</option>
        </select>
      </div>
      <ArticleList rows={rows} />
    </section>
  );
}
EOF

cat > web/src/pages/admin/kb/Edit.tsx <<'EOF'
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { ArticleEditor } from "@/components/admin/kb/ArticleEditor";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  createKBArticle, getKBArticleAdmin, listKBCategories, listKBTags,
  publishKBArticle, unpublishKBArticle, updateKBArticle, type KBArticleWrite,
} from "@/api/kbAdmin";
import type { KBArticle, KBCategory, KBTagWithCount } from "@/lib/types";

export function KBAdminEdit() {
  const idMatch = window.location.pathname.match(/\/admin\/kb\/(?:new|(\d+))/);
  const id = idMatch?.[1] ? Number(idMatch[1]) : null;

  const [categories, setCategories] = useState<KBCategory[]>([]);
  const [allTags, setAllTags] = useState<KBTagWithCount[]>([]);
  const [article, setArticle] = useState<KBArticle | null>(null);
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [bodyHtml, setBodyHtml] = useState("");
  const [categoryId, setCategoryId] = useState<number>(0);
  const [tagLabels, setTagLabels] = useState<string[]>([]);
  const [publishAt, setPublishAt] = useState<string>("");

  useEffect(() => {
    listKBCategories().then(setCategories).catch(() => {});
    listKBTags().then(setAllTags).catch(() => {});
    if (id !== null) {
      getKBArticleAdmin(id).then((a) => {
        setArticle(a);
        setTitle(a.title);
        setSummary(a.summary);
        setBodyHtml(a.bodyHtml);
        setCategoryId(a.categoryId);
        setTagLabels(a.tags.map((t) => t.label));
        setPublishAt(a.publishAt ?? "");
      }).catch(() => toast.error("Could not load article"));
    }
  }, [id]);

  function buildWrite(status: "draft" | "published"): KBArticleWrite {
    return { title, summary, bodyHtml, categoryId, status, publishAt: publishAt || null, tagLabels };
  }

  async function save(status: "draft" | "published") {
    try {
      const saved = id === null
        ? await createKBArticle(buildWrite(status))
        : await updateKBArticle(id, buildWrite(status));
      setArticle(saved);
      toast.success("Saved.");
      if (id === null) window.location.assign(`./kb/${saved.id}`);
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function togglePublish() {
    if (!article) return;
    try {
      const updated = article.status === "draft"
        ? await publishKBArticle(article.id)
        : await unpublishKBArticle(article.id);
      setArticle(updated);
      toast.success(updated.status === "published" ? "Published." : "Reverted to draft.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Action failed"); }
  }

  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">{id ? "Edit article" : "New article"}</h2>

      <Card>
        <CardHeader><CardTitle>Basics</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1">
            <Label htmlFor="title">Title</Label>
            <Input id="title" value={title} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label htmlFor="category">Category</Label>
            <select id="category" value={categoryId} onChange={(e) => setCategoryId(Number(e.target.value))}
                    className="w-full rounded border border-border bg-background px-2 py-1 text-sm">
              <option value={0}>— pick a category —</option>
              {categories.filter((c) => c.active).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="tags">Tags (comma-separated)</Label>
            <Input id="tags" value={tagLabels.join(", ")}
                   onChange={(e) => setTagLabels(e.target.value.split(",").map((s) => s.trim()).filter(Boolean))}
                   placeholder="beginner, video, mobile" />
            <p className="text-xs text-muted-foreground">
              Existing tags: {allTags.length === 0 ? "none yet" : allTags.map((t) => t.label).join(", ")}
            </p>
          </div>
          <div className="space-y-1">
            <Label htmlFor="summary">Summary</Label>
            <Textarea id="summary" rows={2} value={summary} onChange={(e) => setSummary(e.target.value)}
                      placeholder="One-line description shown in search results." />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Body</CardTitle></CardHeader>
        <CardContent>
          <ArticleEditor initialHTML={article?.bodyHtml ?? ""} articleId={article?.id} onChange={setBodyHtml} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Publish</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          {article && (
            <p className="text-sm text-muted-foreground">
              Status: <strong>{article.status}</strong>{article.publishedAt ? ` · last published ${new Date(article.publishedAt).toLocaleString()}` : ""}
            </p>
          )}
          <div className="space-y-1">
            <Label htmlFor="schedule">Schedule (optional RFC3339)</Label>
            <Input id="schedule" placeholder="2026-06-01T09:00:00Z" value={publishAt}
                   onChange={(e) => setPublishAt(e.target.value)} />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => save("draft")}>Save draft</Button>
            <Button onClick={() => save("published")} variant="default">Save &amp; publish</Button>
            {article && (
              <Button variant="secondary" onClick={togglePublish}>
                {article.status === "draft" ? "Publish now" : "Revert to draft"}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
EOF

cat > web/src/pages/admin/kb/Categories.tsx <<'EOF'
import { useEffect, useState } from "react";
import { CategoryAdmin } from "@/components/admin/kb/CategoryAdmin";
import { listKBCategories } from "@/api/kbAdmin";
import type { KBCategory } from "@/lib/types";

export function KBAdminCategories() {
  const [initial, setInitial] = useState<KBCategory[] | null>(null);
  useEffect(() => { listKBCategories().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">KB Categories</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <CategoryAdmin initial={initial} />}
    </section>
  );
}
EOF

cat > web/src/pages/admin/kb/Tags.tsx <<'EOF'
import { useEffect, useState } from "react";
import { TagAdmin } from "@/components/admin/kb/TagAdmin";
import { listKBTags } from "@/api/kbAdmin";
import type { KBTagWithCount } from "@/lib/types";

export function KBAdminTags() {
  const [initial, setInitial] = useState<KBTagWithCount[] | null>(null);
  useEffect(() => { listKBTags().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">KB Tags</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <TagAdmin initial={initial} />}
    </section>
  );
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/pages/admin/kb/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): kb admin pages — List / Edit / Categories / Tags"
```

---

## Phase L — Final wiring + smoke

### Task L1: App.tsx dispatcher for new modes

**Files:**
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Replace the dispatcher**

```bash
cat > web/src/App.tsx <<'EOF'
import { Toaster } from "@/components/ui/sonner";
import { readBootstrap } from "@/lib/bootstrap";
import { AdminHome } from "@/pages/AdminHome";
import { CustomerHome } from "@/pages/CustomerHome";
import { KBBrowse } from "@/pages/kb/Browse";
import { KBDetail } from "@/pages/kb/Detail";
import { KBAdminList } from "@/pages/admin/kb/List";
import { KBAdminEdit } from "@/pages/admin/kb/Edit";
import { KBAdminCategories } from "@/pages/admin/kb/Categories";
import { KBAdminTags } from "@/pages/admin/kb/Tags";

export function App() {
  const bootstrap = readBootstrap();
  let page: React.ReactNode;
  switch (bootstrap.mode) {
    case "admin-home":            page = <AdminHome bootstrap={bootstrap} />; break;
    case "kb-browse":             page = <KBBrowse bootstrap={bootstrap} />; break;
    case "kb-detail":             page = <KBDetail />; break;
    case "admin-kb-list":         page = <KBAdminList />; break;
    case "admin-kb-edit":         page = <KBAdminEdit />; break;
    case "admin-kb-categories":   page = <KBAdminCategories />; break;
    case "admin-kb-tags":         page = <KBAdminTags />; break;
    default:                       page = <CustomerHome bootstrap={bootstrap} />;
  }
  return (
    <>
      {page}
      <Toaster />
    </>
  );
}
EOF
```

- [ ] **Step 2: Build + test**

```bash
cd web && pnpm test && pnpm build
cd ..
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/App.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): App dispatches kb-* + admin-kb-* modes"
```

---

### Task L2: Flip shipped flags + default

**Files:**
- Modify: `web/src/lib/modules.ts`
- Modify: `internal/runtime/runtime.go`
- Modify: `internal/runtime/runtime_test.go`

- [ ] **Step 1: Flip the SPA constant**

```bash
sed -i 's/kb: false/kb: true/' web/src/lib/modules.ts
grep 'kb:' web/src/lib/modules.ts
```

- [ ] **Step 2: Flip the Go default**

Edit `internal/runtime/runtime.go`. `DefaultAppConfig()` becomes:

```go
func DefaultAppConfig() Config {
	return Config{Modules: ModuleToggles{KB: true}}
}
```

- [ ] **Step 3: Update the runtime test**

Edit `internal/runtime/runtime_test.go`. Replace the `TestConfigureDefaultsAllModulesOff` assertion block with:

```go
	if !observed.Modules.KB {
		t.Fatalf("KB should default ON now that the module ships; got %+v", observed.Modules)
	}
	if observed.Modules.Speedtest || observed.Modules.Tickets || observed.Modules.AI {
		t.Fatalf("non-shipped modules should still default off; got %+v", observed.Modules)
	}
```

Rename the test to `TestConfigureDefaultsKBOnOthersOff` to keep the name truthful.

- [ ] **Step 4: Verify**

```bash
cd web && pnpm test && pnpm exec tsc -b --noEmit
cd ..
go test ./...
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/lib/modules.ts internal/runtime/runtime.go internal/runtime/runtime_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat: ship kb — flip SHIPPED_MODULES.kb + Modules.KB default true"
```

---

### Task L3: README + final smoke

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README**

```bash
cat > README.md <<'EOF'
# Continuum Support Plugin

`continuum.support` is the customer-facing support surface for a
Continuum deployment.

**Shipped modules:**

| Module | Status |
|---|---|
| Knowledge Base | Shipped (v0.2) |
| Speedtest | Coming soon |
| Tickets | Coming soon |
| AI Assistance | Coming soon |

See `docs/superpowers/specs/` for designs and `docs/superpowers/plans/`
for implementation plans.

## Build

```sh
make build
make test
```

`make test` runs Go tests + the vitest SPA suite. Some KB tests are
gated on `PG_DSN` (a Postgres connection string) for integration
coverage; without it those tests skip cleanly.

## Configuration

Requires `database_url` — a Postgres DSN, e.g.
`postgres://plugin_support:...@host:5432/continuum?search_path=support&sslmode=disable`.

The plugin manages its own schema; the operator only needs to create
the schema and grant connect rights.

## KB events emitted

The KB module publishes lifecycle events to the host bus, which
`continuum.notifications` routes per its admin rules:

- `plugin.continuum.support.kb_article_published`
- `plugin.continuum.support.kb_article_updated`
- `plugin.continuum.support.kb_article_unhelpful`

## KB cron

Scheduled publishing + unhelpful-article detection run via the daily
admin button `POST /api/admin/kb/cron/run`. A native `scheduled_task.v1`
SDK capability is a follow-up.
EOF
```

- [ ] **Step 2: make build + make test**

```bash
make build
make test
```

- [ ] **Step 3: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add README.md
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "docs: README — kb module shipped"
```

---

### Task L4: Push

```bash
git push 2>&1 | tail -5
```

---

## Self-Review

**Spec coverage** (against `2026-05-21-support-kb-design.md`):

- All 7 KB tables + FTS column + indexes → Task A1.
- `body_text` derivation from sanitised HTML → Task A3.
- Slug derivation + collision suffix → Task C1.
- Image-ref extraction for orphan adoption → Task C2.
- Category / tag / article CRUD + lifecycle → Tasks B1, B2, B3, E1.
- Slug-unique vote upsert + 24h view dedup → Tasks B5, D1.
- Image upload 5 MB / MIME / SVG safety → Task F1.
- FTS search + related articles → Tasks B4, D1.
- Events via runtimehost → Task G1.
- Cron + admin-trigger fallback → Task G2.
- Customer SPA (browse / search / detail / vote / related) → Tasks J5, J6 + J3, J4.
- Admin SPA (list / Tiptap editor / categories / tags / engagement aggregate) → Tasks K3, K4, K5.
- Shell adjustments (SHIPPED_MODULES, DefaultAppConfig.KB, manifest routes + version) → Tasks H1, L2.
- App dispatcher → Task L1.
- TrustedHTML for body AND search snippet → Tasks J5, J6.
- Cross-module: tickets-aware "Open a ticket" CTA → Task J5.

**Coverage gap noted:** the spec mentions a per-article engagement view with "views over 30 days" and "thumbs ratio over time" — the implementation ships the aggregate (not a time series). Note this in the v1 README or the spec's "Out of Scope" follow-up section; not a hidden gap.

**Type / method-name consistency:**

- Go: `KBArticle` / `KBCategory` / `KBTag` / `KBImage` / `KBVoteAggregate` / `KBViewAggregate` consistent.
- Go store methods consistently prefixed `KB*`.
- Event names: `kb_article_published` / `_updated` / `_unhelpful` match spec.
- TS types mirror Go shapes via JSON-tag camelCase.
- TS bootstrap modes match the Go-side strings.

**Placeholder scan:** searched the plan for "TODO" / "TBD" / "implement later" / "Similar to Task" — none present.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-21-support-kb.md`. Two execution options:

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, two-stage review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using `executing-plans`, batch execution with checkpoints.

Which approach?
