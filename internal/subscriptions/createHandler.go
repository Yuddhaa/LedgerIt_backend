package subscriptions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	// check if the offer is already used if offercode has been passed in the body
	// ****************************************************************************************************************
	if body.OfferCode != "" {
		if curPlan.IsOfferUsed {
			helpers.RespondWithError(w, http.StatusConflict, "offer has already been used")
			helpers.LogError("CreateHandler", "offer has already been used", "businessId", businessId)
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
	// get offerid if body offercode exists
	// ****************************************************************************************************************
	rpOfferId := ""
	var marketerId pgtype.UUID
	if body.OfferCode != "" {
		offerRow, err := h.db.GetOfferDetailsByCode(r.Context(), body.OfferCode)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				helpers.RespondWithError(w, http.StatusNotFound, fmt.Sprintf("offerid does not exist for offercode: %s", body.OfferCode))
				helpers.LogInfo("CreateHandler", fmt.Sprintf("offerid does not exist for offercode: %s", body.OfferCode))
				return
			}
			helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
			helpers.LogInfo("CreateHandler", "error in GetRpOfferId", "err", err.Error())
			return
		}
		rpOfferId = offerRow.RazorpayOfferID
		marketerId = offerRow.ID
	}

	// ****************************************************************************************************************
	// if plan is "owner" branch of to create order (instead of subscription)
	// ****************************************************************************************************************
	notes := map[string]any{
		"type":              "fresh", // Tells Webhook: "Don't look for old subs to cancel"
		"user_id":           userId,
		"business_id":       businessId,
		"offer_code":        body.OfferCode,
		"razorpay_offer_id": rpOfferId,
	}
	if plan.ID == configs.PERMANENT_PLAN_ID {
		h.createOrder(w, r.Context(), businessId, plan, notes, rpOfferId, marketerId)
		return
	}
	// ****************************************************************************************************************
	// now that we have the plan, create the subscription
	// ****************************************************************************************************************
	// Start Immediately (startAt = nil)
	res, ok := h.createSubscription(w, r, plan, notes, businessId, nil, rpOfferId, marketerId)
	if !ok {
		return
	}
	helpers.RespondWithJSON(w, 201, res)
	helpers.LogInfo("CreateHandler", "subscription created", "res", res)
}
