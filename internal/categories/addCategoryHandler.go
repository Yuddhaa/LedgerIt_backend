package categories

import (
	"encoding/json"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
)

func (h *Handler) AddCategoryHandler(w http.ResponseWriter, r *http.Request) {
	type reqType struct {
		Name string `json:"name"`
	}
	type resType struct {
		Category db.TransactionCategory `json:"category"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad request")
		helpers.LogError("AddCategoryHandler", "json decode error", "err", err)
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	category, err := h.db.CreateCategory(r.Context(), db.CreateCategoryParams{
		BusinessID: businessId,
		Name:       body.Name,
	})
	if err != nil {
		if helpers.IsUniqueViolation(err) {
			helpers.RespondWithError(w, http.StatusConflict, "category already exists")
			// ADDED: Log the conflict
			helpers.LogInfo("AddCategoryHandler", "conflict: category already exists", "party name", body.Name, "business_id", businessId)
			return
		}
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("AddCategoryHandler", "error in db CreateCategory", "err", err)
		return
	}
	res := resType{Category: category}
	helpers.LogInfo("AddCategoryHandler", "response sent", "response", res)
	helpers.RespondWithJSON(w, 201, res)
}
