package db

import (
	"context"
	"database/sql"
)

// Driver is the driver-agnostic database interface that all tools use.
// Concrete implementations live in internal/db/postgres, internal/db/mysql, and internal/db/mssql.
type Driver interface {
	// QueryContext runs a query returning rows.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	// ExecContext runs a statement returning result.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	// BeginTx starts a transaction.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	// PingContext checks connectivity.
	PingContext(ctx context.Context) error
	// Close closes the underlying connection pool.
	Close() error
	// DriverName returns "postgres", "mysql", or "sqlserver".
	DriverName() string
}
