# Architecture

The plugin is one Go binary plus one embedded React SPA, fronted
by chi. The SDK's runtime delivers a `Configure` RPC; on first
configure the plugin opens its Postgres pool, runs embedded
migrations, mounts the chi router, and starts serving the SPA and
its JSON APIs.

## Shared shell + four modules

```
┌──────────────────────────────────────────────────────────────┐
│  cmd/silo-plugin-support  (main: wiring, onConfig)      │
├──────────────────────────────────────────────────────────────┤
│  internal/runtime       SDK runtime, app_config defaults     │
│  internal/migrate       golang-migrate, embedded SQL         │
│  internal/store         pgx/v5, per-prefix store files       │
│  internal/httproutes    pluginv1.HttpRoutes server adapter   │
│  internal/server        chi router, handlers, middleware     │
│  internal/htmlx         SPA shell HTML rendering             │
│                                                              │
│  internal/kb            KB module non-handler logic (cron,   │
│                         slug, image refs)                    │
│  internal/speedtest     auto-resolver, GeoIP chain, MMDB     │
│  internal/tickets       lifecycle transitions, auto-close    │
│                         cron                                 │
└──────────────────────────────────────────────────────────────┘
```

The four modules — Shell, KB, Speedtest, Tickets — share the
shell's bootstrap, auth gates, body cap, security headers, and
`app_config` singleton. Each module owns its own table prefix,
event names, customer + admin SPA sections, and (where relevant)
its own cron pass. A planned fifth module, AI Assistance, is not
started yet — see [`follow-ups.md`](./follow-ups.md).

## Routing

Routes live in [`internal/server/server.go`](../internal/server/server.go).
chi mounts everything at the host-assigned per-installation path;
the plugin itself doesn't know that prefix. Three families:

- `GET /`, `GET /admin`, `GET /api/admin/config`, `PATCH
  /api/admin/config`, `GET /assets/*` — shell.
- `/kb/*`, `/api/customer/kb/*`, `/admin/kb/*`,
  `/api/admin/kb/*`, `/api/kb/images/*` — KB.
- `/speedtest`, `/api/customer/speedtest/*`,
  `/admin/speedtest/*`, `/api/admin/speedtest/*` — Speedtest.
- `/tickets/*`, `/api/customer/tickets`,
  `/api/customer/categories`, `/admin/tickets/*`,
  `/api/admin/tickets/*`, `/api/admin/categories/*`,
  `/api/admin/subcategories/*`, `/api/admin/category-fields/*`,
  `/api/tickets/entries/*`, `/api/attachments/*` — Tickets.

`r.Use(limitBody(12 << 20))` caps every inbound request body at
12 MB. KB image upload further self-limits at 5 MB; ticket
attachments self-limit at 10 MB.

## Auth

The host injects two headers on every request after authentication:

- `X-Silo-User-Id` — present means "authenticated user". Empty
  on `requireUser` → 401 `unauthenticated`.
- `X-Silo-User-Role` — `admin` means "elevated". Anything else
  (including missing) on `requireAdmin` → 403 `forbidden`.

The plugin actively strips any incoming `X-Silo-*` header on
the `ServeHTTP` path before invoking chi, so a misconfigured edge
can never spoof these from the outside. Internally, handlers read
the headers directly via `r.Header.Get(...)`.

## Database schema layout

The plugin lives in a dedicated Postgres schema (typically
`support`, set via `?search_path=support` on the DSN). Within
that schema, tables are prefixed by module:

| Prefix       | Module     | Tables |
| ------------ | ---------- | ------ |
| `app_config` | Shell      | `app_config` (singleton with `CHECK (id=1)`) |
| `kb_*`       | KB         | `kb_categories`, `kb_tags`, `kb_articles`, `kb_article_tags`, `kb_images`, `kb_votes`, `kb_views` |
| `st_*`       | Speedtest  | `st_endpoints`, `st_geoip_sources`, `st_results` |
| `tk_*`       | Tickets    | `tk_categories`, `tk_subcategories`, `tk_category_fields`, `tk_tickets`, `tk_ticket_entries`, `tk_ticket_field_values`, `tk_attachments`, `tk_ticket_sequence` |

