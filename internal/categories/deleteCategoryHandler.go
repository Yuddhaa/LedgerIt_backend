package categories

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) DeleteCategoryHandler(w http.ResponseWriter, r *http.Request) {
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	categoryId, ok := auth.ExtractUUID(w, r, "category_id")
	if !ok {
		return
	}
	_, err := h.db.DeleteCategory(r.Context(), db.DeleteCategoryParams{
		ID:         categoryId,
		BusinessID: businessId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "category not found")
			helpers.LogInfo("DeleteCategoryHandler", "category not found in DeleteCategory", "categoryId", categoryId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("DeleteCategoryHandler", "Db error in DeleteCategory", "err", err.Error(), "categoryId", categoryId)
		return
	}
	helpers.LogInfo("DeleteCategoryHandler", "204 response sent")
	w.WriteHeader(http.StatusNoContent)
}
