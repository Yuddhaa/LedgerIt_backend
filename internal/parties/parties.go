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

	r.Group(func(r chi.Router) {
		r.Use(h.CheckMember)
		// for all of people in a business
		r.Get("/{party_id}", h.GetAPartyHandler)
		r.Get("/", h.GetPartyHandler)
		r.Get("/places", h.GetPlacesHandler)
	})
	return r
}

// --- middlerware and helper funcitons

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

// CheckMember checks if the user is a member of the business.
// This is the base-level check for all routes in this package.
func (h *Handler) CheckMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId, ok := auth.GetUserIdFromContext(w, r)
		if !ok {
			return
		}
		businessId, ok := auth.ExtractUUID(w, r, "id")
		if !ok {
			return
		}

		// Use the new CheckMember query
		_, err := h.db.CheckMember(r.Context(), db.CheckMemberParams{
			UserID:     userId,
			BusinessID: businessId,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// 403 Forbidden is more accurate than 401
				// 401 = "I don't know who you are"
				// 403 = "I know who you are, and you aren't allowed"
				helpers.RespondWithError(w, http.StatusForbidden, "You are not a member of this business")
				helpers.LogInfo("CheckMember", "Non-member access attempt", "userId", userId, "businessId", businessId)
				return
			}
			helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
			helpers.LogError("CheckMember", "Internal server error", "err", err.Error(), "userId", userId, "businessId", businessId)
			return
		}

		// User is a member, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}
