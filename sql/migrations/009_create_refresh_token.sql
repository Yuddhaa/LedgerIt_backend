-- +goose Up
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- The foreign key to your existing users table
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- We will store a HASH of the token, not the token itself.
    -- It must be unique to prevent duplicates.
    token_hash TEXT NOT NULL UNIQUE,
    
    -- When this token is no longer valid
    expires_at TIMESTAMPTZ NOT NULL,

    -- Optional: Info to show the user in a "my devices" list
    -- e.g., "Pixel 8 Pro" or the request User-Agent
    device_info TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- We will look up tokens by user_id to implement "log out all"
-- and to show a user their list of devices.
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- +goose Down
DROP TABLE refresh_tokens;
