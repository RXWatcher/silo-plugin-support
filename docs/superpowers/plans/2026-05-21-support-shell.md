# Support Plugin Shell — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundation of `silo-plugin-support` — manifest, auth-gated routes, customer + admin SPA shells, migrations runner, `app_config` singleton — with no module business logic. Future modules (KB, Speedtest, Tickets, AI) land on top.

**Architecture:** Go binary + embedded React/TS SPA, served through the Silo SDK's `HttpRoutes` capability (no standalone listener). Mirrors `silo-plugin-public-catalog` exactly — same toolchain, same patterns. One Postgres schema (`support`) with `golang-migrate`-managed migrations; one singleton `app_config` table holding module toggles in JSONB.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5, golang-migrate, hashicorp/go-hclog, ContinuumApp/continuum-plugin-sdk v0.3.10. Frontend: React 19, TypeScript, Vite 8, Vitest 4, Tailwind v4, radix-ui 1.4, lucide-react, sonner. Package manager: pnpm.

**Reference repo:** `/opt/silo_plugins/silo-plugin-public-catalog/` — when a task says "match public-catalog's `X`," the engineer should open that file and adapt. Strong preference: identical patterns reduce review burden.

**Spec lineage:**
- Program: [`../specs/2026-05-21-support-plugin-program-design.md`](../specs/2026-05-21-support-plugin-program-design.md)
- Shell: [`../specs/2026-05-21-support-shell-design.md`](../specs/2026-05-21-support-shell-design.md)

---

## File Structure

All paths relative to `/opt/silo_plugins/silo-plugin-support/`.

