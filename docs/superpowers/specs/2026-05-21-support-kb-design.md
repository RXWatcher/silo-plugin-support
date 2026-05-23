# Support Plugin — Knowledge Base Module v1 Design

**Status:** Sub-project design under the program spec
([`2026-05-21-support-plugin-program-design.md`](2026-05-21-support-plugin-program-design.md)).
Second module to ship after the shell, per the program ship order
(shell → **KB** → speedtest → tickets → AI).

**Date:** 2026-05-21
**Sub-project:** Knowledge Base module v1
**Predecessor:** Shell ([`2026-05-21-support-shell-design.md`](2026-05-21-support-shell-design.md))
**Successor:** Speedtest module

## Purpose

Operator-authored articles for customer self-serve. Cuts ticket volume
by giving customers answers before they need to ask. Plugged into the
support plugin's existing shell — adds two customer surfaces (browse
+ detail), one admin surface (article CRUD + taxonomy admin), a
scheduled-publish cron, and events that the existing
`silo.notifications` plugin routes to whatever delivery channels
the operator configured.

## Decisions Locked During Brainstorm

- **WYSIWYG editor — Tiptap.** Non-technical staff may write articles.
  Storage is sanitised HTML (Tiptap-native), not Markdown. A derived
  `body_text` column powers FTS without parsing HTML at query time.
- **Search backend — Postgres FTS** with a `tsvector` generated column
  and GIN index. Plus a "Related articles" panel on each detail page,
  using FTS similarity against the article's own search vector.
- **Taxonomy — flat categories + free-form tags.** Each article picks
  exactly one category and zero-or-more tags. New tags are created on
  first use from the admin tag picker (no separate "create tag"
  workflow needed in v1).
- **Inline image upload — Postgres bytea, 5 MB per file.** Same
  storage strategy as ticket attachments will use, just a smaller cap
  because article images don't need to be huge. Served via
  `/api/kb/images/{id}` to both customers and admins.
- **Scheduled publishing** via `publish_at` timestamp + a daily cron
  pass. SDK `scheduled_task.v1` capability if available; admin button
  fallback otherwise (same as the tickets-module cron fallback).
- **Engagement — per-customer thumbs + per-view rows.** Dedupe votes
  with a (article_id, customer_id) PK; record one view row per
  customer per article per 24h. Admin gets distinct-viewer and
  thumbs-ratio numbers without raw PII beyond `customer_id`.
- **Slug-based URLs** (`/kb/getting-started`). Auto-derived from
  title via the standard lowercase-kebab pass (strip diacritics,
  replace non-alphanumeric runs with `-`, trim leading/trailing
  `-`). Admin can override before save. Unique-indexed; on
  collision the server appends `-2`, `-3`, … and surfaces the
  final value back in the response so the admin sees what was
  actually used. Same collision policy applies to `kb_tags.slug`
  (auto-derived from the label on first use).
- **Notifications via events**, no SMTP/push config in this module.

## Schema (extends the `support` Postgres schema)

```sql
-- Taxonomy --------------------------------------------------------------
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
    slug        TEXT NOT NULL UNIQUE,   -- lowercase-kebab; derived on create
    label       TEXT NOT NULL,           -- operator-supplied display form
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Articles --------------------------------------------------------------
CREATE TABLE kb_articles (
    id              BIGSERIAL PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    body_html       TEXT NOT NULL,
    body_text       TEXT NOT NULL,                       -- HTML-stripped, for FTS
    category_id     BIGINT NOT NULL REFERENCES kb_categories(id) ON DELETE RESTRICT,
    status          TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','published')),
    publish_at      TIMESTAMPTZ,                          -- when draft should auto-publish
    published_at    TIMESTAMPTZ,                          -- set when status becomes 'published'
    last_edited_by  TEXT NOT NULL,                        -- X-Silo-User-Id snapshot
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    search_vector   tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title,'')),   'A') ||
        setweight(to_tsvector('english', coalesce(summary,'')), 'B') ||
        setweight(to_tsvector('english', coalesce(body_text,'')), 'C')
    ) STORED
);
CREATE INDEX kb_articles_search_idx     ON kb_articles USING GIN (search_vector);
CREATE INDEX kb_articles_category_idx   ON kb_articles (category_id, status, published_at DESC);
CREATE INDEX kb_articles_schedule_idx   ON kb_articles (publish_at) WHERE status = 'draft';

CREATE TABLE kb_article_tags (
    article_id  BIGINT NOT NULL REFERENCES kb_articles(id) ON DELETE CASCADE,
    tag_id      BIGINT NOT NULL REFERENCES kb_tags(id)     ON DELETE RESTRICT,
    PRIMARY KEY (article_id, tag_id)
);
CREATE INDEX kb_article_tags_tag_idx ON kb_article_tags(tag_id);

-- Inline images (Tiptap upload target). 5 MB cap enforced at the handler.
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

-- Engagement ------------------------------------------------------------
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
CREATE INDEX kb_views_article_idx  ON kb_views (article_id, viewed_at DESC);
CREATE INDEX kb_views_dedup_idx    ON kb_views (article_id, customer_id, viewed_at DESC);
```

