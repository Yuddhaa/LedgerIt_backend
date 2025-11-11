-- name: UpsertUserByEmail :one
WITH inserted AS (
  INSERT INTO users (google_id, email, name, picture)
  VALUES ($1, $2, $3, $4)
  ON CONFLICT (email) DO NOTHING
  RETURNING * -- Key change 1: Return the *whole user row*, not just the id
)
SELECT * FROM inserted -- Key change 2: Select the new user from the CTE
UNION
SELECT * FROM users WHERE email = $2 -- Select the existing user if the CTE is empty
LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1;

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


