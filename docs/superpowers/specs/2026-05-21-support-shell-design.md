# Support Plugin — Shell Design

**Status:** Sub-project design under the program spec
([2026-05-21-support-plugin-program-design.md](2026-05-21-support-plugin-program-design.md)).
The shell is the foundation that ships before any module. It is not
user-visible beyond an empty "Support" surface with "Coming soon" cards.

**Date:** 2026-05-21
**Sub-project:** Shell (foundation)
**Successor:** Knowledge Base module (next brainstorm)

## Purpose

Establish the plugin's permanent skeleton: manifest, auth gates, theme
inheritance, navigation, migrations runner, Postgres schema container,
and the SPA shells (customer + admin) that each module will hang into
when it ships. Includes the module-enable toggle mechanism so future
modules can be turned off by operators without uninstalling the plugin.

The shell has no business logic of its own. Its success criterion is
"future module work plugs in cleanly."

## Architecture

Mirrors `continuum-plugin-public-catalog`'s shape exactly:

- One Go binary, served by the Continuum host through the SDK's
  `HttpRoutes` capability (no standalone listener; this is
  portal-internal).
- One Postgres schema (`support`) — operator creates the schema and
  grants connect rights; `golang-migrate` runs on plugin start.
- One React + TypeScript + Vite SPA, bundled into the binary via
  `//go:embed public/dist/*`. Dispatches by server-baked bootstrap
  mode (`customer-home` | `admin-home`) — no client-side router.

## File Layout

```
continuum-plugin-support/
├── cmd/continuum-plugin-support/
│   ├── main.go
│   └── manifest.json
├── internal/
│   ├── runtime/        runtime.go, runtime_test.go        # Config + Configure RPC
│   ├── httproutes/     server.go                           # SDK Handle shim, header stripping
│   ├── server/         server.go                           # router + Deps + New
│   │                   middleware.go                       # securityHeaders, requireUser, requireAdmin
│   │                   spa.go                              # //go:embed + bootstrap render
│   │                   response.go                         # writeJSON / writeErr / writeInternal
│   │                   handlers_customer.go                # GET / and GET /api/customer/bootstrap
│   │                   handlers_admin.go                   # GET /admin, GET/PATCH /api/admin/config
│   │                   server_test.go
│   ├── store/          store.go, config.go, types.go       # pgxpool + app_config CRUD
│   └── migrate/        runner.go
│                       files/0001_init.up.sql
│                       files/0001_init.down.sql
├── web/                                                     # same toolchain as public-catalog
│   ├── package.json, vite.config.ts, tsconfig*.json
│   ├── index.html
│   └── src/
│       ├── main.tsx, App.tsx, index.css, vite-env.d.ts
│       ├── lib/        bootstrap.ts, mountPath.ts, api.ts, authToken.ts, types.ts, utils.ts
│       ├── pages/      CustomerHome.tsx, AdminHome.tsx
│       ├── components/
│       │   ├── ui/                      # shadcn primitives (button, card, switch, label, etc.)
│       │   ├── shared/                  # TopBar, ModuleCard
│       │   └── admin/                   # AdminLayout, AdminSidebar, AdminOverview,
│       │                                # AdminConfig, ModuleStatusCard, ModuleTogglesPanel
│       └── api/        admin.ts
├── Makefile
├── go.mod / go.sum
├── README.md
└── docs/superpowers/...                                      # this spec + program spec
```

## Manifest

```json
{
  "plugin_id": "continuum.support",
  "version": "0.1.0",
  "category": "Operations",
  "continuum_api_version": "v1",
  "capabilities": [
    { "type": "http_routes.v1", "id": "support",
      "display_name": "Support",
      "description": "Customer support shell — modules ship in follow-up releases." }
  ],
  "http_routes": [
    { "id": "customer_home", "method": "GET", "path": "/",       "access": "user",
      "navigable": true, "navigation_label": "Support", "navigation_kind": "user" },
    { "id": "customer_bootstrap", "method": "GET", "path": "/api/customer/bootstrap",
      "access": "user" },
    { "id": "admin_page", "method": "GET", "path": "/admin", "access": "admin",
      "navigable": true, "navigation_label": "Support", "navigation_kind": "admin" },
    { "id": "admin_get_config",   "method": "GET",   "path": "/api/admin/config", "access": "admin" },
    { "id": "admin_patch_config", "method": "PATCH", "path": "/api/admin/config", "access": "admin" }
  ],
  "global_config_schema": [
    {
      "key": "database_url",
      "title": "Postgres connection string",
      "description": "DSN for the dedicated `support` schema.",
      "required": true,
      "admin_form": { "fields": [{ "key": "value", "label": "Connection URL",
        "control": "ADMIN_FORM_CONTROL_PASSWORD", "required": true, "secret": true }] }
    }
  ]
}
```

