package business

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
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
	if role == 2 {
		helpers.RespondWithError(w, http.StatusUnauthorized, "Not enough credentials")
		helpers.LogInfo("AddMemberHandler", "not an admin|creator")
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
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
