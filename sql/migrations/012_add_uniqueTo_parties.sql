-- +goose Up
-- Add a unique constraint to ensure a party's name
-- is unique within a specific business.
-- This will also automatically create a new b-tree index
-- on (business_id, name).
ALTER TABLE parties
ADD CONSTRAINT unique_party_name_per_business
UNIQUE (business_id, name);

-- +goose Down
-- Remove the unique constraint from the parties table.
ALTER TABLE parties
DROP CONSTRAINT unique_party_name_per_business;
