package tools

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAuditLogger(t *testing.T) *audit.Logger {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "audit-*.log")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	logger, err := audit.New(f.Name())
	require.NoError(t, err)
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}

type mockManager struct {
	drivers  map[string]db.Driver
	readOnly map[string]bool
}

func (m *mockManager) Get(connectionID string) (db.Driver, error) {
	if drv, ok := m.drivers[connectionID]; ok {
		return drv, nil
	}
	return nil, errors.New("unknown connection: " + connectionID)
}

func (m *mockManager) IsReadOnly(connectionID string) bool {
	return m.readOnly[connectionID]
}

func newMockManager(drivers map[string]db.Driver) *mockManager {
	return &mockManager{drivers: drivers, readOnly: map[string]bool{}}
}

func newMockManagerReadOnly(drivers map[string]db.Driver, readOnly map[string]bool) *mockManager {
	return &mockManager{drivers: drivers, readOnly: readOnly}
}


func TestListConnections(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &ListConnectionsHandler{
		Connections: []ConnectionInfo{
			{ID: "pg1", Driver: "postgres"},
			{ID: "my1", Driver: "mysql"},
		},
		Audit: logger,
	}

	res, err := h.Handle(context.Background(), ListConnectionsInput{})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
}

func TestTestConnection_Success(t *testing.T) {
	logger := newTestAuditLogger(t)
	mock := &db.MockDriver{
		PingContextFn: func(ctx context.Context) error { return nil },
		DriverNameFn:  func() string { return "postgres" },
	}
	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{"pg1": mock}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), TestConnectionInput{ConnectionID: "pg1"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
}

func TestTestConnection_NotFound(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), TestConnectionInput{ConnectionID: "missing"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestTestConnection_MissingID(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), TestConnectionInput{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestTestConnection_PingFails(t *testing.T) {
	logger := newTestAuditLogger(t)
	mock := &db.MockDriver{
		PingContextFn: func(ctx context.Context) error { return errors.New("timeout") },
		DriverNameFn:  func() string { return "postgres" },
	}
	h := &TestConnectionHandler{
		Manager: newMockManager(map[string]db.Driver{"pg1": mock}),
		Audit:   logger,
	}

	res, err := h.Handle(context.Background(), TestConnectionInput{ConnectionID: "pg1"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
