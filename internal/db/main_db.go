package db

import (
	"context"
	"fmt"

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

type GetFilteredTransactionsParams struct {
	BusinessID pgtype.UUID          `json:"business_id"`
	UserID     pgtype.UUID          `json:"user_id"`
	PartyID    pgtype.UUID          `json:"party_id"`
	CategoryID pgtype.UUID          `json:"category_id"`
	Mode       TransactionMode      `json:"mode"`
	Direction  TransactionDirection `json:"direction"`
}

func (db *DBStore) GetFilteredTransactions(ctx context.Context, arg GetFilteredTransactionsParams) ([]Transaction, error) {
	helpers.LogInfo("db GetTransactionsHandler", "1")
	query := "SELECT * From transactions WHERE business_id = $1"
	queryArgs := []any{arg.BusinessID}
	count := 2

	if arg.UserID.Valid {
		query += fmt.Sprintf(" AND user_id = $%d", count)
		queryArgs = append(queryArgs, arg.UserID)
		count += 1
	}
	if arg.PartyID.Valid {
		query += fmt.Sprintf(" AND party_id = $%d", count)
		queryArgs = append(queryArgs, arg.PartyID)
		count += 1
	}
	if arg.CategoryID.Valid {
		query += fmt.Sprintf(" AND category_id = $%d", count)
		queryArgs = append(queryArgs, arg.CategoryID)
		count += 1
	}
	if arg.Mode != "" {
		query += fmt.Sprintf(" AND mode = $%d", count)
		queryArgs = append(queryArgs, arg.Mode)
		count += 1
	}
	if arg.Direction != "" {
		query += fmt.Sprintf(" AND direction = $%d", count)
		queryArgs = append(queryArgs, arg.Direction)
		count += 1
	}
	query += " ORDER BY receipt_no ASC"
	helpers.LogInfo("db GetFilteredTransactions", "2", "query", query, "args", queryArgs)
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
