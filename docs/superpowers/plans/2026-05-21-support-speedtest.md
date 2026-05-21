# Support Plugin — Speedtest Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Speedtest module to `continuum-plugin-support` — customer browser runs a LibreSpeed test against admin-defined endpoints; "Auto" switchable between latency-probe and admin-orderable geoip-source chain (db-ip.com seeded by default, auto-downloaded and kept current); results persisted per customer; admin gets endpoint / geoip / results / dashboards UIs.

**Architecture:** Extends the support plugin shell + KB module. Three new tables under the `support` schema (`st_endpoints`, `st_geoip_sources`, `st_results`). New routes added to the existing manifest. GeoIP resolution is a pluggable chain (`mmdb_auto` / `mmdb_file` / `http_api` / `request_header`) walked in admin-set `sort_order`. SPA gains 5 new bootstrap modes; customer page embeds LibreSpeed's official JS worker.

**Tech Stack:** Go 1.26 (existing). New Go deps: `github.com/oschwald/geoip2-golang` for `.mmdb` reads. Frontend new: LibreSpeed's `speedtest_worker.js` (vendored verbatim from the upstream repo, GPL-2 licensed; the implementer wraps it in a small TS module — no npm package is depended upon). Everything else is the same stack as the shell + KB.

**Spec:** [`../specs/2026-05-21-support-speedtest-design.md`](../specs/2026-05-21-support-speedtest-design.md)
**Predecessor:** KB module shipped in commits `2e7255e..602d71d` on `main`.

---

## File Structure

All paths relative to `/opt/continuum_plugins/continuum-plugin-support/`.

### Go side

| File | Responsibility |
|---|---|
| `internal/migrate/files/0003_speedtest_init.up.sql` + `.down.sql` | Create `st_*` tables + indexes + db-ip seed row |
| `internal/store/st_types.go` | Go types for ST rows (Endpoint, GeoIPSource, Result, ResultFilter) |
| `internal/store/st_endpoints.go` | Endpoints CRUD + ping helper |
| `internal/store/st_geoip_sources.go` | GeoIP sources CRUD + reorder + status updates |
| `internal/store/st_results.go` | Results insert + history queries + dashboard aggregates |
| `internal/speedtest/iptrunc.go` | IP truncation (/24 v4, /48 v6) + Off mode |
| `internal/speedtest/iptrunc_test.go` | Truncation tests |
| `internal/speedtest/geoip/source.go` | `Source` interface + shared types |
| `internal/speedtest/geoip/chain.go` | Chain walker (calls sources in `sort_order`, returns first non-empty) |
| `internal/speedtest/geoip/chain_test.go` | Chain walker tests with fake sources |
| `internal/speedtest/geoip/mmdb_reader.go` | Shared mmdb reader used by `mmdb_auto` + `mmdb_file` |
| `internal/speedtest/geoip/mmdb_auto.go` | `mmdb_auto` source kind |
| `internal/speedtest/geoip/mmdb_file.go` | `mmdb_file` source kind |
| `internal/speedtest/geoip/http_api.go` | `http_api` source kind + 30-day per-IP cache |
| `internal/speedtest/geoip/http_api_test.go` | URL pattern + json/text format + cache tests |
| `internal/speedtest/geoip/request_header.go` | `request_header` source kind |
| `internal/speedtest/geoip/request_header_test.go` | Header reading tests |
| `internal/speedtest/geoip/downloader.go` | mmdb_auto background download lifecycle (gunzip, validate, atomic swap) |
| `internal/speedtest/auto.go` | Auto-strategy resolver (latency vs geoip) |
| `internal/speedtest/auto_test.go` | Resolver tests with fakes |
| `internal/server/handlers_st_customer.go` | Customer ST API + SPA shell handler |
| `internal/server/handlers_st_admin.go` | Admin endpoints + geoip + results + dashboards handlers |
| `internal/server/st_events.go` | `speedtest_run` / `speedtest_slow` publisher helpers |
| `internal/server/server.go` | Add ST routes to the chi router |
| `internal/server/spa.go` | Add new bootstrap modes to `supportBootstrap` |
| `cmd/continuum-plugin-support/main.go` | Wire GeoIP chain + downloader ticker into `applyConfig` |
| `cmd/continuum-plugin-support/manifest.json` | Add ST http_routes + bump version to 0.3.0 |
| `internal/runtime/runtime.go` | Add ST-related Config fields + flip `Modules.Speedtest` default |

### Web side

| File | Responsibility |
|---|---|
| `web/public/speedtest_worker.js` | Vendored LibreSpeed worker (GPL-2; verbatim from upstream) |
| `web/src/lib/modules.ts` | Flip `SHIPPED_MODULES.speedtest` to `true` (final unit) |
| `web/src/lib/types.ts` | Extend with ST types + bootstrap mode union |
| `web/src/lib/librespeedClient.ts` | TS wrapper around the worker (typed events, start/stop) |
| `web/src/api/st.ts` | Customer ST API client |
| `web/src/api/stAdmin.ts` | Admin ST API client |
| `web/src/components/st/EndpointPicker.tsx` (+ test) | Auto + manual dropdown |
| `web/src/components/st/SpeedGauge.tsx` | Live numbers panel (download / upload / ping / jitter) |
| `web/src/components/st/HistoryList.tsx` | Customer's last-N test rows |
| `web/src/components/admin/st/EndpointAdmin.tsx` | CRUD table for endpoints |
| `web/src/components/admin/st/GeoIPSourceAdmin.tsx` | Draggable source list + per-kind edit dialogs |
| `web/src/components/admin/st/ResultsTable.tsx` | Filterable results table |
| `web/src/components/admin/st/Dashboards.tsx` | Aggregate bar charts |
| `web/src/pages/st/Speedtest.tsx` | Customer speedtest page (state machine: idle/probing/running/done/error) |
| `web/src/pages/admin/st/Endpoints.tsx` | Admin endpoints page |
| `web/src/pages/admin/st/GeoIP.tsx` | Admin geoip sources page |
| `web/src/pages/admin/st/Results.tsx` | Admin results page |
| `web/src/pages/admin/st/Dashboards.tsx` | Admin dashboards page |
| `web/src/App.tsx` | Dispatch 5 new bootstrap modes |

---

## Phase A — Foundation

### Task A1: Migration `0003_speedtest_init`

**Files:**
- Create: `internal/migrate/files/0003_speedtest_init.up.sql`
- Create: `internal/migrate/files/0003_speedtest_init.down.sql`

