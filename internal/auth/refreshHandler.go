package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// RefreshHandler implements refresh token rotation.
// It performs the following steps:
//  1. Receives an old refresh token.
//  2. Finds the token in the database. If not found, returns 401 Unauthorized.
//  3. Generates a new access token AND a new refresh token.
//  4. Starts a transaction:
//     a. Inserts the *new* refresh token.
//     b. Deletes the *old* refresh token.
//  5. Commits the transaction.
//  6. Returns both new tokens to the client.
//
// This process ensures that each refresh token can only be used once,
// enhancing security.
func (h *Handler) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	// reqType and resType and decode body
	type reqType struct {
		RefreshToken string `json:"refresh_token"`
	}
	type resType struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		// CHANGED: This is a client error, log as Info.
		helpers.LogError("RefreshHandler", "failed to decode request body", "error", err.Error())
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// Hash the incoming token
	hashedToken := hashToken(body.RefreshToken)

	// ---------------------------------------------------------------------------------------------------
	// 1. GET: Find the token. This can happen *before* the transaction.
	helpers.LogInfo("RefreshHandler", "before GetRefreshTokenByHash")
	row, err := h.db.GetRefreshTokenByHash(r.Context(), hashedToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// This is an expected "error" (client token is invalid/not found).
			helpers.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
			helpers.LogInfo("RefreshHandler", "Unauthorized: refresh token not found, expired, or already used")
			return
		} else {
			// This is a real database error.
			helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
			// CHANGED: Slightly more specific message.
			helpers.LogError("RefreshHandler", "failed to get refresh token by hash", "error", err.Error())
			return
		}
	}
	helpers.LogInfo("RefreshHandler", "after GetRefreshTokenByHash")

	// ---------------------------------------------------------------------------------------------------
	// 2. GENERATE: Create new tokens *before* starting the DB transaction.

	// Generate new Access Token
	accessToken, err := GenerateJwt(&Claims{
		UserId: h.uuidToString(row.UserID),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "LedgerIt",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), // 1 hour is good
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}, configs.Configs.JWT_SECRET)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("RefreshHandler", "error in signing the jwt token", "error", err.Error())
		return
	}

	// Generate new Refresh Token
	newRefreshToken, err := generateSecureRandomString(32)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("RefreshHandler", "error in generateSecureRandomString", "error", err.Error())
		return
	}
	newHashedToken := hashToken(newRefreshToken)
	deviceInfo := r.UserAgent()

	// ---------------------------------------------------------------------------------------------------
	// 3. TRANSACTION: Begin atomic database operation

	helpers.LogInfo("RefreshHandler", "before Begin tx")
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error starting transaction")
		helpers.LogError("RefreshHandler", "Failed to begin transaction", "error", err.Error())
		return
	}
	// Defer a rollback. If Commit() is called, this does nothing.
	defer tx.Rollback(r.Context())
	helpers.LogInfo("RefreshHandler", "after Begin tx")

	qtx := h.db.WithTx(tx)
	helpers.LogInfo("RefreshHandler", "before InsertRefreshToken")
	// 3a. INSERT the new token
	if _, err := qtx.InsertRefreshToken(r.Context(), db.InsertRefreshTokenParams{
		UserID:    row.UserID,
		TokenHash: newHashedToken,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(30 * 24 * time.Hour),
			Valid: true,
		},
		DeviceInfo: pgtype.Text{
			String: deviceInfo,
			Valid:  deviceInfo != "",
		},
	}); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error saving new token")
		helpers.LogError("RefreshHandler", "err in tx InsertRefreshToken", "error", err.Error())
		return // Rollback is deferred
	}
	helpers.LogInfo("RefreshHandler", "after InsertRefreshToken")
	helpers.LogInfo("RefreshHandler", "before DeleteRefreshTokenByHash")
	// 3b. DELETE the old token
	if err := qtx.DeleteRefreshTokenByHash(r.Context(), hashedToken); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error invalidating old token")
		helpers.LogError("RefreshHandler", "err in tx DeleteRefreshTokenByHash", "error", err.Error())
		return // Rollback is deferred
	}
	helpers.LogInfo("RefreshHandler", "after DeleteRefreshTokenByHash")
	helpers.LogInfo("RefreshHandler", "before commit")
	// 3c. COMMIT: If all went well, commit the transaction.
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error committing transaction")
		helpers.LogError("RefreshHandler", "Failed to commit transaction", "error", err.Error())
		return
	}
	helpers.LogInfo("RefreshHandler", "after commit")
	// ---------------------------------------------------------------------------------------------------
	// 4. RESPOND: All database work is done. Send the new tokens to the client.
	res := resType{
		RefreshToken: newRefreshToken, // The new, raw token
		AccessToken:  accessToken,
	}

	helpers.LogInfo("RefreshHandler", "token refresh successful")
	helpers.RespondWithJSON(w, 200, res)
}
