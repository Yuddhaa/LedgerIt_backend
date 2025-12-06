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
	"github.com/jackc/pgx/v5"
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
	r.Use(h.GetRole)
	r.Post("/", h.AddTransactionsHandler)
	r.Get("/", h.GetTransactionsHandler)
	r.Get("/{tran_id}", h.GetSingleTransactionsHandler)
	r.Patch("/{tran_id}", h.UpdateTransactionHandler)
	// approvals related
	r.Get("/approvals", h.GetApprovalsHandler)
	return r
}

// tranReqType used for add and update transaction
type tranReqType struct {
	Amount      string                  `json:"amount"`
	Direction   db.TransactionDirection `json:"direction"`
	CategoryID  string                  `json:"category_id"`
	PartyID     string                  `json:"party_id"`
	Mode        db.TransactionMode      `json:"mode"`
	ReceiptNo   string                  `json:"receipt_no"`
	Description string                  `json:"description"`
	Reason      string                  `json:"reason,omitempty"`
}

// --- middlerware and helper funcitons

// TransactionAuthMiddleware
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

// parseDate is used to parse date for get transactions and edit requests
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