- [ ] **Step 1: Write the up migration (includes db-ip seed row)**

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > internal/migrate/files/0003_speedtest_init.up.sql <<'EOF'
CREATE TABLE st_endpoints (
    id          BIGSERIAL PRIMARY KEY,
    label       TEXT NOT NULL,
    url         TEXT NOT NULL,
    country     TEXT NOT NULL DEFAULT '',
    region      TEXT NOT NULL DEFAULT '',
    sort_order  INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX st_endpoints_active_sort_idx ON st_endpoints (active, sort_order);

CREATE TABLE st_geoip_sources (
    id                BIGSERIAL PRIMARY KEY,
    label             TEXT NOT NULL,
    kind              TEXT NOT NULL
        CHECK (kind IN ('mmdb_auto','mmdb_file','http_api','request_header')),
    config            JSONB NOT NULL DEFAULT '{}',
    sort_order        INT NOT NULL DEFAULT 0,
    active            BOOLEAN NOT NULL DEFAULT TRUE,
    last_status       TEXT NOT NULL DEFAULT '',
    last_used_at      TIMESTAMPTZ,
    last_refreshed_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX st_geoip_sources_active_sort_idx ON st_geoip_sources (active, sort_order);

CREATE TABLE st_results (
    id             BIGSERIAL PRIMARY KEY,
    customer_id    TEXT NOT NULL,
    endpoint_id    BIGINT REFERENCES st_endpoints(id) ON DELETE SET NULL,
    endpoint_label TEXT NOT NULL,
    auto_strategy  TEXT NOT NULL DEFAULT '',
    download_mbps  NUMERIC(8,2) NOT NULL,
    upload_mbps    NUMERIC(8,2) NOT NULL,
    ping_ms        NUMERIC(8,2) NOT NULL,
    jitter_ms      NUMERIC(8,2) NOT NULL,
    client_ip      INET,
    user_agent     TEXT NOT NULL DEFAULT '',
    ran_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX st_results_customer_idx ON st_results (customer_id, ran_at DESC);
CREATE INDEX st_results_endpoint_idx ON st_results (endpoint_id, ran_at DESC);
CREATE INDEX st_results_ran_at_idx   ON st_results (ran_at DESC);

-- Seed one mmdb_auto source pointing at db-ip.com's free country-lite
-- feed so geoip resolution Just Works out of the box. Admin can
-- disable, reorder, or delete this row.
INSERT INTO st_geoip_sources (label, kind, config, sort_order, active) VALUES (
    'db-ip.com free country-lite',
    'mmdb_auto',
    '{"url_pattern": "https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.mmdb.gz", "refresh_days": 25}'::jsonb,
    0,
    TRUE
);
EOF
```

- [ ] **Step 2: Write the down migration**

```bash
cat > internal/migrate/files/0003_speedtest_init.down.sql <<'EOF'
DROP TABLE IF EXISTS st_results;
DROP TABLE IF EXISTS st_geoip_sources;
DROP TABLE IF EXISTS st_endpoints;
EOF
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/migrate/files/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(migrate): 0003 speedtest tables + db-ip seed"
```

---

### Task A2: Store types

**Files:**
- Create: `internal/store/st_types.go`

- [ ] **Step 1: Write the file**

```bash
cat > internal/store/st_types.go <<'EOF'
package store

import (
	"encoding/json"
	"time"
)

// STEndpoint mirrors a row in st_endpoints.
type STEndpoint struct {
	ID        int64     `json:"id"`
	Label     string    `json:"label"`
	URL       string    `json:"url"`
	Country   string    `json:"country"`
	Region    string    `json:"region"`
	SortOrder int       `json:"sortOrder"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// STGeoIPSource mirrors a row in st_geoip_sources. Config is kept as
// raw JSON so the source-kind packages can unmarshal into their own
// typed config struct.
type STGeoIPSource struct {
	ID              int64           `json:"id"`
	Label           string          `json:"label"`
	Kind            string          `json:"kind"`
	Config          json.RawMessage `json:"config"`
	SortOrder       int             `json:"sortOrder"`
	Active          bool            `json:"active"`
	LastStatus      string          `json:"lastStatus"`
	LastUsedAt      *time.Time      `json:"lastUsedAt,omitempty"`
	LastRefreshedAt *time.Time      `json:"lastRefreshedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// STResult mirrors a row in st_results. ClientIP is rendered as the
// truncated CIDR string the truncation logic wrote.
type STResult struct {
	ID            int64     `json:"id"`
	CustomerID    string    `json:"customerId"`
	EndpointID    *int64    `json:"endpointId,omitempty"`
	EndpointLabel string    `json:"endpointLabel"`
	AutoStrategy  string    `json:"autoStrategy"`
	DownloadMbps  float64   `json:"downloadMbps"`
	UploadMbps    float64   `json:"uploadMbps"`
	PingMs        float64   `json:"pingMs"`
	JitterMs      float64   `json:"jitterMs"`
	ClientIP      string    `json:"clientIp,omitempty"` // truncated CIDR or empty
	UserAgent     string    `json:"userAgent,omitempty"`
	RanAt         time.Time `json:"ranAt"`
}

// STResultFilter narrows what STListResults returns. Zero values are
// wildcards.
type STResultFilter struct {
	CustomerID   string
	EndpointID   int64
	AutoStrategy string
	SlowOnly     bool   // download below admin-configured threshold
	SlowThresh   float64
	Since        time.Time
	Limit        int
	Offset       int
}

// STDashboardAggregates is the per-endpoint rollup the admin
// dashboard page renders. Counts are over the last 30 days.
type STDashboardAggregates struct {
	PerEndpoint []STEndpointAggregate `json:"perEndpoint"`
	PerDay      []STDailyCount        `json:"perDay"`
	SlowTop10   []STResult            `json:"slowTop10"`
	CountryHits []STCountryCount      `json:"countryHits"`
}

type STEndpointAggregate struct {
	EndpointID    *int64  `json:"endpointId,omitempty"`
	Label         string  `json:"label"`
	MedianDownload float64 `json:"medianDownload"`
	MedianUpload   float64 `json:"medianUpload"`
	MedianPing     float64 `json:"medianPing"`
	ResultCount    int     `json:"resultCount"`
}

type STDailyCount struct {
	Day   string `json:"day"`   // YYYY-MM-DD
	Count int    `json:"count"`
}

type STCountryCount struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}
EOF

go build ./internal/store/...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/st_types.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): speedtest row types"
```

---

### Task A3: IP truncation helper (TDD)

**Files:**
- Create: `internal/speedtest/iptrunc.go`
- Create: `internal/speedtest/iptrunc_test.go`

- [ ] **Step 1: Write the failing tests**

```bash
mkdir -p internal/speedtest
cat > internal/speedtest/iptrunc_test.go <<'EOF'
package speedtest

import "testing"

func TestTruncateIPv4To24(t *testing.T) {
	got := TruncateIP("192.0.2.123", "truncated")
	if got != "192.0.2.0/24" {
		t.Fatalf("TruncateIP = %q, want 192.0.2.0/24", got)
	}
}

func TestTruncateIPv6To48(t *testing.T) {
	got := TruncateIP("2001:db8:1234:5678::abcd", "truncated")
	if got != "2001:db8:1234::/48" {
		t.Fatalf("TruncateIP = %q, want 2001:db8:1234::/48", got)
	}
}

func TestTruncateOffReturnsEmpty(t *testing.T) {
	if got := TruncateIP("192.0.2.123", "off"); got != "" {
		t.Fatalf("TruncateIP(off) = %q, want empty", got)
	}
}

func TestTruncateUnknownStorageDefaultsToTruncated(t *testing.T) {
	if got := TruncateIP("192.0.2.123", "weird"); got != "192.0.2.0/24" {
		t.Fatalf("TruncateIP(weird) = %q, want truncated default", got)
	}
}

func TestTruncateInvalidIPReturnsEmpty(t *testing.T) {
	if got := TruncateIP("not-an-ip", "truncated"); got != "" {
		t.Fatalf("TruncateIP(bad) = %q, want empty", got)
	}
}
EOF
go test ./internal/speedtest/... 2>&1 | tail -5    # expect undefined
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/speedtest/iptrunc.go <<'EOF'
// Package speedtest holds the speedtest module's non-store helpers:
// IP truncation, the auto-strategy resolver, and the GeoIP chain
// glue. The geoip subpackage holds source-kind implementations.
package speedtest

import "net/netip"

// TruncateIP reduces a client IP per the operator's privacy setting.
// "off" returns empty (caller persists NULL); anything else (default
// "truncated") returns the /24 (IPv4) or /48 (IPv6) CIDR string.
// Invalid input returns empty.
func TruncateIP(ip, storage string) string {
	if storage == "off" {
		return ""
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	var prefix netip.Prefix
	if addr.Is4() {
		prefix = netip.PrefixFrom(addr, 24).Masked()
	} else {
		prefix = netip.PrefixFrom(addr, 48).Masked()
	}
	return prefix.String()
}
EOF
go test ./internal/speedtest/... -v   # 5 tests must pass
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(speedtest): IP truncation helper (/24 v4, /48 v6, off)"
```

---

## Phase B — GeoIP infrastructure

### Task B1: Source interface + chain walker (TDD)

**Files:**
- Create: `internal/speedtest/geoip/source.go`
- Create: `internal/speedtest/geoip/chain.go`
- Create: `internal/speedtest/geoip/chain_test.go`

- [ ] **Step 1: Failing chain test**

```bash
mkdir -p internal/speedtest/geoip
cat > internal/speedtest/geoip/chain_test.go <<'EOF'
package geoip

import (
	"context"
	"net/http"
	"testing"
)

type fakeSource struct {
	id       int64
	country  string
	failErr  error
	callCount int
}

func (f *fakeSource) ID() int64 { return f.id }
func (f *fakeSource) Kind() string { return "fake" }
func (f *fakeSource) Resolve(_ context.Context, _ string, _ *http.Request) (string, error) {
	f.callCount++
	return f.country, f.failErr
}

type recordingStatus struct {
	updates []statusUpdate
}

type statusUpdate struct {
	sourceID int64
	used     bool
	status   string
}

func (r *recordingStatus) MarkUsed(id int64)           { r.updates = append(r.updates, statusUpdate{id, true, "ok"}) }
func (r *recordingStatus) MarkStatus(id int64, s string) { r.updates = append(r.updates, statusUpdate{id, false, s}) }

func TestChainReturnsFirstNonEmpty(t *testing.T) {
	a := &fakeSource{id: 1, country: ""}
	b := &fakeSource{id: 2, country: "GB"}
	c := &fakeSource{id: 3, country: "US"}
	rec := &recordingStatus{}
	chain := NewChain([]Source{a, b, c}, rec)

	got, srcID, err := chain.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "GB" || srcID != 2 {
		t.Fatalf("got (%q, %d), want (GB, 2)", got, srcID)
	}
	if c.callCount != 0 {
		t.Fatalf("third source should not have been called once we got a hit")
	}
}

func TestChainAllMissReturnsEmpty(t *testing.T) {
	a := &fakeSource{country: ""}
	b := &fakeSource{country: ""}
	chain := NewChain([]Source{a, b}, &recordingStatus{})
	got, srcID, err := chain.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil || got != "" || srcID != 0 {
		t.Fatalf("got (%q, %d, %v), want ('', 0, nil)", got, srcID, err)
	}
}

func TestChainErrorOnSourceMovesOn(t *testing.T) {
	a := &fakeSource{country: "", failErr: context.Canceled}
	b := &fakeSource{id: 2, country: "GB"}
	chain := NewChain([]Source{a, b}, &recordingStatus{})
	got, srcID, _ := chain.Resolve(context.Background(), "192.0.2.1", nil)
	if got != "GB" || srcID != 2 {
		t.Fatalf("got (%q, %d), want (GB, 2)", got, srcID)
	}
}
EOF
go test ./internal/speedtest/geoip/... 2>&1 | tail -5   # expect undefined
```

- [ ] **Step 2: Implementation — `source.go` interface**

```bash
cat > internal/speedtest/geoip/source.go <<'EOF'
// Package geoip implements GeoIP source kinds + the chain walker
// the auto-strategy resolver consults.
package geoip

import (
	"context"
	"net/http"
)

// Source is a single GeoIP lookup strategy. Resolve returns:
//   - country: ISO 3166-1 alpha-2 (uppercased) or "" if the source
//     can't answer for this IP / request
//   - err:     a transient/operational error (network, file read).
//     The chain walker treats "" + err as "miss" and moves on; the
//     error is propagated to status logging via the StatusSink.
type Source interface {
	ID() int64
	Kind() string
	Resolve(ctx context.Context, ip string, r *http.Request) (country string, err error)
}

// StatusSink lets the chain record per-source success / failure so the
// admin UI can surface "ok / used 12 min ago" or "error: dns timeout".
type StatusSink interface {
	MarkUsed(sourceID int64)
	MarkStatus(sourceID int64, status string) // "ok" on success, "error: ..." on failure
}
EOF
```

- [ ] **Step 3: Implementation — `chain.go`**

```bash
cat > internal/speedtest/geoip/chain.go <<'EOF'
package geoip

import (
	"context"
	"fmt"
	"net/http"
)

// Chain walks an ordered list of Sources, returning the country from
// the first source that returns a non-empty result. Sources that
// error are marked in the status sink and skipped.
type Chain struct {
	sources []Source
	status  StatusSink
}

func NewChain(sources []Source, status StatusSink) *Chain {
	return &Chain{sources: sources, status: status}
}

// Resolve returns (country, sourceID, err). country may be "" if no
// source answered. err is non-nil only for context cancellation —
// individual source failures are absorbed and logged via the status
// sink so a flaky source can't block the chain.
func (c *Chain) Resolve(ctx context.Context, ip string, r *http.Request) (string, int64, error) {
	for _, src := range c.sources {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		country, err := src.Resolve(ctx, ip, r)
		if err != nil {
			c.status.MarkStatus(src.ID(), fmt.Sprintf("error: %s", err))
			continue
		}
		if country == "" {
			continue
		}
		c.status.MarkUsed(src.ID())
		return country, src.ID(), nil
	}
	return "", 0, nil
}
EOF
go test ./internal/speedtest/geoip/... -v   # 3 chain tests pass
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/geoip/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(geoip): Source interface + chain walker"
```

---

### Task B2: `mmdb_reader.go` shared reader

**Files:**
- Create: `internal/speedtest/geoip/mmdb_reader.go`

- [ ] **Step 1: Add the dep**

```bash
cd /opt/continuum_plugins/continuum-plugin-support
go get github.com/oschwald/geoip2-golang@latest
go mod tidy
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/speedtest/geoip/mmdb_reader.go <<'EOF'
package geoip

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

// mmdbReader wraps geoip2.Reader with a mutex so the file can be
// hot-swapped (mmdb_auto downloader replaces the underlying file +
// reopens). Both mmdb_auto and mmdb_file use one of these.
type mmdbReader struct {
	mu     sync.RWMutex
	reader *geoip2.Reader
	path   string
}

func newMMDBReader() *mmdbReader { return &mmdbReader{} }

// Open the .mmdb at path. Replaces any previously open reader.
// Returns an error if the file is missing or unparseable.
func (m *mmdbReader) Open(path string) error {
	r, err := geoip2.Open(path)
	if err != nil {
		return fmt.Errorf("open mmdb %q: %w", path, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reader != nil {
		_ = m.reader.Close()
	}
	m.reader = r
	m.path = path
	return nil
}

// Country looks up the ISO country code for ip. Returns "" if the
// reader has not been opened yet OR the lookup misses (private IP,
// reserved range). Errors are returned for parse failures only.
func (m *mmdbReader) Country(_ context.Context, ip string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.reader == nil {
		return "", nil
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("invalid ip %q", ip)
	}
	rec, err := m.reader.Country(parsed)
	if err != nil {
		return "", fmt.Errorf("mmdb lookup: %w", err)
	}
	return rec.Country.IsoCode, nil
}
EOF
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/geoip/mmdb_reader.go go.mod go.sum
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(geoip): shared mmdb reader (hot-swap-safe)"
```

---

### Task B3: `mmdb_file` source kind

**Files:**
- Create: `internal/speedtest/geoip/mmdb_file.go`

- [ ] **Step 1: Implementation**

```bash
cat > internal/speedtest/geoip/mmdb_file.go <<'EOF'
package geoip

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
)

// MMDBFileConfig is the operator-supplied path to an .mmdb file the
// plugin only reads. Updates are the operator's job (cron,
// geoipupdate, etc).
type MMDBFileConfig struct {
	Path string `json:"path"`
}

// MMDBFileSource implements Source for `kind = 'mmdb_file'`.
type MMDBFileSource struct {
	id     int64
	cfg    MMDBFileConfig
	reader *mmdbReader
	once   sync.Once
}

func NewMMDBFileSource(id int64, rawCfg json.RawMessage) (*MMDBFileSource, error) {
	var cfg MMDBFileConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	return &MMDBFileSource{id: id, cfg: cfg, reader: newMMDBReader()}, nil
}

func (m *MMDBFileSource) ID() int64    { return m.id }
func (m *MMDBFileSource) Kind() string { return "mmdb_file" }

func (m *MMDBFileSource) Resolve(ctx context.Context, ip string, _ *http.Request) (string, error) {
	if m.cfg.Path == "" {
		return "", nil
	}
	if _, err := os.Stat(m.cfg.Path); err != nil {
		return "", err
	}
	m.once.Do(func() { _ = m.reader.Open(m.cfg.Path) })
	return m.reader.Country(ctx, ip)
}
EOF
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/geoip/mmdb_file.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(geoip): mmdb_file source kind"
```

---

### Task B4: `request_header` source kind (TDD)

**Files:**
- Create: `internal/speedtest/geoip/request_header.go`
- Create: `internal/speedtest/geoip/request_header_test.go`

- [ ] **Step 1: Failing tests**

```bash
cat > internal/speedtest/geoip/request_header_test.go <<'EOF'
package geoip

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestHeaderReadsConfiguredHeader(t *testing.T) {
	src, err := NewRequestHeaderSource(1, json.RawMessage(`{"header": "CF-IPCountry"}`))
	if err != nil { t.Fatal(err) }
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("CF-IPCountry", "gb")
	got, err := src.Resolve(context.Background(), "192.0.2.1", r)
	if err != nil { t.Fatal(err) }
	if got != "GB" {
		t.Fatalf("got %q, want GB", got)
	}
}

func TestRequestHeaderMissingHeaderReturnsEmpty(t *testing.T) {
	src, _ := NewRequestHeaderSource(1, json.RawMessage(`{"header": "CF-IPCountry"}`))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	got, _ := src.Resolve(context.Background(), "192.0.2.1", r)
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestRequestHeaderXXIsTreatedAsEmpty(t *testing.T) {
	// Cloudflare uses "XX" for unknown country.
	src, _ := NewRequestHeaderSource(1, json.RawMessage(`{"header": "CF-IPCountry"}`))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("CF-IPCountry", "XX")
	got, _ := src.Resolve(context.Background(), "192.0.2.1", r)
	if got != "" {
		t.Fatalf("got %q, want empty for XX", got)
	}
}
EOF
go test ./internal/speedtest/geoip/... 2>&1 | tail -5
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/speedtest/geoip/request_header.go <<'EOF'
package geoip

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type RequestHeaderConfig struct {
	Header string `json:"header"`
}

type RequestHeaderSource struct {
	id  int64
	cfg RequestHeaderConfig
}

func NewRequestHeaderSource(id int64, rawCfg json.RawMessage) (*RequestHeaderSource, error) {
	var cfg RequestHeaderConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	return &RequestHeaderSource{id: id, cfg: cfg}, nil
}

func (s *RequestHeaderSource) ID() int64    { return s.id }
func (s *RequestHeaderSource) Kind() string { return "request_header" }

func (s *RequestHeaderSource) Resolve(_ context.Context, _ string, r *http.Request) (string, error) {
	if r == nil || s.cfg.Header == "" {
		return "", nil
	}
	v := strings.ToUpper(strings.TrimSpace(r.Header.Get(s.cfg.Header)))
	// CF and most CDNs use "XX" for unknown — treat as a miss.
	if v == "" || v == "XX" {
		return "", nil
	}
	return v, nil
}
EOF
go test ./internal/speedtest/geoip/... -v   # 6 tests now (3 chain + 3 header)
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/geoip/request_header.go internal/speedtest/geoip/request_header_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(geoip): request_header source kind"
```

---

### Task B5: `http_api` source kind + 30-day cache (TDD)

**Files:**
- Create: `internal/speedtest/geoip/http_api.go`
- Create: `internal/speedtest/geoip/http_api_test.go`

- [ ] **Step 1: Failing tests**

```bash
cat > internal/speedtest/geoip/http_api_test.go <<'EOF'
package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPAPITextFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "192.0.2.1") {
			t.Fatalf("path missing ip: %s", r.URL.Path)
		}
		fmt.Fprintln(w, "gb")
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`{"url_pattern": %q, "format": "text"}`, srv.URL+"/{ip}/country/")
	src, err := NewHTTPAPISource(1, json.RawMessage(cfg), nil)
	if err != nil { t.Fatal(err) }
	got, err := src.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil { t.Fatal(err) }
	if got != "GB" {
		t.Fatalf("got %q, want GB", got)
	}
}

func TestHTTPAPIJSONFormatWithPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"country_code": "de", "city": "Berlin"}`)
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`{"url_pattern": %q, "format": "json", "json_path": "country_code"}`, srv.URL+"/{ip}")
	src, err := NewHTTPAPISource(1, json.RawMessage(cfg), nil)
	if err != nil { t.Fatal(err) }
	got, _ := src.Resolve(context.Background(), "203.0.113.7", nil)
	if got != "DE" {
		t.Fatalf("got %q, want DE", got)
	}
}

func TestHTTPAPICacheHits(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		fmt.Fprintln(w, "fr")
	}))
	defer srv.Close()

	cache := newCountryCache()
	cfg := fmt.Sprintf(`{"url_pattern": %q, "format": "text"}`, srv.URL+"/{ip}")
	src, _ := NewHTTPAPISource(1, json.RawMessage(cfg), cache)
	src.Resolve(context.Background(), "203.0.113.8", nil)
	src.Resolve(context.Background(), "203.0.113.8", nil)
	src.Resolve(context.Background(), "203.0.113.8", nil)
	if calls != 1 {
		t.Fatalf("upstream called %d times, want 1 (cache hit)", calls)
	}
}

func TestHTTPAPICacheExpiresAfter30Days(t *testing.T) {
	cache := newCountryCache()
	cache.set("203.0.113.9", "ES", time.Now().Add(-31*24*time.Hour))
	if v := cache.get("203.0.113.9"); v != "" {
		t.Fatalf("expired entry returned %q, want empty", v)
	}
}
EOF
go test ./internal/speedtest/geoip/... 2>&1 | tail -5
```

- [ ] **Step 2: Implementation — `http_api.go`**

```bash
cat > internal/speedtest/geoip/http_api.go <<'EOF'
package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type HTTPAPIConfig struct {
	URLPattern string `json:"url_pattern"`        // contains {ip}
	Format     string `json:"format"`              // "text" | "json"
	JSONPath   string `json:"json_path,omitempty"` // dot-path for json format
}

type HTTPAPISource struct {
	id    int64
	cfg   HTTPAPIConfig
	cache *countryCache
	httpc *http.Client
}

// NewHTTPAPISource. cache may be nil — a new one is created if so.
func NewHTTPAPISource(id int64, rawCfg json.RawMessage, cache *countryCache) (*HTTPAPISource, error) {
	var cfg HTTPAPIConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	if cache == nil {
		cache = newCountryCache()
	}
	return &HTTPAPISource{
		id:    id,
		cfg:   cfg,
		cache: cache,
		httpc: &http.Client{Timeout: 2 * time.Second},
	}, nil
}

func (s *HTTPAPISource) ID() int64    { return s.id }
func (s *HTTPAPISource) Kind() string { return "http_api" }

func (s *HTTPAPISource) Resolve(ctx context.Context, ip string, _ *http.Request) (string, error) {
	if s.cfg.URLPattern == "" || ip == "" {
		return "", nil
	}
	if cached := s.cache.get(ip); cached != "" {
		return cached, nil
	}
	url := strings.ReplaceAll(s.cfg.URLPattern, "{ip}", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http_api %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	country, err := s.extract(body)
	if err != nil || country == "" {
		return country, err
	}
	country = strings.ToUpper(strings.TrimSpace(country))
	if country == "XX" {
		country = ""
	}
	if country != "" {
		s.cache.set(ip, country, time.Now())
	}
	return country, nil
}

func (s *HTTPAPISource) extract(body []byte) (string, error) {
	if s.cfg.Format == "json" {
		var any any
		if err := json.Unmarshal(body, &any); err != nil {
			return "", err
		}
		return jsonPath(any, s.cfg.JSONPath), nil
	}
	return string(body), nil
}

// jsonPath walks a dot-separated path through a generic JSON tree.
// Returns "" for any miss.
func jsonPath(node any, path string) string {
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		m, ok := node.(map[string]any)
		if !ok {
			return ""
		}
		node = m[part]
	}
	if s, ok := node.(string); ok {
		return s
	}
	return ""
}

// countryCache: per-IP cache with 30-day TTL. Lost on restart;
// re-warms quickly. Mutex-protected so multiple sources sharing the
// cache (rare but allowed) don't race.
type countryCache struct {
	mu  sync.RWMutex
	m   map[string]cacheEntry
}

type cacheEntry struct {
	country string
	setAt   time.Time
}

const cacheTTL = 30 * 24 * time.Hour

func newCountryCache() *countryCache {
	return &countryCache{m: map[string]cacheEntry{}}
}

func (c *countryCache) get(ip string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[ip]
	if !ok || time.Since(e.setAt) > cacheTTL {
		return ""
	}
	return e.country
}

func (c *countryCache) set(ip, country string, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[ip] = cacheEntry{country: country, setAt: at}
}
EOF
go test ./internal/speedtest/geoip/... -v   # 10 tests total now
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/geoip/http_api.go internal/speedtest/geoip/http_api_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(geoip): http_api source + 30-day per-IP cache"
```

---

### Task B6: `mmdb_auto` downloader + source

**Files:**
- Create: `internal/speedtest/geoip/downloader.go`
- Create: `internal/speedtest/geoip/mmdb_auto.go`

- [ ] **Step 1: `downloader.go`**

```bash
cat > internal/speedtest/geoip/downloader.go <<'EOF'
package geoip

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// downloadMMDB fetches the .mmdb (or .mmdb.gz) at url, validates by
// opening it with geoip2, and atomically renames into dest. Falls
// back to the previous UTC month's URL once if the primary 404s
// (db-ip.com publishes around the 1st of the month).
func downloadMMDB(ctx context.Context, urlPattern, dest string) error {
	now := time.Now().UTC()
	primary := strings.ReplaceAll(urlPattern, "{YYYY-MM}", now.Format("2006-01"))
	prev := strings.ReplaceAll(urlPattern, "{YYYY-MM}", now.AddDate(0, -1, 0).Format("2006-01"))

	if err := tryDownload(ctx, primary, dest); err == nil {
		return nil
	} else {
		// Fall back once to last month.
		if err2 := tryDownload(ctx, prev, dest); err2 == nil {
			return nil
		} else {
			return fmt.Errorf("download mmdb: primary %v; fallback %v", err, err2)
		}
	}
}

func tryDownload(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	c := &http.Client{Timeout: 60 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	var reader io.Reader = io.LimitReader(resp.Body, 50<<20) // 50 MB cap
	if strings.HasSuffix(url, ".gz") {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			out.Close()
			os.Remove(tmp)
			return fmt.Errorf("gunzip: %w", err)
		}
		defer gz.Close()
		reader = gz
	}
	if _, err := io.Copy(out, reader); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()

	// Validate by opening.
	r, err := geoip2.Open(tmp)
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("validate: %w", err)
	}
	r.Close()

	return os.Rename(tmp, dest)
}
EOF
```

- [ ] **Step 2: `mmdb_auto.go`**

```bash
cat > internal/speedtest/geoip/mmdb_auto.go <<'EOF'
package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

type MMDBAutoConfig struct {
	URLPattern   string `json:"url_pattern"` // contains {YYYY-MM}
	RefreshDays  int    `json:"refresh_days,omitempty"`
}

// MMDBAutoSource wraps mmdbReader plus a background refresh
// lifecycle. The downloader is invoked from main.go (or admin
// refresh trigger) rather than per-Resolve so resolution stays
// fast and lock-free in the steady state.
type MMDBAutoSource struct {
	id       int64
	cfg      MMDBAutoConfig
	cacheDir string
	reader   *mmdbReader

	mu              sync.Mutex
	lastRefreshedAt time.Time
}

func NewMMDBAutoSource(id int64, rawCfg json.RawMessage, cacheDir string) (*MMDBAutoSource, error) {
	var cfg MMDBAutoConfig
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, err
	}
	if cfg.RefreshDays <= 0 {
		cfg.RefreshDays = 25
	}
	return &MMDBAutoSource{
		id:       id,
		cfg:      cfg,
		cacheDir: cacheDir,
		reader:   newMMDBReader(),
	}, nil
}

func (m *MMDBAutoSource) ID() int64    { return m.id }
func (m *MMDBAutoSource) Kind() string { return "mmdb_auto" }

func (m *MMDBAutoSource) Resolve(ctx context.Context, ip string, _ *http.Request) (string, error) {
	return m.reader.Country(ctx, ip)
}

// LocalPath returns the cache-dir path where the downloader writes
// this source's .mmdb. Stable across restarts.
func (m *MMDBAutoSource) LocalPath() string {
	return filepath.Join(m.cacheDir, fmt.Sprintf("%d.mmdb", m.id))
}

// NeedsRefresh: true if we've never loaded OR last refresh is older
// than RefreshDays. Caller decides whether to fire the download.
func (m *MMDBAutoSource) NeedsRefresh() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastRefreshedAt.IsZero() {
		return true
	}
	return time.Since(m.lastRefreshedAt) > time.Duration(m.cfg.RefreshDays)*24*time.Hour
}

// Refresh downloads the mmdb and opens the reader. Safe to call from
// a goroutine; uses an internal lock.
func (m *MMDBAutoSource) Refresh(ctx context.Context) error {
	if err := downloadMMDB(ctx, m.cfg.URLPattern, m.LocalPath()); err != nil {
		return err
	}
	if err := m.reader.Open(m.LocalPath()); err != nil {
		return err
	}
	m.mu.Lock()
	m.lastRefreshedAt = time.Now()
	m.mu.Unlock()
	return nil
}

// LoadCached opens whatever file is already at LocalPath without
// downloading. Used on plugin start to make resolution work
// immediately if a previous run already downloaded the file.
func (m *MMDBAutoSource) LoadCached() error {
	if err := m.reader.Open(m.LocalPath()); err != nil {
		return err
	}
	// Don't update lastRefreshedAt — caller seeded it from the DB row.
	return nil
}
EOF
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/geoip/downloader.go internal/speedtest/geoip/mmdb_auto.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(geoip): mmdb_auto source + atomic-swap downloader"
```

---

### Task B7: Source factory — build source from a DB row

**Files:**
- Create: `internal/speedtest/geoip/factory.go`

- [ ] **Step 1: Implementation**

```bash
cat > internal/speedtest/geoip/factory.go <<'EOF'
package geoip

import (
	"fmt"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// BuildSource constructs a concrete Source from a store row.
// Returns an error for unknown kinds or invalid config JSON.
// `cacheDir` is used only by mmdb_auto.
func BuildSource(row store.STGeoIPSource, cacheDir string) (Source, error) {
	switch row.Kind {
	case "mmdb_auto":
		return NewMMDBAutoSource(row.ID, row.Config, cacheDir)
	case "mmdb_file":
		return NewMMDBFileSource(row.ID, row.Config)
	case "http_api":
		return NewHTTPAPISource(row.ID, row.Config, nil)
	case "request_header":
		return NewRequestHeaderSource(row.ID, row.Config)
	default:
		return nil, fmt.Errorf("unknown geoip source kind: %q", row.Kind)
	}
}
EOF
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/geoip/factory.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(geoip): source factory keyed by store row kind"
```

---

## Phase C — Store CRUD

### Task C1: `st_endpoints.go` — CRUD

**Files:**
- Create: `internal/store/st_endpoints.go`

- [ ] **Step 1: Write the file**

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > internal/store/st_endpoints.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// STListEndpoints returns every endpoint, ordered by sort_order then
// label. activeOnly skips soft-deleted rows.
func (s *Store) STListEndpoints(ctx context.Context, activeOnly bool) ([]STEndpoint, error) {
	q := `SELECT id, label, url, country, region, sort_order, active, created_at, updated_at
	      FROM st_endpoints`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(label)`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list st_endpoints: %w", err)
	}
	defer rows.Close()
	out := []STEndpoint{}
	for rows.Next() {
		var e STEndpoint
		if err := rows.Scan(&e.ID, &e.Label, &e.URL, &e.Country, &e.Region,
			&e.SortOrder, &e.Active, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan st_endpoint: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) STGetEndpoint(ctx context.Context, id int64) (STEndpoint, error) {
	var e STEndpoint
	err := s.pool.QueryRow(ctx, `
		SELECT id, label, url, country, region, sort_order, active, created_at, updated_at
		FROM st_endpoints WHERE id = $1`, id).
		Scan(&e.ID, &e.Label, &e.URL, &e.Country, &e.Region,
			&e.SortOrder, &e.Active, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STEndpoint{}, ErrNotFound
	}
	if err != nil {
		return STEndpoint{}, fmt.Errorf("get st_endpoint: %w", err)
	}
	return e, nil
}

func (s *Store) STCreateEndpoint(ctx context.Context, in STEndpoint) (STEndpoint, error) {
	var out STEndpoint
	err := s.pool.QueryRow(ctx, `
		INSERT INTO st_endpoints (label, url, country, region, sort_order, active)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, label, url, country, region, sort_order, active, created_at, updated_at`,
		in.Label, in.URL, in.Country, in.Region, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.URL, &out.Country, &out.Region,
			&out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return STEndpoint{}, fmt.Errorf("insert st_endpoint: %w", err)
	}
	return out, nil
}

func (s *Store) STUpdateEndpoint(ctx context.Context, in STEndpoint) (STEndpoint, error) {
	var out STEndpoint
	err := s.pool.QueryRow(ctx, `
		UPDATE st_endpoints SET
		  label = $2, url = $3, country = $4, region = $5,
		  sort_order = $6, active = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING id, label, url, country, region, sort_order, active, created_at, updated_at`,
		in.ID, in.Label, in.URL, in.Country, in.Region, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.URL, &out.Country, &out.Region,
			&out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STEndpoint{}, ErrNotFound
	}
	if err != nil {
		return STEndpoint{}, fmt.Errorf("update st_endpoint: %w", err)
	}
	return out, nil
}

func (s *Store) STDeleteEndpoint(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM st_endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete st_endpoint: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
EOF
go build ./internal/store/...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/st_endpoints.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): st_endpoints CRUD"
```

---

### Task C2: `st_geoip_sources.go` — CRUD + status updates

**Files:**
- Create: `internal/store/st_geoip_sources.go`

```bash
cat > internal/store/st_geoip_sources.go <<'EOF'
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (s *Store) STListGeoIPSources(ctx context.Context, activeOnly bool) ([]STGeoIPSource, error) {
	q := `SELECT id, label, kind, config, sort_order, active,
	             last_status, last_used_at, last_refreshed_at,
	             created_at, updated_at
	      FROM st_geoip_sources`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, id`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list st_geoip_sources: %w", err)
	}
	defer rows.Close()
	out := []STGeoIPSource{}
	for rows.Next() {
		var src STGeoIPSource
		var cfg []byte
		if err := rows.Scan(&src.ID, &src.Label, &src.Kind, &cfg,
			&src.SortOrder, &src.Active, &src.LastStatus,
			&src.LastUsedAt, &src.LastRefreshedAt,
			&src.CreatedAt, &src.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan st_geoip_source: %w", err)
		}
		src.Config = json.RawMessage(cfg)
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) STGetGeoIPSource(ctx context.Context, id int64) (STGeoIPSource, error) {
	var src STGeoIPSource
	var cfg []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, label, kind, config, sort_order, active,
		       last_status, last_used_at, last_refreshed_at,
		       created_at, updated_at
		FROM st_geoip_sources WHERE id = $1`, id).
		Scan(&src.ID, &src.Label, &src.Kind, &cfg,
			&src.SortOrder, &src.Active, &src.LastStatus,
			&src.LastUsedAt, &src.LastRefreshedAt,
			&src.CreatedAt, &src.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STGeoIPSource{}, ErrNotFound
	}
	if err != nil {
		return STGeoIPSource{}, fmt.Errorf("get st_geoip_source: %w", err)
	}
	src.Config = json.RawMessage(cfg)
	return src, nil
}

func (s *Store) STCreateGeoIPSource(ctx context.Context, in STGeoIPSource) (STGeoIPSource, error) {
	var out STGeoIPSource
	var cfg []byte
	err := s.pool.QueryRow(ctx, `
		INSERT INTO st_geoip_sources (label, kind, config, sort_order, active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, label, kind, config, sort_order, active,
		          last_status, last_used_at, last_refreshed_at,
		          created_at, updated_at`,
		in.Label, in.Kind, []byte(in.Config), in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.Kind, &cfg,
			&out.SortOrder, &out.Active, &out.LastStatus,
			&out.LastUsedAt, &out.LastRefreshedAt,
			&out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return STGeoIPSource{}, fmt.Errorf("insert st_geoip_source: %w", err)
	}
	out.Config = json.RawMessage(cfg)
	return out, nil
}

func (s *Store) STUpdateGeoIPSource(ctx context.Context, in STGeoIPSource) (STGeoIPSource, error) {
	var out STGeoIPSource
	var cfg []byte
	err := s.pool.QueryRow(ctx, `
		UPDATE st_geoip_sources SET
		  label = $2, config = $3, sort_order = $4, active = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING id, label, kind, config, sort_order, active,
		          last_status, last_used_at, last_refreshed_at,
		          created_at, updated_at`,
		in.ID, in.Label, []byte(in.Config), in.SortOrder, in.Active).
		Scan(&out.ID, &out.Label, &out.Kind, &cfg,
			&out.SortOrder, &out.Active, &out.LastStatus,
			&out.LastUsedAt, &out.LastRefreshedAt,
			&out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return STGeoIPSource{}, ErrNotFound
	}
	if err != nil {
		return STGeoIPSource{}, fmt.Errorf("update st_geoip_source: %w", err)
	}
	out.Config = json.RawMessage(cfg)
	return out, nil
}

func (s *Store) STDeleteGeoIPSource(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM st_geoip_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete st_geoip_source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// STMarkGeoIPSourceUsed updates last_used_at + last_status = 'ok'.
// Called from the chain walker's StatusSink implementation.
func (s *Store) STMarkGeoIPSourceUsed(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE st_geoip_sources SET last_used_at = NOW(), last_status = 'ok' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark geoip source used: %w", err)
	}
	return nil
}

// STMarkGeoIPSourceStatus updates last_status (typically 'error: ...').
func (s *Store) STMarkGeoIPSourceStatus(ctx context.Context, id int64, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE st_geoip_sources SET last_status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("mark geoip source status: %w", err)
	}
	return nil
}

// STMarkGeoIPSourceRefreshed updates last_refreshed_at + last_status = 'ok'.
func (s *Store) STMarkGeoIPSourceRefreshed(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE st_geoip_sources SET last_refreshed_at = NOW(), last_status = 'ok' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark geoip source refreshed: %w", err)
	}
	return nil
}
EOF
go build ./internal/store/...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/st_geoip_sources.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): st_geoip_sources CRUD + status helpers"
```

---

### Task C3: `st_results.go` — insert + history + aggregates + rate limit

**Files:**
- Create: `internal/store/st_results.go`

```bash
cat > internal/store/st_results.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// STInsertResult writes a row and returns it.
func (s *Store) STInsertResult(ctx context.Context, in STResult) (STResult, error) {
	var out STResult
	var ipBytes any
	if in.ClientIP != "" {
		ipBytes = in.ClientIP
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO st_results (
		  customer_id, endpoint_id, endpoint_label, auto_strategy,
		  download_mbps, upload_mbps, ping_ms, jitter_ms,
		  client_ip, user_agent
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::inet,$10)
		RETURNING id, customer_id, endpoint_id, endpoint_label, auto_strategy,
		          download_mbps, upload_mbps, ping_ms, jitter_ms,
		          host(client_ip), user_agent, ran_at`,
		in.CustomerID, in.EndpointID, in.EndpointLabel, in.AutoStrategy,
		in.DownloadMbps, in.UploadMbps, in.PingMs, in.JitterMs,
		ipBytes, in.UserAgent).
		Scan(&out.ID, &out.CustomerID, &out.EndpointID, &out.EndpointLabel, &out.AutoStrategy,
			&out.DownloadMbps, &out.UploadMbps, &out.PingMs, &out.JitterMs,
			&out.ClientIP, &out.UserAgent, &out.RanAt)
	if err != nil {
		return STResult{}, fmt.Errorf("insert st_result: %w", err)
	}
	return out, nil
}

// STCustomerHistory returns the calling customer's last N results.
func (s *Store) STCustomerHistory(ctx context.Context, customerID string, limit int) ([]STResult, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.stListResults(ctx, `WHERE customer_id = $1`, []any{customerID}, "ran_at DESC", limit, 0)
}

// STLastResultAt returns when this customer last ran a test, or
// zero time if never. Used by the 60s rate-limit check.
func (s *Store) STLastResultAt(ctx context.Context, customerID string) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT ran_at FROM st_results WHERE customer_id = $1 ORDER BY ran_at DESC LIMIT 1`,
		customerID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("last st_result at: %w", err)
	}
	return t, nil
}

// STListResults serves the admin filtered listing.
func (s *Store) STListResults(ctx context.Context, f STResultFilter) ([]STResult, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	args := []any{}
	clauses := []string{}
	if f.CustomerID != "" {
		args = append(args, f.CustomerID)
		clauses = append(clauses, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.EndpointID > 0 {
		args = append(args, f.EndpointID)
		clauses = append(clauses, fmt.Sprintf("endpoint_id = $%d", len(args)))
	}
	if f.AutoStrategy != "" {
		args = append(args, f.AutoStrategy)
		clauses = append(clauses, fmt.Sprintf("auto_strategy = $%d", len(args)))
	}
	if !f.Since.IsZero() {
		args = append(args, f.Since)
		clauses = append(clauses, fmt.Sprintf("ran_at >= $%d", len(args)))
	}
	if f.SlowOnly && f.SlowThresh > 0 {
		args = append(args, f.SlowThresh)
		clauses = append(clauses, fmt.Sprintf("download_mbps < $%d", len(args)))
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	return s.stListResults(ctx, where, args, "ran_at DESC", f.Limit, f.Offset)
}

func (s *Store) stListResults(ctx context.Context, where string, args []any, orderBy string, limit, offset int) ([]STResult, error) {
	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, customer_id, endpoint_id, endpoint_label, auto_strategy,
		       download_mbps, upload_mbps, ping_ms, jitter_ms,
		       COALESCE(host(client_ip), ''), user_agent, ran_at
		FROM st_results
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, where, orderBy, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list st_results: %w", err)
	}
	defer rows.Close()
	out := []STResult{}
	for rows.Next() {
		var r STResult
		if err := rows.Scan(&r.ID, &r.CustomerID, &r.EndpointID, &r.EndpointLabel, &r.AutoStrategy,
			&r.DownloadMbps, &r.UploadMbps, &r.PingMs, &r.JitterMs,
			&r.ClientIP, &r.UserAgent, &r.RanAt); err != nil {
			return nil, fmt.Errorf("scan st_result: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// STDashboardAggregatesData returns the four aggregate slices the
// admin dashboard page renders. Window is fixed at 30 days.
func (s *Store) STDashboardAggregatesData(ctx context.Context, slowThresh float64) (STDashboardAggregates, error) {
	out := STDashboardAggregates{
		PerEndpoint: []STEndpointAggregate{},
		PerDay:      []STDailyCount{},
		SlowTop10:   []STResult{},
		CountryHits: []STCountryCount{},
	}

	// Per-endpoint medians.
	rows, err := s.pool.Query(ctx, `
		SELECT endpoint_id, endpoint_label,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY download_mbps)::float8,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY upload_mbps)::float8,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY ping_ms)::float8,
		       COUNT(*)
		FROM st_results
		WHERE ran_at > NOW() - INTERVAL '30 days'
		GROUP BY endpoint_id, endpoint_label
		ORDER BY COUNT(*) DESC`)
	if err != nil {
		return out, fmt.Errorf("per-endpoint medians: %w", err)
	}
	for rows.Next() {
		var a STEndpointAggregate
		if err := rows.Scan(&a.EndpointID, &a.Label, &a.MedianDownload, &a.MedianUpload, &a.MedianPing, &a.ResultCount); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan per-endpoint: %w", err)
		}
		out.PerEndpoint = append(out.PerEndpoint, a)
	}
	rows.Close()

	// Per-day counts.
	rows, err = s.pool.Query(ctx, `
		SELECT to_char(date_trunc('day', ran_at), 'YYYY-MM-DD'), COUNT(*)
		FROM st_results
		WHERE ran_at > NOW() - INTERVAL '30 days'
		GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return out, fmt.Errorf("per-day counts: %w", err)
	}
	for rows.Next() {
		var d STDailyCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan per-day: %w", err)
		}
		out.PerDay = append(out.PerDay, d)
	}
	rows.Close()

	// Slow top 10 (last 7 days, median download below threshold per customer).
	if slowThresh > 0 {
		results, err := s.stListResults(ctx,
			`WHERE ran_at > NOW() - INTERVAL '7 days' AND download_mbps < $1`,
			[]any{slowThresh},
			`download_mbps ASC`, 10, 0)
		if err != nil {
			return out, fmt.Errorf("slow top-10: %w", err)
		}
		out.SlowTop10 = results
	}

	// Country hits — only populated when geoip resolution captured a country.
	// We don't store country on the row in v1 (geoip happens at request time,
	// not save time). For the dashboard, the count is just zero. A follow-up
	// could add a per-result resolved-country column.
	// (Empty CountryHits is fine; the SPA renders "no data" gracefully.)

	return out, nil
}
EOF
go build ./internal/store/...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/st_results.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): st_results insert + history + aggregates + rate-limit helper"
```

---

## Phase D — Auto resolver

### Task D1: `auto.go` resolver (TDD)

**Files:**
- Create: `internal/speedtest/auto.go`
- Create: `internal/speedtest/auto_test.go`

The resolver consults the GeoIP chain or returns the latency-mode candidate list. Pure function over a fake store so the test is self-contained.

- [ ] **Step 1: Failing tests**

```bash
cat > internal/speedtest/auto_test.go <<'EOF'
package speedtest

