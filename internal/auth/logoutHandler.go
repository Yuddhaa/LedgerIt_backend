package auth

import (
	"encoding/json"
	"net/http"

	"LedgerIt/internal/helpers"
)

// LogoutHandler handles a single-device logout.
// It receives a refresh token, hashes it, and deletes the corresponding
// entry from the refresh_tokens table.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// req and res type + decode body
	type reqType struct {
		RefreshToken string `json:"refresh_token"`
	}
	type resType struct {
		Msg string `json:"message"`
	}

	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		// CHANGED: This is a client error, not a server error. Log as Info.
		helpers.LogInfo("LogoutHandler", "failed to decode request body", "error", err.Error())
		return
	}

	// ADDED: Log the action, but NOT the sensitive token.
	helpers.LogInfo("LogoutHandler", "processing logout request")

	// ADDED: Validate that the token is not empty.
	if body.RefreshToken == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "refresh_token is required")
		helpers.LogInfo("LogoutHandler", "logout request failed: missing refresh_token")
		return
	}

	// hash the refresh token
	hashedToken := hashToken(body.RefreshToken)

	// delete from refresh token table
	if err := h.db.DeleteRefreshTokenByHash(r.Context(), hashedToken); err != nil {
		// CHANGED: Don't leak the DB error to the client.
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: Use helper and structured logging. This is a real server error.
		helpers.LogError("LogoutHandler", "error in DeleteRefreshTokenByHash", "error", err.Error(), "token_hash", hashedToken)
		return
	}

	// ADDED: Log the successful logout event.
	helpers.LogInfo("LogoutHandler", "user logged out successfully", "token_hash", hashedToken)
	helpers.RespondWithJSON(w, 200, resType{Msg: "User logged out"})
}
