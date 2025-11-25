package transactions

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	db *db.DBStore
}

func NewHandler(db *db.DBStore) *Handler {
	return &Handler{
		db: db,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(h.CheckAdmin)
	})

	// for all of people in a business
	r.Group(func(r chi.Router) {
		r.Use(h.CheckMember)
		r.Post("/", h.AddTransactionsHandler)
	})
	return r
}

// --- middlerware and helper funcitons

// TransactionAuthMiddleware ba
func (h *Handler) TransactionAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}

// Getrole returns int corresponding to the role as below
// -1 - err
// 0 - not a member
// 1 - admin/creator
// 2 - employee
func (h *Handler) GetRole(w http.ResponseWriter, r *http.Request, businessId, userId pgtype.UUID) int {
	role, err := h.db.GetUserRole(r.Context(), db.GetUserRoleParams{
		UserID:     userId,
		BusinessID: businessId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusUnauthorized, "no user found")
			helpers.LogInfo("GetRole", "no user found", "userId", userId, "businessId", businessId)
			return 0
		}
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		helpers.LogError("getrole", "db error in GetUserRole", "err", err, "userId", userId, "businessId", businessId)
		return -1
	}
	if role == db.BusinessRoleEmployee {
		return 2
	}
	return 1
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
			helpers.LogError("CheckMember", "Internal server error", "err", err, "userId", userId, "businessId", businessId)
			return
		}

		// User is a member, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}