import (
	"context"
	"net/http"
	"testing"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

type fakeEPStore struct{ endpoints []store.STEndpoint }

func (f *fakeEPStore) STListEndpoints(_ context.Context, _ bool) ([]store.STEndpoint, error) {
	return f.endpoints, nil
}

type fakeGeoIP struct {
	country  string
	srcID    int64
}

func (f *fakeGeoIP) Resolve(_ context.Context, _ string, _ *http.Request) (string, int64, error) {
	return f.country, f.srcID, nil
}

func endpoints() []store.STEndpoint {
	return []store.STEndpoint{
		{ID: 1, Label: "London",    URL: "https://lon/", Country: "GB", Active: true, SortOrder: 0},
		{ID: 2, Label: "Frankfurt", URL: "https://fra/", Country: "DE", Active: true, SortOrder: 1},
		{ID: 3, Label: "Disabled",  URL: "https://x/",   Country: "FR", Active: false, SortOrder: 2},
	}
}

func TestResolveGeoIPPicksMatchingCountry(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{country: "DE", srcID: 7}, "geoip")
	out, err := r.Resolve(context.Background(), "192.0.2.1", nil)
	if err != nil { t.Fatal(err) }
	if out.Strategy != "geoip" || out.Endpoint == nil || out.Endpoint.ID != 2 {
		t.Fatalf("got %+v, want geoip strategy + endpoint 2", out)
	}
	if out.GeoIP.Country != "DE" || out.GeoIP.SourceID != 7 {
		t.Fatalf("got GeoIP %+v, want {DE, 7}", out.GeoIP)
	}
}