Notes:

- `search_vector GENERATED ALWAYS AS … STORED` keeps the FTS index
  in sync without triggers. Re-saving an article re-derives it.
- `body_text` is derived server-side on save from `body_html` (HTML
  walker, not regex). Stored separately so FTS doesn't have to strip
  tags at query time.
- `kb_images.article_id` nullable + `ON DELETE SET NULL` so an image
  uploaded during a "new article" draft (before the article row
  exists) can be orphaned-but-recoverable. A weekly cleanup pass
  (out of v1) reaps `article_id IS NULL AND created_at < now() - 7d`.
- `kb_views_dedup_idx` supports the 24h view-dedup check (insert a
  new row only if no row in the last 24h for the same customer +
  article).

## Article Lifecycle

```
                ┌────────────────┐
                │     draft      │  ← admin creates / saves
                │ (publish_at?)  │
                └──────┬─────┬───┘
                       │     │ admin: publish now
                       │     ▼
                       │   ┌────────────────────┐
                       │   │     published      │  ← visible to customers
                       │   │ published_at = NOW │
                       │   └─────────┬──────────┘
   cron daily:                       │ admin: revert to draft
   publish_at <= NOW                 ▼
   AND status='draft'           ┌────────────────┐
                                │     draft      │
                                │ (re-edit cycle)│
                                └────────────────┘
```

- A `published` article that's reverted to `draft` keeps its
  `published_at` (historical). Re-publishing updates `published_at`
  again.
- The cron only acts on `draft` rows where `publish_at <= NOW()`.
  Manual "publish now" clears `publish_at`.

## Customer UX

Sidebar entry "Knowledge Base" on the customer SPA (surfaced by the
shell once `SHIPPED_MODULES.kb` AND `modules.kb` are both true; the
shell's `ModuleCard` renders the right state automatically).

### Browse page (`mode: kb-browse`, route `GET /kb`)

```
┌───────────────────────────────────────────────────────────────┐
│  Knowledge Base                                                │
│  Search articles, FAQs, and how-tos.                           │
├───────────────────────────────────────────────────────────────┤
│  [search ...                                            🔍  ]  │
│  Tags: [ all ] [ beginner ] [ video ] [ mobile ] [ billing ]   │
├───────────────────────────────────────────────────────────────┤
│  Streaming                                       See all →     │
│   • Why is my video buffering?                                 │
│   • How to switch quality manually                             │
│   • Supported devices                                          │
│                                                                │
│  Billing                                         See all →     │
│   • Reading your invoice                                       │
│   • Updating payment methods                                   │
│   • Cancellation policy                                        │
│                                                                │
│  Account                                         See all →     │
│   ...                                                          │
└───────────────────────────────────────────────────────────────┘
```

- Search bar debounces 250 ms; hits `/api/customer/kb/search?q=`.
  Showing results replaces the categorised list with a single
  ranked list until the customer clears the query.
- Tag chips along the top: clicking one filters the current view
  (categorised list or search results) to articles bearing that
  tag. Chip state lives in the URL (`?tag=mobile`) so deep links
  work.
- Empty-state CTA when search has no hits: "Can't find what you
  need? Open a ticket →" (only when the tickets module is enabled —
  detected via `bootstrap.modules.tickets`).

### Detail page (`mode: kb-detail`, route `GET /kb/{slug}`)

```
┌────────────────────────────────────────────────────────────────┐
│  ← Back to Knowledge Base                                       │
│                                                                 │
│  Why is my video buffering?                                     │
│  [Streaming]  [beginner]  [video]      Updated 2 days ago      │
│                                                                 │
│  ... sanitised HTML body, including <img> served from           │
│  /api/kb/images/{id}, links to other modules ...                │
│                                                                 │
│  ─────────────────────────────────────────────────────────────  │
│  Was this helpful?    [👍 helpful]   [👎 not helpful]            │
│                                                                 │
│  Related articles                                               │
│  • How to switch quality manually                               │
│  • Supported devices                                            │
│  • Troubleshooting connection issues                            │
└────────────────────────────────────────────────────────────────┘
```