| File | Responsibility |
|---|---|
| `go.mod`, `go.sum` | Module path `github.com/RXWatcher/silo-plugin-support`, deps match public-catalog |
| `Makefile` | `build` / `web-build` / `test` / `test-go` / `test-web` / `clean` |
| `README.md` | One-paragraph overview, links to spec docs |
| `.gitignore` | Built binary, web/node_modules, web/dist, web/*.tsbuildinfo, web/vite.config.{js,d.ts}, internal/server/public/dist |
| `cmd/silo-plugin-support/main.go` | Entry point: load manifest, build server, hand off to SDK |
| `cmd/silo-plugin-support/manifest.json` | Plugin manifest — routes + global_config_schema |
| `internal/runtime/runtime.go` | Plugin SDK `runtime.Server`: Configure RPC, Config struct, normalization |
| `internal/runtime/runtime_test.go` | Configure roundtrip + validation tests |
| `internal/httproutes/server.go` | SDK `HttpRoutes` shim — strips `X-Silo-*` on `ServeHTTP`, forwards to wrapped handler |
| `internal/server/server.go` | chi router + `Deps` + `New(Deps) http.Handler` |
| `internal/server/middleware.go` | `securityHeaders`, `requireUser`, `requireAdmin` |
| `internal/server/response.go` | `writeJSON`, `writeErr`, `writeInternal` |
| `internal/server/spa.go` | `//go:embed public/dist/*` + bootstrap render + theme escape |
| `internal/server/handlers_customer.go` | `GET /` SPA shell, `GET /api/customer/bootstrap` |
| `internal/server/handlers_admin.go` | `GET /admin` SPA shell, `GET/PATCH /api/admin/config` |
| `internal/server/server_test.go` | Middleware + handler + config CRUD tests |
| `internal/store/store.go` | `pgxpool` wrapper, `Store` struct |
| `internal/store/config.go` | `GetConfig` / `UpdateConfig` / `Bootstrap` against `app_config` |
| `internal/store/types.go` | DB types (if any beyond Config) |
| `internal/migrate/runner.go` | `Run(ctx, dsn)` — golang-migrate driver |
| `internal/migrate/files/0001_init.up.sql` | Creates `app_config` singleton |
| `internal/migrate/files/0001_init.down.sql` | Drops `app_config` |
| `web/package.json`, `pnpm-lock.yaml` | Frontend deps + scripts |
| `web/index.html` | Vite entry, bootstrap script placeholder |
| `web/vite.config.ts`, `tsconfig*.json` | Vite + TS config (emits to `../internal/server/public/dist`) |
| `web/src/main.tsx` | React root, `captureTokenFromURL` |
| `web/src/App.tsx` | Bootstrap-mode dispatcher (`customer-home` / `admin-home`) |
| `web/src/index.css` | Tailwind v4 setup, design tokens, theme variables |
| `web/src/vite-env.d.ts` | Vite ambient types |
| `web/src/lib/bootstrap.ts` | Read & parse `#support-bootstrap` JSON |
| `web/src/lib/bootstrap.test.ts` | Parse + default tests |
| `web/src/lib/api.ts` | `api<T>(path, init)` + `absoluteURL` |
| `web/src/lib/authToken.ts` | `captureTokenFromURL` + `authHeaders` |
| `web/src/lib/mountPath.ts` | `mountPath()` from `window.location.pathname` |
| `web/src/lib/types.ts` | Shared TS shapes (Config, Bootstrap, ModuleToggles) |
| `web/src/lib/utils.ts` | `cn()` Tailwind className merger |
| `web/src/lib/section.ts` | `?section=` URL state read/write helpers |
| `web/src/lib/section.test.ts` | Section helper tests |
| `web/src/api/admin.ts` | `getConfig`, `updateConfig` typed wrappers |
| `web/src/pages/CustomerHome.tsx` | Module-card grid |
| `web/src/pages/AdminHome.tsx` | Renders `<AdminLayout>` |
| `web/src/components/ui/*` | shadcn primitives copied from public-catalog: button, card, switch, label, badge, skeleton, sonner |
| `web/src/components/shared/TopBar.tsx` | Shared page header |
| `web/src/components/shared/ModuleCard.tsx` | Customer-side module entry card |
| `web/src/components/shared/ModuleCard.test.tsx` | Render tests |
| `web/src/components/admin/AdminLayout.tsx` | Sidebar + main + `?section=` URL state |
| `web/src/components/admin/AdminLayout.test.tsx` | Section-state tests |
| `web/src/components/admin/AdminSidebar.tsx` | Sidebar entries (built-in + per-module) |
| `web/src/components/admin/AdminOverview.tsx` | System status row + module status grid |
| `web/src/components/admin/AdminConfig.tsx` | Hosts `<ModuleTogglesPanel>` + placeholder for future settings |
| `web/src/components/admin/ModuleStatusCard.tsx` | Admin-side per-module status card |
| `web/src/components/admin/ModuleStatusCard.test.tsx` | Render tests |
| `web/src/components/admin/ModuleTogglesPanel.tsx` | Four toggle switches + save |
| `web/src/components/admin/ModuleTogglesPanel.test.tsx` | Toggle → onSave wired |

---

## Phase A — Repo + Go Scaffolding

### Task A1: Initialize go.mod and add to workspace

**Files:**
- Create: `go.mod`
- Modify: `/opt/silo_plugins/go.work`

- [ ] **Step 1: Create go.mod**

```bash
cd /opt/silo_plugins/silo-plugin-support
cat > go.mod <<'EOF'
module github.com/RXWatcher/silo-plugin-support

go 1.26.0

require (
	github.com/ContinuumApp/continuum-plugin-sdk v0.3.10
	github.com/go-chi/chi/v5 v5.2.5
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/hashicorp/go-hclog v1.6.3
	github.com/jackc/pgx/v5 v5.9.2
	google.golang.org/protobuf v1.36.11
)
EOF
```

- [ ] **Step 2: Add to workspace**

Append `./silo-plugin-support` to `/opt/silo_plugins/go.work`'s `use ( ... )` block, keeping alphabetical order (between `stream-dashboard` and `whmcs-login`).

- [ ] **Step 3: Resolve deps**

```bash
cd /opt/silo_plugins/silo-plugin-support
go mod tidy
```

Expected: writes `go.sum`, no errors. The SDK resolves locally via `go.work`. Verify with `go list -m github.com/ContinuumApp/continuum-plugin-sdk`.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: initialize go.mod for support plugin"
```

(The `go.work` change at workspace root isn't inside the plugin repo — note it as a separate workspace-level edit.)

---

### Task A2: Create manifest.json

**Files:**
- Create: `cmd/silo-plugin-support/manifest.json`

- [ ] **Step 1: Write the manifest**

```bash
mkdir -p cmd/silo-plugin-support
cat > cmd/silo-plugin-support/manifest.json <<'EOF'
{
  "plugin_id": "silo.support",
  "version": "0.1.0",
  "checksum": "__CHECKSUM__",
  "silo_api_version": "v1",
  "category": "Operations",
  "supported_platforms": [
    { "os": "linux", "arch": "amd64" }
  ],
  "capabilities": [
    {
      "type": "http_routes.v1",
      "id": "support",
      "display_name": "Support",
      "description": "Customer support shell — modules ship in follow-up releases."
    }
  ],
  "http_routes": [
    { "id": "customer_home", "method": "GET", "path": "/", "access": "user",
      "navigable": true, "navigation_label": "Support", "navigation_kind": "user" },
    { "id": "customer_bootstrap", "method": "GET", "path": "/api/customer/bootstrap", "access": "user" },
    { "id": "admin_page", "method": "GET", "path": "/admin", "access": "admin",
      "navigable": true, "navigation_label": "Support", "navigation_kind": "admin" },
    { "id": "admin_get_config", "method": "GET", "path": "/api/admin/config", "access": "admin" },
    { "id": "admin_patch_config", "method": "PATCH", "path": "/api/admin/config", "access": "admin" }
  ],
  "global_config_schema": [
    {
      "key": "database_url",
      "title": "Postgres connection string",
      "description": "DSN for the dedicated support schema (e.g. postgres://plugin_support:...@host:5432/silo?search_path=support&sslmode=disable).",
      "json_schema": "{\"type\":\"object\",\"properties\":{\"value\":{\"type\":\"string\"}},\"required\":[\"value\"]}",
      "required": true,
      "admin_form": {
        "fields": [
          { "key": "value", "label": "Connection URL", "control": "ADMIN_FORM_CONTROL_PASSWORD",
            "required": true, "secret": true }
        ]
      }
    }
  ]
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add cmd/silo-plugin-support/manifest.json
git commit -m "chore: add plugin manifest"
```

---

### Task A3: Skeleton main.go — load manifest, serve "not configured" until Configure RPC

**Files:**
- Create: `cmd/silo-plugin-support/main.go`

- [ ] **Step 1: Write the skeleton**

```go
package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	goruntime "runtime"

	"github.com/hashicorp/go-hclog"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/manifest"
	sdkruntime "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/runtime"

	"github.com/RXWatcher/silo-plugin-support/internal/httproutes"
)

//go:embed manifest.json
var manifestRaw []byte

func main() {
	logger := hclog.New(&hclog.LoggerOptions{Name: "silo-plugin-support"})
	manifest, err := loadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load manifest: %v\n", err)
		os.Exit(1)
	}

	httpSrv := httproutes.NewServer()
	_ = manifest
	_ = logger

	sdkruntime.Serve(sdkruntime.ServeConfig{
		Logger: logger,
		Servers: sdkruntime.CapabilityServers{
			HttpRoutes: httpSrv,
		},
	})
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestRaw)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	binaryData, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}
	checksum := sha256.Sum256(binaryData)
	manifest.Checksum = hex.EncodeToString(checksum[:])
	if len(manifest.GetSupportedPlatforms()) == 0 {
		manifest.SupportedPlatforms = []*pluginv1.SupportedPlatform{
			{Os: goruntime.GOOS, Arch: goruntime.GOARCH},
		}
	}
	return manifest, nil
}
```

This won't compile until Task A4 creates `internal/httproutes`.

- [ ] **Step 2: Don't commit yet — depends on Task A4 to compile.**

---

### Task A4: Minimal `httproutes` shim so main.go compiles

**Files:**
- Create: `internal/httproutes/server.go`

Copy from public-catalog verbatim and re-namespace.

- [ ] **Step 1: Write the shim**

```bash
mkdir -p internal/httproutes
cat > internal/httproutes/server.go <<'EOF'
package httproutes

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
)

type Server struct {
	pluginv1.UnimplementedHttpRoutesServer
	handler atomic.Pointer[http.Handler]
}

func NewServer() *Server { return &Server{} }

func (s *Server) SetHandler(h http.Handler) {
	if h == nil {
		s.handler.Store(nil)
		return
	}
	s.handler.Store(&h)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hPtr := s.handler.Load()
	if hPtr == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":"not_ready","message":"plugin not configured"}}`))
		return
	}
	for k := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-silo-") {
			r.Header.Del(k)
		}
	}
	(*hPtr).ServeHTTP(w, r)
}

func (s *Server) Handle(_ context.Context, req *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	hPtr := s.handler.Load()
	if hPtr == nil {
		return &pluginv1.HandleHTTPResponse{
			StatusCode: http.StatusServiceUnavailable,
			Body:       []byte(`{"error":{"code":"not_ready","message":"plugin not configured"}}`),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	rawQuery := ""
	if req.GetQuery() != nil {
		vals := url.Values{}
		for k, v := range req.GetQuery().GetFields() {
			if sv := v.GetStringValue(); sv != "" {
				vals.Set(k, sv)
			} else {
				vals.Set(k, v.String())
			}
		}
		rawQuery = vals.Encode()
	}
	method := req.GetMethod()
	if method == "" {
		method = http.MethodGet
	}
	httpReq := httptest.NewRequest(method, (&url.URL{Path: req.GetPath(), RawQuery: rawQuery}).String(), bytes.NewReader(req.GetBody()))
	for k, v := range req.GetHeaders() {
		httpReq.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	(*hPtr).ServeHTTP(rec, httpReq)
	body, _ := io.ReadAll(rec.Result().Body)
	headers := map[string]string{}
	for k, vs := range rec.Header() {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}
	return &pluginv1.HandleHTTPResponse{StatusCode: int32(rec.Code), Headers: headers, Body: body}, nil
}
EOF
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: succeeds.

- [ ] **Step 3: Commit A3 + A4 together**

```bash
git add cmd/ internal/httproutes/
git commit -m "feat: bootstrap main.go and SDK http_routes shim"
```

---

### Task A5: Makefile + .gitignore

**Files:**
- Create: `Makefile`
- Create: `.gitignore`

- [ ] **Step 1: Write Makefile**

```bash
cat > Makefile <<'EOF'
BINARY := silo-plugin-support
GO ?= go
PNPM ?= pnpm

.PHONY: build web-deps web-build test test-go test-web clean

build: web-build
	$(GO) build -o $(BINARY) ./cmd/silo-plugin-support

web-deps:
	cd web && $(PNPM) install --frozen-lockfile

web-build: web-deps
	cd web && $(PNPM) build

test: test-go test-web

test-go:
	$(GO) test ./...

test-web:
	cd web && $(PNPM) run test

clean:
	rm -f $(BINARY)
	rm -rf web/node_modules internal/server/public/dist
EOF
```

- [ ] **Step 2: Write .gitignore**

```bash
cat > .gitignore <<'EOF'
/silo-plugin-support
/web/node_modules
/web/dist
/web/*.tsbuildinfo
/web/vite.config.js
/web/vite.config.d.ts
/internal/server/public/dist
EOF
```

- [ ] **Step 3: Commit**

```bash
git add Makefile .gitignore
git commit -m "chore: add Makefile and .gitignore"
```

---

## Phase B — Runtime Config

### Task B1: Write runtime config struct + Configure RPC (TDD)

**Files:**
- Create: `internal/runtime/runtime.go`
- Create: `internal/runtime/runtime_test.go`

- [ ] **Step 1: Write failing test for missing DSN + defaults**

```bash
mkdir -p internal/runtime
cat > internal/runtime/runtime_test.go <<'EOF'
package runtime

import (
	"testing"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestConfigureRejectsMissingDatabaseURL(t *testing.T) {
	s := New(nil, nil)
	req := &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{}}
	if _, err := s.Configure(t.Context(), req); err == nil {
		t.Fatal("expected missing database_url to fail; got nil")
	}
}

func TestConfigureDefaultsAllModulesOff(t *testing.T) {
	var observed Config
	s := New(nil, func(cfg Config) error {
		observed = cfg
		return nil
	})
	if _, err := s.Configure(t.Context(), configureRequest()); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if observed.Modules.KB || observed.Modules.Speedtest || observed.Modules.Tickets || observed.Modules.AI {
		t.Fatalf("modules should default off; got %+v", observed.Modules)
	}
	if observed.DatabaseURL != "postgres://x" {
		t.Fatalf("DatabaseURL = %q, want postgres://x", observed.DatabaseURL)
	}
}

func configureRequest() *pluginv1.ConfigureRequest {
	db, err := structpb.NewStruct(map[string]any{"value": "postgres://x"})
	if err != nil {
		panic(err)
	}
	return &pluginv1.ConfigureRequest{
		Config: []*pluginv1.ConfigEntry{
			{Key: "database_url", Value: db},
		},
	}
}
EOF
```

- [ ] **Step 2: Run test, expect compile fail**

```bash
go test ./internal/runtime/...
```

Expected: fails ("undefined: New" / "undefined: Config").

- [ ] **Step 3: Write minimal runtime.go**

```bash
cat > internal/runtime/runtime.go <<'EOF'
// Package runtime is the SDK-facing plugin runtime: it owns the
// GetManifest / Configure RPC and hands a normalized Config to
// main.go's onConfig callback.
package runtime

import (
	"context"
	"fmt"
	"sync"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/runtimedefault"
)

// Config is the union of manifest-supplied and DB-persisted plugin
// settings. DatabaseURL is manifest-only; everything else round-trips
// through app_config.data as JSONB.
type Config struct {
	DatabaseURL string        `json:"-"`
	Modules     ModuleToggles `json:"modules"`
}

// ModuleToggles controls which modules are exposed in the UI. All
// default off; each module's release flips its own toggle to true in
// DefaultAppConfig and adds its routes to the manifest.
type ModuleToggles struct {
	KB        bool `json:"kb"`
	Speedtest bool `json:"speedtest"`
	Tickets   bool `json:"tickets"`
	AI        bool `json:"ai"`
}

type Server struct {
	runtimedefault.Server
	manifest *pluginv1.PluginManifest
	onConfig func(Config) error

	mu  sync.RWMutex
	cfg Config
}

func New(manifest *pluginv1.PluginManifest, onConfig func(Config) error) *Server {
	return &Server{manifest: manifest, onConfig: onConfig}
}

func (s *Server) GetManifest(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

// DefaultAppConfig returns the in-code defaults applied when no DB
// row exists yet. Each module ship flips its own toggle to true.
func DefaultAppConfig() Config {
	return Config{Modules: ModuleToggles{}}
}

// NormalizeAppConfig validates a Config and returns it. Validation
// is minimal at shell time — module-specific validation lives in
// module specs.
func NormalizeAppConfig(cfg Config) (Config, error) {
	return cfg, nil
}

func (s *Server) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	cfg := DefaultAppConfig()
	for _, e := range req.GetConfig() {
		if e.GetValue() == nil {
			continue
		}
		m := e.GetValue().AsMap()
		switch e.GetKey() {
		case "database_url":
			cfg.DatabaseURL = stringValue(m["value"], firstString(m))
		}
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database_url is required")
	}
	var err error
	cfg, err = NormalizeAppConfig(cfg)
	if err != nil {
		return nil, err
	}
	if s.onConfig != nil {
		if err := s.onConfig(cfg); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

func stringValue(candidates ...any) string {
	for _, c := range candidates {
		if s, ok := c.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func firstString(m map[string]any) any {
	for _, v := range m {
		if _, ok := v.(string); ok {
			return v
		}
	}
	return nil
}
EOF
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/runtime/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/
git commit -m "feat(runtime): Configure RPC + Config struct + module toggles"
```

---

## Phase C — Store + Migrations

### Task C1: Write the initial migration

**Files:**
- Create: `internal/migrate/runner.go`
- Create: `internal/migrate/files/0001_init.up.sql`
- Create: `internal/migrate/files/0001_init.down.sql`

- [ ] **Step 1: Write migration files**

```bash
mkdir -p internal/migrate/files
cat > internal/migrate/files/0001_init.up.sql <<'EOF'
CREATE TABLE app_config (
    id          SMALLINT PRIMARY KEY CHECK (id = 1),
    data        JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO app_config (id, data) VALUES (1, '{}'::jsonb)
ON CONFLICT (id) DO NOTHING;
EOF

cat > internal/migrate/files/0001_init.down.sql <<'EOF'
DROP TABLE IF EXISTS app_config;
EOF
```

- [ ] **Step 2: Write runner**

```bash
cat > internal/migrate/runner.go <<'EOF'
// Package migrate runs the plugin's schema migrations on start.
// Files are embedded so the binary is self-contained.
package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed files/*.sql
var migrationsFS embed.FS

// Run applies every pending migration against the database at dsn.
// Returns nil on success (including "no change").
func Run(ctx context.Context, dsn string) error {
	src, err := iofs.New(migrationsFS, "files")
	if err != nil {
		return fmt.Errorf("load migrations FS: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("open migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	_ = ctx // reserved for future cancellation
	return nil
}
EOF
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/migrate/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/migrate/
git commit -m "feat(migrate): app_config initial migration + runner"
```

---

### Task C2: Store wrapper with GetConfig / UpdateConfig / Bootstrap

**Files:**
- Create: `internal/store/store.go`, `internal/store/config.go`, `internal/store/types.go`

- [ ] **Step 1: Write store.go**

```bash
mkdir -p internal/store
cat > internal/store/store.go <<'EOF'
// Package store wraps the pgxpool used by the plugin and exposes
// typed accessors for app_config.
package store

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }
EOF
```

- [ ] **Step 2: Write config.go**

```bash
cat > internal/store/config.go <<'EOF'
package store

import (
	"context"
	"encoding/json"
	"fmt"

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
)

// GetConfig reads the singleton app_config row. Returns the
// in-code default if the row is empty or missing.
func (s *Store) GetConfig(ctx context.Context) (pluginrt.Config, error) {
	var data []byte
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM app_config WHERE id = 1`).Scan(&data)
	if err != nil {
		return pluginrt.DefaultAppConfig(), fmt.Errorf("read app_config: %w", err)
	}
	cfg := pluginrt.DefaultAppConfig()
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return pluginrt.DefaultAppConfig(), fmt.Errorf("parse app_config.data: %w", err)
		}
	}
	return cfg, nil
}

// UpdateConfig persists the JSONB shape of cfg into the singleton
// row. DatabaseURL is never persisted (it's manifest-only).
func (s *Store) UpdateConfig(ctx context.Context, cfg pluginrt.Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal app_config: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE app_config SET data = $1, updated_at = NOW() WHERE id = 1`, data)
	if err != nil {
		return fmt.Errorf("update app_config: %w", err)
	}
	return nil
}

// Bootstrap merges manifest-supplied cfg with whatever is already
// persisted, applies in-code defaults, normalises, and persists the
// result. Returns the canonical config that survives reinstalls.
func (s *Store) Bootstrap(ctx context.Context, cfg pluginrt.Config) (pluginrt.Config, error) {
	stored, err := s.GetConfig(ctx)
	if err != nil {
		stored = pluginrt.DefaultAppConfig()
	}
	merged := stored
	merged.DatabaseURL = cfg.DatabaseURL
	merged, err = pluginrt.NormalizeAppConfig(merged)
	if err != nil {
		return pluginrt.Config{}, err
	}
	if err := s.UpdateConfig(ctx, merged); err != nil {
		return pluginrt.Config{}, err
	}
	return merged, nil
}
EOF
```

- [ ] **Step 3: Write types.go**

```bash
cat > internal/store/types.go <<'EOF'
package store

// Types shared across module-specific store files land here as
// modules ship. The shell defines none on its own.
EOF
```

- [ ] **Step 4: Verify build**

```bash
go build ./internal/store/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): app_config GetConfig/UpdateConfig/Bootstrap"
```

---

## Phase D — Server Core

### Task D1: writeJSON / writeErr / writeInternal

**Files:**
- Create: `internal/server/response.go`

- [ ] **Step 1: Write file**

```bash
mkdir -p internal/server
cat > internal/server/response.go <<'EOF'
package server

import (
	"encoding/json"
	"net/http"

	"github.com/hashicorp/go-hclog"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func writeInternal(w http.ResponseWriter, r *http.Request, d Deps, code string, err error) {
	if d.Logger != nil {
		d.Logger.Error("internal error", "code", code, "err", err, "path", r.URL.Path)
	} else {
		hclog.L().Error("internal error", "code", code, "err", err, "path", r.URL.Path)
	}
	writeErr(w, http.StatusInternalServerError, code, "internal error")
}
EOF
```

Depends on `Deps` (Task E1). Don't compile in isolation — wait for E1.

- [ ] **Step 2: No commit yet — depends on Task E1.**

---

### Task D2: Auth + security headers middleware (TDD)

**Files:**
- Create: `internal/server/middleware.go`
- Create: `internal/server/middleware_test.go`

- [ ] **Step 1: Write failing tests**

```bash
cat > internal/server/middleware_test.go <<'EOF'
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireUserBlocksMissingIdentity(t *testing.T) {
	called := false
	h := requireUser(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("inner handler must not run when identity is missing")
	}
}

func TestRequireUserAllowsKnownIdentity(t *testing.T) {
	called := false
	h := requireUser(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Silo-User-Id", "42")
	rec := httptest.NewRecorder()
	h(rec, req)
	if !called {
		t.Fatal("inner handler must run for authenticated user")
	}
}

func TestRequireAdminBlocksNonAdmin(t *testing.T) {
	called := false
	h := requireAdmin(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Silo-User-Id", "42")
	req.Header.Set("X-Silo-User-Role", "user")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if called {
		t.Fatal("inner handler must not run for non-admin")
	}
}

func TestRequireAdminAllowsAdmin(t *testing.T) {
	called := false
	h := requireAdmin(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Silo-User-Id", "1")
	req.Header.Set("X-Silo-User-Role", "admin")
	rec := httptest.NewRecorder()
	h(rec, req)
	if !called {
		t.Fatal("inner handler must run for admin")
	}
}

func TestSecurityHeadersApplyOnEveryResponse(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := securityHeaders(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}
EOF
```

- [ ] **Step 2: Write middleware**

```bash
cat > internal/server/middleware.go <<'EOF'
package server

import "net/http"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Silo-User-Id") == "" {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "log in to continue")
			return
		}
		next(w, r)
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Silo-User-Id") == "" {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "admin login required")
			return
		}
		if r.Header.Get("X-Silo-User-Role") != "admin" {
			writeErr(w, http.StatusForbidden, "forbidden", "admin access required")
			return
		}
		next(w, r)
	}
}
EOF
```

- [ ] **Step 3: No commit yet — package compiles only after E1.**

---

### Task D3: SPA shell rendering (`spa.go`) + placeholder index

**Files:**
- Create: `internal/server/spa.go`
- Create: `internal/server/public/dist/index.html` (placeholder so `//go:embed` succeeds before `pnpm build` runs)

- [ ] **Step 1: Write spa.go**

```bash
cat > internal/server/spa.go <<'EOF'
package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
)

//go:embed public/dist/* public/dist/assets/*
var publicSPA embed.FS

type supportBootstrap struct {
	Mode    string                 `json:"mode"`
	Theme   string                 `json:"theme"`
	Modules pluginrt.ModuleToggles `json:"modules"`
	UserID  string                 `json:"userId"`
	IsAdmin bool                   `json:"isAdmin"`
}

func hPublicAsset() http.HandlerFunc {
	dist, err := fs.Sub(publicSPA, "public/dist")
	if err != nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, nil)
		}
	}
	handler := http.StripPrefix("/", http.FileServer(http.FS(dist)))
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

func writeSPA(w http.ResponseWriter, r *http.Request, bs supportBootstrap, status int) {
	if bs.Theme == "" {
		bs.Theme = "midnight-cinema"
	}
	index, err := publicSPA.ReadFile("public/dist/index.html")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "spa_unavailable", "support app has not been built")
		return
	}
	rawBootstrap, err := json.Marshal(bs)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "bootstrap_failed", "failed to render bootstrap")
		return
	}
	index = bytes.Replace(index, []byte("%SUPPORT_BOOTSTRAP%"), rawBootstrap, 1)
	index = bytes.Replace(index, []byte(`<html lang="en">`),
		[]byte(`<html lang="en" data-theme="`+html.EscapeString(bs.Theme)+`">`), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(index)
}

func adminTheme(r *http.Request) string {
	theme := r.URL.Query().Get("theme")
	if theme == "" {
		theme = r.Header.Get("X-Silo-Theme")
	}
	if theme == "" {
		theme = r.Header.Get("X-Silo-User-Theme")
	}
	if theme == "" {
		theme = "default"
	}
	return html.EscapeString(theme)
}
EOF
```

- [ ] **Step 2: Write placeholder index.html for the embed**

The real `index.html` ships from Vite (Phase F). For now the placeholder lets `go build` succeed. `internal/server/public/dist/` is gitignored.

```bash
mkdir -p internal/server/public/dist/assets
touch internal/server/public/dist/assets/.gitkeep
cat > internal/server/public/dist/index.html <<'EOF'
<!doctype html>
<html lang="en"><head><meta charset="UTF-8"></head>
<body><script id="support-bootstrap" type="application/json">%SUPPORT_BOOTSTRAP%</script>
<div id="root"></div></body></html>
EOF
```

- [ ] **Step 3: No commit — still depends on E1.**

---

### Task D4: Customer handlers

**Files:**
- Create: `internal/server/handlers_customer.go`

- [ ] **Step 1: Write file**

```bash
cat > internal/server/handlers_customer.go <<'EOF'
package server

import "net/http"

func hCustomerHome(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modules := currentModules(r.Context(), d)
		writeSPA(w, r, supportBootstrap{
			Mode:    "customer-home",
			Theme:   adminTheme(r),
			Modules: modules,
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: r.Header.Get("X-Silo-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hCustomerBootstrap(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modules := currentModules(r.Context(), d)
		writeJSON(w, http.StatusOK, map[string]any{
			"modules": modules,
			"userId":  r.Header.Get("X-Silo-User-Id"),
			"isAdmin": r.Header.Get("X-Silo-User-Role") == "admin",
		})
	}
}
EOF
```

- [ ] **Step 2: No commit — wait for E1.**

---

### Task D5: Admin handlers — bootstrap, GET/PATCH config

**Files:**
- Create: `internal/server/handlers_admin.go`

- [ ] **Step 1: Write file**

```bash
cat > internal/server/handlers_admin.go <<'EOF'
package server

import (
	"context"
	"encoding/json"
	"net/http"

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
)

func hAdminPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modules := currentModules(r.Context(), d)
		writeSPA(w, r, supportBootstrap{
			Mode:    "admin-home",
			Theme:   adminTheme(r),
			Modules: modules,
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: true,
		}, http.StatusOK)
	}
}

func hGetConfig(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.ConfigStore == nil {
			writeErr(w, http.StatusServiceUnavailable, "config_store_unavailable", "config storage is not configured")
			return
		}
		cfg, err := d.ConfigStore.GetConfig(r.Context())
		if err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, redactConfig(cfg))
	}
}

func hPatchConfig(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.ConfigStore == nil {
			writeErr(w, http.StatusServiceUnavailable, "config_store_unavailable", "config storage is not configured")
			return
		}
		cur, err := d.ConfigStore.GetConfig(r.Context())
		if err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		var req pluginrt.Config
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body")
			return
		}
		cur.Modules = req.Modules
		next, err := pluginrt.NormalizeAppConfig(cur)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "config_failed", err.Error())
			return
		}
		if err := d.ConfigStore.UpdateConfig(r.Context(), next); err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		fresh, err := d.ConfigStore.GetConfig(r.Context())
		if err != nil {
			writeInternal(w, r, d, "config_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, redactConfig(fresh))
	}
}

func currentModules(ctx context.Context, d Deps) pluginrt.ModuleToggles {
	if d.ConfigStore == nil {
		return pluginrt.DefaultAppConfig().Modules
	}
	cfg, err := d.ConfigStore.GetConfig(ctx)
	if err != nil {
		return pluginrt.DefaultAppConfig().Modules
	}
	return cfg.Modules
}

func redactConfig(cfg pluginrt.Config) pluginrt.Config {
	return cfg
}
EOF
```

- [ ] **Step 2: No commit — wait for E1.**

---

## Phase E — Wire Together

### Task E1: Server `Deps` + `New` + chi router

**Files:**
- Create: `internal/server/server.go`

- [ ] **Step 1: Write file**

```bash
cat > internal/server/server.go <<'EOF'
// Package server is the chi-mounted HTTP handler for the support
// plugin shell. It serves the customer + admin SPA shells and the
// minimal admin JSON API.
package server

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/go-hclog"

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
)

type ConfigStore interface {
	GetConfig(ctx context.Context) (pluginrt.Config, error)
	UpdateConfig(ctx context.Context, cfg pluginrt.Config) error
}

type Deps struct {
	DatabaseURL string
	Logger      hclog.Logger
	ConfigStore ConfigStore
}

func New(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Get("/", requireUser(hCustomerHome(d)))
	r.Get("/api/customer/bootstrap", requireUser(hCustomerBootstrap(d)))
	r.Get("/admin", requireAdmin(hAdminPage(d)))
	r.Get("/api/admin/config", requireAdmin(hGetConfig(d)))
	r.Patch("/api/admin/config", requireAdmin(hPatchConfig(d)))
	r.Get("/assets/*", hPublicAsset())

	return r
}
EOF
```

- [ ] **Step 2: Build the whole server package**

```bash
go build ./internal/server/...
```

Expected: success.

- [ ] **Step 3: Run server-package tests**

```bash
go test ./internal/server/...
```

Expected: middleware tests PASS.

- [ ] **Step 4: Commit Phase D+E together**

```bash
git add internal/server/
git commit -m "feat(server): chi router, middleware, SPA + admin handlers"
```

---

### Task E2: Wire main.go to full server + Configure RPC

**Files:**
- Modify: `cmd/silo-plugin-support/main.go`

- [ ] **Step 1: Replace skeleton with full wire-up**

```bash
cat > cmd/silo-plugin-support/main.go <<'EOF'
package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	goruntime "runtime"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/jackc/pgx/v5/pgxpool"

	pluginv1 "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/manifest"
	sdkruntime "github.com/ContinuumApp/continuum-plugin-sdk/pkg/pluginsdk/runtime"

	"github.com/RXWatcher/silo-plugin-support/internal/httproutes"
	"github.com/RXWatcher/silo-plugin-support/internal/migrate"
	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
	"github.com/RXWatcher/silo-plugin-support/internal/server"
	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

//go:embed manifest.json
var manifestRaw []byte

func main() {
	logger := hclog.New(&hclog.LoggerOptions{Name: "silo-plugin-support"})
	manifest, err := loadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load manifest: %v\n", err)
		os.Exit(1)
	}

	httpSrv := httproutes.NewServer()
	var poolPtr atomic.Pointer[pgxpool.Pool]

	applyConfig := func(cfg pluginrt.Config) error {
		ctx := context.Background()
		if err := migrate.Run(ctx, cfg.DatabaseURL); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		pcfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("parse database_url: %w", err)
		}
		if pcfg.MaxConns < 4 {
			pcfg.MaxConns = 4
		}
		pool, err := pgxpool.NewWithConfig(ctx, pcfg)
		if err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		st := store.New(pool)
		cfg, err = st.Bootstrap(ctx, cfg)
		if err != nil {
			pool.Close()
			return fmt.Errorf("bootstrap config: %w", err)
		}
		httpSrv.SetHandler(server.New(server.Deps{
			DatabaseURL: cfg.DatabaseURL,
			Logger:      logger,
			ConfigStore: st,
		}))
		if old := poolPtr.Swap(pool); old != nil {
			old.Close()
		}
		logger.Info("configured support plugin")
		return nil
	}

	rt := pluginrt.New(manifest, applyConfig)

	sdkruntime.Serve(sdkruntime.ServeConfig{
		Logger: logger,
		Servers: sdkruntime.CapabilityServers{
			Runtime:    rt,
			HttpRoutes: httpSrv,
		},
	})
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestRaw)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	binaryData, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}
	checksum := sha256.Sum256(binaryData)
	manifest.Checksum = hex.EncodeToString(checksum[:])
	if len(manifest.GetSupportedPlatforms()) == 0 {
		manifest.SupportedPlatforms = []*pluginv1.SupportedPlatform{
			{Os: goruntime.GOOS, Arch: goruntime.GOARCH},
		}
	}
	return manifest, nil
}
EOF
```

- [ ] **Step 2: Verify build + tests**

```bash
go build ./...
go test ./...
```

Expected: all green.

- [ ] **Step 3: Commit**

```bash
git add cmd/silo-plugin-support/main.go
git commit -m "feat(main): wire applyConfig, migrate, store, server"
```

---

### Task E2b: Body-size cap middleware (TDD)

**Files:**
- Modify: `internal/server/middleware.go` (add `limitBody`)
- Modify: `internal/server/middleware_test.go` (add cap test)
- Modify: `internal/server/server.go` (apply middleware after `securityHeaders`)

Caps request bodies at 12 MB so a stray big upload can't OOM the
plugin. 12 MB chosen as the v1 ceiling: shell has only tiny PATCH
config; the tickets module (4th ship) sends 10 MB attachments +
JSON envelope. Modules that need bigger can opt out per-route later.

- [ ] **Step 1: Add failing test**

Append to `internal/server/middleware_test.go`:

```go
func TestLimitBodyRejectsOversizedRequests(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h := limitBody(12<<20)(inner)
	big := bytes.Repeat([]byte("x"), (12<<20)+1)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(big))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}
```

Add the imports (`bytes`, `io`) at the top of the file if not present.

- [ ] **Step 2: Run, expect fail**

```bash
go test ./internal/server/...
```

Expected: `limitBody` undefined.

- [ ] **Step 3: Implement `limitBody`**

Append to `internal/server/middleware.go`:

```go
// limitBody caps inbound request bodies. The wrapped handler sees a
// MaxBytesReader; reading past max returns an error which http
// surfaces as a 413 if the handler writes nothing else first.
func limitBody(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Wire into router (in `internal/server/server.go`)**

```go
func New(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(limitBody(12 << 20))

	r.Get("/", requireUser(hCustomerHome(d)))
	// ... rest unchanged
}
```

- [ ] **Step 5: Run tests, expect pass**

```bash
go test ./internal/server/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/server/middleware.go internal/server/middleware_test.go internal/server/server.go
git commit -m "feat(server): 12 MB request body cap middleware"
```

---

### Task E3: Server tests — full auth gates + config GET/PATCH

**Files:**
- Create: `internal/server/server_test.go`

- [ ] **Step 1: Write tests**

```bash
cat > internal/server/server_test.go <<'EOF'
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
)

type fakeConfigStore struct {
	mu  sync.Mutex
	cfg pluginrt.Config
}

func (s *fakeConfigStore) GetConfig(context.Context) (pluginrt.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg, nil
}

func (s *fakeConfigStore) UpdateConfig(_ context.Context, cfg pluginrt.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	return nil
}

func newTestDeps() (Deps, *fakeConfigStore) {
	cs := &fakeConfigStore{}
	return Deps{ConfigStore: cs}, cs
}

func adminHeaders(r *http.Request) {
	r.Header.Set("X-Silo-User-Id", "1")
	r.Header.Set("X-Silo-User-Role", "admin")
}

func userHeaders(r *http.Request) {
	r.Header.Set("X-Silo-User-Id", "42")
}

func TestCustomerHomeRequiresUserIdentity(t *testing.T) {
	h := New(Deps{ConfigStore: &fakeConfigStore{}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdminPageRequiresAdminRole(t *testing.T) {
	h := New(Deps{ConfigStore: &fakeConfigStore{}})
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	userHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestGetConfigReturnsToggles(t *testing.T) {
	d, cs := newTestDeps()
	cs.cfg = pluginrt.Config{Modules: pluginrt.ModuleToggles{KB: true}}
	h := New(d)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	adminHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body pluginrt.Config
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Modules.KB {
		t.Fatalf("KB toggle = false, want true; body=%s", rec.Body.String())
	}
}

func TestPatchConfigUpdatesToggles(t *testing.T) {
	d, cs := newTestDeps()
	h := New(d)

	body := `{"modules":{"kb":true,"speedtest":true,"tickets":false,"ai":false}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/config", bytes.NewBufferString(body))
	adminHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !cs.cfg.Modules.KB || !cs.cfg.Modules.Speedtest {
		t.Fatalf("toggles not persisted: %+v", cs.cfg.Modules)
	}
}

func TestCustomerBootstrapReturnsModulesAndIdentity(t *testing.T) {
	d, cs := newTestDeps()
	cs.cfg = pluginrt.Config{Modules: pluginrt.ModuleToggles{Speedtest: true}}
	h := New(d)

	req := httptest.NewRequest(http.MethodGet, "/api/customer/bootstrap", nil)
	userHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Modules pluginrt.ModuleToggles `json:"modules"`
		UserID  string                 `json:"userId"`
		IsAdmin bool                   `json:"isAdmin"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Modules.Speedtest {
		t.Fatalf("Speedtest module should be exposed; got %+v", body.Modules)
	}
	if body.UserID != "42" {
		t.Fatalf("UserID = %q, want 42", body.UserID)
	}
	if body.IsAdmin {
		t.Fatal("user request should NOT be flagged isAdmin")
	}
}
EOF
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/server/...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/server/server_test.go
git commit -m "test(server): auth gates + config GET/PATCH coverage"
```

---

## Phase F — Web Foundation

### Task F1: package.json + lockfile

**Files:**
- Create: `web/package.json`

- [ ] **Step 1: Write package.json**

```bash
mkdir -p web
cat > web/package.json <<'EOF'
{
  "name": "silo-plugin-support-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "build": "tsc -b && vite build",
    "dev": "vite",
    "preview": "vite preview",
    "test": "vitest run"
  },
  "dependencies": {
    "class-variance-authority": "^0.7.1",
    "clsx": "^2.1.1",
    "lucide-react": "^1.16.0",
    "radix-ui": "^1.4.3",
    "react": "^19.2.6",
    "react-dom": "^19.2.6",
    "sonner": "^2.0.7",
    "tailwind-merge": "^3.6.0"
  },
  "devDependencies": {
    "@tailwindcss/vite": "^4.3.0",
    "@types/node": "^25.8.0",
    "@types/react": "^19.2.14",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^6.0.2",
    "jsdom": "^29.1.1",
    "tailwindcss": "^4.3.0",
    "typescript": "^6.0.3",
    "vite": "^8.0.13",
    "vitest": "^4.1.6"
  }
}
EOF
```

- [ ] **Step 2: Install + lock**

```bash
cd web
pnpm install
```

- [ ] **Step 3: Commit**

```bash
cd ..
git add web/package.json web/pnpm-lock.yaml
git commit -m "chore(web): pnpm scaffolding"
```

---

### Task F2: tsconfig + vite config + index.html

**Files:**
- Create: `web/tsconfig.json`, `web/tsconfig.node.json`, `web/vite.config.ts`, `web/index.html`

- [ ] **Step 1: Copy tsconfig files from public-catalog**

```bash
cp /opt/silo_plugins/silo-plugin-public-catalog/web/tsconfig.json web/
cp /opt/silo_plugins/silo-plugin-public-catalog/web/tsconfig.node.json web/
```

- [ ] **Step 2: Write vite.config.ts**

```bash
cat > web/vite.config.ts <<'EOF'
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "src") },
  },
  build: {
    outDir: "../internal/server/public/dist",
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
    globals: false,
    setupFiles: ["./src/test-setup.ts"],
  },
});
EOF
```

- [ ] **Step 3: Write index.html**

```bash
cat > web/index.html <<'EOF'
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Support</title>
  </head>
  <body>
    <script id="support-bootstrap" type="application/json">%SUPPORT_BOOTSTRAP%</script>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
EOF
```

- [ ] **Step 4: Commit**

```bash
git add web/tsconfig.json web/tsconfig.node.json web/vite.config.ts web/index.html
git commit -m "chore(web): tsconfig + vite + index.html"
```

---

### Task F3: index.css + main.tsx + vite-env.d.ts

**Files:**
- Create: `web/src/main.tsx`, `web/src/index.css`, `web/src/vite-env.d.ts`

- [ ] **Step 1: Copy index.css + vite-env.d.ts from public-catalog**

```bash
mkdir -p web/src
cp /opt/silo_plugins/silo-plugin-public-catalog/web/src/index.css web/src/
cp /opt/silo_plugins/silo-plugin-public-catalog/web/src/vite-env.d.ts web/src/
```

- [ ] **Step 2: Write main.tsx**

```bash
cat > web/src/main.tsx <<'EOF'
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/App";
import { captureTokenFromURL } from "@/lib/authToken";
import "./index.css";

captureTokenFromURL();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
EOF
```

- [ ] **Step 3: Commit**

```bash
git add web/src/index.css web/src/vite-env.d.ts web/src/main.tsx
git commit -m "chore(web): index.css + main.tsx + vite env"
```

---

### Task F4: lib/types.ts + bootstrap.ts + bootstrap.test.ts (TDD)

**Files:**
- Create: `web/src/lib/types.ts`, `web/src/lib/bootstrap.ts`, `web/src/lib/bootstrap.test.ts`, `web/src/test-setup.ts`

- [ ] **Step 1: Write types.ts**

```bash
mkdir -p web/src/lib
cat > web/src/lib/types.ts <<'EOF'
export type ModuleToggles = {
  kb: boolean;
  speedtest: boolean;
  tickets: boolean;
  ai: boolean;
};

export type SupportBootstrap = {
  mode: "customer-home" | "admin-home";
  theme: string;
  modules: ModuleToggles;
  userId: string;
  isAdmin: boolean;
};

export type PluginConfig = {
  modules: ModuleToggles;
};

export type APIError = Error & {
  responseStatus?: number;
  responseCode?: string;
};
EOF
```

- [ ] **Step 2: Install testing-library + write setup file**

```bash
cd web
pnpm add -D @testing-library/react @testing-library/jest-dom
cat > src/test-setup.ts <<'EOF'
import "@testing-library/jest-dom/vitest";
EOF
cd ..
```

- [ ] **Step 3: Write failing test**

The test uses `replaceChildren()` to reset the DOM between cases — avoids `innerHTML` per the project's security-hook policy. The bootstrap script is created with `createElement` + `appendChild`.

```bash
cat > web/src/lib/bootstrap.test.ts <<'EOF'
import { describe, expect, it, beforeEach } from "vitest";
import { readBootstrap } from "./bootstrap";

function injectBootstrap(json: string | null) {
  document.body.replaceChildren();
  if (json !== null) {
    const s = document.createElement("script");
    s.id = "support-bootstrap";
    s.type = "application/json";
    s.textContent = json;
    document.body.appendChild(s);
  }
}

describe("readBootstrap", () => {
  beforeEach(() => injectBootstrap(null));

  it("returns customer-home defaults when no bootstrap is injected", () => {
    const bs = readBootstrap();
    expect(bs.mode).toBe("customer-home");
    expect(bs.modules.kb).toBe(false);
    expect(bs.modules.speedtest).toBe(false);
    expect(bs.userId).toBe("");
    expect(bs.isAdmin).toBe(false);
  });

  it("returns customer-home defaults when the placeholder is still present", () => {
    injectBootstrap("%SUPPORT_BOOTSTRAP%");
    const bs = readBootstrap();
    expect(bs.mode).toBe("customer-home");
  });

  it("parses an injected bootstrap and fills missing keys with defaults", () => {
    injectBootstrap(JSON.stringify({ mode: "admin-home", modules: { kb: true }, isAdmin: true }));
    const bs = readBootstrap();
    expect(bs.mode).toBe("admin-home");
    expect(bs.modules.kb).toBe(true);
    expect(bs.modules.speedtest).toBe(false);
    expect(bs.isAdmin).toBe(true);
  });
});
EOF
```

- [ ] **Step 4: Run test, expect fail**

```bash
cd web
pnpm test
```

- [ ] **Step 5: Write bootstrap.ts**

```bash
cat > src/lib/bootstrap.ts <<'EOF'
import type { ModuleToggles, SupportBootstrap } from "@/lib/types";

const DEFAULT_MODULES: ModuleToggles = {
  kb: false,
  speedtest: false,
  tickets: false,
  ai: false,
};

export function readBootstrap(): SupportBootstrap {
  if (typeof document === "undefined") return defaultBootstrap();
  const node = document.getElementById("support-bootstrap");
  if (!node || !node.textContent || node.textContent.includes("%SUPPORT_BOOTSTRAP%")) {
    return defaultBootstrap();
  }
  try {
    const parsed = JSON.parse(node.textContent) as Partial<SupportBootstrap>;
    return {
      mode: parsed.mode ?? "customer-home",
      theme: parsed.theme ?? "default",
      modules: { ...DEFAULT_MODULES, ...(parsed.modules ?? {}) },
      userId: parsed.userId ?? "",
      isAdmin: Boolean(parsed.isAdmin),
    };
  } catch {
    return defaultBootstrap();
  }
}

function defaultBootstrap(): SupportBootstrap {
  return { mode: "customer-home", theme: "default", modules: DEFAULT_MODULES, userId: "", isAdmin: false };
}
EOF
```

- [ ] **Step 6: Run tests, expect pass**

```bash
pnpm test
```

- [ ] **Step 7: Commit**

```bash
cd ..
git add web/package.json web/pnpm-lock.yaml web/src/test-setup.ts web/src/lib/types.ts web/src/lib/bootstrap.ts web/src/lib/bootstrap.test.ts
git commit -m "feat(web): bootstrap parser + types + testing-library setup"
```

---

### Task F5: lib/api.ts + authToken.ts + mountPath.ts + utils.ts

**Files:**
- Create (copy from public-catalog): `web/src/lib/api.ts`, `authToken.ts`, `mountPath.ts`, `utils.ts`

- [ ] **Step 1: Copy from public-catalog**

```bash
cp /opt/silo_plugins/silo-plugin-public-catalog/web/src/lib/api.ts        web/src/lib/
cp /opt/silo_plugins/silo-plugin-public-catalog/web/src/lib/authToken.ts  web/src/lib/
cp /opt/silo_plugins/silo-plugin-public-catalog/web/src/lib/mountPath.ts  web/src/lib/
cp /opt/silo_plugins/silo-plugin-public-catalog/web/src/lib/utils.ts      web/src/lib/
```

- [ ] **Step 2: Verify TS compiles**

```bash
cd web
pnpm exec tsc -b --noEmit
```

- [ ] **Step 3: Commit**

```bash
cd ..
git add web/src/lib/api.ts web/src/lib/authToken.ts web/src/lib/mountPath.ts web/src/lib/utils.ts
git commit -m "chore(web): api / authToken / mountPath / utils helpers"
```

---

### Task F6: lib/section.ts + section.test.ts — `?section=` URL state (TDD)

**Files:**
- Create: `web/src/lib/section.ts`, `web/src/lib/section.test.ts`

- [ ] **Step 1: Write failing test**

```bash
cat > web/src/lib/section.test.ts <<'EOF'
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { readSectionFromURL, writeSectionToURL, type AdminSection } from "./section";

describe("section URL helpers", () => {
  const pushStateSpy = vi.fn();
  const replaceStateSpy = vi.fn();

  beforeEach(() => {
    history.pushState({}, "", "/admin");
    vi.spyOn(window.history, "pushState").mockImplementation(pushStateSpy);
    vi.spyOn(window.history, "replaceState").mockImplementation(replaceStateSpy);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    pushStateSpy.mockReset();
    replaceStateSpy.mockReset();
  });

  it("defaults to overview when ?section= is missing", () => {
    history.pushState({}, "", "/admin");
    expect(readSectionFromURL()).toBe("overview");
  });

  it("returns the section from the query string when valid", () => {
    history.pushState({}, "", "/admin?section=config");
    expect(readSectionFromURL()).toBe("config");
  });

  it("falls back to overview for unknown sections", () => {
    history.pushState({}, "", "/admin?section=bogus");
    expect(readSectionFromURL()).toBe("overview");
  });

  it("writes the chosen section via pushState", () => {
    writeSectionToURL("config" as AdminSection);
    expect(pushStateSpy).toHaveBeenCalled();
    const url = pushStateSpy.mock.calls[0][2] as string;
    expect(url).toContain("section=config");
  });
});
EOF
```

- [ ] **Step 2: Run, expect fail**

```bash
cd web && pnpm test
```

- [ ] **Step 3: Write section.ts**

```bash
cat > src/lib/section.ts <<'EOF'
export type AdminSection = "overview" | "config" | "kb" | "speedtest" | "tickets" | "ai";

const KNOWN: ReadonlyArray<AdminSection> = ["overview", "config", "kb", "speedtest", "tickets", "ai"];

export function readSectionFromURL(): AdminSection {
  if (typeof window === "undefined") return "overview";
  const raw = new URLSearchParams(window.location.search).get("section") ?? "";
  return (KNOWN as readonly string[]).includes(raw) ? (raw as AdminSection) : "overview";
}

export function writeSectionToURL(section: AdminSection): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  if (section === "overview") {
    url.searchParams.delete("section");
  } else {
    url.searchParams.set("section", section);
  }
  window.history.pushState({}, "", url.toString());
}
EOF
```

- [ ] **Step 4: Run, expect pass**

```bash
pnpm test
```

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/lib/section.ts web/src/lib/section.test.ts
git commit -m "feat(web): ?section= URL state helpers"
```

---

## Phase G — UI Primitives

### Task G1: Copy shadcn primitives from public-catalog

**Files:**
- Create: `web/src/components/ui/{button,card,switch,label,badge,skeleton,sonner,input,separator}.tsx`

- [ ] **Step 1: Copy**

```bash
mkdir -p web/src/components/ui
cd web/src/components/ui
for f in button card switch label badge skeleton sonner input separator; do
  cp "/opt/silo_plugins/silo-plugin-public-catalog/web/src/components/ui/${f}.tsx" "./${f}.tsx"
done
cd /opt/silo_plugins/silo-plugin-support
```

- [ ] **Step 2: Verify TS compiles**

```bash
cd web
pnpm exec tsc -b --noEmit
```

- [ ] **Step 3: Commit**

```bash
cd ..
git add web/src/components/ui/
git commit -m "chore(web): shadcn primitives (copied from public-catalog)"
```

---

## Phase H — Customer Home

### Task H1: TopBar shared component

**Files:**
- Create: `web/src/components/shared/TopBar.tsx`

- [ ] **Step 1: Copy + adapt brand line**

```bash
mkdir -p web/src/components/shared
cp /opt/silo_plugins/silo-plugin-public-catalog/web/src/components/shared/TopBar.tsx web/src/components/shared/TopBar.tsx
```

Open the copied file and replace the brand string `"Silo public catalog"` with `"Silo support"`.

- [ ] **Step 2: Commit**

```bash
git add web/src/components/shared/TopBar.tsx
git commit -m "feat(web): TopBar header"
```

---

### Task H2: ModuleCard + test (TDD)

**Files:**
- Create: `web/src/components/shared/ModuleCard.tsx`, `web/src/components/shared/ModuleCard.test.tsx`

- [ ] **Step 1: Write failing test**

```bash
cat > web/src/components/shared/ModuleCard.test.tsx <<'EOF'
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ModuleCard } from "./ModuleCard";

describe("ModuleCard", () => {
  it("renders an anchor when enabled", () => {
    render(<ModuleCard title="Knowledge Base" href="./kb" enabled description="Browse articles" />);
    expect(screen.getByRole("link", { name: /knowledge base/i })).toHaveAttribute("href", "./kb");
    expect(screen.queryByText(/coming soon/i)).not.toBeInTheDocument();
  });

  it("renders a non-clickable placeholder when disabled", () => {
    render(<ModuleCard title="Speedtest" href="./speedtest" enabled={false} description="Test your connection" />);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
  });
});
EOF
```

- [ ] **Step 2: Run, expect fail**

```bash
cd web && pnpm test
```

- [ ] **Step 3: Write ModuleCard.tsx**

```bash
cat > src/components/shared/ModuleCard.tsx <<'EOF'
import { ArrowRight } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

type Props = {
  title: string;
  href: string;
  enabled: boolean;
  description: string;
};

export function ModuleCard({ title, href, enabled, description }: Props) {
  if (enabled) {
    return (
      <a href={href} className="block rounded-md focus:outline-none focus-visible:ring-2 focus-visible:ring-ring">
        <Card className="transition-colors hover:border-accent/40">
          <CardContent className="space-y-2 py-5">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold">{title}</h3>
              <ArrowRight className="h-4 w-4 text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">{description}</p>
          </CardContent>
        </Card>
      </a>
    );
  }
  return (
    <Card>
      <CardContent className="space-y-2 py-5">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-muted-foreground">{title}</h3>
          <Badge variant="outline">Coming soon</Badge>
        </div>
        <p className="text-sm text-muted-foreground">{description}</p>
      </CardContent>
    </Card>
  );
}
EOF
```

- [ ] **Step 4: Run, expect pass**

```bash
pnpm test
```

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/components/shared/ModuleCard.tsx web/src/components/shared/ModuleCard.test.tsx
git commit -m "feat(web): ModuleCard"
```

---

### Task H3: CustomerHome page

**Files:**
- Create: `web/src/pages/CustomerHome.tsx`

- [ ] **Step 1: Write file**

```bash
mkdir -p web/src/pages
cat > web/src/pages/CustomerHome.tsx <<'EOF'
import { ModuleCard } from "@/components/shared/ModuleCard";
import { TopBar } from "@/components/shared/TopBar";
import type { SupportBootstrap } from "@/lib/types";

type Props = { bootstrap: SupportBootstrap };

export function CustomerHome({ bootstrap }: Props) {
  const m = bootstrap.modules;
  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-5xl space-y-8 px-4 py-10 md:px-8">
        <TopBar
          eyebrow="Support"
          title="Get help"
          subtitle="Browse answers, run a connection test, or open a ticket if you're stuck."
        />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <ModuleCard title="Knowledge Base" href="./kb"        enabled={m.kb}        description="Browse articles and FAQs." />
          <ModuleCard title="Speedtest"      href="./speedtest" enabled={m.speedtest} description="Test your connection." />
          <ModuleCard title="Tickets"        href="./tickets"   enabled={m.tickets}   description="View or open a support ticket." />
          <ModuleCard title="AI Assistant"   href="./ai"        enabled={m.ai}        description="Ask a question." />
        </div>
      </div>
    </main>
  );
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/CustomerHome.tsx
git commit -m "feat(web): CustomerHome module-card grid"
```

---

## Phase I — Admin Shell

### Task I1: api/admin.ts

**Files:**
- Create: `web/src/api/admin.ts`

- [ ] **Step 1: Write**

```bash
mkdir -p web/src/api
cat > web/src/api/admin.ts <<'EOF'
import { api } from "@/lib/api";
import type { PluginConfig } from "@/lib/types";

export function getAdminConfig(): Promise<PluginConfig> {
  return api<PluginConfig>("/api/admin/config");
}

export function updateAdminConfig(patch: Partial<PluginConfig>): Promise<PluginConfig> {
  return api<PluginConfig>("/api/admin/config", {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add web/src/api/admin.ts
git commit -m "feat(web): admin API client"
```

---

### Task I2: ModuleStatusCard + test (TDD)

**Files:**
- Create: `web/src/components/admin/ModuleStatusCard.tsx`, `web/src/components/admin/ModuleStatusCard.test.tsx`

- [ ] **Step 1: Write failing test**

```bash
mkdir -p web/src/components/admin
cat > web/src/components/admin/ModuleStatusCard.test.tsx <<'EOF'
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ModuleStatusCard } from "./ModuleStatusCard";

describe("ModuleStatusCard", () => {
  it("shows 'not shipped' state when shipped=false", () => {
    render(<ModuleStatusCard title="KB" shipped={false} enabled={false} manageHref="./kb" />);
    expect(screen.getByText(/not shipped/i)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /manage/i })).toBeNull();
  });

  it("shows 'disabled' state when shipped but not enabled", () => {
    render(<ModuleStatusCard title="KB" shipped enabled={false} manageHref="./kb" />);
    expect(screen.getByText(/disabled/i)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /manage/i })).toBeNull();
  });

  it("renders a Manage link when shipped and enabled", () => {
    render(<ModuleStatusCard title="KB" shipped enabled manageHref="./kb" />);
    expect(screen.getByRole("link", { name: /manage/i })).toHaveAttribute("href", "./kb");
  });
});
EOF
```

- [ ] **Step 2: Run, expect fail**

```bash
cd web && pnpm test
```

- [ ] **Step 3: Write ModuleStatusCard.tsx**

```bash
cat > src/components/admin/ModuleStatusCard.tsx <<'EOF'
import { ArrowRight } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

type Props = {
  title: string;
  shipped: boolean;
  enabled: boolean;
  manageHref: string;
};

export function ModuleStatusCard({ title, shipped, enabled, manageHref }: Props) {
  const stateLabel = !shipped ? "not shipped" : enabled ? "enabled" : "disabled";
  const stateVariant: "outline" | "default" | "secondary" =
    !shipped ? "outline" : enabled ? "default" : "secondary";

  return (
    <Card>
      <CardContent className="flex items-center justify-between gap-4 py-4">
        <div>
          <p className="font-semibold">{title}</p>
          <p className="text-xs text-muted-foreground capitalize">{stateLabel}</p>
        </div>
        <div className="flex items-center gap-3">
          <Badge variant={stateVariant}>{stateLabel}</Badge>
          {shipped && enabled && (
            <a
              href={manageHref}
              className="inline-flex items-center gap-1 text-sm font-medium text-accent hover:underline"
            >
              Manage <ArrowRight className="h-3.5 w-3.5" />
            </a>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
EOF
```

- [ ] **Step 4: Run, expect pass**

```bash
pnpm test
```

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/components/admin/ModuleStatusCard.tsx web/src/components/admin/ModuleStatusCard.test.tsx
git commit -m "feat(web): ModuleStatusCard"
```

---

### Task I3: ModuleTogglesPanel + test (TDD)

**Files:**
- Create: `web/src/components/admin/ModuleTogglesPanel.tsx`, `web/src/components/admin/ModuleTogglesPanel.test.tsx`

- [ ] **Step 1: Write failing test**

```bash
cat > web/src/components/admin/ModuleTogglesPanel.test.tsx <<'EOF'
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ModuleTogglesPanel } from "./ModuleTogglesPanel";

describe("ModuleTogglesPanel", () => {
  it("renders one row per module with the current state", () => {
    render(<ModuleTogglesPanel
      modules={{ kb: true, speedtest: false, tickets: false, ai: false }}
      onSave={async () => {}}
    />);
    const switches = screen.getAllByRole("switch");
    expect(switches).toHaveLength(4);
  });

  it("calls onSave with a patch when a switch is toggled", async () => {
    const onSave = vi.fn(async () => {});
    render(<ModuleTogglesPanel
      modules={{ kb: false, speedtest: false, tickets: false, ai: false }}
      onSave={onSave}
    />);
    const switches = screen.getAllByRole("switch");
    switches[0].click(); // KB
    expect(onSave).toHaveBeenCalledWith({ modules: { kb: true, speedtest: false, tickets: false, ai: false } });
  });
});
EOF
```

- [ ] **Step 2: Run, expect fail**

```bash
cd web && pnpm test
```

- [ ] **Step 3: Write ModuleTogglesPanel.tsx**

```bash
cat > src/components/admin/ModuleTogglesPanel.tsx <<'EOF'
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ModuleToggles, PluginConfig } from "@/lib/types";

type Props = {
  modules: ModuleToggles;
  onSave: (patch: Partial<PluginConfig>) => Promise<void>;
};

const ROWS: Array<{ key: keyof ModuleToggles; label: string; description: string }> = [
  { key: "kb",        label: "Knowledge Base", description: "Operator-authored articles and FAQs." },
  { key: "speedtest", label: "Speedtest",      description: "Multi-endpoint connection diagnostic." },
  { key: "tickets",   label: "Tickets",        description: "Typed support intake (bad media / billing / config)." },
  { key: "ai",        label: "AI Assistance",  description: "Suggest KB articles + auto-categorise tickets." },
];

export function ModuleTogglesPanel({ modules, onSave }: Props) {
  function toggle(key: keyof ModuleToggles, value: boolean) {
    void onSave({ modules: { ...modules, [key]: value } });
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>Modules</CardTitle>
        <p className="text-sm text-muted-foreground">
          Enable a module to surface it in the customer portal and admin nav.
        </p>
      </CardHeader>
      <CardContent className="divide-y divide-border">
        {ROWS.map(({ key, label, description }) => (
          <div key={key} className="flex items-start justify-between gap-3 py-3">
            <div>
              <p className="text-sm font-medium">{label}</p>
              <p className="text-xs text-muted-foreground">{description}</p>
            </div>
            <Switch
              checked={modules[key]}
              onCheckedChange={(v) => toggle(key, Boolean(v))}
              aria-label={`Toggle ${label}`}
            />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
EOF
```

- [ ] **Step 4: Run, expect pass**

```bash
pnpm test
```

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/components/admin/ModuleTogglesPanel.tsx web/src/components/admin/ModuleTogglesPanel.test.tsx
git commit -m "feat(web): ModuleTogglesPanel"
```

---

### Task I4: AdminSidebar

**Files:**
- Create: `web/src/components/admin/AdminSidebar.tsx`

- [ ] **Step 1: Write**

```bash
cat > web/src/components/admin/AdminSidebar.tsx <<'EOF'
import type { ModuleToggles } from "@/lib/types";
import type { AdminSection } from "@/lib/section";

type Entry = {
  id: AdminSection;
  label: string;
  moduleKey?: keyof ModuleToggles;
};

const ENTRIES: ReadonlyArray<Entry> = [
  { id: "overview", label: "Overview" },
  { id: "config",   label: "Configuration" },
  { id: "kb",        label: "Knowledge Base", moduleKey: "kb" },
  { id: "speedtest", label: "Speedtest",      moduleKey: "speedtest" },
  { id: "tickets",   label: "Tickets",        moduleKey: "tickets" },
  { id: "ai",        label: "AI Assistance",   moduleKey: "ai" },
];

type Props = {
  current: AdminSection;
  modules: ModuleToggles;
  onSelect: (section: AdminSection) => void;
};

export function AdminSidebar({ current, modules, onSelect }: Props) {
  return (
    <nav className="w-56 shrink-0 border-r border-border bg-card py-4 text-sm">
      <p className="px-4 pb-3 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
        Support admin
      </p>
      <ul className="space-y-0.5">
        {ENTRIES.map((entry) => {
          const isModule = entry.moduleKey !== undefined;
          const shipped = isModule ? modules[entry.moduleKey!] : true;

          if (isModule && shipped) {
            return (
              <li key={entry.id}>
                <a href={`./${entry.id}`} className="block px-4 py-2 hover:bg-accent/10">
                  {entry.label}
                </a>
              </li>
            );
          }
          if (isModule && !shipped) {
            return (
              <li key={entry.id}>
                <button
                  type="button"
                  onClick={() => onSelect(entry.id)}
                  className={`w-full px-4 py-2 text-left ${current === entry.id ? "bg-accent/10 font-medium" : "text-muted-foreground hover:bg-accent/5"}`}
                >
                  {entry.label}
                  <span className="ml-2 text-xs text-muted-foreground">(coming soon)</span>
                </button>
              </li>
            );
          }
          return (
            <li key={entry.id}>
              <button
                type="button"
                onClick={() => onSelect(entry.id)}
                className={`w-full px-4 py-2 text-left ${current === entry.id ? "bg-accent/10 font-medium" : "hover:bg-accent/5"}`}
              >
                {entry.label}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/admin/AdminSidebar.tsx
git commit -m "feat(web): AdminSidebar"
```

---

### Task I5: AdminOverview + AdminConfig

**Files:**
- Create: `web/src/components/admin/AdminOverview.tsx`, `web/src/components/admin/AdminConfig.tsx`

- [ ] **Step 1: Write AdminOverview.tsx**

```bash
cat > web/src/components/admin/AdminOverview.tsx <<'EOF'
import { ModuleStatusCard } from "./ModuleStatusCard";
import type { ModuleToggles } from "@/lib/types";

const ENTRIES: Array<{ title: string; key: keyof ModuleToggles; href: string }> = [
  { title: "Knowledge Base", key: "kb",        href: "./kb" },
  { title: "Speedtest",      key: "speedtest", href: "./speedtest" },
  { title: "Tickets",        key: "tickets",   href: "./tickets" },
  { title: "AI Assistance",  key: "ai",        href: "./ai" },
];

export function AdminOverview({ modules }: { modules: ModuleToggles }) {
  return (
    <section className="space-y-6">
      <h2 className="text-2xl font-semibold">Overview</h2>
      <div className="rounded-md border border-border bg-card p-4 text-sm">
        <p className="font-medium">System</p>
        <ul className="mt-1 space-y-1 text-muted-foreground">
          <li>● Plugin version 0.1.0</li>
        </ul>
      </div>
      <div>
        <p className="mb-2 text-sm font-medium">Modules</p>
        <div className="grid gap-2">
          {ENTRIES.map((e) => (
            <ModuleStatusCard
              key={e.key}
              title={e.title}
              shipped={false}
              enabled={modules[e.key]}
              manageHref={e.href}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
EOF
```

- [ ] **Step 2: Write AdminConfig.tsx**

```bash
cat > web/src/components/admin/AdminConfig.tsx <<'EOF'
import { ModuleTogglesPanel } from "./ModuleTogglesPanel";
import type { ModuleToggles, PluginConfig } from "@/lib/types";

type Props = {
  modules: ModuleToggles;
  onSave: (patch: Partial<PluginConfig>) => Promise<void>;
};

export function AdminConfig({ modules, onSave }: Props) {
  return (
    <section className="space-y-6">
      <h2 className="text-2xl font-semibold">Configuration</h2>
      <ModuleTogglesPanel modules={modules} onSave={onSave} />
    </section>
  );
}
EOF
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/admin/AdminOverview.tsx web/src/components/admin/AdminConfig.tsx
git commit -m "feat(web): AdminOverview + AdminConfig sections"
```

---

### Task I6: AdminLayout + test (TDD)

**Files:**
- Create: `web/src/components/admin/AdminLayout.tsx`, `web/src/components/admin/AdminLayout.test.tsx`

- [ ] **Step 1: Write failing test**

```bash
cat > web/src/components/admin/AdminLayout.test.tsx <<'EOF'
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { AdminLayout } from "./AdminLayout";

describe("AdminLayout", () => {
  it("renders the Overview section by default", () => {
    history.pushState({}, "", "/admin");
    render(<AdminLayout modules={{ kb: false, speedtest: false, tickets: false, ai: false }} />);
    expect(screen.getByRole("heading", { name: /overview/i })).toBeInTheDocument();
  });

  it("renders the Configuration section when ?section=config is present", () => {
    history.pushState({}, "", "/admin?section=config");
    render(<AdminLayout modules={{ kb: false, speedtest: false, tickets: false, ai: false }} />);
    expect(screen.getByRole("heading", { name: /configuration/i })).toBeInTheDocument();
  });

  it("falls back to Overview on an unknown section value", () => {
    history.pushState({}, "", "/admin?section=bogus");
    render(<AdminLayout modules={{ kb: false, speedtest: false, tickets: false, ai: false }} />);
    expect(screen.getByRole("heading", { name: /overview/i })).toBeInTheDocument();
  });
});
EOF
```

- [ ] **Step 2: Run, expect fail**

```bash
cd web && pnpm test
```

- [ ] **Step 3: Write AdminLayout.tsx**

```bash
cat > src/components/admin/AdminLayout.tsx <<'EOF'
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { AdminSidebar } from "./AdminSidebar";
import { AdminOverview } from "./AdminOverview";
import { AdminConfig } from "./AdminConfig";
import { readSectionFromURL, writeSectionToURL, type AdminSection } from "@/lib/section";
import { getAdminConfig, updateAdminConfig } from "@/api/admin";
import type { ModuleToggles, PluginConfig } from "@/lib/types";

type Props = { modules: ModuleToggles };

export function AdminLayout({ modules: initialModules }: Props) {
  const [section, setSection] = useState<AdminSection>(() => readSectionFromURL());
  const [modules, setModules] = useState<ModuleToggles>(initialModules);

  useEffect(() => {
    const onPop = () => setSection(readSectionFromURL());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  function onSelect(next: AdminSection) {
    writeSectionToURL(next);
    setSection(next);
  }

  async function onSave(patch: Partial<PluginConfig>) {
    try {
      const fresh = await updateAdminConfig(patch);
      setModules(fresh.modules);
      toast.success("Settings saved.");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save settings.");
    }
  }

  useEffect(() => {
    let cancelled = false;
    getAdminConfig()
      .then((cfg) => { if (!cancelled) setModules(cfg.modules); })
      .catch(() => { /* keep bootstrap-derived state */ });
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="flex min-h-[100dvh] bg-background text-foreground">
      <AdminSidebar current={section} modules={modules} onSelect={onSelect} />
      <main className="flex-1 px-6 py-8 md:px-10">
        {section === "overview" && <AdminOverview modules={modules} />}
        {section === "config"   && <AdminConfig modules={modules} onSave={onSave} />}
        {section === "kb"        && !modules.kb        && <ComingSoon title="Knowledge Base" />}
        {section === "speedtest" && !modules.speedtest && <ComingSoon title="Speedtest" />}
        {section === "tickets"   && !modules.tickets   && <ComingSoon title="Tickets" />}
        {section === "ai"        && !modules.ai        && <ComingSoon title="AI Assistance" />}
      </main>
    </div>
  );
}

