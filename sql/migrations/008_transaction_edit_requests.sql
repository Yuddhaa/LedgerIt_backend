-- +goose Up
CREATE TABLE transaction_edit_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    requested_by_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reviewed_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    status edit_request_status NOT NULL DEFAULT 'pending',
    requested_changes JSONB NOT NULL, -- JSONB is preferred over JSON
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transaction_edit_requests_transaction_id ON transaction_edit_requests(transaction_id);
CREATE INDEX idx_transaction_edit_requests_requested_by_id ON transaction_edit_requests(requested_by_id);
CREATE INDEX idx_transaction_edit_requests_reviewed_by_id ON transaction_edit_requests(reviewed_by_id);
CREATE INDEX idx_transaction_edit_requests_status ON transaction_edit_requests(status);

-- +goose Down
DROP TABLE IF EXISTS transaction_edit_requests;
