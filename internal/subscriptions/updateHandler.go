package subscriptions

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role != 1 {
		return
	}
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

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
	// get business current plan
	// ****************************************************************************************************************
	curPlan, err := h.db.GetBusinessCurrentPlan(r.Context(), businessId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Business not found")
			helpers.LogInfo("UpdateHandler", "Business not found", "businessId", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("UpdateHandler", "db error fetching plan", "err", err.Error())
		return
	}
	if !curPlan.CurrentPlanID.Valid {
		helpers.RespondWithError(w, http.StatusConflict, "business doesn't have a current plan")
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
	}
	var startAt *int64
	if !body.IsImmediate && curPlan.SubscriptionEndPeriod.Valid {
		temp := curPlan.SubscriptionEndPeriod.Time.Unix()
		startAt = &temp
	}
	res, ok := h.createSubscription(w, r, body, plan, notes, businessId, startAt)
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("UpdateHandler", "subscription created", "res", res)
}
