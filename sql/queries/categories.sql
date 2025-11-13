-- name: CreateTransactionCategory :one
INSERT INTO transaction_categories (
    business_id,
    name
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetTransactionCategory :one
SELECT * FROM transaction_categories
WHERE id = $1 AND business_id = $2;

-- name: ListTransactionCategories :many
SELECT * FROM transaction_categories
WHERE business_id = $1
ORDER BY name;

-- name: UpdateTransactionCategory :one
UPDATE transaction_categories
SET name = $2
WHERE id = $1 AND business_id = $3
RETURNING *;

-- name: DeleteTransactionCategory :exec
DELETE FROM transaction_categories
WHERE id = $1 AND business_id = $2;
