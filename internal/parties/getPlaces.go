package parties

import (
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/helpers"
)

func (h *Handler) GetPlacesHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Places []string `json:"places"`
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	places, err := h.db.ListUniquePartyPlacesByBusiness(r.Context(), businessId)
	if err != nil {
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("GetPlacesHandler", "Db error in ListUniquePartyPlacesByBusiness", "err", err.Error(), "businessId", businessId)
		return
	}
	res := resType{Places: places}
	helpers.LogInfo("GetPlacesHandler", "all the unique places of parties sent", "businessId", businessId, "places count", len(places))
	helpers.RespondWithJSON(w, 200, res)
}
