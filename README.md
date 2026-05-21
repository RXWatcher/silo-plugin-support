# Continuum Support Plugin

`continuum.support` is the customer-facing support surface for a
Continuum deployment. One Go binary, one embedded React SPA, four
modules under a shared shell.

## Modules

| Module | Status | One-line |
|---|---|---|
| **Shell** (v0.1) | Shipped | Bootstrap, customer + admin SPA shells, config singleton, auth gates |
| **Knowledge Base** (v0.2) | Shipped | Operator-authored articles (Tiptap WYSIWYG), Postgres FTS + related, slug URLs, inline images, scheduled publishing, per-customer thumbs |
| **Speedtest** (v0.3) | Shipped | LibreSpeed-protocol customer tests, admin endpoint catalogue, switchable auto-strategy (latency vs operator-ordered geoip chain), 30-day result history, slow-event hooks |
| **Tickets** (v0.4) | Shipped | Typed-intake categories + per-category form fields, SUP-N tracking numbers, status lifecycle with reopen, append-only entries with internal notes, 10 MB attachments, configurable auto-close cron |
| AI Assistance | Coming soon | KB suggestion on ticket create, embedding similarity over articles |

## Architecture

- **Go binary** built from `cmd/continuum-plugin-support/`. Speaks the Continuum SDK's `HttpRoutes` capability via gRPC; the host plugin runtime mounts this binary's chi router under a per-installation path.
- **Embedded SPA** at `internal/server/public/dist/`, served from the binary. Built from `web/` (Vite + React 19 + Tailwind v4 + shadcn primitives).
- **Single Postgres schema** (`support`) with table prefixes per module: `kb_*`, `st_*`, `tk_*`, plus the singleton `app_config` table the shell owns.
- **Auth** flows through `X-Continuum-User-Id` and `X-Continuum-User-Role` headers that the host plugin injects. Customer routes require user; admin routes require admin.
- **Events** are published to the host's event bus and routed by `continuum.notifications` per the operator's rules. This plugin emits — it never delivers email / push / chat directly.

## Repository layout

```
cmd/continuum-plugin-support/      Plugin entrypoint + manifest
internal/migrate/                  golang-migrate runner + .sql files (0001-0004)
internal/store/                    Postgres-backed types (kb_*, st_*, tk_*)
internal/server/                   chi router, handlers, middleware, SPA serve
internal/runtime/                  Config + Configure RPC + module toggles
internal/htmlx/                    HTML sanitization (KB + future modules)
internal/kb/                       KB-specific helpers (slug, image-refs, cron)
internal/speedtest/                Speedtest helpers (IP truncation, auto resolver)
internal/speedtest/geoip/          GeoIP source kinds + chain walker + mmdb downloader
internal/tickets/                  Tickets lifecycle map + cron
web/                               React SPA source
web/public/speedtest_worker.js     Vendored LibreSpeed worker (LGPLv3)
docs/superpowers/specs/            Per-module designs
docs/superpowers/plans/            Per-module implementation plans
```

## Build

```sh
make build      # vite build then go build → ./continuum-plugin-support
make test       # go test ./... + pnpm -C web test
```

`make test` runs the full suite. PG_DSN-gated integration tests skip
cleanly when `PG_DSN` is unset. MMDB-fixture tests skip cleanly when
no fixture is available (set `MMDB_FIXTURE=/path/to/file.mmdb` or
drop one at `internal/speedtest/geoip/testdata/sample.mmdb`).

## Configuration

Plugin requires `database_url` — a Postgres DSN, e.g.
`postgres://plugin_support:...@host:5432/continuum?search_path=support&sslmode=disable`.
The plugin manages its own schema migrations; the operator only
needs to create the schema and grant connect rights.

| Key | Default | Module | Purpose |
|---|---|---|---|
| `database_url` | (required) | core | Postgres DSN |
| `auto_strategy` | `latency` | speedtest | `latency` or `geoip` — how "Auto" picks the endpoint |
| `client_ip_storage` | `truncated` | speedtest | `truncated` (/24 v4, /48 v6) or `off` |
| `slow_threshold_mbps` | `5` | speedtest | Download below this fires `speedtest_slow` event |
| `geoip_cache_dir` | XDG cache dir | speedtest | Where `mmdb_auto` sources are downloaded |
| `tickets_auto_close_enabled` | `true` | tickets | Master toggle for the auto-close cron |
| `tickets_resolved_close_after_days` | `7` | tickets | Days idle after `resolved` before auto-close (0 = skip) |
| `tickets_waiting_close_after_days` | `14` | tickets | Days idle in `waiting_customer` before auto-close (0 = skip) |

Module toggles live in the singleton `app_config` row managed by
the shell's admin UI; the shipped modules (KB, speedtest, tickets)
default ON. AI defaults OFF until it ships.

## Events emitted

Routed via the existing `continuum.notifications` plugin per admin rules.

- **KB:** `kb_article_published` / `kb_article_updated` / `kb_article_unhelpful`
- **Speedtest:** `speedtest_run` / `speedtest_slow`
- **Tickets:** `ticket_submitted` / `ticket_replied` / `ticket_status_changed` / `ticket_assigned` / `ticket_resolved` / `ticket_reopened` / `ticket_closed`

Event payloads are documented in each module's design spec under
`docs/superpowers/specs/`.

## Crons (admin-trigger endpoints)

The plugin ships without a native `scheduled_task.v1` SDK capability;
each cron is invokable by an admin via:

- KB: `POST /api/admin/kb/cron/run` — publish-due drafts + unhelpful-article sweep
- Tickets: `POST /api/admin/tickets/cron/run` — auto-close idle (configurable)
- GeoIP mmdb refresh: automatic on plugin start; manual via `POST /api/admin/speedtest/geoip/{id}/refresh`

Native scheduled-task wiring is a follow-up across all three; see
`docs/follow-ups.md`.

## Development

```sh
# Run Go tests with a real Postgres (for integration suites)
PG_DSN="postgres://localhost/continuum_dev?sslmode=disable" go test ./...

# Iterate on the SPA standalone (mock bootstrap; auth headers stubbed)
cd web && pnpm dev

# Rebuild + reinstall against a running Continuum host
make build
# then redeploy via your host's plugin install path
```

Specs (designs) live under `docs/superpowers/specs/`. Implementation
plans live under `docs/superpowers/plans/`. Each module followed the
same pipeline: brainstorm → spec → plan → subagent-driven
implementation, all committed.

## License

Plugin source: project license.
Vendored worker (`web/public/speedtest_worker.js`): LGPL-2.1 from
[librespeed/speedtest](https://github.com/librespeed/speedtest).

## See also

- `docs/superpowers/specs/2026-05-21-support-plugin-program-design.md` — program-level decomposition
- `docs/follow-ups.md` — v1.1 follow-ups across all four modules
- `CHANGELOG.md` — release history
