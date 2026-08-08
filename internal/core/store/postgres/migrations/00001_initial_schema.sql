-- +goose Up
CREATE TABLE sources (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    url        text NOT NULL,
    type       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Persistent (cross-run) article dedup lives here via the UNIQUE constraint
-- on dedup_key: fetcher/pipeline code only filters duplicates within a
-- single fetch, this is what stops the same article being saved twice
-- across separate runs.
CREATE TABLE articles (
    id           bigserial PRIMARY KEY,
    source_id    text NOT NULL REFERENCES sources(id),
    dedup_key    text NOT NULL UNIQUE CHECK (dedup_key <> ''),
    url          text NOT NULL,
    title        text NOT NULL,
    content      text NOT NULL,
    published_at timestamptz,
    fetched_at   timestamptz NOT NULL DEFAULT now(),
    -- NULL until successfully submitted to the search index; the
    -- reconciliation job re-attempts indexing for any row still NULL.
    indexed_at   timestamptz
);
CREATE INDEX articles_published_at_idx ON articles (published_at);
CREATE INDEX articles_unindexed_idx ON articles (id) WHERE indexed_at IS NULL;

CREATE TYPE entity_type AS ENUM ('PERSON', 'ORG', 'EVENT');

CREATE TABLE entities (
    id         bigserial PRIMARY KEY,
    name       text NOT NULL,
    type       entity_type NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name, type)
);

-- Per-article, per-entity sentiment: sentence-level sentiment scores
-- (issue #3) get averaged into sentiment_score per entity per article here.
CREATE TABLE mentions (
    id              bigserial PRIMARY KEY,
    article_id      bigint NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    entity_id       bigint NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    sentiment_score real,
    mention_count   integer NOT NULL DEFAULT 1,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (article_id, entity_id)
);
CREATE INDEX mentions_entity_id_idx ON mentions (entity_id);

-- Raw co-occurrence facts (one row per entity pair per article); batch
-- aggregation jobs roll these up into windowed relationship strength.
-- entity_a_id < entity_b_id keeps each pair canonical (one row, not two).
CREATE TABLE entity_cooccurrence (
    article_id  bigint NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    entity_a_id bigint NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    entity_b_id bigint NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (article_id, entity_a_id, entity_b_id),
    CHECK (entity_a_id < entity_b_id)
);
CREATE INDEX entity_cooccurrence_entity_a_idx ON entity_cooccurrence (entity_a_id);
CREATE INDEX entity_cooccurrence_entity_b_idx ON entity_cooccurrence (entity_b_id);

-- +goose Down
DROP TABLE entity_cooccurrence;
DROP TABLE mentions;
DROP TABLE entities;
DROP TYPE entity_type;
DROP TABLE articles;
DROP TABLE sources;
