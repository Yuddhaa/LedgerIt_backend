
-- +goose Up
ALTER TABLE parties
    RENAME COLUMN ph_no TO phone_number;

-- +goose Down
ALTER TABLE parties
    RENAME COLUMN phone_number TO ph_no;
