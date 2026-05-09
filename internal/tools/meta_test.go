package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAuditLogger(t *testing.T) *audit.Logger {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "audit-*.log")
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	logger, err := audit.New(f.Name())
	require.NoError(t, err)
	t.Cleanup(func() { logger.Close() })
	return logger
}

// mockManager is a simple Manager mock for testing.
type mockManager struct {
	drivers map[string]db.Driver
}

func (m *mockManager) Get(connectionID string) (db.Driver, error) {
	if drv, ok := m.drivers[connectionID]; ok {
		return drv, nil
	}
	return nil, errors.New("unknown connection: " + connectionID)
}

func newMockManager(drivers map[string]db.Driver) *mockManager {
	return &mockManager{drivers: drivers}
}

func makeCallToolRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// resultText extracts the text from the first content item of a CallToolResult.
func resultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	switch v := result.Content[0].(type) {
	case mcp.TextContent:
		return v.Text
	case mcp.EmbeddedResource:
		return fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("%v", result.Content[0])
}

func TestListConnectionsHandler_Handle(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &ListConnectionsHandler{
		Connections: []ConnectionInfo{
			{ID: "conn1", Driver: "postgres"},
			{ID: "conn2", Driver: "mysql"},
		},
		Audit: auditLogger,
	}

	req := makeCallToolRequest(map[string]any{})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(result)
	assert.Contains(t, text, "conn1")
	assert.Contains(t, text, "postgres")
	assert.Contains(t, text, "conn2")
	assert.Contains(t, text, "mysql")
}

func TestTestConnectionHandler_Handle_Success(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		PingContextFn: func(ctx context.Context) error { return nil },
		DriverNameFn:  func() string { return "postgres" },
	}

	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(result)
	assert.Contains(t, text, `"ok":true`)
	assert.Contains(t, text, "myconn")
}

func TestTestConnectionHandler_Handle_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestTestConnectionHandler_Handle_UnknownConnection(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "nonexistent",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestTestConnectionHandler_Handle_PingError(t *testing.T) {
	auditLogger := newTestAuditLogger(t)

	mock := &db.MockDriver{
		PingContextFn: func(ctx context.Context) error { return errors.New("connection refused") },
		DriverNameFn:  func() string { return "postgres" },
	}

	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}
