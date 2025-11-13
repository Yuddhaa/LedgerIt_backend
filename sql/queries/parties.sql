-- name: CreateParty :one
INSERT INTO parties (
    business_id,
    name,
    type,
    phone_number
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetParty :one
SELECT * FROM parties
WHERE id = $1 AND business_id = $2;

-- name: ListParties :many
SELECT * FROM parties
WHERE business_id = $1
ORDER BY name;

-- name: ListPartiesByType :many
SELECT * FROM parties
WHERE business_id = $1 AND type = $2
ORDER BY name;

-- name: UpdateParty :one
UPDATE parties
SET
    name = $2,
    type = $3,
    phone_number = $4
WHERE id = $1 AND business_id = $5
RETURNING *;

-- name: DeleteParty :exec
DELETE FROM parties
WHERE id = $1 AND business_id = $2;
