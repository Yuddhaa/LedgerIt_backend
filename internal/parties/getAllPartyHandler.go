package parties

import (
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
)

// GetPartyHandler is kind of a helper handler funciton
// based on the presence of a query parameter this calls the actual handler funcs
func (h *Handler) GetPartyHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Parties []db.Party `json:"parties"`
	}
	var parties []db.Party
	var err error
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	place := r.URL.Query().Get("place")
	// --------------------------------------------------------
	if place != "" {
		// Handle case: GET /?place=...
		parties, err = h.db.ListPartiesByBusinessAndPlace(r.Context(), db.ListPartiesByBusinessAndPlaceParams{
			BusinessID: businessId,
			Place:      place,
		})
		// --------------------------------------------------------
	} else {
		parties, err = h.db.ListPartiesByBusiness(r.Context(), businessId)
		// --------------------------------------------------------
	}
	if err != nil {
		helpers.LogError("GetPartyHandler", "error in either ListPartiesByBusiness or ListPartiesByBusinessAndPlace",
			"err", err.Error(), "businessId", businessId, "place", place)
		helpers.RespondWithError(w, 500, "internal server error")
		return
	}
	res := resType{Parties: parties}
	helpers.LogInfo("GetPartyHandler", "response sent", "res", res, "count", len(res.Parties), "businessId", businessId, "place", place)
	helpers.RespondWithJSON(w, 200, res)
}
