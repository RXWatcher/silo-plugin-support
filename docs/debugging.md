# Debugging

Indexed by **symptom**, not by module. Each entry lists the most
likely cause first, then less common ones, then how to confirm.

## "plugin not configured" / 503 from every route

The `httproutes.Server` has no inner handler yet. This means
`Configure` has not been called, or `Configure` returned an error
and `onConfig` was never invoked.

Check, in order:

1. Host log for a config-rejected message during install. If
   `database_url` is missing or unreachable, the plugin returns a
   gRPC error from `Configure` and the host backs off.
2. Plugin log on startup — migration failure also surfaces as a
   `Configure` error.
3. From the operator's side: `GET /api/admin/config` will also
   return 503 if not configured, but with the same body, so check
   the host's install-status view first.

## All requests return 401 `unauthenticated`

The host is not injecting `X-Continuum-User-Id`. The plugin
strips any incoming `X-Continuum-*` header on the way in
(see `httproutes.Server.ServeHTTP`) and relies on the host to
re-inject it after auth. If the host edge is misconfigured (no
auth middleware, or a session cookie that the auth layer doesn't
recognise), no value is ever set.

Confirm: `GET /api/admin/config` from a known-admin browser
session — if it's 401, the host edge is the problem, not the
plugin.

## All admin routes return 403 `forbidden`

`X-Continuum-User-Role` is something other than `admin`. The
plugin compares with an exact string equal — `Admin`, `ADMIN`,
or `administrator` will all be rejected. This is a deliberate
narrow contract.

## Migrations fail on startup

Most common cause: the role on the DSN doesn't own the schema or
doesn't have CREATE on it. `golang-migrate` reports the underlying
SQLSTATE on the host log line; `42501` is permission-denied.

Less common: a half-applied migration from a hand-aborted previous
run. The plugin uses a `schema_migrations` table managed by
`golang-migrate`; if that table is in a `dirty` state, fix the
underlying SQL by hand and `UPDATE schema_migrations SET dirty =
FALSE WHERE version = N` from a SQL prompt. Then restart.

## KB articles missing from search results

The customer FTS endpoint
(`GET /api/customer/kb/search?q=...`) hits the
`kb_articles.search_vector` GIN index, but only published
articles are visible. Confirm `status = 'published'` and
`published_at IS NOT NULL`. A draft never appears in search,
related, or list endpoints regardless of `publish_at`.

If the article is published but still doesn't match, remember the
tsvector uses the `english` config with weights A/B/C — short
words like "wifi" tokenize as `wifi`, not `wi-fi`. The
`websearch_to_tsquery('english', ...)` parser in
`KBCustomerSearch` is permissive about phrase punctuation, but
stop-word matching on a 1-2 word title can still come up empty.

## KB images return 404

The customer fetches them via `GET /api/kb/images/{id}`. Two
cases:

1. The `kb_images.id` simply doesn't exist (e.g. the body HTML
   references an ID from an earlier paste). Edit the article to
   strip the broken `<img>`.
2. `kb_images.article_id` is `NULL`. The FK is
   `ON DELETE SET NULL`, so deleting an article does not delete
   its images, but they still serve. If you intentionally want
   to clean orphans, delete by `id` directly.

## A scheduled KB publish never fires

The publish-due pass only runs when
`POST /api/admin/kb/cron/run` is invoked (manually or by the
host scheduler). If nobody hits it, scheduled articles stay in
`draft` past their `publish_at`.

Confirm:

- `SELECT id, slug, publish_at FROM kb_articles WHERE status='draft' AND publish_at <= now();` should return your article.
- Then `POST /api/admin/kb/cron/run`; the article should flip to
  `published` and `published_at = now()`.

The cron is also the source of `kb_article_published` events,
so without the trigger the notifications plugin never gets the
nudge.

## Speedtest: `Auto` always picks the same endpoint

