CREATE TABLE kb_categories (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kb_tags (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    label       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kb_articles (
    id              BIGSERIAL PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    body_html       TEXT NOT NULL,
    body_text       TEXT NOT NULL,
    category_id     BIGINT NOT NULL REFERENCES kb_categories(id) ON DELETE RESTRICT,
    status          TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','published')),
    publish_at      TIMESTAMPTZ,
    published_at    TIMESTAMPTZ,
    last_edited_by  TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    search_vector   tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title,'')),     'A') ||
        setweight(to_tsvector('english', coalesce(summary,'')),   'B') ||
        setweight(to_tsvector('english', coalesce(body_text,'')), 'C')
    ) STORED
);
CREATE INDEX kb_articles_search_idx   ON kb_articles USING GIN (search_vector);
CREATE INDEX kb_articles_category_idx ON kb_articles (category_id, status, published_at DESC);
CREATE INDEX kb_articles_schedule_idx ON kb_articles (publish_at) WHERE status = 'draft';

CREATE TABLE kb_article_tags (
    article_id BIGINT NOT NULL REFERENCES kb_articles(id) ON DELETE CASCADE,
    tag_id     BIGINT NOT NULL REFERENCES kb_tags(id)     ON DELETE RESTRICT,
    PRIMARY KEY (article_id, tag_id)
);
CREATE INDEX kb_article_tags_tag_idx ON kb_article_tags(tag_id);

CREATE TABLE kb_images (
    id          BIGSERIAL PRIMARY KEY,
    article_id  BIGINT REFERENCES kb_articles(id) ON DELETE SET NULL,
    filename    TEXT NOT NULL,
    mime        TEXT NOT NULL,
    bytes       BIGINT NOT NULL,
    content     BYTEA NOT NULL,
    sha256      BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kb_votes (
    article_id  BIGINT NOT NULL REFERENCES kb_articles(id) ON DELETE CASCADE,
    customer_id TEXT   NOT NULL,
    vote        TEXT   NOT NULL CHECK (vote IN ('up','down')),
    voted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (article_id, customer_id)
);

CREATE TABLE kb_views (
    id          BIGSERIAL PRIMARY KEY,
    article_id  BIGINT NOT NULL REFERENCES kb_articles(id) ON DELETE CASCADE,
    customer_id TEXT   NOT NULL,
    viewed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX kb_views_article_idx ON kb_views (article_id, viewed_at DESC);
CREATE INDEX kb_views_dedup_idx   ON kb_views (article_id, customer_id, viewed_at DESC);
