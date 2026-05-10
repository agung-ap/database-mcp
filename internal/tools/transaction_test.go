package tools

import (
	"context"
	"testing"
	"time"

	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTxStore_AddGetRemove(t *testing.T) {
	store := NewTxStore(30 * time.Second)
	id := store.Add("conn1", "postgres", nil)
	assert.NotEmpty(t, id)

	entry, ok := store.Get(id)
	require.True(t, ok)
	assert.Equal(t, "conn1", entry.ConnID)

	store.Remove(id)
	_, ok = store.Get(id)
	assert.False(t, ok)
}

func TestBeginTransaction_MissingConnection(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &BeginTransactionHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   logger,
		TxStore: NewTxStore(30 * time.Second),
	}
	res, err := h.Handle(context.Background(), BeginTransactionInput{ConnectionID: "nope"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestCommitTransaction_NotFound(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &CommitTransactionHandler{
		Audit:   logger,
		TxStore: NewTxStore(30 * time.Second),
	}
	res, err := h.Handle(context.Background(), CommitTransactionInput{TxID: "nonexistent"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestRollbackTransaction_NotFound(t *testing.T) {
	logger := newTestAuditLogger(t)
	h := &RollbackTransactionHandler{
		Audit:   logger,
		TxStore: NewTxStore(30 * time.Second),
	}
	res, err := h.Handle(context.Background(), RollbackTransactionInput{TxID: "nonexistent"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
