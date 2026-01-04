package parties

import (
	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db   *db.Queries
	pool *pgxpool.Pool
}

func NewHandler(db *db.Queries, pool *pgxpool.Pool) *Handler {
	return &Handler{
		db:   db,
		pool: pool,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Use(auth.GetRole(h.db))
	r.Use(auth.ValidatePlan(h.db))

	// for admins | creators
	r.Post("/", h.AddPartyHandler)
	r.Put("/{party_id}", h.UpdatePartyHandler)
	r.Delete("/{party_id}", h.DeletePartyHandler)

	// for all members
	r.Get("/{party_id}", h.GetAPartyHandler)
	r.Get("/", h.GetPartyHandler)
	r.Get("/places", h.GetPlacesHandler)

	return r
}
