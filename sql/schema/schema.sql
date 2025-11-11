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

-- --- NEW ENUM Types (from 011, 012, 013, 014) ---
CREATE TYPE party_type AS ENUM ('customer', 'supplier');
CREATE TYPE transaction_mode AS ENUM ('online', 'check', 'cash'); -- Renamed from expense_mode
CREATE TYPE transaction_direction AS ENUM ('in', 'out'); -- Added in 014


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

    -- A user can only be in one role per business
    CONSTRAINT unique_user_business UNIQUE (user_id, business_id)
);

CREATE INDEX idx_business_members_user_id ON business_members(user_id);
CREATE INDEX idx_business_members_business_id ON business_members(business_id);

-- --- 'parties' Table (from 011) ---
CREATE TABLE parties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    type party_type NOT NULL,
    phone_number TEXT,
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_parties_business_id ON parties(business_id);
CREATE INDEX idx_parties_phone_number ON parties(phone_number);
CREATE INDEX idx_parties_type ON parties(type);

-- --- 'transactions' Table (Renamed from 'expenses' in 013) ---
-- NOTE: The 'expenses' table was not provided in the original schema.
-- This is an assumed structure based on the request to add columns.
-- Your actual 'expenses' table migration should come before 012.
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    amount DECIMAL(10, 2) NOT NULL,
    description TEXT,
    
    -- Columns added in 012
    party_id UUID REFERENCES parties(id) ON DELETE SET NULL,
    mode transaction_mode, -- Renamed from expense_mode
    
    -- Column added in 014
    direction transaction_direction,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index (Renamed in 013)
CREATE INDEX idx_transactions_party_id ON transactions(party_id);
-- Index (Added in 014)
CREATE INDEX idx_transactions_direction ON transactions(direction);


-- --- Your Functions ---
-- (This is the same function from your migration,
-- so sqlc can understand its arguments and return type)

CREATE OR REPLACE FUNCTION create_business_and_add_owner(
    p_owner_id UUID,
    p_name TEXT
)
RETURNS businesses -- Returns the complete new business row
LANGUAGE plpgsql
AS $$
DECLARE
    new_business businesses;
BEGIN
    -- Create the business
    INSERT INTO businesses (name, owner_id)
    VALUES (p_name, p_owner_id)
    RETURNING * INTO new_business;

    -- Add the owner as a 'creator'
    INSERT INTO business_members (user_id, business_id, role)
    VALUES (p_owner_id, new_business.id, 'creator');

    -- Return the business created in the first step
    RETURN new_business;
END;
$$;
