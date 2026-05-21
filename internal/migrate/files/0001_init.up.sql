CREATE TABLE app_config (
    id          SMALLINT PRIMARY KEY CHECK (id = 1),
    data        JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO app_config (id, data) VALUES (1, '{}'::jsonb)
ON CONFLICT (id) DO NOTHING;
