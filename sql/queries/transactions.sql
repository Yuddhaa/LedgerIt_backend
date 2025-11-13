-- name: CreateTransaction :one
INSERT INTO transactions (
    business_id,
    amount,
    direction,
    description,
    party_id,
    mode,
    category_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetTransaction :one
SELECT * FROM transactions
WHERE id = $1 AND business_id = $2;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = $1 AND business_id = $2;

-- name: UpdateTransaction :one
UPDATE transactions
SET
    amount = $2,
    direction = $3,
    description = $4,
    party_id = $5,
    mode = $6,
    category_id = $7,
    updated_at = now()
WHERE id = $1 AND business_id = $8
RETURNING *;

-- name: ListTransactions :many
-- This query joins parties and categories to get their names
SELECT
    t.*,
    p.name AS party_name,
    tc.name AS category_name
FROM
    transactions t
LEFT JOIN
    parties p ON t.party_id = p.id
LEFT JOIN
    transaction_categories tc ON t.category_id = tc.id
WHERE
    t.business_id = $1
ORDER BY
    t.created_at DESC
LIMIT $2
OFFSET $3;

-- name: GetTransactionRich :one
-- Gets a single transaction with party and category names
SELECT
    t.*,
    p.name AS party_name,
    tc.name AS category_name
FROM
    transactions t
LEFT JOIN
    parties p ON t.party_id = p.id
LEFT JOIN
    transaction_categories tc ON t.category_id = tc.id
WHERE
    t.id = $1 AND t.business_id = $2;

-- --- Transaction Edit Requests ---

-- name: CreateTransactionEditRequest :one
INSERT INTO transaction_edit_requests (
    transaction_id,
    requested_by_id,
    requested_changes,
    reason
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetTransactionEditRequest :one
SELECT * FROM transaction_edit_requests
WHERE id = $1;

-- name: ListEditRequestsForTransaction :many
SELECT * FROM transaction_edit_requests
WHERE transaction_id = $1
ORDER BY created_at DESC;

-- name: ListPendingEditRequestsForBusiness :many
SELECT ter.*
FROM transaction_edit_requests ter
JOIN transactions t ON ter.transaction_id = t.id
WHERE
    t.business_id = $1
    AND ter.status = 'pending'
ORDER BY
    ter.created_at ASC;

-- name: UpdateEditRequestStatus :one
UPDATE transaction_edit_requests
SET
    status = $2,
    reviewed_by_id = $3,
    updated_at = now()
WHERE
    id = $1
RETURNING *;
