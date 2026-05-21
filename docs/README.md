# Support plugin docs

Operator-facing companion to the top-level
[`README.md`](../README.md). The README covers what the plugin is,
its capabilities, its dependencies, and its config keys. These docs
go deeper on how it actually behaves at runtime — useful when
something is wrong, when onboarding a new module to an existing
install, or when scoping a v1.1 follow-up.

## Index

- [`architecture.md`](./architecture.md) — shared shell plus four
  modules, table-prefix layout, auth header flow, where each
  responsibility lives in `internal/`.
- [`operations.md`](./operations.md) — first install, schema setup,
  upgrade procedure, the runtime `Configure` lifecycle, cron
  trigger endpoints, module enable / disable.
- [`debugging.md`](./debugging.md) — known sharp edges and how to
  diagnose them. Indexed by symptom, not by module.
- Per-module quickstarts:
  - [`modules/kb.md`](./modules/kb.md) — Knowledge Base (admin
    authoring, customer browsing, FTS, scheduled publishing,
    inline images).
  - [`modules/speedtest.md`](./modules/speedtest.md) — Speedtest
    (endpoints, GeoIP source chain, auto strategy, MMDB lifecycle,
    slow-event hook).
  - [`modules/tickets.md`](./modules/tickets.md) — Tickets
    (category taxonomy, per-category fields, lifecycle, reopen
    window, attachments, auto-close cron).
- [`follow-ups.md`](./follow-ups.md) — known v1.1 items per
  module. Nothing here blocks production use.
- [`superpowers/specs/`](./superpowers/specs/) — design specs that
  shipped each module. Historical; read these when you need to
  know *why* something is the way it is.
- [`superpowers/plans/`](./superpowers/plans/) — implementation
  plans those specs produced. Also historical.

The repository-root [`CHANGELOG.md`](../CHANGELOG.md) is the
canonical per-release record.

## Audiences

Most of this doc tree is for **operators** — the person who
installs the plugin, owns the Postgres schema, watches the host
event bus, and answers customer escalations.

Where a section is relevant to **end customers** (e.g. how the
reopen window works in tickets, what the speedtest auto button
does), it is called out explicitly. Customer-facing screens
themselves live under `web/src/` in the repo; these docs describe
their backend behaviour, not their copy.
