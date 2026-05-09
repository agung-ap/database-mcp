package tools

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	"github.com/agp/db-mcp/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRows builds a *sql.Rows from a mock database/driver.
// We use a custom driver registered once.
func init() {
	sql.Register("testdrv", &testDriver{})
}

type testDriver struct{}

func (d *testDriver) Open(name string) (driver.Conn, error) {
	return &testConn{}, nil
}

type testConn struct{}

func (c *testConn) Prepare(query string) (driver.Stmt, error) { return &testStmt{}, nil }
func (c *testConn) Close() error                              { return nil }
func (c *testConn) Begin() (driver.Tx, error)                 { return &testTx{}, nil }

type testStmt struct{}

func (s *testStmt) Close() error  { return nil }
func (s *testStmt) NumInput() int { return -1 }
func (s *testStmt) Exec(args []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (s *testStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &testRows{}, nil
}

type testRows struct {
	pos int
}

func (r *testRows) Columns() []string { return []string{"id", "name"} }
func (r *testRows) Close() error      { return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.pos >= 1 {
		return io.EOF
	}
	r.pos++
	dest[0] = int64(1)
	dest[1] = "Alice"
	return nil
}

type testTx struct{}

func (t *testTx) Commit() error   { return nil }
func (t *testTx) Rollback() error { return nil }

// openTestDB opens a test DB using the fake driver.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("testdrv", "test")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

// TestExecuteQueryHandler_ConfirmNotRequired verifies read-only tx setup is attempted.
func TestExecuteQueryHandler_ReadOnlyTx(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	var beginTxCalled bool

	// Track calls: BeginTx returns a fake tx, ExecContext records the SET command
	testDB := openTestDB(t)

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
		BeginTxFn: func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
			beginTxCalled = true
			return testDB.BeginTx(ctx, opts)
		},
	}

	h := &ExecuteQueryHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	input := ExecuteQueryInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Query:        "SELECT 1",
		Limit:        10,
	}

	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.NoError(t, err)
	// Should have called BeginTx
	assert.True(t, beginTxCalled, "BeginTx should have been called")
	// Result may succeed or not (depends on test driver), but no panic
	_ = result
}

func TestExecuteQueryHandler_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ExecuteQueryHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := ExecuteQueryInput{
		Query: "SELECT 1",
	}

	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestExecuteQueryHandler_MissingQuery(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ExecuteQueryHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := ExecuteQueryInput{
		ConnectionID: "myconn",
	}

	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestExecuteQueryHandler_DBError(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
		BeginTxFn: func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
			return nil, errors.New("db connection error")
		},
	}

	h := &ExecuteQueryHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	input := ExecuteQueryInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Query:        "SELECT 1",
	}

	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestExecuteMutationHandler_ConfirmFalse(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	var execCalled bool
	mock := &db.MockDriver{
		ExecContextFn: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			execCalled = true
			return db.MockResult{RowsAffectedVal: 1}, nil
		},
	}

	h := &ExecuteMutationHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	input := ExecuteMutationInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Query:        "DELETE FROM users",
		Confirm:      false,
	}

	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	// Must be rejected
	assert.True(t, result.IsError)
	// ExecContext must NOT have been called
	assert.False(t, execCalled, "ExecContext should not be called when confirm=false")

	text := resultText(result)
	assert.Contains(t, text, "error")
}

func TestExecuteMutationHandler_ConfirmTrue(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
		ExecContextFn: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return db.MockResult{RowsAffectedVal: 3}, nil
		},
	}

	h := &ExecuteMutationHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	input := ExecuteMutationInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Query:        "DELETE FROM users WHERE id > 0",
		Confirm:      true,
	}

	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := resultText(result)
	assert.Contains(t, text, `"rows_affected":3`)
}

func TestExecuteMutationHandler_MissingConfirm(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	var execCalled bool
	mock := &db.MockDriver{
		ExecContextFn: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			execCalled = true
			return db.MockResult{}, nil
		},
	}

	h := &ExecuteMutationHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	// confirm is absent (defaults to false)
	input := ExecuteMutationInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Query:        "DELETE FROM users",
	}

	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
	assert.False(t, execCalled)
}

func TestClampLimit(t *testing.T) {
	assert.Equal(t, DefaultRowLimit, clampLimit(0))
	assert.Equal(t, DefaultRowLimit, clampLimit(-5))
	assert.Equal(t, 50, clampLimit(50))
	assert.Equal(t, MaxRowLimit, clampLimit(MaxRowLimit+1))
	assert.Equal(t, MaxRowLimit, clampLimit(99999))
	assert.Equal(t, 1, clampLimit(1))
}