func TestResolveGeoIPNoMatchFallsThroughToFirstActive(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{country: "JP"}, "geoip")
	out, _ := r.Resolve(context.Background(), "192.0.2.1", nil)
	if out.Strategy != "fallback" || out.Endpoint == nil || out.Endpoint.ID != 1 {
		t.Fatalf("got %+v, want fallback strategy + endpoint 1", out)
	}
}

func TestResolveLatencyReturnsActiveCandidates(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{}, "latency")
	out, _ := r.Resolve(context.Background(), "192.0.2.1", nil)
	if out.Strategy != "latency" {
		t.Fatalf("got strategy %q, want latency", out.Strategy)
	}
	if len(out.Candidates) != 2 || out.Endpoint != nil {
		t.Fatalf("got candidates %v + endpoint %v; want 2 candidates, nil endpoint", out.Candidates, out.Endpoint)
	}
}

func TestResolveGeoIPEmptyResolverFallsThrough(t *testing.T) {
	r := NewResolver(&fakeEPStore{endpoints: endpoints()}, &fakeGeoIP{country: ""}, "geoip")
	out, _ := r.Resolve(context.Background(), "192.0.2.1", nil)
	if out.Strategy != "fallback" || out.Endpoint == nil || out.Endpoint.ID != 1 {
		t.Fatalf("got %+v, want fallback to first active", out)
	}
}
EOF
go test ./internal/speedtest/... 2>&1 | tail -5   # expect undefined Resolver
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/speedtest/auto.go <<'EOF'
package speedtest

