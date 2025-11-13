package parties

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
	return r
}

// --- middlerware and helper funcitons

// ExtractUUID extracts uuids from the path variable
func (h *Handler) ExtractUUID(w http.ResponseWriter, r *http.Request, variable string) (pgtype.UUID, bool) {
	uuidStr := chi.URLParam(r, variable)
	UUID, err := uuid.Parse(uuidStr)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Url Param")
		helpers.LogError("ExtractBusinessUUID", "bad url param", "Err", err)
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{
		Bytes: UUID,
		Valid: UUID != uuid.Nil,
	}, true
}