function ComingSoon({ title }: { title: string }) {
  return (
    <section className="space-y-3">
      <h2 className="text-2xl font-semibold">{title}</h2>
      <p className="text-sm text-muted-foreground">This module hasn't shipped yet.</p>
    </section>
  );
}
EOF
```

- [ ] **Step 4: Run, expect pass**

```bash
pnpm test
```

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/components/admin/AdminLayout.tsx web/src/components/admin/AdminLayout.test.tsx
git commit -m "feat(web): AdminLayout with section URL state"
```

---

### Task I7: AdminHome page

**Files:**
- Create: `web/src/pages/AdminHome.tsx`

- [ ] **Step 1: Write file**

```bash
cat > web/src/pages/AdminHome.tsx <<'EOF'
import { AdminLayout } from "@/components/admin/AdminLayout";
import type { SupportBootstrap } from "@/lib/types";

type Props = { bootstrap: SupportBootstrap };

export function AdminHome({ bootstrap }: Props) {
  return <AdminLayout modules={bootstrap.modules} />;
}
EOF
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/AdminHome.tsx
git commit -m "feat(web): AdminHome page wrapper"
```

---

## Phase J — App + Integration

### Task J1: App.tsx — bootstrap-mode dispatcher

**Files:**
- Create: `web/src/App.tsx`