import (
	"context"
	"net/http"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// EndpointLister is the slice of Store the resolver needs.
type EndpointLister interface {
	STListEndpoints(ctx context.Context, activeOnly bool) ([]store.STEndpoint, error)
}

// GeoIPResolver mirrors the geoip.Chain.Resolve signature without
// importing the geoip package (keeps the resolver free of that
// dependency for testability).
type GeoIPResolver interface {
	Resolve(ctx context.Context, ip string, r *http.Request) (country string, sourceID int64, err error)
}

// AutoResolution is what the /api/customer/speedtest/auto handler
// returns to the SPA.
type AutoResolution struct {
	Strategy   string             `json:"strategy"` // "latency" | "geoip" | "fallback"
	Endpoint   *store.STEndpoint  `json:"endpoint,omitempty"`
	Candidates []store.STEndpoint `json:"candidates,omitempty"`
	GeoIP      AutoGeoIPHint      `json:"geoip"`
}

type AutoGeoIPHint struct {
	Country     string `json:"country,omitempty"`
	SourceID    int64  `json:"sourceId,omitempty"`
	SourceLabel string `json:"sourceLabel,omitempty"`
}

type Resolver struct {
	store    EndpointLister
	geoip    GeoIPResolver
	strategy string // "latency" | "geoip"
}

func NewResolver(store EndpointLister, geoip GeoIPResolver, strategy string) *Resolver {
	if strategy != "latency" && strategy != "geoip" {
		strategy = "latency"
	}
	return &Resolver{store: store, geoip: geoip, strategy: strategy}
}

// Resolve picks the endpoint (or returns the latency candidate list)
// per the configured strategy. Falls through to "fallback" if the
// configured strategy can't produce a usable answer.
func (r *Resolver) Resolve(ctx context.Context, clientIP string, req *http.Request) (AutoResolution, error) {
	all, err := r.store.STListEndpoints(ctx, true)
	if err != nil {
		return AutoResolution{}, err
	}

	if r.strategy == "geoip" && r.geoip != nil {
		country, srcID, _ := r.geoip.Resolve(ctx, clientIP, req)
		if country != "" {
			for _, ep := range all {
				if ep.Country == country {
					ep := ep
					return AutoResolution{
						Strategy: "geoip",
						Endpoint: &ep,
						GeoIP:    AutoGeoIPHint{Country: country, SourceID: srcID},
					}, nil
				}
			}
		}
		// Country resolved or didn't, but no match — fall through.
	}

	if r.strategy == "latency" {
		return AutoResolution{
			Strategy:   "latency",
			Candidates: all,
		}, nil
	}

	// Last-resort fallback: first active endpoint.
	if len(all) > 0 {
		ep := all[0]
		return AutoResolution{Strategy: "fallback", Endpoint: &ep}, nil
	}
	return AutoResolution{Strategy: "fallback"}, nil
}
EOF
go test ./internal/speedtest/... -v   # 9 tests now (5 truncate + 4 resolver)
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/speedtest/auto.go internal/speedtest/auto_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(speedtest): auto-strategy resolver (latency vs geoip)"
```

---

## Phase E — Customer handlers

### Task E1: `handlers_st_customer.go`

**Files:**
- Create: `internal/server/handlers_st_customer.go`

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > internal/server/handlers_st_customer.go <<'EOF'
package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ContinuumApp/continuum-plugin-support/internal/speedtest"
	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// stCustomerStore unwraps Deps.ConfigStore into the concrete *store.Store
// (same pattern KB uses).
func stCustomerStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

func hSTSpeedtestPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode:    "speedtest",
			Theme:   adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hSTCustomerEndpoints(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eps, err := stCustomerStore(d).STListEndpoints(r.Context(), true)
		if err != nil {
			writeInternal(w, r, d, "st_endpoints_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, eps)
	}
}

func hSTCustomerAuto(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.STAutoResolver == nil {
			writeErr(w, http.StatusServiceUnavailable, "st_unconfigured", "speedtest resolver not configured")
			return
		}
		ip := clientIP(r)
		out, err := d.STAutoResolver.Resolve(r.Context(), ip, r)
		if err != nil {
			writeInternal(w, r, d, "st_auto_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type stResultRequest struct {
	EndpointID    int64   `json:"endpointId,omitempty"`
	EndpointLabel string  `json:"endpointLabel"`
	AutoStrategy  string  `json:"autoStrategy,omitempty"`
	DownloadMbps  float64 `json:"downloadMbps"`
	UploadMbps    float64 `json:"uploadMbps"`
	PingMs        float64 `json:"pingMs"`
	JitterMs      float64 `json:"jitterMs"`
}

func hSTCustomerSaveResult(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerID := r.Header.Get("X-Continuum-User-Id")

		// 60s per-customer rate limit.
		last, err := stCustomerStore(d).STLastResultAt(r.Context(), customerID)
		if err != nil {
			writeInternal(w, r, d, "st_rate_check_failed", err)
			return
		}
		if !last.IsZero() && time.Since(last) < 60*time.Second {
			retryIn := 60 - int(time.Since(last).Seconds())
			writeErr(w, http.StatusTooManyRequests, "st_rate_limited",
				"please wait "+ts(retryIn)+" before running another test")
			return
		}

		var req stResultRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}

		ip := clientIP(r)
		truncated := speedtest.TruncateIP(ip, d.STClientIPStorage)

		var epID *int64
		if req.EndpointID > 0 {
			id := req.EndpointID
			epID = &id
		}

		saved, err := stCustomerStore(d).STInsertResult(r.Context(), store.STResult{
			CustomerID:    customerID,
			EndpointID:    epID,
			EndpointLabel: req.EndpointLabel,
			AutoStrategy:  req.AutoStrategy,
			DownloadMbps:  req.DownloadMbps,
			UploadMbps:    req.UploadMbps,
			PingMs:        req.PingMs,
			JitterMs:      req.JitterMs,
			ClientIP:      truncated,
			UserAgent:     r.UserAgent(),
		})
		if err != nil {
			writeInternal(w, r, d, "st_save_failed", err)
			return
		}

		// Emit events.
		stPublishEvent(d, "speedtest_run", saved, nil)
		if d.STSlowThresholdMbps > 0 && saved.DownloadMbps < d.STSlowThresholdMbps {
			stPublishEvent(d, "speedtest_slow", saved, map[string]any{
				"threshold_mbps": d.STSlowThresholdMbps,
				"slow_by_mbps":   d.STSlowThresholdMbps - saved.DownloadMbps,
			})
		}

		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTCustomerHistory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hist, err := stCustomerStore(d).STCustomerHistory(r.Context(),
			r.Header.Get("X-Continuum-User-Id"), 20)
		if err != nil {
			writeInternal(w, r, d, "st_history_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, hist)
	}
}

// clientIP returns the best-guess client IP, preferring X-Forwarded-For's
// first entry when present (Continuum runs behind a reverse proxy).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func ts(n int) string {
	if n <= 1 {
		return "1 second"
	}
	return itoa(n) + " seconds"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
EOF
```

The handler file references `Deps.STAutoResolver`, `Deps.STClientIPStorage`, `Deps.STSlowThresholdMbps` and the `stPublishEvent` helper — these are added in Task G2 (Deps extension) and Task E2 (event helper stub). For now the file won't compile until those land — that's fine, we land them next.

- [ ] **Verify the file is syntactically valid (other than the missing references):**

```bash
go vet ./internal/server/... 2>&1 | head -20
```

Don't commit yet — wait for Task E2.

---

### Task E2: `st_events.go` stub + commit E1+E2 together

**Files:**
- Create: `internal/server/st_events.go`
- Modify: `internal/server/server.go` (Deps extension)

```bash
cat > internal/server/st_events.go <<'EOF'
package server

import (
	"context"

	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

// stPublishEvent assembles the base speedtest payload + extra keys
// and hands off to Deps.EventPublisher. No-ops when EventPublisher
// is nil (test contexts). Best-effort.
func stPublishEvent(d Deps, name string, r store.STResult, extra map[string]any) {
	if d.EventPublisher == nil {
		return
	}
	payload := map[string]any{
		"customer_id":    r.CustomerID,
		"endpoint_id":    r.EndpointID,
		"endpoint_label": r.EndpointLabel,
		"download_mbps":  r.DownloadMbps,
		"upload_mbps":    r.UploadMbps,
		"ping_ms":        r.PingMs,
		"jitter_ms":      r.JitterMs,
		"auto_strategy":  r.AutoStrategy,
		"ran_at":         r.RanAt,
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := d.EventPublisher.PublishEvent(context.Background(),
		"plugin.continuum.support."+name, payload); err != nil && d.Logger != nil {
		d.Logger.Warn("speedtest event publish failed", "event", name, "err", err)
	}
}
EOF
```

Now extend `server.Deps`. Read `internal/server/server.go` first; find the `Deps` struct (added in KB Unit 12). Add four fields:

```go
type Deps struct {
    DatabaseURL         string
    Logger              hclog.Logger
    ConfigStore         ConfigStore
    EventPublisher      EventPublisher

    // Speedtest module wiring; nil-safe (resolver-nil → 503).
    STAutoResolver      STAutoResolver
    STClientIPStorage   string  // "truncated" (default) | "off"
    STSlowThresholdMbps float64
}

// STAutoResolver wraps the speedtest.Resolver without making the
// server package import the speedtest package directly. main.go
// constructs a *speedtest.Resolver and assigns it here.
type STAutoResolver interface {
    Resolve(ctx context.Context, ip string, r *http.Request) (any, error)
}
```

Wait — `Resolve` returns `speedtest.AutoResolution`, not `any`. Cleaner: keep the import. Use this instead (replacing the placeholder above):

```go
import (
    // ... existing imports
    "github.com/ContinuumApp/continuum-plugin-support/internal/speedtest"
)

type Deps struct {
    DatabaseURL         string
    Logger              hclog.Logger
    ConfigStore         ConfigStore
    EventPublisher      EventPublisher

    STAutoResolver      *speedtest.Resolver
    STClientIPStorage   string
    STSlowThresholdMbps float64
}
```

And update the handler check `if d.STAutoResolver == nil` accordingly. (`*speedtest.Resolver` compares to nil cleanly.)

- [ ] **Verify + commit E1 + E2 together**

```bash
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_st_customer.go internal/server/st_events.go internal/server/server.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): speedtest customer handlers + event helper + Deps wiring"
```

---

## Phase F — Admin handlers

### Task F1: `handlers_st_admin.go`

**Files:**
- Create: `internal/server/handlers_st_admin.go`

```bash
cat > internal/server/handlers_st_admin.go <<'EOF'
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ContinuumApp/continuum-plugin-support/internal/speedtest/geoip"
	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

func stAdminStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// Admin SPA shell handlers.
func hSTAdminEndpointsPage(d Deps) http.HandlerFunc  { return adminSPAHandler(d, "admin-st-endpoints") }
func hSTAdminGeoIPPage(d Deps) http.HandlerFunc      { return adminSPAHandler(d, "admin-st-geoip") }
func hSTAdminResultsPage(d Deps) http.HandlerFunc    { return adminSPAHandler(d, "admin-st-results") }
func hSTAdminDashboardsPage(d Deps) http.HandlerFunc { return adminSPAHandler(d, "admin-st-dashboards") }

// --- Endpoints CRUD --------------------------------------------------

type stEndpointRequest struct {
	Label     string `json:"label"`
	URL       string `json:"url"`
	Country   string `json:"country"`
	Region    string `json:"region"`
	SortOrder int    `json:"sortOrder"`
	Active    bool   `json:"active"`
}

func hSTAdminListEndpoints(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eps, err := stAdminStore(d).STListEndpoints(r.Context(), false)
		if err != nil {
			writeInternal(w, r, d, "st_endpoints_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, eps)
	}
}

func hSTAdminCreateEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req stEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if req.Label == "" || req.URL == "" {
			writeErr(w, http.StatusBadRequest, "bad_endpoint", "label and url are required")
			return
		}
		saved, err := stAdminStore(d).STCreateEndpoint(r.Context(), store.STEndpoint{
			Label: req.Label, URL: req.URL, Country: req.Country, Region: req.Region,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil {
			writeInternal(w, r, d, "st_endpoint_create_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminUpdateEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid endpoint id")
			return
		}
		var req stEndpointRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		saved, err := stAdminStore(d).STUpdateEndpoint(r.Context(), store.STEndpoint{
			ID: id, Label: req.Label, URL: req.URL, Country: req.Country, Region: req.Region,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "endpoint not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_endpoint_update_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminDeleteEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid endpoint id")
			return
		}
		if err := stAdminStore(d).STDeleteEndpoint(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "st_not_found", "endpoint not found")
				return
			}
			writeInternal(w, r, d, "st_endpoint_delete_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hSTAdminPingEndpoint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid endpoint id")
			return
		}
		ep, err := stAdminStore(d).STGetEndpoint(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "endpoint not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_endpoint_get_failed", err)
			return
		}
		// HEAD the LibreSpeed empty.php — fast, ~50 bytes.
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, ep.URL+"/empty.php", nil)
		client := &http.Client{Timeout: 5 * time.Second}
		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start).Milliseconds()
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "elapsed_ms": elapsed})
			return
		}
		resp.Body.Close()
		writeJSON(w, http.StatusOK, map[string]any{"ok": resp.StatusCode == http.StatusOK, "status": resp.StatusCode, "elapsed_ms": elapsed})
	}
}

// --- GeoIP sources CRUD ---------------------------------------------

type stGeoIPSourceRequest struct {
	Label     string          `json:"label"`
	Kind      string          `json:"kind"`
	Config    json.RawMessage `json:"config"`
	SortOrder int             `json:"sortOrder"`
	Active    bool            `json:"active"`
}

func hSTAdminListGeoIPSources(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		srcs, err := stAdminStore(d).STListGeoIPSources(r.Context(), false)
		if err != nil {
			writeInternal(w, r, d, "st_geoip_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, srcs)
	}
}

func hSTAdminCreateGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req stGeoIPSourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		validKinds := map[string]bool{"mmdb_auto": true, "mmdb_file": true, "http_api": true, "request_header": true}
		if !validKinds[req.Kind] {
			writeErr(w, http.StatusBadRequest, "bad_kind", "kind must be mmdb_auto / mmdb_file / http_api / request_header")
			return
		}
		if req.Label == "" {
			writeErr(w, http.StatusBadRequest, "bad_label", "label is required")
			return
		}
		if len(req.Config) == 0 {
			req.Config = json.RawMessage(`{}`)
		}
		saved, err := stAdminStore(d).STCreateGeoIPSource(r.Context(), store.STGeoIPSource{
			Label: req.Label, Kind: req.Kind, Config: req.Config,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil {
			writeInternal(w, r, d, "st_geoip_create_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminUpdateGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		var req stGeoIPSourceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		if len(req.Config) == 0 {
			req.Config = json.RawMessage(`{}`)
		}
		// Kind is immutable; ignore req.Kind on update.
		cur, err := stAdminStore(d).STGetGeoIPSource(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_geoip_get_failed", err)
			return
		}
		cur.Label = req.Label
		cur.Config = req.Config
		cur.SortOrder = req.SortOrder
		cur.Active = req.Active
		saved, err := stAdminStore(d).STUpdateGeoIPSource(r.Context(), cur)
		if err != nil {
			writeInternal(w, r, d, "st_geoip_update_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

func hSTAdminDeleteGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		if err := stAdminStore(d).STDeleteGeoIPSource(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
				return
			}
			writeInternal(w, r, d, "st_geoip_delete_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hSTAdminRefreshGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		row, err := stAdminStore(d).STGetGeoIPSource(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_geoip_get_failed", err)
			return
		}
		if row.Kind != "mmdb_auto" {
			writeErr(w, http.StatusBadRequest, "st_geoip_not_auto", "only mmdb_auto sources can be refreshed")
			return
		}
		cacheDir := geoipCacheDir(d)
		src, err := geoip.NewMMDBAutoSource(row.ID, row.Config, cacheDir)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "st_geoip_bad_config", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		if err := src.Refresh(ctx); err != nil {
			_ = stAdminStore(d).STMarkGeoIPSourceStatus(r.Context(), row.ID, "error: "+err.Error())
			writeInternal(w, r, d, "st_geoip_refresh_failed", err)
			return
		}
		_ = stAdminStore(d).STMarkGeoIPSourceRefreshed(r.Context(), row.ID)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func hSTAdminTestGeoIPSource(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid source id")
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<10))
		var req struct {
			IP string `json:"ip"`
		}
		_ = json.Unmarshal(body, &req)
		if req.IP == "" {
			req.IP = clientIP(r)
		}
		row, err := stAdminStore(d).STGetGeoIPSource(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "st_not_found", "source not found")
			return
		}
		if err != nil {
			writeInternal(w, r, d, "st_geoip_get_failed", err)
			return
		}
		src, err := geoip.BuildSource(row, geoipCacheDir(d))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "st_geoip_bad_config", err.Error())
			return
		}
		country, srcErr := src.Resolve(r.Context(), req.IP, r)
		writeJSON(w, http.StatusOK, map[string]any{
			"ip":      req.IP,
			"country": country,
			"error":   errString(srcErr),
		})
	}
}

func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// geoipCacheDir picks the on-disk cache dir for mmdb_auto downloads.
// Pulled from Deps for testability.
func geoipCacheDir(d Deps) string {
	if d.STGeoIPCacheDir != "" {
		return d.STGeoIPCacheDir
	}
	return "/var/cache/continuum-plugin-support/geoip"
}

// --- Results + dashboards -------------------------------------------

func hSTAdminListResults(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.STResultFilter{
			CustomerID:   r.URL.Query().Get("customerId"),
			EndpointID:   parseInt64(r.URL.Query().Get("endpointId")),
			AutoStrategy: r.URL.Query().Get("autoStrategy"),
			Limit:        parseLimit(r.URL.Query().Get("limit"), 200),
			Offset:       parseInt(r.URL.Query().Get("offset")),
		}
		if since := r.URL.Query().Get("since"); since != "" {
			if ts, err := time.Parse(time.RFC3339, since); err == nil {
				f.Since = ts
			}
		}
		if r.URL.Query().Get("slowOnly") == "true" {
			f.SlowOnly = true
			f.SlowThresh = d.STSlowThresholdMbps
		}
		out, err := stAdminStore(d).STListResults(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "st_results_list_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func hSTAdminDashboards(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := stAdminStore(d).STDashboardAggregatesData(r.Context(), d.STSlowThresholdMbps)
		if err != nil {
			writeInternal(w, r, d, "st_dashboards_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
EOF
```

This file adds two more `Deps` fields used by `geoipCacheDir` and inline. Update `server.go`'s `Deps` (Phase E2 added 3 fields; add `STGeoIPCacheDir` now):

```go
type Deps struct {
    DatabaseURL         string
    Logger              hclog.Logger
    ConfigStore         ConfigStore
    EventPublisher      EventPublisher

    STAutoResolver      *speedtest.Resolver
    STClientIPStorage   string
    STSlowThresholdMbps float64
    STGeoIPCacheDir     string
}
```

Verify + commit:

```bash
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_st_admin.go internal/server/server.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): speedtest admin handlers (endpoints / geoip / results / dashboards)"
```

---

## Phase G — Routes + manifest + main.go wiring

### Task G1: Register ST routes in `server.go`

Edit `internal/server/server.go`'s `New(d Deps)` and append after the existing route block:

```go
	// Speedtest module routes.
	r.Get("/speedtest",                       requireUser(hSTSpeedtestPage(d)))
	r.Get("/api/customer/speedtest/endpoints",requireUser(hSTCustomerEndpoints(d)))
	r.Get("/api/customer/speedtest/auto",     requireUser(hSTCustomerAuto(d)))
	r.Post("/api/customer/speedtest/results", requireUser(hSTCustomerSaveResult(d)))
	r.Get("/api/customer/speedtest/results",  requireUser(hSTCustomerHistory(d)))

	r.Get("/admin/speedtest",            requireAdmin(hSTAdminEndpointsPage(d)))
	r.Get("/admin/speedtest/geoip",      requireAdmin(hSTAdminGeoIPPage(d)))
	r.Get("/admin/speedtest/results",    requireAdmin(hSTAdminResultsPage(d)))
	r.Get("/admin/speedtest/dashboards", requireAdmin(hSTAdminDashboardsPage(d)))

	r.Get   ("/api/admin/speedtest/endpoints",          requireAdmin(hSTAdminListEndpoints(d)))
	r.Post  ("/api/admin/speedtest/endpoints",          requireAdmin(hSTAdminCreateEndpoint(d)))
	r.Put   ("/api/admin/speedtest/endpoints/{id}",     requireAdmin(hSTAdminUpdateEndpoint(d)))
	r.Delete("/api/admin/speedtest/endpoints/{id}",     requireAdmin(hSTAdminDeleteEndpoint(d)))
	r.Post  ("/api/admin/speedtest/endpoints/{id}/ping",requireAdmin(hSTAdminPingEndpoint(d)))

	r.Get   ("/api/admin/speedtest/geoip",              requireAdmin(hSTAdminListGeoIPSources(d)))
	r.Post  ("/api/admin/speedtest/geoip",              requireAdmin(hSTAdminCreateGeoIPSource(d)))
	r.Put   ("/api/admin/speedtest/geoip/{id}",         requireAdmin(hSTAdminUpdateGeoIPSource(d)))
	r.Delete("/api/admin/speedtest/geoip/{id}",         requireAdmin(hSTAdminDeleteGeoIPSource(d)))
	r.Post  ("/api/admin/speedtest/geoip/{id}/refresh", requireAdmin(hSTAdminRefreshGeoIPSource(d)))
	r.Post  ("/api/admin/speedtest/geoip/{id}/test",    requireAdmin(hSTAdminTestGeoIPSource(d)))

	r.Get   ("/api/admin/speedtest/results",            requireAdmin(hSTAdminListResults(d)))
	r.Get   ("/api/admin/speedtest/dashboards",         requireAdmin(hSTAdminDashboards(d)))
```

```bash
go build ./...
```

### Task G2: Update manifest.json

Edit `cmd/continuum-plugin-support/manifest.json`. Bump `version` to `0.3.0`. Append (preserve all existing entries):

```json
    { "id": "st_browse",        "method": "GET",  "path": "/speedtest",                          "access": "user" },
    { "id": "st_api_endpoints", "method": "GET",  "path": "/api/customer/speedtest/endpoints",   "access": "user" },
    { "id": "st_api_auto",      "method": "GET",  "path": "/api/customer/speedtest/auto",        "access": "user" },
    { "id": "st_api_results",   "method": "*",    "path": "/api/customer/speedtest/results",     "access": "user" },
    { "id": "st_admin_pages",   "method": "GET",  "path": "/admin/speedtest/*",                  "access": "admin" },
    { "id": "st_admin_root",    "method": "GET",  "path": "/admin/speedtest",                    "access": "admin" },
    { "id": "st_admin_api",     "method": "*",    "path": "/api/admin/speedtest/*",              "access": "admin" }
```

### Task G3: Wire `main.go` (chain + downloader + resolver)

Edit `cmd/continuum-plugin-support/main.go`. Inside `applyConfig`, AFTER `st := store.New(pool)` and BEFORE `httpSrv.SetHandler(server.New(...))`, build the GeoIP chain + resolver:

```go
	// Build GeoIP chain from the store's active source rows.
	geoipSources := []geoip.Source{}
	srcRows, _ := st.STListGeoIPSources(ctx, true)
	for _, row := range srcRows {
		src, err := geoip.BuildSource(row, cfg.GeoIPCacheDir)
		if err != nil {
			logger.Warn("skip bad geoip source", "id", row.ID, "err", err)
			continue
		}
		geoipSources = append(geoipSources, src)
		// mmdb_auto: load cached file (if present) and fire-and-forget refresh.
		if mauto, ok := src.(*geoip.MMDBAutoSource); ok {
			if err := mauto.LoadCached(); err == nil {
				logger.Info("loaded cached geoip mmdb", "source_id", row.ID, "path", mauto.LocalPath())
			}
			if mauto.NeedsRefresh() {
				go func(m *geoip.MMDBAutoSource, id int64) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()
					if err := m.Refresh(ctx); err != nil {
						_ = st.STMarkGeoIPSourceStatus(ctx, id, "error: "+err.Error())
						logger.Warn("geoip mmdb refresh failed", "source_id", id, "err", err)
						return
					}
					_ = st.STMarkGeoIPSourceRefreshed(ctx, id)
					logger.Info("geoip mmdb refreshed", "source_id", id)
				}(mauto, row.ID)
			}
		}
	}
	chain := geoip.NewChain(geoipSources, &geoipStatusSink{st: st})

	resolver := speedtest.NewResolver(st, chain, cfg.AutoStrategy)

	httpSrv.SetHandler(server.New(server.Deps{
		DatabaseURL:         cfg.DatabaseURL,
		Logger:              logger,
		ConfigStore:         st,
		EventPublisher:      publisher,
		STAutoResolver:      resolver,
		STClientIPStorage:   cfg.ClientIPStorage,
		STSlowThresholdMbps: cfg.SlowThresholdMbps,
		STGeoIPCacheDir:     cfg.GeoIPCacheDir,
	}))
```

Add a tiny status-sink adapter at the bottom of main.go:

```go
type geoipStatusSink struct{ st *store.Store }

func (s *geoipStatusSink) MarkUsed(id int64) {
	_ = s.st.STMarkGeoIPSourceUsed(context.Background(), id)
}
func (s *geoipStatusSink) MarkStatus(id int64, status string) {
	_ = s.st.STMarkGeoIPSourceStatus(context.Background(), id, status)
}
```

Imports to add at the top of `main.go`:

```go
"time"

"github.com/ContinuumApp/continuum-plugin-support/internal/speedtest"
"github.com/ContinuumApp/continuum-plugin-support/internal/speedtest/geoip"
```

### Task G4: Add Config fields + flip Modules.Speedtest default

Edit `internal/runtime/runtime.go`. Locate the `Config` struct and append:

```go
type Config struct {
    DatabaseURL       string         `json:"-"`
    Modules           ModuleToggles  `json:"modules"`

    // Speedtest module config.
    AutoStrategy      string  `json:"auto_strategy,omitempty"`        // "latency" (default) | "geoip"
    GeoIPCacheDir     string  `json:"geoip_cache_dir,omitempty"`
    ClientIPStorage   string  `json:"client_ip_storage,omitempty"`    // "truncated" (default) | "off"
    SlowThresholdMbps float64 `json:"slow_threshold_mbps,omitempty"`
}
```

Update `DefaultAppConfig()`:

```go
func DefaultAppConfig() Config {
	return Config{
		Modules:           ModuleToggles{KB: true, Speedtest: true},
		AutoStrategy:      "latency",
		ClientIPStorage:   "truncated",
		SlowThresholdMbps: 5,
	}
}
```

Update `NormalizeAppConfig` to validate the strategy + storage:

```go
func NormalizeAppConfig(cfg Config) (Config, error) {
	if cfg.AutoStrategy == "" {
		cfg.AutoStrategy = "latency"
	}
	if cfg.AutoStrategy != "latency" && cfg.AutoStrategy != "geoip" {
		return Config{}, fmt.Errorf("auto_strategy must be 'latency' or 'geoip'")
	}
	if cfg.ClientIPStorage == "" {
		cfg.ClientIPStorage = "truncated"
	}
	if cfg.ClientIPStorage != "truncated" && cfg.ClientIPStorage != "off" {
		return Config{}, fmt.Errorf("client_ip_storage must be 'truncated' or 'off'")
	}
	if cfg.SlowThresholdMbps < 0 {
		return Config{}, fmt.Errorf("slow_threshold_mbps must be >= 0")
	}
	return cfg, nil
}
```

Also update `internal/runtime/runtime_test.go` `TestConfigureDefaultsKBOnOthersOff` to assert Speedtest defaults on (rename if needed):

```go
if !observed.Modules.KB || !observed.Modules.Speedtest {
    t.Fatalf("KB + Speedtest should default ON; got %+v", observed.Modules)
}
if observed.Modules.Tickets || observed.Modules.AI {
    t.Fatalf("non-shipped modules should still default off; got %+v", observed.Modules)
}
```

### Task G5: Verify + single commit for G1-G4

```bash
cd /opt/continuum_plugins/continuum-plugin-support
go build ./...
go test ./...
GOWORK=off go build ./...

git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/server.go cmd/continuum-plugin-support/manifest.json cmd/continuum-plugin-support/main.go internal/runtime/runtime.go internal/runtime/runtime_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat: wire speedtest routes + main.go + manifest 0.3.0 + Modules.Speedtest default true"
```

---

## Phase H — Server integration tests (PG_DSN-gated)

### Task H1: KB-style integration test sweep for speedtest

**Files:**
- Create: `internal/server/server_st_test.go`

```bash
cat > internal/server/server_st_test.go <<'EOF'
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ContinuumApp/continuum-plugin-support/internal/migrate"
	"github.com/ContinuumApp/continuum-plugin-support/internal/speedtest"
	"github.com/ContinuumApp/continuum-plugin-support/internal/store"
)

func stTestDeps(t *testing.T) (Deps, *store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN unset; skipping speedtest integration test")
	}
	ctx := context.Background()
	if err := migrate.Run(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	st := store.New(pool)
	// Resolver with nil geoip — latency mode returns candidates.
	resolver := speedtest.NewResolver(st, nil, "latency")
	d := Deps{
		ConfigStore:         st,
		STAutoResolver:      resolver,
		STClientIPStorage:   "truncated",
		STSlowThresholdMbps: 5,
	}
	return d, st, func() { pool.Close() }
}

func TestSTCustomerEndpointsRequiresAuth(t *testing.T) {
	d, _, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/endpoints", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestSTAdminRoutesRejectNonAdmin(t *testing.T) {
	d, _, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)
	for _, path := range []string{
		"/admin/speedtest",
		"/api/admin/speedtest/endpoints",
		"/api/admin/speedtest/geoip",
		"/api/admin/speedtest/results",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Continuum-User-Id", "42")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("path %s status = %d, want 403", path, rec.Code)
		}
	}
}

func TestSTEndpointCRUDRoundTrip(t *testing.T) {
	d, _, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)

	// Create endpoint
	body := `{"label":"London","url":"https://lon/","country":"GB","sortOrder":0,"active":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/speedtest/endpoints", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create endpoint status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var ep store.STEndpoint
	if err := json.Unmarshal(rec.Body.Bytes(), &ep); err != nil {
		t.Fatal(err)
	}

	// List as customer
	req = httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/endpoints", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("customer list status = %d", rec.Code)
	}

	// Save a result
	rbody := fmt.Sprintf(`{"endpointId":%d,"endpointLabel":"London","downloadMbps":142.3,"uploadMbps":18.7,"pingMs":28,"jitterMs":2.1}`, ep.ID)
	req = httptest.NewRequest(http.MethodPost, "/api/customer/speedtest/results", bytes.NewBufferString(rbody))
	req.Header.Set("X-Continuum-User-Id", "9")
	req.RemoteAddr = "192.0.2.50:1234"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save result status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// Second result within 60s -> 429
	req = httptest.NewRequest(http.MethodPost, "/api/customer/speedtest/results", bytes.NewBufferString(rbody))
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate-limit 429, got %d", rec.Code)
	}

	// History
	req = httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/results", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d", rec.Code)
	}
}

