-- +goose Up
CREATE TYPE rollup_window AS ENUM ('day', 'week', 'month', 'year');

-- One row per (entity, window granularity, window start): ranked
-- mention-frequency ("hot topics") and average sentiment ("reputation
-- trend") for that window. Recomputed in full by the batch aggregation job
-- (issue #5) rather than updated incrementally.
-- window_kind, not window: "window" is a reserved word in Postgres' SQL
-- grammar (window functions) and can't be used as an unquoted column name.
CREATE TABLE entity_rollups (
    entity_id      bigint NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    window_kind    rollup_window NOT NULL,
    window_start   date NOT NULL,
    mention_count  integer NOT NULL,
    sentiment_score real NOT NULL,
    computed_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_id, window_kind, window_start)
);

-- Serves "top N entities for this window/window_start" (hot topics).
CREATE INDEX entity_rollups_ranking_idx ON entity_rollups (window_kind, window_start, mention_count DESC);

-- +goose Down
DROP TABLE entity_rollups;
DROP TYPE rollup_window;
