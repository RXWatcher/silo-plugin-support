# Support Plugin — Speedtest Module v1 Design

**Status:** Sub-project design under the program spec
([`2026-05-21-support-plugin-program-design.md`](2026-05-21-support-plugin-program-design.md)).
Third module to ship after the shell + KB, per the program ship order
(shell → KB → **speedtest** → tickets → AI).

**Date:** 2026-05-21
**Sub-project:** Speedtest module v1
**Predecessors:** Shell ([`2026-05-21-support-shell-design.md`](2026-05-21-support-shell-design.md)), KB ([`2026-05-21-support-kb-design.md`](2026-05-21-support-kb-design.md))
**Successor:** Tickets module

## Purpose

Customer self-serve connection diagnostic. Customer hits `/speedtest`,
their browser runs a LibreSpeed-protocol download / upload / ping /
jitter test against an admin-defined endpoint (or "Auto"), result
persists per customer, admin gets dashboards. Cuts ticket volume from
"my video is buffering" by letting the customer see whether the
bottleneck is on their end.

## Decisions Locked During Brainstorm

- **Speedtest engine — LibreSpeed's official JS client.** The
  operator already runs a `librespeed-rs` instance on their
  `speedtest` host, so the protocol is already in place. The JS
  client (~60 KB) handles download/upload/ping/jitter with parallel
  streams and a configurable test profile. New admin-added endpoints
  just need to be LibreSpeed-compatible.
- **"Auto" selection — admin-switchable between latency and geoip.**
  Latency: client fires parallel `/empty.php` pings against every
  active endpoint and picks the lowest RTT (~300 ms before the test
  can start). GeoIP: server resolves the client IP to a country and
  picks the first active endpoint with a matching country tag.
- **GeoIP sources — operator-orderable list, not a hardcoded chain.**
  An admin-managed `st_geoip_sources` table with four kinds:
  `mmdb_auto` (plugin downloads + keeps current; default seeded with
  db-ip.com free), `mmdb_file` (operator-supplied .mmdb path),
  `http_api` (outbound HTTP per IP with 30-day per-IP cache),
  `request_header` (trust an inbound header like `CF-IPCountry`).
  Resolver walks active sources in `sort_order`; first one returning
  a country wins. All-miss falls through to "first enabled endpoint".
- **db-ip.com seeded by default.** First-install migration inserts a
  single `mmdb_auto` source pointing at db-ip.com's free country-lite
  feed (`https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.mmdb.gz`).
  Zero-config GeoIP works on day one; no MaxMind license required.
- **Endpoint label frozen on results.** `st_results.endpoint_label`
  is snapshotted at run time so history stays readable across
  endpoint renames / deletes.
- **Client IP storage — truncated by default.** `/24` for IPv4,
  `/48` for IPv6. Admin can flip a config to `off` (never store).
- **Customer rate limit — 1 test per 60 seconds.** Handler-side
  check against `st_results.ran_at` for the calling `customer_id`.
- **Slow-result event** — `speedtest_slow` fires when download falls
  below `slow_threshold_mbps` (admin-configurable, default 5).
  Operator routes via notifications.

## Schema (extends the `support` Postgres schema)

```sql
CREATE TABLE st_endpoints (
    id          BIGSERIAL PRIMARY KEY,
    label       TEXT NOT NULL,                     -- "London CDN", "Frankfurt AS3320"
    url         TEXT NOT NULL,                     -- LibreSpeed-compatible base URL
    country     TEXT NOT NULL DEFAULT '',          -- ISO 3166-1 alpha-2; '' = any
    region      TEXT NOT NULL DEFAULT '',          -- free-text op note ("EU-West")
    sort_order  INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE st_geoip_sources (
    id                BIGSERIAL PRIMARY KEY,
    label             TEXT NOT NULL,
    kind              TEXT NOT NULL CHECK (kind IN ('mmdb_auto','mmdb_file','http_api','request_header')),
    config            JSONB NOT NULL DEFAULT '{}',
    sort_order        INT NOT NULL DEFAULT 0,
    active            BOOLEAN NOT NULL DEFAULT TRUE,
    last_status       TEXT NOT NULL DEFAULT '',    -- 'ok' | 'error: <msg>' | ''
    last_used_at      TIMESTAMPTZ,
    last_refreshed_at TIMESTAMPTZ,                 -- mmdb_auto only
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE st_results (
    id             BIGSERIAL PRIMARY KEY,
    customer_id    TEXT NOT NULL,
    endpoint_id    BIGINT REFERENCES st_endpoints(id) ON DELETE SET NULL,
    endpoint_label TEXT NOT NULL,                  -- frozen at run time
    auto_strategy  TEXT NOT NULL DEFAULT '',       -- 'latency' | 'geoip' | '' (manual pick)
    download_mbps  NUMERIC(8,2) NOT NULL,
    upload_mbps    NUMERIC(8,2) NOT NULL,
    ping_ms        NUMERIC(8,2) NOT NULL,
    jitter_ms      NUMERIC(8,2) NOT NULL,
    client_ip      INET,                           -- truncated per client_ip_storage
    user_agent     TEXT NOT NULL DEFAULT '',
    ran_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX st_results_customer_idx ON st_results (customer_id, ran_at DESC);
CREATE INDEX st_results_endpoint_idx ON st_results (endpoint_id, ran_at DESC);
CREATE INDEX st_results_ran_at_idx   ON st_results (ran_at DESC);
```

