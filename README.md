# Continuum Support Plugin

`continuum.support` is the customer-facing support surface for a
Continuum deployment. The shell ships first; modules (Knowledge Base,
Speedtest, Tickets, AI Assistance) follow in their own releases.

See `docs/superpowers/specs/` for designs and `docs/superpowers/plans/`
for implementation plans.

## Build

```sh
make build
make test
```

## Configuration

Requires `database_url` — a Postgres DSN, e.g.
`postgres://plugin_support:...@host:5432/continuum?search_path=support&sslmode=disable`.

The plugin manages its own schema; the operator only needs to create
the schema and grant connect rights.
