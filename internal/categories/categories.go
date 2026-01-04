package categories

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

	// for admins and creators
	r.Post("/", h.AddCategoryHandler)
	r.Put("/{category_id}", h.UpdateCategoryHandler)
	r.Delete("/{category_id}", h.DeleteCategoryHandler)

	// for members only
	r.Get("/{category_id}", h.GetACategoryHandler)
	r.Get("/", h.GetAllCategoriesHandler)

	return r
}
