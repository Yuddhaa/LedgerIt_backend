package business

import (
	"encoding/json"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
)

// CreateBusinessHandler creates a new business and automatically adds
// the creator as the 'creator' (owner).
func (h *Handler) CreateBusinessHandler(w http.ResponseWriter, r *http.Request) {
	type reqType struct {
		Name string `json:"name"`
	}
	type resType struct {
		Business db.Business `json:"business"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		// CHANGED: Client error, log as Info.
		helpers.LogInfo("CreateBusinessHandler", "failed to decode request body", "error", err.Error())
		return
	}
	// extract userId from r.context
	userUuid, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return // error and response already sent in that helper func
	}

	// create business add the member owner in business_members
	business, err := h.db.CreateBusinessAndAddOwner(r.Context(), db.CreateBusinessAndAddOwnerParams{
		POwnerID: userUuid,
		PName:    body.Name,
	})
	if err != nil {
		// CHANGED: Don't leak DB error. Use structured logging.
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("CreateBusinessHandler", "db error in CreateBusinessAndAddOwner", "error", err.Error())
		return
	}

	// ADDED: Log successful creation
	helpers.LogInfo("CreateBusinessHandler", "business created successfully", "business_id", business.ID, "user_id", userUuid)
	helpers.RespondWithJSON(w, 201, resType{
		Business: business,
	})
}