**Seed row** in the up migration (first-install zero-config geoip):

```sql
INSERT INTO st_geoip_sources (label, kind, config, sort_order, active) VALUES (
    'db-ip.com free country-lite',
    'mmdb_auto',
    '{"url_pattern": "https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.mmdb.gz", "refresh_days": 25}'::jsonb,
    0,
    TRUE
);
```

## GeoIP Source Kinds

Each kind interprets `config` differently and resolves IP → ISO
country code (or empty string for "couldn't tell"). Implementations
live behind a `GeoIPSource` interface in `internal/speedtest/geoip/`.

### `mmdb_auto`

Plugin downloads + maintains the `.mmdb` file automatically.

```json
{
  "url_pattern": "https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.mmdb.gz",
  "refresh_days": 25
}
```

- `{YYYY-MM}` interpolated to the current UTC month
- On plugin start: check `last_refreshed_at`; if missing or older
  than `refresh_days`, fetch in a background goroutine. While fetch
  is in flight, source returns empty (chain falls through).
- Daily check via the same admin-trigger cron endpoint the KB
  module already exposes (or a small in-process ticker).
- Atomic swap: download to `${cache_dir}/<source_id>.mmdb.new`,
  fsync, rename over `<source_id>.mmdb`.
- Read via `github.com/oschwald/geoip2-golang`.

### `mmdb_file`

Operator-supplied file; plugin only reads.

```json
{ "path": "/var/lib/maxmind/GeoLite2-Country.mmdb" }
```

