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

func TestGetTableStatsHandler_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &GetTableStatsHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"table": "users",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetTableStatsHandler_MissingTable(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &GetTableStatsHandler{
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

func TestGetTableStatsHandler_UnknownConnection(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &GetTableStatsHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "nonexistent",
		"driver":        "postgres",
		"table":         "users",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetTableStatsHandler_QueryError(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
		QueryContextFn: func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
			return nil, errors.New("query error")
		},
	}

	h := &GetTableStatsHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
		"table":         "users",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetTableStatsHandler_UnsupportedDriver(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "oracle" },
	}

	h := &GetTableStatsHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "oracle",
		"table":         "users",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
