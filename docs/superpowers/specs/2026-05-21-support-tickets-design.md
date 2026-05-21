# Support Plugin — Tickets Module v1 Design

**Status:** Sub-project design under the program spec
([`2026-05-21-support-plugin-program-design.md`](2026-05-21-support-plugin-program-design.md)).
Ships as the 4th module of the support plugin per the program ship
order (shell → KB → Speedtest → Tickets → AI).

**Date:** 2026-05-21
**Sub-project:** Tickets module v1
**Successor:** AI Assistance module (final brainstorm)

## Purpose

A typed-intake customer support ticket system inside the support
plugin. Customers open tickets via a category-specific form; admins
handle them through an agent queue with a status lifecycle and
threaded comments. **Built from scratch** rather than adopted, with
schema + URL shapes kept loosely compatible with go-help-desk for a
future swap if the operator outgrows v1.

## Decisions Locked During Brainstorm

- **Two-role model** (user + admin). No separate "agent" role. Any
  admin can answer tickets.
- **Polling, not SSE.** The SDK `HttpRoutes.Handle` RPC is
  request/response (no streaming); SSE would require a standalone
  listener which the shell spec deliberately omits. Agent queue and
  customer ticket list refetch every 30s while open.
- **Postgres `bytea` attachments**, 10 MB cap per file. Shell adds a
  12 MB request body limit middleware.
- **Email / push / chat notifications via the `continuum.notifications`
  plugin.** Tickets module emits well-formed events; operator wires
  delivery rules in the notifications admin. **Zero SMTP config in
  this module.**
- **Build it ourselves with loose swap-ability to go-help-desk later**
  — keep schema + URL shapes compatible-ish; no live abstraction
  layer.

## Schema (extends `support` Postgres schema)

```sql
-- Category taxonomy ----------------------------------------------------
CREATE TABLE tk_categories (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE tk_subcategories (
    id          BIGSERIAL PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES tk_categories(id) ON DELETE RESTRICT,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (category_id, slug)
);

-- Per-category form fields. Subcategories share the parent category's
-- fields in v1; per-subcategory fields are explicitly out of scope.
CREATE TABLE tk_category_fields (
    id          BIGSERIAL PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES tk_categories(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    label       TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('text','textarea','number','url')),
    required    BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order  INT NOT NULL DEFAULT 0,
    UNIQUE (category_id, key)
);

-- Tickets --------------------------------------------------------------
CREATE TABLE tk_tickets (
    id                  BIGSERIAL PRIMARY KEY,
    tracking_number     TEXT NOT NULL UNIQUE,           -- 'SUP-247'
    customer_id         TEXT NOT NULL,                  -- X-Continuum-User-Id
    customer_email      TEXT NOT NULL,                  -- snapshot at create
    category_id         BIGINT NOT NULL REFERENCES tk_categories(id) ON DELETE RESTRICT,
    subcategory_id      BIGINT REFERENCES tk_subcategories(id) ON DELETE RESTRICT,
    subject             TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open','in_progress','waiting_customer','resolved','closed')),
    assigned_admin_id   TEXT,                           -- nullable
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    waiting_since       TIMESTAMPTZ,                    -- set when status='waiting_customer'
    resolved_at         TIMESTAMPTZ                     -- set when status='resolved'
);
CREATE INDEX tk_tickets_customer_idx ON tk_tickets(customer_id, updated_at DESC);
CREATE INDEX tk_tickets_queue_idx    ON tk_tickets(status, updated_at DESC);

-- Append-only entries on each ticket. 'system' is module-generated
-- (e.g. status transitions); 'internal_note' is admin-only and never
-- returned by customer-side handlers.
CREATE TABLE tk_ticket_entries (
    id          BIGSERIAL PRIMARY KEY,
    ticket_id   BIGINT NOT NULL REFERENCES tk_tickets(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL
        CHECK (kind IN ('initial','reply','internal_note','status_change','system')),
    author_id   TEXT NOT NULL,                          -- X-Continuum-User-Id or 'system'
    author_role TEXT NOT NULL CHECK (author_role IN ('customer','admin','system')),
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX tk_ticket_entries_ticket_idx ON tk_ticket_entries(ticket_id, created_at);

-- Field values captured on ticket creation. Frozen — not edited.
CREATE TABLE tk_ticket_field_values (
    ticket_id   BIGINT NOT NULL REFERENCES tk_tickets(id) ON DELETE CASCADE,
    field_id    BIGINT NOT NULL REFERENCES tk_category_fields(id) ON DELETE RESTRICT,
    value       TEXT NOT NULL,
    PRIMARY KEY (ticket_id, field_id)
);

-- Attachments live alongside the entry that introduced them. bytea
-- storage; 10 MB cap enforced at the handler.
CREATE TABLE tk_attachments (
    id              BIGSERIAL PRIMARY KEY,
    entry_id        BIGINT NOT NULL REFERENCES tk_ticket_entries(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    mime            TEXT NOT NULL,
    bytes           BIGINT NOT NULL,
    content_bytea   BYTEA NOT NULL,
    sha256          BYTEA NOT NULL,                     -- 32 bytes
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Sequence for SUP-N tracking numbers. Atomic UPDATE ... RETURNING.
CREATE TABLE tk_ticket_sequence (
    id      SMALLINT PRIMARY KEY CHECK (id = 1),
    next_n  BIGINT NOT NULL DEFAULT 1
);
INSERT INTO tk_ticket_sequence (id, next_n) VALUES (1, 1) ON CONFLICT (id) DO NOTHING;
```

