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

func (h *Handler) UpdateTransactionHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Body
	// Assumes tranReqType is defined in this package
	var body tranReqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		helpers.LogError("UpdateTransactionHandler", "json decode error", "err", err)
		return
	}

	// 2. Extract Context
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	helpers.PrintJson("role", role)

	tranId, ok := auth.ExtractUUID(w, r, "tran_id")
	if !ok {
		return
	}

	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}
	helpers.PrintJson("userID", userId)

	// 3. START TRANSACTION (Parent Level)
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Server error")
		helpers.LogError("UpdateTransactionHandler", "failed to begin tx", "err", err)
		return
	}
	defer tx.Rollback(r.Context()) // Safety rollback

	// Create QTX (Queries bound to this transaction)
	qtx := h.db.WithTx(tx)

	// 4. FETCH & LOCK (Parent Level)
	oldTran, err := qtx.GetTransactionForUpdate(r.Context(), db.GetTransactionForUpdateParams{
		ID:         tranId,
		BusinessID: businessId,
	})
	if err != nil {
		helpers.RespondWithError(w, http.StatusNotFound, "Transaction not found")
		helpers.LogInfo("UpdateTransactionHandler", "transaction not found for update", "tran_id", tranId)
		return
	}

	// 5. LOGIC BRANCH
	// We pass 'tx' to helpers so they can Commit BEFORE responding
	if role == 2 {
		// --- EMPLOYEE LOGIC ---
		if oldTran.UserID.Bytes != userId.Bytes {
			helpers.RespondWithError(w, http.StatusForbidden, "Access denied: You can only edit your own transactions")
			helpers.LogInfo("UpdateTransactionHandler", "access denied", "user_id", userId, "tran_owner", oldTran.UserID)
			return
		}

		if body.Reason == "" {
			helpers.RespondWithError(w, http.StatusBadRequest, "No reason given")
			helpers.LogInfo("UpdateTransactionHandler", "No reason given", "user_id", userId, "tran_owner", oldTran.UserID)
			return
		}

		h.processEmployeeEditRequest(r, w, tx, qtx, body, tranId, userId)
	} else {
		// --- ADMIN/CREATOR LOGIC ---
		h.processAdminUpdate(r, w, tx, qtx, body, oldTran, businessId)
	}
	// No Commit here. Helpers do it to ensure Response happens AFTER Commit.
}

