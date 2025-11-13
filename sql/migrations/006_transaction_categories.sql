-- +goose Up
CREATE TABLE transaction_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT unique_business_category_name UNIQUE (business_id, name)
);

CREATE INDEX idx_transaction_categories_business_id ON transaction_categories(business_id);

-- +goose Down
DROP TABLE IF EXISTS transaction_categories;