func TestSTAutoLatencyReturnsCandidates(t *testing.T) {
	d, st, cleanup := stTestDeps(t)
	defer cleanup()
	h := New(d)

	// Seed an endpoint so the resolver has something to return.
	_, err := st.STCreateEndpoint(context.Background(), store.STEndpoint{
		Label: "London", URL: "https://lon/", Country: "GB", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customer/speedtest/auto", nil)
	req.Header.Set("X-Continuum-User-Id", "9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("auto status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out speedtest.AutoResolution
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Strategy != "latency" || len(out.Candidates) == 0 {
		t.Fatalf("unexpected resolution: %+v", out)
	}
}
EOF
go test ./internal/server/... -v -run TestST 2>&1 | tail -15   # all skip without PG_DSN
GOWORK=off go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/server_st_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "test(server): speedtest integration tests (PG_DSN-gated)"
```

---

## Phase I — SPA types + bootstrap + API clients

### Task I1: Extend `lib/types.ts` + bootstrap mode union

**Files:**
- Modify: `web/src/lib/types.ts`

Read the current file first. Append the following AFTER the existing exports:

```ts
export type STEndpoint = {
  id: number;
  label: string;
  url: string;
  country: string;
  region: string;
  sortOrder: number;
  active: boolean;
  createdAt: string;
  updatedAt: string;
};

export type STGeoIPSourceKind = "mmdb_auto" | "mmdb_file" | "http_api" | "request_header";

export type STGeoIPSource = {
  id: number;
  label: string;
  kind: STGeoIPSourceKind;
  config: Record<string, unknown>;
  sortOrder: number;
  active: boolean;
  lastStatus: string;
  lastUsedAt?: string | null;
  lastRefreshedAt?: string | null;
  createdAt: string;
  updatedAt: string;
};

export type STResult = {
  id: number;
  customerId: string;
  endpointId?: number | null;
  endpointLabel: string;
  autoStrategy: string;
  downloadMbps: number;
  uploadMbps: number;
  pingMs: number;
  jitterMs: number;
  clientIp?: string;
  userAgent?: string;
  ranAt: string;
};

export type STAutoResolution = {
  strategy: "latency" | "geoip" | "fallback";
  endpoint?: STEndpoint | null;
  candidates?: STEndpoint[];
  geoip: { country?: string; sourceId?: number; sourceLabel?: string };
};

export type STDashboardAggregates = {
  perEndpoint: Array<{
    endpointId?: number | null;
    label: string;
    medianDownload: number;
    medianUpload: number;
    medianPing: number;
    resultCount: number;
  }>;
  perDay: Array<{ day: string; count: number }>;
  slowTop10: STResult[];
  countryHits: Array<{ country: string; count: number }>;
};
```

Extend `SupportBootstrap.mode` to add the 5 new modes:

```ts
  mode:
    | "customer-home"
    | "admin-home"
    | "kb-browse"
    | "kb-detail"
    | "admin-kb-list"
    | "admin-kb-edit"
    | "admin-kb-categories"
    | "admin-kb-tags"
    | "speedtest"
    | "admin-st-endpoints"
    | "admin-st-geoip"
    | "admin-st-results"
    | "admin-st-dashboards";
```

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cd web && pnpm test && pnpm exec tsc -b --noEmit && cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/lib/types.ts
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): speedtest types + bootstrap mode union extension"
```

---

### Task I2: Customer + admin API clients

**Files:**
- Create: `web/src/api/st.ts`
- Create: `web/src/api/stAdmin.ts`

```bash
cat > web/src/api/st.ts <<'EOF'
import { api } from "@/lib/api";
import type { STAutoResolution, STEndpoint, STResult } from "@/lib/types";

export function listSTEndpoints(): Promise<STEndpoint[]> {
  return api<STEndpoint[]>("/api/customer/speedtest/endpoints");
}

export function getSTAuto(): Promise<STAutoResolution> {
  return api<STAutoResolution>("/api/customer/speedtest/auto");
}

export type STSaveResultPayload = {
  endpointId?: number;
  endpointLabel: string;
  autoStrategy?: string;
  downloadMbps: number;
  uploadMbps: number;
  pingMs: number;
  jitterMs: number;
};

export function saveSTResult(p: STSaveResultPayload): Promise<STResult> {
  return api<STResult>("/api/customer/speedtest/results", {
    method: "POST",
    body: JSON.stringify(p),
  });
}

export function getSTHistory(): Promise<STResult[]> {
  return api<STResult[]>("/api/customer/speedtest/results");
}
EOF

cat > web/src/api/stAdmin.ts <<'EOF'
import { api } from "@/lib/api";
import type {
  STDashboardAggregates,
  STEndpoint,
  STGeoIPSource,
  STGeoIPSourceKind,
  STResult,
} from "@/lib/types";

// Endpoints --------------------------------------------------

export function listSTEndpointsAdmin(): Promise<STEndpoint[]> {
  return api<STEndpoint[]>("/api/admin/speedtest/endpoints");
}

export type STEndpointWrite = {
  label: string;
  url: string;
  country: string;
  region: string;
  sortOrder: number;
  active: boolean;
};

export function createSTEndpoint(w: STEndpointWrite): Promise<STEndpoint> {
  return api<STEndpoint>("/api/admin/speedtest/endpoints", {
    method: "POST", body: JSON.stringify(w),
  });
}

export function updateSTEndpoint(id: number, w: STEndpointWrite): Promise<STEndpoint> {
  return api<STEndpoint>(`/api/admin/speedtest/endpoints/${id}`, {
    method: "PUT", body: JSON.stringify(w),
  });
}

export function deleteSTEndpoint(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/speedtest/endpoints/${id}`, { method: "DELETE" });
}

export function pingSTEndpoint(id: number): Promise<{ ok: boolean; status?: number; error?: string; elapsed_ms: number }> {
  return api(`/api/admin/speedtest/endpoints/${id}/ping`, { method: "POST" });
}

// GeoIP sources ----------------------------------------------

export function listSTGeoIPSourcesAdmin(): Promise<STGeoIPSource[]> {
  return api<STGeoIPSource[]>("/api/admin/speedtest/geoip");
}

export type STGeoIPSourceWrite = {
  label: string;
  kind: STGeoIPSourceKind;
  config: Record<string, unknown>;
  sortOrder: number;
  active: boolean;
};

export function createSTGeoIPSource(w: STGeoIPSourceWrite): Promise<STGeoIPSource> {
  return api<STGeoIPSource>("/api/admin/speedtest/geoip", {
    method: "POST", body: JSON.stringify(w),
  });
}

export function updateSTGeoIPSource(id: number, w: STGeoIPSourceWrite): Promise<STGeoIPSource> {
  return api<STGeoIPSource>(`/api/admin/speedtest/geoip/${id}`, {
    method: "PUT", body: JSON.stringify(w),
  });
}

export function deleteSTGeoIPSource(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/speedtest/geoip/${id}`, { method: "DELETE" });
}

export function refreshSTGeoIPSource(id: number): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>(`/api/admin/speedtest/geoip/${id}/refresh`, { method: "POST" });
}

export function testSTGeoIPSource(id: number, ip?: string): Promise<{ ip: string; country: string; error: string }> {
  return api(`/api/admin/speedtest/geoip/${id}/test`, {
    method: "POST",
    body: JSON.stringify({ ip: ip ?? "" }),
  });
}

// Results + dashboards ---------------------------------------

export type STResultsListParams = {
  customerId?: string;
  endpointId?: number;
  autoStrategy?: string;
  slowOnly?: boolean;
  since?: string;
  limit?: number;
  offset?: number;
};

export function listSTResultsAdmin(p: STResultsListParams = {}): Promise<STResult[]> {
  const qs = new URLSearchParams();
  if (p.customerId) qs.set("customerId", p.customerId);
  if (p.endpointId) qs.set("endpointId", String(p.endpointId));
  if (p.autoStrategy) qs.set("autoStrategy", p.autoStrategy);
  if (p.slowOnly) qs.set("slowOnly", "true");
  if (p.since) qs.set("since", p.since);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/admin/speedtest/results" + (qs.toString() ? `?${qs}` : "");
  return api<STResult[]>(path);
}

export function getSTDashboards(): Promise<STDashboardAggregates> {
  return api<STDashboardAggregates>("/api/admin/speedtest/dashboards");
}
EOF

cd web && pnpm exec tsc -b --noEmit && cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/api/st.ts web/src/api/stAdmin.ts
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): speedtest customer + admin API clients"
```

---

## Phase J — LibreSpeed wrapper

### Task J1: Vendor the LibreSpeed worker

**Files:**
- Create: `web/public/speedtest_worker.js`

The LibreSpeed JS client is the file `speedtest_worker.js` from
<https://github.com/librespeed/speedtest> (GPL-2 licensed; vendoring
in our GPL-compatible / commercial-but-internal plugin is fine).
Vendor the file verbatim — do not modify.

```bash
cd /opt/continuum_plugins/continuum-plugin-support
mkdir -p web/public
curl -fsSL https://raw.githubusercontent.com/librespeed/speedtest/master/speedtest_worker.js \
     -o web/public/speedtest_worker.js
```

Sanity check: the file should be ~50–80 KB and start with a comment block + `var settings = {...}`.

```bash
wc -l web/public/speedtest_worker.js
head -20 web/public/speedtest_worker.js
```

Vite serves `web/public/*` at the root, so the SPA can load it as `/speedtest_worker.js`. Since the SPA is mounted via Continuum's plugin router, the actual served path is `<mount>/speedtest_worker.js` — confirm by examining how the shell's main.tsx / index.html references static assets and adjust the loader path in Task J2 accordingly.

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/public/speedtest_worker.js
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "chore(web): vendor LibreSpeed speedtest_worker.js (GPL-2)"
```

### Task J2: `librespeedClient.ts` — typed wrapper

**Files:**
- Create: `web/src/lib/librespeedClient.ts`

The wrapper instantiates the worker, sets the endpoint, exposes
start / abort, and emits typed progress events. LibreSpeed's worker
uses `postMessage` with a comma-separated CSV string of measured
values — the wrapper parses that into typed events.

```bash
cat > web/src/lib/librespeedClient.ts <<'EOF'
// Typed wrapper around LibreSpeed's speedtest_worker.js.
// The worker is loaded as a Web Worker and emits CSV-formatted
// progress messages: `<state>;<dlStatus>;<ulStatus>;<pingStatus>;<...>`.
// State: 0=idle, 1=download, 2=ping, 3=upload, 4=done, 5=abort.

export type LibreSpeedPhase = "idle" | "download" | "ping" | "upload" | "done" | "abort";

export type LibreSpeedProgress = {
  phase: LibreSpeedPhase;
  download: number;  // Mbit/s
  upload: number;
  ping: number;       // ms
  jitter: number;
};

export type LibreSpeedParams = {
  endpointURL: string;          // base URL of the LibreSpeed endpoint
  workerURL?: string;           // defaults to "./speedtest_worker.js"
  onProgress: (p: LibreSpeedProgress) => void;
  onError: (msg: string) => void;
};

export type LibreSpeedRunner = {
  start: () => void;
  abort: () => void;
};

export function createLibreSpeedRunner(params: LibreSpeedParams): LibreSpeedRunner {
  const workerURL = params.workerURL ?? "./speedtest_worker.js";
  const worker = new Worker(workerURL);
  let lastPhase: LibreSpeedPhase = "idle";

  worker.onmessage = (e: MessageEvent<string>) => {
    const parts = e.data.split(";");
    const stateRaw = parseInt(parts[0] ?? "0", 10);
    const phase = phaseFromState(stateRaw);
    const progress: LibreSpeedProgress = {
      phase,
      download: parseFloat(parts[1] ?? "0") || 0,
      upload: parseFloat(parts[2] ?? "0") || 0,
      ping: parseFloat(parts[3] ?? "0") || 0,
      jitter: parseFloat(parts[4] ?? "0") || 0,
    };
    params.onProgress(progress);
    if (phase === "done" || phase === "abort") {
      lastPhase = phase;
      worker.terminate();
    } else {
      lastPhase = phase;
    }
  };
  worker.onerror = (e) => params.onError(e.message || "speedtest worker error");

  return {
    start: () => {
      // LibreSpeed accepts a config object via postMessage("start <json>").
      const config = {
        url_dl: params.endpointURL.replace(/\/$/, "") + "/garbage.php",
        url_ul: params.endpointURL.replace(/\/$/, "") + "/empty.php",
        url_ping: params.endpointURL.replace(/\/$/, "") + "/empty.php",
        time_dl_max: 15,
        time_ul_max: 15,
        count_ping: 10,
      };
      worker.postMessage("start " + JSON.stringify(config));
    },
    abort: () => {
      if (lastPhase !== "done" && lastPhase !== "abort") {
        worker.postMessage("abort");
      }
    },
  };
}

function phaseFromState(s: number): LibreSpeedPhase {
  switch (s) {
    case 1: return "download";
    case 2: return "ping";
    case 3: return "upload";
    case 4: return "done";
    case 5: return "abort";
    default: return "idle";
  }
}
EOF

cd web && pnpm exec tsc -b --noEmit && cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/lib/librespeedClient.ts
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): typed LibreSpeed worker wrapper"
```

---

## Phase K — Customer SPA

### Task K1: EndpointPicker (TDD) + SpeedGauge + HistoryList

**Files:**
- Create: `web/src/components/st/EndpointPicker.tsx` (+ test)
- Create: `web/src/components/st/SpeedGauge.tsx`
- Create: `web/src/components/st/HistoryList.tsx`

```bash
cd /opt/continuum_plugins/continuum-plugin-support
mkdir -p web/src/components/st

cat > web/src/components/st/EndpointPicker.test.tsx <<'EOF'
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { EndpointPicker } from "./EndpointPicker";
import type { STEndpoint } from "@/lib/types";

const endpoints: STEndpoint[] = [
  { id: 1, label: "London",    url: "https://lon/", country: "GB", region: "", sortOrder: 0, active: true, createdAt: "", updatedAt: "" },
  { id: 2, label: "Frankfurt", url: "https://fra/", country: "DE", region: "", sortOrder: 1, active: true, createdAt: "", updatedAt: "" },
];

describe("EndpointPicker", () => {
  afterEach(() => cleanup());

  it("renders 'Auto' plus one option per endpoint", () => {
    render(<EndpointPicker endpoints={endpoints} value="auto" onChange={() => {}} />);
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(3); // Auto + 2 endpoints
  });

  it("calls onChange with 'auto' or the endpoint id when selected", () => {
    const onChange = vi.fn();
    render(<EndpointPicker endpoints={endpoints} value="auto" onChange={onChange} />);
    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "2" } });
    expect(onChange).toHaveBeenLastCalledWith(2);
    fireEvent.change(select, { target: { value: "auto" } });
    expect(onChange).toHaveBeenLastCalledWith("auto");
  });
});
EOF

cat > web/src/components/st/EndpointPicker.tsx <<'EOF'
import type { STEndpoint } from "@/lib/types";

type Props = {
  endpoints: STEndpoint[];
  value: "auto" | number;
  onChange: (v: "auto" | number) => void;
  disabled?: boolean;
};

export function EndpointPicker({ endpoints, value, onChange, disabled }: Props) {
  return (
    <select
      role="combobox"
      disabled={disabled}
      value={value === "auto" ? "auto" : String(value)}
      onChange={(e) => {
        const v = e.target.value;
        onChange(v === "auto" ? "auto" : Number(v));
      }}
      className="rounded border border-border bg-background px-2 py-1 text-sm"
    >
      <option value="auto">Auto</option>
      {endpoints.map((e) => (
        <option key={e.id} value={String(e.id)}>{e.label}{e.country ? ` (${e.country})` : ""}</option>
      ))}
    </select>
  );
}
EOF

cat > web/src/components/st/SpeedGauge.tsx <<'EOF'
import type { LibreSpeedProgress } from "@/lib/librespeedClient";

type Props = { progress: LibreSpeedProgress | null };

export function SpeedGauge({ progress }: Props) {
  const dl = progress?.download ?? 0;
  const up = progress?.upload ?? 0;
  const pg = progress?.ping ?? 0;
  const jt = progress?.jitter ?? 0;
  const phase = progress?.phase ?? "idle";
  return (
    <div className="rounded-md border border-border bg-card p-6 space-y-3 text-center">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Stat label="Download" value={dl} unit="Mb/s" />
        <Stat label="Upload"   value={up} unit="Mb/s" />
        <Stat label="Ping"     value={pg} unit="ms" />
        <Stat label="Jitter"   value={jt} unit="ms" />
      </div>
      <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">
        {phaseLabel(phase)}
      </p>
    </div>
  );
}

function Stat({ label, value, unit }: { label: string; value: number; unit: string }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">{label}</p>
      <p className="text-2xl font-semibold tabular-nums">
        {value.toFixed(value < 10 ? 2 : 1)}<span className="text-sm font-normal text-muted-foreground ml-1">{unit}</span>
      </p>
    </div>
  );
}

