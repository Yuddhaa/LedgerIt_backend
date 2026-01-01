package auth

import (
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/api/idtoken"
)

// GoogleAuthHandler handles the server-side validation of a Google ID token.
// It validates the token, upserts the user, creates a refresh token,
// and returns both an access and refresh token.
func (h *Handler) GoogleAuthHandler(w http.ResponseWriter, r *http.Request) {
	// req and res type
	type reqType struct {
		IdToken string `json:"id_token"`
	}
	type resType struct {
		User         db.UpsertUserByEmailRow `json:"user"`
		AccessToken  string                  `json:"access_token"`
		RefreshToken string                  `json:"refresh_token"`
	}

	// ---------------------------------------------------------------------------------------------------
	// 1. Decode and Validate (No DB ops yet)
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		helpers.LogInfo("GoogleAuthHandler", "failed to decode request body", "error", err.Error())
		return
	}

	payload, err := idtoken.Validate(r.Context(), body.IdToken, "")
	if err != nil {
		helpers.RespondWithError(w, http.StatusUnauthorized, "invalid ID token")
		helpers.LogInfo("GoogleAuthHandler", "failed to validate id token", "error", err.Error())
		return
	}

	// 3. MANUAL AUDIENCE CHECK [CRITICAL FIX]
	// Define all the Client IDs your backend should trust.
	// ideally, load these from your config/env variables
	trustedClientIDs := []string{
		configs.Configs.GOOGLE_WEB_CLIENT_ID, // Your existing Web ID
		configs.Configs.GOOGLE_ANDROID_ID,    // Add your Android Client ID here
		configs.Configs.GOOGLE_IOS_ID,        // Add your iOS Client ID here
		// If you are using Expo Go, it might have a specific ID too
	}

	isValidAudience := slices.Contains(trustedClientIDs, payload.Audience)

	if !isValidAudience {
		helpers.RespondWithError(w, http.StatusUnauthorized, "Token audience mismatch")
		helpers.LogInfo("GoogleAuthHandler", "audience mismatch",
			"token_aud", payload.Audience,
			"expected_one_of", trustedClientIDs)
		return
	}

	googleId, ok := payload.Claims["sub"].(string)
	if !ok || googleId == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "invalid token: missing googleId (sub) claim")
		helpers.LogInfo("GoogleAuthHandler", "invalid token: missing googleId (sub) claim")
		return
	}
	email, ok := payload.Claims["email"].(string)
	if !ok || email == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "invalid token: missing email claim")
		helpers.LogInfo("GoogleAuthHandler", "invalid token: missing email claim")
		return
	}
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	deviceInfo := r.UserAgent()

	// ---------------------------------------------------------------------------------------------------
	// 2. Generate Refresh Token string (No DB ops yet)
	refreshToken, err := generateSecureRandomString(32)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		// CHANGED: Use helper for structured logging. This is a real server error.
		helpers.LogError("GoogleAuthHandler", "err in generateSecureRandomString", "error", err.Error())
		return
	}
	hashedRandStr := hashToken(refreshToken)

	// ---------------------------------------------------------------------------------------------------
	// 3. Begin Transaction for atomic User + Token creation
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("GoogleAuthHandler", "Failed to begin transaction", "error", err.Error())
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
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("GoogleAuthHandler", "error in tx UpsertUserByEmail", "error", err.Error())
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
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("GoogleAuthHandler", "err in tx InsertRefreshToken", "error", err.Error())
		return // Rollback is deferred
	}

	// 3c. Commit the transaction
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("GoogleAuthHandler", "Failed to commit transaction", "error", err.Error())
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 4. Generate Access Token (Post-Transaction)
	accessToken, err := GenerateJwt(&Claims{
		UserId: h.uuidToString(user.ID),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "LedgerIt",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}, configs.Configs.JWT_SECRET)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("GoogleAuthHandler", "error in signing the jwt token", "error", err.Error())
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 5. Send Response
	helpers.LogInfo("GoogleAuthHandler", "response", "user_id", user.ID, "username", user.Name.String)
	helpers.RespondWithJSON(w, 201, resType{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken, // <-- Send the raw, unhashed token
	})
}