- No background refresh — operator manages updates externally
  (cron, MaxMind's `geoipupdate` daemon, etc).
- Fails the source if file missing / unreadable / unparseable.

### `http_api`

Outbound HTTP per unique IP, cached per-IP for 30 days in a
process-local `map[string]countryCacheEntry` (lost on restart;
acceptable — re-warms quickly).

```json
{
  "url_pattern": "https://ipapi.co/{ip}/country/",
  "format": "text",
  "json_path": ""
}
```

- `{ip}` interpolated with the client IP
- `format: "text"` → response body trimmed = country code
- `format: "json"` → response parsed, `json_path` (dot-path)
  followed (e.g. `"country_code"`)
- Per-source request timeout 2 s; failures recorded in
  `last_status` and the source falls through

### `request_header`

Trust an inbound HTTP header (for setups behind Cloudflare /
similar reverse proxies that already inject country).

```json
{ "header": "CF-IPCountry" }
```

- Zero outbound calls, zero latency.
- Returns the header value (uppercased + trimmed); empty string if
  header absent. Reserved header values like `XX` (CF's "unknown
  country") treated as empty.

## Auto-strategy Resolution

`auto_strategy` admin config: `latency` (default) or `geoip`.

`GET /api/customer/speedtest/auto` returns:

```jsonc
{
  "strategy": "latency" | "geoip" | "fallback",
  "endpoint": {                   // null when strategy == "latency"
    "id": 7, "label": "London CDN", "url": "https://lon.cdn.example/"
  },
  "candidates": [                 // populated only when strategy == "latency"
    { "id": 7, "label": "London CDN", "url": "..." },
    { "id": 9, "label": "Frankfurt", "url": "..." }
  ],
  "geoip": {                      // populated only when strategy == "geoip"
    "country": "GB",
    "source_label": "db-ip.com free country-lite"
  }
}
```

Server logic:

```
if AutoStrategy == "geoip":
    country = geoipChain.Resolve(clientIP, request)
    if country != "":
        candidates = endpoints where active AND country = country
        if candidates: return first by sort_order, strategy "geoip"
if AutoStrategy == "latency":
    return strategy "latency" with all active endpoints
# all fallthroughs land here
return endpoints where active order by sort_order limit 1, strategy "fallback"
```

For `strategy: "latency"`, the SPA fires parallel `HEAD` pings
against each `${url}/empty.php` (LibreSpeed's canonical ping
endpoint), measures with `performance.now()`, picks the lowest RTT,
runs the test. The chosen endpoint + measured RTT are surfaced to
the customer ("Auto: London CDN · selected by latency (28 ms)") and
written into the result row as `auto_strategy = "latency"`.

## Customer UX (`/speedtest`)

Single page, three states (idle / running / done).

```
┌──────────────────────────────────────────────────────────┐
│  Speedtest                                                │
│  Test your connection against our endpoints.             │
│                                                           │
│  Run against [Auto                  ▾] [Run test]        │
│   Auto: London CDN · selected by latency (28 ms)         │
│                                                           │
│   ┌───────────────────────────────────────────────────┐  │
│   │                                                   │  │
│   │      Download    Upload      Ping     Jitter      │  │
│   │      142.3 Mb/s  18.7 Mb/s   28 ms    2.1 ms      │  │
│   │                                                   │  │
│   │            ▓▓▓▓▓▓▓░░░░░  Running upload…          │  │
│   │                                                   │  │
│   └───────────────────────────────────────────────────┘  │
│                                                           │
│   Your last 5 tests                                      │
│   2026-05-21 14:02  London CDN     142↓ 18↑ 28ms          │
│   2026-05-20 09:15  Auto·London    138↓ 17↑ 31ms          │
│   ...                                                     │
└──────────────────────────────────────────────────────────┘
```

- Dropdown defaults to "Auto"; admin-active endpoints listed under it
  for manual pick.
- "Run test" disabled while a test is in progress.
- During the test, LibreSpeed's client emits progress events; the
  four numbers update live and a phase line ("Probing… / Downloading…
  / Uploading… / Done") shows below them.
- On completion, POST result to `/api/customer/speedtest/results`;
  list prepends.
- Rate limit: handler returns 429 if the customer's last result is
  within 60 s; SPA shows a friendly "Please wait N seconds" message.

## Admin UX

Sidebar entry "Speedtest" lights up when
`SHIPPED_MODULES.speedtest && modules.speedtest`. Four sub-sections
(in-page sidebar links inside the admin shell):

### Endpoints (`mode: admin-st-endpoints`)

CRUD table: label, URL, country (ISO-2), region (free text),
sort_order, active. Per-row "Test connectivity" button hits the URL
+ reports HTTP status. Drag-to-reorder.

### GeoIP sources (`mode: admin-st-geoip`)

Draggable-reorder list. Each row:

```
  ☰  [icon] mmdb_auto   db-ip.com free country-lite   ok · used 12 min ago  [Refresh] [Edit] [Delete]
  ☰  [icon] http_api    ipapi.co fallback             ok · used 3 min ago   [Test]    [Edit] [Delete]
  ☰  [icon] request_h.  Cloudflare CF-IPCountry       no header seen        [Test]    [Edit] [Delete]
                                                                            [+ Add source]
```

Edit dialog presents only the config fields relevant to the chosen
`kind`. "Test now" button runs a resolution against an operator-
supplied sample IP (defaults to the admin's own IP) and shows the
returned country + which source answered.

### Results (`mode: admin-st-results`)

Table of last 200 results across all customers, filterable by
customer_id, endpoint, date range, strategy, slow-flag. Each row
links to the customer's full history (drawer or separate page).

### Dashboards (`mode: admin-st-dashboards`)

Aggregate charts (last 30 days):

- Median download / upload / ping per endpoint
- Result count per day
- Top 10 slowest customers (median download last 7 days)
- Geographic distribution (count by resolved country) — only when at
  least one geoip source has answered in the window

### Settings (under the shell's existing Configuration page)

`auto_strategy` (radio: Latency / GeoIP), `client_ip_storage` (radio:
Truncated / Off), `slow_threshold_mbps` (number), `geoip_cache_dir`
(text, optional — defaults to OS XDG cache dir).

## Routes (added to the support manifest at this module's release)

| Route | Access | Purpose |
|---|---|---|
| `GET /speedtest` | user | SPA shell, `mode: speedtest` |
| `GET /api/customer/speedtest/endpoints` | user | Active endpoints list |
| `GET /api/customer/speedtest/auto` | user | Resolves "Auto" → endpoint + strategy |
| `POST /api/customer/speedtest/results` | user | Persist result + return updated history |
| `GET /api/customer/speedtest/results` | user | Calling customer's history (last 20) |
| `GET /admin/speedtest` | admin | SPA shell, `mode: admin-st-endpoints` |
| `GET /admin/speedtest/geoip` | admin | SPA shell, `mode: admin-st-geoip` |
| `GET /admin/speedtest/results` | admin | SPA shell, `mode: admin-st-results` |
| `GET /admin/speedtest/dashboards` | admin | SPA shell, `mode: admin-st-dashboards` |
| `GET    /api/admin/speedtest/endpoints` | admin | List |
| `POST   /api/admin/speedtest/endpoints` | admin | Create |
| `PUT    /api/admin/speedtest/endpoints/{id}` | admin | Update |
| `DELETE /api/admin/speedtest/endpoints/{id}` | admin | Delete |
| `POST   /api/admin/speedtest/endpoints/{id}/ping` | admin | Connectivity test |
| `GET    /api/admin/speedtest/geoip` | admin | List sources |
| `POST   /api/admin/speedtest/geoip` | admin | Create source |
| `PUT    /api/admin/speedtest/geoip/{id}` | admin | Update / reorder |
| `DELETE /api/admin/speedtest/geoip/{id}` | admin | Delete |
| `POST   /api/admin/speedtest/geoip/{id}/refresh` | admin | Force re-download (mmdb_auto) |
| `POST   /api/admin/speedtest/geoip/{id}/test` | admin | Run a resolution against a sample IP |
| `GET    /api/admin/speedtest/results` | admin | Filterable list |
| `GET    /api/admin/speedtest/dashboards` | admin | Pre-aggregated chart data |

## Events emitted

Base payload: `customer_id`, `endpoint_id`, `endpoint_label`,
`download_mbps`, `upload_mbps`, `ping_ms`, `jitter_ms`,
`auto_strategy`, `ran_at`.

| Event | Extra keys | When |
|---|---|---|
| `plugin.silo.support.speedtest_run` | — | Every completed test |
| `plugin.silo.support.speedtest_slow` | `threshold_mbps`, `slow_by_mbps` | `download_mbps` < `slow_threshold_mbps` |

Both routed via the existing `silo.notifications` plugin per
admin rules. No SMTP / push config in this module.

## SPA Bootstrap Modes

Extends `supportBootstrap.mode`:

- `speedtest` — pre-bakes active endpoints, default auto resolution
  result, customer's last-5 history
- `admin-st-endpoints` — endpoint list
- `admin-st-geoip` — geoip source list
- `admin-st-results` — filtered result rows (first page)
- `admin-st-dashboards` — 30-day aggregates

## GeoIP DB Lifecycle (`mmdb_auto`)

```
on plugin start:
    for each active mmdb_auto source:
        if cached file missing OR older than (refresh_days × 24h):
            queue background download

background download(source):
    url = source.url_pattern with {YYYY-MM} = current UTC month
    tmp = "${cache_dir}/${source.id}.mmdb.new"
    download url to tmp (HEAD-then-GET, max 50 MB, 60 s timeout)
    gunzip if .gz
    validate by opening with geoip2-golang
    rename(tmp, "${cache_dir}/${source.id}.mmdb")
    update last_refreshed_at + last_status = "ok"

daily ticker (or cron trigger):
    same logic — only re-downloads if month rolled over OR refresh window elapsed
```

Cache dir resolution:
- `geoip_cache_dir` config key if set
- else `$XDG_CACHE_HOME/silo-plugin-support/geoip/`
- else `~/.cache/silo-plugin-support/geoip/`

If the URL pattern's published month doesn't yet exist (db-ip.com
publishes around the 1st), the downloader falls back to the previous
month's URL once before failing.

## Cross-module Integration Points

- **KB → speedtest** (already shipped): article links to
  `/speedtest` work as regular Tiptap links; no special handling.
- **Speedtest → tickets** (when tickets ships): the slow-result
  event can be wired by the operator to auto-open a ticket; tickets
  module reads `speedtest_slow` events via the notifications plugin.
- **Speedtest → AI** (when AI ships): aggregate result data can feed
  the AI module's suggestion engine; AI reads `st_results` directly
  via the shared `Store`.

## Shell Adjustments at This Module's Release

1. `web/src/lib/modules.ts`: flip `SHIPPED_MODULES.speedtest` to `true`.
2. `cmd/.../manifest.json`: append the routes in the table above;
   bump `version` to `0.3.0`.
3. `internal/runtime/runtime.go`:
   - Flip `DefaultAppConfig().Modules.Speedtest` from `false` to `true`.
   - Add `AutoStrategy string`, `GeoIPCacheDir string`,
     `ClientIPStorage string`, `SlowThresholdMbps int` to `Config`.
   - Update `NormalizeAppConfig` defaults.
4. `internal/migrate/files/0003_speedtest_init.up.sql` (+ `.down.sql`).

## Tests

### Go

- Endpoint CRUD round-trip
- GeoIP source CRUD + reorder
- `mmdb_auto` lifecycle: download → atomic swap → version check
  (use a tiny stub server in the test)
- `mmdb_file` reads operator path; absence → empty result, no panic
- `http_api` URL pattern interpolation + text vs json response
  formats + 30-day per-IP cache
- `request_header` reads the header; empty / `XX` → empty result
- Resolver chain walks in `sort_order`; first non-empty wins;
  all-empty falls through
- Auto resolver: latency mode returns candidates list; geoip mode
  returns matched endpoint; fallback when no candidates
- IP truncation: `192.0.2.123` → `192.0.2.0/24`;
  `2001:db8::abcd` → `2001:db8::/48`; `off` storage mode → NULL
- Result insert + 60 s rate limit (returns 429 not OK)
- Slow-event threshold: 4.5 Mbps with threshold 5 → fires;
  5.1 Mbps → doesn't fire

### SPA (vitest + testing-library)

- bootstrap parse for each new mode
- Dropdown defaults to Auto + lists active endpoints
- Auto resolver result renders the chosen endpoint + RTT label
- LibreSpeed client state machine reaches done; on done a POST
  fires with the right payload shape
- 429 surfaces as a "Please wait N seconds" toast, doesn't append
  to history
- Admin geoip sources: drag reorder updates sort_order via PUT;
  Refresh button hits `/refresh`; Test button hits `/test`

## Out of Scope for v1

- Customer-vs-baseline ("expected" speed per plan)
- Per-customer rate-limit bypass for admin testing (admin can run
  tests via their own customer account)
- Self-hosting the LibreSpeed endpoint (operator brings one)
- ASN / ISP lookup (out of scope of LibreSpeed protocol + GeoLite
  country-only DBs)
- Distributed multi-endpoint simultaneous testing
- Mobile-app specific telemetry hooks
- Cron-driven scheduled tests ("test me every Monday")
- Geofencing ("block tests from this country")
- CSV export of results (admin can query DB directly)
- Time-series result charting beyond simple bar dashboards
- Customer-side comparison view ("today vs last week")
- WebRTC P2P speedtest
- IPv4 mapped IPv6 collapsing for the truncation logic (`::ffff:a.b.c.d`)

## Success Criteria

- Admin adds two endpoints (London tagged `GB`, Frankfurt tagged
  `DE`), keeps the default geoip source, switches `auto_strategy`
  to `geoip`.
- Customer in GB hits `/speedtest`; "Auto" picks London; test runs;
  four numbers persist; row appears in their history.
- Customer in DE hits `/speedtest`; "Auto" picks Frankfurt.
- Admin disables the geoip source; "Auto" falls through to the
  first enabled endpoint (London) without error.
- Admin switches `auto_strategy=latency`; customer page now probes
  both endpoints and runs against whichever responds fastest.
- Admin adds a `request_header` geoip source (`CF-IPCountry`),
  drags it above db-ip; a customer behind CF gets routed without
  the mmdb being consulted.
- `mmdb_auto` source on first run: file downloads in background;
  resolver returns empty briefly; after ~10 s starts returning
  countries; status shows `ok` + `last_refreshed_at` populated.
- Slow-result event fires for a sub-threshold download; the
  notifications plugin can route it.
- `make build` + `make test` (Go + vitest) all green.
