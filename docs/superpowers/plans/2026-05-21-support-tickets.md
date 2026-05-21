# Support Plugin — Tickets Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Tickets module to `continuum-plugin-support` — customer-opens-ticket-via-category-form, admin-handles-via-queue, status lifecycle with reopen + configurable auto-close, append-only entries with internal notes, 10 MB bytea attachments, events to the notifications plugin.

**Architecture:** Extends the support plugin shell + KB + Speedtest. Eight new tables under the `support` schema (`tk_categories`, `tk_subcategories`, `tk_category_fields`, `tk_tickets`, `tk_ticket_entries`, `tk_ticket_field_values`, `tk_attachments`, `tk_ticket_sequence`). New routes added to the existing manifest. Status transitions enforced server-side via a transition map; forbidden transitions return 409. SPA gains 4 new bootstrap modes; 30-second polling on queue + detail (no SSE — SDK `Handle` is request/response).

**Tech Stack:** Go 1.26 (existing). No new Go deps. Frontend: same stack as prior modules (React 19, Tailwind v4, shadcn primitives, vitest). Multipart upload via standard `net/http`.

**Spec:** [`../specs/2026-05-21-support-tickets-design.md`](../specs/2026-05-21-support-tickets-design.md)
**Predecessor:** Speedtest module shipped in commits `439e1a3..e7a16f1` on `main`.

---

## File Structure

All paths relative to `/opt/continuum_plugins/continuum-plugin-support/`.

### Go side

| File | Responsibility |
|---|---|
| `internal/migrate/files/0004_tickets_init.up.sql` + `.down.sql` | Create the eight tk_* tables + indexes + sequence seed |
| `internal/store/tk_types.go` | Go types (Category, Subcategory, Field, Ticket, Entry, FieldValue, Attachment, ResultFilter, etc.) |
| `internal/store/tk_categories.go` | Categories + subcategories + fields CRUD |
| `internal/store/tk_tickets.go` | Ticket CRUD, list with filters, sequence-backed tracking number generation |
| `internal/store/tk_entries.go` | Entry insert + list (with internal-note filter) |
| `internal/store/tk_attachments.go` | Attachment insert / get; per-entry attachment list |
| `internal/tickets/lifecycle.go` | Transition map + ApplyTransition pure function |
| `internal/tickets/lifecycle_test.go` | Every allowed + every forbidden transition |
| `internal/tickets/cron.go` | CloseIdle pass (configurable enable / day thresholds) |
| `internal/tickets/cron_test.go` | Auto-close pass tests with fake store |
| `internal/server/handlers_tk_customer.go` | Customer ticket API + SPA shell handlers |
| `internal/server/handlers_tk_admin.go` | Admin ticket API + SPA shell handlers + categories CRUD |
| `internal/server/handlers_tk_attachments.go` | Upload (multipart, 10 MB) + serve |
| `internal/server/tk_events.go` | Event publisher helpers (ticket_submitted / replied / status_changed / assigned / resolved / reopened / closed) |
| `internal/server/server.go` | Register tk_* routes + extend Deps with tickets config |
| `internal/server/spa.go` | (no change — `supportBootstrap.Mode` is `string`, modes added in TS) |
| `cmd/continuum-plugin-support/main.go` | Pass tickets config fields to server.Deps |
| `cmd/continuum-plugin-support/manifest.json` | Add tk routes + bump version to 0.4.0 |
| `internal/runtime/runtime.go` | Add ticket config fields + flip Modules.Tickets default to true |

### Web side

| File | Responsibility |
|---|---|
| `web/src/lib/modules.ts` | Flip `SHIPPED_MODULES.tickets` to `true` (final unit) |
| `web/src/lib/types.ts` | Extend with Tickets types + bootstrap mode union |
| `web/src/api/tk.ts` | Customer ticket API client |
| `web/src/api/tkAdmin.ts` | Admin ticket API client |
| `web/src/components/tk/CategoryStep.tsx` (+ test) | Step 1 of new-ticket flow |
| `web/src/components/tk/SubcategoryStep.tsx` | Step 2 |
| `web/src/components/tk/FieldsForm.tsx` | Per-category form fields with required validation |
| `web/src/components/tk/TicketCard.tsx` | List item card |
| `web/src/components/tk/Thread.tsx` | Append-only entry timeline |
| `web/src/components/tk/ReplyBox.tsx` (+ test) | Reply composer with optional attachments |
| `web/src/components/tk/StatusBadge.tsx` | Status pill with consistent colors |
| `web/src/components/admin/tk/Queue.tsx` | Admin queue table + filters |
| `web/src/components/admin/tk/ActionPanel.tsx` | Status / assignee / internal-note controls |
| `web/src/components/admin/tk/CategoryAdmin.tsx` | Category + subcategory + field tree editor |
| `web/src/pages/tk/List.tsx` | Customer list page |
| `web/src/pages/tk/New.tsx` | New-ticket flow |
| `web/src/pages/tk/Detail.tsx` | Customer detail page |
| `web/src/pages/admin/tk/Queue.tsx` | Admin queue page |
| `web/src/pages/admin/tk/Detail.tsx` | Admin detail page |
| `web/src/pages/admin/tk/Categories.tsx` | Admin categories page |
| `web/src/App.tsx` | Dispatch 5 new bootstrap modes (customer list + new + detail + admin queue + admin detail; admin categories reuses an existing admin shell pattern) |

---

## Phase A — Foundation

### Task A1: Migration `0004_tickets_init`

**Files:**
- Create: `internal/migrate/files/0004_tickets_init.up.sql`
- Create: `internal/migrate/files/0004_tickets_init.down.sql`

- [ ] **Step 1: Write the up migration**

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > internal/migrate/files/0004_tickets_init.up.sql <<'EOF'
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

CREATE TABLE tk_tickets (
    id                BIGSERIAL PRIMARY KEY,
    tracking_number   TEXT NOT NULL UNIQUE,
    customer_id       TEXT NOT NULL,
    customer_email    TEXT NOT NULL,
    category_id       BIGINT NOT NULL REFERENCES tk_categories(id) ON DELETE RESTRICT,
    subcategory_id    BIGINT REFERENCES tk_subcategories(id) ON DELETE RESTRICT,
    subject           TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open','in_progress','waiting_customer','resolved','closed')),
    assigned_admin_id TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    waiting_since     TIMESTAMPTZ,
    resolved_at       TIMESTAMPTZ
);
CREATE INDEX tk_tickets_customer_idx ON tk_tickets(customer_id, updated_at DESC);
CREATE INDEX tk_tickets_queue_idx    ON tk_tickets(status, updated_at DESC);
CREATE INDEX tk_tickets_assigned_idx ON tk_tickets(assigned_admin_id, status) WHERE assigned_admin_id IS NOT NULL;

CREATE TABLE tk_ticket_entries (
    id          BIGSERIAL PRIMARY KEY,
    ticket_id   BIGINT NOT NULL REFERENCES tk_tickets(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL
        CHECK (kind IN ('initial','reply','internal_note','status_change','system')),
    author_id   TEXT NOT NULL,
    author_role TEXT NOT NULL CHECK (author_role IN ('customer','admin','system')),
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX tk_ticket_entries_ticket_idx ON tk_ticket_entries(ticket_id, created_at);

CREATE TABLE tk_ticket_field_values (
    ticket_id BIGINT NOT NULL REFERENCES tk_tickets(id) ON DELETE CASCADE,
    field_id  BIGINT NOT NULL REFERENCES tk_category_fields(id) ON DELETE RESTRICT,
    value     TEXT NOT NULL,
    PRIMARY KEY (ticket_id, field_id)
);

CREATE TABLE tk_attachments (
    id              BIGSERIAL PRIMARY KEY,
    entry_id        BIGINT NOT NULL REFERENCES tk_ticket_entries(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    mime            TEXT NOT NULL,
    bytes           BIGINT NOT NULL,
    content_bytea   BYTEA NOT NULL,
    sha256          BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX tk_attachments_entry_idx ON tk_attachments(entry_id);

CREATE TABLE tk_ticket_sequence (
    id      SMALLINT PRIMARY KEY CHECK (id = 1),
    next_n  BIGINT NOT NULL DEFAULT 1
);
INSERT INTO tk_ticket_sequence (id, next_n) VALUES (1, 1) ON CONFLICT (id) DO NOTHING;
EOF
```

- [ ] **Step 2: Write the down migration**

```bash
cat > internal/migrate/files/0004_tickets_init.down.sql <<'EOF'
DROP TABLE IF EXISTS tk_ticket_sequence;
DROP TABLE IF EXISTS tk_attachments;
DROP TABLE IF EXISTS tk_ticket_field_values;
DROP TABLE IF EXISTS tk_ticket_entries;
DROP TABLE IF EXISTS tk_tickets;
DROP TABLE IF EXISTS tk_category_fields;
DROP TABLE IF EXISTS tk_subcategories;
DROP TABLE IF EXISTS tk_categories;
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
  commit -m "feat(migrate): 0004 tickets tables + sequence seed"
```

---

### Task A2: Store types

**Files:**
- Create: `internal/store/tk_types.go`

```bash
cat > internal/store/tk_types.go <<'EOF'
package store

import "time"

// Category taxonomy --------------------------------------------------

type TKCategory struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sortOrder"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TKSubcategory struct {
	ID         int64     `json:"id"`
	CategoryID int64     `json:"categoryId"`
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
	SortOrder  int       `json:"sortOrder"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type TKCategoryField struct {
	ID         int64  `json:"id"`
	CategoryID int64  `json:"categoryId"`
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`     // 'text' | 'textarea' | 'number' | 'url'
	Required   bool   `json:"required"`
	SortOrder  int    `json:"sortOrder"`
}

// Ticket -------------------------------------------------------------

type TKTicket struct {
	ID               int64      `json:"id"`
	TrackingNumber   string     `json:"trackingNumber"`
	CustomerID       string     `json:"customerId"`
	CustomerEmail    string     `json:"customerEmail"`
	CategoryID       int64      `json:"categoryId"`
	SubcategoryID    *int64     `json:"subcategoryId,omitempty"`
	Subject          string     `json:"subject"`
	Status           string     `json:"status"`
	AssignedAdminID  *string    `json:"assignedAdminId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	WaitingSince     *time.Time `json:"waitingSince,omitempty"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	// Convenience joins; loaded by Get* methods, omitted by list methods.
	Category    *TKCategory      `json:"category,omitempty"`
	Subcategory *TKSubcategory   `json:"subcategory,omitempty"`
	FieldValues []TKFieldValue   `json:"fieldValues,omitempty"`
}

type TKEntry struct {
	ID         int64     `json:"id"`
	TicketID   int64     `json:"ticketId"`
	Kind       string    `json:"kind"`        // 'initial' | 'reply' | 'internal_note' | 'status_change' | 'system'
	AuthorID   string    `json:"authorId"`
	AuthorRole string    `json:"authorRole"`  // 'customer' | 'admin' | 'system'
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	Attachments []TKAttachmentMeta `json:"attachments,omitempty"`
}

// TKFieldValue is one (field, value) pair captured at ticket creation.
// Field is joined in by the read query so the SPA can render labels
// without a second roundtrip.
type TKFieldValue struct {
	FieldID    int64  `json:"fieldId"`
	FieldKey   string `json:"fieldKey"`
	FieldLabel string `json:"fieldLabel"`
	Value      string `json:"value"`
}

// TKAttachmentMeta is the metadata-only projection returned in entry
// payloads. Bytes stream separately from /api/attachments/{id}.
type TKAttachmentMeta struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	MIME      string `json:"mime"`
	Bytes     int64  `json:"bytes"`
	CreatedAt time.Time `json:"createdAt"`
}

// TKAttachment is the full row including bytes — used internally by
// the serve handler.
type TKAttachment struct {
	ID        int64
	EntryID   int64
	Filename  string
	MIME      string
	Bytes     int64
	Content   []byte
	SHA256    []byte
	CreatedAt time.Time
}

// TKTicketListFilter narrows what TKListTickets returns.
type TKTicketListFilter struct {
	CustomerID      string  // restrict to one customer (customer endpoint)
	Status          string  // "" = any
	StatusGroup     string  // "active" (open|in_progress|waiting_customer) | "closed" | ""
	CategoryID      int64
	AssigneeID      string  // "__mine__" → caller id; "__unassigned__" → IS NULL
	CallerAdminID   string  // used to resolve "__mine__"
	Search          string  // tracking-number prefix OR subject substring
	Limit           int
	Offset          int
}
EOF
go build ./internal/store/...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/tk_types.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): tickets row types"
```

---

## Phase B — Store CRUD

### Task B1: tk_categories.go — categories + subcategories + fields

**Files:**
- Create: `internal/store/tk_categories.go`

This is the largest store file for tickets — it bundles the three taxonomy tables (categories, subcategories, category fields) since they're tightly coupled and the admin tree editor needs all three together.

```bash
cat > internal/store/tk_categories.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// --- Categories ----------------------------------------------------------

func (s *Store) TKListCategories(ctx context.Context, activeOnly bool) ([]TKCategory, error) {
	q := `SELECT id, slug, name, sort_order, active, created_at, updated_at FROM tk_categories`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(name)`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list tk_categories: %w", err)
	}
	defer rows.Close()
	out := []TKCategory{}
	for rows.Next() {
		var c TKCategory
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) TKGetCategory(ctx context.Context, id int64) (TKCategory, error) {
	var c TKCategory
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, sort_order, active, created_at, updated_at
		FROM tk_categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Slug, &c.Name, &c.SortOrder, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategory{}, ErrNotFound
	}
	if err != nil {
		return TKCategory{}, fmt.Errorf("get tk_category: %w", err)
	}
	return c, nil
}

func (s *Store) TKCreateCategory(ctx context.Context, in TKCategory) (TKCategory, error) {
	var out TKCategory
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_categories (slug, name, sort_order, active)
		VALUES ($1,$2,$3,$4)
		RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		in.Slug, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return TKCategory{}, fmt.Errorf("insert tk_category: %w", err)
	}
	return out, nil
}

func (s *Store) TKUpdateCategory(ctx context.Context, in TKCategory) (TKCategory, error) {
	var out TKCategory
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_categories SET
		  name = $2, sort_order = $3, active = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, slug, name, sort_order, active, created_at, updated_at`,
		in.ID, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategory{}, ErrNotFound
	}
	if err != nil {
		return TKCategory{}, fmt.Errorf("update tk_category: %w", err)
	}
	return out, nil
}

func (s *Store) TKDeleteCategory(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tk_categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tk_category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TKCategorySlugExists(ctx context.Context, slug string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM tk_categories WHERE slug = $1`, slug).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// --- Subcategories -------------------------------------------------------

func (s *Store) TKListSubcategories(ctx context.Context, categoryID int64, activeOnly bool) ([]TKSubcategory, error) {
	q := `SELECT id, category_id, slug, name, sort_order, active, created_at, updated_at
	      FROM tk_subcategories WHERE category_id = $1`
	if activeOnly {
		q += ` AND active = TRUE`
	}
	q += ` ORDER BY sort_order, lower(name)`
	rows, err := s.pool.Query(ctx, q, categoryID)
	if err != nil {
		return nil, fmt.Errorf("list tk_subcategories: %w", err)
	}
	defer rows.Close()
	out := []TKSubcategory{}
	for rows.Next() {
		var x TKSubcategory
		if err := rows.Scan(&x.ID, &x.CategoryID, &x.Slug, &x.Name, &x.SortOrder, &x.Active, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) TKGetSubcategory(ctx context.Context, id int64) (TKSubcategory, error) {
	var x TKSubcategory
	err := s.pool.QueryRow(ctx, `
		SELECT id, category_id, slug, name, sort_order, active, created_at, updated_at
		FROM tk_subcategories WHERE id = $1`, id).
		Scan(&x.ID, &x.CategoryID, &x.Slug, &x.Name, &x.SortOrder, &x.Active, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKSubcategory{}, ErrNotFound
	}
	if err != nil {
		return TKSubcategory{}, fmt.Errorf("get tk_subcategory: %w", err)
	}
	return x, nil
}

func (s *Store) TKCreateSubcategory(ctx context.Context, in TKSubcategory) (TKSubcategory, error) {
	var out TKSubcategory
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_subcategories (category_id, slug, name, sort_order, active)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, category_id, slug, name, sort_order, active, created_at, updated_at`,
		in.CategoryID, in.Slug, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.CategoryID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return TKSubcategory{}, fmt.Errorf("insert tk_subcategory: %w", err)
	}
	return out, nil
}

