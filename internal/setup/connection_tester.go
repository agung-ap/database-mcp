package setup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// ConnectionTester tests database connections.
type ConnectionTester struct {
	timeout time.Duration
}

// NewConnectionTester creates a new connection tester.
func NewConnectionTester(timeout time.Duration) *ConnectionTester {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &ConnectionTester{timeout: timeout}
}

// TestConnection tests a database connection.
func (ct *ConnectionTester) TestConnection(cfg *DatabaseConfig) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ct.timeout)
	defer cancel()

	var dsn string

	switch cfg.Driver {
	case "postgres":
		dsn = buildPostgresDSN(cfg)
	case "mysql":
		dsn = buildMySQLDSN(cfg)
	case "sqlite":
		dsn = buildSQLiteDSN(cfg)
	default:
		return false, "", fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}

	db, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return false, fmt.Sprintf("failed to open connection: %v", err), nil
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return false, fmt.Sprintf("connection failed: %v", err), nil
	}

	slog.Debug("connection test successful", "driver", cfg.Driver, "name", cfg.Name)
	return true, "Connected!", nil
}

// buildPostgresDSN builds a PostgreSQL DSN.
func buildPostgresDSN(cfg *DatabaseConfig) string {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, sslMode)

	return dsn
}

// buildMySQLDSN builds a MySQL DSN.
func buildMySQLDSN(cfg *DatabaseConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
}

// buildSQLiteDSN builds a SQLite DSN.
func buildSQLiteDSN(cfg *DatabaseConfig) string {
	return cfg.FilePath
}
