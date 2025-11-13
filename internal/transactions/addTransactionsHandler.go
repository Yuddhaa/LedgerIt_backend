package transactions

import (
	"net/http"

	"LedgerIt/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) AddTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	businessUuid, ok := h.ExtractBusinessUUID(w, r)
	if !ok {
		return
	}

	type reqType struct {
		Amount      float64                     `json:"amount"`
		Direction   db.NullTransactionDirection `json:"direction"`
		Description pgtype.Text                 `json:"description"`
		PartyID     pgtype.UUID                 `json:"party_id"`
		Mode        db.NullTransactionMode      `json:"mode"`
		CategoryID  pgtype.Int4                 `json:"category_id"`
	}
	type resType struct{}
	transaction, err := h.db.CreateTransaction(r.Context(), db.CreateTransactionParams{
		BusinessID: businessUuid,
		Amount:     pgtype.Numeric{},
	})
}
