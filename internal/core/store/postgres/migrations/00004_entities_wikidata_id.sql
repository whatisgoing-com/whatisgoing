-- +goose Up
ALTER TABLE entities ADD COLUMN wikidata_id text;
-- Partial (not full-table) unique index, scoped to (wikidata_id, type)
-- rather than wikidata_id alone: entity resolution (issue #26) merges
-- are type-scoped (a PERSON and an ORG entity are never merged into each
-- other even if a mistyped mention resolves both to the same QID), so
-- the same QID legitimately appearing once per type must stay possible
-- rather than erroring the resolver job.
CREATE UNIQUE INDEX entities_wikidata_id_idx ON entities (wikidata_id, type) WHERE wikidata_id IS NOT NULL;

-- +goose Down
DROP INDEX entities_wikidata_id_idx;
ALTER TABLE entities DROP COLUMN wikidata_id;
