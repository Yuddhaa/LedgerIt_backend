package transactions

import (
	"encoding/json"
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) PatchApprovalHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Request
	type resType struct {
		Request db.PatchEditRequestRow `json:"request"`
	}
	type reqType struct {
		tranReqType        // Embeds Amount, Mode, etc.
		Status      string `json:"status,omitempty"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Json Request")
		helpers.LogError("PatchApprovalHandler", "json decode error", "err", err)
		return
	}

	// 2. Auth & Context
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}

	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}

	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	approvalId, ok := auth.ExtractUUID(w, r, "approval_id")
	if !ok {
		return
	}

	// ---------------------------------------------------------
	// 3. Prepare Logic Variables
	// ---------------------------------------------------------

	// A. Security Check ID (For WHERE clause)
	var checkUserID pgtype.UUID
	if role == 2 {
		// Employee: Can only edit OWN requests
		checkUserID = userId

		// Security: Employee CANNOT change status
		if body.Status != "" && body.Status != "pending" {
			helpers.RespondWithError(w, http.StatusUnauthorized, "Employees cannot change status")
			// ADDED LOG
			helpers.LogInfo("PatchApprovalHandler", "unauthorized status change attempt", "user_id", userId)
			return
		}
		// Force status to pending for employees
		body.Status = "pending"
	} else {
		// Admin: Can edit ANY request
		checkUserID = pgtype.UUID{Valid: false} // SQL will ignore check
	}

	// B. Status & Reviewer Logic
	// Default to 'pending' if not sent
	targetStatus := db.EditRequestStatusPending
	var reviewerID pgtype.UUID

	if body.Status != "" {
		targetStatus = db.EditRequestStatus(body.Status)
		// If Admin is approving/rejecting, set them as reviewer
		if targetStatus == db.EditRequestStatusApproved || targetStatus == db.EditRequestStatusRejected {
			reviewerID = userId
		}
	}

	// C. Marshal Content
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

	// ---------------------------------------------------------
	// 4. START TRANSACTION (The Atomic Operation)
	// ---------------------------------------------------------
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Server Error")
		// ADDED LOG
		helpers.LogError("PatchApprovalHandler", "failed to begin tx", "err", err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.db.WithTx(tx)

	// 5. Update the Edit Request Table
	updatedReq, err := qtx.PatchEditRequest(r.Context(), db.PatchEditRequestParams{
		ID:               approvalId,
		Column2:          checkUserID, // The security check ID
		RequestedChanges: string(changesJson),
		Reason:           pgtype.Text{String: body.Reason, Valid: body.Reason != ""},
		Status:           targetStatus,
		ReviewedByID:     reviewerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Request not found or not editable")
			// ADDED LOG
			helpers.LogInfo("PatchApprovalHandler", "request not found or access denied", "approval_id", approvalId)
			return
		}
		helpers.LogError("PatchApprovalHandler", "db update error", "err", err)
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// ---------------------------------------------------------
	// 6. IF APPROVED: Apply to Main Transaction Table
	// ---------------------------------------------------------
	if updatedReq.Status == db.EditRequestStatusApproved {

		// A. Fetch Original Transaction (Locked)
		oldTran, err := qtx.GetTransactionForUpdate(r.Context(), db.GetTransactionForUpdateParams{
			ID:         updatedReq.TransactionID,
			BusinessID: businessId,
		})
		if err != nil {
			helpers.LogError("PatchApprovalHandler", "original transaction missing", "id", updatedReq.TransactionID)
			helpers.RespondWithError(w, http.StatusInternalServerError, "Original transaction not found")
			return
		}

		// B. Apply Updates (Using the Helper from previous step)
		// We pass 'tx' (so it doesn't commit yet) and 'qtx'.
		// We reuse 'body.tranReqType' because that contains the EXACT data we just saved to the request table.
		h.processAdminUpdate(r, w, tx, qtx, body.tranReqType, oldTran, businessId)

		// processAdminUpdate performs the Commit inside. We are done.
		// Note: processAdminUpdate sends the "Transaction Updated" JSON.
		// If you want the response to be the *Request* object instead,
		// you might need to adjust processAdminUpdate to NOT send JSON,
		// or send a custom response here.

		// Assuming processAdminUpdate sends response is fine for "Approval Action".
		return
	}

	// ---------------------------------------------------------
	// 7. FINISH (For Pending Edits or Rejections)
	// ---------------------------------------------------------
	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Commit failed")
		// ADDED LOG
		helpers.LogError("PatchApprovalHandler", "commit failed", "err", err)
		return
	}

	res := resType{
		Request: updatedReq,
	}
	helpers.RespondWithJSON(w, http.StatusOK, res)
	helpers.LogInfo("PatchApprovalHandler", "request updated", "id", approvalId, "status", targetStatus)
}
