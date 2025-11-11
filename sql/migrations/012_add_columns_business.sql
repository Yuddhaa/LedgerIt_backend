-- +goose Up
-- Create the new ENUM type for expense mode
CREATE TYPE expense_mode AS ENUM ('online', 'check', 'cash');

-- Add party_id to the expenses table
-- We make it NULLable and ON DELETE SET NULL for safety on an existing table
ALTER TABLE expenses
ADD COLUMN party_id UUID REFERENCES parties(id) ON DELETE SET NULL;

-- Add the new mode column, making it nullable
ALTER TABLE expenses
ADD COLUMN mode expense_mode;

-- Add an index for the new foreign key
CREATE INDEX idx_expenses_party_id ON expenses(party_id);

-- +goose Down
-- Drop in reverse order
ALTER TABLE expenses
DROP COLUMN IF EXISTS mode;

ALTER TABLE expenses
DROP COLUMN IF EXISTS party_id; -- The index idx_expenses_party_id is dropped automatically

DROP TYPE IF EXISTS expense_mode;