- [ ] **Step 1: Write**

```bash
cat > web/src/App.tsx <<'EOF'
import { Toaster } from "@/components/ui/sonner";
import { readBootstrap } from "@/lib/bootstrap";
import { AdminHome } from "@/pages/AdminHome";
import { CustomerHome } from "@/pages/CustomerHome";

export function App() {
  const bootstrap = readBootstrap();
  const page =
    bootstrap.mode === "admin-home"
      ? <AdminHome bootstrap={bootstrap} />
      : <CustomerHome bootstrap={bootstrap} />;
  return (
    <>
      {page}
      <Toaster />
    </>
  );
}
EOF
```

- [ ] **Step 2: SPA test + build**

```bash
cd web
pnpm test
pnpm build
```

Expected: all tests pass, `vite build` emits to `../internal/server/public/dist/`.

- [ ] **Step 3: Full Go build**

```bash
cd ..
go build ./...
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): App mode dispatcher"
```

---

### Task J2: `make build` + `make test` + README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Run `make build`**

```bash
make build
```

Expected: SPA build + Go binary `./silo-plugin-support` produced.

- [ ] **Step 2: Run `make test`**

```bash
make test
```

Expected: Go tests + SPA tests pass.

- [ ] **Step 3: Write README**

```bash
cat > README.md <<'EOF'
# Silo Support Plugin

`silo.support` is the customer-facing support surface for a
Silo deployment. The shell ships first; modules (Knowledge Base,
Speedtest, Tickets, AI Assistance) follow in their own releases.

See `docs/superpowers/specs/` for designs and `docs/superpowers/plans/`
for implementation plans.

## Build

```
make build
make test
```

## Configuration

Requires `database_url` — a Postgres DSN, e.g.
`postgres://plugin_support:...@host:5432/silo?search_path=support&sslmode=disable`.

