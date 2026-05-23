# Operations

Day-to-day operator concerns: install, configure, upgrade, run the
crons, enable / disable modules, audit data.

## Install

1. **Create the Postgres schema and role.**

   ```sql
   CREATE ROLE plugin_support LOGIN PASSWORD '...';
   CREATE SCHEMA support AUTHORIZATION plugin_support;
   ```

   The plugin runs its own migrations from a fresh schema; it does
   not need extra grants beyond `CONNECT` on the database and
   ownership (or full DDL grant) on `support`.

2. **Install the plugin via the host catalog.** The manifest
   declares one required admin-config field, `database_url`, as a
   password input. Supply a DSN with the schema in
   `search_path`:

   ```
   postgres://plugin_support:...@host:5432/silo?search_path=support&sslmode=disable
   ```

   On `Configure` the plugin opens a pgx pool, runs migrations
   (`internal/migrate/files/0001_init.up.sql` through
   `0004_tickets_init.up.sql`), reads the singleton `app_config`
   row, and mounts routes. First-run defaults flip KB / Speedtest
   / Tickets on; AI is left off until it ships.

3. **Visit `/admin`.** The operator landing page (chi route
   `GET /admin`) requires `X-Silo-User-Role: admin`. The
   sidebar reflects `SHIPPED_MODULES ∩ enabled`. Configure each
   module from its admin section.

## The `app_config` singleton

Every non-DSN setting lives in a single row of the `app_config`
table (`id = 1`, JSONB column `data`). Read and write it via the
shell:

- `GET /api/admin/config` → current `Config` JSON.
- `PATCH /api/admin/config` → partial merge, re-normalized, then
  the result is what the plugin uses immediately. The pgx pool is
  not re-opened (DSN comes from the manifest, not from
  `app_config`).

Validation lives in
`runtime.NormalizeAppConfig`. Bad values return a 400 with a
descriptive message; the row is not written.

| Key | Default | Module | Behaviour |
| --- | --- | --- | --- |
| `modules.kb`, `modules.speedtest`, `modules.tickets`, `modules.ai` | true / true / true / false | shell | Hides the module's sidebar entry. Does not drop tables, does not hide rows. |
| `auto_strategy` | `latency` | speedtest | `latency` (client picks fastest of all active endpoints) or `geoip` (server resolves IP → country → matching endpoint). |
| `client_ip_storage` | `truncated` | speedtest | `truncated` writes /24 v4 or /48 v6 into `st_results.client_ip`. `off` stores NULL. |
| `slow_threshold_mbps` | `5` | speedtest | Download below this fires `speedtest_slow`. `0` disables the event. |
| `geoip_cache_dir` | XDG cache dir | speedtest | Where `mmdb_auto` sources are downloaded. See note below. |
| `tickets_auto_close_enabled` | `true` | tickets | Master toggle for the cron. False = endpoint is a no-op. |
| `tickets_resolved_close_after_days` | `7` | tickets | Days idle in `resolved` before auto-close. `0` skips the pass. |
| `tickets_waiting_close_after_days` | `14` | tickets | Days idle in `waiting_customer` before auto-close. `0` skips the pass. |

The plugin walks **XDG_CACHE_HOME → `~/.cache` → relative
fallback** to pick the default `geoip_cache_dir`. It used to
hardcode `/var/cache`, which is unwritable for non-root daemons
on most distros — this was fixed in 0.3.0. If you set this
explicitly, make sure the plugin's runtime user can write to it
and has at least a few MB free.

## Cron triggers

Three admin endpoints stand in for proper scheduled tasks. Hit
them on whatever cadence makes sense. Idempotent; safe to retry.

