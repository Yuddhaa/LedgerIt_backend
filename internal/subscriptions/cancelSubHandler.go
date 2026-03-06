package subscriptions

import (
	"errors"
	"net/http"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) CancelSubHandler(w http.ResponseWriter, r *http.Request) {
	// ****************************************************************************************************************
	// 1. Auth & Context
	// ****************************************************************************************************************
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// ****************************************************************************************************************
	// 2. Check Role (Only Owner/Creator should cancel)
	// ****************************************************************************************************************
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok || role != 1 { // Assuming 1 = Creator/Owner
		helpers.RespondWithError(w, http.StatusForbidden, "Only the owner can cancel subscriptions")
		return
	}

	// ****************************************************************************************************************
	// 3. Get Current Plan Details
	// ****************************************************************************************************************
	curPlan, err := h.db.GetBusinessCurrentPlan(r.Context(), businessId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "No active subscription found")
			return
		}
		helpers.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if curPlan.CurrentPlanID.Valid && curPlan.CurrentPlanID.String == configs.PERMANENT_PLAN_ID {
		helpers.RespondWithError(w, http.StatusForbidden, "cannot cancel lifetime plan, already paid")
		helpers.LogError("CancelSubHandler", "cannot cancel lifetime plan, already paid", "businessId", businessId)
		return
	}

	isTrial := curPlan.SubscriptionsStatus == db.SubscriptionsStatusTrialing ||
		curPlan.SubscriptionsStatus == db.SubscriptionsStatusTrialEnded

	if isTrial {
		// ****************************************************************************************************************
		// Update Database immediately as it is trial
		// ****************************************************************************************************************
		err = h.db.CancelTrial(r.Context(), businessId)
		if err != nil {
			helpers.LogError("CancelHandler", "Local DB, trial cancel update failed", "err", err.Error())
			helpers.RespondWithError(w, 500, "Internal Server Error")
			return
		}
		helpers.RespondWithJSON(w, http.StatusOK, map[string]string{
			"message": "Subscription cancelled successfully",
			"status":  "canceled",
		})
		return
	}

	// Validate: Is there actually a Razorpay Sub ID to cancel?
	if !curPlan.RazorpaySubscriptionID.Valid || curPlan.RazorpaySubscriptionID.String == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "No active recurring subscription to cancel")
		return
	}

	// Validate: Is it already cancelled?
	// (Optional check, but saves an API call)
	if curPlan.SubscriptionsStatus == db.SubscriptionsStatusCanceled {
		helpers.RespondWithError(w, http.StatusConflict, "Subscription is already cancelled")
		return
	}
	// ****************************************************************************************************************
	// 4. Call Razorpay API
	// ****************************************************************************************************************
	rzpSubID := curPlan.RazorpaySubscriptionID.String

	// cancel_at_cycle_end = 0 (Immediate) or 1 (End of Cycle)
	_, err = h.rp_client.Subscription.Cancel(rzpSubID, map[string]any{"cancel_at_cycle_end": 0}, nil)
	if err != nil {
		// Edge Case: If Razorpay says "It's already cancelled", we can treat that as success.
		if strings.Contains(strings.ToLower(err.Error()), "cancelled") {
			helpers.LogInfo("CancelHandler", "Subscription was already cancelled on Razorpay", "id", rzpSubID)
			// Fall through to update DB
		} else {
			helpers.LogError("CancelHandler", "Razorpay cancel failed", "err", err.Error())
			helpers.RespondWithError(w, http.StatusBadGateway, "Could not cancel subscription. Please try again or contact support.")
			return
		}
	}

	// ****************************************************************************************************************
	// 5. Update Database (Only reachable if Razorpay call succeeded)
	// ****************************************************************************************************************
	err = h.db.CancelSubscription(r.Context(), rzpSubID)
	if err != nil {
		// Rare Edge Case: Razorpay cancelled, but local DB failed.
		// This is acceptable because the Webhook will eventually arrive and fix the DB state.
		// We can still tell the user "Success" because the billing has stopped.
		helpers.LogError("CancelHandler", "Local DB update failed (Webhook will sync later)", "err", err.Error())
	}

	// ****************************************************************************************************************
	// 6. Response
	// ****************************************************************************************************************
	helpers.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Subscription cancelled successfully",
		"status":  "canceled",
	})
	helpers.LogInfo("CancelHandler", "Subscription cancelled by user", "business_id", businessId, "sub_id", rzpSubID)
}