func (s *Store) TKUpdateSubcategory(ctx context.Context, in TKSubcategory) (TKSubcategory, error) {
	var out TKSubcategory
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_subcategories SET
		  name = $2, sort_order = $3, active = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, category_id, slug, name, sort_order, active, created_at, updated_at`,
		in.ID, in.Name, in.SortOrder, in.Active).
		Scan(&out.ID, &out.CategoryID, &out.Slug, &out.Name, &out.SortOrder, &out.Active, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKSubcategory{}, ErrNotFound
	}
	if err != nil {
		return TKSubcategory{}, fmt.Errorf("update tk_subcategory: %w", err)
	}
	return out, nil
}

func (s *Store) TKDeleteSubcategory(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tk_subcategories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tk_subcategory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Category fields -----------------------------------------------------

func (s *Store) TKListCategoryFields(ctx context.Context, categoryID int64) ([]TKCategoryField, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, category_id, key, label, kind, required, sort_order
		FROM tk_category_fields WHERE category_id = $1
		ORDER BY sort_order, key`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("list tk_category_fields: %w", err)
	}
	defer rows.Close()
	out := []TKCategoryField{}
	for rows.Next() {
		var f TKCategoryField
		if err := rows.Scan(&f.ID, &f.CategoryID, &f.Key, &f.Label, &f.Kind, &f.Required, &f.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) TKGetCategoryField(ctx context.Context, id int64) (TKCategoryField, error) {
	var f TKCategoryField
	err := s.pool.QueryRow(ctx, `
		SELECT id, category_id, key, label, kind, required, sort_order
		FROM tk_category_fields WHERE id = $1`, id).
		Scan(&f.ID, &f.CategoryID, &f.Key, &f.Label, &f.Kind, &f.Required, &f.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategoryField{}, ErrNotFound
	}
	return f, err
}

func (s *Store) TKCreateCategoryField(ctx context.Context, in TKCategoryField) (TKCategoryField, error) {
	var out TKCategoryField
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_category_fields (category_id, key, label, kind, required, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, category_id, key, label, kind, required, sort_order`,
		in.CategoryID, in.Key, in.Label, in.Kind, in.Required, in.SortOrder).
		Scan(&out.ID, &out.CategoryID, &out.Key, &out.Label, &out.Kind, &out.Required, &out.SortOrder)
	if err != nil {
		return TKCategoryField{}, fmt.Errorf("insert tk_category_field: %w", err)
	}
	return out, nil
}

func (s *Store) TKUpdateCategoryField(ctx context.Context, in TKCategoryField) (TKCategoryField, error) {
	var out TKCategoryField
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_category_fields SET
		  label = $2, kind = $3, required = $4, sort_order = $5
		WHERE id = $1
		RETURNING id, category_id, key, label, kind, required, sort_order`,
		in.ID, in.Label, in.Kind, in.Required, in.SortOrder).
		Scan(&out.ID, &out.CategoryID, &out.Key, &out.Label, &out.Kind, &out.Required, &out.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKCategoryField{}, ErrNotFound
	}
	return out, err
}

func (s *Store) TKDeleteCategoryField(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tk_category_fields WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tk_category_field: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
EOF
go build ./internal/store/...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/tk_categories.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): tk categories + subcategories + fields CRUD"
```

---

### Task B2: tk_tickets.go — ticket CRUD + sequence + filterable list

**Files:**
- Create: `internal/store/tk_tickets.go`

```bash
cat > internal/store/tk_tickets.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TKNextTrackingNumber atomically increments the sequence row and
// returns the new tracking number in the SUP-N format. Safe to call
// from multiple goroutines / connections — Postgres UPDATE … RETURNING
// is atomic.
func (s *Store) TKNextTrackingNumber(ctx context.Context) (string, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_ticket_sequence SET next_n = next_n + 1
		WHERE id = 1
		RETURNING next_n - 1`).Scan(&n)
	if err != nil {
		return "", fmt.Errorf("tk next tracking number: %w", err)
	}
	return fmt.Sprintf("SUP-%d", n), nil
}

// TKCreateTicket inserts a ticket. Tracking number must already be
// generated via TKNextTrackingNumber (handler concatenates the two so
// the row creation can be rolled back if the entry/field-value insert
// later in the same tx fails).
func (s *Store) TKCreateTicket(ctx context.Context, tx pgx.Tx, in TKTicket) (TKTicket, error) {
	var out TKTicket
	err := tx.QueryRow(ctx, `
		INSERT INTO tk_tickets (
		  tracking_number, customer_id, customer_email,
		  category_id, subcategory_id, subject, status)
		VALUES ($1,$2,$3,$4,$5,$6,'open')
		RETURNING id, tracking_number, customer_id, customer_email,
		          category_id, subcategory_id, subject, status,
		          assigned_admin_id, created_at, updated_at,
		          waiting_since, resolved_at`,
		in.TrackingNumber, in.CustomerID, in.CustomerEmail,
		in.CategoryID, in.SubcategoryID, in.Subject).
		Scan(&out.ID, &out.TrackingNumber, &out.CustomerID, &out.CustomerEmail,
			&out.CategoryID, &out.SubcategoryID, &out.Subject, &out.Status,
			&out.AssignedAdminID, &out.CreatedAt, &out.UpdatedAt,
			&out.WaitingSince, &out.ResolvedAt)
	if err != nil {
		return TKTicket{}, fmt.Errorf("insert tk_ticket: %w", err)
	}
	return out, nil
}

func (s *Store) TKGetTicketByTracking(ctx context.Context, tn string) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		SELECT id, tracking_number, customer_id, customer_email,
		       category_id, subcategory_id, subject, status,
		       assigned_admin_id, created_at, updated_at,
		       waiting_since, resolved_at
		FROM tk_tickets WHERE tracking_number = $1`, tn).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	if err != nil {
		return TKTicket{}, fmt.Errorf("get tk_ticket: %w", err)
	}
	return t, nil
}

func (s *Store) TKGetTicketByID(ctx context.Context, id int64) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		SELECT id, tracking_number, customer_id, customer_email,
		       category_id, subcategory_id, subject, status,
		       assigned_admin_id, created_at, updated_at,
		       waiting_since, resolved_at
		FROM tk_tickets WHERE id = $1`, id).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	return t, err
}

// TKListTickets serves both the customer and admin listings. The
// filter struct narrows which.
func (s *Store) TKListTickets(ctx context.Context, f TKTicketListFilter) ([]TKTicket, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	args := []any{}
	clauses := []string{}

	if f.CustomerID != "" {
		args = append(args, f.CustomerID)
		clauses = append(clauses, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	switch f.StatusGroup {
	case "active":
		clauses = append(clauses, "status IN ('open','in_progress','waiting_customer')")
	case "closed":
		clauses = append(clauses, "status = 'closed'")
	}
	if f.CategoryID > 0 {
		args = append(args, f.CategoryID)
		clauses = append(clauses, fmt.Sprintf("category_id = $%d", len(args)))
	}
	switch f.AssigneeID {
	case "__mine__":
		args = append(args, f.CallerAdminID)
		clauses = append(clauses, fmt.Sprintf("assigned_admin_id = $%d", len(args)))
	case "__unassigned__":
		clauses = append(clauses, "assigned_admin_id IS NULL")
	default:
		if f.AssigneeID != "" {
			args = append(args, f.AssigneeID)
			clauses = append(clauses, fmt.Sprintf("assigned_admin_id = $%d", len(args)))
		}
	}
	if f.Search != "" {
		args = append(args, f.Search+"%")
		args = append(args, "%"+f.Search+"%")
		clauses = append(clauses,
			fmt.Sprintf("(tracking_number ILIKE $%d OR subject ILIKE $%d)", len(args)-1, len(args)))
	}

	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`
		SELECT id, tracking_number, customer_id, customer_email,
		       category_id, subcategory_id, subject, status,
		       assigned_admin_id, created_at, updated_at,
		       waiting_since, resolved_at
		FROM tk_tickets
		%s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tk_tickets: %w", err)
	}
	defer rows.Close()
	out := []TKTicket{}
	for rows.Next() {
		var t TKTicket
		if err := rows.Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TKUpdateTicketStatus persists the lifecycle transition + side
// effects in a single UPDATE. The transition map (internal/tickets)
// validates BEFORE this is called.
func (s *Store) TKUpdateTicketStatus(ctx context.Context, id int64, newStatus string, waitingSince, resolvedAt *interface{}) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_tickets SET
		  status        = $2,
		  waiting_since = CASE WHEN $2 = 'waiting_customer' THEN NOW()
		                       WHEN $2 IN ('in_progress','open') THEN NULL
		                       ELSE waiting_since END,
		  resolved_at   = CASE WHEN $2 = 'resolved' THEN NOW()
		                       WHEN $2 = 'in_progress' THEN NULL
		                       ELSE resolved_at END,
		  updated_at    = NOW()
		WHERE id = $1
		RETURNING id, tracking_number, customer_id, customer_email,
		          category_id, subcategory_id, subject, status,
		          assigned_admin_id, created_at, updated_at,
		          waiting_since, resolved_at`, id, newStatus).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	if err != nil {
		return TKTicket{}, fmt.Errorf("update tk_ticket status: %w", err)
	}
	_ = waitingSince
	_ = resolvedAt
	return t, nil
}

// TKAssignTicket sets or clears assigned_admin_id. Pass nil to unassign.
func (s *Store) TKAssignTicket(ctx context.Context, id int64, adminID *string) (TKTicket, error) {
	var t TKTicket
	err := s.pool.QueryRow(ctx, `
		UPDATE tk_tickets SET assigned_admin_id = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, tracking_number, customer_id, customer_email,
		          category_id, subcategory_id, subject, status,
		          assigned_admin_id, created_at, updated_at,
		          waiting_since, resolved_at`, id, adminID).
		Scan(&t.ID, &t.TrackingNumber, &t.CustomerID, &t.CustomerEmail,
			&t.CategoryID, &t.SubcategoryID, &t.Subject, &t.Status,
			&t.AssignedAdminID, &t.CreatedAt, &t.UpdatedAt,
			&t.WaitingSince, &t.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKTicket{}, ErrNotFound
	}
	return t, err
}

// TKTouchTicket bumps updated_at — called from the entry-insert
// transaction so the queue sort stays current.
func (s *Store) TKTouchTicket(ctx context.Context, tx pgx.Tx, id int64) error {
	_, err := tx.Exec(ctx, `UPDATE tk_tickets SET updated_at = NOW() WHERE id = $1`, id)
	return err
}

// TKLoadTicketAux fills Category, Subcategory, and FieldValues onto a
// ticket. Customer + admin detail handlers call this after the basic
// row load.
func (s *Store) TKLoadTicketAux(ctx context.Context, t *TKTicket) error {
	cat, err := s.TKGetCategory(ctx, t.CategoryID)
	if err == nil {
		t.Category = &cat
	}
	if t.SubcategoryID != nil {
		sub, err := s.TKGetSubcategory(ctx, *t.SubcategoryID)
		if err == nil {
			t.Subcategory = &sub
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT f.id, f.key, f.label, fv.value
		FROM tk_ticket_field_values fv
		JOIN tk_category_fields f ON f.id = fv.field_id
		WHERE fv.ticket_id = $1
		ORDER BY f.sort_order`, t.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	t.FieldValues = []TKFieldValue{}
	for rows.Next() {
		var v TKFieldValue
		if err := rows.Scan(&v.FieldID, &v.FieldKey, &v.FieldLabel, &v.Value); err != nil {
			return err
		}
		t.FieldValues = append(t.FieldValues, v)
	}
	return rows.Err()
}

