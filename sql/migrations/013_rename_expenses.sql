-- +goose Up
-- Rename the table from 'expenses' to 'transactions'
ALTER TABLE expenses RENAME TO transactions;

-- Rename the index
ALTER INDEX idx_expenses_party_id RENAME TO idx_transactions_party_id;

-- Rename the associated ENUM type
ALTER TYPE expense_mode RENAME TO transaction_mode;

-- +goose Down
-- Revert in reverse order
ALTER TYPE transaction_mode RENAME TO expense_mode;

ALTER INDEX idx_transactions_party_id RENAME TO idx_expenses_party_id;

ALTER TABLE transactions RENAME TO expenses;
