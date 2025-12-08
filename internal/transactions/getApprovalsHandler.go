package transactions

import (
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) GetApprovalsHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		Requests []db.GetTransactionApprovalsRow `json:"requests"`
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

	// Ensure we return [] instead of null in JSON if no rows found
	if rows == nil {
		rows = []db.GetTransactionApprovalsRow{}
	}

	res := rows
	response := resType{Requests: res}

	// 5. Success Response & Log
	helpers.LogInfo("GetApprovalsHandler", "approvals listed successfully", "count", len(rows), "business_id", businessId)
	helpers.RespondWithJSON(w, http.StatusOK, response)
}
