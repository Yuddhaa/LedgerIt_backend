package parties

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) GetAPartyHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Party db.Party `json:"party"`
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	partyId, ok := auth.ExtractUUID(w, r, "party_id")
	if !ok {
		return
	}
	party, err := h.db.GetParty(r.Context(), db.GetPartyParams{
		ID:         partyId,
		BusinessID: businessId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Party not found")
			helpers.LogInfo("GetAPartyHandler", "party not found in GetParty", "party_id", partyId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("GetAPartyHandler", "Db error in GetParty", "err", err, "party_id", partyId)
		return
	}
	res := resType{Party: party}
	helpers.LogInfo("GetAPartyHandler", "response sent", "response", res)
	helpers.RespondWithJSON(w, 200, res)
}
