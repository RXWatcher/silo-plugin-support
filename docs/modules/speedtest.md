# Speedtest

LibreSpeed-protocol browser test against operator-defined
endpoints. Two ways to pick an endpoint automatically: parallel
latency probes from the client, or server-side GeoIP IP →
country → matching endpoint. Backing tables `st_*`. Routes under
`/speedtest`, `/admin/speedtest`, `/api/customer/speedtest`,
`/api/admin/speedtest`.

## Admin (operator) quickstart

1. **Add endpoints.** `Admin → Speedtest → Endpoints`. Each row
   in `st_endpoints` is a LibreSpeed backend (PHP or compatible)
   running on infrastructure you control or trust:

   | Field | Notes |
   | --- | --- |
   | `label` | Shown to the customer in the picker. |
   | `url` | Base URL of the LibreSpeed install (no trailing slash). The vendored worker (LGPLv3) appends `/garbage.php`, `/empty.php`, `/getIP.php`, `/upload.php`. |
   | `country` | ISO-3166-1 alpha-2. Used by the `geoip` auto strategy to match. |
   | `region` | Free-form for admin organisation. Not displayed to customer. |
   | `sort_order`, `active` | Standard. Inactive endpoints disappear from customer view immediately. |

   `POST /api/admin/speedtest/endpoints/{id}/ping` runs a one-shot
   liveness check from the plugin process and updates the row's
   transient status. Use this when adding a new endpoint to
   verify it's reachable from the plugin's network.

2. **Pick the auto strategy.** `app_config.auto_strategy` is
   either `latency` (default) or `geoip`:

   - **`latency`** — `GET /api/customer/speedtest/auto` returns
     the full active endpoint list; the SPA is supposed to probe
     each one and pick the fastest. Today it just takes the
     first; see follow-ups for the client-side fix.
   - **`geoip`** — the resolver runs the GeoIP chain against the
     client IP, gets a country code, and returns the first
     active endpoint with a matching `country`. If nothing
     matches it falls back to the first active endpoint with
     `Strategy: "fallback"`. The response always carries an
     `AutoGeoIPHint{country, sourceId, sourceLabel}` for the
     SPA to render "Auto picked X (NL via db-ip.com)".

3. **Configure the GeoIP source chain.** `Admin → Speedtest →
   GeoIP`. Drag-reorder, per-kind config editor, per-source
   refresh + test. Four kinds:

   | Kind | Config | Behaviour |
   | --- | --- | --- |
   | `mmdb_auto` | `{"url_pattern":"...{YYYY-MM}.mmdb.gz","refresh_days":N}` | Downloads, decompresses, atomic-renames into `geoip_cache_dir`. Self-refreshes when the file ages past `refresh_days`. Falls back to prev-month URL if current-month 404s. Pre-seeded with db-ip.com country-lite. |
   | `mmdb_file` | `{"path":"/path/to/file.mmdb"}` | Reads from a path the operator manages out-of-band (e.g. a paid MaxMind feed). Now surfaces the open error on every Resolve (fixed in 0.3.0). |
   | `http_api` | `{"url":"https://...","country_path":"country.code"}` | Calls an external HTTP service (e.g. ipinfo.io), extracts country from a JSON Pointer-style path. |
   | `request_header` | `{"header":"CF-IPCountry"}` | Trusts a header from the edge (Cloudflare, Fastly, an upstream WAF). Use only when you control the edge. |

   The chain is walked top-down on each customer request; the
   first source that returns a non-empty country wins. Errors
   are non-fatal — chain falls through to the next source. The
   winning source's ID and label are returned to the SPA.

4. **Read results.** `Admin → Speedtest → Results` is a paged
   table over `st_results`. `Admin → Speedtest → Dashboards`
   aggregates over the last 30 days. `countryHits` is currently
   empty (see follow-ups).

## Customer (end-user) surface

`/speedtest` — endpoint picker (Auto + manual list), big speed
gauge with download / upload / ping / jitter, 30-day history of
their own runs.

- **Auto button.** The page calls
  `GET /api/customer/speedtest/auto`. The response shape varies
  with strategy — see above.
- **Run.** The vendored LibreSpeed worker drives the test
  client-side. On completion the SPA POSTs the four metrics plus
  `endpoint_id` and `endpoint_label` to
  `POST /api/customer/speedtest/results`.
- **Slow-event hook.** If `download_mbps <
  slow_threshold_mbps`, the server emits a `speedtest_slow`
  event in addition to the always-emitted `speedtest_run`. Set
  `slow_threshold_mbps = 0` to disable.
- **History.** `GET /api/customer/speedtest/results?limit=N`.
  Customer can only see their own (`WHERE customer_id =
  $headers.user-id`).

## Client IP handling

The plugin records the client IP into `st_results.client_ip`
according to `app_config.client_ip_storage`:

- `truncated` (default) — IPv4 → /24, IPv6 → /48. Done via
  `internal/speedtest/iptrunc.go`. Useful enough for country
  spread / unique-prefix counts; not personally identifying.
- `off` — column is NULL.

Setting this to anything else is a config validation error.

## MMDB lifecycle (the slightly fiddly part)

The `mmdb_auto` source's downloader:

1. Substitutes `{YYYY-MM}` in `url_pattern` with the current
   month.
2. Downloads to `<cache_dir>/<source_id>.mmdb.gz.new`.
3. Decompresses to `<cache_dir>/<source_id>.mmdb.new`.
4. Validates via `mmdb_reader.Open`.
5. `os.Rename` over the live file. Atomic on same-fs; that's
   why the cache dir should not span a mount boundary with the
   tmpdir.
6. Updates `last_refreshed_at` and `last_status`.

If the current-month URL 404s (common in the first day or two of
a month if the upstream hasn't published yet), the downloader
substitutes the previous month and retries. `Resolve` on this
source keeps using the previously-cached file while a download
is in-flight; the swap is atomic.

A corrupt `.mmdb` (truncated download, wrong magic, wrong
metadata) is detected by `mmdb_reader.Open` at validation time
and the `.new` files are removed. `last_status` carries the
error message; `Resolve` keeps returning empty until a successful
refresh.

## Events

| Event | When | Payload |
| --- | --- | --- |
| `speedtest_run` | Every successful save | The full `st_results` row |
| `speedtest_slow` | `download_mbps < slow_threshold_mbps` | Same payload plus `threshold_mbps` in `extra` |

## Operator gotchas specific to speedtest

- The `latency` auto strategy currently returns the same first
  endpoint every time (client-side probe not implemented). If
  you have multiple endpoints and care, set `auto_strategy =
  geoip` for now.
- A `request_header` GeoIP source is only safe if you control the
  edge. Anyone who can set that header can spoof their country.
- Deleting an endpoint sets `st_results.endpoint_id` to NULL but
  retains `endpoint_label` so historical results are still
  readable.
