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

Scheduled publishing + unhelpful-article detection run via the admin
button `POST /api/admin/kb/cron/run`. A native `scheduled_task.v1`
SDK capability is a follow-up.
