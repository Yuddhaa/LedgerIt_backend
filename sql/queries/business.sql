-- name: CreateBusinessAndAddOwner :one
SELECT * FROM create_business_and_add_owner(
    p_owner_id := $1,
    p_name := $2
);

-- name: UpdateBusiness :one
UPDATE businesses
SET
  name = $2,
  updated_at = NOW()
WHERE
  id = $1
RETURNING *;

-- Based on userid and businessid it will return role
-- name: GetMemberRole :one
Select bm.role from business_members bm WHERE bm.user_id = $1 AND bm.business_id = $2;

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
SELECT b.*,bm.user_id,bm.role,bm.current_balance 
FROM businesses b
JOIN
    business_members bm ON b.id = bm.business_id
WHERE bm.user_id = $1 and b.id = $2;

-- Retrieves all businesses a user is a member of, including their role and balance in each.
-- name: GetBusinessesByUserID :many
SELECT 
    b.*
    -- bm.role, 
    -- bm.current_balance
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

-- GetBusinessMembers returns all the particular business members
-- name: GetBusinessMembers :many
SELECT u.id, u.name, u.email, u.phone_number, bm.role, bm.current_balance
FROM users u
JOIN business_members bm ON bm.user_id = u.id
WHERE bm.business_id = $1
ORDER BY u.name;

-- returns 1 if a user is a member of the given businessId
-- name: IsUserMemberOfBusiness :one
SELECT 1
FROM business_members
WHERE user_id = $1 AND business_id = $2;

-- returns 1 if a user is admin or creator of a given business_id
-- name: CheckAdmin :one
SELECT 1
FROM business_members
WHERE user_id = $1 AND business_id = $2 AND role in ('admin', 'creator');


-- returns the role of the user
-- name: GetUserRole :one
SELECT role FROM business_members
WHERE user_id = $1 AND business_id = $2;

-- used to update balance in transactions
-- name: UpdateBusinessMemberBalance :exec
UPDATE business_members
SET current_balance = current_balance + $1
WHERE user_id = $2 AND business_id = $3;

-- used to update role in business_members
-- name: UpdateBusinessMember :one
UPDATE business_members
SET role = $1
WHERE user_id = $2 AND business_id = $3
RETURNING *;

-- used to delete a business
-- name: DeleteBusiness :exec
DELETE FROM businesses WHERE id = $1;

-- used to remove business member
-- name: DeleteBusinessMember :exec
DELETE FROM business_members WHERE user_id = $1 AND business_id = $2;

-- used to get a member details
-- name: GetMemberRoleBalance :one
Select role, current_balance FROM business_members WHERE user_id = $1 AND business_id = $2;
