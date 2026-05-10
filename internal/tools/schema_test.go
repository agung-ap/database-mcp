package tools

import (
	"context"
	"testing"

	"github.com/agp/db-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDatabases_MissingConnection(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &ListDatabasesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
	}
	res, err := h.Handle(context.Background(), ListDatabasesInput{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestListTables_UnknownConnection(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &ListTablesHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
	}
	res, err := h.Handle(context.Background(), ListTablesInput{ConnectionID: "nope"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestDescribeTable_MissingTable(t *testing.T) {
	logger := newTestAuditLogger(t)
	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
	}
	h := &DescribeTableHandler{
		Manager: newMockManager(map[string]db.Driver{"pg1": mock}),
		Audit:   logger,
	}
	res, err := h.Handle(context.Background(), DescribeTableInput{ConnectionID: "pg1"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
