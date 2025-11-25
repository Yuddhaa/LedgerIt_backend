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
