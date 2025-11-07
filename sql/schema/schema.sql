-- This extension is needed for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- This is a placeholder for your actual 'users' table.
-- sqlc needs this to understand the foreign key references.
-- Make sure this matches your real 'users' migration.
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT,
    email TEXT UNIQUE NOT NULL,
    phone_number TEXT,
    google_id TEXT UNIQUE NOT NULL,
    picture TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    device_info TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- Your ENUM Types ---

CREATE TYPE business_role AS ENUM ('creator', 'admin', 'employee');
CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'rejected');

-- --- Your 'businesses' Table ---

CREATE TABLE businesses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_businesses_owner_id ON businesses(owner_id);

-- --- Your 'business_members' Table ---

CREATE TABLE business_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    role business_role NOT NULL,
    current_balance DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A user can only have one role per business
    CONSTRAINT unique_user_business UNIQUE (user_id, business_id)
);

CREATE INDEX idx_business_members_user_id ON business_members(user_id);
CREATE INDEX idx_business_members_business_id ON business_members(business_id);
