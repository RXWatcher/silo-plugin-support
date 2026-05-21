# Tickets

Typed-intake customer support tickets. Two-level category
taxonomy with per-category form fields, SUP-N tracking numbers, a
strict status lifecycle with a customer reopen window,
append-only entries (including admin-only internal notes), 10 MB
attachments, and a daily auto-close cron with two independent
passes. Backing tables `tk_*`. Routes under `/tickets`,
`/admin/tickets`, `/api/customer/tickets`,
`/api/admin/tickets`, `/api/admin/categories`,
`/api/admin/subcategories`, `/api/admin/category-fields`,
`/api/tickets/entries`, `/api/attachments`.

## Admin (operator) quickstart

1. **Set up the category taxonomy.** `Admin → Tickets →
   Categories`. Two levels: `tk_categories` (e.g. "Connection
   problem") and `tk_subcategories` (e.g. "DSL", "Fibre",
   "Mobile") FK-linked to their parent. Both are
   `ON DELETE RESTRICT` against tickets; you cannot delete a
   category that has any tickets ever written against it.

2. **Add per-category form fields.** Each `tk_categories` row
   can carry any number of `tk_category_fields`. Fields are
   typed:

   | `kind` | Renders as | Notes |
   | --- | --- | --- |
   | `text` | `<input type="text">` | Single line. |
   | `textarea` | `<textarea>` | Multi-line. |
   | `number` | `<input type="number">` | Stored as text in `tk_ticket_field_values.value`; client parses. |
   | `url` | `<input type="url">` | Client-side URL validation only. |

   `required` is enforced server-side at ticket create. Field
   `key` is the form name (`UNIQUE (category_id, key)`); `label`
   is the human-readable prompt. Reorder via `sort_order`. v1
   has fields per-category only — subcategories share their
   parent's field set. See follow-ups for per-subcategory.

3. **Watch the queue.** `Admin → Tickets`. Filters: status,
   category, assignee, free-text search (over `tracking_number`
   and `subject`). Status tabs `open / in_progress /
   waiting_customer / resolved / closed` reflect
   `tk_tickets.status` directly. Search is currently un-debounced
   (every keystroke hits the API — fine for small installs;
   v1.1 fix is a 5-line change).

4. **Work a ticket.** `Admin → Tickets → {SUP-N}` shows the
   conversation thread plus an action panel:

   - **Reply.** `POST /api/admin/tickets/{tn}/reply` — appends
     a `reply` entry, transitions `open → in_progress` if
     applicable (the lifecycle map enforces this), emits
     `ticket_replied`.
   - **Internal note.** `POST /api/admin/tickets/{tn}/note` —
     appends an `internal_note` entry. Customer never sees these;
     the customer detail endpoint filters them out.
   - **Status change.** `POST /api/admin/tickets/{tn}/status` —
     explicit lifecycle move (e.g. `in_progress → resolved`).
     Allowed transitions are in `internal/tickets/lifecycle.go`;
     anything else returns 409.
   - **Assign.** `POST /api/admin/tickets/{tn}/assign` —
     `{"adminId": "..."}` or `null` to unassign. Emits
     `ticket_assigned` with the old + new admin in `extra`.

5. **Run the cron.** Trigger
   `POST /api/admin/tickets/cron/run` daily off-peak.
   Resolved → Closed after `tickets_resolved_close_after_days`
   (default 7); Waiting-customer → Closed after
   `tickets_waiting_close_after_days` (default 14). Each closed
   ticket gets a `system` entry body of
   `Ticket auto-closed by cron (resolved_idle)` or
   `(waiting_idle)`, and a `ticket_closed` event with
   `extra.by="system"` and `extra.reason=resolved_idle |
   waiting_idle`. Disable per-pass with `0`; disable globally
   with `tickets_auto_close_enabled = false`.

## Customer (end-user) surface

`/tickets` — list with status tabs over `tk_tickets` filtered to
the current `customer_id`. 30-second polling.

`/tickets/new` — three-step intake:

1. Pick a category (`GET /api/customer/categories`). Only
   categories with at least one active subcategory appear.
