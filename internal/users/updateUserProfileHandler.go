package users

import (
	"encoding/json"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UpdateUserProfileHandler updates the name and phone number for the
// currently authenticated user.
func (h *Handler) UpdateUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Get claims from context (guaranteed by auth middleware)
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: This is a server error; middleware should guarantee claims.
		helpers.LogError("UpdateUserProfileHandler", "could not retrieve claims from context")
		return
	}

	userId := claims.UserId
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: Claims data is malformed, this is a server-side issue.
		helpers.LogError("UpdateUserProfileHandler", "could not parse userId from claims", "error", err.Error(), "claim_user_id", userId)
		return
	}

	// 2. Define and decode request body
	type reqType struct {
		Name        string `json:"name"`
		PhoneNumber string `json:"phone_number"`
	}
	type resType struct {
		User db.User `json:"user"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		// CHANGED: This is a client error. Log as Info.
		helpers.LogInfo("UpdateUserProfileHandler", "failed to decode request body", "error", err.Error())
		return
	}

	// 3. Log the processing event
	helpers.LogInfo("UpdateUserProfileHandler", "processing user profile update",
		"user_id", userId,
		"name", body.Name,
		"phone", body.PhoneNumber,
	)

	// 4. Perform database update
	user, err := h.db.UpdateUserProfile(r.Context(), db.UpdateUserProfileParams{
		ID: pgtype.UUID{
			Bytes: userUuid,
			Valid: userUuid != uuid.Nil,
		},
		Name: pgtype.Text{
			String: body.Name,
			Valid:  body.Name != "",
		},
		PhoneNumber: pgtype.Text{
			String: body.PhoneNumber,
			Valid:  body.PhoneNumber != "",
		},
	})
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: This is a 500 error. Use LogError and don't leak details.
		helpers.LogError("UpdateUserProfileHandler", "error in sql UpdateUserProfile", "error", err.Error(), "user_id", userId)
		return
	}

	// 5. Send successful response
	helpers.LogInfo("UpdateUserProfileHandler", "user profile updated successfully", "user_id", userId)
	helpers.RespondWithJSON(w, 200, resType{
		User: user,
	})
}
