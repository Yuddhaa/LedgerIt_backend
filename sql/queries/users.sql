-- name: CreateUser :one
INSERT INTO users (
    google_id,
    email,
    name,
    picture
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- -- name: UpsertUserByEmail :one
-- WITH inserted AS (
--   INSERT INTO users (google_id, email, name)
--   VALUES ($1, $2, $3)
--   ON CONFLICT (email) DO NOTHING
--   RETURNING *
-- )
-- SELECT * FROM inserted
-- UNION ALL
-- SELECT * FROM users WHERE email = $2
-- LIMIT 1;


-- -- name: UpsertUserByEmail :one
-- WITH inserted AS (
--   INSERT INTO users (google_id, email, name,picture)
--   VALUES ($1, $2, $3, $4)
--   ON CONFLICT (email) DO NOTHING
--   RETURNING id
-- )
-- SELECT u.*
-- FROM users AS u
-- WHERE u.id IN (
--   SELECT i.id FROM inserted AS i
--   UNION
--   SELECT u2.id FROM users AS u2 WHERE u2.email = $2
-- )
-- LIMIT 1;

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


