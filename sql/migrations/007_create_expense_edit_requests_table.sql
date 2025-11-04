-- +goose Up
-- +goose StatementBegin
CREATE TABLE expense_edit_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    requested_by_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reviewed_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    status approval_status NOT NULL DEFAULT 'pending',
    -- JSONB is efficient for storing the proposed changes
    requested_changes JSONB NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_expense_edit_requests_expense_id ON expense_edit_requests(expense_id);
CREATE INDEX idx_expense_edit_requests_status ON expense_edit_requests(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE expense_edit_requests;
-- +goose StatementEnd
