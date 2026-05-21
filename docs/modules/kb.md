# Knowledge Base

Operator-authored support articles, browsed by customers, scored
by their votes. Backing tables `kb_*`. Routes under `/kb`,
`/admin/kb`, `/api/customer/kb`, `/api/admin/kb`,
`/api/kb/images`.

## Admin (operator) quickstart

1. **Create categories.** `Admin → KB → Categories`. Flat
   taxonomy (one level — no nesting). Each article belongs to
   exactly one category (`kb_articles.category_id`,
   `ON DELETE RESTRICT`). Reorder via `sort_order`. Deactivating
   a category hides its articles from the customer list, but does
   not delete anything.

2. **Optionally pre-create tags.** Free-form many-to-many via
   `kb_article_tags`. You can also create tags inline while
   editing an article. Tags admin
   (`Admin → KB → Tags`) supports rename, merge (re-points all
   refs and deletes the old tag in a single transaction), and
   delete.

3. **Write an article.** `Admin → KB → New`. Tiptap WYSIWYG; the
   editor stores HTML in `body_html` and a plain-text projection
   in `body_text`. The plain-text version is what feeds the FTS
   tsvector — weighted A (title), B (summary), C (body).

   - **Inline images.** Drop them in the editor. They POST to
     `/api/admin/kb/images` multipart, get persisted into
     `kb_images.content` (`BYTEA`, capped at 5 MB), and the
     editor inserts an `<img src="/api/kb/images/{id}">`. SVG
     is not in the allowlist post-0.2.0 (see follow-ups).
   - **Status.** Save as draft (`status='draft'`); flip to
     published with `Publish` (`POST /api/admin/kb/articles/{id}/publish`).
   - **Scheduled publishing.** Set `publish_at` on a draft; the
     cron pass (`POST /api/admin/kb/cron/run`) flips the row to
     `published` when `publish_at <= now()` and emits
     `kb_article_published{by:system}`. Without a cron trigger
     the article stays draft forever.
   - **Slug.** Auto-derived from the title; collisions get a
     numeric suffix (`my-article-2`). Editing the title does
     **not** retroactively change the slug — that would break
     bookmarks.

4. **Read engagement.** `Admin → KB → Articles → row` shows the
   aggregate engagement panel: views, helpful, not-helpful, ratio
   bar. Per-customer rows live in `kb_views` (24-hour dedup) and
   `kb_votes` (one row per `(article, customer)`). Time-series
   chart is a v1.1 follow-up.

5. **Run the cron.** Trigger
   `POST /api/admin/kb/cron/run` on a schedule (every 5-15 min is
   reasonable). It does two passes:

   - `PublishDue` — any draft with `publish_at <= now()` flips
     to published and emits `kb_article_published`.
   - `UnhelpfulSweep` — any published article whose last-24h
     vote window has `≥ 5` votes and `helpful_ratio < 0.5`
     emits `kb_article_unhelpful` with `helpful_ratio_24h`,
     `threshold`, and `votes_24h` in `extra`. Defaults are
     hardcoded in `internal/kb/cron.go`.

## Customer (end-user) surface

`/kb` — browse: search bar, category list, tag chips. Hits
`GET /api/customer/kb/articles` (paged) and
`GET /api/customer/kb/search?q=...` (FTS via
`websearch_to_tsquery`).

`/kb/{slug}` — detail: title, summary, body (rendered as
`TrustedHTML`), per-article vote control, related-articles panel.

- **Vote.** `POST /api/customer/kb/articles/{slug}/vote` with
  `{"vote":"up"|"down"}`. Persisted to `kb_votes` keyed by
  `(article_id, customer_id)`; upsert semantics. The detail
  payload carries `myVote` so the UI shows the previous vote on
  reload (fixed in 0.2.0).
- **Related.** `GET /api/customer/kb/related/{slug}` runs an FTS
  similarity query against the current article's tsvector. No
  separate "related links" table.
- **Views.** Every detail GET inserts a row into `kb_views`
  unless the same `(article, customer)` has a view in the last
  24 hours.

## Events

| Event | Trigger | Payload notes |
| --- | --- | --- |
| `kb_article_published` | Manual publish, scheduled publish via cron | `extra.by = admin_id` or `"system"` |
| `kb_article_updated` | PUT to an already-published article | Fixed in 0.2.0; pre-0.2.0 silently skipped |
| `kb_article_unhelpful` | Cron `UnhelpfulSweep` | `extra.helpful_ratio_24h`, `threshold`, `votes_24h` |

## Operator gotchas specific to KB

- Deleting a category fails if any article references it
  (`ON DELETE RESTRICT`). Reassign first.
- Deleting an article cascades to `kb_article_tags`, `kb_votes`,
  `kb_views`, but **sets `kb_images.article_id` to NULL** — the
  images outlive their article. Manual cleanup if you care.
- Re-publishing a draft preserves the original `published_at`;
  the timestamp is set on first publish and not touched again.
  `updated_at` moves on every save.
