-- +goose Up
-- Create the new ENUM type for transaction direction
CREATE TYPE transaction_direction AS ENUM ('in', 'out','deposit');

-- Add the new direction column to the transactions table, making it nullable
ALTER TABLE transactions
ADD COLUMN direction transaction_direction;

-- Add an index for the new column as it will be frequently queried
CREATE INDEX idx_transactions_direction ON transactions(direction);

-- +goose Down
-- Drop in reverse order
DROP INDEX IF EXISTS idx_transactions_direction;

ALTER TABLE transactions
DROP COLUMN IF EXISTS direction;

DROP TYPE IF EXISTS transaction_direction;
