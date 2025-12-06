-- name: VerifyPartyBelongsToBusiness :one
SELECT EXISTS (
    SELECT 1 FROM parties 
    WHERE id = $1 AND business_id = $2
);

-- name: VerifyCategoryBelongsToBusiness :one
SELECT EXISTS (
    SELECT 1 FROM transaction_categories 
    WHERE id = $1 AND business_id = $2
);

-- name: CreateTransactionWithValidation :one
WITH validation AS (
    SELECT 
        -- Check 1: Party must exist in this business
        EXISTS(
            SELECT 1 FROM parties 
            WHERE id = sqlc.arg(party_id) AND business_id = sqlc.arg(business_id)
        ) AS party_valid,

        -- Check 2: Category must exist in this business (OR be NULL)
        (
            sqlc.narg(category_id)::uuid IS NULL OR 
            EXISTS(
                SELECT 1 FROM transaction_categories 
                WHERE id = sqlc.narg(category_id) AND business_id = sqlc.arg(business_id)
            )
        ) AS category_valid
)
INSERT INTO transactions (
    business_id, 
    user_id, 
    party_id, 
    category_id, 
    amount, 
    direction, 
    mode, 
    receipt_no, 
    description
)
SELECT 
    sqlc.arg(business_id), 
    sqlc.arg(user_id), 
    sqlc.arg(party_id), 
    sqlc.narg(category_id), 
    sqlc.arg(amount), 
    sqlc.arg(direction), 
    sqlc.arg(mode), 
    sqlc.arg(receipt_no), 
    sqlc.narg(description)
FROM validation
WHERE party_valid = true AND category_valid = true
    RETURNING *;
-- RETURNING id, created_at;

-- name: GetTransactionForUpdate :one
SELECT * FROM transactions 
WHERE id = $1 AND business_id = $2 
FOR NO KEY UPDATE;

-- name: UpdateTransaction :one
UPDATE transactions
SET
    amount = $3,
    direction = $4,
    category_id = $5,
    party_id = $6,
    mode = $7,
    receipt_no = $8,
    description = $9,
    updated_at = NOW()
WHERE id = $1 AND business_id = $2
RETURNING *;

-- name: CreateEditRequest :one
INSERT INTO transaction_edit_requests (
    transaction_id,
    requested_by_id,
    type,
    requested_changes,
    reason,
    status
) VALUES (
    $1, 
    $2, 
    'edit', 
    sqlc.arg(requested_changes)::text::jsonb, -- <--- THE FIX
    $3, 
    'pending'
)
RETURNING id, transaction_id, requested_by_id, reviewed_by_id, status, type, requested_changes, reason, created_at, updated_at;
