-- +goose Up
CREATE TABLE deposits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    depositer_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount DECIMAL(10, 2) NOT NULL,
    remarks TEXT,
    status deposit_status NOT NULL DEFAULT 'pending',
    reviewer_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deposits_business_id ON deposits(business_id);
CREATE INDEX idx_deposits_depositer_id ON deposits(depositer_id);
CREATE INDEX idx_deposits_reviewer_id ON deposits(reviewer_id);
CREATE INDEX idx_deposits_status ON deposits(status);

-- +goose Down
DROP TABLE IF EXISTS deposits;
