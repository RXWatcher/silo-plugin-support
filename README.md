# Support for Silo

`silo.support` is the customer-facing support surface for a Silo deployment — one Go binary plus an embedded React SPA serving four modules under a shared shell. Versioned 0.4.0; the manifest version maps to module shipping milestones rather than API stability.

## Category

Lives under **Operations** alongside [`silo.notifications`](https://github.com/RXWatcher/silo-plugin-notifications) and [`silo.stream-dashboard`](https://github.com/RXWatcher/silo-plugin-stream-dashboard).

## Capabilities

| Type | ID | Purpose |
| --- | --- | --- |
| `http_routes.v1` | `support` | Customer + admin SPA shell mounting Knowledge Base, Speedtest, and Tickets modules; exposes navigable customer and admin entry points plus per-module JSON APIs and asset routes. |

The capability registers ~45 routes spanning the shared shell (`/`, `/admin`, `/api/admin/config`), the KB module (`/kb/*`, `/api/customer/kb/*`, `/api/admin/kb/*`, `/api/kb/images/*`), the Speedtest module (`/speedtest`, `/api/customer/speedtest/*`, `/admin/speedtest/*`, `/api/admin/speedtest/*`), and the Tickets module (`/tickets/*`, `/api/customer/tickets`, `/api/customer/categories`, `/admin/tickets/*`, `/api/admin/tickets/*`, `/api/admin/categories/*`, `/api/admin/subcategories/*`, `/api/admin/category-fields/*`, `/api/tickets/entries/*`, `/api/attachments/*`).

## Dependencies

Standalone. The plugin is a self-contained customer-facing surface — it mounts its own chi router behind the host's per-installation path and embeds its SPA build. It emits events to the host's event bus; routing those events into email / push / chat is the operator's responsibility (typically via [`silo.notifications`](https://github.com/RXWatcher/silo-plugin-notifications)), but the support plugin itself has no hard dependency on any other plugin.

Auth flows through `X-Silo-User-Id` and `X-Silo-User-Role` headers injected by the host.

Host: [`Silo-Server/silo-server`](https://github.com/Silo-Server/silo-server). SDK: [`Silo-Server/silo-plugin-sdk`](https://github.com/Silo-Server/silo-plugin-sdk).

## External services

Requires a **Postgres** database. The plugin owns a dedicated `support` schema (table prefixes: `app_config`, `kb_*`, `st_*`, `tk_*`) and runs its own embedded `golang-migrate` migrations on startup via pgx/v5. The operator only needs to create the schema and grant connect rights.

Optional outbound HTTP is used by the Speedtest module's GeoIP layer to download MMDB databases (`mmdb_auto` sources, e.g. db-ip.com country-lite, seeded by default) into the configured cache directory.

## Status

Alpha. All four modules in the program are now shipped at v0.4.0:

| Module | Status | Notes |
| --- | --- | --- |
| Shell (v0.1) | Shipped | Bootstrap, customer + admin SPA shells, `app_config` singleton, auth gates |
| Knowledge Base (v0.2) | Shipped | Tiptap WYSIWYG, Postgres FTS + related, slug URLs, inline images, scheduled publishing, per-customer thumbs |
| Speedtest (v0.3) | Shipped | LibreSpeed-protocol customer tests, admin endpoint catalogue, switchable auto-strategy (latency vs operator-ordered GeoIP chain), 30-day result history, slow-event hooks |
| Tickets (v0.4) | Shipped | Typed-intake categories with per-category form fields, SUP-N tracking numbers, status lifecycle with reopen, append-only entries with internal notes, 10 MB attachments, configurable auto-close cron |
| AI Assistance | Planned | KB suggestion on ticket create, embedding similarity over articles |

The 0.x line reflects that the program is still early: each module shipped with documented post-ship follow-ups (see `CHANGELOG.md` and `docs/follow-ups.md`), the plugin emits events but cannot deliver them, and there is no native `scheduled_task.v1` SDK capability yet — crons are admin-trigger endpoints. API surfaces and the on-disk schema may still change before a 1.0.

## Configuration

The plugin requires a single global config key, `database_url`, declared in the manifest as a required password-input admin form field:

```text
postgres://plugin_support:...@host:5432/silo?search_path=support&sslmode=disable
```

All further configuration lives in the singleton `app_config` row, managed via the shell's admin UI and the `GET/PATCH /api/admin/config` endpoints:

| Key | Default | Module | Purpose |
| --- | --- | --- | --- |
| `database_url` | (required) | core | Postgres DSN |
| `auto_strategy` | `latency` | speedtest | `latency` or `geoip` — how "Auto" picks the endpoint |
| `client_ip_storage` | `truncated` | speedtest | `truncated` (/24 v4, /48 v6) or `off` |
| `slow_threshold_mbps` | `5` | speedtest | Download below this fires `speedtest_slow` |
| `geoip_cache_dir` | XDG cache dir | speedtest | Where `mmdb_auto` sources are downloaded |
| `tickets_auto_close_enabled` | `true` | tickets | Master toggle for the auto-close cron |
| `tickets_resolved_close_after_days` | `7` | tickets | Days idle after `resolved` before auto-close (0 = skip) |
| `tickets_waiting_close_after_days` | `14` | tickets | Days idle in `waiting_customer` before auto-close (0 = skip) |

Module toggles (KB, Speedtest, Tickets, AI) live in the same `app_config` row. Shipped modules default ON; AI defaults OFF until it ships.

## Detailed docs

- `CHANGELOG.md` — release history including post-ship follow-ups per module
- `docs/follow-ups.md` — v1.1 follow-ups across all four modules
- `docs/superpowers/specs/` — per-module design specs
- `docs/superpowers/plans/` — per-module implementation plans

## Build and release

```bash
make build   # builds the SPA (pnpm) then the Go binary
make test    # go test ./... + pnpm test
```

CI builds linux-amd64 binaries on push to main via the reusable workflow in [RXWatcher/silo-plugin-repository](https://github.com/RXWatcher/silo-plugin-repository) and publishes them to the catalog at [`./binaries/`](https://github.com/RXWatcher/silo-plugin-repository/tree/main/binaries).