// TKInsertFieldValue is called inside the ticket-creation transaction.
func (s *Store) TKInsertFieldValue(ctx context.Context, tx pgx.Tx, ticketID, fieldID int64, value string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO tk_ticket_field_values (ticket_id, field_id, value)
		VALUES ($1, $2, $3)`, ticketID, fieldID, value)
	return err
}

// TKBegin starts a transaction. Tickets call this to atomically write
// (ticket + initial entry + field values).
func (s *Store) TKBegin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

// TKResolvedAtIdleSince returns tickets in 'resolved' state whose
// resolved_at is older than the cutoff. Used by the close-idle cron.
func (s *Store) TKResolvedAtIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM tk_tickets
		WHERE status = 'resolved'
		  AND resolved_at < NOW() - ($1 || ' days')::interval
		ORDER BY resolved_at
		LIMIT $2`, fmt.Sprintf("%d", cutoffDays), limit)
	if err != nil {
		return nil, fmt.Errorf("tk resolved idle: %w", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// TKWaitingIdleSince returns tickets in 'waiting_customer' state whose
// waiting_since is older than the cutoff.
func (s *Store) TKWaitingIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM tk_tickets
		WHERE status = 'waiting_customer'
		  AND waiting_since < NOW() - ($1 || ' days')::interval
		ORDER BY waiting_since
		LIMIT $2`, fmt.Sprintf("%d", cutoffDays), limit)
	if err != nil {
		return nil, fmt.Errorf("tk waiting idle: %w", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
EOF
go build ./internal/store/...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/tk_tickets.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): tk tickets — CRUD + sequence + status updates + cron-list helpers"
```

---

### Task B3: tk_entries.go + tk_attachments.go

**Files:**
- Create: `internal/store/tk_entries.go`
- Create: `internal/store/tk_attachments.go`

```bash
cat > internal/store/tk_entries.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TKInsertEntry writes an append-only entry on a ticket. Used inside
// transactions (initial entry + ticket create) and outside (admin
// replies, internal notes, status_change auto-entries).
func (s *Store) TKInsertEntry(ctx context.Context, tx pgx.Tx, e TKEntry) (TKEntry, error) {
	var out TKEntry
	err := tx.QueryRow(ctx, `
		INSERT INTO tk_ticket_entries (ticket_id, kind, author_id, author_role, body)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, ticket_id, kind, author_id, author_role, body, created_at`,
		e.TicketID, e.Kind, e.AuthorID, e.AuthorRole, e.Body).
		Scan(&out.ID, &out.TicketID, &out.Kind, &out.AuthorID, &out.AuthorRole, &out.Body, &out.CreatedAt)
	if err != nil {
		return TKEntry{}, fmt.Errorf("insert tk_ticket_entry: %w", err)
	}
	return out, nil
}

// TKInsertEntryNoTx is the convenience wrapper for callers that don't
// need a transaction (most reply / note paths).
func (s *Store) TKInsertEntryNoTx(ctx context.Context, e TKEntry) (TKEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TKEntry{}, err
	}
	defer tx.Rollback(ctx)
	saved, err := s.TKInsertEntry(ctx, tx, e)
	if err != nil {
		return TKEntry{}, err
	}
	if err := s.TKTouchTicket(ctx, tx, e.TicketID); err != nil {
		return TKEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TKEntry{}, err
	}
	return saved, nil
}

// TKListEntries returns ticket entries oldest-first. customerView
// filters out internal_note rows for the customer detail endpoint.
func (s *Store) TKListEntries(ctx context.Context, ticketID int64, customerView bool) ([]TKEntry, error) {
	q := `SELECT id, ticket_id, kind, author_id, author_role, body, created_at
	      FROM tk_ticket_entries WHERE ticket_id = $1`
	if customerView {
		q += ` AND kind != 'internal_note'`
	}
	q += ` ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list tk_ticket_entries: %w", err)
	}
	defer rows.Close()
	out := []TKEntry{}
	for rows.Next() {
		var e TKEntry
		if err := rows.Scan(&e.ID, &e.TicketID, &e.Kind, &e.AuthorID, &e.AuthorRole, &e.Body, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attach attachment metadata.
	for i := range out {
		atts, err := s.TKListEntryAttachments(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Attachments = atts
	}
	return out, nil
}

// TKGetEntry — used by the attachment-upload handler to verify the
// caller owns the parent ticket (customers) or is admin.
func (s *Store) TKGetEntry(ctx context.Context, id int64) (TKEntry, error) {
	var e TKEntry
	err := s.pool.QueryRow(ctx, `
		SELECT id, ticket_id, kind, author_id, author_role, body, created_at
		FROM tk_ticket_entries WHERE id = $1`, id).
		Scan(&e.ID, &e.TicketID, &e.Kind, &e.AuthorID, &e.AuthorRole, &e.Body, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKEntry{}, ErrNotFound
	}
	return e, err
}
EOF

cat > internal/store/tk_attachments.go <<'EOF'
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TKInsertAttachment writes the bytes alongside the entry that
// introduced them. Caller has already capped at 10 MB.
func (s *Store) TKInsertAttachment(ctx context.Context, a TKAttachment) (TKAttachmentMeta, error) {
	var out TKAttachmentMeta
	err := s.pool.QueryRow(ctx, `
		INSERT INTO tk_attachments (entry_id, filename, mime, bytes, content_bytea, sha256)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, filename, mime, bytes, created_at`,
		a.EntryID, a.Filename, a.MIME, a.Bytes, a.Content, a.SHA256).
		Scan(&out.ID, &out.Filename, &out.MIME, &out.Bytes, &out.CreatedAt)
	if err != nil {
		return TKAttachmentMeta{}, fmt.Errorf("insert tk_attachment: %w", err)
	}
	return out, nil
}

// TKGetAttachment streams bytes for the serve handler.
func (s *Store) TKGetAttachment(ctx context.Context, id int64) (TKAttachment, error) {
	var a TKAttachment
	err := s.pool.QueryRow(ctx, `
		SELECT id, entry_id, filename, mime, bytes, content_bytea, sha256, created_at
		FROM tk_attachments WHERE id = $1`, id).
		Scan(&a.ID, &a.EntryID, &a.Filename, &a.MIME, &a.Bytes, &a.Content, &a.SHA256, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TKAttachment{}, ErrNotFound
	}
	return a, err
}

// TKListEntryAttachments returns metadata only — used by entry-list
// queries to attach pointers without streaming bytes.
func (s *Store) TKListEntryAttachments(ctx context.Context, entryID int64) ([]TKAttachmentMeta, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, filename, mime, bytes, created_at
		FROM tk_attachments WHERE entry_id = $1
		ORDER BY id`, entryID)
	if err != nil {
		return nil, fmt.Errorf("list entry attachments: %w", err)
	}
	defer rows.Close()
	out := []TKAttachmentMeta{}
	for rows.Next() {
		var m TKAttachmentMeta
		if err := rows.Scan(&m.ID, &m.Filename, &m.MIME, &m.Bytes, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Suppress import-unused warnings when this file is the only one using pgx.
var _ = pgx.ErrNoRows
EOF

go build ./internal/store/...
GOWORK=off go build ./...

git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/store/tk_entries.go internal/store/tk_attachments.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(store): tk entries + attachments (metadata projection + bytes serve)"
```

---

## Phase C — Lifecycle

### Task C1: `internal/tickets/lifecycle.go` + tests

**Files:**
- Create: `internal/tickets/lifecycle.go`
- Create: `internal/tickets/lifecycle_test.go`

The transition map is the single source of truth for what's allowed. Handlers consult it BEFORE calling the store update. Returns clear errors that handlers map to 409.

- [ ] **Step 1: Failing tests**

```bash
cd /opt/continuum_plugins/continuum-plugin-support
mkdir -p internal/tickets
cat > internal/tickets/lifecycle_test.go <<'EOF'
package tickets

import (
	"strings"
	"testing"
	"time"
)

func TestAllowedTransitions(t *testing.T) {
	cases := []struct {
		from, to string
		trigger  Trigger
	}{
		{"open", "in_progress", TriggerAdminReply},
		{"in_progress", "waiting_customer", TriggerAdminStatus},
		{"waiting_customer", "in_progress", TriggerCustomerReply},
		{"in_progress", "resolved", TriggerAdminStatus},
		{"waiting_customer", "resolved", TriggerAdminStatus},
		{"resolved", "in_progress", TriggerCustomerReopen},
		{"resolved", "closed", TriggerCronIdle},
		{"waiting_customer", "closed", TriggerCronIdle},
		{"open", "closed", TriggerAdminStatus},
		{"in_progress", "closed", TriggerAdminStatus},
	}
	for _, c := range cases {
		if err := AllowTransition(c.from, c.to, c.trigger, time.Now()); err != nil {
			t.Errorf("AllowTransition(%s -> %s via %v) = %v, want nil", c.from, c.to, c.trigger, err)
		}
	}
}

func TestForbiddenTransitionsAreRejected(t *testing.T) {
	cases := []struct{ from, to string }{
		{"closed", "in_progress"},        // closed is terminal
		{"closed", "open"},
		{"open", "resolved"},             // must go through in_progress
		{"open", "waiting_customer"},     // ditto
		{"waiting_customer", "open"},
		{"resolved", "waiting_customer"},
	}
	for _, c := range cases {
		err := AllowTransition(c.from, c.to, TriggerAdminStatus, time.Now())
		if err == nil {
			t.Errorf("AllowTransition(%s -> %s) accepted; want rejected", c.from, c.to)
		}
	}
}

func TestReopenWindowEnforced(t *testing.T) {
	// 7-day reopen window.
	recent := time.Now().Add(-6 * 24 * time.Hour)
	old := time.Now().Add(-8 * 24 * time.Hour)

	if err := AllowReopen(recent); err != nil {
		t.Errorf("AllowReopen(6d ago) = %v, want nil", err)
	}
	err := AllowReopen(old)
	if err == nil {
		t.Errorf("AllowReopen(8d ago) accepted; want rejected")
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("AllowReopen error %q should mention 7-day window", err)
	}
}

func TestUnknownStatusRejected(t *testing.T) {
	if err := AllowTransition("open", "frobnicated", TriggerAdminStatus, time.Now()); err == nil {
		t.Errorf("unknown target accepted")
	}
}
EOF
go test ./internal/tickets/... 2>&1 | tail -5   # expect undefined
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/tickets/lifecycle.go <<'EOF'
// Package tickets holds the lifecycle transition map, the cron pass,
// and event-payload assembly helpers — the bits that aren't store
// or HTTP handlers.
package tickets

import (
	"fmt"
	"time"
)

// Trigger names the cause of a transition. The map is keyed by
// (from, to, trigger) so we can reject e.g. an admin trying to
// reopen via the "resolved -> in_progress" edge (only the customer
// can do that).
type Trigger int

const (
	TriggerAdminReply Trigger = iota
	TriggerAdminStatus
	TriggerCustomerReply
	TriggerCustomerReopen
	TriggerCronIdle
)

// ReopenWindow is the spec-defined limit on customer reopens.
const ReopenWindow = 7 * 24 * time.Hour

// Map of allowed transitions. Anything not here is forbidden.
var allowed = map[string]map[string]map[Trigger]bool{
	"open": {
		"in_progress":      {TriggerAdminReply: true},
		"closed":           {TriggerAdminStatus: true},
	},
	"in_progress": {
		"waiting_customer": {TriggerAdminStatus: true},
		"resolved":         {TriggerAdminStatus: true},
		"closed":           {TriggerAdminStatus: true},
	},
	"waiting_customer": {
		"in_progress":      {TriggerCustomerReply: true},
		"resolved":         {TriggerAdminStatus: true},
		"closed":           {TriggerAdminStatus: true, TriggerCronIdle: true},
	},
	"resolved": {
		"in_progress":      {TriggerCustomerReopen: true},
		"closed":           {TriggerAdminStatus: true, TriggerCronIdle: true},
	},
	// closed is terminal — no outgoing edges.
}

// AllowTransition validates a status change. Returns nil if allowed,
// otherwise a descriptive error suitable for the handler to map to 409.
// `now` is passed in so callers (cron, tests) can drive deterministic
// time without touching globals.
func AllowTransition(from, to string, trigger Trigger, _ time.Time) error {
	if from == to {
		return fmt.Errorf("ticket already in status %q", to)
	}
	tos, ok := allowed[from]
	if !ok {
		return fmt.Errorf("ticket in unknown or terminal status %q", from)
	}
	triggers, ok := tos[to]
	if !ok {
		return fmt.Errorf("transition %s -> %s is not allowed", from, to)
	}
	if !triggers[trigger] {
		return fmt.Errorf("transition %s -> %s not allowed via this trigger", from, to)
	}
	return nil
}

// AllowReopen enforces the 7-day customer-reopen window. Pass the
// ticket's resolved_at; nil-resolved-at is always rejected.
func AllowReopen(resolvedAt time.Time) error {
	if resolvedAt.IsZero() {
		return fmt.Errorf("ticket has no resolved_at; cannot reopen")
	}
	if time.Since(resolvedAt) > ReopenWindow {
		return fmt.Errorf("reopen window (7 days) has elapsed; please open a new ticket")
	}
	return nil
}
EOF
go test ./internal/tickets/... -v   # 4 tests must pass
GOWORK=off go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/tickets/lifecycle.go internal/tickets/lifecycle_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(tickets): lifecycle transition map + reopen window"
```

---

## Phase D — Cron (auto-close)

### Task D1: `internal/tickets/cron.go` + tests

**Files:**
- Create: `internal/tickets/cron.go`
- Create: `internal/tickets/cron_test.go`

```bash
cat > internal/tickets/cron_test.go <<'EOF'
package tickets

import (
	"context"
	"testing"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

type fakeStore struct {
	resolved       []int64
	waiting        []int64
	closed         map[int64]string // id -> reason ("resolved_idle" | "waiting_idle")
	tickets        map[int64]store.TKTicket
	systemEntries  []store.TKEntry
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		closed:        map[int64]string{},
		tickets:       map[int64]store.TKTicket{},
		systemEntries: []store.TKEntry{},
	}
}

func (f *fakeStore) TKResolvedAtIdleSince(_ context.Context, _ int, _ int) ([]int64, error) {
	return f.resolved, nil
}
func (f *fakeStore) TKWaitingIdleSince(_ context.Context, _ int, _ int) ([]int64, error) {
	return f.waiting, nil
}
func (f *fakeStore) TKGetTicketByID(_ context.Context, id int64) (store.TKTicket, error) {
	if t, ok := f.tickets[id]; ok { return t, nil }
	return store.TKTicket{ID: id, Status: "resolved", TrackingNumber: "SUP-X"}, nil
}
func (f *fakeStore) TKUpdateTicketStatus(_ context.Context, id int64, newStatus string, _, _ *interface{}) (store.TKTicket, error) {
	t := f.tickets[id]
	t.ID = id
	t.Status = newStatus
	f.tickets[id] = t
	return t, nil
}
func (f *fakeStore) TKInsertEntryNoTx(_ context.Context, e store.TKEntry) (store.TKEntry, error) {
	f.systemEntries = append(f.systemEntries, e)
	return e, nil
}

type fakeEmitter struct{ events []string }
func (f *fakeEmitter) PublishTicketEvent(_ context.Context, name string, _ store.TKTicket, _ map[string]any) {
	f.events = append(f.events, name)
}

func TestCloseIdleClosesResolvedAndWaiting(t *testing.T) {
	s := newFakeStore()
	s.resolved = []int64{1, 2}
	s.waiting = []int64{3}
	s.tickets[1] = store.TKTicket{ID: 1, Status: "resolved", TrackingNumber: "SUP-1"}
	s.tickets[2] = store.TKTicket{ID: 2, Status: "resolved", TrackingNumber: "SUP-2"}
	s.tickets[3] = store.TKTicket{ID: 3, Status: "waiting_customer", TrackingNumber: "SUP-3"}
	em := &fakeEmitter{}
	c := &Cron{Store: s, Emitter: em, Enabled: true, ResolvedAfterDays: 7, WaitingAfterDays: 14}
	if err := c.CloseIdle(context.Background()); err != nil {
		t.Fatalf("CloseIdle: %v", err)
	}
	if s.tickets[1].Status != "closed" || s.tickets[2].Status != "closed" || s.tickets[3].Status != "closed" {
		t.Fatalf("expected all 3 closed; got %+v", s.tickets)
	}
	if len(em.events) != 3 {
		t.Fatalf("expected 3 ticket_closed events; got %d", len(em.events))
	}
	for _, ev := range em.events {
		if ev != "ticket_closed" {
			t.Fatalf("unexpected event %q", ev)
		}
	}
	if len(s.systemEntries) != 3 {
		t.Fatalf("expected one system entry per close; got %d", len(s.systemEntries))
	}
}

func TestCloseIdleNoOpWhenDisabled(t *testing.T) {
	s := newFakeStore()
	s.resolved = []int64{1}
	s.waiting = []int64{2}
	s.tickets[1] = store.TKTicket{ID: 1, Status: "resolved"}
	s.tickets[2] = store.TKTicket{ID: 2, Status: "waiting_customer"}
	c := &Cron{Store: s, Emitter: &fakeEmitter{}, Enabled: false, ResolvedAfterDays: 7, WaitingAfterDays: 14}
	if err := c.CloseIdle(context.Background()); err != nil { t.Fatal(err) }
	if s.tickets[1].Status == "closed" || s.tickets[2].Status == "closed" {
		t.Fatalf("disabled cron should be a no-op; got %+v", s.tickets)
	}
}

func TestCloseIdleZeroDaysSkipsThatPass(t *testing.T) {
	s := newFakeStore()
	s.resolved = []int64{1}
	s.waiting = []int64{2}
	s.tickets[1] = store.TKTicket{ID: 1, Status: "resolved"}
	s.tickets[2] = store.TKTicket{ID: 2, Status: "waiting_customer"}
	// ResolvedAfterDays = 0 → that pass is skipped, but waiting still runs.
	c := &Cron{Store: s, Emitter: &fakeEmitter{}, Enabled: true, ResolvedAfterDays: 0, WaitingAfterDays: 14}
	if err := c.CloseIdle(context.Background()); err != nil { t.Fatal(err) }
	if s.tickets[1].Status == "closed" {
		t.Fatalf("resolved-pass should have been skipped (days=0)")
	}
	if s.tickets[2].Status != "closed" {
		t.Fatalf("waiting-pass should have closed ticket 2")
	}
}
EOF
go test ./internal/tickets/... 2>&1 | tail -5   # expect undefined Cron
```

- [ ] **Step 2: Implementation**

```bash
cat > internal/tickets/cron.go <<'EOF'
package tickets

import (
	"context"
	"time"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

// CronStore is the slice of *store.Store the cron needs.
type CronStore interface {
	TKResolvedAtIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error)
	TKWaitingIdleSince(ctx context.Context, cutoffDays int, limit int) ([]int64, error)
	TKGetTicketByID(ctx context.Context, id int64) (store.TKTicket, error)
	TKUpdateTicketStatus(ctx context.Context, id int64, newStatus string, waitingSince, resolvedAt *interface{}) (store.TKTicket, error)
	TKInsertEntryNoTx(ctx context.Context, e store.TKEntry) (store.TKEntry, error)
}

// Emitter mirrors the server's tk event publisher without making
// this package depend on the server.
type Emitter interface {
	PublishTicketEvent(ctx context.Context, name string, t store.TKTicket, extra map[string]any)
}

// Cron groups the per-day auto-close behaviour. Enabled, plus per-
// pass day thresholds, come from plugin config (defaults documented
// in the spec: enabled=true, resolved=7, waiting=14).
type Cron struct {
	Store              CronStore
	Emitter            Emitter
	Enabled            bool
	ResolvedAfterDays  int
	WaitingAfterDays   int
}

// CloseIdle runs both passes (resolved + waiting). No-op when Enabled
// is false. When either day threshold is 0, that pass is skipped.
func (c *Cron) CloseIdle(ctx context.Context) error {
	if !c.Enabled {
		return nil
	}
	if c.ResolvedAfterDays > 0 {
		ids, err := c.Store.TKResolvedAtIdleSince(ctx, c.ResolvedAfterDays, 200)
		if err != nil { return err }
		for _, id := range ids {
			c.closeOne(ctx, id, "resolved_idle")
		}
	}
	if c.WaitingAfterDays > 0 {
		ids, err := c.Store.TKWaitingIdleSince(ctx, c.WaitingAfterDays, 200)
		if err != nil { return err }
		for _, id := range ids {
			c.closeOne(ctx, id, "waiting_idle")
		}
	}
	return nil
}

func (c *Cron) closeOne(ctx context.Context, id int64, reason string) {
	updated, err := c.Store.TKUpdateTicketStatus(ctx, id, "closed", nil, nil)
	if err != nil {
		return
	}
	_, _ = c.Store.TKInsertEntryNoTx(ctx, store.TKEntry{
		TicketID: id, Kind: "system", AuthorID: "system", AuthorRole: "system",
		Body: "Ticket auto-closed by cron (" + reason + ")",
	})
	c.Emitter.PublishTicketEvent(ctx, "ticket_closed", updated, map[string]any{
		"by":     "system",
		"reason": reason,
	})
}

var _ = time.Time{} // imported for future use of timeouts on per-id ops
EOF
go test ./internal/tickets/... -v   # 3 cron tests pass (plus 4 lifecycle = 7)
GOWORK=off go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/tickets/cron.go internal/tickets/cron_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(tickets): auto-close cron with operator-configurable enable + day thresholds"
```

---

## Phase E — Customer handlers

### Task E1: `handlers_tk_customer.go`

**Files:**
- Create: `internal/server/handlers_tk_customer.go`

This file covers the customer surface: SPA shells (list + detail), categories form lookup, ticket create / list / detail / reply / reopen. The attachment upload is its own handler (Phase G).

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cat > internal/server/handlers_tk_customer.go <<'EOF'
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
	"github.com/RXWatcher/continuum-plugin-support/internal/tickets"
)

func tkCustomerStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// SPA shell handlers.
func hTKListPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-list", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hTKDetailPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-detail", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}

// --- Categories (form rendering) -----------------------------------

type tkCategoriesResponse struct {
	Categories    []store.TKCategory               `json:"categories"`
	Subcategories map[int64][]store.TKSubcategory  `json:"subcategories"`
	Fields        map[int64][]store.TKCategoryField `json:"fields"`
}

func hTKCategoriesForCustomer(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st := tkCustomerStore(d)
		cats, err := st.TKListCategories(r.Context(), true)
		if err != nil {
			writeInternal(w, r, d, "tk_categories_failed", err); return
		}
		out := tkCategoriesResponse{
			Categories:    cats,
			Subcategories: map[int64][]store.TKSubcategory{},
			Fields:        map[int64][]store.TKCategoryField{},
		}
		for _, c := range cats {
			subs, err := st.TKListSubcategories(r.Context(), c.ID, true)
			if err == nil { out.Subcategories[c.ID] = subs }
			fields, err := st.TKListCategoryFields(r.Context(), c.ID)
			if err == nil { out.Fields[c.ID] = fields }
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// --- Customer ticket list ------------------------------------------

func hTKCustomerList(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.TKTicketListFilter{
			CustomerID:  r.Header.Get("X-Continuum-User-Id"),
			StatusGroup: r.URL.Query().Get("statusGroup"),
			Limit:       parseLimit(r.URL.Query().Get("limit"), 100),
			Offset:      parseInt(r.URL.Query().Get("offset")),
		}
		out, err := tkCustomerStore(d).TKListTickets(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "tk_list_failed", err); return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// --- Customer ticket detail (excludes internal notes) --------------

type tkDetailResponse struct {
	Ticket  store.TKTicket  `json:"ticket"`
	Entries []store.TKEntry `json:"entries"`
}

func hTKCustomerDetail(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkCustomerStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err); return
		}
		if t.CustomerID != r.Header.Get("X-Continuum-User-Id") {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err := st.TKLoadTicketAux(r.Context(), &t); err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err); return
		}
		entries, err := st.TKListEntries(r.Context(), t.ID, true) // customer view (no internal_notes)
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err); return
		}
		writeJSON(w, http.StatusOK, tkDetailResponse{Ticket: t, Entries: entries})
	}
}

// --- Customer creates a ticket -------------------------------------

type tkCreateRequest struct {
	CategoryID    int64             `json:"categoryId"`
	SubcategoryID *int64            `json:"subcategoryId,omitempty"`
	Subject       string            `json:"subject"`
	Body          string            `json:"body"`
	FieldValues   map[string]string `json:"fieldValues,omitempty"`
	CustomerEmail string            `json:"customerEmail"`
}

