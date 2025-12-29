package webhooks

import (
	"fmt"
	"os"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db                    *db.Queries
	pool                  *pgxpool.Pool
	razorpayWebhookSecret string
}

func NewHandler(db *db.Queries, pool *pgxpool.Pool) (*Handler, error) {
	razorpayWebhookSecret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	if razorpayWebhookSecret == "" {
		helpers.LogError("webhooks.NewHandler", "RAZORPAY_WEBHOOK_SECRET is not set in environment")
		return nil, fmt.Errorf("RAZORPAY_WEBHOOK_SECRET is not set")
	}
	return &Handler{
		db:   db,
		pool: pool,
	}, nil
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/razorpay", h.RazorpayHandler)
	return r
}
