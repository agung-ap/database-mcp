package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// PoolConfig holds connection pool settings.
type PoolConfig struct {
	MaxOpen                int
	MaxIdle                int
	ConnMaxLifetimeMinutes int
}

// Driver wraps *sqlx.DB and implements the db.Driver interface.
type Driver struct {
	db *sqlx.DB
}

// New creates a new PostgreSQL driver with the given DSN and pool config.
func New(dsn string, pool PoolConfig) (*Driver, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}

	if pool.MaxOpen > 0 {
		db.SetMaxOpenConns(pool.MaxOpen)
	}
	if pool.MaxIdle > 0 {
		db.SetMaxIdleConns(pool.MaxIdle)
	}
	if pool.ConnMaxLifetimeMinutes > 0 {
		db.SetConnMaxLifetime(time.Duration(pool.ConnMaxLifetimeMinutes) * time.Minute)
	}
	db.SetConnMaxIdleTime(5 * time.Minute)

	return &Driver{db: db}, nil
}

func (d *Driver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

func (d *Driver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

func (d *Driver) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, opts)
}

func (d *Driver) Conn(ctx context.Context) (*sql.Conn, error) {
	return d.db.Conn(ctx)
}

func (d *Driver) PingContext(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *Driver) Close() error {
	return d.db.Close()
}

func (d *Driver) DriverName() string {
	return d.db.DriverName()
}