- Body is rendered via the shell's `TrustedHTML` (DOMPurify) component
  — defence in depth, even though the server sanitises on save.
- "Was this helpful?" buttons show the customer's existing vote (if
  any) as pressed-in. Clicking flips/sets the vote via
  `POST /api/customer/kb/articles/{slug}/vote`.
- View row recorded on first paint per customer per article per 24h
  (handler-side dedup query).

## Admin UX

Sidebar entry "Knowledge Base" on the admin SPA, becomes a live
anchor when `SHIPPED_MODULES.kb && modules.kb`; otherwise an in-page
"(coming soon)" / "(disabled)" placeholder per the shell's rule.

### Article list (`mode: admin-kb-list`, route `GET /admin/kb`)

| Title | Category | Tags | Status | Updated | Views (30d) | Helpful |
|---|---|---|---|---|---|---|
| Why is my video buffering? | Streaming | beginner, video | Published | 2d ago | 412 | 87 % |
| New: invoice format change | Billing | billing | Scheduled (in 3d) | 4h ago | — | — |
| ... | | | | | | |

Filters above the table: status, category, tag, search by title.

### Article editor (`mode: admin-kb-edit`, route `GET /admin/kb/{id}` or `/admin/kb/new`)

- Title input (with live slug preview underneath, editable).
- Category dropdown (from `kb_categories WHERE active`).
- Tag picker: chip-style. Type to create a new tag (slug
  auto-derived, label = the typed text).
- Summary textarea (160-char soft limit, shown in search snippets +
  related-articles cards).
- Tiptap WYSIWYG body. Image upload via toolbar button or drag-drop.
  Allowed nodes: heading 1-3, paragraph, bold, italic, code, code
  block, link, bullet list, ordered list, image, blockquote.
- Status group: `[ Draft | Scheduled | Publish now ]`. Scheduling
  reveals a datetime input.
- Save (always), Publish (status → published), Schedule (status
  stays draft, sets publish_at).

### Categories admin (sub-section `mode: admin-kb-categories`)

Table with drag-to-reorder, inline rename, soft-delete via
`active = false`. A category can only be hard-deleted if no articles
reference it.

### Tags admin (sub-section `mode: admin-kb-tags`)

List view with usage count per tag. Rename, merge (move all
`kb_article_tags` rows from tag A to tag B, then delete A), hard
delete (only when usage = 0).

## Routes (added to the support manifest at this module's release)

