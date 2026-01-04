package subscriptions

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/helpers"
)

func (h *Handler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
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
		helpers.RespondWithError(w, http.StatusForbidden, "Only creator can create subscriptions")
		helpers.LogError("UpdateHandler", "Only creator can update subscriptions", "role", role, "userId", userId)
		return
	}
	if role != 1 {
		return
	}

	// ****************************************************************************************************************
	// if creator, decode the body and validate the parameters
	// ****************************************************************************************************************
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request")
		helpers.LogInfo("UpdateHandler", "cannot decode body", "err", err.Error())
		return
	}
	if ok := validateRequestBody(w, body); !ok {
		return // response is sent in the validateRequestBody
	}

	// ****************************************************************************************************************
	// for now this will be hardcoded to true regardless of the actual body input
	// in future if this option needs to be used, can be used
	// ****************************************************************************************************************
	body.IsImmediate = true

	// ****************************************************************************************************************
	// get business current plan
	// ****************************************************************************************************************
	curPlan, ok := h.getCurrentPlan(w, r.Context(), businessId)
	if !ok {
		return
	}
	if !curPlan.SubscriptionID.Valid {
		helpers.RespondWithError(w, http.StatusConflict, `No active Razorpay subscription found.
		Please use /create to upgrade from a Trial.`)
		helpers.LogError("UpdateHandler", "business doesn't have a current plan", "businessId", businessId)
		return
	}
	curBasePlan := strings.Split(curPlan.CurrentPlanID.String, "-")[1]

	// check if the plans is a degrade
	if configs.Plans[curBasePlan].Level > configs.Plans[body.BasePlan].Level {

		addOnInt, err := strconv.Atoi(body.AddOn)
		if err != nil {
			helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:bad add on")
			helpers.LogInfo("UpdateHandler", "bad 'add_on' in body")
			return
		}
		newPlanMembersCount := configs.Plans[body.BasePlan].UsersLimit + addOnInt
		if int(curPlan.MembersCount) > newPlanMembersCount {
			helpers.RespondWithError(w, http.StatusConflict, "Too many active members, can't update the plan")
			helpers.LogError("UpdateHandler", "too many active members to degrade the plan", "newPlanMembersCount",
				newPlanMembersCount, "curPlanMembersCount", curPlan.MembersCount)
			return
		}

		if body.BasePlan == "solo" {
			// update db to solo plan
			h.handleSoloPlan(w, r, body, userId, businessId, true, curPlan)
			return
		}
	}
	// ****************************************************************************************************************
	// if degrade is valid and newplan is not a solo plan,
	// continue to create a new subscription
	// ****************************************************************************************************************
	plan, ok := h.getOrCreatePlan(w, r, body)
	if !ok {
		return
	}
	// ****************************************************************************************************************
	// now that we have the plan, create the subscription
	// ****************************************************************************************************************
	notes := map[string]any{
		"type":                "update", // Tells Webhook: "This is a replacement!"
		"user_id":             userId,
		"business_id":         businessId,
		"old_sub_id":          curPlan.SubscriptionID,
		"old_sub_razorpay_id": curPlan.RazorpaySubscriptionID,
		"is_immediate":        body.IsImmediate,
	}
	var startAt *int64
	if !body.IsImmediate && curPlan.SubscriptionEndPeriod.Valid {
		temp := curPlan.SubscriptionEndPeriod.Time.Unix()
		startAt = &temp
	}
	res, ok := h.createSubscription(w, r, plan, notes, businessId, startAt)
	helpers.RespondWithJSON(w, 201, res)
	helpers.LogInfo("UpdateHandler", "subscription created", "res", res)
}
