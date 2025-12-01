package transactions

import (
	"net/http"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Transactions []db.Transaction `json:"transactions"`
	}
	loggedUserId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	role, ok := GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	var userId pgtype.UUID
	if role == 2 {
		userId = loggedUserId
	} else {
		userId, ok = convertToUUID(w, "queryUserId", r.URL.Query().Get("user_id"))
		if !ok {
			return
		}
	}
	categoryId, ok := convertToUUID(w, "categoryId", r.URL.Query().Get("category_id"))
	if !ok {
		return
	}
	partyId, ok := convertToUUID(w, "partyId", r.URL.Query().Get("party_id"))
	if !ok {
		return
	}
	mode := strings.ToLower(r.URL.Query().Get("mode"))
	direction := strings.ToLower(r.URL.Query().Get("direction"))
	GetFilteredTransactionsInput := db.GetFilteredTransactionsParams{
		BusinessID: businessId,
		UserID:     userId,
		CategoryID: categoryId,
		PartyID:    partyId,
		Mode:       db.TransactionMode(mode),
		Direction:  db.TransactionDirection(direction),
	}
	transactions, err := h.db.GetFilteredTransactions(r.Context(), GetFilteredTransactionsInput)
	if err != nil {
		helpers.LogError("GetTransactionsHandler", "error in GetFilteredTransactions",
			"err", err.Error(), "GetFilteredTransactionsParams", GetFilteredTransactionsInput)
		helpers.RespondWithError(w, 500, "internal server error")
		return
	}
	res := resType{Transactions: transactions}
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("GetTransactionsHandler", "success", "count of transactions", len(res.Transactions))
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
