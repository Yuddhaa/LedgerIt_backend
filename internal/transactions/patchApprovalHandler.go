package transactions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) PatchApprovalHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Request
	type transaction struct {
		tranReqType
		CategoryName *string `json:"category_name"`
		PartyName    *string `json:"party_name"`
	}
	type dbResult struct {
		ID               pgtype.UUID              `json:"id"`
		TransactionID    pgtype.UUID              `json:"transaction_id"`
		Type             db.TransactionChangeType `json:"type"`
		Status           db.EditRequestStatus     `json:"status"`
		Reason           pgtype.Text              `json:"reason"`
		CreatedAt        pgtype.Timestamptz       `json:"created_at"`
		UpdatedAt        pgtype.Timestamptz       `json:"updated_at"`
		RequestedByID    pgtype.UUID              `json:"requested_by_id"`
		RequestedByName  pgtype.Text              `json:"requested_by_name"`
		ReviewedByID     pgtype.UUID              `json:"reviewed_by_id"`
		ReviewedByName   pgtype.Text              `json:"reviewed_by_name"`
		RequestedChanges transaction              `json:"requested_changes"`
		Original         transaction              `json:"original"`
	}
	type resType struct {
		Request dbResult `json:"request"`
	}
	type reqType struct {
		tranReqType        // Embeds Amount, Mode, etc.
		Status      string `json:"status,omitempty"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Json Request")
		helpers.LogError("PatchApprovalHandler", "json decode error", "err", err.Error())
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
		helpers.LogError("PatchApprovalHandler", "failed to begin tx", "err", err.Error())
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
		helpers.LogError("PatchApprovalHandler", "db update error", "err", err.Error())
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
		helpers.LogError("PatchApprovalHandler", "commit failed", "err", err.Error())
		return
	}

	request := dbResult{}
	// A. Map Basic Fields
	request.ID = updatedReq.ID
	request.TransactionID = updatedReq.TransactionID
	request.Type = updatedReq.Type
	request.Status = updatedReq.Status
	request.Reason = updatedReq.Reason
	request.CreatedAt = updatedReq.CreatedAt
	request.UpdatedAt = updatedReq.UpdatedAt
	request.RequestedByID = updatedReq.RequestedByID
	request.RequestedByName = updatedReq.RequestedByName
	request.ReviewedByID = updatedReq.ReviewedByID
	request.ReviewedByName = updatedReq.ReviewedByName

	// B. Map Original Transaction (From SQL Columns)
	// Convert Numeric Amount to String
	orgAmountFloat, _ := updatedReq.OrgAmount.Float64Value()
	orgAmountStr := fmt.Sprintf("%.2f", orgAmountFloat.Float64)

	request.Original = transaction{
		tranReqType: tranReqType{
			Amount:      orgAmountStr,
			Direction:   updatedReq.OrgDirection,
			CategoryID:  updatedReq.OrgCategoryID.String(), // Convert UUID to string
			PartyID:     updatedReq.OrgPartyID.String(),    // Convert UUID to string
			Mode:        updatedReq.OrgMode,
			ReceiptNo:   updatedReq.OrgReceiptNo,
			Description: updatedReq.OrgDescription.String,
		},
		PartyName:    &updatedReq.OrgPartyName, // Assuming SQL returns string or pointer
		CategoryName: &updatedReq.OrgCategoryName.String,
	}

	// C. Map Requested Changes (From JSON)
	var tempReq tranReqType
	if err := json.Unmarshal([]byte(updatedReq.RequestedChanges), &tempReq); err != nil {
		helpers.LogError("GetApprovalsHandler", "json unmarshal error", "err", err.Error())
	}

	request.RequestedChanges = transaction{
		tranReqType:  tempReq,
		PartyName:    &updatedReq.ReqPartyName.String,    // From SQL Join
		CategoryName: &updatedReq.ReqCategoryName.String, // From SQL Join
	}
	res := resType{
		Request: request,
	}
	helpers.RespondWithJSON(w, http.StatusOK, res)
	helpers.LogInfo("PatchApprovalHandler", "request updated", "id", approvalId, "status", targetStatus)
}