Migrations are embedded SQL files under `internal/migrate/files/`,
applied in numeric order on every `Configure` call via
`golang-migrate` over a pgx/v5 driver. The plugin owns the schema
exclusively — no other plugin and no operator should write to it.

Notable schema choices worth knowing for debugging:

- `kb_articles.search_vector` is a `GENERATED ALWAYS AS ... STORED`
  tsvector (title^A / summary^B / body_text^C) with a GIN index.
  Articles do not need a separate reindex pass.
- `kb_articles_schedule_idx` is partial (`WHERE status='draft'`)
  so the publish-due query is cheap regardless of article count.
- `tk_tickets.status` is constrained to the lifecycle states; no
  application-level enum exists. New states require a migration.
- `tk_ticket_sequence` is a singleton row used to allocate the
  SUP-N number atomically (`UPDATE ... SET next_n = next_n + 1
  RETURNING next_n`). It is monotonic and never recycled.
- Attachments and KB images are stored as `BYTEA` inline.

## Module toggles

The `Config` struct (in `internal/runtime/runtime.go`) carries
`ModuleToggles{KB, Speedtest, Tickets, AI}`, persisted into
`app_config.data`. `DefaultAppConfig()` flips KB / Speedtest /
Tickets to true on first run; AI stays false until it ships.

The SPA reads `SHIPPED_MODULES` (in `web/src/lib/modules.ts`) to
decide which sections to render in the sidebar. The combination
matters:

| Shipped | Enabled | Result |
| ------- | ------- | ------ |
| no      | n/a     | section hidden |
| yes     | no      | section greyed in admin overview, hidden from customer |
| yes     | yes     | section fully active |

Toggling a module off does **not** drop tables, run migrations
down, or hide existing rows — it only hides routes/UI for that
module from new traffic. Re-enabling it is instantaneous.

## Event bus

The plugin emits events to the host event bus via the SDK; it does
not deliver them. Routing those events into email, push, or chat
is the operator's job (typically via
[`silo.notifications`](https://github.com/RXWatcher/silo-plugin-notifications)).

Current event names:

- KB: `kb_article_published`, `kb_article_updated`,
  `kb_article_unhelpful`.
- Speedtest: `speedtest_run`, `speedtest_slow`.
- Tickets: `ticket_submitted`, `ticket_replied`,
  `ticket_status_changed`, `ticket_assigned`, `ticket_resolved`,
  `ticket_reopened`, `ticket_closed`.

Lifecycle events include the loaded ticket plus category and
subcategory — `TKLoadTicketAux` is called before the payload is
assembled (this used to be a contract gap; see the 0.4.0 fix in
the CHANGELOG).

## Scheduled work

The SDK does not yet expose a `scheduled_task.v1` capability, so
each module's cron is an admin-button endpoint that the operator
calls (or a host-level scheduler hits) on a cadence:

- `POST /api/admin/kb/cron/run` — publish-due + unhelpful sweep.
- `POST /api/admin/speedtest/geoip/{id}/refresh` — per-source MMDB
  refresh; the `mmdb_auto` kind self-refreshes on its
  `refresh_days` cadence but the endpoint forces a refresh now.
- `POST /api/admin/tickets/cron/run` — resolved-idle + waiting-idle
  auto-close pass.

When the native capability ships, all three should declare it and
the endpoints stay as manual-trigger fallbacks. See
[`follow-ups.md`](./follow-ups.md#cross-cutting).

## Security defaults

Every response gets the headers set by `securityHeaders`:

```
X-Content-Type-Options:  nosniff
Referrer-Policy:         no-referrer
X-Frame-Options:         DENY
Permissions-Policy:      camera=(), microphone=(), geolocation=()
Content-Security-Policy: base-uri 'none'; frame-ancestors 'none'
```

The CSP is intentionally narrow — only `base-uri` and
`frame-ancestors`. The plugin's own SPA inlines its bundle hash;
operators do not need to relax CSP on the host side to embed it.
