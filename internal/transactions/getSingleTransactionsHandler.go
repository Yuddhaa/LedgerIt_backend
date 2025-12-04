package transactions

import (
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
)

// GetSingleTransactionsHandler returns a single transaction based on the "tran_id" path parameter
func (h *Handler) GetSingleTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Transaction db.GetFilteredTransactionsRows `json:"transaction"`
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
	transaction, err := h.db.GetFilteredTransactions(r.Context(), filters)
	if err != nil {
		if len(transaction) == 0 {
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
	if !ok {
		return
	}
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
		if transaction[0].UserID.Bytes != userId.Bytes {
			helpers.RespondWithError(w, http.StatusForbidden, "cannot get this transaction")
			helpers.LogInfo("GetSingleTransaction", "Unauthorized", "userId", userId, "businessId", businessId, "transactionId", transactionId)
			return
		}
	}

	// **********************************************
	// respond
	// **********************************************
	res := resType{Transaction: transaction[0]}
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("GetSingleTransaction", "success", "res", res)
}
