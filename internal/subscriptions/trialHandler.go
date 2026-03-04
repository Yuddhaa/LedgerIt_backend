package subscriptions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// trialHandler sets the trialing for the business
func (h *Handler) trialHandler(w http.ResponseWriter, r *http.Request) {
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request")
		helpers.LogError("trialHandler", "bad body", "err", err.Error())
		return
	}
	// ****************************************************************************************************************
	// get loggedin userInfo
	// ****************************************************************************************************************
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role != 1 {
		helpers.RespondWithError(w, http.StatusUnauthorized, "only creator can opt trial")
		helpers.LogError("trialHandler", "only creator can opt trial")
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
	planId := fmt.Sprintf("%s-%s-%s", body.Period, body.BasePlan, body.AddOn)
	if !validateRequestBody(w, body) {
		return
	}
	if planId == configs.FREE_PLAN_ID {
		helpers.LogInfo("trialHandler", "trial is not allowed for solo", "businessId", businessId)
		helpers.RespondWithError(w, http.StatusForbidden, "trial is not allowed for "+configs.FREE_PLAN_ID)
		return
	}

	// ****************************************************************************************************************
	// check if a plan already exists
	// ****************************************************************************************************************
	curPlan, err := h.db.GetBusinessCurrentPlan(r.Context(), businessId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, 404, "business not found")
			helpers.LogError("trialHandler", "business not found", "businessId", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("trialHandler", "db error in GetBusinessCurrentPlan", "err", err.Error())
		return
	}
	// Block if they have EVER used a trial (IsTrialUsed)
	// OR
	// Block if they have EVER had a subscription status change (paid/cancelled/expired)
	// (i.e. If your status is ANYTHING other than 'inactive', you have a history.)
	if curPlan.IsTrialUsed || curPlan.SubscriptionsStatus != db.SubscriptionsStatusInactive {
		helpers.RespondWithError(w, 409, "trial is already used or status is other than inactive which means"+
			" business has a history with subscriptions")
		helpers.LogError("trialHandler", "trial is already used", "businessId", businessId)
		return
	}
	// ****************************************************************************************************************
	// check if trial is available
	// ****************************************************************************************************************
	eligibility, err := h.db.CheckUserPlanEligibility(r.Context(), userId)
	if err != nil {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("trialHandler", "db error in CheckUserPlanEligibility", "err", err.Error())
		return
	}
	if !eligibility.TrialAvailable {
		helpers.RespondWithError(w, http.StatusForbidden, "trial not available")
		helpers.LogInfo("trialHandler", "trial not available", "creatorId", userId)
		return
	}
	// ****************************************************************************************************************
	// if trial available and business doesn't has a plan already, grant the trial
	// ****************************************************************************************************************
	business, err := h.db.UpdateBusinessSubscription(r.Context(), db.UpdateBusinessSubscriptionParams{
		ID:                    businessId,
		CurrentSubscriptionID: pgtype.UUID{Valid: false},
		CurrentPlanID:         pgtype.Text{String: planId, Valid: true},
		SubscriptionsStatus:   db.SubscriptionsStatusTrialing,
		SubscriptionEndPeriod: pgtype.Timestamptz{Time: time.Now().Add(configs.TRIAL_DAYS), Valid: true},
		IsTrialUsed:           pgtype.Bool{Bool: true, Valid: true},
		IsOfferUsed:           pgtype.Bool{Valid: false},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, 404, "business not found")
			helpers.LogError("trialHandler", "business not found", "businessId", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("trialHandler", "db error in UpdateBusinessSubscription", "err", err.Error())
		return
	}
	// ****************************************************************************************************************
	// return the response
	// ****************************************************************************************************************
	res := struct {
		Business db.Business `json:"business"`
	}{Business: business}
	helpers.LogInfo("trialHandler", "trial set", "businessId", businessId)
	helpers.RespondWithJSON(w, 201, res)
}
