package transactions

import (
	"encoding/json"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// AddTransactionsHandler creates a new transaction row
func (h *Handler) AddTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	var body tranReqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		helpers.LogInfo("AddTransactionsHandler", "failed to decode request body", "error", err.Error())
		return
	}
	type resType struct {
		Transaction db.Transaction `json:"transaction"`
	}
	// set up all the transactions column
	// user id
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}

	// business id
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// amount
	var amount pgtype.Numeric
	if err := amount.Scan(body.Amount); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid amount")
		helpers.LogInfo("AddTransactionsHandler", "failed to convert amount str to int", "error", err.Error(), "amount", body.Amount)
		return
	}

	//  categoryId
	var categoryID pgtype.UUID
	if body.CategoryID == "" {
		categoryID = pgtype.UUID{
			Valid: false,
		}
	} else {
		tempCategoryId, err := uuid.Parse(body.CategoryID)
		if err != nil {
			helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid category_id")
			helpers.LogInfo("AddTransactionsHandler", "failed to convert category_id str to uuid", "error", err.Error(), "category_id", body.CategoryID)
			return
		}
		categoryID = pgtype.UUID{
			Bytes: tempCategoryId,
			Valid: tempCategoryId != uuid.Nil,
		}
	}

	// partyId
	tempPartyId, err := uuid.Parse(body.PartyID)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid or no party_id")
		helpers.LogInfo("AddTransactionsHandler", "failed to convert partyId str to uuid", "error", err.Error(), "party_id", body.PartyID)
		return
	}
	partyId := pgtype.UUID{
		Bytes: tempPartyId,
		Valid: true,
	}

	// receipt number
	if body.ReceiptNo == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: no receipt_no")
		helpers.LogInfo("AddTransactionsHandler", "bad request: no receipt_no")
		return
	}

	// description
	description := pgtype.Text{
		String: body.Description,
		Valid:  body.Description != "",
	}
	// -------------------------------------------------------------------------
	// START DATABASE TRANSACTION
	// -------------------------------------------------------------------------
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "server error")
		helpers.LogError("AddTransactionsHandler", "failed to begin tx", "err", err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.db.WithTx(tx)

	// -------------------------------------------------------------------------
	// STEP 1: Create Transaction
	// -------------------------------------------------------------------------
	transaction, err := qtx.CreateTransactionWithValidation(r.Context(), db.CreateTransactionWithValidationParams{
		UserID:      userId,
		BusinessID:  businessId,
		Amount:      amount,
		Direction:   body.Direction,
		CategoryID:  categoryID,
		PartyID:     partyId,
		Mode:        body.Mode,
		ReceiptNo:   body.ReceiptNo,
		Description: description,
	})
	if err == pgx.ErrNoRows {
		// This means the WHERE clause in the SQL failed (invalid party/category)
		helpers.RespondWithError(w, http.StatusBadRequest, "Invalid Party or Category for this Business")
		return
	} else if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Failed to create transaction")
		helpers.LogError("AddTx", "Insert failed", "err", err.Error())
		return
	}
	// -------------------------------------------------------------------------
	// STEP 2: Update Business Member Balance
	// -------------------------------------------------------------------------

	// Logic: If I collected Cash (IN), my "cash in hand" increases.
	// If I Paid Cash (OUT), my "cash in hand" decreases.
	signedAmount := amount
	if body.Direction == db.TransactionDirectionOut {
		signedAmount.Int.Neg(signedAmount.Int)
	}
	if err := qtx.UpdateBusinessMemberBalance(r.Context(), db.UpdateBusinessMemberBalanceParams{
		CurrentBalance: signedAmount,
		UserID:         userId,
		BusinessID:     businessId,
	}); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "error updating balance")
		helpers.LogError("AddTransactionsHandler", "balance update error", "err", err.Error())
		return
	}
	// -------------------------------------------------------------------------
	// STEP 3: Commit
	// -------------------------------------------------------------------------
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "commit failed")
		helpers.LogError("AddTransactionsHandler", "commit error", "err", err.Error())
		return
	}

	res := resType{Transaction: transaction}
	helpers.LogInfo("AddTransactionsHandler", "transaction added", "transactionId", transaction.ID)
	helpers.RespondWithJSON(w, 201, res)
}
