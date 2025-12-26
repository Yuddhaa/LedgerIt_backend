package business

import (
	"encoding/json"
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

// UpdateBusinessHandler updates the business name if the
// user is admin|creator
func (h *Handler) UpdateBusinessHandler(w http.ResponseWriter, r *http.Request) {
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role == 2 {
		helpers.RespondWithError(w, http.StatusUnauthorized, "Not an admin or creator")
		helpers.LogInfo("UpdateBusinessHandler", "Not an admin or creator")
		return
	}

	// ****************************************************************************************************************
	// request and response types
	// ****************************************************************************************************************
	type reqType struct {
		Name string `json:"name"`
	}
	type resType struct {
		Business db.Business `json:"business"`
	}
	// decode req body
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		helpers.LogInfo("UpdateBusinessHandler", "failed to decode request body", "error", err.Error())
		return
	}
	if body.Name == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: name is empty")
		helpers.LogInfo("UpdateBusinessHandler", "bad request: name is empty")
		return
	}

	// extract businessId
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// ****************************************************************************************************************
	// DB call
	// ****************************************************************************************************************
	business, err := h.db.UpdateBusiness(r.Context(), db.UpdateBusinessParams{
		ID:   businessId,
		Name: body.Name,
	})
	if err != nil {
		if helpers.IsUniqueViolation(err) {
			helpers.RespondWithError(w, http.StatusConflict, "Business already exists")
			helpers.LogInfo("UpdateBusinessHandler", "Business already exists")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Business not found")
			helpers.LogInfo("UpdateBusinessHandler", "Business not found in UpdateBusiness", "businessId", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("UpdateBusinessHandler", "Db error in UpdateBusiness", "businessId", businessId, "err", err.Error())
		return
	}
	// ****************************************************************************************************************
	// send response
	// ****************************************************************************************************************
	res := resType{Business: business}
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("UpdateBusinessHandler", "response sent", "businessId", businessId)
}
