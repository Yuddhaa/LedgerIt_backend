-- Creates a new business with a specified name and owner ID, returning the new record.
-- name: CreateBusiness :one
INSERT INTO businesses (
    name,
    owner_id
) VALUES (
    $1, $2
)
RETURNING *;

-- Adds a user to a business with a specific role, returning the new membership record.
-- name: AddBusinessMember :one
INSERT INTO business_members (
    user_id,
    business_id,
    role
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- Retrieves a single business record by its unique ID.
-- name: GetBusinessByID :one
SELECT * FROM businesses
WHERE id = $1;

-- Retrieves all businesses a user is a member of, including their role and balance in each.
-- name: GetBusinessesByUserID :many
SELECT 
    b.*, 
    bm.role, 
    bm.current_balance
FROM 
    businesses b
JOIN 
    business_members bm ON b.id = bm.business_id
WHERE 
    bm.user_id = $1;

-- Retrieves all businesses owned by a specific user ID.
-- name: GetBusinessesByOwnerID :many
SELECT * FROM businesses
WHERE owner_id = $1;