Each future module ships its own `version` bump that adds its routes
and config schema entries. The shell never advertises routes for
unshipped modules.

## Schema

`internal/migrate/files/0001_init.up.sql`:

```sql
CREATE TABLE app_config (
  id          SMALLINT PRIMARY KEY CHECK (id = 1),
  data        JSONB    NOT NULL DEFAULT '{}'::jsonb,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO app_config (id, data) VALUES (1, '{}'::jsonb)
ON CONFLICT (id) DO NOTHING;
```

That's the entire shell schema. Module migrations are additive and
own their own table prefixes (`kb_*`, `st_*`, `tk_*`, `ai_*`).

## Config

```go
package runtime

type Config struct {
    DatabaseURL string        `json:"-"`         // manifest-only; never persisted
    Modules     ModuleToggles `json:"modules"`
}

type ModuleToggles struct {
    KB        bool `json:"kb"`
    Speedtest bool `json:"speedtest"`
    Tickets   bool `json:"tickets"`
    AI        bool `json:"ai"`
}
```

`Modules` defaults to all-`false` (no modules shipped at shell time).
When a module ships, its release:

1. Adds its routes to `manifest.json`.
2. Bumps the version.
3. Updates `DefaultAppConfig()` to flip its own toggle to `true`.
4. Adds its migration file (`0002_kb.up.sql`, `0003_speedtest.up.sql`, …).

`NormalizeAppConfig(cfg Config) (Config, error)` validates and returns
the canonical form. The only validation the shell needs is the DSN
presence check — module-specific validation lives in module specs.

`store.Bootstrap` reads `app_config.data`, merges with the manifest-
supplied `Config`, and persists back. Mirrors public-catalog's
pattern.

## Auth Middleware

`internal/server/middleware.go`:

```go
func requireUser(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-Continuum-User-Id") == "" {
            writeErr(w, http.StatusUnauthorized, "unauthenticated", "log in to continue")
            return
        }
        next(w, r)
    }
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-Continuum-User-Id") == "" {
            writeErr(w, http.StatusUnauthorized, "unauthenticated", "admin login required")
            return
        }
        if r.Header.Get("X-Continuum-User-Role") != "admin" {
            writeErr(w, http.StatusForbidden, "forbidden", "admin access required")
            return
        }
        next(w, r)
    }
}
```

`httproutes/server.go` strips inbound `X-Continuum-*` headers on its
`ServeHTTP` path (defence-in-depth in case a standalone listener is
ever added later) — copied verbatim from public-catalog.

## Theming

The `spa.go` shell renderer reads `X-Continuum-Theme` (or
`X-Continuum-User-Theme`) and sets `data-theme="…"` on the rendered
`<html>` tag. Default `midnight-cinema`.

```go
func adminTheme(r *http.Request) string {
    theme := r.Header.Get("X-Continuum-Theme")
    if theme == "" { theme = r.Header.Get("X-Continuum-User-Theme") }
    if theme == "" { theme = "default" }
    return html.EscapeString(theme)
}
```

(Plus an `?theme=` query-string override for the admin SPA path, same
as public-catalog.)

## SPA Surfaces

### Customer home (`mode: "customer-home"`)

```
+-----------------------------------------------------+
|  SUPPORT                                            |
|  Get help when you need it.                         |
+-----------------------------------------------------+
|  +--------+ +--------+ +--------+ +--------+        |
|  | KB     | | Speed  | | Tickets| | AI     |        |
|  | (coming| | (coming| | (coming| | (coming|        |
|  |  soon) | |  soon) | |  soon) | |  soon) |        |
|  +--------+ +--------+ +--------+ +--------+        |
+-----------------------------------------------------+
```

Cards rendered from `bootstrap.modules`. Each card knows its target
path (`./kb`, `./speedtest`, `./tickets`); when the module is enabled
the card is an `<a href>`, when disabled it's a non-clickable div with
a "Coming soon" badge.

### Admin home (`mode: "admin-home"`)