func hTKCustomerCreate(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Body) == "" || req.CategoryID == 0 {
			writeErr(w, http.StatusBadRequest, "tk_bad_ticket", "subject, body, and categoryId are required"); return
		}
		if strings.TrimSpace(req.CustomerEmail) == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_email", "customerEmail is required"); return
		}

		st := tkCustomerStore(d)

		// Validate required category fields are present.
		fields, err := st.TKListCategoryFields(r.Context(), req.CategoryID)
		if err != nil {
			writeInternal(w, r, d, "tk_categories_failed", err); return
		}
		for _, f := range fields {
			if f.Required && strings.TrimSpace(req.FieldValues[f.Key]) == "" {
				writeErr(w, http.StatusBadRequest, "tk_missing_field", "required field missing: "+f.Key); return
			}
		}

		tn, err := st.TKNextTrackingNumber(r.Context())
		if err != nil {
			writeInternal(w, r, d, "tk_tn_failed", err); return
		}

		tx, err := st.TKBegin(r.Context())
		if err != nil {
			writeInternal(w, r, d, "tk_tx_failed", err); return
		}
		defer tx.Rollback(r.Context())

		saved, err := st.TKCreateTicket(r.Context(), tx, store.TKTicket{
			TrackingNumber: tn,
			CustomerID:     r.Header.Get("X-Continuum-User-Id"),
			CustomerEmail:  req.CustomerEmail,
			CategoryID:     req.CategoryID,
			SubcategoryID:  req.SubcategoryID,
			Subject:        req.Subject,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_create_failed", err); return
		}

		// Initial entry.
		_, err = st.TKInsertEntry(r.Context(), tx, store.TKEntry{
			TicketID:   saved.ID,
			Kind:       "initial",
			AuthorID:   saved.CustomerID,
			AuthorRole: "customer",
			Body:       req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_entry_failed", err); return
		}

		// Field values.
		for _, f := range fields {
			if v, ok := req.FieldValues[f.Key]; ok && v != "" {
				if err := st.TKInsertFieldValue(r.Context(), tx, saved.ID, f.ID, v); err != nil {
					writeInternal(w, r, d, "tk_field_value_failed", err); return
				}
			}
		}

		if err := tx.Commit(r.Context()); err != nil {
			writeInternal(w, r, d, "tk_commit_failed", err); return
		}

		tkPublishEvent(d, "ticket_submitted", saved, nil)
		writeJSON(w, http.StatusOK, saved)
	}
}

// --- Customer reply ------------------------------------------------

type tkReplyRequest struct {
	Body string `json:"body"`
}

func hTKCustomerReply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkCustomerStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err); return
		}
		if t.CustomerID != r.Header.Get("X-Continuum-User-Id") {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if t.Status == "closed" {
			writeErr(w, http.StatusConflict, "tk_closed", "ticket is closed; please open a new one"); return
		}
		var req tkReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if strings.TrimSpace(req.Body) == "" {
			writeErr(w, http.StatusBadRequest, "tk_empty_body", "reply body cannot be empty"); return
		}

		entry, err := st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID:   t.ID,
			Kind:       "reply",
			AuthorID:   t.CustomerID,
			AuthorRole: "customer",
			Body:       req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err); return
		}

		// Status transition: waiting_customer -> in_progress (customer reply).
		if t.Status == "waiting_customer" {
			if err := tickets.AllowTransition(t.Status, "in_progress", tickets.TriggerCustomerReply, timeNow()); err == nil {
				updated, _ := st.TKUpdateTicketStatus(r.Context(), t.ID, "in_progress", nil, nil)
				tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
					"from": t.Status, "to": "in_progress", "by": "customer",
				})
				t = updated
			}
		}

		tkPublishEvent(d, "ticket_replied", t, map[string]any{
			"author_role": "customer", "author_id": t.CustomerID,
			"excerpt": excerpt(req.Body, 280),
		})

		writeJSON(w, http.StatusOK, map[string]any{"entry": entry, "ticket": t})
	}
}

// --- Customer reopen -----------------------------------------------

func hTKCustomerReopen(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkCustomerStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_reopen_failed", err); return
		}
		if t.CustomerID != r.Header.Get("X-Continuum-User-Id") {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err := tickets.AllowTransition(t.Status, "in_progress", tickets.TriggerCustomerReopen, timeNow()); err != nil {
			writeErr(w, http.StatusConflict, "tk_reopen_denied", err.Error()); return
		}
		if t.ResolvedAt == nil {
			writeErr(w, http.StatusConflict, "tk_reopen_denied", "ticket has no resolved_at"); return
		}
		if err := tickets.AllowReopen(*t.ResolvedAt); err != nil {
			writeErr(w, http.StatusConflict, "tk_reopen_window", err.Error()); return
		}
		updated, err := st.TKUpdateTicketStatus(r.Context(), t.ID, "in_progress", nil, nil)
		if err != nil {
			writeInternal(w, r, d, "tk_reopen_failed", err); return
		}
		_, _ = st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "system", AuthorID: "system", AuthorRole: "system",
			Body: "Reopened by customer",
		})
		tkPublishEvent(d, "ticket_reopened", updated, map[string]any{"by": "customer"})
		tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
			"from": "resolved", "to": "in_progress", "by": "customer",
		})
		writeJSON(w, http.StatusOK, updated)
	}
}

// excerpt is a tiny helper for event payloads. Lives here so the
// admin handlers can reuse it without round-tripping the file.
func excerpt(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "…"
}
EOF
```

The file references `tkPublishEvent` (Phase H) and inherits `timeNow` from KB-era helpers (`kb_helpers.go`). Won't compile yet — verify after Phase H lands.

Don't commit yet — wait for the events stub.

---

### Task E2: tk_events.go stub + commit E1 together

```bash
cat > internal/server/tk_events.go <<'EOF'
package server

import (
	"context"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

// tkPublishEvent assembles the base ticket payload + extra keys and
// hands off to Deps.EventPublisher. No-ops when EventPublisher is nil.
func tkPublishEvent(d Deps, name string, t store.TKTicket, extra map[string]any) {
	if d.EventPublisher == nil {
		return
	}
	payload := map[string]any{
		"ticket_id":         t.ID,
		"tracking_number":   t.TrackingNumber,
		"subject":           t.Subject,
		"status":            t.Status,
		"customer_id":       t.CustomerID,
		"customer_email":    t.CustomerEmail,
		"assigned_admin_id": t.AssignedAdminID,
		"deep_link":         "/tickets/" + t.TrackingNumber,
	}
	if t.Category != nil {
		payload["category"] = map[string]any{
			"id": t.Category.ID, "slug": t.Category.Slug, "name": t.Category.Name,
		}
	}
	if t.Subcategory != nil {
		payload["subcategory"] = map[string]any{
			"id": t.Subcategory.ID, "slug": t.Subcategory.Slug, "name": t.Subcategory.Name,
		}
	}
	for k, v := range extra {
		payload[k] = v
	}
	if err := d.EventPublisher.PublishEvent(context.Background(),
		"plugin.continuum.support."+name, payload); err != nil && d.Logger != nil {
		d.Logger.Warn("ticket event publish failed", "event", name, "err", err)
	}
}
EOF

go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_tk_customer.go internal/server/tk_events.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): tickets customer handlers + event publisher"
```

---

## Phase F — Admin handlers

### Task F1: `handlers_tk_admin.go`

**Files:**
- Create: `internal/server/handlers_tk_admin.go`

The admin file is the longest: queue, detail (includes internal notes), reply, internal note, status change, assign, plus category / subcategory / field CRUD.

```bash
cat > internal/server/handlers_tk_admin.go <<'EOF'
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
	"github.com/RXWatcher/continuum-plugin-support/internal/tickets"
)

func tkAdminStore(d Deps) *store.Store {
	if cs, ok := d.ConfigStore.(*store.Store); ok {
		return cs
	}
	return nil
}

// Admin SPA shells.
func hTKAdminQueuePage(d Deps) http.HandlerFunc      { return adminSPAHandler(d, "admin-tickets-queue") }
func hTKAdminDetailPage(d Deps) http.HandlerFunc     { return adminSPAHandler(d, "admin-tickets-detail") }
func hTKAdminCategoriesPage(d Deps) http.HandlerFunc { return adminSPAHandler(d, "admin-tickets-categories") }

// --- Queue ---------------------------------------------------------

func hTKAdminQueue(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.TKTicketListFilter{
			Status:        r.URL.Query().Get("status"),
			StatusGroup:   r.URL.Query().Get("statusGroup"),
			CategoryID:    parseInt64(r.URL.Query().Get("categoryId")),
			AssigneeID:    r.URL.Query().Get("assignee"),
			CallerAdminID: r.Header.Get("X-Continuum-User-Id"),
			Search:        r.URL.Query().Get("q"),
			Limit:         parseLimit(r.URL.Query().Get("limit"), 200),
			Offset:        parseInt(r.URL.Query().Get("offset")),
		}
		out, err := tkAdminStore(d).TKListTickets(r.Context(), f)
		if err != nil {
			writeInternal(w, r, d, "tk_queue_failed", err); return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// --- Admin detail (includes internal notes) ------------------------

func hTKAdminDetail(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err); return
		}
		if err := st.TKLoadTicketAux(r.Context(), &t); err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err); return
		}
		entries, err := st.TKListEntries(r.Context(), t.ID, false) // admin view
		if err != nil {
			writeInternal(w, r, d, "tk_detail_failed", err); return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ticket": t, "entries": entries})
	}
}

// --- Admin reply (visible) -----------------------------------------

func hTKAdminReply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err); return
		}
		if t.Status == "closed" {
			writeErr(w, http.StatusConflict, "tk_closed", "ticket is closed"); return
		}
		var req struct{ Body string `json:"body"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if strings.TrimSpace(req.Body) == "" {
			writeErr(w, http.StatusBadRequest, "tk_empty_body", "reply body cannot be empty"); return
		}
		adminID := r.Header.Get("X-Continuum-User-Id")
		entry, err := st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "reply", AuthorID: adminID, AuthorRole: "admin", Body: req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_reply_failed", err); return
		}

		// open -> in_progress on first admin reply.
		if t.Status == "open" {
			if err := tickets.AllowTransition(t.Status, "in_progress", tickets.TriggerAdminReply, timeNow()); err == nil {
				updated, _ := st.TKUpdateTicketStatus(r.Context(), t.ID, "in_progress", nil, nil)
				tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
					"from": t.Status, "to": "in_progress", "by": adminID,
				})
				t = updated
			}
		}

		tkPublishEvent(d, "ticket_replied", t, map[string]any{
			"author_role": "admin", "author_id": adminID, "excerpt": excerpt(req.Body, 280),
		})
		writeJSON(w, http.StatusOK, map[string]any{"entry": entry, "ticket": t})
	}
}

// --- Admin internal note -------------------------------------------

func hTKAdminNote(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_note_failed", err); return
		}
		var req struct{ Body string `json:"body"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if strings.TrimSpace(req.Body) == "" {
			writeErr(w, http.StatusBadRequest, "tk_empty_body", "note body cannot be empty"); return
		}
		entry, err := st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "internal_note",
			AuthorID: r.Header.Get("X-Continuum-User-Id"), AuthorRole: "admin", Body: req.Body,
		})
		if err != nil {
			writeInternal(w, r, d, "tk_note_failed", err); return
		}
		writeJSON(w, http.StatusOK, entry)
	}
}

// --- Admin status change -------------------------------------------

func hTKAdminStatus(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_status_failed", err); return
		}
		var req struct{ To string `json:"to"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if err := tickets.AllowTransition(t.Status, req.To, tickets.TriggerAdminStatus, timeNow()); err != nil {
			writeErr(w, http.StatusConflict, "tk_bad_transition", err.Error()); return
		}
		adminID := r.Header.Get("X-Continuum-User-Id")
		updated, err := st.TKUpdateTicketStatus(r.Context(), t.ID, req.To, nil, nil)
		if err != nil {
			writeInternal(w, r, d, "tk_status_failed", err); return
		}
		_, _ = st.TKInsertEntryNoTx(r.Context(), store.TKEntry{
			TicketID: t.ID, Kind: "status_change", AuthorID: adminID, AuthorRole: "admin",
			Body: "Status changed: " + t.Status + " → " + req.To,
		})
		tkPublishEvent(d, "ticket_status_changed", updated, map[string]any{
			"from": t.Status, "to": req.To, "by": adminID,
		})
		if req.To == "resolved" {
			tkPublishEvent(d, "ticket_resolved", updated, map[string]any{"by": adminID})
		}
		if req.To == "closed" {
			tkPublishEvent(d, "ticket_closed", updated, map[string]any{"by": adminID})
		}
		writeJSON(w, http.StatusOK, updated)
	}
}

// --- Admin assign --------------------------------------------------

func hTKAdminAssign(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tn := chi.URLParam(r, "tracking_number")
		st := tkAdminStore(d)
		t, err := st.TKGetTicketByTracking(r.Context(), tn)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "ticket not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_assign_failed", err); return
		}
		var req struct{ AdminID *string `json:"adminId"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		updated, err := st.TKAssignTicket(r.Context(), t.ID, req.AdminID)
		if err != nil {
			writeInternal(w, r, d, "tk_assign_failed", err); return
		}
		tkPublishEvent(d, "ticket_assigned", updated, map[string]any{
			"from_admin_id": t.AssignedAdminID, "to_admin_id": req.AdminID,
		})
		writeJSON(w, http.StatusOK, updated)
	}
}

// --- Admin cron trigger --------------------------------------------

func hTKAdminRunCron(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := &tickets.Cron{
			Store:             tkAdminStore(d),
			Emitter:           tkEventEmitter{d: d},
			Enabled:           d.TKAutoCloseEnabled,
			ResolvedAfterDays: d.TKResolvedCloseAfterDays,
			WaitingAfterDays:  d.TKWaitingCloseAfterDays,
		}
		if err := c.CloseIdle(r.Context()); err != nil {
			writeInternal(w, r, d, "tk_cron_failed", err); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// tkEventEmitter bridges the cron's Emitter interface to tkPublishEvent.
type tkEventEmitter struct{ d Deps }

func (e tkEventEmitter) PublishTicketEvent(_ context.Context, name string, t store.TKTicket, extra map[string]any) {
	tkPublishEvent(e.d, name, t, extra)
}

// --- Categories admin (mirrors KB categories patterns) -------------

type tkCategoryRequest struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Active    bool   `json:"active"`
}

func hTKAdminListCategoriesAdmin(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cats, err := tkAdminStore(d).TKListCategories(r.Context(), false)
		if err != nil { writeInternal(w, r, d, "tk_categories_list_failed", err); return }
		writeJSON(w, http.StatusOK, cats)
	}
}

func hTKAdminCreateCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if req.Slug == "" || req.Name == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_cat", "slug and name required"); return
		}
		saved, err := tkAdminStore(d).TKCreateCategory(r.Context(), store.TKCategory{
			Slug: req.Slug, Name: req.Name, SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil { writeInternal(w, r, d, "tk_cat_create_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminUpdateCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		var req tkCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		saved, err := tkAdminStore(d).TKUpdateCategory(r.Context(), store.TKCategory{
			ID: id, Name: req.Name, SortOrder: req.SortOrder, Active: req.Active,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "category not found"); return
		}
		if err != nil { writeInternal(w, r, d, "tk_cat_update_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminDeleteCategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		if err := tkAdminStore(d).TKDeleteCategory(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "tk_not_found", "category not found"); return
			}
			writeErr(w, http.StatusConflict, "tk_cat_in_use", "category is in use"); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// --- Subcategories admin -------------------------------------------

type tkSubcategoryRequest struct {
	CategoryID int64  `json:"categoryId"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	SortOrder  int    `json:"sortOrder"`
	Active     bool   `json:"active"`
}

func hTKAdminListSubcategories(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID := parseInt64(r.URL.Query().Get("categoryId"))
		if categoryID == 0 {
			writeErr(w, http.StatusBadRequest, "bad_query", "categoryId is required"); return
		}
		subs, err := tkAdminStore(d).TKListSubcategories(r.Context(), categoryID, false)
		if err != nil { writeInternal(w, r, d, "tk_subs_list_failed", err); return }
		writeJSON(w, http.StatusOK, subs)
	}
}

func hTKAdminCreateSubcategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkSubcategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		if req.CategoryID == 0 || req.Slug == "" || req.Name == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_sub", "categoryId, slug, name required"); return
		}
		saved, err := tkAdminStore(d).TKCreateSubcategory(r.Context(), store.TKSubcategory{
			CategoryID: req.CategoryID, Slug: req.Slug, Name: req.Name,
			SortOrder: req.SortOrder, Active: req.Active,
		})
		if err != nil { writeInternal(w, r, d, "tk_sub_create_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminUpdateSubcategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		var req tkSubcategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		saved, err := tkAdminStore(d).TKUpdateSubcategory(r.Context(), store.TKSubcategory{
			ID: id, Name: req.Name, SortOrder: req.SortOrder, Active: req.Active,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "subcategory not found"); return
		}
		if err != nil { writeInternal(w, r, d, "tk_sub_update_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminDeleteSubcategory(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		if err := tkAdminStore(d).TKDeleteSubcategory(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "tk_not_found", "subcategory not found"); return
			}
			writeErr(w, http.StatusConflict, "tk_sub_in_use", "subcategory is in use"); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// --- Category fields admin -----------------------------------------

type tkFieldRequest struct {
	CategoryID int64  `json:"categoryId"`
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Required   bool   `json:"required"`
	SortOrder  int    `json:"sortOrder"`
}

func hTKAdminListFields(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID := parseInt64(r.URL.Query().Get("categoryId"))
		fields, err := tkAdminStore(d).TKListCategoryFields(r.Context(), categoryID)
		if err != nil { writeInternal(w, r, d, "tk_fields_list_failed", err); return }
		writeJSON(w, http.StatusOK, fields)
	}
}

func hTKAdminCreateField(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tkFieldRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		validKinds := map[string]bool{"text": true, "textarea": true, "number": true, "url": true}
		if !validKinds[req.Kind] || req.CategoryID == 0 || req.Key == "" || req.Label == "" {
			writeErr(w, http.StatusBadRequest, "tk_bad_field", "categoryId, key, label, and a valid kind required"); return
		}
		saved, err := tkAdminStore(d).TKCreateCategoryField(r.Context(), store.TKCategoryField{
			CategoryID: req.CategoryID, Key: req.Key, Label: req.Label, Kind: req.Kind,
			Required: req.Required, SortOrder: req.SortOrder,
		})
		if err != nil { writeInternal(w, r, d, "tk_field_create_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminUpdateField(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		var req tkFieldRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON body"); return
		}
		saved, err := tkAdminStore(d).TKUpdateCategoryField(r.Context(), store.TKCategoryField{
			ID: id, Label: req.Label, Kind: req.Kind, Required: req.Required, SortOrder: req.SortOrder,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "field not found"); return
		}
		if err != nil { writeInternal(w, r, d, "tk_field_update_failed", err); return }
		writeJSON(w, http.StatusOK, saved)
	}
}

func hTKAdminDeleteField(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid id"); return
		}
		if err := tkAdminStore(d).TKDeleteCategoryField(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusNotFound, "tk_not_found", "field not found"); return
			}
			writeInternal(w, r, d, "tk_field_delete_failed", err); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
EOF
```

The file references `Deps.TKAutoCloseEnabled`, `Deps.TKResolvedCloseAfterDays`, `Deps.TKWaitingCloseAfterDays` (added next), plus `context` (already imported in the file). Won't compile until Deps extends — single commit at the end of Phase G after the attachment handler lands.

---

## Phase G — Attachments

### Task G1: `handlers_tk_attachments.go` + Deps extension

**Files:**
- Create: `internal/server/handlers_tk_attachments.go`
- Modify: `internal/server/server.go` (Deps struct)

```bash
cat > internal/server/handlers_tk_attachments.go <<'EOF'
package server

import (
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

const tkAttachmentMaxBytes = 10 << 20 // 10 MB

// hTKUploadAttachment accepts a multipart upload, validates size,
// persists, returns the attachment metadata. The caller must own the
// entry's ticket (customer) OR be admin.
func hTKUploadAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryID, err := strconv.ParseInt(chi.URLParam(r, "entry_id"), 10, 64)
		if err != nil || entryID <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_id", "invalid entry id"); return
		}
		st := tkCustomerStore(d)
		entry, err := st.TKGetEntry(r.Context(), entryID)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "tk_not_found", "entry not found"); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_entry_get_failed", err); return
		}
		// Authorize: customer who owns the parent ticket OR admin.
		if r.Header.Get("X-Continuum-User-Role") != "admin" {
			ticket, err := st.TKGetTicketByID(r.Context(), entry.TicketID)
			if err != nil {
				writeInternal(w, r, d, "tk_ticket_get_failed", err); return
			}
			if ticket.CustomerID != r.Header.Get("X-Continuum-User-Id") {
				writeErr(w, http.StatusForbidden, "tk_forbidden", "not your ticket"); return
			}
		}

		if r.ContentLength > tkAttachmentMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "tk_too_large",
				"attachment must be 10 MB or smaller"); return
		}
		if err := r.ParseMultipartForm(tkAttachmentMaxBytes); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_multipart", "could not parse multipart body"); return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "missing_file", "file field required"); return
		}
		defer file.Close()

		body, err := io.ReadAll(io.LimitReader(file, tkAttachmentMaxBytes+1))
		if err != nil {
			writeInternal(w, r, d, "tk_attachment_read_failed", err); return
		}
		if int64(len(body)) > tkAttachmentMaxBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "tk_too_large",
				"attachment must be 10 MB or smaller"); return
		}

		mime := header.Header.Get("Content-Type")
		if mime == "" {
			mime = http.DetectContentType(body)
		}
		sum := sha256.Sum256(body)

		meta, err := st.TKInsertAttachment(r.Context(), store.TKAttachment{
			EntryID: entry.ID, Filename: header.Filename, MIME: mime,
			Bytes: int64(len(body)), Content: body, SHA256: sum[:],
		})
		if err != nil {
			writeInternal(w, r, d, "tk_attachment_insert_failed", err); return
		}
		writeJSON(w, http.StatusOK, meta)
	}
}

// hTKServeAttachment streams the bytes. Available to the owning
// customer + any admin; the entry-owning-ticket lookup gates access.
func hTKServeAttachment(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r); return
		}
		st := tkCustomerStore(d)
		att, err := st.TKGetAttachment(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r); return
		}
		if err != nil {
			writeInternal(w, r, d, "tk_attachment_get_failed", err); return
		}
		// Same auth gate as upload.
		if r.Header.Get("X-Continuum-User-Role") != "admin" {
			entry, err := st.TKGetEntry(r.Context(), att.EntryID)
			if err != nil {
				writeInternal(w, r, d, "tk_entry_get_failed", err); return
			}
			ticket, err := st.TKGetTicketByID(r.Context(), entry.TicketID)
			if err != nil {
				writeInternal(w, r, d, "tk_ticket_get_failed", err); return
			}
			if ticket.CustomerID != r.Header.Get("X-Continuum-User-Id") {
				http.NotFound(w, r); return  // don't leak existence
			}
		}
		w.Header().Set("Content-Type", att.MIME)
		w.Header().Set("Content-Length", strconv.FormatInt(att.Bytes, 10))
		w.Header().Set("Content-Disposition", "inline; filename=\""+att.Filename+"\"")
		_, _ = w.Write(att.Content)
	}
}
EOF
```

Now extend `server.Deps`. Read `internal/server/server.go`, find the existing `Deps` struct, add three fields at the end:

```go
    // Tickets config.
    TKAutoCloseEnabled        bool
    TKResolvedCloseAfterDays  int
    TKWaitingCloseAfterDays   int
```

Verify everything compiles + commit Phase F + G together (3 files: handlers_tk_admin.go, handlers_tk_attachments.go, server.go):

```bash
go build ./...
GOWORK=off go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_tk_admin.go internal/server/handlers_tk_attachments.go internal/server/server.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): tickets admin handlers + attachments + Deps wiring"
```

---

## Phase H — Routes + manifest + main.go wiring + runtime

### Task H1: All wiring in one commit

```bash
cd /opt/continuum_plugins/continuum-plugin-support
```

### H1.1 — Register tk routes in `internal/server/server.go`

After the existing route block (KB + speedtest), append:

```go
	// Tickets module routes.
	r.Get ("/tickets",                            requireUser(hTKListPage(d)))
	r.Get ("/tickets/{tracking_number}",          requireUser(hTKDetailPage(d)))
	r.Get ("/api/customer/categories",            requireUser(hTKCategoriesForCustomer(d)))
	r.Get ("/api/customer/tickets",               requireUser(hTKCustomerList(d)))
	r.Post("/api/customer/tickets",               requireUser(hTKCustomerCreate(d)))
	r.Get ("/api/customer/tickets/{tracking_number}",         requireUser(hTKCustomerDetail(d)))
	r.Post("/api/customer/tickets/{tracking_number}/reply",   requireUser(hTKCustomerReply(d)))
	r.Post("/api/customer/tickets/{tracking_number}/reopen",  requireUser(hTKCustomerReopen(d)))

	r.Get ("/admin/tickets",                      requireAdmin(hTKAdminQueuePage(d)))
	r.Get ("/admin/tickets/categories",           requireAdmin(hTKAdminCategoriesPage(d)))
	r.Get ("/admin/tickets/{tracking_number}",    requireAdmin(hTKAdminDetailPage(d)))
	r.Get ("/api/admin/tickets",                  requireAdmin(hTKAdminQueue(d)))
	r.Get ("/api/admin/tickets/{tracking_number}",            requireAdmin(hTKAdminDetail(d)))
	r.Post("/api/admin/tickets/{tracking_number}/reply",      requireAdmin(hTKAdminReply(d)))
	r.Post("/api/admin/tickets/{tracking_number}/note",       requireAdmin(hTKAdminNote(d)))
	r.Post("/api/admin/tickets/{tracking_number}/status",     requireAdmin(hTKAdminStatus(d)))
	r.Post("/api/admin/tickets/{tracking_number}/assign",     requireAdmin(hTKAdminAssign(d)))
	r.Post("/api/admin/tickets/cron/run",         requireAdmin(hTKAdminRunCron(d)))

	r.Get   ("/api/admin/categories",             requireAdmin(hTKAdminListCategoriesAdmin(d)))
	r.Post  ("/api/admin/categories",             requireAdmin(hTKAdminCreateCategory(d)))
	r.Put   ("/api/admin/categories/{id}",        requireAdmin(hTKAdminUpdateCategory(d)))
	r.Delete("/api/admin/categories/{id}",        requireAdmin(hTKAdminDeleteCategory(d)))

	r.Get   ("/api/admin/subcategories",          requireAdmin(hTKAdminListSubcategories(d)))
	r.Post  ("/api/admin/subcategories",          requireAdmin(hTKAdminCreateSubcategory(d)))
	r.Put   ("/api/admin/subcategories/{id}",     requireAdmin(hTKAdminUpdateSubcategory(d)))
	r.Delete("/api/admin/subcategories/{id}",     requireAdmin(hTKAdminDeleteSubcategory(d)))

	r.Get   ("/api/admin/category-fields",        requireAdmin(hTKAdminListFields(d)))
	r.Post  ("/api/admin/category-fields",        requireAdmin(hTKAdminCreateField(d)))
	r.Put   ("/api/admin/category-fields/{id}",   requireAdmin(hTKAdminUpdateField(d)))
	r.Delete("/api/admin/category-fields/{id}",   requireAdmin(hTKAdminDeleteField(d)))

	r.Post  ("/api/tickets/entries/{entry_id}/attachments", requireUser(hTKUploadAttachment(d)))
	r.Get   ("/api/attachments/{id}",             requireUser(hTKServeAttachment(d)))
```

### H1.2 — manifest.json

Bump `version` to `0.4.0`. Append (preserving prior entries):

```json
    { "id": "tk_list",          "method": "GET",  "path": "/tickets",                          "access": "user" },
    { "id": "tk_detail",        "method": "GET",  "path": "/tickets/*",                        "access": "user" },
    { "id": "tk_api_customer",  "method": "*",    "path": "/api/customer/tickets",             "access": "user" },
    { "id": "tk_api_customer2", "method": "*",    "path": "/api/customer/tickets/*",           "access": "user" },
    { "id": "tk_api_cats",      "method": "GET",  "path": "/api/customer/categories",          "access": "user" },
    { "id": "tk_admin_root",    "method": "GET",  "path": "/admin/tickets",                    "access": "admin" },
    { "id": "tk_admin_pages",   "method": "GET",  "path": "/admin/tickets/*",                  "access": "admin" },
    { "id": "tk_admin_api",     "method": "*",    "path": "/api/admin/tickets",                "access": "admin" },
    { "id": "tk_admin_api2",    "method": "*",    "path": "/api/admin/tickets/*",              "access": "admin" },
    { "id": "tk_admin_cats",    "method": "*",    "path": "/api/admin/categories",             "access": "admin" },
    { "id": "tk_admin_cats2",   "method": "*",    "path": "/api/admin/categories/*",           "access": "admin" },
    { "id": "tk_admin_subs",    "method": "*",    "path": "/api/admin/subcategories",          "access": "admin" },
    { "id": "tk_admin_subs2",   "method": "*",    "path": "/api/admin/subcategories/*",        "access": "admin" },
    { "id": "tk_admin_fields",  "method": "*",    "path": "/api/admin/category-fields",        "access": "admin" },
    { "id": "tk_admin_fields2", "method": "*",    "path": "/api/admin/category-fields/*",      "access": "admin" },
    { "id": "tk_attach_upload", "method": "POST", "path": "/api/tickets/entries/*",            "access": "user" },
    { "id": "tk_attach_serve",  "method": "GET",  "path": "/api/attachments/*",                "access": "user" }
```

### H1.3 — main.go wiring

Edit `cmd/continuum-plugin-support/main.go`. Update the `httpSrv.SetHandler(server.New(server.Deps{...}))` call to pass the three new tickets fields:

```go
	httpSrv.SetHandler(server.New(server.Deps{
		// ... existing fields ...
		TKAutoCloseEnabled:       cfg.TicketsAutoCloseEnabled,
		TKResolvedCloseAfterDays: cfg.TicketsResolvedCloseAfterDays,
		TKWaitingCloseAfterDays:  cfg.TicketsWaitingCloseAfterDays,
	}))
```

### H1.4 — runtime.go config fields

Edit `internal/runtime/runtime.go`. Append to the `Config` struct:

```go
    // Tickets module config.
    TicketsAutoCloseEnabled        bool `json:"tickets_auto_close_enabled"`
    TicketsResolvedCloseAfterDays  int  `json:"tickets_resolved_close_after_days"`
    TicketsWaitingCloseAfterDays   int  `json:"tickets_waiting_close_after_days"`
```

Update `DefaultAppConfig`:

```go
func DefaultAppConfig() Config {
	return Config{
		Modules: ModuleToggles{KB: true, Speedtest: true, Tickets: true},
		AutoStrategy: "latency", ClientIPStorage: "truncated", SlowThresholdMbps: 5,

		TicketsAutoCloseEnabled:       true,
		TicketsResolvedCloseAfterDays: 7,
		TicketsWaitingCloseAfterDays:  14,
	}
}
```

Update `NormalizeAppConfig` to enforce non-negative days:

```go
	if cfg.TicketsResolvedCloseAfterDays < 0 {
		return Config{}, fmt.Errorf("tickets_resolved_close_after_days must be >= 0")
	}
	if cfg.TicketsWaitingCloseAfterDays < 0 {
		return Config{}, fmt.Errorf("tickets_waiting_close_after_days must be >= 0")
	}
```

### H1.5 — runtime_test.go update

Update the test that asserts module defaults (renamed `TestConfigureDefaultsKBAndSpeedtestOnOthersOff` in the speedtest unit). Bring Tickets into the "on" set, leave AI off:

```go
if !observed.Modules.KB || !observed.Modules.Speedtest || !observed.Modules.Tickets {
    t.Fatalf("KB + Speedtest + Tickets should default ON; got %+v", observed.Modules)
}
if observed.Modules.AI {
    t.Fatalf("AI should default off (not shipped); got %+v", observed.Modules)
}
```

Rename to `TestConfigureDefaultsKBSpeedtestTicketsOnAIOff`.

### H1.6 — Verify + single commit

```bash
go build ./...
go test ./...
GOWORK=off go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/server.go cmd/continuum-plugin-support/manifest.json cmd/continuum-plugin-support/main.go internal/runtime/runtime.go internal/runtime/runtime_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat: wire tickets routes + main.go + manifest 0.4.0 + Modules.Tickets default true"
```

---

## Phase J — Server integration tests

### Task J1: `server_tk_test.go` — PG_DSN-gated sweep

**Files:**
- Create: `internal/server/server_tk_test.go`

```bash
cat > internal/server/server_tk_test.go <<'EOF'
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

	"github.com/RXWatcher/continuum-plugin-support/internal/migrate"
	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

func tkTestDeps(t *testing.T) (Deps, *store.Store, func()) {
	t.Helper()
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN unset; skipping tickets integration test")
	}
	ctx := context.Background()
	if err := migrate.Run(ctx, dsn); err != nil { t.Fatal(err) }
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatal(err) }
	st := store.New(pool)
	d := Deps{
		ConfigStore:              st,
		TKAutoCloseEnabled:       true,
		TKResolvedCloseAfterDays: 7,
		TKWaitingCloseAfterDays:  14,
	}
	return d, st, func() { pool.Close() }
}

func TestTKCustomerListRequiresAuth(t *testing.T) {
	d, _, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer/tickets", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTKAdminRoutesRejectNonAdmin(t *testing.T) {
	d, _, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	for _, path := range []string{"/admin/tickets", "/api/admin/tickets", "/api/admin/categories"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Continuum-User-Id", "42")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("path %s status = %d, want 403", path, rec.Code)
		}
	}
}

func TestTKCustomerCreateAndDetailRoundTrip(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	ctx := context.Background()

	cat, _ := st.TKCreateCategory(ctx, store.TKCategory{
		Slug: "test", Name: "Test", Active: true,
	})

	body := fmt.Sprintf(`{"categoryId":%d,"subject":"hello","body":"world","customerEmail":"a@b"}`, cat.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/customer/tickets", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var created store.TKTicket
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil { t.Fatal(err) }
	if created.TrackingNumber == "" {
		t.Fatalf("missing tracking number: %+v", created)
	}

	// Detail (owner)
	req = httptest.NewRequest(http.MethodGet, "/api/customer/tickets/"+created.TrackingNumber, nil)
	req.Header.Set("X-Continuum-User-Id", "42")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// Detail (different customer → 404, not 403; spec: don't leak existence)
	req = httptest.NewRequest(http.MethodGet, "/api/customer/tickets/"+created.TrackingNumber, nil)
	req.Header.Set("X-Continuum-User-Id", "99")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-customer detail status = %d, want 404", rec.Code)
	}
}

func TestTKAdminLifecycle(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	ctx := context.Background()
	cat, _ := st.TKCreateCategory(ctx, store.TKCategory{Slug: "tx", Name: "TX", Active: true})

	// Customer creates
	body := fmt.Sprintf(`{"categoryId":%d,"subject":"lifecycle","body":"start","customerEmail":"c@d"}`, cat.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/customer/tickets", bytes.NewBufferString(body))
	req.Header.Set("X-Continuum-User-Id", "100")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var created store.TKTicket
	json.Unmarshal(rec.Body.Bytes(), &created)
	tn := created.TrackingNumber

	// Admin reply -> open -> in_progress
	req = httptest.NewRequest(http.MethodPost, "/api/admin/tickets/"+tn+"/reply",
		bytes.NewBufferString(`{"body":"hi"}`))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("admin reply: %d %s", rec.Code, rec.Body.String()) }

	// Admin mark resolved
	req = httptest.NewRequest(http.MethodPost, "/api/admin/tickets/"+tn+"/status",
		bytes.NewBufferString(`{"to":"resolved"}`))
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("X-Continuum-User-Role", "admin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("admin resolve: %d %s", rec.Code, rec.Body.String()) }

	// Customer reopens
	req = httptest.NewRequest(http.MethodPost, "/api/customer/tickets/"+tn+"/reopen", nil)
	req.Header.Set("X-Continuum-User-Id", "100")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("reopen: %d %s", rec.Code, rec.Body.String()) }
}

func TestTKAttachmentTooLargeReturns413(t *testing.T) {
	d, _, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)

	// 11 MB body
	big := bytes.Repeat([]byte("x"), 11*1024*1024)
	body := &bytes.Buffer{}
	body.Write(big)

	req := httptest.NewRequest(http.MethodPost, "/api/tickets/entries/1/attachments", body)
	req.Header.Set("X-Continuum-User-Id", "1")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=BOUNDARY")
	req.ContentLength = int64(body.Len())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}
EOF
go test ./internal/server/... -v -run TestTK 2>&1 | tail -15   # all skip without PG_DSN
GOWORK=off go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/server_tk_test.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "test(server): tickets integration tests (PG_DSN-gated)"
```

---

## Phase K — SPA types + bootstrap + API clients

### Task K1: Extend `web/src/lib/types.ts`

Read the current file. Append TK types AFTER existing exports:

```ts
export type TKCategory = {
  id: number; slug: string; name: string; sortOrder: number; active: boolean;
  createdAt: string; updatedAt: string;
};
export type TKSubcategory = {
  id: number; categoryId: number; slug: string; name: string;
  sortOrder: number; active: boolean; createdAt: string; updatedAt: string;
};
export type TKCategoryField = {
  id: number; categoryId: number; key: string; label: string;
  kind: "text" | "textarea" | "number" | "url";
  required: boolean; sortOrder: number;
};

export type TKAttachmentMeta = {
  id: number; filename: string; mime: string; bytes: number; createdAt: string;
};

export type TKEntry = {
  id: number; ticketId: number;
  kind: "initial" | "reply" | "internal_note" | "status_change" | "system";
  authorId: string; authorRole: "customer" | "admin" | "system";
  body: string; createdAt: string;
  attachments?: TKAttachmentMeta[];
};

export type TKFieldValue = {
  fieldId: number; fieldKey: string; fieldLabel: string; value: string;
};

export type TKTicket = {
  id: number; trackingNumber: string;
  customerId: string; customerEmail: string;
  categoryId: number; subcategoryId?: number | null;
  subject: string;
  status: "open" | "in_progress" | "waiting_customer" | "resolved" | "closed";
  assignedAdminId?: string | null;
  createdAt: string; updatedAt: string;
  waitingSince?: string | null; resolvedAt?: string | null;
  category?: TKCategory; subcategory?: TKSubcategory;
  fieldValues?: TKFieldValue[];
};

export type TKCategoriesResponse = {
  categories: TKCategory[];
  subcategories: Record<number, TKSubcategory[]>;
  fields: Record<number, TKCategoryField[]>;
};

export type TKDetailResponse = {
  ticket: TKTicket;
  entries: TKEntry[];
};
```

Extend `SupportBootstrap.mode` to add 5 new modes (`tickets-list`, `tickets-new`, `tickets-detail`, `admin-tickets-queue`, `admin-tickets-detail`, `admin-tickets-categories`). The current union has 13 modes (after speedtest). Append:

```ts
    | "tickets-list"
    | "tickets-new"
    | "tickets-detail"
    | "admin-tickets-queue"
    | "admin-tickets-detail"
    | "admin-tickets-categories"
```

Verify + commit:

```bash
cd /opt/continuum_plugins/continuum-plugin-support
cd web && pnpm test && pnpm exec tsc -b --noEmit && cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/lib/types.ts
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): tickets types + bootstrap mode union extension"
```

---

### Task K2: API clients

**Files:**
- Create: `web/src/api/tk.ts`
- Create: `web/src/api/tkAdmin.ts`

```bash
cat > web/src/api/tk.ts <<'EOF'
import { api, absoluteURL } from "@/lib/api";
import type {
  TKCategoriesResponse, TKDetailResponse, TKEntry, TKTicket,
} from "@/lib/types";

export type TKListParams = {
  statusGroup?: "active" | "closed";
  limit?: number;
  offset?: number;
};

export function listTKTickets(p: TKListParams = {}): Promise<TKTicket[]> {
  const qs = new URLSearchParams();
  if (p.statusGroup) qs.set("statusGroup", p.statusGroup);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/customer/tickets" + (qs.toString() ? `?${qs}` : "");
  return api<TKTicket[]>(path);
}

export function getTKTicket(tn: string): Promise<TKDetailResponse> {
  return api<TKDetailResponse>(`/api/customer/tickets/${encodeURIComponent(tn)}`);
}

export function getTKCategoriesForm(): Promise<TKCategoriesResponse> {
  return api<TKCategoriesResponse>("/api/customer/categories");
}

export type TKCreateRequest = {
  categoryId: number;
  subcategoryId?: number;
  subject: string;
  body: string;
  fieldValues?: Record<string, string>;
  customerEmail: string;
};

export function createTKTicket(req: TKCreateRequest): Promise<TKTicket> {
  return api<TKTicket>("/api/customer/tickets", {
    method: "POST", body: JSON.stringify(req),
  });
}

export function replyTKTicket(tn: string, body: string): Promise<{ entry: TKEntry; ticket: TKTicket }> {
  return api(`/api/customer/tickets/${encodeURIComponent(tn)}/reply`, {
    method: "POST", body: JSON.stringify({ body }),
  });
}

export function reopenTKTicket(tn: string): Promise<TKTicket> {
  return api<TKTicket>(`/api/customer/tickets/${encodeURIComponent(tn)}/reopen`, { method: "POST" });
}

export async function uploadTKAttachment(entryID: number, file: File): Promise<{ id: number; url: string }> {
  const fd = new FormData();
  fd.append("file", file);
  const res = await fetch(absoluteURL(`/api/tickets/entries/${entryID}/attachments`), {
    method: "POST", body: fd,
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.error?.message ?? `Upload failed (${res.status})`);
  }
  const meta = await res.json();
  return { id: meta.id, url: `/api/attachments/${meta.id}` };
}
EOF

cat > web/src/api/tkAdmin.ts <<'EOF'
import { api } from "@/lib/api";
import type {
  TKCategory, TKCategoryField, TKDetailResponse, TKEntry, TKSubcategory, TKTicket,
} from "@/lib/types";

export type TKQueueParams = {
  status?: string;
  statusGroup?: "active" | "closed";
  categoryId?: number;
  assignee?: string;       // "__mine__" | "__unassigned__" | admin id
  q?: string;
  limit?: number;
  offset?: number;
};

export function listTKAdminQueue(p: TKQueueParams = {}): Promise<TKTicket[]> {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.statusGroup) qs.set("statusGroup", p.statusGroup);
  if (p.categoryId) qs.set("categoryId", String(p.categoryId));
  if (p.assignee) qs.set("assignee", p.assignee);
  if (p.q) qs.set("q", p.q);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const path = "/api/admin/tickets" + (qs.toString() ? `?${qs}` : "");
  return api<TKTicket[]>(path);
}

export function getTKAdminTicket(tn: string): Promise<TKDetailResponse> {
  return api<TKDetailResponse>(`/api/admin/tickets/${encodeURIComponent(tn)}`);
}

export function replyTKAdmin(tn: string, body: string): Promise<{ entry: TKEntry; ticket: TKTicket }> {
  return api(`/api/admin/tickets/${encodeURIComponent(tn)}/reply`, {
    method: "POST", body: JSON.stringify({ body }),
  });
}

export function noteTKAdmin(tn: string, body: string): Promise<TKEntry> {
  return api<TKEntry>(`/api/admin/tickets/${encodeURIComponent(tn)}/note`, {
    method: "POST", body: JSON.stringify({ body }),
  });
}

export function statusTKAdmin(tn: string, to: string): Promise<TKTicket> {
  return api<TKTicket>(`/api/admin/tickets/${encodeURIComponent(tn)}/status`, {
    method: "POST", body: JSON.stringify({ to }),
  });
}

export function assignTKAdmin(tn: string, adminID: string | null): Promise<TKTicket> {
  return api<TKTicket>(`/api/admin/tickets/${encodeURIComponent(tn)}/assign`, {
    method: "POST", body: JSON.stringify({ adminId: adminID }),
  });
}

export function runTKCronAdmin(): Promise<{ ok: boolean }> {
  return api<{ ok: boolean }>("/api/admin/tickets/cron/run", { method: "POST" });
}

// Categories
export type TKCategoryWrite = { slug: string; name: string; sortOrder: number; active: boolean };
export function listTKCategoriesAdmin(): Promise<TKCategory[]> { return api("/api/admin/categories"); }
export function createTKCategoryAdmin(w: TKCategoryWrite) { return api<TKCategory>("/api/admin/categories", { method: "POST", body: JSON.stringify(w) }); }
export function updateTKCategoryAdmin(id: number, w: TKCategoryWrite) { return api<TKCategory>(`/api/admin/categories/${id}`, { method: "PUT", body: JSON.stringify(w) }); }
export function deleteTKCategoryAdmin(id: number) { return api<{ ok: boolean }>(`/api/admin/categories/${id}`, { method: "DELETE" }); }

// Subcategories
export type TKSubcategoryWrite = { categoryId: number; slug: string; name: string; sortOrder: number; active: boolean };
export function listTKSubcategoriesAdmin(categoryID: number): Promise<TKSubcategory[]> {
  return api(`/api/admin/subcategories?categoryId=${categoryID}`);
}
export function createTKSubcategoryAdmin(w: TKSubcategoryWrite) { return api<TKSubcategory>("/api/admin/subcategories", { method: "POST", body: JSON.stringify(w) }); }
export function updateTKSubcategoryAdmin(id: number, w: TKSubcategoryWrite) { return api<TKSubcategory>(`/api/admin/subcategories/${id}`, { method: "PUT", body: JSON.stringify(w) }); }
export function deleteTKSubcategoryAdmin(id: number) { return api<{ ok: boolean }>(`/api/admin/subcategories/${id}`, { method: "DELETE" }); }

// Category fields
export type TKFieldWrite = { categoryId: number; key: string; label: string; kind: TKCategoryField["kind"]; required: boolean; sortOrder: number };
export function listTKFieldsAdmin(categoryID: number): Promise<TKCategoryField[]> {
  return api(`/api/admin/category-fields?categoryId=${categoryID}`);
}
export function createTKFieldAdmin(w: TKFieldWrite) { return api<TKCategoryField>("/api/admin/category-fields", { method: "POST", body: JSON.stringify(w) }); }
export function updateTKFieldAdmin(id: number, w: TKFieldWrite) { return api<TKCategoryField>(`/api/admin/category-fields/${id}`, { method: "PUT", body: JSON.stringify(w) }); }
export function deleteTKFieldAdmin(id: number) { return api<{ ok: boolean }>(`/api/admin/category-fields/${id}`, { method: "DELETE" }); }
EOF

cd web && pnpm exec tsc -b --noEmit && pnpm test
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/api/tk.ts web/src/api/tkAdmin.ts
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): tickets customer + admin API clients"
```

---

## Phase L — Customer SPA

### Task L1: Components — TicketCard + StatusBadge + Thread + ReplyBox (TDD on ReplyBox)

**Files:**
- Create: `web/src/components/tk/*` (5 files)

```bash
cd /opt/continuum_plugins/continuum-plugin-support
mkdir -p web/src/components/tk

cat > web/src/components/tk/StatusBadge.tsx <<'EOF'
import { Badge } from "@/components/ui/badge";
import type { TKTicket } from "@/lib/types";

const VARIANT: Record<TKTicket["status"], "default" | "secondary" | "outline" | "destructive"> = {
  open:              "default",
  in_progress:       "default",
  waiting_customer:  "secondary",
  resolved:          "secondary",
  closed:            "outline",
};

const LABEL: Record<TKTicket["status"], string> = {
  open:              "Open",
  in_progress:       "In progress",
  waiting_customer:  "Waiting on you",
  resolved:          "Resolved",
  closed:            "Closed",
};

export function StatusBadge({ status }: { status: TKTicket["status"] }) {
  return <Badge variant={VARIANT[status]}>{LABEL[status]}</Badge>;
}
EOF

cat > web/src/components/tk/TicketCard.tsx <<'EOF'
import { Card, CardContent } from "@/components/ui/card";
import { StatusBadge } from "./StatusBadge";
import type { TKTicket } from "@/lib/types";

export function TicketCard({ t }: { t: TKTicket }) {
  return (
    <a href={`./tickets/${encodeURIComponent(t.trackingNumber)}`} className="block">
      <Card className="transition-colors hover:border-accent/40">
        <CardContent className="space-y-1 py-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="font-mono">{t.trackingNumber}</span>
            <StatusBadge status={t.status} />
            <span className="ml-auto">{new Date(t.updatedAt).toLocaleString()}</span>
          </div>
          <p className="font-medium">{t.subject}</p>
        </CardContent>
      </Card>
    </a>
  );
}
EOF

cat > web/src/components/tk/Thread.tsx <<'EOF'
import { Card, CardContent } from "@/components/ui/card";
import type { TKEntry } from "@/lib/types";

type Props = {
  entries: TKEntry[];
  isAdmin?: boolean;
};

export function Thread({ entries, isAdmin }: Props) {
  return (
    <ol className="space-y-3">
      {entries.map((e) => (
        <li key={e.id}>
          {e.kind === "system" || e.kind === "status_change" ? (
            <p className="text-xs italic text-muted-foreground text-center">
              {e.body} · {new Date(e.createdAt).toLocaleString()}
            </p>
          ) : (
            <Card className={e.kind === "internal_note" ? "border-amber-500/60 bg-amber-500/10" : ""}>
              <CardContent className="space-y-1 py-3">
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span className="font-medium">
                    {e.authorRole === "admin" ? "Support team" :
                     e.authorRole === "system" ? "System" : "You"}
                  </span>
                  {e.kind === "internal_note" && isAdmin && (
                    <span className="rounded bg-amber-600 px-1.5 py-0.5 text-xs font-semibold text-white">
                      INTERNAL · admin-only
                    </span>
                  )}
                  <span className="ml-auto">{new Date(e.createdAt).toLocaleString()}</span>
                </div>
                <p className="whitespace-pre-wrap text-sm">{e.body}</p>
                {e.attachments && e.attachments.length > 0 && (
                  <ul className="mt-2 space-y-1 text-xs">
                    {e.attachments.map((a) => (
                      <li key={a.id}>
                        <a className="text-accent hover:underline" href={`/api/attachments/${a.id}`}>
                          📎 {a.filename} ({Math.round(a.bytes / 1024)} KB)
                        </a>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          )}
        </li>
      ))}
    </ol>
  );
}
EOF

cat > web/src/components/tk/ReplyBox.test.tsx <<'EOF'
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ReplyBox } from "./ReplyBox";

describe("ReplyBox", () => {
  afterEach(() => cleanup());

  it("disables submit when empty", () => {
    render(<ReplyBox onSubmit={async () => {}} disabled={false} />);
    expect(screen.getByRole("button", { name: /send/i })).toBeDisabled();
  });

  it("calls onSubmit with the body when submitted", () => {
    const onSubmit = vi.fn(async () => {});
    render(<ReplyBox onSubmit={onSubmit} disabled={false} />);
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Hello there" } });
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(onSubmit).toHaveBeenCalledWith("Hello there");
  });

  it("does not submit when disabled prop is true", () => {
    const onSubmit = vi.fn(async () => {});
    render(<ReplyBox onSubmit={onSubmit} disabled />);
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "x" } });
    expect(screen.getByRole("button", { name: /send/i })).toBeDisabled();
  });
});
EOF

cat > web/src/components/tk/ReplyBox.tsx <<'EOF'
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

type Props = {
  onSubmit: (body: string) => Promise<void>;
  disabled?: boolean;
  placeholder?: string;
};

export function ReplyBox({ onSubmit, disabled, placeholder = "Write a reply…" }: Props) {
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (!body.trim() || disabled || busy) return;
    setBusy(true);
    try {
      await onSubmit(body.trim());
      setBody("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-2">
      <Textarea
        rows={4}
        value={body}
        onChange={(e) => setBody(e.target.value)}
        placeholder={placeholder}
        disabled={disabled || busy}
      />
      <div className="flex justify-end">
        <Button onClick={submit} disabled={disabled || busy || !body.trim()}>
          {busy ? "Sending…" : "Send"}
        </Button>
      </div>
    </div>
  );
}
EOF

cd web && pnpm test 2>&1 | tail -6
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/tk/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): tickets components (StatusBadge / TicketCard / Thread / ReplyBox + test)"
```

---

### Task L2: Customer list + new-flow + detail pages

**Files:**
- Create: `web/src/pages/tk/List.tsx`
- Create: `web/src/pages/tk/New.tsx`
- Create: `web/src/pages/tk/Detail.tsx`

```bash
mkdir -p web/src/pages/tk

cat > web/src/pages/tk/List.tsx <<'EOF'
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { TopBar } from "@/components/shared/TopBar";
import { TicketCard } from "@/components/tk/TicketCard";
import { listTKTickets } from "@/api/tk";
import type { TKTicket } from "@/lib/types";

export function TKList() {
  const [rows, setRows] = useState<TKTicket[]>([]);
  const [tab, setTab] = useState<"active" | "closed" | "all">("active");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    listTKTickets({ statusGroup: tab === "all" ? undefined : tab, limit: 100 })
      .then((r) => { if (!cancelled) setRows(r); })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [tab]);

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <TopBar eyebrow="Support" title="Your tickets" subtitle="Open, in-progress, and resolved tickets." />
        <div className="flex items-center gap-2">
          <Button variant={tab === "active" ? "default" : "outline"} size="sm" onClick={() => setTab("active")}>Active</Button>
          <Button variant={tab === "closed" ? "default" : "outline"} size="sm" onClick={() => setTab("closed")}>Closed</Button>
          <Button variant={tab === "all" ? "default" : "outline"} size="sm" onClick={() => setTab("all")}>All</Button>
          <Button asChild className="ml-auto"><a href="./tickets/new">Open new ticket</a></Button>
        </div>
        {loading ? <p className="text-sm text-muted-foreground">Loading…</p> :
          rows.length === 0 ? (
            <div className="rounded-md border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
              <p className="font-medium text-foreground">No tickets yet.</p>
              <p className="mt-2">When you open one, it'll show up here.</p>
              <Button asChild className="mt-4"><a href="./tickets/new">Open your first ticket</a></Button>
            </div>
          ) :
          <ul className="grid gap-2">{rows.map((t) => <li key={t.id}><TicketCard t={t} /></li>)}</ul>}
      </div>
    </main>
  );
}
EOF

cat > web/src/pages/tk/New.tsx <<'EOF'
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { TopBar } from "@/components/shared/TopBar";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { createTKTicket, getTKCategoriesForm } from "@/api/tk";
import type { TKCategoriesResponse, TKCategory, TKCategoryField, TKSubcategory } from "@/lib/types";

type Step = "category" | "subcategory" | "form" | "done";

export function TKNew() {
  const [data, setData] = useState<TKCategoriesResponse | null>(null);
  const [step, setStep] = useState<Step>("category");
  const [category, setCategory] = useState<TKCategory | null>(null);
  const [subcategory, setSubcategory] = useState<TKSubcategory | null>(null);
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [customerEmail, setCustomerEmail] = useState("");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [createdTN, setCreatedTN] = useState<string | null>(null);

  useEffect(() => {
    getTKCategoriesForm().then(setData).catch(() => toast.error("Could not load categories"));
  }, []);

  function fieldsFor(c: TKCategory | null): TKCategoryField[] {
    if (!data || !c) return [];
    return data.fields[c.id] ?? [];
  }
  function subcategoriesFor(c: TKCategory | null): TKSubcategory[] {
    if (!data || !c) return [];
    return data.subcategories[c.id] ?? [];
  }

  async function submit() {
    if (!category) return;
    setSubmitting(true);
    try {
      const t = await createTKTicket({
        categoryId: category.id,
        subcategoryId: subcategory?.id,
        subject: subject.trim(),
        body: body.trim(),
        customerEmail: customerEmail.trim(),
        fieldValues,
      });
      setCreatedTN(t.trackingNumber);
      setStep("done");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Submit failed");
    } finally {
      setSubmitting(false);
    }
  }

  if (!data) return <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">Loading…</main>;

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <TopBar eyebrow="Support" title="Open a new ticket" subtitle="Tell us what's going on." />

        {step === "category" && (
          <div className="grid gap-2 sm:grid-cols-2">
            {data.categories.filter((c) => c.active).map((c) => (
              <button key={c.id} type="button"
                      className="rounded-md border border-border bg-card p-4 text-left transition-colors hover:border-accent/60"
                      onClick={() => { setCategory(c); setStep(subcategoriesFor(c).length > 0 ? "subcategory" : "form"); }}>
                <p className="font-medium">{c.name}</p>
              </button>
            ))}
          </div>
        )}

        {step === "subcategory" && category && (
          <>
            <p className="text-sm text-muted-foreground">Category: <strong>{category.name}</strong></p>
            <div className="grid gap-2 sm:grid-cols-2">
              {subcategoriesFor(category).filter((s) => s.active).map((s) => (
                <button key={s.id} type="button"
                        className="rounded-md border border-border bg-card p-4 text-left transition-colors hover:border-accent/60"
                        onClick={() => { setSubcategory(s); setStep("form"); }}>
                  <p className="font-medium">{s.name}</p>
                </button>
              ))}
            </div>
            <Button variant="ghost" onClick={() => { setCategory(null); setStep("category"); }}>← Back</Button>
          </>
        )}

        {step === "form" && category && (
          <Card>
            <CardHeader><CardTitle>{category.name}{subcategory ? ` · ${subcategory.name}` : ""}</CardTitle></CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-1">
                <Label htmlFor="subject">Subject</Label>
                <Input id="subject" value={subject} onChange={(e) => setSubject(e.target.value)} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="email">Email</Label>
                <Input id="email" type="email" value={customerEmail} onChange={(e) => setCustomerEmail(e.target.value)} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="body">Describe the problem</Label>
                <Textarea id="body" rows={5} value={body} onChange={(e) => setBody(e.target.value)} />
              </div>
              {fieldsFor(category).map((f) => (
                <div key={f.id} className="space-y-1">
                  <Label htmlFor={`f-${f.id}`}>{f.label}{f.required ? " *" : ""}</Label>
                  {f.kind === "textarea" ? (
                    <Textarea id={`f-${f.id}`} rows={3} value={fieldValues[f.key] ?? ""}
                              onChange={(e) => setFieldValues({ ...fieldValues, [f.key]: e.target.value })} />
                  ) : (
                    <Input id={`f-${f.id}`} type={f.kind === "number" ? "number" : f.kind === "url" ? "url" : "text"}
                           value={fieldValues[f.key] ?? ""}
                           onChange={(e) => setFieldValues({ ...fieldValues, [f.key]: e.target.value })} />
                  )}
                </div>
              ))}
              <div className="flex justify-between">
                <Button variant="ghost" onClick={() => setStep(subcategoriesFor(category).length > 0 ? "subcategory" : "category")}>← Back</Button>
                <Button onClick={submit} disabled={submitting || !subject.trim() || !body.trim() || !customerEmail.trim()}>
                  {submitting ? "Submitting…" : "Submit ticket"}
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {step === "done" && createdTN && (
          <Card>
            <CardHeader><CardTitle>Ticket opened</CardTitle></CardHeader>
            <CardContent className="space-y-3">
              <p>Your tracking number is <strong className="font-mono">{createdTN}</strong>.</p>
              <div className="flex gap-2">
                <Button onClick={() => navigator.clipboard.writeText(createdTN)}>Copy tracking number</Button>
                <Button asChild variant="outline"><a href={`./tickets/${createdTN}`}>View ticket →</a></Button>
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </main>
  );
}
EOF

cat > web/src/pages/tk/Detail.tsx <<'EOF'
import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ReplyBox } from "@/components/tk/ReplyBox";
import { StatusBadge } from "@/components/tk/StatusBadge";
import { Thread } from "@/components/tk/Thread";
import { getTKTicket, reopenTKTicket, replyTKTicket } from "@/api/tk";
import type { TKEntry, TKTicket } from "@/lib/types";

export function TKDetail() {
  const tn = decodeURIComponent(window.location.pathname.split("/tickets/")[1] ?? "");
  const [ticket, setTicket] = useState<TKTicket | null>(null);
  const [entries, setEntries] = useState<TKEntry[]>([]);
  const [err, setErr] = useState("");

  async function refresh() {
    try {
      const r = await getTKTicket(tn);
      setTicket(r.ticket);
      setEntries(r.entries);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Not found");
    }
  }

  useEffect(() => {
    refresh();
    const iv = window.setInterval(refresh, 30_000);
    return () => window.clearInterval(iv);
  }, [tn]);

  if (err) return (
    <main className="mx-auto max-w-3xl px-4 py-16 text-center">
      <h1 className="text-2xl font-semibold">Ticket unavailable</h1>
      <p className="text-muted-foreground">{err}</p>
      <a href="../tickets" className="mt-4 inline-flex items-center gap-1 text-sm text-accent">
        <ArrowLeft className="h-4 w-4" /> Back to your tickets
      </a>
    </main>
  );

  if (!ticket) return <main className="mx-auto max-w-3xl px-4 py-16 text-center text-sm text-muted-foreground">Loading…</main>;

  const canReply = ticket.status !== "closed";
  const canReopen = ticket.status === "resolved" && ticket.resolvedAt &&
    Date.now() - new Date(ticket.resolvedAt).getTime() < 7 * 24 * 60 * 60 * 1000;

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <a href="../tickets" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" /> Back to your tickets
        </a>
        <header className="space-y-1">
          <p className="font-mono text-xs text-muted-foreground">{ticket.trackingNumber}</p>
          <h1 className="text-2xl font-semibold">{ticket.subject}</h1>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <StatusBadge status={ticket.status} />
            {ticket.category && <span>{ticket.category.name}{ticket.subcategory ? ` · ${ticket.subcategory.name}` : ""}</span>}
          </div>
        </header>
        <Thread entries={entries} />
        {canReply && (
          <ReplyBox onSubmit={async (body) => {
            try {
              await replyTKTicket(tn, body);
              await refresh();
              toast.success("Reply sent.");
            } catch (e) { toast.error(e instanceof Error ? e.message : "Send failed"); }
          }} />
        )}
        {canReopen && (
          <Button variant="secondary" onClick={async () => {
            try { await reopenTKTicket(tn); await refresh(); toast.success("Ticket reopened."); }
            catch (e) { toast.error(e instanceof Error ? e.message : "Reopen failed"); }
          }}>Reopen ticket</Button>
        )}
      </div>
    </main>
  );
}
EOF

cd web && pnpm exec tsc -b --noEmit && pnpm test
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/pages/tk/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): tickets customer pages (list / new-flow / detail with 30s polling)"
```

---

## Phase M — Admin SPA

### Task M1: Admin Queue + ActionPanel + pages

**Files:**
- Create: `web/src/components/admin/tk/Queue.tsx`
- Create: `web/src/components/admin/tk/ActionPanel.tsx`
- Create: `web/src/components/admin/tk/CategoryAdmin.tsx`
- Create: `web/src/pages/admin/tk/Queue.tsx`
- Create: `web/src/pages/admin/tk/Detail.tsx`
- Create: `web/src/pages/admin/tk/Categories.tsx`

```bash
mkdir -p web/src/components/admin/tk web/src/pages/admin/tk

cat > web/src/components/admin/tk/Queue.tsx <<'EOF'
import { Card, CardContent } from "@/components/ui/card";
import { StatusBadge } from "@/components/tk/StatusBadge";
import type { TKTicket } from "@/lib/types";

export function Queue({ rows }: { rows: TKTicket[] }) {
  if (rows.length === 0) return (
    <Card><CardContent className="py-10 text-center text-sm text-muted-foreground">
      No tickets match the current filters.
    </CardContent></Card>
  );
  return (
    <table className="w-full border-collapse text-sm">
      <thead className="text-left text-xs uppercase tracking-[0.08em] text-muted-foreground">
        <tr>
          <th className="py-2">Tracking</th>
          <th className="py-2">Subject</th>
          <th className="py-2">Customer</th>
          <th className="py-2">Category</th>
          <th className="py-2">Status</th>
          <th className="py-2">Assignee</th>
          <th className="py-2">Updated</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((t) => (
          <tr key={t.id} className="border-t border-border">
            <td className="py-2 font-mono text-xs">
              <a href={`./tickets/${encodeURIComponent(t.trackingNumber)}`} className="hover:underline">{t.trackingNumber}</a>
            </td>
            <td className="py-2"><a href={`./tickets/${encodeURIComponent(t.trackingNumber)}`} className="hover:underline">{t.subject}</a></td>
            <td className="py-2 text-xs text-muted-foreground">{t.customerEmail}</td>
            <td className="py-2 text-xs">{t.category?.name ?? `#${t.categoryId}`}</td>
            <td className="py-2"><StatusBadge status={t.status} /></td>
            <td className="py-2 font-mono text-xs">{t.assignedAdminId ?? "—"}</td>
            <td className="py-2 text-xs text-muted-foreground">{new Date(t.updatedAt).toLocaleString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
EOF

cat > web/src/components/admin/tk/ActionPanel.tsx <<'EOF'
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { assignTKAdmin, noteTKAdmin, statusTKAdmin } from "@/api/tkAdmin";
import type { TKTicket } from "@/lib/types";

type Props = {
  ticket: TKTicket;
  onChange: () => Promise<void>;
};

export function ActionPanel({ ticket, onChange }: Props) {
  const [note, setNote] = useState("");
  const [assignee, setAssignee] = useState(ticket.assignedAdminId ?? "");

  async function postStatus(to: string) {
    try { await statusTKAdmin(ticket.trackingNumber, to); await onChange(); toast.success(`Status → ${to}`); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Status change failed"); }
  }

  async function postAssign() {
    const trimmed = assignee.trim();
    try {
      await assignTKAdmin(ticket.trackingNumber, trimmed === "" ? null : trimmed);
      await onChange();
      toast.success(trimmed === "" ? "Unassigned." : `Assigned to ${trimmed}`);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Assign failed"); }
  }

  async function postNote() {
    if (!note.trim()) return;
    try { await noteTKAdmin(ticket.trackingNumber, note.trim()); setNote(""); await onChange(); toast.success("Internal note added."); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Note failed"); }
  }

  return (
    <Card>
      <CardHeader><CardTitle className="text-base">Actions</CardTitle></CardHeader>
      <CardContent className="space-y-3 text-sm">
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Status</p>
          <div className="flex flex-wrap gap-1">
            {(["in_progress","waiting_customer","resolved","closed"] as const).map((s) => (
              <Button key={s} size="sm" variant={ticket.status === s ? "default" : "outline"} onClick={() => postStatus(s)}>{s}</Button>
            ))}
          </div>
        </div>
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Assignee</p>
          <div className="flex gap-1">
            <Input value={assignee} onChange={(e) => setAssignee(e.target.value)} placeholder="admin id (empty = unassigned)" />
            <Button size="sm" onClick={postAssign}>Set</Button>
          </div>
        </div>
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Internal note</p>
          <Textarea rows={3} value={note} onChange={(e) => setNote(e.target.value)} placeholder="Visible to admins only" />
          <Button size="sm" onClick={postNote} disabled={!note.trim()}>Add note</Button>
        </div>
      </CardContent>
    </Card>
  );
}
EOF

cat > web/src/components/admin/tk/CategoryAdmin.tsx <<'EOF'
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  createTKCategoryAdmin, createTKFieldAdmin, createTKSubcategoryAdmin,
  deleteTKCategoryAdmin, deleteTKFieldAdmin, deleteTKSubcategoryAdmin,
  listTKFieldsAdmin, listTKSubcategoriesAdmin, updateTKCategoryAdmin,
  updateTKFieldAdmin, updateTKSubcategoryAdmin,
} from "@/api/tkAdmin";
import type { TKCategory, TKCategoryField, TKSubcategory } from "@/lib/types";

type Props = { initial: TKCategory[] };

export function CategoryAdmin({ initial }: Props) {
  const [cats, setCats] = useState<TKCategory[]>(initial);
  const [selected, setSelected] = useState<TKCategory | null>(initial[0] ?? null);
  const [subs, setSubs] = useState<TKSubcategory[]>([]);
  const [fields, setFields] = useState<TKCategoryField[]>([]);

  useEffect(() => {
    if (!selected) { setSubs([]); setFields([]); return; }
    listTKSubcategoriesAdmin(selected.id).then(setSubs).catch(() => {});
    listTKFieldsAdmin(selected.id).then(setFields).catch(() => {});
  }, [selected]);

  async function addCategory() {
    const name = window.prompt("Category name?")?.trim();
    if (!name) return;
    const slug = window.prompt("Slug?", name.toLowerCase().replace(/\s+/g, "-"))?.trim() ?? "";
    if (!slug) return;
    try {
      const c = await createTKCategoryAdmin({ slug, name, sortOrder: cats.length, active: true });
      setCats((rs) => [...rs, c]); setSelected(c);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Create failed"); }
  }

  async function saveCategory(c: TKCategory) {
    try {
      const u = await updateTKCategoryAdmin(c.id, { slug: c.slug, name: c.name, sortOrder: c.sortOrder, active: c.active });
      setCats((rs) => rs.map((x) => x.id === u.id ? u : x));
      if (selected?.id === u.id) setSelected(u);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Save failed"); }
  }

  async function removeCategory(c: TKCategory) {
    if (!confirm(`Delete category "${c.name}"?`)) return;
    try {
      await deleteTKCategoryAdmin(c.id);
      setCats((rs) => rs.filter((x) => x.id !== c.id));
      if (selected?.id === c.id) setSelected(null);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Delete failed"); }
  }

  async function addSubcategory() {
    if (!selected) return;
    const name = window.prompt("Subcategory name?")?.trim(); if (!name) return;
    const slug = window.prompt("Slug?", name.toLowerCase().replace(/\s+/g, "-"))?.trim(); if (!slug) return;
    try {
      const s = await createTKSubcategoryAdmin({ categoryId: selected.id, slug, name, sortOrder: subs.length, active: true });
      setSubs((rs) => [...rs, s]);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Create failed"); }
  }

  async function removeSubcategory(s: TKSubcategory) {
    if (!confirm(`Delete subcategory "${s.name}"?`)) return;
    try { await deleteTKSubcategoryAdmin(s.id); setSubs((rs) => rs.filter((x) => x.id !== s.id)); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Delete failed"); }
  }

  async function addField() {
    if (!selected) return;
    const key = window.prompt("Field key (e.g. order_id)?")?.trim(); if (!key) return;
    const label = window.prompt("Label?", key)?.trim() ?? key;
    const kind = (window.prompt("Kind: text / textarea / number / url", "text") ?? "text").trim();
    if (!["text","textarea","number","url"].includes(kind)) { toast.error("Invalid kind"); return; }
    try {
      const f = await createTKFieldAdmin({
        categoryId: selected.id, key, label,
        kind: kind as TKCategoryField["kind"],
        required: false, sortOrder: fields.length,
      });
      setFields((rs) => [...rs, f]);
    } catch (e) { toast.error(e instanceof Error ? e.message : "Create failed"); }
  }

  async function removeField(f: TKCategoryField) {
    if (!confirm(`Delete field "${f.label}"?`)) return;
    try { await deleteTKFieldAdmin(f.id); setFields((rs) => rs.filter((x) => x.id !== f.id)); }
    catch (e) { toast.error(e instanceof Error ? e.message : "Delete failed"); }
  }

  return (
    <div className="grid gap-4 md:grid-cols-3">
      <Card>
        <CardHeader><CardTitle>Categories</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          <ul className="space-y-1">
            {cats.map((c) => (
              <li key={c.id} className={`flex items-center gap-2 rounded px-2 py-1 text-sm ${selected?.id === c.id ? "bg-accent/20" : ""}`}>
                <button onClick={() => setSelected(c)} className="flex-1 text-left">{c.name}</button>
                <Switch checked={c.active} onCheckedChange={(v) => saveCategory({ ...c, active: v })} />
                <Button size="sm" variant="destructive" onClick={() => removeCategory(c)}>×</Button>
              </li>
            ))}
          </ul>
          <Button size="sm" onClick={addCategory}>+ Add category</Button>
        </CardContent>
      </Card>

      <Card className="md:col-span-2">
        <CardHeader><CardTitle>{selected ? `${selected.name} — details` : "Pick a category"}</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          {selected && (
            <>
              <div className="space-y-1">
                <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">Rename</p>
                <div className="flex gap-2">
                  <Input value={selected.name} onChange={(e) => setSelected({ ...selected, name: e.target.value })}
                         onBlur={() => saveCategory(selected)} />
                </div>
              </div>

              <div>
                <p className="mb-1 text-xs uppercase tracking-[0.08em] text-muted-foreground">Subcategories</p>
                <ul className="space-y-1 text-sm">
                  {subs.map((s) => (
                    <li key={s.id} className="flex items-center gap-2">
                      <Input value={s.name} onChange={(e) => setSubs((rs) => rs.map((x) => x.id === s.id ? { ...x, name: e.target.value } : x))}
                             onBlur={() => updateTKSubcategoryAdmin(s.id, { categoryId: s.categoryId, slug: s.slug, name: s.name, sortOrder: s.sortOrder, active: s.active })}
                             className="flex-1" />
                      <Button size="sm" variant="destructive" onClick={() => removeSubcategory(s)}>×</Button>
                    </li>
                  ))}
                </ul>
                <Button size="sm" className="mt-2" onClick={addSubcategory}>+ Add subcategory</Button>
              </div>

              <div>
                <p className="mb-1 text-xs uppercase tracking-[0.08em] text-muted-foreground">Form fields</p>
                <ul className="space-y-1 text-sm">
                  {fields.map((f) => (
                    <li key={f.id} className="grid grid-cols-12 items-center gap-2">
                      <span className="col-span-2 font-mono text-xs">{f.key}</span>
                      <Input className="col-span-4" value={f.label}
                             onChange={(e) => setFields((rs) => rs.map((x) => x.id === f.id ? { ...x, label: e.target.value } : x))}
                             onBlur={() => updateTKFieldAdmin(f.id, { categoryId: f.categoryId, key: f.key, label: f.label, kind: f.kind, required: f.required, sortOrder: f.sortOrder })} />
                      <select className="col-span-2 rounded border border-border bg-background px-2 py-1 text-sm"
                              value={f.kind}
                              onChange={(e) => {
                                const next = { ...f, kind: e.target.value as TKCategoryField["kind"] };
                                setFields((rs) => rs.map((x) => x.id === f.id ? next : x));
                                updateTKFieldAdmin(f.id, { categoryId: next.categoryId, key: next.key, label: next.label, kind: next.kind, required: next.required, sortOrder: next.sortOrder });
                              }}>
                        <option value="text">text</option>
                        <option value="textarea">textarea</option>
                        <option value="number">number</option>
                        <option value="url">url</option>
                      </select>
                      <label className="col-span-2 text-xs"><input type="checkbox" checked={f.required}
                        onChange={(e) => {
                          const next = { ...f, required: e.target.checked };
                          setFields((rs) => rs.map((x) => x.id === f.id ? next : x));
                          updateTKFieldAdmin(f.id, { categoryId: next.categoryId, key: next.key, label: next.label, kind: next.kind, required: next.required, sortOrder: next.sortOrder });
                        }} /> required</label>
                      <Button size="sm" variant="destructive" className="col-span-2" onClick={() => removeField(f)}>×</Button>
                    </li>
                  ))}
                </ul>
                <Button size="sm" className="mt-2" onClick={addField}>+ Add field</Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
EOF

cat > web/src/pages/admin/tk/Queue.tsx <<'EOF'
import { useEffect, useState } from "react";
import { Queue } from "@/components/admin/tk/Queue";
import { Input } from "@/components/ui/input";
import { listTKAdminQueue } from "@/api/tkAdmin";
import type { TKTicket } from "@/lib/types";

export function TKAdminQueue() {
  const [rows, setRows] = useState<TKTicket[]>([]);
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [assignee, setAssignee] = useState("");

  async function refresh() {
    try {
      const r = await listTKAdminQueue({
        q: q || undefined, status: status || undefined, assignee: assignee || undefined, limit: 200,
      });
      setRows(r);
    } catch {}
  }

  useEffect(() => {
    refresh();
    const iv = window.setInterval(refresh, 30_000);
    return () => window.clearInterval(iv);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q, status, assignee]);

  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Tickets</h2>
      <div className="flex flex-wrap items-center gap-2">
        <Input className="max-w-sm" placeholder="Tracking# or subject…" value={q} onChange={(e) => setQ(e.target.value)} />
        <select value={status} onChange={(e) => setStatus(e.target.value)} className="rounded border border-border bg-background px-2 py-1 text-sm">
          <option value="">All statuses</option>
          <option value="open">Open</option>
          <option value="in_progress">In progress</option>
          <option value="waiting_customer">Waiting on customer</option>
          <option value="resolved">Resolved</option>
          <option value="closed">Closed</option>
        </select>
        <select value={assignee} onChange={(e) => setAssignee(e.target.value)} className="rounded border border-border bg-background px-2 py-1 text-sm">
          <option value="">All assignees</option>
          <option value="__mine__">Mine</option>
          <option value="__unassigned__">Unassigned</option>
        </select>
      </div>
      <Queue rows={rows} />
    </section>
  );
}
EOF

cat > web/src/pages/admin/tk/Detail.tsx <<'EOF'
import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";

import { ActionPanel } from "@/components/admin/tk/ActionPanel";
import { ReplyBox } from "@/components/tk/ReplyBox";
import { StatusBadge } from "@/components/tk/StatusBadge";
import { Thread } from "@/components/tk/Thread";
import { Card, CardContent } from "@/components/ui/card";
import { getTKAdminTicket, replyTKAdmin } from "@/api/tkAdmin";
import type { TKEntry, TKTicket } from "@/lib/types";

export function TKAdminDetail() {
  const tn = decodeURIComponent(window.location.pathname.split("/admin/tickets/")[1] ?? "");
  const [ticket, setTicket] = useState<TKTicket | null>(null);
  const [entries, setEntries] = useState<TKEntry[]>([]);
  const [err, setErr] = useState("");

  async function refresh() {
    try {
      const r = await getTKAdminTicket(tn);
      setTicket(r.ticket); setEntries(r.entries);
    } catch (e) { setErr(e instanceof Error ? e.message : "Not found"); }
  }

  useEffect(() => {
    refresh();
    const iv = window.setInterval(refresh, 30_000);
    return () => window.clearInterval(iv);
  }, [tn]);

  if (err) return (
    <main className="mx-auto max-w-5xl px-4 py-16 text-center">
      <h1 className="text-2xl font-semibold">Ticket unavailable</h1>
      <p className="text-muted-foreground">{err}</p>
      <a href="../tickets" className="mt-4 inline-flex items-center gap-1 text-sm text-accent">
        <ArrowLeft className="h-4 w-4" /> Back to queue
      </a>
    </main>
  );
  if (!ticket) return <p className="px-4 py-8 text-sm text-muted-foreground">Loading…</p>;

  return (
    <section className="grid gap-6 md:grid-cols-3">
      <div className="space-y-5 md:col-span-2">
        <a href="../tickets" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" /> Back to queue
        </a>
        <header className="space-y-1">
          <p className="font-mono text-xs text-muted-foreground">{ticket.trackingNumber}</p>
          <h2 className="text-2xl font-semibold">{ticket.subject}</h2>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <StatusBadge status={ticket.status} />
            <span>{ticket.customerEmail}</span>
            {ticket.category && <span>· {ticket.category.name}{ticket.subcategory ? ` · ${ticket.subcategory.name}` : ""}</span>}
          </div>
        </header>
        {ticket.fieldValues && ticket.fieldValues.length > 0 && (
          <Card>
            <CardContent className="py-3">
              <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground mb-2">Form fields</p>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                {ticket.fieldValues.map((fv) => (
                  <div key={fv.fieldId}>
                    <dt className="text-xs text-muted-foreground">{fv.fieldLabel}</dt>
                    <dd>{fv.value}</dd>
                  </div>
                ))}
              </dl>
            </CardContent>
          </Card>
        )}
        <Thread entries={entries} isAdmin />
        <ReplyBox onSubmit={async (body) => {
          try { await replyTKAdmin(tn, body); await refresh(); toast.success("Reply sent."); }
          catch (e) { toast.error(e instanceof Error ? e.message : "Send failed"); }
        }} disabled={ticket.status === "closed"} />
      </div>
      <ActionPanel ticket={ticket} onChange={refresh} />
    </section>
  );
}
EOF

cat > web/src/pages/admin/tk/Categories.tsx <<'EOF'
import { useEffect, useState } from "react";
import { CategoryAdmin } from "@/components/admin/tk/CategoryAdmin";
import { listTKCategoriesAdmin } from "@/api/tkAdmin";
import type { TKCategory } from "@/lib/types";

export function TKAdminCategories() {
  const [initial, setInitial] = useState<TKCategory[] | null>(null);
  useEffect(() => { listTKCategoriesAdmin().then(setInitial).catch(() => setInitial([])); }, []);
  return (
    <section className="space-y-4">
      <h2 className="text-2xl font-semibold">Ticket categories</h2>
      {initial === null ? <p className="text-sm text-muted-foreground">Loading…</p> : <CategoryAdmin initial={initial} />}
    </section>
  );
}
EOF

cd web && pnpm exec tsc -b --noEmit && pnpm test
cd ..
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/components/admin/tk/ web/src/pages/admin/tk/
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): tickets admin pages (queue / detail / categories tree editor)"
```

---

## Phase N — Final wiring + smoke

### Task N1: App.tsx dispatcher for new modes

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
import { TKList } from "@/pages/tk/List";
import { TKNew } from "@/pages/tk/New";
import { TKDetail } from "@/pages/tk/Detail";
import { TKAdminQueue } from "@/pages/admin/tk/Queue";
import { TKAdminDetail } from "@/pages/admin/tk/Detail";
import { TKAdminCategories } from "@/pages/admin/tk/Categories";

export function App() {
  const bootstrap = readBootstrap();
  let page: ReactNode;
  switch (bootstrap.mode) {
    case "admin-home":             page = <AdminHome bootstrap={bootstrap} />; break;
    case "kb-browse":              page = <KBBrowse bootstrap={bootstrap} />; break;
    case "kb-detail":              page = <KBDetail />; break;
    case "admin-kb-list":          page = <KBAdminList />; break;
    case "admin-kb-edit":          page = <KBAdminEdit />; break;
    case "admin-kb-categories":    page = <KBAdminCategories />; break;
    case "admin-kb-tags":          page = <KBAdminTags />; break;
    case "speedtest":              page = <Speedtest />; break;
    case "admin-st-endpoints":     page = <STAdminEndpoints />; break;
    case "admin-st-geoip":         page = <STAdminGeoIP />; break;
    case "admin-st-results":       page = <STAdminResults />; break;
    case "admin-st-dashboards":    page = <STAdminDashboards />; break;
    case "tickets-list":           page = <TKList />; break;
    case "tickets-new":            page = <TKNew />; break;
    case "tickets-detail":         page = <TKDetail />; break;
    case "admin-tickets-queue":    page = <TKAdminQueue />; break;
    case "admin-tickets-detail":   page = <TKAdminDetail />; break;
    case "admin-tickets-categories": page = <TKAdminCategories />; break;
    default:                       page = <CustomerHome bootstrap={bootstrap} />;
  }
  return (
    <>
      {page}
      <Toaster />
    </>
  );
}
EOF

cd web && pnpm test && pnpm build 2>&1 | tail -10
cd ..
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/App.tsx
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(web): App dispatches tickets-* + admin-tickets-* modes"
```

### Task N2: Server-side `tickets-new` route + small SPA wiring

The customer "New ticket" page expects `GET /tickets/new` to render with `mode: tickets-new`. We didn't add that route in Phase H. Add it now:

In `internal/server/server.go`, just before the existing `r.Get("/tickets/{tracking_number}", ...)` line, add:

```go
	r.Get("/tickets/new", requireUser(hTKNewPage(d)))
```

(NOTE: chi matches in registration order, so this must be ABOVE the `{tracking_number}` route — chi's `{}` would otherwise swallow `new` as a tracking number.)

In `internal/server/handlers_tk_customer.go`, add:

```go
func hTKNewPage(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeSPA(w, r, supportBootstrap{
			Mode: "tickets-new", Theme: adminTheme(r),
			Modules: currentModules(r.Context(), d),
			UserID:  r.Header.Get("X-Continuum-User-Id"),
			IsAdmin: r.Header.Get("X-Continuum-User-Role") == "admin",
		}, http.StatusOK)
	}
}
```

Verify + commit:

```bash
go build ./...
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add internal/server/handlers_tk_customer.go internal/server/server.go
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat(server): add /tickets/new route for the new-ticket flow page"
```

### Task N3: Flip SHIPPED_MODULES.tickets

```bash
sed -i 's/tickets: false/tickets: true/' web/src/lib/modules.ts
grep 'tickets:' web/src/lib/modules.ts
```

### Task N4: README + final smoke

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
| Tickets | Shipped (v0.4) |
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

Speedtest-related config keys (all optional, sane defaults):

- `auto_strategy` — `latency` (default) or `geoip`
- `client_ip_storage` — `truncated` (default) or `off`
- `slow_threshold_mbps` — default `5`
- `geoip_cache_dir` — default `$XDG_CACHE_HOME/continuum-plugin-support/geoip/`

Tickets-related config keys:

- `tickets_auto_close_enabled` — default `true`. Set to `false` to disable the auto-close cron entirely.
- `tickets_resolved_close_after_days` — default `7`. Setting to `0` skips the resolved-pass while keeping the waiting-pass running.
- `tickets_waiting_close_after_days` — default `14`. Setting to `0` skips the waiting-pass.

## Events emitted

Routed via the existing `continuum.notifications` plugin per admin rules.

**KB:** `kb_article_published / _updated / _unhelpful`
**Speedtest:** `speedtest_run / _slow`
**Tickets:** `ticket_submitted / _replied / _status_changed / _assigned / _resolved / _reopened / _closed`

## Crons (admin-trigger endpoints)

- KB: `POST /api/admin/kb/cron/run` (publish-due + unhelpful sweep)
- Tickets: `POST /api/admin/tickets/cron/run` (auto-close idle)
- GeoIP mmdb refresh: automatic on plugin start; manual via `POST /api/admin/speedtest/geoip/{id}/refresh`

Native `scheduled_task.v1` SDK wiring is a follow-up for all three.
EOF

make build
make test
```

### Task N5: Commit + push

```bash
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  add web/src/lib/modules.ts README.md
git -c user.name="Claude Code" -c user.email="noreply@anthropic.com" \
  commit -m "feat: ship tickets — flip SHIPPED_MODULES.tickets + README"
git log --oneline | head -5
git push 2>&1 | tail -5
```

---

## Self-Review

**Spec coverage:**

- All 8 tables (`tk_categories`, `tk_subcategories`, `tk_category_fields`, `tk_tickets`, `tk_ticket_entries`, `tk_ticket_field_values`, `tk_attachments`, `tk_ticket_sequence`) + indexes + seed → Task A1.
- Tracking number sequence atomicity → Task B2 (`TKNextTrackingNumber` uses `UPDATE … RETURNING`).
- Lifecycle transition map + reopen window → Task C1.
- Auto-close cron with operator-configurable enable / day thresholds → Task D1.
- Customer ticket CRUD + reply + reopen + categories form → Tasks E1 + E2.
- Admin queue / detail / reply / note / status / assign + categories + subcategories + fields CRUD → Task F1.
- Attachment upload + serve with 10 MB cap and ticket-owner / admin auth gate → Task G1.
- Events (`ticket_submitted / _replied / _status_changed / _assigned / _resolved / _reopened / _closed`) → Task E2 + F1 + D1.
- Routes + manifest 0.4.0 + main.go wiring + runtime defaults → Task H1.
- Integration tests covering auth gates, lifecycle round-trip, attachment 413 → Task J1.
- SPA types + bootstrap modes + API clients → Tasks K1 + K2.
- Customer SPA (list / new flow / detail with 30s polling) → Tasks L1 + L2.
- Admin SPA (queue with 30s polling / detail with action panel / category tree editor) → Task M1.
- App dispatcher + `/tickets/new` server route + SHIPPED flip + README → Tasks N1-N5.

**Coverage gap noted:**

- The spec mentions a "Last speedtest: 220 Mbps, 3 min ago" cross-module display on the admin detail page (when speedtest module is enabled). NOT implemented in this plan — would need a new endpoint or a join through `st_results`. Documented as a v1.1 follow-up.
- The cron runs as an admin-trigger endpoint (`POST /api/admin/tickets/cron/run`) rather than via `scheduled_task.v1`. Same fallback the KB and speedtest modules use. Acknowledged in the README.

**Placeholder scan:** searched for `TODO` / `TBD` / `Similar to Task` / `implement later` — none present. The 60s rate-limit pattern, IP truncation, and event-publish helpers all reuse code from prior modules verbatim, with explicit copy in this plan rather than cross-task references.

**Type / method-name consistency:**

- Go: `TKTicket` / `TKEntry` / `TKAttachment` / `TKCategory` / `TKSubcategory` / `TKCategoryField` / `TKFieldValue` / `TKAttachmentMeta` / `TKTicketListFilter` used consistently across types → store → handlers → tests.
- Go store methods consistently prefixed `TK*`.
- Event names: `ticket_submitted` / `_replied` / `_status_changed` / `_assigned` / `_resolved` / `_reopened` / `_closed` match the spec exactly.
- TS types mirror Go shapes via JSON-tag camelCase (`trackingNumber`, `customerEmail`, `assignedAdminId`, `resolvedAt`, `authorRole`, etc.).
- Bootstrap modes: `tickets-list` / `tickets-new` / `tickets-detail` / `admin-tickets-queue` / `admin-tickets-detail` / `admin-tickets-categories` — match Go side and the App.tsx switch.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-21-support-tickets.md`. Two execution options:

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, two-stage review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using `executing-plans`, batch execution with checkpoints.

Which approach?
