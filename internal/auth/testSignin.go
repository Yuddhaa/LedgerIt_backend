package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestSignin handles the server-side validation of a test user
// It validates the token, upserts the user, creates a refresh token,
// and returns both an access and refresh token.
func (h *Handler) TestSignin(w http.ResponseWriter, r *http.Request) {
	// req and res type
	type reqType struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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

	// **************************************************************************************************************
	// validate test email and pwd
	// **************************************************************************************************************
	test_email := os.Getenv("TEST_EMAIL")
	test_pwd := os.Getenv("TEST_PWD")
	if test_email == "" || test_pwd == "" {
		helpers.RespondWithError(w, http.StatusInternalServerError, "missing test pwd or email in env")
		helpers.LogInfo("TestSignin", "missing test pwd or email in env")
		return
	}
	if body.Email != test_email {
		helpers.RespondWithError(w, http.StatusUnauthorized, "invalid test email")
		helpers.LogInfo("TestSignin", "invalid test email")
		return
	}
	if body.Password != test_pwd {
		helpers.RespondWithError(w, http.StatusUnauthorized, "invalid test password")
		helpers.LogInfo("TestSignin", "invalid test password")
		return
	}

	// **************************************************************************************************************
	// get test.yuddhaa@gmail.com acc's details
	// **************************************************************************************************************
	googleId := os.Getenv("TEST_USER_GOOGLE_ID")
	if googleId == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "invalid token: missing googleId environment")
		helpers.LogInfo("TestSignin", "invalid token: missing googleId (sub) claim")
		return
	}
	email := os.Getenv("TEST_USER_EMAIL")
	if email == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "invalid token: missing email  in environment")
		helpers.LogInfo("TestSignin", "invalid token: missing email environment")
		return
	}
	name := os.Getenv("TEST_USER_NAME")
	if name == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "invalid token: missing name  in environment")
		helpers.LogInfo("TestSignin", "invalid token: missing name environment")
		return
	}
	picture := os.Getenv("TEST_USER_PICTURE")
	if picture == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "invalid token: missing picture  in environment")
		helpers.LogInfo("TestSignin", "invalid token: missing picture env")
		return
	}
	deviceInfo := r.UserAgent()

	// ---------------------------------------------------------------------------------------------------
	// 2. Generate Refresh Token string (No DB ops yet)
	refreshToken, err := generateSecureRandomString(32)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		// CHANGED: Use helper for structured logging. This is a real server error.
		helpers.LogError("TestSignin", "err in generateSecureRandomString", "error", err.Error())
		return
	}
	hashedRandStr := hashToken(refreshToken)

	// ---------------------------------------------------------------------------------------------------
	// 3. Begin Transaction for atomic User + Token creation
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		// CHANGED: Use helper for structured logging.
		helpers.LogError("TestSignin", "Failed to begin transaction", "error", err.Error())
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
		helpers.LogError("TestSignin", "error in tx UpsertUserByEmail", "error", err.Error())
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
		helpers.LogError("TestSignin", "err in tx InsertRefreshToken", "error", err.Error())
		return // Rollback is deferred
	}

	// 3c. Commit the transaction
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("TestSignin", "Failed to commit transaction", "error", err.Error())
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
		helpers.LogError("TestSignin", "error in signing the jwt token", "error", err.Error())
		return
	}

	// ---------------------------------------------------------------------------------------------------
	// 5. Send Response
	helpers.LogInfo("TestSignin", "response", "user_id", user.ID, "username", user.Name.String)
	helpers.RespondWithJSON(w, 201, resType{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken, // <-- Send the raw, unhashed token
	})
}
