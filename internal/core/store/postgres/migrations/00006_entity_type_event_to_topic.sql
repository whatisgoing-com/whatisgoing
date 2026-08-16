-- +goose Up
-- The EVENT extractor (spaCy NER) was poor quality — see issue #39.
-- Replaced with topic extraction (YAKE keyphrase extraction), so the
-- label changes to TOPIC. Renaming the enum value in place (rather than
-- add-new/drop-old) relabels every existing EVENT-typed row to TOPIC
-- without touching any data — this is v1 and never released, so a
-- schema change is fine, but there is no reason to erase anything.
ALTER TYPE entity_type RENAME VALUE 'EVENT' TO 'TOPIC';

-- +goose Down
ALTER TYPE entity_type RENAME VALUE 'TOPIC' TO 'EVENT';
