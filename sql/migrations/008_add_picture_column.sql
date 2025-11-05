-- +goose Up
ALTER TABLE users ADD COLUMN picture TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN picture;
