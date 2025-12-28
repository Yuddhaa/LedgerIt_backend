-- CheckUserPlanEligibility checks if the user has free businesses
-- or trial period available
-- name: CheckUserPlanEligibility :one
SELECT 
    -- Logic: If count of 'free' plans is 0, then Free IS available.
    (COUNT(*) FILTER (WHERE current_plan_id = 'permanent-solo-0') = 0)::BOOLEAN AS free_available,
    
    -- Logic: If count of businesses that used a trial is 0, then Trial IS available.
    (COUNT(*) FILTER (WHERE is_trial_used = true) = 0)::BOOLEAN AS trial_available
FROM businesses
WHERE owner_id = $1;

-- GetBusinessCurrentPlan gets the details on the current plan
-- name: GetBusinessCurrentPlan :one
SELECT 
    b.current_plan_id,
    p.name AS plan_name,
    p.amount,
    p.currency,
    b.subscriptions_status,
    b.subscription_end_date,
    (
        SELECT COUNT(*)::INT 
        FROM business_members bm 
        WHERE bm.business_id = b.id
    ) AS members_count
FROM businesses b
LEFT JOIN plans p ON b.current_plan_id = p.id
WHERE b.id = $1;

-- GetPlan query gets plan based on the planId
-- name: GetPlan :one
SELECT * FROM plans WHERE id = $1;


-- CreatePlan adds a new plan to plans table
-- name: CreatePlan :one
INSERT INTO plans (
  id,
  razorpay_plan_id,
  name,
  description,
  amount,
  currency,
  user_limit,
  period,
  active
) VALUES (
  sqlc.arg(id),
  sqlc.arg(razorpay_plan_id),
  sqlc.arg(name),
  sqlc.arg(description),
  sqlc.arg(amount),
  sqlc.arg(currency),
  sqlc.arg(user_limit),
  sqlc.arg(period),
  sqlc.arg(active)
)
RETURNING *;

-- CreateSubscription creates new row in subscriptions table
-- name: CreateSubscription :one
INSERT INTO subscriptions (
    business_id,
    plan_id,
    razorpay_subscription_id,
    status,
    marketer_id,
    is_offer_applied
) VALUES (
    $1, $2, $3,$4, $5, $6
) RETURNING *;

-- UpdateBusinessSubscription updates subscriptions related columns 
-- name: UpdateBusinessSubscription :exec
UPDATE businesses
SET 
    current_plan_id = $2,
    subscriptions_status = $3,
    subscription_end_date = $4,
    -- If $5 is NULL, it keeps the existing 'is_trial_used' value
    is_trial_used = COALESCE($5, is_trial_used),
    updated_at = now()
WHERE id = $1;
