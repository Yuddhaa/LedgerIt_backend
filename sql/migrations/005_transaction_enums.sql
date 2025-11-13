-- +goose Up
CREATE TYPE transaction_direction AS ENUM ('in', 'out');
CREATE TYPE transaction_mode AS ENUM ('online', 'cash', 'cheque');
CREATE TYPE edit_request_status AS ENUM ('pending', 'approved', 'rejected');
CREATE TYPE deposit_status AS ENUM ('pending', 'approved', 'rejected');

-- +goose Down
DROP TYPE IF EXISTS transaction_direction;
DROP TYPE IF EXISTS transaction_mode;
DROP TYPE IF EXISTS edit_request_status;
DROP TYPE IF EXISTS deposit_status;