Tracking number format: `SUP-` + `next_n` (no padding). Zero-padding
adds visual noise without meaningful sort benefit; the column is
unique-indexed for prefix search.

## Lifecycle

```
                 ┌────────────────┐
                 │      open      │  ← customer creates ticket
                 └───────┬────────┘
                         ▼
   admin replies    ┌────────────────┐    customer replies
   ───────────────► │  in_progress   │ ◄────────────────────────┐
                    └──┬──────────┬──┘                          │
        admin sets     │          │  admin marks resolved        │
        waiting        ▼          ▼                              │
              ┌──────────────┐  ┌──────────────┐                 │
              │waiting_      │  │   resolved   │─────────────────┘
              │customer      │  │              │  customer reopens
              │              │  │              │  (within 7d)
              └───────┬──────┘  └───────┬──────┘
                      │ 14d idle        │ 7d idle
                      ▼                 ▼
                          ┌──────────────┐
                          │    closed    │  ← terminal
                          └──────────────┘
```

**Transitions (canonical, others rejected with 409):**

| From → To | Trigger | Side-effects |
|---|---|---|
| `open` → `in_progress` | admin reply | `updated_at` |
| `in_progress` → `waiting_customer` | admin status change | sets `waiting_since` |
| `waiting_customer` → `in_progress` | customer reply | clears `waiting_since` |
| `in_progress` → `resolved` | admin marks resolved | sets `resolved_at` |
| `waiting_customer` → `resolved` | admin marks resolved | sets `resolved_at` |
| `resolved` → `in_progress` | customer reopens (≤ 7d after `resolved_at`) | clears `resolved_at` |
| `resolved` → `closed` | daily cron, idle 7d | terminal |
| `waiting_customer` → `closed` | daily cron, idle 14d | terminal |
| any → `closed` | admin explicit close | terminal |

Daily cron runs in the shell's existing scheduled-task slot once a day
(or, if no scheduled-task capability is declared yet, an explicit
admin button "Close idle tickets" is the v1 fallback — verify SDK
support pre-implement). Timings (`7d`, `14d`) are stored in the
plugin config as `tickets.resolved_close_after_days` and
`tickets.waiting_close_after_days`; defaults above.

## Routes (added to the support manifest at this module's release)

