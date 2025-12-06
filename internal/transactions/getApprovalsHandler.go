package transactions

import (
	"encoding/json"
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) GetApprovalsHandler(w http.ResponseWriter, r *http.Request) {
	type reqChanges struct {
		tranReqType
		PartyName    string `json:"party_name"`
		CategoryName string `json:"category_name"`
	}
	type approvalRequest struct {
		ID               pgtype.UUID              `json:"id"`
		TransactionID    pgtype.UUID              `json:"transaction_id"`
		Type             db.TransactionChangeType `json:"type"`
		Status           db.EditRequestStatus     `json:"status"`
		RequestedChanges reqChanges               `json:"requested_changes"`
		Reason           pgtype.Text              `json:"reason"`
		CreatedAt        pgtype.Timestamptz       `json:"created_at"`
		UpdatedAt        pgtype.Timestamptz       `json:"updated_at"`
		RequestedByID    pgtype.UUID              `json:"requested_by_id"`
		RequestedByName  pgtype.Text              `json:"requested_by_name"`
		ReviewedByID     pgtype.UUID              `json:"reviewed_by_id"`
		ReviewedByName   pgtype.Text              `json:"reviewed_by_name"`
	}
	type resType struct {
		Requests []approvalRequest `json:"requests"`
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

	status := r.URL.Query().Get("status")
	approvalType := r.URL.Query().Get("type")
	fromDate, ok := parseDate(w, "from date", r.URL.Query().Get("from"))
	if !ok {
		return
	}
	toDate, ok := parseDate(w, "from date", r.URL.Query().Get("to"))
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

	res := make([]approvalRequest, len(rows))
	for i := range rows {
		var temp tranReqType
		if err := json.Unmarshal(rows[i].RequestedChanges, &temp); err != nil {
			helpers.PrintJson("err in Unmarshal", err.Error())
		}
		res[i].ID = rows[i].ID
		res[i].TransactionID = rows[i].TransactionID
		res[i].Type = rows[i].Type
		res[i].Status = rows[i].Status
		res[i].RequestedChanges = reqChanges{
			tranReqType:  temp,
			PartyName:    rows[i].PartyName.String,
			CategoryName: rows[i].CategoryName.String,
		}
		res[i].Reason = rows[i].Reason
		res[i].CreatedAt = rows[i].CreatedAt
		res[i].UpdatedAt = rows[i].UpdatedAt
		res[i].RequestedByID = rows[i].RequestedByID
		res[i].RequestedByName = rows[i].RequestedByName
		res[i].ReviewedByID = rows[i].ReviewedByID
		res[i].ReviewedByName = rows[i].ReviewedByName
	}

	response := resType{Requests: res}
	helpers.RespondWithJSON(w, http.StatusOK, response)
}
