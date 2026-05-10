package tools

import (
	"context"
	"testing"

	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTableStats_MissingConnection(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &GetTableStatsHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
	}
	res, err := h.Handle(context.Background(), GetTableStatsInput{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestGetTableStats_MissingTable(t *testing.T) {
	logger := newTestAuditLogger(t)
	mock := &db.MockDriver{DriverNameFn: func() string { return "postgres" }}
	h := &GetTableStatsHandler{
		Manager: newMockManager(map[string]db.Driver{"pg1": mock}),
		Audit:   logger,
	}
	res, err := h.Handle(context.Background(), GetTableStatsInput{ConnectionID: "pg1"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
