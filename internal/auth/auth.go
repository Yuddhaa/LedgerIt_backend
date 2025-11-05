package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"LedgerIt/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Claims struct {
	UserId string `json:"userId"`
	jwt.RegisteredClaims
}

// Define a custom context key type to avoid collisions
type contextKey string

// userClaimsKey is the key we'll use to store and retrieve user Claims in the request context.
const userClaimsKey contextKey = "userClaims"

type Handler struct {
	logger            *slog.Logger
	db                *db.Queries
	pool              *pgxpool.Pool
	googleWebClientId string
	jwtSecret         string
}

func NewHandler(db *db.Queries, pool *pgxpool.Pool, logger *slog.Logger) (*Handler, error) {
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is not set")
	}
	return &Handler{
		logger:            logger,
		db:                db,
		pool:              pool,
		googleWebClientId: googleClientID,
		jwtSecret:         jwtSecret,
	}, nil
}

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

// helper funcitons below
// GenerateJwt creates a signed JWT token using the provided claims and secret key.
// It returns the generated JWT string or an error if signing fails.
func GenerateJwt(jwtClaims *Claims, jwtSecret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	jwt, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
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
			h.logger.Warn("Auth failed: Authorization header missing")
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// 2. Validate the format ("Bearer <token>")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			h.logger.Warn("Auth failed: Invalid Authorization header format")
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}
		tokenString := parts[1]

		// 3. Parse and validate the token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Security check: Make sure the token's signing method is what we expect
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Return the secret key
			return []byte(h.jwtSecret), nil
		})
		// 4. Handle parsing errors
		if err != nil {
			h.logger.Warn("Auth failed: Token parsing error", "error", err)
			if err == jwt.ErrTokenExpired {
				http.Error(w, "Token has expired", http.StatusUnauthorized)
			} else {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
			}
			return
		}

		// 5. Final check for token validity
		if !token.Valid {
			h.logger.Warn("Auth failed: Invalid token", "token", tokenString)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// 6. Success! Store claims in context for downstream handlers
		ctx := context.WithValue(r.Context(), userClaimsKey, claims)

		// 7. Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
//
// We use crypto/rand, which is a cryptographically secure pseudorandom number
// generator, as opposed to math/rand, which is not secure.
//
// n: The number of random bytes to generate.
// The resulting hex string will be 2*n characters long.
// For example, n=32 bytes will produce a 64-character string.
func generateSecureRandomString(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("number of bytes (n) must be positive")
	}

	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// This is a rare error, indicating a problem with the OS's entropy source.
		return "", fmt.Errorf("failed to read from crypto/rand: %w", err)
	}

	// hex.EncodeToString is fast, URL-safe, and doubles the length
	// of the byte slice.
	return hex.EncodeToString(b), nil
}

// hashToken creates a SHA-256 hash of a given string.
//
// We use SHA-256 (not bcrypt) because the input token is already a high-entropy
// random string. We don't need a slow, "password-stretching" algorithm like
// bcrypt. We just need a fast, one-way hash to store in the database
// so we aren't storing the raw token in plaintext.
func hashToken(token string) string {
	// Create a new SHA-256 hash object
	// sha256.Sum256 returns a [32]byte array
	hashBytes := sha256.Sum256([]byte(token))

	// Convert the byte array to a hex string
	// We use hashBytes[:] to get a slice from the array
	return hex.EncodeToString(hashBytes[:])
}
