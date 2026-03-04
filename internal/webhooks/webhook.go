package webhooks

import (
	"fmt"
	"os"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/razorpay/razorpay-go"
)

type Handler struct {
	db                    *db.Queries
	pool                  *pgxpool.Pool
	razorpayWebhookSecret string
	rp_client             *razorpay.Client
}

func NewHandler(db *db.Queries, pool *pgxpool.Pool, rp_client *razorpay.Client) (*Handler, error) {
	razorpayWebhookSecret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	if razorpayWebhookSecret == "" {
		helpers.LogError("webhooks.NewHandler", "RAZORPAY_WEBHOOK_SECRET is not set in environment")
		return nil, fmt.Errorf("RAZORPAY_WEBHOOK_SECRET is not set")
	}
	return &Handler{
		db:                    db,
		pool:                  pool,
		razorpayWebhookSecret: razorpayWebhookSecret,
		rp_client:             rp_client,
	}, nil
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/razorpay", h.RazorpayWebhooks)
	return r
}
