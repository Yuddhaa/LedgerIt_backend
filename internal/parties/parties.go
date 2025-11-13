package parties

import (
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
	// protected routes, only for admin or creator
	r.Group(func(r chi.Router) {
		r.Use(h.CheckAdmin)

		r.Post("/", h.AddPartyHandler)
		r.Put("/{party_id}", h.UpdatePartyHandler)
		r.Delete("/{party_id}", h.DeletePartyHandler)
	})
	// for all of people in a business
	r.Get("/{party_id}", h.GetAPartyHandler)
	r.Get("/", h.GetPartyHandler)
	r.Get("/places", h.GetPlacesHandler)
	return r
}

// --- middlerware and helper funcitons

// GetPartyHandler is kind of a helper handler funciton
// based on the presence of a query parameter this calls the actual handler funcs
func (h *Handler) GetPartyHandler(w http.ResponseWriter, r *http.Request) {
	place := r.URL.Query().Get("place")

	if place != "" {
		// Handle case: GET /?place=...
		h.GetPlacePartyHandler(w, r)
		return
	}

	// Default: GET /
	h.GetAllPartyHandler(w, r)
}

// checkAdmin checks if the user is admin or creator or not..
// if the user is admin/creator flow is moved forward otherwise StatusUnauthorized is returned
func (h *Handler) CheckAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId, ok := auth.GetUserIdFromContext(w, r)
		if !ok {
			return
		}
		businessId, ok := auth.ExtractUUID(w, r, "id")
		if !ok {
			return
		}
		_, err := h.db.CheckAdmin(r.Context(), db.CheckAdminParams{
			UserID:     userId,
			BusinessID: businessId,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				helpers.RespondWithError(w, http.StatusUnauthorized, "Not an admin or creator")
				helpers.LogInfo("CheckAdmin", "Not an admin or creator", "userId", userId, "businessId", businessId)
				return
			}
			helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
			helpers.LogError("CheckAdmin", "Internal server error", "userId", userId, "businessId", businessId)
			return
		}
		next.ServeHTTP(w, r)
	})
}
