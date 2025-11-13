-- +goose Up
CREATE TYPE business_role AS ENUM ('creator', 'admin', 'employee');

CREATE TABLE businesses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_businesses_owner_id ON businesses(owner_id);

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

-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS create_business_and_add_owner(UUID, TEXT);
DROP TABLE IF EXISTS business_members;
DROP TABLE IF EXISTS businesses;
DROP TYPE IF EXISTS business_role;
