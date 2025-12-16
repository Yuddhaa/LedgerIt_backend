package categories

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) GetACategoryHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Category db.TransactionCategory `json:"category"`
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	categoryId, ok := auth.ExtractUUID(w, r, "category_id")
	if !ok {
		return
	}
	category, err := h.db.GetCategory(r.Context(), db.GetCategoryParams{
		ID:         categoryId,
		BusinessID: businessId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Party not found")
			helpers.LogInfo("GetACategoryHandler", "category not found in GetCategory", "categoryId", categoryId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("GetACategoryHandler", "Db error in GetCategory", "err", err.Error(), "categoryId", categoryId)
		return
	}
	res := resType{Category: category}
	helpers.LogInfo("GetACategoryHandler", "response sent", "response", res)
	helpers.RespondWithJSON(w, 200, res)
}
