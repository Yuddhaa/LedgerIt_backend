package subscriptions

import (
	"encoding/json"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
)

// CreateHandler creates a plan(if not exists) and
// creates+returns corresponding subscriptions
func (h *Handler) CreateHandler(w http.ResponseWriter, r *http.Request) {
	// ****************************************************************************************************************
	// get loggedin userInfo
	// ****************************************************************************************************************
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// if not creator -> get out
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role != 1 {
		helpers.RespondWithError(w, http.StatusForbidden, "Only creator can create subscriptions")
		helpers.LogError("CreateHandler", "Only creator can create subscriptions", "role", role, "userId", userId)
		return
	}

	// ****************************************************************************************************************
	// if creator, decode the body and validate the parameters
	// ****************************************************************************************************************
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request")
		helpers.LogInfo("CreateHandler", "cannot decode body", "err", err.Error())
		return
	}
	if ok := validateRequestBody(w, body); !ok {
		return // response is sent in the validateRequestBody
	}

	// ****************************************************************************************************************
	// check if a subscription already exists, if so, use /update
	// ****************************************************************************************************************
	curPlan, ok := h.getCurrentPlan(w, r.Context(), businessId)
	if !ok {
		return
	}
	// Logic: If they have a plan ID, and the status implies it's still "alive" (not canceled/expired), block them.
	if curPlan.SubscriptionID.Valid {
		status := curPlan.SubscriptionsStatus

		// Logic: We want to BLOCK if the status is "Alive".
		// "Alive" means it is NOT canceled AND it is NOT past_due.
		isAlive := status != db.SubscriptionsStatusCanceled && status != db.SubscriptionsStatusPastDue

		// If it IS alive, we throw the error.
		if isAlive {
			helpers.LogError("CreateHandler", "Business already has an active link", "businessId", businessId, "status", status)
			helpers.RespondWithError(w, http.StatusConflict, "Subscription already exists. Please use /update to change your plan.")
			return
		}
	}
	// ****************************************************************************************************************
	// check if either solo or trial available before moving forward if thats what the user has clicked
	// ****************************************************************************************************************
	if body.BasePlan == "solo" {
		h.handleSoloPlan(w, r, body, userId, businessId, false, db.GetBusinessCurrentPlanRow{})
		return
	}

	// ****************************************************************************************************************
	// get or create plan
	// ****************************************************************************************************************
	plan, ok := h.getOrCreatePlan(w, r, body)
	if !ok {
		return
	}
	// ****************************************************************************************************************
	// now that we have the plan, create the subscription
	// ****************************************************************************************************************
	notes := map[string]any{
		"type":        "fresh", // Tells Webhook: "Don't look for old subs to cancel"
		"user_id":     userId,
		"business_id": businessId,
	}

	// Start Immediately (startAt = nil)
	res, ok := h.createSubscription(w, r, plan, notes, businessId, nil)
	helpers.RespondWithJSON(w, 201, res)
	helpers.LogInfo("CreateHandler", "subscription created", "res", res)
}
