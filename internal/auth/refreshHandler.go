package auth

import (
	// <-- Make sure context is imported
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	// "github.com/jackc/pgx/v5" // <-- May be needed for pgx.Tx
)

func (h *Handler) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	// reqType and resType and decode body
	type reqType struct {
		RefreshToken string `json:"refreshToken"`
	}
	type resType struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request")
		h.logger.Error("bad request,err:" + err.Error())
		return
	}
	helpers.PrintResponse("RefreshHandler reqbody", body)

	// ---------------------------------------------------------------------------------------------------
	// Hash the incoming token
	hashedToken := hashToken(body.RefreshToken)

	// ---------------------------------------------------------------------------------------------------
	// 1. GET: Find the token. This can happen *before* the transaction.
	row, err := h.db.GetRefreshTokenByHash(r.Context(), hashedToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
			h.logger.Warn("Unauthorized: refresh token not found or expired")
			return
		} else {
			helpers.RespondWithError(w, 500, "Error looking up token")
			h.logger.Error("Error in GetRefreshTokenByHash", "error", err)
			return
		}
	}

	// ---------------------------------------------------------------------------------------------------
	// 2. GENERATE: Create new tokens *before* starting the DB transaction.
	// No point in starting a transaction if we fail to generate a crypto string.

	// Generate new Access Token
	accessToken, err := GenerateJwt(&Claims{
		UserId: h.uuidToString(row.UserID),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "LedgerIt",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), // 1 hour is good
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}, h.jwtSecret)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error signing new token")
		h.logger.Error("error in signing the jwt token", "error", err)
		return
	}

	// Generate new Refresh Token
	newRefreshToken, err := generateSecureRandomString(32)
	if err != nil {
		helpers.RespondWithError(w, 500, "Error generating new token")
		h.logger.Error("err in generateSecureRandomString", "error", err)
		return
	}
	newHashedToken := hashToken(newRefreshToken)
	deviceInfo := r.UserAgent()

	// ---------------------------------------------------------------------------------------------------
	// 3. TRANSACTION: Begin atomic database operation

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, 500, "Error starting transaction")
		h.logger.Error("Failed to begin transaction", "error", err)
		return
	}
	// Defer a rollback. If Commit() is called, this does nothing.
	// If we return early due to an error, this cleans everything up.
	defer tx.Rollback(r.Context())

	// Get a new *Queries struct that is bound to this transaction
	qtx := h.db.WithTx(tx)

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
		helpers.RespondWithError(w, 500, "Error saving new token")
		h.logger.Error("err in tx InsertRefreshToken", "error", err)
		return // Rollback is deferred
	}

	// 3b. DELETE the old token
	if err := qtx.DeleteRefreshTokenByHash(r.Context(), hashedToken); err != nil {
		helpers.RespondWithError(w, 500, "Error invalidating old token")
		h.logger.Error("err in tx DeleteRefreshTokenByHash", "error", err)
		return // Rollback is deferred
	}

	// 3c. COMMIT: If all went well, commit the transaction.
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, 500, "Error committing transaction")
		h.logger.Error("Failed to commit transaction", "error", err)
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 4. RESPOND: All database work is done. Send the new tokens to the client.
	helpers.RespondWithJSON(w, 200, resType{
		RefreshToken: newRefreshToken, // The new, raw token
		AccessToken:  accessToken,
	})
}
