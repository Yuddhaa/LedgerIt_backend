-- +goose Up
-- +goose StatementBegin
-- Rename the 'expense_categories' table
ALTER TABLE expense_categories RENAME TO transaction_categories;
-- Rename its index
ALTER INDEX idx_expense_categories_business_id RENAME TO idx_transaction_categories_business_id;
-- Rename its unique constraint
ALTER TABLE transaction_categories RENAME CONSTRAINT unique_business_category_name TO unique_business_transaction_category_name;

-- Rename the 'expense_edit_requests' table
ALTER TABLE expense_edit_requests RENAME TO transaction_edit_requests;
-- Rename the 'expense_id' column to 'transaction_id'
ALTER TABLE transaction_edit_requests RENAME COLUMN expense_id TO transaction_id;
-- Rename its indexes
ALTER INDEX idx_expense_edit_requests_expense_id RENAME TO idx_transaction_edit_requests_transaction_id;
ALTER INDEX idx_expense_edit_requests_status RENAME TO idx_transaction_edit_requests_status;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert 'transaction_edit_requests'
ALTER TABLE transaction_edit_requests RENAME TO expense_edit_requests;
ALTER TABLE expense_edit_requests RENAME COLUMN transaction_id TO expense_id;
ALTER INDEX idx_transaction_edit_requests_transaction_id RENAME TO idx_expense_edit_requests_expense_id;
ALTER INDEX idx_transaction_edit_requests_status RENAME TO idx_expense_edit_requests_status;

-- Revert 'transaction_categories'
ALTER TABLE transaction_categories RENAME TO expense_categories;
ALTER INDEX idx_transaction_categories_business_id RENAME TO idx_expense_categories_business_id;
ALTER TABLE expense_categories RENAME CONSTRAINT unique_business_transaction_category_name TO unique_business_category_name;
-- +goose StatementEnd
