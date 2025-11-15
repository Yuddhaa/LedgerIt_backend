-- name: CreateCategory :one
-- add a new category
INSERT INTO transaction_categories (
  business_id,
  name
) VALUES (
  $1, $2
)
RETURNING *;

-- name: UpdateCategory :one
-- update a particular category
UPDATE transaction_categories
SET
  name = $3,
  updated_at = now()
WHERE
  id = $1 AND business_id = $2
RETURNING *;

-- name: DeleteCategory :one
-- delete a particular category
DELETE FROM transaction_categories
WHERE id = $1 AND business_id = $2
RETURNING id;

-- name: GetCategory :one
-- get a particular category
SELECT * FROM transaction_categories
WHERE id = $1 AND business_id = $2;

-- name: ListCategoriesByBusiness :many
-- get all category that belongs to a business_id
SELECT * FROM transaction_categories
WHERE business_id = $1
ORDER BY name ASC;
