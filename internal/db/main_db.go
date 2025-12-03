package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBStore implements all database query functions.
// It embeds sqlc's generated Queries struct
// and holds the pool for custom queries.
type DBStore struct {
	*Queries
	Pool *pgxpool.Pool
}

func NewDBStore(pool *pgxpool.Pool) *DBStore {
	return &DBStore{
		Queries: New(pool),
		Pool:    pool,
	}
}

// GetFilteredTransactionsParams refers to params required to build whereClause
type FilterParams struct {
	BusinessID pgtype.UUID          `json:"business_id"`
	UserID     []pgtype.UUID        `json:"user_id"`
	PartyID    []pgtype.UUID        `json:"party_id"`
	CategoryID []pgtype.UUID        `json:"category_id"`
	Mode       []string             `json:"mode"`
	Direction  TransactionDirection `json:"direction"`
	FromDate   time.Time            `json:"from_date"`
	ToDate     time.Time            `json:"to_date"`
	SortBy     string               `json:"sortBy"`
	SortOrder  string               `json:"order"`
}

// buildWhereClause Helper: Builds the WHERE clause and Arguments
// Returns: (WHERE string, args []any)
func (db *DBStore) buildWhereClause(arg FilterParams) (string, []any) {
	helpers.LogInfo("db GetTransactionsHandler", "1")
	query := "From transactions WHERE business_id = $1"
	queryArgs := []any{arg.BusinessID}
	count := 2

	// build the query and args sequentially
	if len(arg.UserID) > 0 {
		query += fmt.Sprintf(" AND user_id = ANY($%d)", count)
		queryArgs = append(queryArgs, arg.UserID)
		count += 1
	}
	if len(arg.PartyID) > 0 {
		query += fmt.Sprintf(" AND party_id = ANY($%d)", count)
		queryArgs = append(queryArgs, arg.PartyID)
		count += 1
	}
	if len(arg.CategoryID) > 0 {
		query += fmt.Sprintf(" AND category_id = ANY($%d)", count)
		queryArgs = append(queryArgs, arg.CategoryID)
		count += 1
	}
	if len(arg.Mode) > 0 {
		query += fmt.Sprintf(" AND mode = ANY($%d)", count)
		queryArgs = append(queryArgs, arg.Mode)
		count += 1
	}
	if arg.Direction != "" {
		query += fmt.Sprintf(" AND direction = $%d", count)
		queryArgs = append(queryArgs, arg.Direction)
		count += 1
	}

	if !arg.FromDate.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", count)
		queryArgs = append(queryArgs, arg.FromDate)
		count += 1
	}

	if !arg.ToDate.IsZero() {
		query += fmt.Sprintf(" AND created_at <= $%d", count)
		queryArgs = append(queryArgs, arg.ToDate)
		count += 1
	}
	return query, queryArgs
}

// GetFilteredTransactions does a db call to get all the filtered transactions
func (db *DBStore) GetFilteredTransactions(ctx context.Context, arg FilterParams) ([]Transaction, error) {
	whereClause, queryArgs := db.buildWhereClause(arg)

	query := "SELECT * " + whereClause
	helpers.PrintJson("query", query)

	orderByClause := " ORDER BY created_at DESC"

	// Whitelist allowed columns to prevent SQL Injection
	validSortColumns := map[string]string{
		"amount":     "amount",
		"created_at": "created_at",
	}
	// Check if user provided a valid sort column
	if col, exists := validSortColumns[arg.SortBy]; exists {
		direction := "DESC"
		if strings.ToUpper(arg.SortOrder) == "ASC" {
			direction = "ASC"
		}
		orderByClause = fmt.Sprintf(" ORDER BY %s %s", col, direction)
	} else if arg.SortOrder != "" && (strings.ToUpper(arg.SortOrder) == "ASC" || strings.ToUpper(arg.SortOrder) == "DESC") {
		orderByClause = fmt.Sprintf("ORDER BY created_at %s", strings.ToUpper(arg.SortOrder))
	}

	query += orderByClause
	helpers.LogInfo("db GetFilteredTransactions", "2", "query", query, "args", queryArgs)

	// query call
	rows, err := db.Pool.Query(ctx, query, queryArgs...)
	if err != nil {
		helpers.LogError("db GetFilteredTransactions", "error in sending the query itself", "err", err.Error())
		return nil, err
	}
	defer rows.Close()
	var transactions []Transaction
	for rows.Next() {
		var i Transaction
		if err := rows.Scan(
			&i.ID,
			&i.BusinessID,
			&i.UserID,
			&i.Amount,
			&i.Direction,
			&i.CategoryID,
			&i.PartyID,
			&i.Mode,
			&i.ReceiptNo,
			&i.Description,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		transactions = append(transactions, i)
		helpers.LogInfo("db GetFilteredTransactions", "i", "transactions", transactions)
	}
	return transactions, nil
}

// TransactionStats holds the summary data
type TransactionStats struct {
	CashIn     float64 `json:"cash_in"`
	CashOut    float64 `json:"cash_out"`
	NetBalance float64 `json:"net_balance"`
}

// GetTransactionStats gets the stats based on the filter query
func (db *DBStore) GetTransactionStats(ctx context.Context, arg FilterParams) (TransactionStats, error) {
	whereClause, args := db.buildWhereClause(arg)

	// We calculate sums directly in SQL
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN direction = 'in' THEN amount ELSE 0 END), 0) as cash_in,
			COALESCE(SUM(CASE WHEN direction = 'out' THEN amount ELSE 0 END), 0) as cash_out
		` + whereClause

	row := db.Pool.QueryRow(ctx, query, args...)

	var stats TransactionStats
	// We scan into float64 for easy JSON response.
	// Since DB is DECIMAL(10,2), float64 is safe enough for display purposes.
	err := row.Scan(&stats.CashIn, &stats.CashOut)
	if err != nil {
		return stats, err
	}

	// Calculate Net in Go (Safe because we have simple floats now)
	stats.NetBalance = stats.CashIn - stats.CashOut

	return stats, nil
}
