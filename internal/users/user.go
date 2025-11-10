package users

import (
	"log/slog"

	"LedgerIt/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler holds all dependencies for the user-related HTTP handlers.
type Handler struct {
	db     *db.Queries
	pool   *pgxpool.Pool
	logger *slog.Logger // Base file logger
}

// NewHandler creates a new instance of the user Handler.
func NewHandler(db *db.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		pool:   pool,
		logger: logger,
	}
}

// Routes defines and returns all routes for the user package.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	// All routes in this package are protected and require a valid JWT.
	// The '/me' route refers to the authenticated user.
	r.Put("/me", h.UpdateUserProfileHandler)
	r.Get("/{phone_no}", h.GetUserByPhone)
	return r
}