function phaseLabel(p: LibreSpeedProgress["phase"]): string {
  switch (p) {
    case "download": return "Downloading…";
    case "upload":   return "Uploading…";
    case "ping":     return "Measuring ping…";
    case "done":     return "Done";
    case "abort":    return "Cancelled";
    default:         return "Ready";
  }
}
EOF

cat > web/src/components/st/HistoryList.tsx <<'EOF'
import { Card, CardContent } from "@/components/ui/card";
import type { STResult } from "@/lib/types";

type Props = { history: STResult[] };

export function HistoryList({ history }: Props) {
  if (history.length === 0) {
    return (
      <Card>
        <CardContent className="py-6 text-center text-sm text-muted-foreground">
          No tests yet — run one above to start your history.
        </CardContent>
      </Card>
    );
  }
  return (
    <ul className="divide-y divide-border rounded-md border border-border">
      {history.slice(0, 5).map((r) => (
        <li key={r.id} className="flex items-center gap-3 px-3 py-2 text-sm">
          <span className="font-mono text-xs text-muted-foreground tabular-nums">
            {new Date(r.ranAt).toLocaleString()}
          </span>
          <span className="flex-1 font-medium">{r.endpointLabel}</span>
          <span className="tabular-nums">↓ {r.downloadMbps.toFixed(1)}</span>
          <span className="tabular-nums">↑ {r.uploadMbps.toFixed(1)}</span>
          <span className="tabular-nums">{r.pingMs.toFixed(0)} ms</span>
        </li>
      ))}
    </ul>
  );
}
EOF

cd web && pnpm test 2>&1 | tail -6
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/st/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): speedtest EndpointPicker + SpeedGauge + HistoryList"
```

---

### Task K2: Speedtest page

**Files:**
- Create: `web/src/pages/st/Speedtest.tsx`

```bash
mkdir -p web/src/pages/st
cat > web/src/pages/st/Speedtest.tsx <<'EOF'
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { EndpointPicker } from "@/components/st/EndpointPicker";
import { HistoryList } from "@/components/st/HistoryList";
import { SpeedGauge } from "@/components/st/SpeedGauge";
import { TopBar } from "@/components/shared/TopBar";
import { Button } from "@/components/ui/button";
import {
  getSTAuto, getSTHistory, listSTEndpoints, saveSTResult,
} from "@/api/st";
import {
  createLibreSpeedRunner, type LibreSpeedProgress, type LibreSpeedRunner,
} from "@/lib/librespeedClient";
import type { STAutoResolution, STEndpoint, STResult } from "@/lib/types";

type Choice = "auto" | number;

export function Speedtest() {
  const [endpoints, setEndpoints] = useState<STEndpoint[]>([]);
  const [history, setHistory] = useState<STResult[]>([]);
  const [choice, setChoice] = useState<Choice>("auto");
  const [autoRes, setAutoRes] = useState<STAutoResolution | null>(null);
  const [progress, setProgress] = useState<LibreSpeedProgress | null>(null);
  const [running, setRunning] = useState(false);
  const runnerRef = useRef<LibreSpeedRunner | null>(null);

  useEffect(() => {
    listSTEndpoints().then(setEndpoints).catch(() => {});
    getSTHistory().then(setHistory).catch(() => {});
    getSTAuto().then(setAutoRes).catch(() => {});
  }, []);

  function resolveEndpoint(): { endpoint: STEndpoint; strategy: string } | null {
    if (choice === "auto") {
      if (autoRes?.endpoint) {
        return { endpoint: autoRes.endpoint, strategy: autoRes.strategy };
      }
      // Latency mode: SPA picks the lowest-RTT candidate via parallel HEADs.
      // For v1 we fall back to "first candidate" if the latency probe
      // hasn't completed — a future iteration can run the probe here.
      const first = autoRes?.candidates?.[0];
      if (first) return { endpoint: first, strategy: "latency" };
      return null;
    }
    const ep = endpoints.find((e) => e.id === choice);
    return ep ? { endpoint: ep, strategy: "" } : null;
  }

  function runTest() {
    const resolved = resolveEndpoint();
    if (!resolved) {
      toast.error("No endpoint available — ask your admin to configure one.");
      return;
    }
    setRunning(true);
    setProgress({ phase: "idle", download: 0, upload: 0, ping: 0, jitter: 0 });
    runnerRef.current = createLibreSpeedRunner({
      endpointURL: resolved.endpoint.url,
      workerURL: "../speedtest_worker.js",
      onProgress: async (p) => {
        setProgress(p);
        if (p.phase === "done") {
          try {
            const saved = await saveSTResult({
              endpointId: resolved.endpoint.id,
              endpointLabel: resolved.endpoint.label,
              autoStrategy: choice === "auto" ? resolved.strategy : "",
              downloadMbps: p.download,
              uploadMbps: p.upload,
              pingMs: p.ping,
              jitterMs: p.jitter,
            });
            setHistory((h) => [saved, ...h]);
            toast.success("Test saved.");
          } catch (err) {
            toast.error(err instanceof Error ? err.message : "Save failed");
          } finally {
            setRunning(false);
          }
        }
        if (p.phase === "abort") {
          setRunning(false);
          toast.info("Test cancelled.");
        }
      },
      onError: (msg) => {
        toast.error(msg);
        setRunning(false);
      },
    });
    runnerRef.current.start();
  }

  function abortTest() {
    runnerRef.current?.abort();
  }

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <TopBar
          eyebrow="Support"
          title="Speedtest"
          subtitle="Test your connection against our endpoints."
        />
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground">Run against</span>
          <EndpointPicker endpoints={endpoints} value={choice} onChange={setChoice} disabled={running} />
          {!running && <Button onClick={runTest}>Run test</Button>}
          {running && <Button variant="destructive" onClick={abortTest}>Cancel</Button>}
        </div>
        {choice === "auto" && autoRes?.endpoint && (
          <p className="text-xs text-muted-foreground">
            Auto: {autoRes.endpoint.label}
            {autoRes.strategy === "geoip" && autoRes.geoip?.country
              ? ` · selected by geoip (${autoRes.geoip.country})` : ""}
            {autoRes.strategy === "latency" ? " · selected by latency" : ""}
            {autoRes.strategy === "fallback" ? " · fallback (no geoip / latency data)" : ""}
          </p>
        )}
        <SpeedGauge progress={progress} />
        <div>
          <h2 className="mb-2 text-sm font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            Your last 5 tests
          </h2>
          <HistoryList history={history} />
        </div>
      </div>
    </main>
  );
}
EOF

cd web && pnpm test && pnpm exec tsc -b --noEmit
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/pages/st/Speedtest.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): Speedtest customer page (runner state machine + history)"
```

---

## Phase L — Admin SPA

### Task L1: EndpointAdmin component + page

**Files:**
- Create: `web/src/components/admin/st/EndpointAdmin.tsx`
- Create: `web/src/pages/admin/st/Endpoints.tsx`

```bash
mkdir -p web/src/components/admin/st web/src/pages/admin/st

cat > web/src/components/admin/st/EndpointAdmin.tsx <<'EOF'
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  createSTEndpoint, deleteSTEndpoint, pingSTEndpoint, updateSTEndpoint,
} from "@/api/stAdmin";
import type { STEndpoint } from "@/lib/types";

type Props = { initial: STEndpoint[] };

export function EndpointAdmin({ initial }: Props) {
  const [rows, setRows] = useState<STEndpoint[]>(initial);
  const [draft, setDraft] = useState({ label: "", url: "", country: "" });

  async function add() {
    if (!draft.label.trim() || !draft.url.trim()) return;
    try {
      const saved = await createSTEndpoint({
        label: draft.label.trim(), url: draft.url.trim(),
        country: draft.country.trim().toUpperCase(), region: "",
        sortOrder: rows.length, active: true,
      });
      setRows((r) => [...r, saved]);
      setDraft({ label: "", url: "", country: "" });
    } catch (err) { toast.error(err instanceof Error ? err.message : "Create failed"); }
  }

  async function save(ep: STEndpoint) {
    try {
      const saved = await updateSTEndpoint(ep.id, {
        label: ep.label, url: ep.url, country: ep.country, region: ep.region,
        sortOrder: ep.sortOrder, active: ep.active,
      });
      setRows((rs) => rs.map((x) => x.id === saved.id ? saved : x));
      toast.success("Saved.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function remove(ep: STEndpoint) {
    if (!confirm(`Delete endpoint "${ep.label}"?`)) return;
    try {
      await deleteSTEndpoint(ep.id);
      setRows((rs) => rs.filter((x) => x.id !== ep.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  async function ping(ep: STEndpoint) {
    try {
      const res = await pingSTEndpoint(ep.id);
      if (res.ok) toast.success(`OK (${res.elapsed_ms}ms)`);
      else toast.error(`${res.error ?? `HTTP ${res.status}`} (${res.elapsed_ms}ms)`);
    } catch (err) { toast.error(err instanceof Error ? err.message : "Ping failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle>Endpoints</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 gap-2 md:grid-cols-4">
          <Input placeholder="Label" value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} />
          <Input placeholder="https://librespeed.example.com" value={draft.url} onChange={(e) => setDraft({ ...draft, url: e.target.value })} />
          <Input placeholder="GB" maxLength={2} value={draft.country} onChange={(e) => setDraft({ ...draft, country: e.target.value })} />
          <Button onClick={add}>Add</Button>
        </div>
        <ul className="divide-y divide-border">
          {rows.map((ep) => (
            <li key={ep.id} className="grid grid-cols-12 items-center gap-2 py-2 text-sm">
              <Input className="col-span-3" value={ep.label}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, label: e.target.value } : x))}
                     onBlur={() => save(ep)} />
              <Input className="col-span-5 font-mono text-xs" value={ep.url}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, url: e.target.value } : x))}
                     onBlur={() => save(ep)} />
              <Input className="col-span-1" maxLength={2} value={ep.country}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, country: e.target.value.toUpperCase() } : x))}
                     onBlur={() => save(ep)} />
              <Badge variant={ep.active ? "default" : "secondary"} className="col-span-1 justify-center">
                {ep.active ? "On" : "Off"}
              </Badge>
              <Switch className="col-span-1"
                      checked={ep.active}
                      onCheckedChange={(v) => {
                        setRows((rs) => rs.map((x) => x.id === ep.id ? { ...x, active: v } : x));
                        save({ ...ep, active: v });
                      }} />
              <div className="col-span-1 flex gap-1">
                <Button size="sm" variant="ghost" onClick={() => ping(ep)}>Ping</Button>
                <Button size="sm" variant="destructive" onClick={() => remove(ep)}>×</Button>
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
EOF

cat > web/src/pages/admin/st/Endpoints.tsx <<'EOF'
import { useEffect, useState } from "react";
import { EndpointAdmin } from "@/components/admin/st/EndpointAdmin";
import { listSTEndpointsAdmin } from "@/api/stAdmin";
import type { STEndpoint } from "@/lib/types";

export function STAdminEndpoints() {
  const [initial, setInitial] = useState<STEndpoint[] | null>(null);
  useEffect(() => { listSTEndpointsAdmin().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Speedtest endpoints</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <EndpointAdmin initial={initial} />}
    </section>
  );
}
EOF

cd web && pnpm exec tsc -b --noEmit && cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/admin/st/EndpointAdmin.tsx web/src/pages/admin/st/Endpoints.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): speedtest admin Endpoints page"
```

---

### Task L2: GeoIPSourceAdmin + page

**Files:**
- Create: `web/src/components/admin/st/GeoIPSourceAdmin.tsx`
- Create: `web/src/pages/admin/st/GeoIP.tsx`

```bash
cat > web/src/components/admin/st/GeoIPSourceAdmin.tsx <<'EOF'
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  createSTGeoIPSource, deleteSTGeoIPSource, refreshSTGeoIPSource,
  testSTGeoIPSource, updateSTGeoIPSource,
} from "@/api/stAdmin";
import type { STGeoIPSource, STGeoIPSourceKind } from "@/lib/types";

type Props = { initial: STGeoIPSource[] };

const DEFAULT_CONFIG: Record<STGeoIPSourceKind, Record<string, unknown>> = {
  mmdb_auto:     { url_pattern: "https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.mmdb.gz", refresh_days: 25 },
  mmdb_file:     { path: "/var/lib/maxmind/GeoLite2-Country.mmdb" },
  http_api:      { url_pattern: "https://ipapi.co/{ip}/country/", format: "text" },
  request_header:{ header: "CF-IPCountry" },
};

