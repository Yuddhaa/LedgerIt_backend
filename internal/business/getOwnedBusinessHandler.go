package business

import (
	"net/http"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetOwnedBusinessHandler returns all the businesses owned (created) by a particular user.
func (h *Handler) GetOwnedBusinessHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Business []db.Business `json:"business"`
	}
	// extract userId from r.context
	userId, ok := h.getUserIDFromContext(w, r)
	if !ok {
		return // error and response already sent in that helper func
	}
	business, err := h.db.GetBusinessesByOwnerID(r.Context(), pgtype.UUID{
		Bytes: userId,
		Valid: userId != uuid.Nil,
	})
	if err != nil {
		// CHANGED: Don't leak DB error. Use structured logging.
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("GetOwnedBusinessHandler", "db error in GetBusinessesByOwnerID", "error", err, "user_id", userId)
		return
	}

	// ADDED: Log successful retrieval
	helpers.LogInfo("GetOwnedBusinessHandler", "retrieved owned businesses for user", "user_id", userId, "count", len(business))
	helpers.RespondWithJSON(w, 200, resType{
		Business: business,
	})
}
