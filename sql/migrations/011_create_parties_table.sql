-- +goose Up
-- Create the parties table
CREATE TABLE parties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    phone_number TEXT,
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Add indexes for common lookups
CREATE INDEX idx_parties_business_id ON parties(business_id);
CREATE INDEX idx_parties_phone_number ON parties(phone_number);

-- +goose Down
-- Drop in reverse order of creation
DROP TABLE IF EXISTS parties;
