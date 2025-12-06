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

-- GetTransactionApprovals returns approvals list based on filters
-- name: GetTransactionApprovals :many
SELECT 
    ter.id,
    ter.transaction_id,
    ter.type,
    ter.status,
    ter.requested_changes, -- Cast to text for Go
    ter.reason,
    ter.created_at,
    ter.updated_at,
    -- Requestor Details
    req_u.id AS requested_by_id,
    req_u.name AS requested_by_name,
    -- Reviewer Details
    rev_u.id AS reviewed_by_id,
    rev_u.name AS reviewed_by_name,
    -- NEW: Extracted Names from JSONB IDs
    p.name AS party_name,
    c.name AS category_name
FROM transaction_edit_requests ter
JOIN transactions t ON ter.transaction_id = t.id
JOIN users req_u ON ter.requested_by_id = req_u.id
LEFT JOIN users rev_u ON ter.reviewed_by_id = rev_u.id
-- Join Party: Extract ID from JSON -> Handle Empty String -> Cast to UUID -> Join
LEFT JOIN parties p ON p.id = NULLIF(ter.requested_changes->>'party_id', '')::uuid
-- Join Category: Extract ID from JSON -> Handle Empty String -> Cast to UUID -> Join
LEFT JOIN transaction_categories c ON c.id = NULLIF(ter.requested_changes->>'category_id', '')::uuid
WHERE 
    t.business_id = @business_id
    AND (
        @requested_by_ids::UUID[] IS NULL 
        OR ter.requested_by_id = ANY(@requested_by_ids::UUID[])
    )
    AND (
        @status::text = '' OR ter.status::text = @status
    )
    AND (
        @type::text = '' OR ter.type::text = @type
    )
    AND (
        @from_date::TIMESTAMPTZ IS NULL 
        OR ter.created_at >= @from_date
    )
    AND (
        @to_date::TIMESTAMPTZ IS NULL 
        OR ter.created_at <= @to_date
    )
ORDER BY ter.created_at DESC;