If `auto_strategy = latency`, the customer SPA *should* probe all
candidates and pick the fastest. Today it just takes
`candidates[0]` — see [`follow-ups.md`](./follow-ups.md#speedtest).
The server returns the full candidate list; client-side probe is
the v1.1 fix.

If `auto_strategy = geoip`, the picker depends on the GeoIP chain
returning a country that matches an endpoint's `st_endpoints.country`.
Confirm:

- `GET /api/admin/speedtest/geoip` returns the source chain in
  `sort_order`. The first source whose `Resolve` returns a
  country wins.
- Each source's `last_status` and `last_used_at` show whether it
  fired. A blank `last_used_at` means no traffic has hit it yet
  (or every prior source short-circuited the chain).
- `Resolve` errors are non-fatal — the chain falls through to the
  next source. To force-test a specific source, hit
  `POST /api/admin/speedtest/geoip/{id}/test`.

If no source matches and the strategy is `geoip`, the resolver
returns the first active endpoint with `Strategy: "fallback"`.
The SPA renders that as a non-Auto choice.

## Speedtest: `mmdb_auto` download keeps failing

The downloader resolves `url_pattern` by substituting
`{YYYY-MM}` with the current month. If the upstream rolls publish
slightly late (db-ip.com flips around the 1st of the month), the
current-month URL is a 404 for a day or two. The downloader has a
prev-month fallback baked in (see `mmdb_auto.go`); the only way
this should fail is:

- The cache dir isn't writable. Check `geoip_cache_dir` and the
  plugin user's permissions. Default walks XDG_CACHE_HOME →
  `~/.cache` → relative fallback.
- The mmdb file downloaded fine but is corrupt / wrong format —
  `MMDBFileSource` now surfaces the open error on every `Resolve`
  (fixed in 0.3.0), so check `last_status`.
- The atomic swap (download to `<file>.new`, rename over) failed
  because the dir is on a different filesystem from the temp
  file. Don't put the cache dir somewhere weird.

Force a refresh with
`POST /api/admin/speedtest/geoip/{id}/refresh`. Validate with
`POST /api/admin/speedtest/geoip/{id}/test`.

## Speedtest: country dashboard is empty

`countryHits` in the dashboard payload is always an empty slice.
Known follow-up — the resolver doesn't persist the resolved
country on `st_results`. See
[`follow-ups.md`](./follow-ups.md#speedtest).

## Tickets: customer can't see a category in `/tickets/new`

The category must be active (`tk_categories.active = TRUE`) and
have at least one active subcategory (`tk_subcategories.active =
TRUE` with the matching `category_id`). The customer endpoint
`GET /api/customer/categories` filters on both. The admin
endpoint `GET /api/admin/categories` does not, so the discrepancy
between "I can see it in the admin tree" and "it's missing from
the customer form" is almost always inactive children.

## Tickets: SUP-N skipped a number

It didn't. `tk_ticket_sequence.next_n` is allocated by
`UPDATE ... SET next_n = next_n + 1 RETURNING next_n`. If a
ticket-create transaction rolls back *after* the sequence
allocation, that number is gone — sequence is monotonic, not
gap-free. Treat SUP-N as opaque; the integer is for sort order,
not for "how many tickets have we had".

## Tickets: ticket auto-closed unexpectedly

The cron runs both passes by default:

- `resolved` → `closed` after 7 idle days (configurable via
  `tickets_resolved_close_after_days`).
- `waiting_customer` → `closed` after 14 idle days
  (`tickets_waiting_close_after_days`).

"Idle" means `updated_at`, not `resolved_at` or `waiting_since`.
A reply in either direction touches `updated_at` and resets the
clock; an admin status change does too.

Disable either pass by setting its threshold to `0`. Disable the
entire cron with `tickets_auto_close_enabled = false`. The closed
ticket carries a `system` entry body of `Ticket auto-closed by
cron (resolved_idle)` or `(waiting_idle)`, and a `ticket_closed`
event with `extra.by = "system"` and `extra.reason` set — easy to
filter notifications on if the operator decides the cron is too
noisy.

## Tickets: customer reopen rejected

The reopen endpoint is `POST
/api/customer/tickets/{tracking_number}/reopen`. Two reasons it
returns 409:

1. The ticket's current status is not `resolved`. Reopen is only
   defined out of `resolved`; from `closed` the customer must file
   a new ticket.
2. The 7-day window from `resolved_at` has elapsed. The window is
   defined as `tickets.ReopenWindow = 7 * 24 * time.Hour`, hard-
   coded — not a config key. Pre-empting the reopen window from
   the auto-close cron is intentional: a ticket can auto-close
   from `resolved` after 7 days exactly because the customer
   reopen window is also 7 days, so closing it is the same as
   ending the reopen window from the customer's side.

## Tickets: attachment upload 413 / 400

Two layers cap the body:

- The shell middleware caps every body at 12 MB.
- The attachment handler caps at 10 MB (`tkAttachmentMaxBytes =
  10 << 20`). Anything between 10 and 12 MB hits the handler-
  level check and returns 400 with slug `attachment_too_large`.

KB image upload has its own 5 MB cap. Attempting to upload an
8 MB PNG to KB returns 400 `image_too_large`, not 413.

## Tickets: admin reply doesn't change status

The admin reply endpoint **always** transitions the ticket: from
`open` it moves to `in_progress`; from `waiting_customer` it
stays put (the customer is on the hook, not the admin). The
`allowed` map in `internal/tickets/lifecycle.go` is the source of
truth — if a transition isn't there, the API returns 409
`transition not allowed`. Add a new transition by editing the
map and writing a test against `AllowTransition`.

## `kb_article_updated` event missing on edit

This is the contract-gap fixed in 0.2.0. PUT to an already-
published article now emits `kb_article_updated`. If you're on a
pre-0.2.0 build, upgrade.

## Events emitted but nothing happens

The plugin emits events to the host event bus. It does not deliver
them. If nothing fires on email / push / chat:

1. Confirm the notifications plugin (or whichever consumer) is
   installed and subscribed to the event name.
2. Confirm the consumer's view of the event payload — KB and
   tickets events carry the loaded entity plus an `extra` map.
   The notifications plugin may need a template that references
   `category`, `subcategory`, `customer_email`, etc.

## How to confirm "the plugin actually saw this request"

There is no per-request access log baked in. The cleanest way is
to look for the side effect:

- KB events show up in the host event bus.
- Speedtest results appear in `st_results` (filter by
  `customer_id`).
- Tickets create / reply / status change appear in
  `tk_ticket_entries` (filter by `ticket_id`).

For pure read endpoints (`GET /api/customer/kb/articles` etc.),
hit them from a known-good admin session and compare. A
permanently-403 endpoint is almost always the role header.
