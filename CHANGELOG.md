# Changelog

All notable changes to the Continuum Support Plugin.

The format follows [Keep a Changelog](https://keepachangelog.com/);
the project loosely follows SemVer where the manifest version maps
to module shipping milestones rather than API stability.

## [0.4.0] — 2026-05-21 — Tickets shipped

### Added

- **Tickets module** — typed-intake support tickets, two-level
  category taxonomy with per-category form fields, SUP-N tracking
  numbers (atomic sequence-row UPDATE…RETURNING), status lifecycle
  (`open / in_progress / waiting_customer / resolved / closed`)
  with customer reopen-within-7-days, append-only entries with
  internal admin notes, 10 MB bytea attachments with ticket-owner /
  admin auth gate, operator-configurable auto-close cron (default
  7d resolved / 14d waiting; either threshold can be set to 0 to
  skip that pass; master enable toggle).
- 8 tickets-side tables: `tk_categories`, `tk_subcategories`,
  `tk_category_fields`, `tk_tickets`, `tk_ticket_entries`,
  `tk_ticket_field_values`, `tk_attachments`, `tk_ticket_sequence`.
- 7 ticket events: `ticket_submitted`, `ticket_replied`,
  `ticket_status_changed`, `ticket_assigned`, `ticket_resolved`,
  `ticket_reopened`, `ticket_closed`.
- Customer SPA pages: list (status tabs), new (3-step flow:
  category → subcategory → form), detail (thread + reply +
  reopen) with 30-second polling.
- Admin SPA pages: queue with filters (status / category /
  assignee / search), detail with action panel + internal notes,
  categories tree editor.
- `tickets_*` config keys for cron behaviour.

### Fixed (post-ship follow-ups)

- `ticket_submitted` and other lifecycle events now load category +
  subcategory into the published payload (previously fired with
  `category: nil` because `TKLoadTicketAux` was not called).
- Admin queue category-filter dropdown added (backend supported it
  all along; only the UI was missing).
- Vitest happy-path test added for the customer new-ticket flow.

## [0.3.0] — 2026-05-21 — Speedtest shipped

### Added

- **Speedtest module** — LibreSpeed-protocol browser test against
  admin-defined endpoints, switchable auto-strategy (`latency` via
  client-side parallel pings vs `geoip` via server-side IP →
  country → matching endpoint), admin-orderable GeoIP source chain
  (`mmdb_auto` / `mmdb_file` / `http_api` / `request_header`),
  db-ip.com country-lite seeded by default, 30-day result history
  with admin dashboards, slow-result event hook.
- 3 speedtest-side tables: `st_endpoints`, `st_geoip_sources`,
  `st_results`.
- 2 speedtest events: `speedtest_run`, `speedtest_slow`.
- Vendored LibreSpeed worker (LGPLv3) + typed TS wrapper.
- Customer SPA: endpoint picker + speed gauge + history list.
- Admin SPA: endpoints CRUD, GeoIP sources drag-reorder + per-kind
  config editor + per-source refresh / test, results table + 30-day
  dashboards.

### Fixed (post-ship follow-ups)

- `MMDBFileSource` now surfaces the open error on every Resolve
  (previously a corrupt mmdb file would silently miss forever).
- `mmdb_auto` lifecycle now has automated tests (download → atomic
  swap → validate; prev-month URL fallback).
- `geoipCacheDir` walks XDG_CACHE_HOME → ~/.cache → relative
  fallback (previously hardcoded to /var/cache).
- `AutoGeoIPHint.SourceLabel` is now populated by the resolver via
  a store-backed label lookup.

## [0.2.0] — 2026-05-21 — Knowledge Base shipped

### Added

- **KB module** — operator-authored articles with Tiptap WYSIWYG +
  inline image upload (5 MB bytea), Postgres FTS via generated
  tsvector column + GIN index, related-articles panel via
  similarity, slug-based URLs with collision suffixing, flat
  categories + free-form tags, scheduled publishing via
  publish_at + cron pass, per-customer thumbs + per-view rows
  (24-hour dedup), draft / published lifecycle.
- 7 KB-side tables: `kb_categories`, `kb_tags`, `kb_articles`,
  `kb_article_tags`, `kb_images`, `kb_votes`, `kb_views`.
- 3 KB events: `kb_article_published`, `kb_article_updated`,
  `kb_article_unhelpful`.
- Customer SPA: browse (categories + search + tag chips), detail
  (TrustedHTML body + vote + related).
- Admin SPA: article list, Tiptap editor + image upload + schedule,
  categories admin, tags admin (rename / merge / delete),
  engagement aggregate per article.

### Fixed (post-ship follow-ups)

- `kb_article_updated` event now fires on PUT to an already-
  published article (previously silently skipped — contract gap
  with the notifications plugin).
- SVG upload dropped from the allowlist (the byte-scan
  half-sanitiser was a stored-XSS risk; v1.1 can re-add with a
  proper Go SVG sanitiser).
- Customer vote now persists across page reloads — `KBArticle`
  response carries `myVote` populated from `KBGetVote`.

## [0.1.0] — 2026-05-21 — Shell shipped

### Added

- **Shell module** — plugin bootstrap, customer + admin SPA shells
  with `?section=` URL state, app_config singleton + GET/PATCH API,
  body-cap middleware (12 MB), CSP / X-Frame-Options / nosniff /
  Referrer-Policy security headers on every response,
  X-Continuum-User-Id + X-Continuum-User-Role auth gates.
- 1 shell-side table: `app_config` (singleton with `CHECK (id=1)`).
- chi router, golang-migrate via embedded SQL with pgx/v5 driver.
- runtime.Configure RPC + module toggles (ModuleToggles{KB,
  Speedtest, Tickets, AI}).

### Fixed (post-ship follow-ups)

- Module shipped vs enabled distinction: `SHIPPED_MODULES`
  constant in `web/src/lib/modules.ts` separates "this release
  contains the code" (binary-time) from "the admin has the toggle
  on" (runtime). Both AdminOverview and AdminSidebar render
  correctly under all four combinations.
- Two missing SPA interaction tests added (sidebar-click section
  switch, switch-fire onSave assertion).

[0.4.0]: https://github.com/ContinuumApp/continuum-plugin-support/releases/tag/v0.4.0
[0.3.0]: https://github.com/ContinuumApp/continuum-plugin-support/releases/tag/v0.3.0
[0.2.0]: https://github.com/ContinuumApp/continuum-plugin-support/releases/tag/v0.2.0
[0.1.0]: https://github.com/ContinuumApp/continuum-plugin-support/releases/tag/v0.1.0
