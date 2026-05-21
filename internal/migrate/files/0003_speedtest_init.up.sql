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

INSERT INTO st_geoip_sources (label, kind, config, sort_order, active) VALUES (
    'db-ip.com free country-lite',
    'mmdb_auto',
    '{"url_pattern": "https://download.db-ip.com/free/dbip-country-lite-{YYYY-MM}.mmdb.gz", "refresh_days": 25}'::jsonb,
    0,
    TRUE
);
