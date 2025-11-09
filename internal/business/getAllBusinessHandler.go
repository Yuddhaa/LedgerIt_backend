package business

import (
	"net/http"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetAllBusinessHandler returns a list of all businesses that a user is a member of.
func (h *Handler) GetAllBusinessHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Business []db.Business `json:"business"`
	}

	// extract userId from r.context
	userId, ok := h.getUserIDFromContext(w, r)
	if !ok {
		return // error and response already sent in that helper func
	}
	business, err := h.db.GetBusinessesByUserID(r.Context(), pgtype.UUID{
		Bytes: userId,
		Valid: userId != uuid.Nil,
	})
	if err != nil {
		// CHANGED: Don't leak DB error. Use structured logging.
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("GetAllBusinessHandler", "db error in GetBusinessesByUserID", "error", err, "user_id", userId)
		return
	}

	// ADDED: Log successful retrieval
	helpers.LogInfo("GetAllBusinessHandler", "retrieved all businesses for user", "user_id", userId, "count", len(business))
	helpers.RespondWithJSON(w, 200, resType{
		Business: business,
	})
}
