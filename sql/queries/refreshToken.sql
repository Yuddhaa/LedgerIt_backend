-- name: InsertRefreshToken :one
-- InsertRefreshToken inserts a new refresh token into the database.
INSERT INTO refresh_tokens (
    user_id,
    token_hash,
    expires_at,
    device_info
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetRefreshTokenByHash :one
-- GetRefreshTokenByHash finds a valid (non-expired) refresh token by its hash.
-- This is used during the /auth/refresh flow.
SELECT id, user_id, expires_at, device_info, created_at
FROM refresh_tokens
WHERE token_hash = $1 AND expires_at > NOW();

-- name: DeleteRefreshTokenByHash :exec
-- DeleteRefreshTokenByHash deletes a single refresh token by its hash.
-- This is used for logging out a single device.
DELETE FROM refresh_tokens
WHERE token_hash = $1;

-- name: DeleteRefreshTokensByUserID :exec
-- DeleteRefreshTokensByUserID deletes all refresh tokens for a specific user.
-- This is used for the "log out from all devices" feature.
DELETE FROM refresh_tokens
WHERE user_id = $1;
