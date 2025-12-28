package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	razorpay "github.com/razorpay/razorpay-go"
)

type Handler struct {
	db        *db.Queries
	pool      *pgxpool.Pool
	rp_client *razorpay.Client
}

func NewHandler(db *db.Queries, pool *pgxpool.Pool) (*Handler, error) {
	razorpayKey := os.Getenv("RAZORPAY_API_KEY")
	if razorpayKey == "" {
		helpers.LogError("subscriptions.NewHandler", "RAZORPAY_API_KEY is not set in environment")
		return nil, fmt.Errorf("RAZORPAY_API_KEY is not set")
	}
	razorpaySecret := os.Getenv("RAZORPAY_API_SECRET")
	if razorpaySecret == "" {
		helpers.LogError("subscriptions.NewHandler", "RAZORPAY_API_SECRET is not set in environment")
		return nil, fmt.Errorf("RAZORPAY_API_SECRET is not set")
	}
	rp_client := razorpay.NewClient(razorpayKey, razorpaySecret)
	return &Handler{
		db:        db,
		pool:      pool,
		rp_client: rp_client,
	}, nil
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.GetRole)
	r.Post("/create", h.CreateHandler)
	return r
}

// Getrole returns int corresponding to the role as below
// -1 - err
// 0 - not a member => for these 2 automatically the middleware returns respective status code
// 1 - creator
// 2 - employee
// 3 - admin
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
			helpers.LogError("getrole", "db error in GetUserRole", "error", err.Error(), "userId", userId, "businessId", businessId)
			roleInt = -1
			return
		}
		switch role {
		case db.BusinessRoleEmployee:
			roleInt = 2 // employee
		case db.BusinessRoleAdmin:
			roleInt = 3 // admin
		default:
			roleInt = 1 // creator
		}

		ctx := context.WithValue(r.Context(), "role", roleInt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
