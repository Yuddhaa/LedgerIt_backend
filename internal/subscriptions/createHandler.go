package subscriptions

import (
	"encoding/json"
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
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
	if curPlan.CurrentPlanID.Valid {
		status := curPlan.SubscriptionsStatus.SubscriptionsStatus

		// Check if the subscription is in a state that blocks new creation
		isActiveOrPaused := status != db.SubscriptionsStatusCanceled && status != db.SubscriptionsStatusExpired

		if isActiveOrPaused {
			helpers.LogError("CreateHandler", "Business already has an active link", "businessId", businessId, "status", status)
			// Use 409 Conflict logic
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

	// check if trial is allowed
	if body.IsTrialing {
		trialOrSoloAllowed, err := h.db.CheckUserPlanEligibility(r.Context(), userId)
		if err != nil {
			helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
			helpers.LogError("CreateHandler", "db error in CheckUserPlanEligibility", "err", err.Error())
			return
		}
		if !trialOrSoloAllowed.TrialAvailable {
			helpers.RespondWithError(w, http.StatusForbidden, "Trial is not available")
			helpers.LogInfo("CreateHandler", "Trial is not available", "businessId", businessId)
			return
		}
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
		"is_trialing": body.IsTrialing,
	}

	var startAt *int64
	if body.IsTrialing {
		temp := time.Now().Add(configs.TRIAL_DAYS).Unix()
		startAt = &temp
	}
	res, ok := h.createSubscription(w, r, body, plan, notes, businessId, startAt)
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("CreateHandler", "subscription created", "res", res)
}
