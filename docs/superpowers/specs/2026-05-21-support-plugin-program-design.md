# Support Plugin — Program Design

## Status (as of 2026-05-21 ship)

All four originally-planned modules in the v1 ship order are now
shipped and live on `main`:

| Module | Manifest version | Released |
|---|---|---|
| Shell | 0.1.0 | First commit; foundation for the rest |
| Knowledge Base | 0.2.0 | After Speedtest in pre-execution order; shipped first per dependency on the shell |
| Speedtest | 0.3.0 | Shipped after KB |
| Tickets | 0.4.0 | Shipped after Speedtest |

**AI Assistance** (program ship-order #5) was always positioned as a
separate cycle. It has no spec or plan yet — fresh brainstorm when
the operator is ready.

**Deviations from this program-level design during execution:**

- Each module's auto-trigger cron was deferred to an admin-button
  endpoint instead of a native `scheduled_task.v1` SDK capability.
  The capability does not yet exist in the SDK; admin-trigger is the
  v1 fallback for all three crons (KB unhelpful sweep, tickets
  auto-close, speedtest mmdb refresh). See `docs/follow-ups.md`.
- Speedtest GeoIP grew from a hardcoded chain into an admin-
  orderable list of sources with four kinds (`mmdb_auto`,
  `mmdb_file`, `http_api`, `request_header`) at the user's request
  during the brainstorm.
- Tickets v1 scope is the minimum decided during the implementation
  spec; the bigger feature list discussed early (linked tickets,
  tags, custom fields, email-to-ticket, live chat, CSAT surveys) is
  documented as explicitly out of scope.

This status block is the only retrospective in the file; the
sections below reflect the original program-level decomposition as
written at planning time.

---

**Status:** Program-level spec. Per-module designs (Knowledge Base, Speedtest, Tickets, AI Assistance) follow as separate documents under this same `specs/` directory and are each their own brainstorm → spec → plan → implementation cycle.

**Date:** 2026-05-21
**Plugin id:** `silo.support`
**Repo:** `silo-plugin-support` (sibling to the other first-party plugins)

## Purpose

A single Silo plugin that bundles the customer-facing support tools — connection diagnostics (speedtest), self-serve answers (knowledge base), and a typed ticket intake — behind one cohesive "Support" surface in the customer portal, with an operator dashboard for staff. AI assistance is a fifth module that consumes the other three's data and only ships after they have real content.

A customer in trouble should never have to figure out which Silo plugin to open. They click "Support" once and the right tool is in front of them.

## Decomposition

Four functional modules + one optional fifth:

| # | Module | Independence | Notes |
|---|---|---|---|
| 1 | **Knowledge Base** | Standalone | Operator-written articles, customer browse + search, categorisation. Highest leverage: deflects tickets that never need to be opened. Customer-facing read-only, admin-writable. |
| 2 | **Speedtest** | Standalone | Connection diagnostic. Multi-endpoint LibreSpeed JS client, admin-configurable endpoint list, per-customer history, admin aggregate dashboard. Design already drafted; ports cleanly into a module of this plugin. |
| 3 | **Tickets** | Standalone but informed by KB | Typed intake (bad media, billing, client config — extensible), agent queue, status lifecycle, customer-side status view. The big one — full product on its own. |
| 4 | **AI Assistance** | Depends on KB + Tickets data | Suggest KB articles at ticket creation, auto-categorise tickets, possibly auto-resolve trivial cases. Speculative until the other three have shipped and accumulated content. |

These four are independent in their data models, their UIs, and most of their backend handlers. They share only the plugin shell — auth, navigation, theming, Postgres schema container, embedded SPA shell, host SDK access.

## Ship Order

1. **Shell** (foundation) — plugin scaffolding, manifest, auth gates, customer + admin SPA shells, navigation, theme inheritance, Postgres schema + migrations baseline, host SDK wiring. No user-visible feature beyond an empty "Support" landing. Single milestone, ~3 days.
2. **Knowledge Base** — articles, categories, browse, search, admin editor. Reduces volume before Tickets ships.
3. **Speedtest** — port the existing design as a module within the plugin. Cross-link: speedtest result page surfaces a "Run into a problem? Check the KB or open a ticket" CTA once Tickets exists.
4. **Tickets** — typed intake forms, agent queue, lifecycle (open / pending / waiting-customer / resolved), threaded comments, attachments. Cross-link: ticket form auto-suggests KB articles by title-match; ticket detail page shows the customer's most recent speedtest if relevant.
5. **AI Assistance** — first feature: KB-article suggestions on ticket creation. Iterates from there. Behind an admin opt-in.

Each module after the shell is its own design + plan + implementation pass. The shell spec is the immediate next deliverable after this program spec.

## Cross-Module Patterns

### Plugin shape

One Go binary, one React/TS SPA, one Postgres schema (`support`). Mirrors the architecture of `silo-plugin-public-catalog`:

- Manifest declares all routes for all modules in one place.
- The SPA dispatches by a server-baked bootstrap mode (`mode: 'support-home' | 'kb-browse' | 'kb-article' | 'speedtest' | 'tickets-list' | 'tickets-detail' | 'admin' | ...`) — no client-side router.
- Single migration runner; each module owns its own table set with a name prefix (`kb_*`, `st_*` for speedtest, `tk_*` for tickets, `ai_*`).
- `app_config` is a singleton JSONB row, per the public-catalog pattern; module-specific settings nest under top-level keys.

This means the binary grows with each module, but the deployment story stays "one plugin to install, one config to manage, one nav item for customers."

### Auth model

Match the public-catalog pattern exactly:

- The Silo host stamps `X-Silo-User-Id` and `X-Silo-User-Role` on inbound requests it has authenticated.
- Manifest `http_routes` declare `access: "user"` for customer-facing routes and `access: "admin"` for operator routes.
- The plugin's `requireUser` and `requireAdmin` middleware reads the header. The standalone-listener path (if we expose one) strips `X-Silo-*` headers so the gates can't be spoofed — same defence as public-catalog.

No anonymous surface in v1; every route requires either a logged-in customer or an admin.

### Navigation

Manifest declares one navigable entry per surface:

| Entry | Kind | Label | Path |
|---|---|---|---|
| Support home | `user` | "Support" | `/` |
| Support admin | `admin` | "Support" | `/admin` |

Inside the SPA, a left-rail (or top-tab) component renders the module nav: Home · Knowledge Base · Speedtest · Tickets. Modules appear only when their feature flag is on; an admin can disable any module without uninstalling the plugin.

### Theming

Inherit `X-Silo-Theme` from the request, default to `midnight-cinema` (matches public-catalog). Tailwind v4 + shadcn primitives; same dependency set as the other first-party plugins.

### Schema isolation

Per-module table prefix is the convention. Cross-module foreign keys are allowed but should be sparse — the cleaner the boundary, the easier each module is to evolve. Concrete cross-links the program design plans for:

- `tk_tickets.related_speedtest_result_id` (nullable) → `st_results.id` — when a customer opens a ticket from the speedtest result page.
- `tk_tickets.suggested_kb_article_id` (nullable) → `kb_articles.id` — set by the AI module when a suggestion was offered and the ticket was opened anyway.

### Integration points (cross-module wiring, summarised)

- **Speedtest → Tickets:** result page CTA "Was this slow? Open a ticket about your connection." Pre-fills ticket type + attaches the result id.
- **Tickets → KB:** at ticket creation, suggest top-3 KB articles by title-match. AI module replaces this with embedding-based suggestion in v2.
- **Tickets → Speedtest:** ticket detail (agent side) shows the customer's last speedtest if recent, for context.
- **KB → Speedtest:** a KB article about "slow streaming" can deep-link `/speedtest` so the customer self-diagnoses.

These are implementation-affordances, not features that gate any module's ship. The MVP of each module can ship without the cross-link; the link is added once both sides exist.

### Host SDK use

- Customer identity for `requireUser` middleware (`X-Silo-User-Id`).
- Customer's assigned streaming server for the speedtest "Your server" dropdown entry (requires SDK accessor — verify pre-deploy).
- Federation to other plugins for cross-data lookups (e.g., billing-related ticket → query WHMCS plugin for invoice state). Cross-plugin calls go through the SDK's `CallPluginJSON` the way public-catalog talks to ebook/audiobook plugins.

## Module Summaries

Each gets its own design + plan + implementation pass when its turn arrives. These paragraphs are intent only.

### 1. Shell (foundation)

The minimum that exists before any module: manifest with the two nav entries above, customer SPA renders a "Support" home with empty module cards (each marked "Coming soon" until enabled), admin SPA renders a config page with module on/off toggles. Migrations runner, Postgres schema container, theme inheritance, auth middleware, host SDK wiring. No user-visible feature beyond the shell itself — this is the platform on which the four modules land.

### 2. Knowledge Base

Operator-authored articles in Markdown (sanitised on render), categorised into a small admin-defined taxonomy, browseable and full-text searchable by customers. Admin CRUD with draft/published states, scheduled publishing optional. Per-article view-count + thumbs-up/down so operators can see which articles work. Articles can link to other articles and to module deep-links (`/speedtest`, `/tickets/new?type=billing`).

### 3. Speedtest

Per the prior brainstorm: multi-endpoint LibreSpeed JS client, admin-configurable endpoint list, "Your server" (resolved from host SDK) + admin CDNs in a dropdown, ping-on-load Auto-select, per-customer history sidebar, admin aggregate dashboard (tests-per-day, median per endpoint, P95 ping per endpoint). Replaces the hardcoded `SPEEDTEST_SERVERS` array currently in `speedtest.wave-ninja.eu`'s `index.html`.

### 4. Tickets

Typed intake (initial categories: **bad media**, **billing**, **client config**; admin can add more). Each category has a typed form (bad media → media link + description; billing → invoice id + description; config → free-text). Tickets land in an agent queue with status lifecycle (open / in-progress / waiting-customer / resolved / closed), assignable to staff, threaded comments visible to both sides, attachment support, status-change emails to the customer (via host SDK email facility — verify pre-deploy). Customer side: list of own tickets + detail view + reply form.

### 5. AI Assistance

First feature: at ticket creation, embed-search the KB for the typed description and suggest the top-3 articles inline ("Before you submit — these might help"). If the customer reads one and abandons the ticket, log a deflection. Behind an admin opt-in (because it touches an external API). Later iterations: auto-categorisation of submitted tickets, auto-reply for known-easy categories (`bad media` with a working stream URL → auto-close). Specifically out of v1 of this module: AI conversation, AI authoring of KB articles, AI ticket prioritisation.

## Out of Scope (v1 of the program)

- **Multi-tenancy / per-customer-org isolation** — one install, one customer base.
- **Public/anonymous surfaces** — every route requires login or admin role.
- **Real-time/chat** — tickets are async messaging only.
- **SLA tracking / agent performance dashboards** — explicitly deferred to a later operator analytics pass.
- **Inbound email-to-ticket** — tickets are opened from the customer UI, not by email. Outbound status emails only.
- **Per-customer ticket assignment ("your agent is X")** — round-robin / unassigned queue is fine for v1.
- **AI conversation / chatbot** — only suggestion + categorisation in v1 of the AI module.
- **Federation across multiple Silo installs** — single-install scope.

## Open Questions (deferred to per-module brainstorms)

These don't need answers now but will gate the per-module designs:

- **KB:** Markdown editor (CodeMirror? Plain textarea + preview?), search (Postgres full-text? a small embedded index?), category taxonomy depth (flat? two-level?).
- **Tickets:** attachment storage (Postgres `bytea`? object storage? host SDK has one?), email sender (host SDK email API exists?), customer-visible ticket numbers (sequential? UUID? slug?).
- **AI:** which LLM provider (operator-configured API key?), embedding model, cost ceiling per install, privacy (does customer-typed ticket text leave the install?).
- **Cross-cutting:** does the host SDK expose the customer's assigned streaming server, and is that consistent across plugins?

## Spec Lineage

This program spec is the parent. Each module's design spec sits alongside it under `docs/superpowers/specs/`, named `YYYY-MM-DD-support-<module>-design.md`. Each module's design spec gets its own implementation plan under `docs/superpowers/plans/`.

Next deliverable: **shell design spec.**
