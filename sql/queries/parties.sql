-- name: CreateParty :one
-- add a party
INSERT INTO parties (
  name,
  place,
  phone_number,
  business_id
) VALUES (
  $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateParty :one
-- update a particular party
UPDATE parties
SET
  name = $2,
  place = $3,
  phone_number = $4,
  updated_at = now()
WHERE
  id = $1 AND business_id = $5
RETURNING *;

-- name: GetParty :one
-- get a particular party
SELECT * FROM parties
WHERE id = $1 AND business_id = $2;

-- name: DeleteParty :one
-- delete a particular party
DELETE FROM parties
WHERE id = $1 AND business_id = $2
RETURNING id;

-- name: ListPartiesByBusiness :many
-- get all parties realated a particualr businessId
SELECT * FROM parties
WHERE business_id = $1
ORDER BY created_at DESC;

-- name: ListPartiesByBusinessAndPlace :many
-- get all parties realted a particular businessId and place
SELECT * FROM parties
WHERE business_id = $1 AND place = $2
ORDER BY created_at DESC;

-- name: ListUniquePartyPlacesByBusiness :many
-- get all the unique places stored in parties table related to a businessId
SELECT DISTINCT place
FROM parties
WHERE business_id = $1
ORDER BY place ASC;

-- name: CheckMember :one
-- returns 1 if a user is a member of a given business_id
SELECT 1
FROM business_members
WHERE user_id = $1 AND business_id = $2
LIMIT 1;
