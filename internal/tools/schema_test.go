package tools

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/agp/db-mcp/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDatabasesHandler_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListDatabasesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := ListDatabasesInput{
		ConnectionID: "",
		Driver:       "postgres",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestListDatabasesHandler_UnknownConnection(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListDatabasesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := ListDatabasesInput{
		ConnectionID: "nonexistent",
		Driver:       "postgres",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
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

	input := ListDatabasesInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestListTablesHandler_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListTablesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := ListTablesInput{
		ConnectionID: "",
		Driver:       "postgres",
		Database:     "testdb",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestDescribeTableHandler_MissingTable(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &DescribeTableHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := DescribeTableInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Table:        "",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestListIndexesHandler_MissingTable(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListIndexesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := ListIndexesInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Table:        "",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}

func TestListForeignKeysHandler_MissingTable(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListForeignKeysHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	input := ListForeignKeysInput{
		ConnectionID: "myconn",
		Driver:       "postgres",
		Table:        "",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
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

	input := ListDatabasesInput{
		ConnectionID: "myconn",
		Driver:       "oracle",
	}
	req := &mcp.CallToolRequest{}
	result, _, err := h.Handle(context.Background(), req, input)
	require.Error(t, err)
	assert.True(t, result.IsError)
}
