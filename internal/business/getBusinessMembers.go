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

// GetBusinessMembers returns all the members' details of a particular business
func (h *Handler) GetBusinessMembers(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Members []db.GetBusinessMembersRow `json:"members"`
	}

	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return // res and err is already sent in above func
	}
	business_id := chi.URLParam(r, "id")
	business_uuid, err := uuid.Parse(business_id)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Id URLParam")
		return
	}

	// check if the user is a member of the business
	_, err = h.db.IsUserMemberOfBusiness(r.Context(), db.IsUserMemberOfBusinessParams{
		UserID: userId,
		BusinessID: pgtype.UUID{
			Bytes: business_uuid,
			Valid: business_uuid != uuid.Nil,
		},
	})
	if err != nil {
		// If the error IS pgx.ErrNoRows, it means they are NOT a member.
		if errors.Is(err, pgx.ErrNoRows) {
			// Return 404 (or 403 Forbidden)
			helpers.RespondWithError(w, http.StatusNotFound, "business not found or access denied")
			return
		}
		// Any other error is a 500
		helpers.RespondWithError(w, 500, "internal server error during authorization")
		helpers.LogError("GetBusinessMembers", "IsUserMemberOfBusiness db error", "err", err, "business_uuid", business_uuid, "user_id", userId)
		return
	}

	// if the user is a member, then
	// GetBusinessMembers
	members, err := h.db.GetBusinessMembers(r.Context(), pgtype.UUID{
		Bytes: business_uuid,
		Valid: business_uuid != uuid.Nil,
	})
	if err != nil {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("GetBusinessMembers", "GetBusinessMembers db error", "err", err, "business_uuid", business_uuid)
		return
	}
	res := resType{
		Members: members,
	}
	helpers.LogInfo("GetBusinessMembers", "response sent", "Members", members)
	helpers.RespondWithJSON(w, 200, res)
}
