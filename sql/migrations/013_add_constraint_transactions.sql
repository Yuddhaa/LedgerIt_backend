-- +goose Up
-- To add NOT NULL, we must first drop the old foreign key
-- that has the conflicting 'ON DELETE SET NULL' rule.

-- 1. Find the name of your foreign key. If you don't know it,
-- psql \d transactions
-- Or, you can just drop it by column definition:
ALTER TABLE transactions
DROP CONSTRAINT transactions_party_id_fkey; -- 'transactions_party_id_fkey' is a common default, yours might be different!

-- 2. Now, add the NOT NULL constraints
ALTER TABLE transactions
    ALTER COLUMN party_id SET NOT NULL,
    ALTER COLUMN receipt_no SET NOT NULL;

-- 3. Finally, re-add the foreign key with the correct (non-conflicting) rule
ALTER TABLE transactions
ADD CONSTRAINT transactions_party_id_fkey
FOREIGN KEY (party_id)
REFERENCES parties(id)
ON DELETE RESTRICT; -- Or ON DELETE NO ACTION


-- +goose Down
-- Revert all changes in reverse order

-- 1. Drop the new foreign key
ALTER TABLE transactions
DROP CONSTRAINT transactions_party_id_fkey;

-- 2. Drop the NOT NULL constraints
ALTER TABLE transactions
    ALTER COLUMN party_id DROP NOT NULL,
    ALTER COLUMN receipt_no DROP NOT NULL;

-- 3. Re-add the original foreign key
ALTER TABLE transactions
ADD CONSTRAINT transactions_party_id_fkey
FOREIGN KEY (party_id)
REFERENCES parties(id)
ON DELETE SET NULL;
