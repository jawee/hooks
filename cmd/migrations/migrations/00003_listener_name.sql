-- +goose Up
ALTER TABLE listeners ADD COLUMN name TEXT;

-- +goose Down
ALTER TABLE listeners DROP COLUMN name;
