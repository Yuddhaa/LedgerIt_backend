package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/api/idtoken"
)

func (h *Handler) GoogleAuthHandler(w http.ResponseWriter, r *http.Request) {
	// req and res type
	type reqType struct {
		IdToken string `json:"idToken"`
	}
	type resType struct {
		User         db.UpsertUserByEmailRow `json:"user"`
		AccessToken  string                  `json:"accessToken"`
		RefreshToken string                  `json:"refreshToken"`
	}

	// ---------------------------------------------------------------------------------------------------
	// 1. Decode and Validate (No DB ops yet)
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request")
		h.logger.Error("bad request, err:" + err.Error())
		return
	}
	helpers.PrintResponse("GoogleAuthHandler req.body:", body)

	payload, err := idtoken.Validate(r.Context(), body.IdToken, h.googleWebClientId)
	if err != nil {
		http.Error(w, "Error: "+err.Error(), http.StatusUnauthorized)
		h.logger.Error("error in validating id token. err:" + err.Error())
		return
	}

	googleId, ok := payload.Claims["sub"].(string)
	if !ok || googleId == "" {
		helpers.RespondWithError(w, 500, "invalid token: googleId")
		h.logger.Error("invalid token: googleId")
		return
	}
	email, ok := payload.Claims["email"].(string)
	if !ok || email == "" {
		helpers.RespondWithError(w, 500, "invalid token: email")
		h.logger.Error("invalid token: email")
		return
	}
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	deviceInfo := r.UserAgent()

	// ---------------------------------------------------------------------------------------------------
	// 2. Generate Refresh Token string (No DB ops yet)
	// We do this first so we can insert it in the transaction
	refreshToken, err := generateSecureRandomString(32)
	if err != nil {
		helpers.RespondWithError(w, 500, "err in generateSecureRandomString, err:"+err.Error())
		h.logger.Error("err in generateSecureRandomString, err:" + err.Error())
		return
	}
	hashedRandStr := hashToken(refreshToken)

	// ---------------------------------------------------------------------------------------------------
	// 3. Begin Transaction for atomic User + Token creation
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, 500, "Error starting transaction")
		h.logger.Error("Failed to begin transaction in GoogleAuthHandler", "error", err)
		return
	}
	defer tx.Rollback(r.Context()) // Rollback on any error

	qtx := h.db.WithTx(tx)

	// 3a. Upsert the user
	user, err := qtx.UpsertUserByEmail(r.Context(), db.UpsertUserByEmailParams{
		GoogleID: googleId,
		Email:    email,
		Picture: pgtype.Text{
			String: picture,
			Valid:  picture != "",
		},
		Name: pgtype.Text{
			String: name,
			Valid:  name != "",
		},
	})
	if err != nil {
		helpers.RespondWithError(w, 500, "error in UpsertUserByEmail,err:"+err.Error())
		h.logger.Error("error in tx UpsertUserByEmail", "error", err)
		return // Rollback is deferred
	}

	// 3b. Insert the refresh token
	if _, err := qtx.InsertRefreshToken(r.Context(), db.InsertRefreshTokenParams{
		UserID:    user.ID, // <-- Use the ID from the user we just upserted
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

	// 3c. Commit the transaction
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, 500, "Error committing transaction")
		h.logger.Error("Failed to commit transaction", "error", err)
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 4. Generate Access Token (Post-Transaction)
	// We do this last, only *after* we know the user and token are successfully in the DB.
	// We use the 'user' object returned from the transaction.
	accessToken, err := GenerateJwt(&Claims{
		UserId: h.uuidToString(user.ID),
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
	// 5. Send Response
	helpers.PrintResponse("GoogleAuthHandler response: ", resType{User: user, AccessToken: accessToken})

	helpers.RespondWithJSON(w, 201, resType{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken, // <-- Send the raw, unhashed token
	})
}