| Method+Path | Access | Purpose |
|---|---|---|
| `GET /tickets` | user | Customer SPA shell, `mode: tickets-list` |
| `GET /tickets/{tracking_number}` | user | Customer SPA shell, `mode: tickets-detail` |
| `POST /api/customer/tickets` | user | Create ticket (returns tracking number) |
| `GET  /api/customer/tickets` | user | Own ticket list, paginated, status filter |
| `GET  /api/customer/tickets/{tracking_number}` | user | Detail (excludes internal_notes) |
| `POST /api/customer/tickets/{tracking_number}/reply` | user | Add reply entry (+ multipart attachments) |
| `POST /api/customer/tickets/{tracking_number}/reopen` | user | If `resolved` and `resolved_at` < 7d ago |
| `GET  /api/customer/categories` | user | Active categories + subcategories + per-category fields for form rendering |
| `GET  /admin/tickets` | admin | Admin SPA shell, `mode: admin-tickets-queue` |
| `GET  /admin/tickets/{tracking_number}` | admin | Admin SPA shell, `mode: admin-tickets-detail` |
| `GET  /api/admin/tickets` | admin | Queue (filters: status, category, assignee, search by tracking# prefix or subject substring) |
| `GET  /api/admin/tickets/{tracking_number}` | admin | Detail (includes internal_notes + field values) |
| `POST /api/admin/tickets/{tracking_number}/reply` | admin | Visible reply |
| `POST /api/admin/tickets/{tracking_number}/note` | admin | Internal note |
| `POST /api/admin/tickets/{tracking_number}/status` | admin | Explicit status change |
| `POST /api/admin/tickets/{tracking_number}/assign` | admin | Assign / unassign (body: `{admin_id: string \| null}`) |
| `GET  /api/attachments/{id}` | user (owner) or admin | Stream attachment bytes (Content-Disposition: inline) |
| `GET /POST /PUT /DELETE /api/admin/categories[/{id}]` | admin | Category CRUD |
| `... /subcategories[/{id}]` | admin | Subcategory CRUD |
| `... /categories/{id}/fields[/{id}]` | admin | Per-category form-field CRUD |

The customer SPA dispatches by mode (`tickets-list` / `tickets-detail`)
exactly the way the shell dispatches by mode (`customer-home`); no
client-side router added.

## Customer UX

Two sidebar entries on the customer SPA (the support plugin's
customer-side nav, surfaced by the shell once `tickets` is enabled):

### List page

- Top: `[Active | Closed | All]` status filter + "Open new ticket" CTA.
- Cards (or rows on wide viewports): tracking number, subject,
  category badge, status badge, last-update relative time, last-entry
  author role.
- Empty state: friendly empty state with the new-ticket CTA front and
  centre.

### New ticket flow (one page, three steps)

1. Pick category (radio cards from `GET /api/customer/categories`).
2. Pick subcategory (radio cards under selected category).
3. Form: subject + body + per-category fields + optional attachments.
   Submit → tracking number shown in confirmation with a copy-to-clipboard
   button and a "View ticket" link.

### Detail page

- Top: tracking number + subject + status badge + category.
- Thread: append-only timeline. Each entry shows author (You / Support
  team / System), relative time, body, attachments inline. `system`
  entries (status changes) render as a light separator: "Status changed
  to *Resolved* by Support team".
- Reply box at bottom with optional attachments. Disabled when status =
  `closed`.
- "Reopen" button when status = `resolved` AND `resolved_at` within 7
  days; otherwise hidden.

## Admin UX

Sidebar entry "Tickets" under support admin (per the shell's
per-module sidebar pattern).

### Queue page

- Table columns: tracking#, subject, customer email, category,
  status, assignee, updated (relative).
- Filters above the table: `[All | Mine | Unassigned] · [All statuses
  ▼] · [All categories ▼] · [Search: tracking# or subject…]`.
- 30s refetch while page is open. Manual refresh button.
- Click row → admin detail page.

### Detail page

- Top: tracking number + subject + status badge + category +
  customer info (email, Continuum user id, "Last speedtest: 220 Mbps,
  3 min ago" — only when the speedtest module is enabled; missing
  module is silent, not error).
- Field values panel (per-category field key/value pairs).
- Thread: same as customer detail BUT with internal notes intermixed
  visually distinct (amber background, "INTERNAL — admin-only" badge).
  System entries inline.
- Right rail "Actions" panel:
  - Status select (open / in_progress / waiting_customer / resolved /
    closed) → posts `/status`.
  - Assignee select (admin list from host SDK) → posts `/assign`.
  - "Add internal note" inline composer → posts `/note`.
- Bottom: reply box (visible to customer) with attachments.

### Categories admin (sub-route under support admin)

- Tree editor: categories on the left, selected category's
  subcategories + form fields on the right.
- Drag-to-reorder, inline rename, soft delete via `active=false`.
- Form-field editor per category: rows of `[key] [label] [kind] [required]`
  with up/down reorder.

## Events emitted

All payloads include this base shape:

```jsonc
{
  "ticket_id": 247,
  "tracking_number": "SUP-247",
  "category": {"id": 3, "slug": "bad-media", "name": "Bad media"},
  "subcategory": {"id": 11, "slug": "audio-glitch", "name": "Audio glitch"},
  "subject": "Audio drops out around the 12:00 mark",
  "status": "open",
  "customer_id": "1234",
  "customer_email": "jane@example.com",
  "assigned_admin_id": null,
  "deep_link": "<plugin mount>/tickets/SUP-247"
}
```

Per-event extras:

| Event | Extra payload keys |
|---|---|
| `plugin.continuum.support.ticket_submitted` | — |
| `plugin.continuum.support.ticket_replied` | `author_role`, `author_id`, `excerpt` (first 280 chars) |
| `plugin.continuum.support.ticket_status_changed` | `from`, `to`, `by` (admin id or "customer" or "system") |
| `plugin.continuum.support.ticket_assigned` | `from_admin_id` (null), `to_admin_id` |
| `plugin.continuum.support.ticket_resolved` | `by` |
| `plugin.continuum.support.ticket_reopened` | `by` (always "customer" in v1) |
| `plugin.continuum.support.ticket_closed` | `by` (admin id or "system") |

Notifications plugin admin defines per-event delivery rules. No
SMTP / push configuration lives in this module.

## SPA Bootstrap Modes

Extends the shell's `supportBootstrap.mode` enum:

- `tickets-list` — customer list page
- `tickets-detail` — customer detail page (tracking_number in URL)
- `admin-tickets-queue` — admin queue
- `admin-tickets-detail` — admin detail

Each mode's bootstrap payload pre-bakes the initial data set so first
paint has content:

- `tickets-list`: first page of tickets + categories for the "new"
  CTA preview.
- `tickets-detail`: full ticket + entries + attachments (metadata
  only — bytes stream from `/api/attachments/{id}`).
- `admin-*`: equivalent admin shapes.

## Swap-ability Notes (loose)

To keep a future swap to go-help-desk realistic without overengineering:

- **Tracking numbers** use a prefix + integer pattern, matching
  go-help-desk's tracking-number-prefix search.
- **Category taxonomy** is two-level; go-help-desk's three-level
  CTI (Category → Type → Item) cleanly accommodates our flat
  subcategory as their "Type", leaving "Item" unused.
- **URLs**: customer detail at `/tickets/{tracking_number}` is the
  same shape go-help-desk uses for its ticket pages.
- **Events** are stable in name and payload — a future adapter can
  publish equivalent events from the new backend so the notifications
  plugin's rules survive.

We do NOT abstract behind a `TicketService` interface or build a
proxy layer. Compatible-ish wire shape only.

## Tests

Go:

- Lifecycle transition tests (every allowed transition + every
  forbidden one returns 409).
- Sequence atomicity (concurrent ticket creation gets distinct
  tracking numbers).
- Customer cannot read another customer's ticket (`GET /api/customer/tickets/{n}`).
- Customer never sees `internal_note` entries.
- Attachment size cap rejects > 10 MB upload with 413.
- Event-emission tests: each lifecycle action emits the right event
  with the documented payload shape (using a fake event publisher).
- Cron auto-close: dry-run table-driven test verifying which tickets
  the close pass picks up.

SPA (vitest + testing-library):

- Customer list renders given a paginated bootstrap.
- New-ticket flow: category → subcategory → form → submit (mocked
  api), confirmation renders tracking number.
- Detail renders thread; reply submits; reopen button visibility
  follows `status` + `resolved_at`.
- Admin queue filters apply (status, assignee, search) — debounced
  search.
- Admin detail renders internal notes distinctly + status / assign
  controls trigger correct POSTs.

## Out of Scope for v1

Each of these is a real feature, but each is its own design + plan
cycle (or its own module). Listed so they're explicit non-goals, not
ambiguous omissions:

- **Live chat / typing indicators** — needs bidirectional realtime
  (WebSocket) which the SDK `Handle` RPC doesn't support. Would force
  a standalone listener.
- **Email-to-ticket (inbound)** — needs IMAP polling or inbound
  webhook + RFC 5322 parsing + threading + dedup.
- **CSAT surveys** — separate post-resolve flow + aggregate dashboard.
- **Attachments > 10 MB** — bytea breaks down; needs object storage.
- **Linked tickets** (related / parent-child / duplicate) — join
  table + UI affordance.
- **Tags** — tag table + selector + filter.
- **Custom fields beyond per-category** — per-ticket attribute
  schema independent of category.
- **Per-subcategory form fields** — extension of category-field
  resolution.
- **AI suggestion at ticket creation** — handled by the AI module
  (program spec module 5), not built into tickets.

If any of these surface as need-to-haves after v1, they re-enter the
brainstorm queue and ship as v1.x / v2 releases.

## Shell Adjustments Required Before This Module Ships

These three are tiny and folded into the shell implementation rather
than re-opening the shell spec:

1. **Body cap middleware** — chi middleware setting `http.MaxBytesReader`
   to ~12 MB on POST/PUT/PATCH routes. Modules that need bigger
   (none in v1, none planned) can opt out per-route.
2. **Mode dispatcher extension** — shell `App.tsx` dispatches by
   `bootstrap.mode`. The shell currently knows `customer-home` /
   `admin-home`; this module's release adds `tickets-list`,
   `tickets-detail`, `admin-tickets-queue`, `admin-tickets-detail`.
   The shell needs no change for this (it just adds cases for the new
   modes); flag noted for completeness.
3. **Scheduled-task capability** — if the SDK supports it (look for
   `scheduled_task.v1` in the notifications plugin's manifest as the
   reference), the shell adds this capability at this module's release
   and the tickets module's cron runs there. If not yet supported,
   fall back to an admin button as documented in the lifecycle
   section. Verify pre-implement.

## Success Criteria

- A customer can open a ticket, see admin replies, reply back, and
  reopen a resolved ticket within 7 days.
- An admin can see the queue, filter / search it, open a ticket,
  reply, leave internal notes, change status, and assign.
- Tracking numbers are unique and monotonically increasing under
  concurrent creation.
- A new ticket fires `plugin.continuum.support.ticket_submitted` with
  the documented payload, and the notifications plugin (configured
  with a rule) delivers an email to the customer-email-snapshot
  address.
- Attachment upload caps at 10 MB with a clear 413 message.
- The cron (or admin button) closes idle resolved + waiting tickets
  per the configured thresholds.
- `make build` passes; `make test` passes; manual smoke against a real
  install creates, replies to, resolves, and reopens a ticket
  end-to-end.
