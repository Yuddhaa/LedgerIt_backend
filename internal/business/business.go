package business

import (
	"errors"
	"log/slog"

	"LedgerIt/internal/db"

	"github.com/go-chi/chi/v5"
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
		r.Post("/members", h.AddMemberHandler)
		r.Get("/members", h.GetBusinessMembers)
		r.Get("/balance", h.GetBalance)
	})
	return r
}

// --- Internal Helper Functions ---

// isUniqueViolation is a helper function to check for a PostgreSQL unique_violation error (code 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
