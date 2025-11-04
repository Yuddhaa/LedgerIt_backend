-- +goose Up
-- +goose StatementBegin
CREATE TABLE expense_categories (
    id SERIAL PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    
    -- A category name should be unique within a business
    CONSTRAINT unique_business_category_name UNIQUE (business_id, name)
);

CREATE INDEX idx_expense_categories_business_id ON expense_categories(business_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE expense_categories;
-- +goose StatementEnd
