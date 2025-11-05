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

func (h *Handler) LogoutAllHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}

	// ---------------------------------------------------------------------------------------------------
	// 1. Get Claims from context
	claims, ok := GetClaimsFromContext(r.Context())
	if !ok {
		helpers.RespondWithError(w, 500, "could now retrieve claims from context")
		h.logger.Error("could now retrieve claims from context")
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 2. Prepare all data *before* the transaction

	// Get User ID and convert
	userId := claims.UserId
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		helpers.RespondWithError(w, 500, "error in converting userId to uuid: "+err.Error())
		h.logger.Error("error in converting userId to uuid", "error", err)
		return
	}
	pgTypeUuid := pgtype.UUID{
		Bytes: userUuid,
		Valid: userUuid != uuid.Nil,
	}

	// Generate the new refresh token
	refreshToken, err := generateSecureRandomString(32)
	if err != nil {
		helpers.RespondWithError(w, 500, "err in generateSecureRandomString, err:"+err.Error())
		h.logger.Error("err in generateSecureRandomString, err:" + err.Error())
		return
	}
	hashedRandStr := hashToken(refreshToken)
	deviceInfo := r.UserAgent()

	// ---------------------------------------------------------------------------------------------------
	// 3. Begin Transaction
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, 500, "Error starting transaction")
		h.logger.Error("Failed to begin transaction", "error", err)
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.db.WithTx(tx)

	// 3a. Delete all old tokens for this user
	if err := qtx.DeleteRefreshTokensByUserID(r.Context(), pgTypeUuid); err != nil {
		helpers.RespondWithError(w, 500, "error in DeleteRefreshTokensByUserID, err:"+err.Error())
		h.logger.Error("error in tx DeleteRefreshTokensByUserID", "error", err)
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
		helpers.RespondWithError(w, 500, "err in InsertRefreshToken, err:"+err.Error())
		h.logger.Error("err in tx InsertRefreshToken", "error", err)
		return // Rollback is deferred
	}

	// 3c. Commit
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, 500, "Error committing transaction")
		h.logger.Error("Failed to commit transaction", "error", err)
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
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error: "+err.Error())
		h.logger.Error("error in signing the jwt token, err:" + err.Error())
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 5. Respond
	helpers.RespondWithJSON(w, 200, resType{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}