Full app shell with persistent left sidebar + main content. The
shell-internal section is React state — no URL change, no
client-side router (matches public-catalog's deliberate avoidance).

```
+----------------+--------------------------------------+
| SUPPORT ADMIN  | Overview                             |
|                +--------------------------------------+
| · Overview     | System                               |
| · Configuration|   ● Database connected               |
| ─────────      |   Plugin version 0.1.0               |
| · KB           |                                      |
|   coming soon  | Modules                              |
| · Speedtest    | ┌────────────────────────────────┐   |
|   coming soon  | │ Knowledge Base   ○ not shipped │   |
| · Tickets      | ├────────────────────────────────┤   |
|   coming soon  | │ Speedtest        ○ not shipped │   |
| · AI           | ├────────────────────────────────┤   |
|   coming soon  | │ ...                            │   |
|                | └────────────────────────────────┘   |
+----------------+--------------------------------------+
```

**Sidebar entries (shell-built-ins):**

- **Overview** (default selected) — `<AdminOverview>` renders the system
  status row (DB connection, plugin version, manifest version) plus a
  module status grid of `<ModuleStatusCard>` (one per module, with
  enabled / disabled / not-shipped state and a "Manage →" link when
  the module is enabled).
- **Configuration** — `<AdminConfig>` houses the
  `<ModuleTogglesPanel>` (the four switches) plus a placeholder section
  for future plugin-wide settings.

**Sidebar entries (per-module, populated as modules ship):**

- **Knowledge Base** — when `modules.kb` is true, the sidebar entry is
  an `<a href="./kb">` linking to the KB module's admin landing
  (route declared in the KB module's manifest release, not in the
  shell's). Until then the entry is rendered as a non-clickable
  "coming soon" placeholder so operators see what's coming.
- Same pattern for Speedtest / Tickets / AI.

**Section state in the URL** — `<AdminLayout>` mirrors the active
section into a `?section=` query param so sidebar selections are
deep-linkable and bookmarkable. No client-side router is added — this
is one `URLSearchParams` read on mount and a `history.pushState` on
each section change, scoped to this one param. Browser back/forward
navigate between sections naturally.

```
/admin                       → Overview (default)
/admin?section=config        → Configuration
/admin?section=kb            → KB module admin entry (placeholder until module ships)
```

Unknown / missing `section` values fall back to Overview. The URL
stays at `/admin` otherwise — no extra manifest routes to declare.

The per-module sidebar entries behave the same way pre- and
post-ship: pre-ship, clicking the sidebar entry switches the
in-page section to a placeholder ("Knowledge Base is not yet
shipped"); post-ship, the entry becomes an `<a href>` to that
module's own admin route (e.g. `./kb`) declared in the module's
manifest release.

## Bootstrap Payload

```ts
type SupportBootstrap = {
  mode: "customer-home" | "admin-home";
  theme: string;
  modules: {
    kb: boolean;
    speedtest: boolean;
    tickets: boolean;
    ai: boolean;
  };
  // identity helpers; admins see their role, customers see only userId
  userId: string;
  isAdmin: boolean;
};
```

Customer bootstrap uses `requireUser`; the SPA shell exposes
`bootstrap.modules` so each card renders correct state without an extra
fetch. Admin bootstrap is the same shape with `isAdmin: true`.

## What's Deliberately Not In The Shell

- No customer-facing module pages — those ship with each module.
- No standalone listener — portal-internal only.
- No business logic, no module-specific tables.
- No federation calls to other plugins.
- No public surface — all routes require login.

## Tests

Go (table-driven, fakes mirror public-catalog):

- `runtime` — Configure RPC roundtrip, normalisation defaults, missing
  DSN rejection.
- `server` — requireUser/requireAdmin gates (401/403), GET /api/admin/config
  returns the stored toggles, PATCH updates them.

SPA (vitest):

- `bootstrap.ts` parse + defaults.
- ModuleCard renders enabled vs disabled variants.
- ModuleStatusCard renders not-shipped vs disabled vs enabled variants.
- ModuleTogglesPanel: switch click → onSave called with patch.
- AdminLayout: sidebar entry click switches the rendered section
  and updates the URL `?section=`; mounting with a `?section=` value
  selects that section on first paint; unknown values fall back to
  Overview; per-module entries are clickable when enabled, placeholders
  when not.

## Out Of Scope

These belong to module specs or to v2 of the program:

- Cross-module integration wiring (built into each module when it lands).
- Module-version-skew handling (e.g., DB migrated for KB v2 but binary is KB v1) — single-binary release strategy means no skew.
- Per-user role beyond `user` / `admin` (no agent role, no super-admin in v1).
- Audit logging of admin config changes.

## Success Criteria

- `make build` produces a working binary.
- `make test` passes.
- Plugin installs against a real Continuum host, both customer and
  admin nav entries appear, both pages render with "Coming soon"
  module cards.
- Admin can toggle a module on/off (toggle has no effect because no
  module is shipped, but the PATCH succeeds and the GET reflects the
  new state).
- Auth gates verified: customer route → 401 without `X-Continuum-User-Id`,
  admin route → 403 without admin role.
