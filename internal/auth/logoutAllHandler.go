package auth

import (
	"net/http"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// LogoutAllHandler logs a user out of all devices.
// It performs the following steps:
// 1. Reads the user's ID from the JWT claims.
// 2. Starts a database transaction.
// 3. Deletes *all* existing refresh tokens for that user.
// 4. Inserts *one* new refresh token for the current device.
// 5. Commits the transaction.
// 6. Issues a new access token and returns both new tokens to the client.
func (h *Handler) LogoutAllHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	// ---------------------------------------------------------------------------------------------------
	// 1. Get Claims from context
	claims, ok := GetClaimsFromContext(r.Context())
	if !ok {
		helpers.RespondWithError(w, http.StatusInternalServerError, "could not retrieve claims from context")
		// CHANGED: This is a server error; the middleware should guarantee claims.
		helpers.LogError("LogoutAllHandler", "could not retrieve claims from context")
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 2. Prepare all data *before* the transaction

	// Get User ID and convert
	userId := claims.UserId
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: This is a critical server error; claims are malformed.
		helpers.LogError("LogoutAllHandler", "error in converting userId to uuid", "error", err, "user_id_from_claim", userId)
		return
	}
	pgTypeUuid := pgtype.UUID{
		Bytes: userUuid,
		Valid: userUuid != uuid.Nil,
	}

	// Generate the new refresh token
	refreshToken, err := generateSecureRandomString(32)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("LogoutAllHandler", "err in generateSecureRandomString", "error", err)
		return
	}
	hashedRandStr := hashToken(refreshToken)
	deviceInfo := r.UserAgent()

	// ---------------------------------------------------------------------------------------------------
	// 3. Begin Transaction
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error starting transaction")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("LogoutAllHandler", "Failed to begin transaction", "error", err)
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.db.WithTx(tx)

	// 3a. Delete all old tokens for this user
	if err := qtx.DeleteRefreshTokensByUserID(r.Context(), pgTypeUuid); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("LogoutAllHandler", "error in tx DeleteRefreshTokensByUserID", "error", err, "user_id", userId)
		return // Rollback is deferred
	}

	// 3b. Insert the *one* new token for the current session
	if _, err := qtx.InsertRefreshToken(r.Context(), db.InsertRefreshTokenParams{
		UserID:    pgTypeUuid,
		TokenHash: hashedRandStr,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(30 * 24 * time.Hour),
			Valid: true,
		},
		DeviceInfo: pgtype.Text{
			String: deviceInfo,
			Valid:  deviceInfo != "",
		},
	}); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("LogoutAllHandler", "err in tx InsertRefreshToken", "error", err, "user_id", userId)
		return // Rollback is deferred
	}

	// 3c. Commit
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error committing transaction")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("LogoutAllHandler", "Failed to commit transaction", "error", err)
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 4. Generate new access token (only after DB success)
	accessToken, err := GenerateJwt(&Claims{
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "LedgerIt",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}, h.jwtSecret)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("LogoutAllHandler", "error in signing the jwt token", "error", err)
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 5. Respond

	// ADDED: Log the successful event.
	helpers.LogInfo("LogoutAllHandler", "user logged out of all devices", "user_id", userId)

	helpers.RespondWithJSON(w, 200, resType{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}
