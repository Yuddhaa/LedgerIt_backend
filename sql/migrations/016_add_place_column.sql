-- +goose Up
ALTER TABLE parties
ADD COLUMN place TEXT;

-- +goose Down
ALTER TABLE parties
DROP COLUMN IF EXISTS place;