export function GeoIPSourceAdmin({ initial }: Props) {
  const [rows, setRows] = useState<STGeoIPSource[]>(initial);
  const [newKind, setNewKind] = useState<STGeoIPSourceKind>("http_api");
  const [newLabel, setNewLabel] = useState("");

  async function add() {
    if (!newLabel.trim()) return;
    try {
      const saved = await createSTGeoIPSource({
        label: newLabel.trim(), kind: newKind,
        config: DEFAULT_CONFIG[newKind],
        sortOrder: rows.length, active: true,
      });
      setRows((r) => [...r, saved]);
      setNewLabel("");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Create failed"); }
  }

  async function save(src: STGeoIPSource) {
    try {
      const saved = await updateSTGeoIPSource(src.id, {
        label: src.label, kind: src.kind, config: src.config,
        sortOrder: src.sortOrder, active: src.active,
      });
      setRows((rs) => rs.map((x) => x.id === saved.id ? saved : x));
      toast.success("Saved.");
    } catch (err) { toast.error(err instanceof Error ? err.message : "Save failed"); }
  }

  async function remove(src: STGeoIPSource) {
    if (!confirm(`Delete source "${src.label}"?`)) return;
    try {
      await deleteSTGeoIPSource(src.id);
      setRows((rs) => rs.filter((x) => x.id !== src.id));
    } catch (err) { toast.error(err instanceof Error ? err.message : "Delete failed"); }
  }

  async function refresh(src: STGeoIPSource) {
    try { await refreshSTGeoIPSource(src.id); toast.success("Refresh queued."); }
    catch (err) { toast.error(err instanceof Error ? err.message : "Refresh failed"); }
  }

  async function test(src: STGeoIPSource) {
    const ip = window.prompt("Test IP (leave empty for your own IP)", "");
    try {
      const res = await testSTGeoIPSource(src.id, ip ?? undefined);
      if (res.error) toast.error(`${res.error}`);
      else toast.success(`${res.ip} → ${res.country || "(no country)"}`);
    } catch (err) { toast.error(err instanceof Error ? err.message : "Test failed"); }
  }

  async function reorder(srcID: number, dir: -1 | 1) {
    const idx = rows.findIndex((r) => r.id === srcID);
    if (idx < 0) return;
    const other = idx + dir;
    if (other < 0 || other >= rows.length) return;
    const a = rows[idx], b = rows[other];
    const next = [...rows];
    next[idx] = { ...b, sortOrder: a.sortOrder };
    next[other] = { ...a, sortOrder: b.sortOrder };
    setRows(next);
    await Promise.all([save(next[idx]), save(next[other])]);
  }

  return (
    <Card>
      <CardHeader><CardTitle>GeoIP sources</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-xs text-muted-foreground">
          Sources are tried in order — first non-empty country wins. Drag-equivalent ↑/↓ buttons reorder.
        </p>
        <ul className="divide-y divide-border">
          {rows.map((src, i) => (
            <li key={src.id} className="grid grid-cols-12 items-center gap-2 py-2 text-sm">
              <div className="col-span-1 flex flex-col">
                <Button size="sm" variant="ghost" disabled={i === 0} onClick={() => reorder(src.id, -1)}>↑</Button>
                <Button size="sm" variant="ghost" disabled={i === rows.length - 1} onClick={() => reorder(src.id, 1)}>↓</Button>
              </div>
              <Badge variant="outline" className="col-span-2 justify-center text-xs">{src.kind}</Badge>
              <Input className="col-span-3" value={src.label}
                     onChange={(e) => setRows((rs) => rs.map((x) => x.id === src.id ? { ...x, label: e.target.value } : x))}
                     onBlur={() => save(src)} />
              <Input className="col-span-3 font-mono text-xs"
                     value={JSON.stringify(src.config)}
                     onChange={(e) => {
                       try {
                         const parsed = JSON.parse(e.target.value);
                         setRows((rs) => rs.map((x) => x.id === src.id ? { ...x, config: parsed } : x));
                       } catch { /* keep typing — apply on blur */ }
                     }}
                     onBlur={() => save(src)} />
              <Switch className="col-span-1"
                      checked={src.active}
                      onCheckedChange={(v) => {
                        setRows((rs) => rs.map((x) => x.id === src.id ? { ...x, active: v } : x));
                        save({ ...src, active: v });
                      }} />
              <div className="col-span-2 flex gap-1">
                <Button size="sm" variant="ghost" onClick={() => test(src)}>Test</Button>
                {src.kind === "mmdb_auto" && <Button size="sm" variant="ghost" onClick={() => refresh(src)}>↻</Button>}
                <Button size="sm" variant="destructive" onClick={() => remove(src)}>×</Button>
              </div>
              <div className="col-span-12 text-xs text-muted-foreground">
                {src.lastStatus || "—"}
                {src.lastUsedAt ? ` · used ${new Date(src.lastUsedAt).toLocaleString()}` : ""}
                {src.lastRefreshedAt ? ` · refreshed ${new Date(src.lastRefreshedAt).toLocaleString()}` : ""}
              </div>
            </li>
          ))}
        </ul>
        <div className="rounded-md border border-border bg-card p-3 space-y-2">
          <p className="text-sm font-medium">Add a source</p>
          <div className="flex flex-wrap gap-2">
            <Input placeholder="Label" value={newLabel} onChange={(e) => setNewLabel(e.target.value)} className="max-w-sm" />
            <select className="rounded border border-border bg-background px-2 py-1 text-sm"
                    value={newKind} onChange={(e) => setNewKind(e.target.value as STGeoIPSourceKind)}>
              <option value="mmdb_auto">mmdb_auto</option>
              <option value="mmdb_file">mmdb_file</option>
              <option value="http_api">http_api</option>
              <option value="request_header">request_header</option>
            </select>
            <Button onClick={add}>Add</Button>
          </div>
          <p className="text-xs text-muted-foreground">Default config will be filled in based on kind; edit inline after creating.</p>
        </div>
      </CardContent>
    </Card>
  );
}
EOF

cat > web/src/pages/admin/st/GeoIP.tsx <<'EOF'
import { useEffect, useState } from "react";
import { GeoIPSourceAdmin } from "@/components/admin/st/GeoIPSourceAdmin";
import { listSTGeoIPSourcesAdmin } from "@/api/stAdmin";
import type { STGeoIPSource } from "@/lib/types";

export function STAdminGeoIP() {
  const [initial, setInitial] = useState<STGeoIPSource[] | null>(null);
  useEffect(() => { listSTGeoIPSourcesAdmin().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">GeoIP sources</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <GeoIPSourceAdmin initial={initial} />}
    </section>
  );
}
EOF

cd web && pnpm exec tsc -b --noEmit && cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/admin/st/GeoIPSourceAdmin.tsx web/src/pages/admin/st/GeoIP.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): speedtest admin GeoIP sources page"
```

---

### Task L3: ResultsTable + Dashboards + 2 pages

**Files:**
- Create: `web/src/components/admin/st/ResultsTable.tsx`
- Create: `web/src/components/admin/st/Dashboards.tsx`
- Create: `web/src/pages/admin/st/Results.tsx`
- Create: `web/src/pages/admin/st/Dashboards.tsx`

```bash
cat > web/src/components/admin/st/ResultsTable.tsx <<'EOF'
import { Card, CardContent } from "@/components/ui/card";
import type { STResult } from "@/lib/types";

type Props = { rows: STResult[] };

export function ResultsTable({ rows }: Props) {
  if (rows.length === 0) {
    return (
      <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">
        No results match the current filters.
      </CardContent></Card>
    );
  }
  return (
    <table className="w-full border-collapse text-sm">
      <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
        <tr>
          <th className="py-2">When</th>
          <th className="py-2">Customer</th>
          <th className="py-2">Endpoint</th>
          <th className="py-2">Strategy</th>
          <th className="py-2 text-right">↓ Mb/s</th>
          <th className="py-2 text-right">↑ Mb/s</th>
          <th className="py-2 text-right">Ping</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id} className="border-t border-border">
            <td className="py-2 font-mono text-xs">{new Date(r.ranAt).toLocaleString()}</td>
            <td className="py-2 font-mono text-xs">{r.customerId}</td>
            <td className="py-2">{r.endpointLabel}</td>
            <td className="py-2 text-xs text-muted-foreground">{r.autoStrategy || "—"}</td>
            <td className="py-2 text-right tabular-nums">{r.downloadMbps.toFixed(1)}</td>
            <td className="py-2 text-right tabular-nums">{r.uploadMbps.toFixed(1)}</td>
            <td className="py-2 text-right tabular-nums">{r.pingMs.toFixed(0)} ms</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
EOF

cat > web/src/components/admin/st/Dashboards.tsx <<'EOF'
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { STDashboardAggregates } from "@/lib/types";

type Props = { data: STDashboardAggregates };

export function Dashboards({ data }: Props) {
  const maxPerDay = Math.max(1, ...data.perDay.map((d) => d.count));
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader><CardTitle>Tests per day (30d)</CardTitle></CardHeader>
        <CardContent>
          {data.perDay.length === 0
            ? <p className="text-sm text-muted-foreground">No data yet.</p>
            : <ul className="space-y-1 text-xs">
                {data.perDay.map((d) => (
                  <li key={d.day} className="flex items-center gap-2">
                    <span className="font-mono text-muted-foreground w-24">{d.day}</span>
                    <div className="h-2 flex-1 rounded-full bg-muted">
                      <div className="h-2 rounded-full bg-accent" style={{ width: `${(d.count / maxPerDay) * 100}%` }} />
                    </div>
                    <span className="w-10 text-right tabular-nums">{d.count}</span>
                  </li>
                ))}
              </ul>}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Median throughput per endpoint (30d)</CardTitle></CardHeader>
        <CardContent>
          {data.perEndpoint.length === 0
            ? <p className="text-sm text-muted-foreground">No data yet.</p>
            : <table className="w-full text-sm">
                <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
                  <tr><th className="py-1">Endpoint</th><th className="py-1 text-right">↓</th><th className="py-1 text-right">↑</th><th className="py-1 text-right">Ping</th><th className="py-1 text-right">N</th></tr>
                </thead>
                <tbody>
                  {data.perEndpoint.map((e) => (
                    <tr key={e.label} className="border-t border-border">
                      <td className="py-1">{e.label}</td>
                      <td className="py-1 text-right tabular-nums">{e.medianDownload.toFixed(1)}</td>
                      <td className="py-1 text-right tabular-nums">{e.medianUpload.toFixed(1)}</td>
                      <td className="py-1 text-right tabular-nums">{e.medianPing.toFixed(0)} ms</td>
                      <td className="py-1 text-right tabular-nums">{e.resultCount}</td>
                    </tr>
                  ))}
                </tbody>
              </table>}
        </CardContent>
      </Card>

      <Card className="md:col-span-2">
        <CardHeader><CardTitle>Slowest results (7d)</CardTitle></CardHeader>
        <CardContent>
          {data.slowTop10.length === 0
            ? <p className="text-sm text-muted-foreground">No slow results — nice.</p>
            : <table className="w-full text-sm">
                <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
                  <tr><th className="py-1">When</th><th className="py-1">Customer</th><th className="py-1">Endpoint</th><th className="py-1 text-right">↓ Mb/s</th></tr>
                </thead>
                <tbody>
                  {data.slowTop10.map((r) => (
                    <tr key={r.id} className="border-t border-border">
                      <td className="py-1 font-mono text-xs">{new Date(r.ranAt).toLocaleString()}</td>
                      <td className="py-1 font-mono text-xs">{r.customerId}</td>
                      <td className="py-1">{r.endpointLabel}</td>
                      <td className="py-1 text-right tabular-nums">{r.downloadMbps.toFixed(1)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>}
        </CardContent>
      </Card>
    </div>
  );
}
EOF

cat > web/src/pages/admin/st/Results.tsx <<'EOF'
import { useEffect, useState } from "react";
import { ResultsTable } from "@/components/admin/st/ResultsTable";
import { Input } from "@/components/ui/input";
import { listSTResultsAdmin } from "@/api/stAdmin";
import type { STResult } from "@/lib/types";

export function STAdminResults() {
  const [rows, setRows] = useState<STResult[]>([]);
  const [customerId, setCustomerId] = useState("");
  const [slowOnly, setSlowOnly] = useState(false);

  useEffect(() => {
    let cancelled = false;
    listSTResultsAdmin({ customerId: customerId || undefined, slowOnly, limit: 200 })
      .then((r) => { if (!cancelled) setRows(r); }).catch(() => {});
    return () => { cancelled = true; };
  }, [customerId, slowOnly]);

  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Speedtest results</h2>
      <div className="flex flex-wrap gap-2 items-center">
        <Input placeholder="Customer id filter…" value={customerId}
               onChange={(e) => setCustomerId(e.target.value)} className="max-w-sm" />
        <label className="text-sm flex items-center gap-1">
          <input type="checkbox" checked={slowOnly} onChange={(e) => setSlowOnly(e.target.checked)} />
          Slow only
        </label>
      </div>
      <ResultsTable rows={rows} />
    </section>
  );
}
EOF

cat > web/src/pages/admin/st/Dashboards.tsx <<'EOF'
import { useEffect, useState } from "react";
import { Dashboards } from "@/components/admin/st/Dashboards";
import { getSTDashboards } from "@/api/stAdmin";
import type { STDashboardAggregates } from "@/lib/types";

export function STAdminDashboards() {
  const [data, setData] = useState<STDashboardAggregates | null>(null);
  useEffect(() => { getSTDashboards().then(setData).catch(() => setData(null)); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Dashboards</h2>
      {data === null
        ? <p className="text-sm text-muted-foreground">Loading…</p>
        : <Dashboards data={data} />}
    </section>
  );
}
EOF

cd web && pnpm exec tsc -b --noEmit && pnpm test
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/admin/st/ResultsTable.tsx web/src/components/admin/st/Dashboards.tsx web/src/pages/admin/st/Results.tsx web/src/pages/admin/st/Dashboards.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): speedtest admin Results + Dashboards pages"
```

---

## Phase M — Final wiring

### Task M1: App.tsx dispatcher for new modes

**Files:**
- Modify: `web/src/App.tsx`

Read the current App.tsx (from KB Unit 26 it already dispatches kb-* modes). Add the 5 new ST cases to the existing switch.

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > web/src/App.tsx <<'EOF'
import type { ReactNode } from "react";
import { Toaster } from "@/components/ui/sonner";
import { readBootstrap } from "@/lib/bootstrap";
import { AdminHome } from "@/pages/AdminHome";
import { CustomerHome } from "@/pages/CustomerHome";
import { KBBrowse } from "@/pages/kb/Browse";
import { KBDetail } from "@/pages/kb/Detail";
import { KBAdminList } from "@/pages/admin/kb/List";
import { KBAdminEdit } from "@/pages/admin/kb/Edit";
import { KBAdminCategories } from "@/pages/admin/kb/Categories";
import { KBAdminTags } from "@/pages/admin/kb/Tags";
import { Speedtest } from "@/pages/st/Speedtest";
import { STAdminEndpoints } from "@/pages/admin/st/Endpoints";
import { STAdminGeoIP } from "@/pages/admin/st/GeoIP";
import { STAdminResults } from "@/pages/admin/st/Results";
import { STAdminDashboards } from "@/pages/admin/st/Dashboards";

export function App() {
  const bootstrap = readBootstrap();
  let page: ReactNode;
  switch (bootstrap.mode) {
    case "admin-home":           page = <AdminHome bootstrap={bootstrap} />; break;
    case "kb-browse":            page = <KBBrowse bootstrap={bootstrap} />; break;
    case "kb-detail":            page = <KBDetail />; break;
    case "admin-kb-list":        page = <KBAdminList />; break;
    case "admin-kb-edit":        page = <KBAdminEdit />; break;
    case "admin-kb-categories":  page = <KBAdminCategories />; break;
    case "admin-kb-tags":        page = <KBAdminTags />; break;
    case "speedtest":            page = <Speedtest />; break;
    case "admin-st-endpoints":   page = <STAdminEndpoints />; break;
    case "admin-st-geoip":       page = <STAdminGeoIP />; break;
    case "admin-st-results":     page = <STAdminResults />; break;
    case "admin-st-dashboards":  page = <STAdminDashboards />; break;
    default:                     page = <CustomerHome bootstrap={bootstrap} />;
  }
  return (
    <>
      {page}
      <Toaster />
    </>
  );
}
EOF

cd web && pnpm test && pnpm build
cd ..
go build ./...

git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/App.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): App dispatches speedtest + admin-st-* modes"
```

### Task M2: Flip SHIPPED_MODULES.speedtest

**Files:**
- Modify: `web/src/lib/modules.ts`

```bash
sed -i 's/speedtest: false/speedtest: true/' web/src/lib/modules.ts
grep 'speedtest:' web/src/lib/modules.ts   # confirm true
```

(`Modules.Speedtest = true` default was already set in Task G4.)

### Task M3: README + final smoke

**Files:**
- Modify: `README.md`

```bash
cat > README.md <<'EOF'
# Continuum Support Plugin

`continuum.support` is the customer-facing support surface for a
Continuum deployment.

**Shipped modules:**

| Module | Status |
|---|---|
| Knowledge Base | Shipped (v0.2) |
| Speedtest | Shipped (v0.3) |
| Tickets | Coming soon |
| AI Assistance | Coming soon |

See `docs/superpowers/specs/` for designs and `docs/superpowers/plans/`
for implementation plans.

## Build

```sh
make build
make test
```

`make test` runs Go tests + the vitest SPA suite. Some integration
tests are gated on `PG_DSN` (a Postgres DSN); without it those tests
skip cleanly.

## Configuration

Requires `database_url` — a Postgres DSN, e.g.
`postgres://plugin_support:...@host:5432/continuum?search_path=support&sslmode=disable`.

The plugin manages its own schema; the operator only needs to create
the schema and grant connect rights.

Speedtest-related config keys (all optional, sane defaults):

- `auto_strategy` — `latency` (default) or `geoip`
- `client_ip_storage` — `truncated` (default) or `off`
- `slow_threshold_mbps` — default `5`
- `geoip_cache_dir` — default `$XDG_CACHE_HOME/continuum-plugin-support/geoip/`

## Events emitted

Routed via the existing `continuum.notifications` plugin per admin rules.

**KB:**
- `plugin.continuum.support.kb_article_published`
- `plugin.continuum.support.kb_article_updated`
- `plugin.continuum.support.kb_article_unhelpful`

**Speedtest:**
- `plugin.continuum.support.speedtest_run`
- `plugin.continuum.support.speedtest_slow`

## Crons

- **KB cron** (publish-due + unhelpful sweep): `POST /api/admin/kb/cron/run`
- **GeoIP mmdb refresh** is automatic on plugin start (background); manual trigger: `POST /api/admin/speedtest/geoip/{id}/refresh`
EOF
```

```bash
make build
make test
```

### Task M4: Commit + push

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/lib/modules.ts README.md
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat: ship speedtest — flip SHIPPED_MODULES.speedtest + README"
git log --oneline | head -5
git push 2>&1 | tail -5
```

---

## Self-Review

**Spec coverage check** (against `2026-05-21-support-speedtest-design.md`):

- All 3 ST tables (`st_endpoints`, `st_geoip_sources`, `st_results`) + indexes + db-ip seed → Task A1.
- IP truncation (/24 v4, /48 v6, off) → Task A3.
- GeoIP source kinds (mmdb_auto / mmdb_file / http_api / request_header) → Tasks B3-B6.
- Chain walker (sort_order, first-non-empty wins, status tracking) → Task B1.
- mmdb_auto download lifecycle (gunzip, atomic swap, prev-month fallback) → Task B6.
- Source factory keyed by row kind → Task B7.
- Endpoints / geoip / results CRUD → Tasks C1-C3.
- Auto resolver (latency candidates / geoip match / fallback) → Task D1.
- Customer handlers (browse page + endpoints + auto + result save + history + 60s rate limit + event emission) → Tasks E1+E2.
- Admin handlers (endpoints + geoip + refresh + test + results + dashboards) → Task F1.
- Routes + manifest 0.3.0 + main.go wiring (chain construction + downloader + resolver) → Tasks G1-G5.
- Server integration tests (auth gates + endpoint CRUD + rate-limit 429 + auto latency) → Task H1.
- SPA types + bootstrap modes + API clients → Tasks I1+I2.
- LibreSpeed worker vendored + typed TS wrapper → Tasks J1+J2.
- Customer UI (EndpointPicker + SpeedGauge + HistoryList + Speedtest page with state machine) → Tasks K1+K2.
- Admin UI (Endpoints + GeoIPSourceAdmin + ResultsTable + Dashboards + 4 pages) → Tasks L1+L2+L3.
- App dispatcher for 5 new modes → Task M1.
- SHIPPED_MODULES.speedtest flip + Modules.Speedtest default → Tasks M2 + G4.

**Coverage gap noted:**

- The spec calls for a customer-side latency probe (parallel HEAD pings against `/empty.php` for each candidate) before running the test in latency mode. The current Speedtest page implementation in Task K2 falls back to "first candidate" if `autoRes.endpoint` is null, with a code comment noting "a future iteration can run the probe here." This is a documented v1 simplification — a follow-up should add the actual probe.

- The spec's `countryHits` dashboard aggregate is wired in `STDashboardAggregates` but the store implementation (Task C3) returns an empty slice with a comment explaining geoip resolution happens at request time, not save time. A column would need adding to `st_results` to populate this. Documented as future work in the store code.

Both gaps are explicit "v1 simplifications" with clear paths forward, not silent omissions.

**Placeholder scan:** searched the plan for "TODO" / "TBD" / "implement later" / "Similar to Task" — none present.

**Type consistency:**

- Go: `STEndpoint`, `STGeoIPSource`, `STResult`, `STDashboardAggregates`, `AutoResolution` consistent across types file → store methods → handlers → tests.
- Go store methods consistently prefixed `ST*`.
- Event names: `speedtest_run` / `speedtest_slow` match the spec.
- TS types mirror Go shapes via JSON-tag camelCase (`downloadMbps`, `ranAt`, `clientIp`, etc.).
- Bootstrap modes: `speedtest` / `admin-st-endpoints` / `admin-st-geoip` / `admin-st-results` / `admin-st-dashboards` — match Go side and the App.tsx switch.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-21-support-speedtest.md`. Two execution options:

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, two-stage review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using `executing-plans`, batch execution with checkpoints.

Which approach?
