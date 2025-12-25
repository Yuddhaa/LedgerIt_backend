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
		helpers.LogError("LogoutHandler", "failed to decode request body", "error", err.Error())
		return
	}

	if body.RefreshToken == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "refresh_token is required")
		helpers.LogInfo("LogoutHandler", "logout request failed: missing refresh_token")
		return
	}

	// hash the refresh token
	hashedToken := hashToken(body.RefreshToken)

	// delete from refresh token table
	if err := h.db.DeleteRefreshTokenByHash(r.Context(), hashedToken); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("LogoutHandler", "error in DeleteRefreshTokenByHash", "error", err.Error())
		return
	}

	helpers.LogInfo("LogoutHandler", "user logged out successfully")
	helpers.RespondWithJSON(w, 200, resType{Msg: "User logged out"})
}
