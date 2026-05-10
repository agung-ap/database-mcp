package tools

import (
	"context"
	"database/sql"
	"testing"

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

	res, err := h.Handle(context.Background(), ExecuteQueryInput{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
