-- +goose Up
-- This extension is needed for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create the foundational users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT,
    email TEXT UNIQUE NOT NULL,
    phone_number TEXT UNIQUE,
    google_id TEXT UNIQUE NOT NULL,
    picture TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS users;
-- We generally don't drop extensions in a down migration
-- as other tables might depend on it.
-- DROP EXTENSION IF EXISTS "pgcrypto";
