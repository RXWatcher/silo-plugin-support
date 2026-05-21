# Continuum Support Plugin

`continuum.support` is the customer-facing support surface for a
Continuum deployment.

**Shipped modules:**

| Module | Status |
|---|---|
| Knowledge Base | Shipped (v0.2) |
| Speedtest | Shipped (v0.3) |
| Tickets | Shipped (v0.4) |
| AI Assistance | Coming soon |

See `docs/superpowers/specs/` for designs and `docs/superpowers/plans/`
for implementation plans.

## Build

```sh
make build
make test
```

`make test` runs Go tests + the vitest SPA suite. Some integration
tests are gated on `PG_DSN` (a Postgres DSN); without it those tests
skip cleanly.

## Configuration

Requires `database_url` — a Postgres DSN, e.g.
`postgres://plugin_support:...@host:5432/continuum?search_path=support&sslmode=disable`.

Speedtest-related config keys (all optional, sane defaults):

- `auto_strategy` — `latency` (default) or `geoip`
- `client_ip_storage` — `truncated` (default) or `off`
- `slow_threshold_mbps` — default `5`
- `geoip_cache_dir` — default `$XDG_CACHE_HOME/continuum-plugin-support/geoip/`

Tickets-related config keys:

- `tickets_auto_close_enabled` — default `true`. Set to `false` to disable the auto-close cron entirely.
- `tickets_resolved_close_after_days` — default `7`. Setting to `0` skips the resolved-pass while keeping the waiting-pass running.
- `tickets_waiting_close_after_days` — default `14`. Setting to `0` skips the waiting-pass.

## Events emitted

Routed via the existing `continuum.notifications` plugin per admin rules.

**KB:** `kb_article_published / _updated / _unhelpful`
**Speedtest:** `speedtest_run / _slow`
**Tickets:** `ticket_submitted / _replied / _status_changed / _assigned / _resolved / _reopened / _closed`

## Crons (admin-trigger endpoints)

- KB: `POST /api/admin/kb/cron/run` (publish-due + unhelpful sweep)
- Tickets: `POST /api/admin/tickets/cron/run` (auto-close idle)
- GeoIP mmdb refresh: automatic on plugin start; manual via `POST /api/admin/speedtest/geoip/{id}/refresh`

Native `scheduled_task.v1` SDK wiring is a follow-up for all three.