2. Pick a subcategory.
3. Fill the per-category form: subject + required body + any
   per-category fields. POST to `POST /api/customer/tickets`
   creates the ticket, allocates the next SUP-N, persists the
   `initial` entry, persists `tk_ticket_field_values` rows, and
   emits `ticket_submitted` with full category + subcategory
   loaded.

`/tickets/{SUP-N}` — detail with thread + reply form +
reopen button (when applicable):

- **Reply.** `POST /api/customer/tickets/{tn}/reply` appends a
  reply, transitions `waiting_customer → in_progress` if
  applicable, emits `ticket_replied`.
- **Reopen.** `POST /api/customer/tickets/{tn}/reopen` only
  allowed when status = `resolved` **and** within 7 days of
  `resolved_at`. Emits `ticket_reopened`. After the window, the
  customer must file a new ticket.
- **Attachments.** Upload to an existing entry via
  `POST /api/tickets/entries/{entry_id}/attachments` (multipart,
  10 MB cap). The owner check ensures only the ticket's customer
  or any admin can attach.

## Lifecycle reference

Source of truth: `internal/tickets/lifecycle.go`.

```
open
  ├─ admin reply ─► in_progress
  └─ admin status ─► closed

in_progress
  ├─ admin status ─► waiting_customer
  ├─ admin status ─► resolved
  └─ admin status ─► closed

waiting_customer
  ├─ customer reply ─► in_progress
  ├─ admin status ─► resolved
  ├─ admin status ─► closed
  └─ cron idle ────► closed

resolved
  ├─ customer reopen ─► in_progress  (only within 7 days)
  ├─ admin status ────► closed
  └─ cron idle ───────► closed

closed
  └─ (terminal — open a new ticket)
```

Every transition fires `ticket_status_changed` with the old +
new status in `extra`. Some transitions additionally fire a
named event: `ticket_resolved` on `* → resolved`,
`ticket_closed` on `* → closed`, `ticket_reopened` on `resolved
→ in_progress` via `TriggerCustomerReopen`.

## Tracking number allocation

`tk_ticket_sequence` is a singleton row. On ticket create the
store runs `UPDATE tk_ticket_sequence SET next_n = next_n + 1
WHERE id = 1 RETURNING next_n` and formats the result as
`SUP-N`. Atomic; concurrent creates serialise on the row. A
rolled-back transaction loses its allocation — sequence is
monotonic, not gap-free.

## Attachments

`tk_attachments`, `BYTEA` inline up to 10 MB. Auth on serve
(`GET /api/attachments/{id}`): admin sees everything; non-admin
must own the parent ticket (`tk_tickets.customer_id = caller`).
Same gate on upload. The 10 MB cap is enforced both at the
handler (`tkAttachmentMaxBytes`) and by the shell's 12 MB body
cap.

## Events

| Event | When | Notes |
| --- | --- | --- |
| `ticket_submitted` | Customer creates a ticket | Includes full category + subcategory (fixed in 0.4.0) |
| `ticket_replied` | Customer or admin reply | `extra.by` = `customer` or `admin` |
| `ticket_status_changed` | Every successful transition | `extra.from`, `extra.to`, `extra.trigger` |
| `ticket_assigned` | `assign` endpoint | `extra.from_admin_id`, `extra.to_admin_id` |
| `ticket_resolved` | `* → resolved` | Subset of `status_changed` |
| `ticket_reopened` | `resolved → in_progress` via customer | Subset of `status_changed` |
| `ticket_closed` | `* → closed` | `extra.by` + `extra.reason` when cron-driven |

## Operator gotchas specific to tickets

- Customer can't see a category in `/tickets/new` →
  `tk_subcategories.active = FALSE` for every child of that
  category. Activate at least one.
- A `text` or `textarea` field marked `required` is enforced on
  create. Renaming the `key` of an existing field after tickets
  reference it will orphan their `tk_ticket_field_values` rows
  (FK is on `field_id`, not on the key string — so the data
  survives, but the admin detail view drops the label).
- Auto-close "idle" is `updated_at`, not `resolved_at` /
  `waiting_since`. Replies and status changes both reset the
  clock.
- `TKUpdateTicketStatus` still takes vestigial `waitingSince,
  resolvedAt *interface{}` parameters that are unused — the SQL
  `CASE` handles timestamps. Cleanup is a v1.1 follow-up.
