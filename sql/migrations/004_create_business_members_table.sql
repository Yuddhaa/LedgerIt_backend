-- +goose Up
-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE business_members;
-- +goose StatementEnd
