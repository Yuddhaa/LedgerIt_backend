package transactions

import (
	"context"
	"errors"
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
		// r.Use(h.CheckMember)
		// GetRole automatically returns 403 for non members
		r.Use(h.GetRole)
		r.Post("/", h.AddTransactionsHandler)
		r.Get("/", h.GetTransactionsHandler)
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
			helpers.LogError("getrole", "db error in GetUserRole", "err", err, "userId", userId, "businessId", businessId)
			roleInt = -1
			return
		}
		if role == db.BusinessRoleEmployee {
			roleInt = 2
		}
		roleInt = 1

		ctx := context.WithValue(r.Context(), "role", roleInt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

func parseDate(w http.ResponseWriter, paramName, dateStr string) (time.Time, bool) {
	if dateStr == "" {
		return time.Time{}, true // Return zero time if empty
	}

	// Layout for YYYY-MM-DD
	layout := "2006-01-02"
	// layout := "2006-01-02 15:04:05.999999+00"
	parsedTime, err := time.Parse(layout, dateStr)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Invalid date format for "+paramName+". Use YYYY-MM-DD")
		helpers.LogError("parseDate", "date parse error", "err", err.Error())
		return time.Time{}, false
	}
	helpers.PrintJson("parsedTime", parsedTime)
	return parsedTime, true
}

func convertToUUID(w http.ResponseWriter, name, uuidStr string) (pgtype.UUID, bool) {
	if uuidStr == "" {
		return pgtype.UUID{
			Valid: false,
		}, true
	}
	UUID, err := uuid.Parse(uuidStr)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Url query")
		helpers.LogError("convertToUUID", "bad query parameter:"+name, "Err", err.Error())
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{
		Bytes: UUID,
		Valid: UUID != uuid.Nil,
	}, true
}
