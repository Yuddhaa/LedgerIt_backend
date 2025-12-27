-- CheckUserPlanEligibility checks if the user has free businesses
-- or trial period available
-- name: CheckUserPlanEligibility :one
SELECT 
    -- Logic: If count of 'free' plans is 0, then Free IS available.
    (COUNT(*) FILTER (WHERE current_plan_id = 'free') = 0)::BOOLEAN AS free_available,
    
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
