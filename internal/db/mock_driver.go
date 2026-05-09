package db

import (
	"context"
	"database/sql"
)

// MockDriver is a hand-written mock of the Driver interface for unit tests.
type MockDriver struct {
	QueryContextFn func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContextFn  func(ctx context.Context, query string, args ...any) (sql.Result, error)
	BeginTxFn      func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	PingContextFn  func(ctx context.Context) error
	CloseFn        func() error
	DriverNameFn   func() string
}

func (m *MockDriver) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if m.QueryContextFn != nil {
		return m.QueryContextFn(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockDriver) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if m.ExecContextFn != nil {
		return m.ExecContextFn(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if m.BeginTxFn != nil {
		return m.BeginTxFn(ctx, opts)
	}
	return nil, nil
}

func (m *MockDriver) PingContext(ctx context.Context) error {
	if m.PingContextFn != nil {
		return m.PingContextFn(ctx)
	}
	return nil
}

func (m *MockDriver) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

func (m *MockDriver) DriverName() string {
	if m.DriverNameFn != nil {
		return m.DriverNameFn()
	}
	return "postgres"
}

// MockResult is a simple implementation of sql.Result for testing.
type MockResult struct {
	LastInsertIDVal int64
	RowsAffectedVal int64
	LastInsertIDErr error
	RowsAffectedErr error
}

func (r MockResult) LastInsertId() (int64, error) {
	return r.LastInsertIDVal, r.LastInsertIDErr
}

func (r MockResult) RowsAffected() (int64, error) {
	return r.RowsAffectedVal, r.RowsAffectedErr
}
