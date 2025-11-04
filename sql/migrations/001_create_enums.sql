-- +goose Up
-- +goose StatementBegin
CREATE TYPE business_role AS ENUM ('creator', 'admin', 'employee');
CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'rejected');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TYPE business_role;
DROP TYPE approval_status;
-- +goose StatementEnd
