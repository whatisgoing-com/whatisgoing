-- +goose Up
ALTER TABLE entities ADD COLUMN description text;

-- +goose Down
ALTER TABLE entities DROP COLUMN description;
