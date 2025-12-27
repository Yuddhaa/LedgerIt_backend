package subscriptions

import (
	"context"
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
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
	r.Use(h.GetRole)
	return r
}

// Getrole returns int corresponding to the role as below
// -1 - err
// 0 - not a member => for these 2 automatically the middleware returns respective status code
// 1 - admin/creator
// 2 - employee
func (h *Handler) GetRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roleInt := -1
		userId, ok := auth.GetUserIdFromContext(w, r)
		if !ok {
			return
		}
		businessId, ok := auth.ExtractUUID(w, r, "id")
		if !ok {
			return
		}
		role, err := h.db.GetUserRole(r.Context(), db.GetUserRoleParams{
			UserID:     userId,
			BusinessID: businessId,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				helpers.RespondWithError(w, http.StatusUnauthorized, "no user found")
				helpers.LogInfo("GetRole", "no user found", "userId", userId, "businessId", businessId)
				roleInt = 0
				return
			}
			helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
			helpers.LogError("getrole", "db error in GetUserRole", "err", err.Error(), "userId", userId, "businessId", businessId)
			roleInt = -1
			return
		}
		if role == db.BusinessRoleEmployee {
			roleInt = 2
		} else {
			roleInt = 1
		}

		ctx := context.WithValue(r.Context(), "role", roleInt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
