package parties

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) DeletePartyHandler(w http.ResponseWriter, r *http.Request) {
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	partyId, ok := auth.ExtractUUID(w, r, "party_id")
	if !ok {
		return
	}
	_, err := h.db.DeleteParty(r.Context(), db.DeletePartyParams{
		ID:         partyId,
		BusinessID: businessId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Party not found")
			helpers.LogInfo("DeletePartyHandler", "party not found in DeleteParty", "party_id", partyId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("DeletePartyHandler", "Db error in DeleteParty", "err", err)
		return
	}
	helpers.LogInfo("DeletePartyHandler", "204 response sent")
	w.WriteHeader(http.StatusNoContent)
}
