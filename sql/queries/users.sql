-- name: CreateUser :one
INSERT INTO users (
    google_id,
    email,
    name
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByPhone :one
SELECT * FROM users
WHERE phone_number = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC;

-- name: UpdateUserProfile :one
UPDATE users
SET 
    name = $2,
    phone_number = $3,
    updated_at = now()
WHERE 
    id = $1
RETURNING *;
