package transactions

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

// GetSingleTransactionsHandler returns a single transaction based on the "tran_id" path parameter
func (h *Handler) GetSingleTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Transaction db.Transaction `json:"transaction"`
	}
	// **********************************************8
	// get the required ids
	// **********************************************8
	transactionId, ok := auth.ExtractUUID(w, r, "tran_id")
	if !ok {
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	filters := db.FilterParams{
		TransactionId: transactionId,
		BusinessID:    businessId,
	}
	// **********************************************8
	// db call
	// **********************************************8
	transaction, err := h.db.GetSingleTransaction(r.Context(), filters)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "transaction not found")
			helpers.LogInfo("GetSingleTransactionsHandler", "transaction not found", "transactionId", transactionId, "businessId", businessId)
			return
		}

		// All other errors are actual server errors
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogInfo("GetSingleTransactionsHandler", "db error in GetSingleTransaction", "transactionId", transactionId, "businessId", businessId)
		return
	}
	userId, ok := auth.GetUserIdFromContext(w, r)
	// **********************************************8
	// if user is employee, return transaction only if transaction.UserId is same as loggedin userid
	// **********************************************8
	var role int
	if role, ok = auth.GetUserRoleFromContext(w, r); !ok {
		return
	} else if role == 2 {
		if !ok {
			return
		}
		if transaction.UserID.Bytes != userId.Bytes {
			helpers.RespondWithError(w, http.StatusForbidden, "cannot get this transaction")
			helpers.LogInfo("GetSingleTransaction", "Unauthorized", "userId", userId, "businessId", businessId, "transactionId", transactionId)
			return
		}
	}

	// **********************************************
	// respond
	// **********************************************
	res := resType{Transaction: transaction}
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("GetSingleTransaction", "success", "res", res)
}
