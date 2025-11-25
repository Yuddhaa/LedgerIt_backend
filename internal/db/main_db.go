package db

import "github.com/jackc/pgx/v5/pgxpool"

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
