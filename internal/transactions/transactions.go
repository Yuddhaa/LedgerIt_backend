package transactions

import (
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	db *db.DBStore
}

func NewHandler(db *db.DBStore) *Handler {
	return &Handler{
		db: db,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// middlerwares
	r.Use(auth.GetRole(h.db))
	r.Use(auth.ValidatePlan(h.db))

	// routes
	r.Post("/", h.AddTransactionsHandler)
	r.Get("/", h.GetTransactionsHandler)
	r.Get("/{tran_id}", h.GetSingleTransactionsHandler)
	r.Patch("/{tran_id}", h.UpdateTransactionHandler)
	r.Delete("/{tran_id}", h.DeleteTransactionHandler)
	// approvals related
	r.Get("/approvals", h.GetApprovalsHandler)
	r.Patch("/approvals/{approval_id}", h.PatchApprovalHandler)
	return r
}

// tranReqType used for add and update transaction
type tranReqType struct {
	Amount      string                  `json:"amount"`
	Direction   db.TransactionDirection `json:"direction"`
	CategoryID  string                  `json:"category_id"`
	PartyID     string                  `json:"party_id"`
	Mode        db.TransactionMode      `json:"mode"`
	ReceiptNo   string                  `json:"receipt_no"`
	Description string                  `json:"description"`
	Reason      string                  `json:"reason,omitempty"`
}

// --- middlerware and helper funcitons

// parseDate is used to parse date for get transactions and edit requests
func parseDate(w http.ResponseWriter, paramName, dateStr string) (time.Time, bool) {
	if dateStr == "" {
		return time.Time{}, true // Return zero time if empty
	}

	// Layout for YYYY-MM-DD
	layout := "2006-01-02"
	// layout := "2006-01-02 15:04:05.999999+00"
	parsedTime, err := time.Parse(layout, dateStr)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Invalid date format for "+paramName+". Use YYYY-MM-DD")
		helpers.LogError("parseDate", "date parse error", "input date string", dateStr, "err", err.Error())
		return time.Time{}, false
	}
	helpers.PrintJson("parsedTime", parsedTime)
	return parsedTime, true
}
