# Follow-ups

Known v1.1+ items across the four shipped modules. None blocks
production use; each is a deliberate v1 scope cut documented at
ship time.

## Cross-cutting

- **Native `scheduled_task.v1` SDK capability.** Each module's
  cron is currently exposed as an admin-button endpoint. When the
  SDK ships the capability, all three (KB unhelpful sweep,
  tickets auto-close, speedtest mmdb refresh) should declare it
  and become true scheduled tasks. The admin-button endpoints
  stay as a manual-trigger fallback.

## Shell

(No outstanding follow-ups. The 0.1 → 0.4 progression has folded
the per-release cleanup into module ships.)

## KB

- **SVG upload with proper sanitiser.** SVG was dropped from the
  image-upload MIME allowlist post-ship because the v1 byte-scan
  ("reject if `<script`/`onerror=`/`onload=` substring present")
  misses CDATA-wrapped scripts and hex-encoded handlers. Adding
  SVG back requires a real Go SVG sanitiser (candidate:
  `github.com/tdewolff/minify/v2/svg` with a strict policy).
- **Time-series engagement chart.** Admin per-article engagement
  view currently shows aggregate counts (helpful / not helpful /
  views) and a ratio bar. Spec hinted at a 30-day time series;
  v1 ships aggregate only.

## Speedtest

- **Client-side latency probe.** In `latency` auto-strategy, the
  customer page currently falls back to "first candidate" instead
  of running parallel `HEAD /empty.php` probes and picking the
  fastest. Real probe is documented in `Speedtest.tsx` with a
  comment block. Cleanest fix is an inline `Promise.all` over
  `performance.now()`-timed fetches.
- **`countryHits` dashboard aggregate.** The dashboard data shape
  includes a `countryHits: { country, count }[]` field but the
  store returns an empty slice. Populating it needs a resolved-
  country column on `st_results` (currently we resolve at request
  time but don't persist the result).

## Tickets

- **"Last speedtest" cross-module hint on admin detail.** Spec
  called for surfacing the customer's most recent speedtest
  result on the admin ticket detail page when the speedtest
  module is enabled. Deferred — requires either a new endpoint or
  a join through `st_results`.
- **Debounced admin queue search.** Spec called for debouncing
  the search input; current implementation fires
  `listTKAdminQueue` on every keystroke. For a small install this
  is harmless; would be noisy for a busy operator. ~5-line fix.
- **Vestigial `*interface{}` parameters on `TKUpdateTicketStatus`.**
  The signature takes `waitingSince, resolvedAt *interface{}`
  that are never read — the SQL `CASE` handles all timestamp
  side-effects internally. Should be removed; the cron's
  `CronStore` interface mirrors the same dead params.
- **Per-subcategory form fields.** v1 has fields per-category
  only; subcategories share the parent's fields. Adding
  per-subcategory fields needs a `tk_subcategory_fields` table or
  a `tk_category_fields.subcategory_id NULL FK`.

## AI Assistance (program ship-order #5)

Not started. Original program spec describes the module as KB
suggestion on ticket create + embedding similarity over articles.
Fresh brainstorm → spec → plan → implement cycle when the
operator is ready.
