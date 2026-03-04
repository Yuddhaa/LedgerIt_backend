package subscriptions

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) GetStatusHandler(w http.ResponseWriter, r *http.Request) {
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role != 1 {
		helpers.RespondWithError(w, http.StatusUnauthorized, "only creator can check status")
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	subscriptionId, ok := auth.ExtractUUID(w, r, "sub_id")
	if !ok {
		return
	}

	status, err := h.db.GetSubscriptionStatus(r.Context(), db.GetSubscriptionStatusParams{
		BusinessID: businessId,
		ID:         subscriptionId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.LogError("GetStatusHandler", "status not found for given path parameters",
				"businessId", businessId, "subscriptionId", subscriptionId)
			helpers.RespondWithError(w, 404, "status not found")
			return
		}
		helpers.LogError("GetStatusHandler", "db error in GetSubscriptionStatus",
			"businessId", businessId, "subscriptionId", subscriptionId)
		helpers.RespondWithError(w, 500, "internal server error")
		return
	}
	res := struct {
		Status db.SubscriptionsStatus `json:"status"`
	}{Status: status}
	helpers.LogInfo("GetStatusHandler", "status sent", "status", status,
		"businessId", businessId, "subscriptionId", subscriptionId)
	helpers.RespondWithJSON(w, 200, res)
}
