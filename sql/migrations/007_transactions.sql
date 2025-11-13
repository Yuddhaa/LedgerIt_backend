-- +goose Up
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT, -- User who entered it
    amount DECIMAL(10, 2) NOT NULL,
    direction transaction_direction NOT NULL,
    category_id UUID REFERENCES transaction_categories(id) ON DELETE SET NULL,
    party_id UUID REFERENCES parties(id) ON DELETE SET NULL,
    mode transaction_mode NOT NULL,
    receipt_no TEXT,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_business_id ON transactions(business_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_category_id ON transactions(category_id);
CREATE INDEX idx_transactions_party_id ON transactions(party_id);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS transactions;