// -----------------------------------------------------------------------------
// Helper: Process Employee Request
// -----------------------------------------------------------------------------
func (h *Handler) processEmployeeEditRequest(r *http.Request, w http.ResponseWriter, tx pgx.Tx, qtx *db.Queries, body tranReqType, tranID, userID pgtype.UUID) {
	// 1. Marshal changes
	changesMap := map[string]any{
		"amount":      body.Amount,
		"direction":   body.Direction,
		"category_id": body.CategoryID,
		"party_id":    body.PartyID,
		"mode":        body.Mode,
		"receipt_no":  body.ReceiptNo,
		"description": body.Description,
	}
	changesJson, _ := json.Marshal(changesMap)

	// 2. Insert Request
	req, err := qtx.CreateEditRequest(r.Context(), db.CreateEditRequestParams{
		TransactionID:    tranID,
		RequestedByID:    userID,
		RequestedChanges: string(changesJson),
		Reason:           pgtype.Text{String: body.Reason, Valid: body.Reason != ""},
	})
	if err != nil {
		if helpers.IsUniqueViolation(err) {
			helpers.LogError("UpdateTransactionHandler", "Edit request for this transaction already exists")
			helpers.RespondWithError(w, http.StatusConflict, "Edit request for this transaction already exists")
			return
		}
		helpers.LogError("UpdateTransactionHandler", "error creating edit request", "err", err)
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// 3. COMMIT (Must happen before response)
	if err := tx.Commit(r.Context()); err != nil {
		helpers.LogError("UpdateTransactionHandler", "commit failed", "err", err)
		helpers.RespondWithError(w, http.StatusInternalServerError, "Commit failed")
		return
	}

	// 4. Respond & Log
	helpers.RespondWithJSON(w, http.StatusAccepted, map[string]any{
		"message":    "Edit request submitted for approval",
		"request_id": req.ID,
	})
	helpers.LogInfo("UpdateTransactionHandler", "edit request submitted", "req_id", req.ID, "user_id", userID)
}

// -----------------------------------------------------------------------------
// Helper: Process Admin Update
// -----------------------------------------------------------------------------
func (h *Handler) processAdminUpdate(r *http.Request, w http.ResponseWriter, tx pgx.Tx, qtx *db.Queries, body tranReqType, oldTran db.Transaction, businessID pgtype.UUID) {
	// 1. Parse Amount
	var newAmount pgtype.Numeric
	if err := newAmount.Scan(body.Amount); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Invalid amount")
		helpers.LogError("UpdateTransactionHandler", "invalid amount format", "amount", body.Amount)
		return
	}

	// 2. Parse UUIDs
	// Party is REQUIRED
	pUUID, err := uuid.Parse(body.PartyID)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Invalid party_id")
		helpers.LogError("UpdateTransactionHandler", "invalid party_id", "id", body.PartyID)
		return
	}
	newPartyID := pgtype.UUID{Bytes: pUUID, Valid: true}

	// Category is OPTIONAL
	var newCategoryID pgtype.UUID
	if body.CategoryID != "" {
		cUUID, err := uuid.Parse(body.CategoryID)
		if err != nil {
			helpers.RespondWithError(w, http.StatusBadRequest, "Invalid category_id")
			helpers.LogError("UpdateTransactionHandler", "invalid category_id", "id", body.CategoryID)
			return
		}
		newCategoryID = pgtype.UUID{Bytes: cUUID, Valid: true}
	} else {
		newCategoryID = pgtype.UUID{Valid: false}
	}

	// 3. Update Transaction Record
	updatedTran, err := qtx.UpdateTransaction(r.Context(), db.UpdateTransactionParams{
		ID:          oldTran.ID,
		BusinessID:  businessID,
		Amount:      newAmount,
		Direction:   body.Direction,
		CategoryID:  newCategoryID,
		PartyID:     newPartyID,
		Mode:        body.Mode,
		ReceiptNo:   body.ReceiptNo,
		Description: pgtype.Text{String: body.Description, Valid: body.Description != ""},
	})
	if err != nil {
		helpers.LogError("UpdateTransactionHandler", "update failed", "err", err)
		helpers.RespondWithError(w, http.StatusInternalServerError, "Update failed")
		return
	}

	// 4. Calculate Balance Adjustment (Reverse Old, Apply New)
	oldVal, _ := oldTran.Amount.Float64Value()
	newVal, _ := newAmount.Float64Value()

	var balanceDelta float64 = 0

	// A. Reverse Old Impact
	if oldTran.Mode == "cash" {
		if oldTran.Direction == "in" {
			balanceDelta -= oldVal.Float64
		} else {
			balanceDelta += oldVal.Float64
		}
	}

	// B. Apply New Impact
	if body.Mode == "cash" {
		if body.Direction == "in" {
			balanceDelta += newVal.Float64
		} else {
			balanceDelta -= newVal.Float64
		}
	}

	// 5. Update Balance (If needed)
	if balanceDelta != 0 {
		var deltaNumeric pgtype.Numeric
		deltaNumeric.Scan(balanceDelta)

		// Note: We update the balance of the ORIGINAL creator (oldTran.UserID)
		err = qtx.UpdateBusinessMemberBalance(r.Context(), db.UpdateBusinessMemberBalanceParams{
			CurrentBalance: deltaNumeric, // Using Amount based on SQL logic (+ $1)
			UserID:         oldTran.UserID,
			BusinessID:     businessID,
		})
		if err != nil {
			helpers.LogError("UpdateTransactionHandler", "balance update failed", "err", err)
			helpers.RespondWithError(w, http.StatusInternalServerError, "Balance update failed")
			return
		}
	}

	// 6. COMMIT (Must happen before response)
	if err := tx.Commit(r.Context()); err != nil {
		helpers.LogError("UpdateTransactionHandler", "commit failed", "err", err)
		helpers.RespondWithError(w, http.StatusInternalServerError, "Commit failed")
		return
	}

	// 7. Respond & Log
	helpers.RespondWithJSON(w, http.StatusOK, map[string]any{
		"transaction": updatedTran,
	})
	helpers.LogInfo("UpdateTransactionHandler", "transaction updated successfully", "tran_id", oldTran.ID)
}
