package tools

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/agp/db-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTxStore_AddGetRemove(t *testing.T) {
	ts := &TxStore{
		store: make(map[string]*TxEntry),
		ttl:   30 * time.Second,
	}

	// Use a test db to get a real *sql.Tx
	testDB, err := sql.Open("testdrv", "test")
	require.NoError(t, err)
	defer func() {
		if err := testDB.Close(); err != nil {
			t.Errorf("failed to close test DB: %v", err)
		}
	}()

	tx, err := testDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	id := ts.Add("conn1", "postgres", tx)
	assert.NotEmpty(t, id)

	entry, ok := ts.Get(id)
	assert.True(t, ok)
	assert.Equal(t, "conn1", entry.ConnID)
	assert.Equal(t, "postgres", entry.Driver)

	ts.Remove(id)
	_, ok = ts.Get(id)
	assert.False(t, ok)
}

func TestTxStore_RollbackExpired(t *testing.T) {
	ts := &TxStore{
		store: make(map[string]*TxEntry),
		ttl:   1 * time.Millisecond,
	}

	testDB, err := sql.Open("testdrv", "test")
	require.NoError(t, err)
	defer func() {
		if err := testDB.Close(); err != nil {
			t.Errorf("failed to close test DB: %v", err)
		}
	}()

	tx, err := testDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	id := ts.Add("conn1", "postgres", tx)
	// Force expiration
	ts.mu.Lock()
	ts.store[id].ExpiresAt = time.Now().Add(-1 * time.Second)
	ts.mu.Unlock()

	ts.rollbackExpired()

	_, ok := ts.Get(id)
	assert.False(t, ok, "expired transaction should have been removed")
}

func TestTxStore_RollbackAll(t *testing.T) {
	ts := &TxStore{
		store: make(map[string]*TxEntry),
		ttl:   30 * time.Second,
	}

	testDB, err := sql.Open("testdrv", "test")
	require.NoError(t, err)
	defer func() {
		if err := testDB.Close(); err != nil {
			t.Errorf("failed to close test DB: %v", err)
		}
	}()

	tx1, err := testDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	tx2, err := testDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	ts.Add("conn1", "postgres", tx1)
	ts.Add("conn2", "mysql", tx2)

	ts.RollbackAll()

	ts.mu.Lock()
	count := len(ts.store)
	ts.mu.Unlock()
	assert.Equal(t, 0, count)
}

func TestBeginTransactionHandler_MissingConnectionID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)
	ts := &TxStore{store: make(map[string]*TxEntry), ttl: 30 * time.Second}

	h := &BeginTransactionHandler{
		Manager: newMockManager(map[string]db.Driver{}),
		Audit:   auditLogger,
		TxStore: ts,
	}

	req := makeCallToolRequest(map[string]any{})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestBeginTransactionHandler_DBError(t *testing.T) {
	auditLogger := newTestAuditLogger(t)
	ts := &TxStore{store: make(map[string]*TxEntry), ttl: 30 * time.Second}

	mock := &db.MockDriver{
		DriverNameFn: func() string { return "postgres" },
		BeginTxFn: func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
			return nil, errors.New("cannot begin tx")
		},
	}

	h := &BeginTransactionHandler{
		Manager: newMockManager(map[string]db.Driver{"myconn": mock}),
		Audit:   auditLogger,
		TxStore: ts,
	}

	req := makeCallToolRequest(map[string]any{
		"connection_id": "myconn",
		"driver":        "postgres",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestCommitTransactionHandler_NotFound(t *testing.T) {
	auditLogger := newTestAuditLogger(t)
	ts := &TxStore{store: make(map[string]*TxEntry), ttl: 30 * time.Second}

	h := &CommitTransactionHandler{
		Audit:   auditLogger,
		TxStore: ts,
	}

	req := makeCallToolRequest(map[string]any{
		"tx_id": "nonexistent-uuid",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestRollbackTransactionHandler_NotFound(t *testing.T) {
	auditLogger := newTestAuditLogger(t)
	ts := &TxStore{store: make(map[string]*TxEntry), ttl: 30 * time.Second}

	h := &RollbackTransactionHandler{
		Audit:   auditLogger,
		TxStore: ts,
	}

	req := makeCallToolRequest(map[string]any{
		"tx_id": "nonexistent-uuid",
	})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestCommitTransactionHandler_MissingTxID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)
	ts := &TxStore{store: make(map[string]*TxEntry), ttl: 30 * time.Second}

	h := &CommitTransactionHandler{
		Audit:   auditLogger,
		TxStore: ts,
	}

	req := makeCallToolRequest(map[string]any{})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestRollbackTransactionHandler_MissingTxID(t *testing.T) {
	auditLogger := newTestAuditLogger(t)
	ts := &TxStore{store: make(map[string]*TxEntry), ttl: 30 * time.Second}

	h := &RollbackTransactionHandler{
		Audit:   auditLogger,
		TxStore: ts,
	}

	req := makeCallToolRequest(map[string]any{})
	result, err := h.Handle(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