| Endpoint | What it does | Suggested cadence |
| --- | --- | --- |
| `POST /api/admin/kb/cron/run` | Flips drafts whose `publish_at` has elapsed to `published` (emits `kb_article_published`); sweeps last-24h votes and emits `kb_article_unhelpful` for any published article below the helpful-ratio threshold (default 0.5, min 5 votes). | every 5–15 min |
| `POST /api/admin/speedtest/geoip/{id}/refresh` | Forces an immediate `mmdb_auto` download cycle for that source. `mmdb_auto` sources also self-refresh once they age past their `refresh_days` (db-ip.com default: 25). | per source, manual or daily |
| `POST /api/admin/tickets/cron/run` | Auto-closes tickets idle in `resolved` for `tickets_resolved_close_after_days` and idle in `waiting_customer` for `tickets_waiting_close_after_days`. Each pass is independently skippable by setting the threshold to 0. Each close inserts a `system` entry and emits `ticket_closed{by:system, reason:resolved_idle|waiting_idle}`. | daily, off-peak |

All three are admin-only (`requireAdmin`). They are deliberately
not background tickers in the plugin process — the SDK does not
yet ship `scheduled_task.v1`. See
[`follow-ups.md`](./follow-ups.md#cross-cutting).

## Upgrade

The plugin is one Go binary plus an embedded SPA. The CI workflow
at
[RXWatcher/silo-plugin-repository](https://github.com/RXWatcher/silo-plugin-repository)
builds linux-amd64 on every push to main. The host downloads the
new artefact and re-runs `Configure`.

On restart the plugin:

1. Re-opens the pgx pool against the same DSN.
2. Re-runs `golang-migrate up`. Newer release adds a new numbered
   `.up.sql`; the runner applies it in order. Down-migrations
   exist but **the plugin never calls them**.
3. Re-normalises the `app_config` row against the new code's
   defaults. Unknown keys are preserved; missing keys take the
   in-code default.
4. Re-mounts the chi router. The `httproutes.Server` swap is
   atomic (`atomic.Pointer[http.Handler]`); in-flight requests
   land on the old handler, new requests on the new one.

Downgrading after a migration has run is not supported — the new
column / table will be unknown to the previous binary. Roll
forward.

## Module enable / disable

Toggle from the admin shell UI or via `PATCH /api/admin/config`
with e.g. `{"modules":{"tickets":false}}`. Effects:

- Sidebar entry disappears for both customer and admin.
- HTTP routes for that module **stay registered** (chi is mounted
  at boot, not on every config change). Requests to them keep
  working — there is no module-level kill switch on the API. This
  is a deliberate design choice: a transient toggle should not
  break links in a notification email that has already left the
  building.
- Existing rows are untouched. Re-enable to surface them again.

The Shell module has no toggle; it is the rest of the plugin.

## Database housekeeping

The schema is owned end-to-end by the plugin. Don't run manual
DDL against it. Operator-friendly things you *can* do safely from
a SQL prompt:

- Audit `app_config.data` to confirm what the plugin will compute
  on next normalize. The row is small and easy to read.
- Query `st_results`, `kb_views`, `tk_tickets` for analytics. All
  three have indexes on `(*_id, *_at DESC)` so paged time-window
  scans are cheap.
- Manually delete a misuploaded KB image by `kb_images.id` — the
  FK from `kb_images.article_id` is `ON DELETE SET NULL`, so
  deleting an article does not auto-purge its images.
- Manually expire old `st_results` if the table grows beyond what
  the dashboard needs. The 30-day dashboards aggregate at query
  time, so old rows are read-cost only.

## Backups

Standard Postgres backup of the `support` schema covers the whole
plugin — there is no on-disk state outside it except the
`geoip_cache_dir` MMDB cache (which is rebuildable on demand from
`mmdb_auto`). Plugin restarts cleanly with an empty cache; the
first GeoIP-strategy speedtest after restart triggers a download.

## Observability

The plugin writes to its host-supplied log sink (`slog` via the
SDK). Notable signal:

- `Configure` logs once per restart with the resulting `Config`
  (DSN redacted; `database_url` has a `json:"-"` tag).
- Migration runs log per applied step.
- Each cron pass logs counts (drafts published, unhelpful events,
  tickets closed).
- HTTP errors go through `writeErr(w, code, slug, msg)` which
  produces a JSON envelope; check the slug for a programmatic
  signal (e.g. `kb_not_found`, `forbidden`, `not_ready`).
