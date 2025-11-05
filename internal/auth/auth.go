package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/api/idtoken"
)

type Handler struct {
	logger            *slog.Logger
	db                *db.Queries
	googleWebClientId string
	jwtSecret         string
}

func NewHandler(logger *slog.Logger, db *db.Queries) *Handler {
	return &Handler{
		logger:            logger,
		db:                db,
		googleWebClientId: os.Getenv("GOOGLE_CLIENT_ID"),
		jwtSecret:         os.Getenv("JWT_SECRET"),
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8000"}, // Your frontend
		AllowedMethods:   []string{"POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300, // Optional
	}))
	r.Post("/google/signin", h.GoogleAuthHandler)
	return r
}

type Claims struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	jwt.RegisteredClaims
}

func (h *Handler) GoogleAuthHandler(w http.ResponseWriter, r *http.Request) {
	// req and res type
	type reqType struct {
		IdToken string `json:"idToken"`
	}
	type resType struct {
		User db.UpsertUserByEmailRow `json:"user"`
		JWT  string                  `json:"jwt"`
	}
	// decode the r.Body
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request")
		return
	}
	helpers.PrintResponse("GoogleAuthHandler req.body:", body)
	// validate the idtoken in req using idtoken package
	payload, err := idtoken.Validate(r.Context(), body.IdToken, h.googleWebClientId)
	if err != nil {
		http.Error(w, "Error: "+err.Error(), http.StatusUnauthorized)
		h.logger.Error("error in validating id token. err:" + err.Error())
		return
	}

	// type assertions of the payload
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
	// name and picture are optional
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	// check if user exist if not create one.
	user, err := h.db.UpsertUserByEmail(r.Context(), db.UpsertUserByEmailParams{
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
		h.logger.Error("error in UpsertUserByEmail,err:" + err.Error())
		return
	}

	// generate jwt
	jwtClaims := &Claims{
		Email:   email,
		Name:    name,
		Picture: picture,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "LedgerIt",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	jwt, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Error: "+err.Error())
		h.logger.Error("error in signing the jwt token, err:" + err.Error())
		return
	}

	helpers.PrintResponse("GoogleAuthHandler response: ", resType{User: user, JWT: jwt})

	// send response
	helpers.RespondWithJSON(w, 200, resType{
		User: user,
		JWT:  jwt,
	})
}
