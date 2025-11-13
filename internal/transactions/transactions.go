package transactions

import (
	"net/http"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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
	r.Post("/", h.AddTransactionsHandler)
	return r
}

func (h *Handler) ExtractBusinessUUID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	businessid := chi.URLParam(r, "id")
	businessUUID, err := uuid.Parse(businessid)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Url Param")
		helpers.LogError("ExtractBusinessUUID", "bad url param", "Err", err)
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{
		Bytes: businessUUID,
		Valid: businessUUID != uuid.Nil,
	}, true
}
