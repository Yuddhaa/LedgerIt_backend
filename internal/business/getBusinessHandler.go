package business

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetBusinessHandler returns a single business by its ID,
// but only if the requesting user is a member of that business.
func (h *Handler) GetBusinessHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Business db.GetBusinessByIDRow `json:"business"`
	}

	// extract userId from r.context
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return // error and response already sent in that helper func
	}

	// extract businessId from url param
	businessId, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid business ID")
		// CHANGED: Client error, log as Info.
		helpers.LogInfo("GetBusinessHandler", "failed to parse business ID from URL", "error", err, "url_param", chi.URLParam(r, "id"))
		return
	}

	business, err := h.db.GetBusinessByID(r.Context(), db.GetBusinessByIDParams{
		UserID: userId,
		ID: pgtype.UUID{
			Bytes: businessId,
			Valid: businessId != uuid.Nil,
		},
	})
	if err != nil {
		// Check if the error is "no rows found"
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Business not found or access denied")
			// ADDED: Log the "not found" event
			helpers.LogInfo("GetBusinessHandler", "business not found or access denied for user", "user_id", userId, "business_id", businessId)
			return
		}

		// Any other error is a real 500
		// CHANGED: Don't leak DB error. Use structured logging.
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("GetBusinessHandler", "db error in GetBusinessByID", "error", err, "user_id", userId, "business_id", businessId)
		return
	}

	// ADDED: Log successful retrieval
	helpers.LogInfo("GetBusinessHandler", "retrieved business successfully", "user_id", userId, "business_id", businessId)
	helpers.RespondWithJSON(w, 200, resType{
		Business: business,
	})
}
