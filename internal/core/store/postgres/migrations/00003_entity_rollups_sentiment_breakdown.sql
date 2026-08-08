-- +goose Up

-- Positive/neutral/negative mention counts per (entity, window, window
-- start), bucketed the same way the UI already classifies a sentiment
-- score (> 0 positive, < 0 negative, = 0 neutral). Alongside the existing
-- averaged sentiment_score, this backs the dashboard's sentiment
-- breakdown pie charts (issue #21) without needing a live query over raw
-- mentions.
ALTER TABLE entity_rollups
    ADD COLUMN positive_count integer NOT NULL DEFAULT 0,
    ADD COLUMN neutral_count integer NOT NULL DEFAULT 0,
    ADD COLUMN negative_count integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE entity_rollups
    DROP COLUMN positive_count,
    DROP COLUMN neutral_count,
    DROP COLUMN negative_count;
