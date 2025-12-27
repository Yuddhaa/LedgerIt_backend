package transactions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) GetApprovalsHandler(w http.ResponseWriter, r *http.Request) {
	type transaction struct {
		tranReqType
		CategoryName *string `json:"category_name"`
		PartyName    *string `json:"party_name"`
	}
	type dbResults struct {
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
		Requests []dbResults `json:"requests"`
	}
	// 1. Auth Check (Admin/Creator only)
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	var requestedByIDs []pgtype.UUID
	if role == 2 {
		loggedinUserId, ok := auth.GetUserIdFromContext(w, r)
		if !ok {
			return
		}
		requestedByIDs = []pgtype.UUID{loggedinUserId}
	} else {
		requestedByIDs, ok = parseUUIDList(w, "reviewedByIDs", r.URL.Query()["requested_by"])
		if !ok {
			return
		}
	}

	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// 2. Parse Query Params
	status := r.URL.Query().Get("status")
	approvalType := r.URL.Query().Get("type")

	fromDate, ok := parseDate(w, "from date", r.URL.Query().Get("from"))
	if !ok {
		return
	}

	toDate, ok := parseDate(w, "to date", r.URL.Query().Get("to"))
	if !ok {
		return
	}

	if !toDate.IsZero() {
		toDate = time.Date(
			toDate.Year(), toDate.Month(), toDate.Day(),
			23, 59, 59, 999999999, // Hour, Min, Sec, Nsec
			toDate.Location(),
		)
	}

	// 3. Prepare DB Params
	params := db.GetTransactionApprovalsParams{
		BusinessID:     businessId,
		RequestedByIds: requestedByIDs,
		Status:         status,
		Type:           approvalType,
		FromDate: pgtype.Timestamptz{
			Time:  fromDate,
			Valid: !fromDate.IsZero(),
		},
		ToDate: pgtype.Timestamptz{
			Time:  toDate,
			Valid: !toDate.IsZero(),
		},
	}

	// 4. Execute Query
	rows, err := h.db.GetTransactionApprovals(r.Context(), params)
	if err != nil {
		helpers.LogError("GetApprovalsHandler", "db error fetching approvals", "err", err.Error())
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	response := make([]dbResults, len(rows))
	for i := range rows {
		// A. Map Basic Fields
		response[i].ID = rows[i].ID
		response[i].TransactionID = rows[i].TransactionID
		response[i].Type = rows[i].Type
		response[i].Status = rows[i].Status
		response[i].Reason = rows[i].Reason
		response[i].CreatedAt = rows[i].CreatedAt
		response[i].UpdatedAt = rows[i].UpdatedAt
		response[i].RequestedByID = rows[i].RequestedByID
		response[i].RequestedByName = rows[i].RequestedByName
		response[i].ReviewedByID = rows[i].ReviewedByID
		response[i].ReviewedByName = rows[i].ReviewedByName

		// B. Map Original Transaction (From SQL Columns)
		// Convert Numeric Amount to String
		orgAmountFloat, _ := rows[i].OrgAmount.Float64Value()
		orgAmountStr := fmt.Sprintf("%.2f", orgAmountFloat.Float64)

		response[i].Original = transaction{
			tranReqType: tranReqType{
				Amount:      orgAmountStr,
				Direction:   rows[i].OrgDirection,
				CategoryID:  rows[i].OrgCategoryID.String(), // Convert UUID to string
				PartyID:     rows[i].OrgPartyID.String(),    // Convert UUID to string
				Mode:        rows[i].OrgMode,
				ReceiptNo:   rows[i].OrgReceiptNo,
				Description: rows[i].OrgDescription.String,
			},
			PartyName:    &rows[i].OrgPartyName, // Assuming SQL returns string or pointer
			CategoryName: &rows[i].OrgCategoryName.String,
		}

		// C. Map Requested Changes (From JSON)
		var tempReq tranReqType
		if err := json.Unmarshal([]byte(rows[i].RequestedChanges), &tempReq); err != nil {
			helpers.LogError("GetApprovalsHandler", "json unmarshal error", "err", err.Error())
		}

		response[i].RequestedChanges = transaction{
			tranReqType:  tempReq,
			PartyName:    &rows[i].ReqPartyName.String,    // From SQL Join
			CategoryName: &rows[i].ReqCategoryName.String, // From SQL Join
		}
	}
	res := resType{Requests: response}
	// 5. Success Response & Log
	helpers.LogInfo("GetApprovalsHandler", "approvals listed successfully", "count", len(rows), "business_id", businessId)
	helpers.RespondWithJSON(w, http.StatusOK, res)
}
