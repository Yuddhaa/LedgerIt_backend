package business

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

// DeleteBusinessHandler deletes a business, user has to be admin|creator
func (h *Handler) DeleteBusinessHandler(w http.ResponseWriter, r *http.Request) {
	// if user is not admin|creator return at the beginning itself
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role != 1 {
		helpers.RespondWithError(w, http.StatusUnauthorized, "Not enough credentials")
		helpers.LogInfo("AddMemberHandler", "not a creator")
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	curPlan, err := h.db.GetBusinessCurrentPlan(r.Context(), businessId)
	if err != nil {
		// err no rows is not required since in getrole its already established that the Business exists
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		helpers.LogError("getrole", "db error in GetBusinessCurrentPlan", "error", err.Error(),
			"businessId", businessId)
		return
	}

	isCancelled := curPlan.SubscriptionsStatus == db.SubscriptionsStatusCanceled ||
		curPlan.SubscriptionsStatus == db.SubscriptionsStatusTrialEnded ||
		curPlan.SubscriptionsStatus == db.SubscriptionsStatusInactive

	if !isCancelled && curPlan.CurrentPlanID.String != configs.FREE_PLAN_ID {
		helpers.RespondWithError(w, http.StatusForbidden, "can't delete active subscribed business")
		helpers.LogInfo("DeleteBusinessHandler", "can't delete active subscribed business")
		return
	}
	if err := h.db.DeleteBusiness(r.Context(), businessId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "business not found")
			helpers.LogInfo("DeleteBusinessHandler", "business not found in DeleteBusiness", "businessId", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("DeleteBusinessHandler", "Db error in DeleteBusiness", "err", err.Error())
		return
	}
	helpers.LogInfo("DeleteBusinessHandler", "204 response sent")
	w.WriteHeader(http.StatusNoContent)
}
