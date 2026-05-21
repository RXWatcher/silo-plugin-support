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
