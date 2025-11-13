-- +goose Up
CREATE TABLE parties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    place TEXT,
    ph_no TEXT,
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_parties_business_id ON parties(business_id);
-- Optional: Add a unique constraint for name per business if needed
-- CREATE UNIQUE INDEX idx_parties_business_id_name ON parties(business_id, name);

-- +goose Down
DROP TABLE IF EXISTS parties;
