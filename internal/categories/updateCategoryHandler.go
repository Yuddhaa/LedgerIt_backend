package categories

import (
	"encoding/json"
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) UpdateCategoryHandler(w http.ResponseWriter, r *http.Request) {
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role == 2 {
		helpers.LogError("UpdateCategoryHandler", "Not an admin or creator to update categories")
		helpers.RespondWithError(w, http.StatusUnauthorized, "Not an admin or creator")
		return
	}
	type reqType struct {
		Name string `json:"name"`
	}
	type resType struct {
		Category db.TransactionCategory `json:"category"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, 400, "Bad Request")
		helpers.LogError("UpdateCategoryHandler", "bad request body", "err", err.Error())
		return
	}
	BusinessID, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	categoryId, ok := auth.ExtractUUID(w, r, "category_id")
	if !ok {
		return
	}
	category, err := h.db.UpdateCategory(r.Context(), db.UpdateCategoryParams{
		ID:         categoryId,
		Name:       body.Name,
		BusinessID: BusinessID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "category not found")
			helpers.LogInfo("UpdateCategoryHandler", "category not found in UpdateCategory", "categoryId", categoryId)
			return
		}
		if helpers.IsUniqueViolation(err) {
			helpers.RespondWithError(w, http.StatusConflict, "category already exists")
			// ADDED: Log the conflict
			helpers.LogInfo("UpdateCategoryHandler", "conflict: category already exists",
				"category name", body.Name, "categoryId", categoryId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("UpdateCategoryHandler", "Db error in UpdateCategory", "err", err.Error(), "body", body)
		return
	}
	res := resType{Category: category}
	helpers.LogInfo("UpdateCategoryHandler", "category updated", "categoryId", categoryId)
	helpers.RespondWithJSON(w, 200, res)
}
