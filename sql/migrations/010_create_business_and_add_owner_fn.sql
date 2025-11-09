-- +goose Up
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
    -- Note: Assumes your enum value is 'creator'. Update if it's different.
    INSERT INTO business_members (user_id, business_id, role)
    VALUES (p_owner_id, new_business.id, 'creator');

    -- Return the business created in the first step
    RETURN new_business;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS create_business_and_add_owner(UUID, TEXT);
