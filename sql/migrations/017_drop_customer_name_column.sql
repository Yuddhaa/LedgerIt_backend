-- +goose Up
-- This removes the redundant customer_name column, as party_id is now used instead.
-- We use IF EXISTS to ensure the migration doesn't fail on a fresh DB
-- that never had this column.
ALTER TABLE transactions
DROP COLUMN IF EXISTS customer_name;

-- +goose Down
-- If we need to roll back, we add the column back as TEXT.
ALTER TABLE transactions
ADD COLUMN customer_name TEXT;
