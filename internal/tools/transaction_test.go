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

func TestTxStore_ClaimIsAtomicGetAndRemove(t *testing.T) {
	store := NewTxStore(30 * time.Second)
	t.Cleanup(store.Stop)

	id := store.Add("conn1", "postgres", nil)

	entry, ok := store.Claim(id)
	require.True(t, ok)
	assert.Equal(t, "conn1", entry.ConnID)

	// A second claim on the same id must fail: the first claim already
	// removed it, preventing a double-commit/rollback race.
	_, ok = store.Claim(id)
	assert.False(t, ok)
}

func TestTxStore_TouchExtendsExpiry(t *testing.T) {
	store := NewTxStore(50 * time.Millisecond)
	t.Cleanup(store.Stop)

	id := store.Add("conn1", "postgres", nil)
	entry, ok := store.Get(id)
	require.True(t, ok)
	firstExpiry := entry.ExpiresAt

	time.Sleep(10 * time.Millisecond)
	touched, ok := store.Touch(id)
	require.True(t, ok)
	assert.True(t, touched.ExpiresAt.After(firstExpiry), "Touch should push the expiry forward")
}

func TestTxStore_Touch_NotFound(t *testing.T) {
	store := NewTxStore(30 * time.Second)
	t.Cleanup(store.Stop)

	_, ok := store.Touch("nonexistent")
	assert.False(t, ok)
}

func TestTxStore_StopHaltsSweeper(t *testing.T) {
	store := NewTxStore(10 * time.Millisecond)
	store.Stop()
	// Give a would-be sweep tick a chance to fire; rollbackExpired should
	// never run after Stop, so a nil Tx (which would panic on Rollback)
	// staying in the store proves the sweeper goroutine exited.
	id := store.Add("conn1", "postgres", nil)
	time.Sleep(20 * time.Millisecond)
	_, ok := store.Get(id)
	assert.True(t, ok, "sweeper should not run after Stop")
}

func TestBeginTransaction_RejectsReadOnlyConnection(t *testing.T) {
	logger := newTestAuditLogger(t)
	mock := &db.MockDriver{DriverNameFn: func() string { return "postgres" }}
	h := &BeginTransactionHandler{
		Manager: newMockManagerReadOnly(map[string]db.Driver{"pg1": mock}, map[string]bool{"pg1": true}),
		Audit:   logger,
		TxStore: NewTxStore(30 * time.Second),
	}
	res, err := h.Handle(context.Background(), BeginTransactionInput{ConnectionID: "pg1"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
