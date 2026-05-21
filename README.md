# Continuum Support Plugin

`continuum.support` is the customer-facing support surface for a
Continuum deployment.

**Shipped modules:**

| Module | Status |
|---|---|
| Knowledge Base | Shipped (v0.2) |
| Speedtest | Shipped (v0.3) |
| Tickets | Coming soon |
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

The plugin manages its own schema; the operator only needs to create
the schema and grant connect rights.

Speedtest-related config keys (all optional, sane defaults):

- `auto_strategy` — `latency` (default) or `geoip`
- `client_ip_storage` — `truncated` (default) or `off`
- `slow_threshold_mbps` — default `5`
- `geoip_cache_dir` — default `/var/cache/continuum-plugin-support/geoip/`

## Events emitted

Routed via the existing `continuum.notifications` plugin per admin rules.

**KB:**
- `plugin.continuum.support.kb_article_published`
- `plugin.continuum.support.kb_article_updated`
- `plugin.continuum.support.kb_article_unhelpful`

**Speedtest:**
- `plugin.continuum.support.speedtest_run`
- `plugin.continuum.support.speedtest_slow`

## Crons

- **KB cron** (publish-due + unhelpful sweep): `POST /api/admin/kb/cron/run`
- **GeoIP mmdb refresh** is automatic on plugin start (background);
  manual trigger: `POST /api/admin/speedtest/geoip/{id}/refresh`
