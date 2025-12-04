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
	TransactionId pgtype.UUID          `json:"transaction_id"`
	BusinessID    pgtype.UUID          `json:"business_id"`
	UserID        []pgtype.UUID        `json:"user_id"`
	PartyID       []pgtype.UUID        `json:"party_id"`
	CategoryID    []pgtype.UUID        `json:"category_id"`
	Mode          []string             `json:"mode"`
	Direction     TransactionDirection `json:"direction"`
	FromDate      time.Time            `json:"from_date"`
	ToDate        time.Time            `json:"to_date"`
	SortBy        string               `json:"sortBy"`
	SortOrder     string               `json:"order"`
}

// buildWhereClause Helper: Builds the WHERE clause and Arguments
// Returns: (WHERE string, args []any)
func (db *DBStore) buildWhereClause(arg FilterParams) (string, []any) {
	helpers.LogInfo("db GetTransactionsHandler", "1")
	query := "From transactions WHERE business_id = $1"
	queryArgs := []any{arg.BusinessID}
	count := 2

	if arg.TransactionId.Valid {
		query += fmt.Sprintf(" AND id = $%d", count)
		queryArgs = append(queryArgs, arg.TransactionId)
		count += 1
	}

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

type GetFilteredTransactionsRows struct {
	Transaction          // Embeds all original fields (ID, Amount, etc.)
	UserName     *string `json:"user_name"`
	PartyName    *string `json:"party_name"`
	CategoryName *string `json:"category_name"`
}

// GetFilteredTransactions does a db call to get all the filtered transactions
func (db *DBStore) GetFilteredTransactions(ctx context.Context, arg FilterParams) ([]GetFilteredTransactionsRows, error) {
	// 1. Get the basic filtering logic (FROM transactions WHERE ...)
	// This ensures we don't break Stats or SingleTransaction
	whereClause, queryArgs := db.buildWhereClause(arg)

	// 2. Wrap it in a CTE (WITH clause)
	// We create a temporary view 't' containing only the filtered transactions,
	// then we join the other tables to 't'.
	query := fmt.Sprintf(`
		WITH filtered_tx AS (
			SELECT * %s
		)
		SELECT 
			t.*,
			u.name as user_name,
			p.name as party_name,
			c.name as category_name
		FROM filtered_tx t
		LEFT JOIN users u ON t.user_id = u.id
		LEFT JOIN parties p ON t.party_id = p.id
		LEFT JOIN transaction_categories c ON t.category_id = c.id
	`, whereClause)

	// 3. Sorting Logic
	// Note: We use "t." prefix to be safe, though strictly not required in this specific structure.
	orderByClause := " ORDER BY t.created_at DESC"

	validSortColumns := map[string]string{
		"amount":     "t.amount",
		"created_at": "t.created_at",
	}

	if col, exists := validSortColumns[arg.SortBy]; exists {
		direction := "DESC"
		if strings.ToUpper(arg.SortOrder) == "ASC" {
			direction = "ASC"
		}
		orderByClause = fmt.Sprintf(" ORDER BY %s %s", col, direction)
	} else if arg.SortOrder != "" && (strings.ToUpper(arg.SortOrder) == "ASC" || strings.ToUpper(arg.SortOrder) == "DESC") {
		orderByClause = fmt.Sprintf(" ORDER BY t.created_at %s", strings.ToUpper(arg.SortOrder))
	}

	query += orderByClause
	helpers.LogInfo("db GetFilteredTransactions", "final_query", "query", query, "args", queryArgs)

	// 4. Execute
	rows, err := db.Pool.Query(ctx, query, queryArgs...)
	if err != nil {
		helpers.LogError("db GetFilteredTransactions", "error in sending the query itself", "err", err.Error())
		return nil, err
	}
	defer rows.Close()

	var transactions []GetFilteredTransactionsRows
	for rows.Next() {
		var i GetFilteredTransactionsRows
		// Scan the embedded Transaction struct fields first
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
			// Scan the new Name fields (pointers handle NULLs automatically)
			&i.UserName,
			&i.PartyName,
			&i.CategoryName,
		); err != nil {
			return nil, err
		}
		transactions = append(transactions, i)
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

// TODO, you can use the above funciton itself instead of below.
func (db *DBStore) GetSingleTransaction(ctx context.Context, arg FilterParams) (Transaction, error) {
	whereClause, args := db.buildWhereClause(arg)

	query := `SELECT * ` + whereClause
	helpers.LogInfo("in maindb GetSingleTransaction", "", "query", query, "args:", args)

	row := db.Pool.QueryRow(ctx, query, args...)

	var transaction Transaction
	err := row.Scan(
		&transaction.ID,
		&transaction.BusinessID,
		&transaction.UserID,
		&transaction.Amount,
		&transaction.Direction,
		&transaction.CategoryID,
		&transaction.PartyID,
		&transaction.Mode,
		&transaction.ReceiptNo,
		&transaction.Description,
		&transaction.CreatedAt,
		&transaction.UpdatedAt,
	)
	if err != nil {
		return transaction, err
	}
	return transaction, nil
}
