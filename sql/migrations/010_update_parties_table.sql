
-- +goose Up
-- +goose StatementBegin

-- 1. Update NULL values to something non-null before constraint
UPDATE parties
SET place = 'Unknown'
WHERE place IS NULL;

-- 2. Alter the column to be NOT NULL
ALTER TABLE parties
ALTER COLUMN place SET NOT NULL;

-- 3. Create index on place
CREATE INDEX idx_parties_place ON parties(place);

-- 4. Create composite index on (business_id, place)
CREATE INDEX idx_parties_business_id_place ON parties(business_id, place);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop the indexes
DROP INDEX IF EXISTS idx_parties_business_id_place;
DROP INDEX IF EXISTS idx_parties_place;

-- Revert the column back to nullable
ALTER TABLE parties
ALTER COLUMN place DROP NOT NULL;

-- +goose StatementEnd
