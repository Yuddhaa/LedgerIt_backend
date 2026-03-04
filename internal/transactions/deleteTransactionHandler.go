package transactions

import (
	"errors"
	"fmt"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) DeleteTransactionHandler(w http.ResponseWriter, r *http.Request) {
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role == 2 {
		helpers.RespondWithError(w, http.StatusUnauthorized, "employee can't delete a transaction")
		helpers.LogError("DeleteTransactionHandler", "employee can't delete a transaction")
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	tranId, ok := auth.ExtractUUID(w, r, "tran_id")
	if !ok {
		return
	}
	//*****************************************************************************************************************
	// start transaction
	//*****************************************************************************************************************
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("DeleteTransactionHandler", "error in pool.Begin", "err", err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	// Create QTX (Queries bound to this transaction)
	qtx := h.db.WithTx(tx)
	//*****************************************************************************************************************
	// delete old transaction while returning the deleted transaction
	//*****************************************************************************************************************
	oldTran, err := qtx.DeleteTransaction(r.Context(), db.DeleteTransactionParams{
		ID:         tranId,
		BusinessID: businessId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.LogError("DeleteTransactionHandler", "no transaction found", "tranId", tranId)
			helpers.RespondWithError(w, http.StatusNotFound, "no transaction found")
			return
		}
		helpers.LogError("DeleteTransactionHandler", "db error in DeleteTransaction", "err", err.Error())
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	//*****************************************************************************************************************
	// update the memeber balance
	//*****************************************************************************************************************
	var oldAmout float64
	float64, _ := oldTran.Amount.Float64Value()
	if oldTran.Direction == "in" { // minus
		oldAmout = -1 * float64.Float64
	} else { // plus
		oldAmout = float64.Float64
	}
	var deltaNumeric pgtype.Numeric
	if err := deltaNumeric.Scan(fmt.Sprintf("%.2f", oldAmout)); err != nil {
		helpers.LogError("DeleteTransactionHandler", "deltaNumeric scan error", "err", err.Error())
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Note: We update the balance of the ORIGINAL creator (oldTran.UserID)
	err = qtx.UpdateBusinessMemberBalance(r.Context(), db.UpdateBusinessMemberBalanceParams{
		CurrentBalance: deltaNumeric, // Using Amount based on SQL logic (+ $1)
		UserID:         oldTran.UserID,
		BusinessID:     businessId,
	})
	if err != nil {
		helpers.LogError("DeleteTransactionHandler", "balance update failed", "err", err.Error())
		helpers.RespondWithError(w, http.StatusInternalServerError, "Balance update failed")
		return
	}
	//*****************************************************************************************************************
	// commit
	//*****************************************************************************************************************
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("DeleteTransactionHandler", "error in tx.commit", "err", err.Error())
		return
	}
	//*****************************************************************************************************************
	// respond
	//*****************************************************************************************************************
	w.WriteHeader(http.StatusNoContent)
}
