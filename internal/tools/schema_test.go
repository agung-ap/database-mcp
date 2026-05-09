package tools

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/agp/db-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDatabasesHandler_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListDatabasesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListDatabasesHandler_UnknownConnection(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListDatabasesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "nonexistent",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListDatabasesHandler_QueryError(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
		QueryContextFn: func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
			return nil, errors.New("query error")
		},
	}

	h := &ListDatabasesHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListTablesHandler_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListTablesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestDescribeTableHandler_MissingTable(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &DescribeTableHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListIndexesHandler_MissingTable(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListIndexesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListForeignKeysHandler_MissingTable(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListForeignKeysHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListDatabasesHandler_UnsupportedDriver(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "oracle" },
	}

	h := &ListDatabasesHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "oracle",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
