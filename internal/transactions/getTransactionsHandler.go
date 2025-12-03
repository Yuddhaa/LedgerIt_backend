package transactions

import (
	"net/http"
	"strings"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetTransactionsHandler returns all the transactions
// after doing filtering at db level based on the query params passed
func (h *Handler) GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Stats        db.TransactionStats `json:"stats"`
		Transactions []db.Transaction    `json:"transactions"`
	}
	// **********************************************
	// get all the data required for the filteing
	// **********************************************
	loggedUserId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	var userId []pgtype.UUID
	if role == 2 {
		userId = append(userId, loggedUserId)
	} else {
		userId, ok = parseUUIDList(w, "queryUserId", r.URL.Query()["user_id"])
		if !ok {
			return
		}
	}
	categoryId, ok := parseUUIDList(w, "categoryId", r.URL.Query()["category_id"])
	if !ok {
		return
	}
	partyId, ok := parseUUIDList(w, "partyId", r.URL.Query()["party_id"])
	if !ok {
		return
	}

	fromDate, ok := parseDate(w, "from", r.URL.Query().Get("from"))
	if !ok {
		return
	}

	toDate, ok := parseDate(w, "to", r.URL.Query().Get("to"))
	if !ok {
		return
	}
	// If we have a 'to' date, set it to the very last nanosecond of that day.
	if !toDate.IsZero() {
		toDate = time.Date(
			toDate.Year(), toDate.Month(), toDate.Day(),
			23, 59, 59, 999999999, // Hour, Min, Sec, Nsec
			toDate.Location(),
		)
	}

	sortBy := r.URL.Query().Get("sortBy")
	order := r.URL.Query().Get("order")
	mode := r.URL.Query()["mode"]
	for i := range mode {
		mode[i] = strings.ToLower(mode[i])
	}
	direction := strings.ToLower(r.URL.Query().Get("direction"))

	GetFilteredTransactionsInput := db.FilterParams{
		BusinessID: businessId,
		UserID:     userId,
		CategoryID: categoryId,
		PartyID:    partyId,
		Mode:       mode,
		Direction:  db.TransactionDirection(direction),
		FromDate:   fromDate,
		ToDate:     toDate,
		SortBy:     sortBy,
		SortOrder:  order,
	}

	// **********************************************
	// Do the db calls [get transactions and get stats]
	// **********************************************

	transactions, err := h.db.GetFilteredTransactions(r.Context(), GetFilteredTransactionsInput)
	if err != nil {
		helpers.LogError("GetTransactionsHandler", "error in GetFilteredTransactions",
			"err", err.Error(), "GetFilteredTransactionsParams", GetFilteredTransactionsInput)
		helpers.RespondWithError(w, 500, "internal server error")
		return
	}

	stats, err := h.db.GetTransactionStats(r.Context(), GetFilteredTransactionsInput)
	if err != nil {
		helpers.LogError("GetTransactionsHandler", "error in GetTransactionStats",
			"err", err.Error(), "GetFilteredTransactionsParams", GetFilteredTransactionsInput)
		helpers.RespondWithError(w, 500, "internal server error")
		return
	}

	// **********************************************
	// respond
	// **********************************************
	res := resType{Transactions: transactions, Stats: stats}
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("GetTransactionsHandler", "success", "count of transactions", len(res.Transactions), "transaction stats", stats)
}

func parseUUIDList(w http.ResponseWriter, paramName string, values []string) ([]pgtype.UUID, bool) {
	if len(values) == 0 {
		return nil, true // Empty is fine (means "all")
	}

	validUUIDs := make([]pgtype.UUID, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		UUID, err := uuid.Parse(v)
		if err != nil {
			helpers.RespondWithError(w, http.StatusBadRequest, "Bad Url query")
			helpers.LogError("convertToUUID", "bad query parameter:"+paramName, "Err", err.Error())
			return nil, false
		}
		validUUIDs = append(validUUIDs, pgtype.UUID{
			Bytes: UUID,
			Valid: uuid.Nil != UUID,
		})
	}
	return validUUIDs, true
}
