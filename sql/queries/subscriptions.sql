-- name: CheckUserPlanEligibility :one
SELECT 
    -- Logic: If they have 0 'solo' businesses, Free is available.
    (SELECT COUNT(*) = 0 FROM businesses WHERE owner_id = $1 AND current_plan_id = 'permanent-solo-0') AS free_available,
    
    -- Logic: Inverse of the boolean flag in the users table.
    COALESCE(NOT is_trial_used, false)::boolean AS trial_available
FROM users
WHERE id = $1;

-- GetBusinessCurrentPlan gets the details on the current plan
-- name: GetBusinessCurrentPlan :one
SELECT 
    b.current_plan_id,
    p.name AS plan_name,
    p.amount,
    p.currency,
    b.subscriptions_status,
    b.subscription_end_period,
    b.is_trial_used,
    b.is_offer_used,
    b.offer_code,
    
    -- Fetch directly from the joined subscription table
    s.id AS subscription_id,
    s.razorpay_subscription_id,
    s.marketer_id,    
    (
        SELECT COUNT(*)::INT 
        FROM business_members bm 
        WHERE bm.business_id = b.id
    ) AS members_count
FROM businesses b
LEFT JOIN plans p ON b.current_plan_id = p.id
LEFT JOIN subscriptions s ON b.current_subscription_id = s.id
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
-- name: UpdateBusinessSubscription :one
UPDATE businesses
SET 
    current_plan_id = $2,
    subscriptions_status = $3,
    subscription_end_period = $4,
    -- If 'is_trial_used' is already true, keep it true.
    -- If input is true, make it true.
    -- Only if BOTH are false does it stay false.
    is_trial_used = (is_trial_used OR COALESCE(sqlc.narg('is_trial_used')::boolean, false)),
    is_offer_used = (is_offer_used OR COALESCE(sqlc.narg('is_offer_used')::boolean, false)),
    current_subscription_id = $5,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- Create the missing Invoice Query (we need this for the charged event)
-- name: CreateInvoice :one
INSERT INTO subscription_invoices (
  subscription_id, business_id, razorpay_payment_id, amount_paid, currency, status
) VALUES (
  $1, $2, $3, $4, $5, $6
)
ON CONFLICT DO NOTHING 
RETURNING id;

-- Query to find subscription by Razorpay ID (Critical for Webhooks)
-- name: GetSubscriptionByRazorpayID :one
SELECT * FROM subscriptions WHERE razorpay_subscription_id = $1 LIMIT 1;

-- name: UpdateSubscriptionAndBusiness :exec
WITH updated_sub AS (
    UPDATE subscriptions
    SET 
        status = $2,
        current_period_start = $3,
        current_period_end = $4,
        updated_at = now()
    WHERE razorpay_subscription_id = $1
    RETURNING id, business_id, status, current_period_end, plan_id, is_offer_applied
)
UPDATE businesses
SET 
    current_plan_id = updated_sub.plan_id,
    subscriptions_status = updated_sub.status,
    subscription_end_period = updated_sub.current_period_end,
    current_subscription_id = updated_sub.id,
    
    -- Trial Logic
    is_trial_used = (is_trial_used OR COALESCE(sqlc.narg('is_trial_used')::boolean, false)),
    
    -- Offer Logic
    is_offer_used = (is_offer_used OR updated_sub.is_offer_applied),
    
    -- Use narg. If input is NULL, COALESCE keeps the old value.
    offer_code = COALESCE(sqlc.narg('offer_code')::text, offer_code)

FROM updated_sub
WHERE businesses.id = updated_sub.business_id;
-- name: UpdateStatusIfPending :exec
WITH updated_sub AS (
    UPDATE subscriptions
    SET 
        status = $2,
        -- Use sqlc.narg to handle optional End Date updates (for Trials)
        current_period_end = COALESCE(sqlc.narg('current_period_end')::TIMESTAMPTZ, current_period_end),
        updated_at = now()
    WHERE razorpay_subscription_id = $1
      -- ✅ ATOMIC GUARD: Only allow update if we are in "start" states
      AND status::text IN ('inactive', 'pending', 'trialing_pending', 'authenticated')
    RETURNING id, business_id, status, current_period_end,plan_id
)
UPDATE businesses
SET 
    subscriptions_status = updated_sub.status,
    subscription_end_period = updated_sub.current_period_end,
    current_plan_id = updated_sub.plan_id,
    current_subscription_id = updated_sub.id,
    is_trial_used = (is_trial_used OR COALESCE(sqlc.narg('is_trial_used')::boolean, false))
FROM updated_sub
WHERE businesses.id = updated_sub.business_id;

-- name: UpdateSubscriptionStatusRaw :exec
-- Used for simple state changes like Paused, Resumed, Pending, Halted
WITH updated_sub AS (
    UPDATE subscriptions
    SET 
        status = $2,
        updated_at = now()
    WHERE razorpay_subscription_id = $1
    RETURNING id, business_id, status
)
UPDATE businesses
SET 
    subscriptions_status = updated_sub.status
FROM updated_sub
WHERE businesses.id = updated_sub.business_id;

-- name: CancelSubscription :exec
WITH updated_sub AS (
    UPDATE subscriptions
    SET 
        status = 'canceled',
        current_period_end = now(), -- Cut off access immediately (or keep date if you prefer)
        updated_at = now()
    WHERE razorpay_subscription_id = $1
    RETURNING id, business_id
)
UPDATE businesses
SET 
    subscriptions_status = 'canceled',
    current_subscription_id = NULL -- Unlink the cancelled sub
FROM updated_sub
WHERE businesses.id = updated_sub.business_id
  -- SAFETY: Only cancel business status if this was the CURRENT active subscription
  AND businesses.current_subscription_id = updated_sub.id;

-- name: CancelTrial :exec
UPDATE businesses
SET 
    subscriptions_status = 'canceled', -- Or 'trial_ended' if you prefer that distinction
    subscription_end_period = now(),   -- Cut off access immediately
    updated_at = now()
WHERE id = $1 
  AND (subscriptions_status = 'trialing' OR subscriptions_status = 'trial_ended');

-- name: GetSubscriptionStatus :one
SELECT status FROM subscriptions WHERE business_id = $1 AND id = $2;



-- ********************************************************************************************************************
-- offers related
-- ********************************************************************************************************************
-- name: GetOfferDetailsByCode :one
SELECT 
    id, 
    commission_percent,
    discount_percent,
    CASE 
        WHEN offer_code_upi  = sqlc.arg(code) THEN razorpay_offer_id_upi
        WHEN offer_code_card = sqlc.arg(code) THEN razorpay_offer_id_card
        WHEN offer_code_life = sqlc.arg(code) THEN razorpay_offer_id_life
    END::text AS razorpay_offer_id
FROM marketers
WHERE offer_code_upi = sqlc.arg(code) 
   OR offer_code_card = sqlc.arg(code) 
   OR offer_code_life = sqlc.arg(code);

-- name: AddMarketerCommission :exec
UPDATE marketers
SET 
    commission_balance = commission_balance + sqlc.arg(amount),
    total_commission = total_commission + sqlc.arg(amount),
    updated_at = now()
WHERE id = $1;

-- name: GetMarketerById :one
SELECT * FROM marketers WHERE id = $1;

-- while developing, subscription cleanup query
-- name: DeleteSubscriptionInvoiceRows :exec
DELETE FROM subscription_invoices WHERE business_id = $1;

-- name: DeleteSubscriptionRows :exec
DELETE FROM subscriptions WHERE business_id = $1;
