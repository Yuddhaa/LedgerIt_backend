package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers" // <-- ADDED THIS IMPORT

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Claims represents the data stored within the JWT.
type Claims struct {
	UserId string `json:"userId"`
	jwt.RegisteredClaims
}

// Define a custom context key type to avoid collisions
type contextKey string

// userClaimsKey is the key we'll use to store and retrieve user Claims in the request context.
const userClaimsKey contextKey = "userClaims"

// Handler holds dependencies for auth-related HTTP handlers.
type Handler struct {
	logger         *slog.Logger
	db             *db.Queries
	pool           *pgxpool.Pool
	googleClientId string
	jwtSecret      string
}

// NewHandler creates a new auth Handler, validating required environment variables.
func NewHandler(db *db.Queries, pool *pgxpool.Pool, logger *slog.Logger) (*Handler, error) {
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		// This is a fatal startup error, log it as such.
		helpers.LogError("auth.NewHandler", "GOOGLE_CLIENT_ID is not set in environment")
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// This is also a fatal startup error.
		helpers.LogError("auth.NewHandler", "JWT_SECRET is not set in environment")
		return nil, fmt.Errorf("JWT_SECRET is not set")
	}
	return &Handler{
		logger:         logger,
		db:             db,
		pool:           pool,
		googleClientId: googleClientID,
		jwtSecret:      jwtSecret,
	}, nil
}

// Routes defines and returns all auth-related routes, applying middleware as needed.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// --- Public Auth Routes ---
	// These routes do NOT have the JWT middleware
	r.Post("/google/signin", h.GoogleAuthHandler)
	r.Post("/refresh", h.RefreshHandler)

	// --- Protected Auth Routes ---
	// These routes WILL require a valid JWT
	r.Group(func(r chi.Router) {
		// Apply our JwtAuthMiddleware *only* to this group
		r.Use(h.JwtAuthMiddleware)

		r.Post("/logout", h.LogoutHandler)
		r.Post("/logoutall", h.LogoutAllHandler)
	})

	return r
}

// --- Helper Functions ---

// GenerateJwt creates a signed JWT token using the provided claims and secret key.
// It returns the generated JWT string or an error if signing fails.
func GenerateJwt(jwtClaims *Claims, jwtSecret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	jwt, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		// This is a server-side error during the token signing process.
		helpers.LogError("auth.GenerateJwt", "failed to sign JWT", "error", err)
		return "", err
	}
	return jwt, nil
}

// JwtAuthMiddleware is a middleware that verifies a JWT token.
// If the token is valid, it extracts the claims and stores them in the
// request context for downstream handlers.
func (h *Handler) JwtAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// This is a client error, not a server error. Log as Info.
			helpers.LogInfo("auth.JwtAuthMiddleware", "Auth failed: Authorization header missing")
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// 2. Validate the format ("Bearer <token>")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			// Client error.
			helpers.LogInfo("auth.JwtAuthMiddleware", "Auth failed: Invalid Authorization header format")
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}
		tokenString := parts[1]

		// 3. Parse and validate the token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Security check: Make sure the token's signing method is what we expect
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				// This is a potential security issue (e.g., alg-none attack). Log as Error.
				alg := token.Header["alg"]
				helpers.LogError("auth.JwtAuthMiddleware", "Unexpected JWT signing method", "algorithm", alg)
				return nil, fmt.Errorf("unexpected signing method: %v", alg)
			}
			// Return the secret key
			return []byte(h.jwtSecret), nil
		})
		// 4. Handle parsing errors
		if err != nil {
			// Client error (e.g., expired token, malformed token). Log as Info.
			helpers.LogInfo("auth.JwtAuthMiddleware", "Auth failed: Token parsing error", "error", err.Error())
			if errors.Is(err, jwt.ErrTokenExpired) {
				http.Error(w, "Token has expired", http.StatusUnauthorized)
			} else {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
			}
			return
		}

		// 5. Final check for token validity
		if !token.Valid {
			// Client error.
			helpers.LogInfo("auth.JwtAuthMiddleware", "Auth failed: Invalid token")
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// 6. Success! Store claims in context for downstream handlers
		ctx := context.WithValue(r.Context(), userClaimsKey, claims)

		// 7. Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserIdFromContext extract and returns user id form context
func GetUserIdFromContext(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	claims, ok := GetClaimsFromContext(r.Context())
	if !ok {
		helpers.RespondWithError(w, http.StatusInternalServerError, "could not retrieve claims from context")
		helpers.LogError("GetUserIdFromContext", "could not retrieve claims from context")
		return pgtype.UUID{}, false
	}
	// Get User ID and convert
	userId := claims.UserId
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("GetUserIdFromContext", "error in converting userId to uuid", "error", err, "user_id_from_claim", userId)
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{
		Bytes: userUuid,
		Valid: userUuid != uuid.Nil,
	}, true
}

// ExtractUUID extracts uuids from the path variable
func ExtractUUID(w http.ResponseWriter, r *http.Request, variable string) (pgtype.UUID, bool) {
	uuidStr := chi.URLParam(r, variable)
	UUID, err := uuid.Parse(uuidStr)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Url Param")
		helpers.LogError("ExtractUUID", "bad url param:"+variable, "err", err.Error())
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{
		Bytes: UUID,
		Valid: UUID != uuid.Nil,
	}, true
}

// GetClaimsFromContext is a helper function to safely retrieve claims
// from the request context.
func GetClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(*Claims)
	return claims, ok
}

// uuidToString converts a pgtype.UUID to its string representation.
// If the UUID is invalid, it returns an empty string.
func (h *Handler) uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}

// generateSecureRandomString creates a cryptographically secure, random string
// encoded as hex.
func generateSecureRandomString(n int) (string, error) {
	if n <= 0 {
		// This is a programmer error (calling the func incorrectly).
		helpers.LogError("auth.generateSecureRandomString", "n must be positive", "n_value", n)
		return "", fmt.Errorf("number of bytes (n) must be positive")
	}

	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// This is a rare, critical system-level error.
		helpers.LogError("auth.generateSecureRandomString", "failed to read from crypto/rand", "error", err)
		return "", fmt.Errorf("failed to read from crypto/rand: %w", err)
	}

	return hex.EncodeToString(b), nil
}

// hashToken creates a SHA-256 hash of a given string (e.g., a refresh token).
// This is for fast, one-way storage, not for password hashing.
func hashToken(token string) string {
	hashBytes := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hashBytes[:])
}