The plugin manages its own schema; the operator only needs to create
the schema and grant connect rights.
EOF
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: project README"
```

---

## Self-Review Notes

**Spec coverage check** (against `2026-05-21-support-shell-design.md`):
- File layout — every file in the spec has a creating task.
- Manifest — Task A2.
- Schema — Task C1.
- Config struct + DefaultAppConfig + NormalizeAppConfig — Task B1.
- Auth middleware — Task D2.
- Theming (`adminTheme` in spa.go) — Task D3.
- Customer home — Task H3.
- Admin home full-page shell — Tasks I4 / I5 / I6 / I7.
- Bootstrap payload — Tasks D4 / D5 (Go) + F4 (TS parser).
- `?section=` URL state — Task F6 (helpers) + I6 (wiring).
- Per-module sidebar pre-/post-ship — Task I4.
- Tests — runtime (B1), middleware (D2), server config + auth gates (E3), bootstrap parser (F4), section helpers (F6), ModuleCard (H2), ModuleStatusCard (I2), ModuleTogglesPanel (I3), AdminLayout (I6).
- Success criteria (`make build`, `make test`, auth gates verified, admin toggle PATCH) — Tasks J2 + E3 + I3.

**Coverage gap addressed:** placeholder `internal/server/public/dist/index.html` so `//go:embed` doesn't fail before `pnpm build` runs — handled in Task D3 step 2.

**Type / method-name consistency:**
- `ModuleToggles` keys (`kb`, `speedtest`, `tickets`, `ai`) match across Go, TS, JSON, and SPA.
- `supportBootstrap` Go struct keys match SPA `SupportBootstrap`.
- Admin sections (`overview` / `config` / `kb` / `speedtest` / `tickets` / `ai`) are spelled identically across `section.ts`, `AdminSidebar.tsx`, `AdminLayout.tsx`.
- `readSectionFromURL` / `writeSectionToURL` exported from `section.ts`, used in `AdminLayout.tsx`.

**Placeholder scan:** searched for "TODO" / "TBD" / "implement later" / "Similar to Task" — none found.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-21-support-shell.md`. Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using `executing-plans`, batch execution with checkpoints.

Which approach?
