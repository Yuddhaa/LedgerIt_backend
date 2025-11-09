package business

import (
	"errors"
	"log/slog"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler holds all dependencies for the business-related HTTP handlers.
type Handler struct {
	db     *db.Queries
	pool   *pgxpool.Pool
	logger *slog.Logger // Base file logger
}

// NewHandler creates a new instance of the business Handler.
func NewHandler(db *db.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		pool:   pool,
		logger: logger,
	}
}

// Routes defines and returns all routes for the business package.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.CreateBusinessHandler)
	r.Get("/", h.GetAllBusinessHandler)
	r.Get("/own", h.GetOwnedBusinessHandler)

	// Routes that require a business ID
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.GetBusinessHandler)
		r.Post("/add", h.AddMemberHandler)
	})
	return r
}

// --- Internal Helper Functions ---

// getUserIDFromContext is an internal helper to centralize extracting
// and parsing the UserID from the request context.
func (h *Handler) getUserIDFromContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		helpers.RespondWithError(w, http.StatusInternalServerError, "could not retrieve claims from context")
		// CHANGED: This is a server error; middleware should guarantee claims.
		helpers.LogError("getUserIDFromContext", "could not retrieve claims from context")
		return uuid.Nil, false
	}

	userId, err := uuid.Parse(claims.UserId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		// CHANGED: Claims are malformed, this is a server bug. Don't leak error.
		helpers.LogError("getUserIDFromContext", "could not parse userId from claims", "error", err, "claim_user_id", claims.UserId)
		return uuid.Nil, false
	}

	return userId, true
}

// isUniqueViolation is a helper function to check for a PostgreSQL unique_violation error (code 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
