package categories

import (
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
)

func (h *Handler) GetAllCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Categories []db.TransactionCategory `json:"categories"`
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	categories, err := h.db.ListCategoriesByBusiness(r.Context(), businessId)
	if err != nil {
		helpers.LogError("GetAllCategoriesHandler", "error in ListCategoriesByBusiness", "err", err.Error(),
			"businessId", businessId)
		helpers.RespondWithError(w, 500, "internal server error")
		return
	}
	res := resType{Categories: categories}
	helpers.LogInfo("GetAllCategoriesHandler", "response sent", "res", res, "count", len(res.Categories),
		"businessId", businessId)
	helpers.RespondWithJSON(w, 200, res)
}
