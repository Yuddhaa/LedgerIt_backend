package users

import (
	"errors"
	"net/http"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetProfileHandler returns the logged in user details
func (h *Handler) GetProfileHandler(w http.ResponseWriter, r *http.Request) {
	userId, ok := h.getUserIDFromContext(w, r)
	if !ok {
		return
	}
	type resType struct {
		User db.User `json:"user"`
	}
	user, err := h.db.GetUserById(r.Context(), pgtype.UUID{
		Bytes: userId,
		Valid: userId != uuid.Nil,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.LogError("GetProfileHandler", "No rows from GetUserById", "userId:", userId)
			helpers.RespondWithError(w, 404, "No user found")
			return
		}
		helpers.LogError("GetProfileHandler", "error in GetUserById", "err", err.Error(), "userId:", userId)
		helpers.RespondWithError(w, 500, "Server Error in getting user details")
		return
	}
	res := resType{User: user}
	helpers.LogInfo("GetProfileHandler", "response sent", "response", res)
	helpers.RespondWithJSON(w, 200, res)
}