| Route | Access | Purpose |
|---|---|---|
| `GET /kb` | user | SPA shell, `mode: kb-browse` |
| `GET /kb/{slug}` | user | SPA shell, `mode: kb-detail` |
| `GET /api/customer/kb/articles` | user | List, filterable (category, tag, status=published implicit) |
| `GET /api/customer/kb/articles/{slug}` | user | Detail (records a view row, deduped 24h) |
| `GET /api/customer/kb/related/{slug}` | user | Top-3 related (FTS similarity, same category bias) |
| `GET /api/customer/kb/search?q=` | user | FTS results |
| `POST /api/customer/kb/articles/{slug}/vote` | user | up / down (upsert into `kb_votes`) |
| `GET /api/kb/images/{id}` | user or admin | Serve image bytes (inline) |
| `GET /admin/kb` | admin | SPA shell, `mode: admin-kb-list` |
| `GET /admin/kb/new` | admin | SPA shell, `mode: admin-kb-edit` (empty article) |
| `GET /admin/kb/{id}` | admin | SPA shell, `mode: admin-kb-edit` |
| `GET /admin/kb/categories` | admin | SPA shell, `mode: admin-kb-categories` |
| `GET /admin/kb/tags` | admin | SPA shell, `mode: admin-kb-tags` |
| `GET /api/admin/kb/articles` | admin | List (admin filters: status, category, tag, q) |
| `POST /api/admin/kb/articles` | admin | Create |
| `GET /api/admin/kb/articles/{id}` | admin | Read |
| `PUT /api/admin/kb/articles/{id}` | admin | Update |
| `DELETE /api/admin/kb/articles/{id}` | admin | Delete |
| `POST /api/admin/kb/articles/{id}/publish` | admin | Publish now |
| `POST /api/admin/kb/articles/{id}/unpublish` | admin | Revert to draft |
| `GET /api/admin/kb/articles/{id}/engagement` | admin | Views + thumbs aggregates for chart |
| `GET  /api/admin/kb/categories` | admin | List |
| `POST /api/admin/kb/categories` | admin | Create |
| `PUT  /api/admin/kb/categories/{id}` | admin | Update (rename, reorder, toggle active) |
| `DELETE /api/admin/kb/categories/{id}` | admin | Hard-delete (rejected with 409 if any articles still reference it) |
| `GET  /api/admin/kb/tags` | admin | List with usage counts |
| `POST /api/admin/kb/tags` | admin | Create (rarely needed — usually auto-created from the picker) |
| `PUT  /api/admin/kb/tags/{id}` | admin | Rename |
| `DELETE /api/admin/kb/tags/{id}` | admin | Delete (rejected with 409 if any articles still reference it) |
| `POST /api/admin/kb/tags/merge` | admin | `{from_id, into_id}` |
| `POST /api/admin/kb/images` | admin | Multipart upload, 5 MB cap (uses shell's body cap; an explicit per-handler check rejects > 5 MB before the body fully streams) |

## Events emitted

Base payload includes `article_id`, `slug`, `title`, `category` (slug
and name), `tags` (array of slugs), `deep_link` (e.g. `<mount>/kb/{slug}`).

| Event | Extra keys | When |
|---|---|---|
| `plugin.silo.support.kb_article_published` | `published_at`, `by` (admin id or "system" for scheduled) | First publish or scheduled cron flip |
| `plugin.silo.support.kb_article_updated`   | `changed_by` (admin id), `since` (last update) | Any save to a `published` article |
| `plugin.silo.support.kb_article_unhelpful` | `helpful_ratio_24h`, `threshold`, `votes_24h` | Daily cron: any published article whose last-24h helpful ratio drops below the configured threshold (default 50 %, configurable via `kb.unhelpful_alert_threshold`) and has at least 5 votes in the window. Operator can route this to notifications for "this article may need a rewrite". |

## SPA Bootstrap Modes

Extends the shell's `supportBootstrap.mode`:

- `kb-browse` — pre-bakes active categories + per-category top-N
  recent articles + all tags (for the chip rail)
- `kb-detail` — pre-bakes the article (slug-resolved), the related
  articles (top-3 by FTS), the customer's existing vote
- `admin-kb-list` — pre-bakes filtered article rows (first page) +
  category + tag selects
- `admin-kb-edit` — pre-bakes the article (or empty for `new`) + all
  active categories + all tags
- `admin-kb-categories` — pre-bakes categories with article counts
- `admin-kb-tags` — pre-bakes tags with article counts

## Image Upload Flow

1. Operator drags an image into Tiptap (or hits the toolbar button).
2. Browser POSTs multipart to `/api/admin/kb/images` with optional
   `?article_id=` query for orphan-recovery. Handler validates
   `Content-Length <= 5 MB` (pre-read) AND `len(body) <= 5 MB` (post-
   read defence) — anything over → 413.
3. Allowed MIME types: `image/png`, `image/jpeg`, `image/gif`,
   `image/webp`, `image/svg+xml`. SVG goes through DOMPurify with
   the `USE_PROFILES: { svg: true }` profile server-side before
   storage. Anything else → 415.
4. Server stores in `kb_images`, returns `{ id, url: "/api/kb/images/{id}" }`.
5. Tiptap inserts the `<img src="/api/kb/images/{id}">` into the body.
6. On article save, `body_html` references survive the round-trip
   (Tiptap serialises `<img>` as-is). On article delete, images are
   FK-cascaded.

## Cross-module Integration Points

- **KB → tickets** (when tickets module enabled): "no search hits"
  empty state on the browse page renders a "Open a ticket →"
  button. Operator-authored articles can also include direct
  `/tickets/new?type=billing` links via Tiptap's link UI.
- **KB → speedtest** (when speedtest module enabled): articles about
  "slow streaming" can link to `/speedtest` to let the customer
  self-diagnose. No special handling — regular Tiptap links.
- **tickets → KB** (built into the tickets module's release): at
  ticket creation, FTS over `kb_articles` to suggest top-3 articles
  matching the typed description. KB exposes nothing extra for this
  — tickets just queries directly via the shared store.
- **AI → KB** (built into the AI module's release): embedding
  similarity over articles for richer suggestion than FTS.
  KB exposes nothing extra; AI module reads `kb_articles`.

## Shell Adjustments at This Module's Release

Three small edits when the KB module lands:

1. `web/src/lib/modules.ts`: flip `SHIPPED_MODULES.kb` to `true`.
2. `cmd/.../manifest.json`: append all the routes in the table
   above; bump `version` to `0.2.0`.
3. `internal/runtime/runtime.go DefaultAppConfig()`: flip
   `Modules.KB` default from `false` to `true` so new installs get
   KB on by default. Existing installs keep their operator-set value.
4. `internal/migrate/files/0002_kb_init.up.sql` (+ `.down.sql`):
   create the kb_* tables. Shell's migrate runner picks this up
   automatically — no runner change.

## Scheduled-Publish Cron

Daily task that walks pending publishes:

```go
// internal/kb/cron.go
func PublishDue(ctx context.Context, st *store.Store) error {
    rows, err := st.KBPendingPublishes(ctx) // SELECT id, slug FROM kb_articles
                                            //   WHERE status='draft' AND publish_at <= NOW()
                                            //   AND publish_at IS NOT NULL
                                            //   ORDER BY publish_at
                                            //   LIMIT 100
    if err != nil { return err }
    for _, r := range rows {
        if err := st.KBPublishArticle(ctx, r.ID); err != nil { log; continue }
        publisher.Publish("plugin.silo.support.kb_article_published", ...)
    }
    return nil
}
```

Wired into `scheduled_task.v1` if the SDK supports the capability
(verify against the notifications plugin which already declares
this capability). Fallback: an admin button "Publish scheduled
drafts now" runs the same function.

The same cron pass computes the `kb_article_unhelpful` event for
articles below threshold.

## Tests

### Go

- Article CRUD round-trip (create → read → update → delete).
- Slug uniqueness; auto-derivation; collision suffix.
- Body sanitisation: SVG with `<script>` is stripped; regular HTML
  passes through.
- `body_text` derivation: HTML with nested tags reduces to
  whitespace-collapsed plain text.
- FTS search: exact title hit ranks above body hit; published-only
  filter excludes drafts.
- Related articles: same-category bias works; never returns the
  source article.
- Vote upsert: same customer voting twice updates, doesn't insert.
- View dedup: two requests within 24h yield one row; outside the
  window yields a second.
- Scheduled-publish cron: dry-run lists the right candidates;
  status flips; event fires.
- Image upload: 4 MB succeeds, 6 MB → 413, non-image MIME → 415,
  SVG with `<script>` is rejected/stripped.
- Lifecycle transitions: only draft → published, only published →
  draft, all others → 409.
- Authorisation: customer GET on a draft article → 404 (not 403,
  to avoid revealing existence).

### SPA (vitest + testing-library)

- bootstrap parse for each new mode
- browse page renders categories from bootstrap + tag-chip filter
  toggles the URL `?tag=` param
- search bar debounce + result render + empty-state CTA only when
  tickets module is enabled
- detail renders body via TrustedHTML; vote buttons call the API
  with correct payload; "your existing vote" displays as pressed-in
- admin article editor: Tiptap renders with a stubbed image-upload
  mock; saving sends the right shape; "Schedule" reveals the
  datetime input
- admin list filters apply

## Out of Scope for v1

- Article version history / diff
- Per-edit attribution beyond `last_edited_by` (one author tracked)
- Multi-locale / translation
- Customer Q&A / comments on articles
- AI-assisted article writing
- Article import / export (Markdown, HTML, JSON)
- Image processing (auto-resize, srcset, format conversion)
- A "what links here" panel on the admin article view
- Customer "saved articles" / bookmarks
- Sitemap / SEO metadata (KB is auth-only anyway)
- Public surface — KB stays behind Silo login

## Success Criteria

- Operator creates an article in WYSIWYG with an inline image, sets
  `publish_at` for tomorrow, sees it auto-publish the next day and a
  `kb_article_published` event hit the notifications plugin.
- Customer searches "buffering", finds the relevant article, opens
  it, scrolls, clicks "helpful". The vote persists; reloading the
  page shows "helpful" still pressed in.
- An admin can rename a tag, merge two tags, and the article tags
  follow.
- Customer visits an article 5 times in a day; `kb_views` records
  exactly one row.
- A 6 MB image upload returns 413; a 4 MB PNG succeeds and renders
  in the article body via `/api/kb/images/{id}`.
- Cross-module: a future tickets module can FTS-search `kb_articles`
  via the shared `store.Store` and find published articles only.
- `make build` + `make test` (Go + vitest) all green.
