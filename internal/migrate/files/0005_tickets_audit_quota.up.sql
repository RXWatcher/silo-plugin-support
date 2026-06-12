-- Append-only audit trail for support actions that touch customer PII
-- (admin status changes, assignments, replies, internal notes). Rows are
-- never updated or deleted by application code; the table records which
-- admin acted, on which ticket, what they did, and when.
CREATE TABLE tk_audit_log (
    id          BIGSERIAL PRIMARY KEY,
    ticket_id   BIGINT NOT NULL REFERENCES tk_tickets(id) ON DELETE CASCADE,
    actor_id    TEXT NOT NULL,
    actor_role  TEXT NOT NULL DEFAULT 'admin',
    action      TEXT NOT NULL
        CHECK (action IN ('reply','note','status_change','assign')),
    detail      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX tk_audit_log_ticket_idx ON tk_audit_log(ticket_id, created_at);
CREATE INDEX tk_audit_log_actor_idx  ON tk_audit_log(actor_id, created_at);

-- Speeds up the per-customer open-ticket cap check (count of a
-- customer's tickets in non-terminal statuses).
CREATE INDEX tk_tickets_customer_open_idx
    ON tk_tickets(customer_id)
    WHERE status IN ('open','in_progress','waiting_customer');
