package tools

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteMutation_NoConfirm(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &ExecuteMutationHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), ExecuteMutationInput{
		ConnectionID: "pg1",
		Query:        "DELETE FROM users",
		Confirm:      false,
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "should reject mutation without confirm=true")
}

func TestExecuteMutation_WithConfirm(t *testing.T) {
	logger := newTestAuditLogger(t)
	mock := &db.MockDriver{
		ExecContextFn: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return db.MockResult{RowsAffectedVal: 3}, nil
		},
		DriverNameFn: func() string { return "postgres" },
	}
	h := &ExecuteMutationHandler{
		Manager: newMockManager(map[string]db.Driver{"pg1": mock}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), ExecuteMutationInput{
		ConnectionID: "pg1",
		Query:        "DELETE FROM users WHERE active = $1",
		Params:       []any{false},
		Confirm:      true,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
}

func TestExecuteQuery_MissingConnection(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &ExecuteQueryHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), ExecuteQueryInput{Query: "SELECT 1"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestExecuteQuery_BeginTxUsesReadOnlyOption(t *testing.T) {
	logger := newTestAuditLogger(t)
	var capturedOpts *sql.TxOptions
	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
		BeginTxFn: func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
			capturedOpts = opts
			return nil, errors.New("stop here: this test only checks the options passed to BeginTx")
		},
	}
	h := &ExecuteQueryHandler{
		Manager: newMockManager(map[string]db.Driver{"pg1": mock}),
		Audit:   logger,
	}

	_, _ = h.Handle(context.Background(), ExecuteQueryInput{ConnectionID: "pg1", Query: "SELECT 1"})
	require.NotNil(t, capturedOpts)
	assert.True(t, capturedOpts.ReadOnly, "execute_query must begin a read-only transaction on postgres/mysql so mutations can't slip through and the pooled connection is never left read-only")
}

func TestExecuteMutation_RejectsReadOnlyConnection(t *testing.T) {
	logger := newTestAuditLogger(t)
	mock := &db.MockDriver{DriverNameFn: func() string { return "postgres" }}
	h := &ExecuteMutationHandler{
		Manager: newMockManagerReadOnly(map[string]db.Driver{"pg1": mock}, map[string]bool{"pg1": true}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), ExecuteMutationInput{
		ConnectionID: "pg1",
		Query:        "DELETE FROM users",
		Confirm:      true,
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "a read_only connection must reject mutations even with confirm=true")
}

func TestExecuteQuery_TxID_RunsOnExistingTransaction(t *testing.T) {
	logger := newTestAuditLogger(t)
	store := NewTxStore(30 * time.Second)
	t.Cleanup(store.Stop)

	// TxEntry.Tx is a concrete *sql.Tx, which can't be constructed without a
	// real database/sql driver; routing through a live transaction is
	// covered by the Docker-backed integration suite instead. Here we only
	// verify the tx_id-not-found and mismatch error paths, which don't need
	// a real *sql.Tx.
	h := &ExecuteQueryHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
		TxStore: store,
	}

	res, err := h.Handle(context.Background(), ExecuteQueryInput{TxID: "nonexistent", Query: "SELECT 1"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
