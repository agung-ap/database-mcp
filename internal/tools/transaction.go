package tools

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TxEntry holds a tracked transaction with its metadata.
type TxEntry struct {
	Tx        *sql.Tx
	ConnID    string
	Driver    string
	StartedAt time.Time
	ExpiresAt time.Time
}

// TxStore tracks open transactions with TTL-based auto-rollback.
type TxStore struct {
	mu    sync.Mutex
	store map[string]*TxEntry
	ttl   time.Duration
}

// NewTxStore creates a TxStore with the given TTL and starts a background sweeper.
func NewTxStore(ttl time.Duration) *TxStore {
	ts := &TxStore{
		store: make(map[string]*TxEntry),
		ttl:   ttl,
	}
	go ts.sweep()
	return ts
}

// sweep periodically rolls back expired transactions.
func (ts *TxStore) sweep() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ts.rollbackExpired()
	}
}

// rollbackExpired rolls back and removes all expired transactions.
func (ts *TxStore) rollbackExpired() {
	now := time.Now()
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for id, entry := range ts.store {
		if now.After(entry.ExpiresAt) {
			_ = entry.Tx.Rollback()
			delete(ts.store, id)
		}
	}
}

// Add stores a new transaction entry and returns its ID.
func (ts *TxStore) Add(connID, driverName string, tx *sql.Tx) string {
	id := uuid.NewString()
	now := time.Now()
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.store[id] = &TxEntry{
		Tx:        tx,
		ConnID:    connID,
		Driver:    driverName,
		StartedAt: now,
		ExpiresAt: now.Add(ts.ttl),
	}
	return id
}

// Get retrieves a transaction entry by ID.
func (ts *TxStore) Get(txID string) (*TxEntry, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	entry, ok := ts.store[txID]
	return entry, ok
}

// Remove deletes a transaction entry by ID (after commit or rollback).
func (ts *TxStore) Remove(txID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.store, txID)
}

// RollbackAll rolls back all open transactions (called on server shutdown).
func (ts *TxStore) RollbackAll() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for id, entry := range ts.store {
		_ = entry.Tx.Rollback()
		delete(ts.store, id)
	}
}

// BeginTransactionInput is the input for begin_transaction tool.
type BeginTransactionInput struct {
	ConnectionID string `json:"connection_id"`
	Driver       string `json:"driver"`
}

// BeginTransactionHandler handles the begin_transaction tool.
type BeginTransactionHandler struct {
	Manager DBManager
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *BeginTransactionHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input BeginTransactionInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	if input.ConnectionID == "" {
		err := fmt.Errorf("missing connection_id: connection_id is required")
		return newToolError(err), nil, err
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "begin_transaction",
		ConnectionID: input.ConnectionID,
		Driver:       input.Driver,
	})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "begin_transaction",
			ConnectionID: input.ConnectionID,
			Driver:       input.Driver,
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return newToolError(err), nil, err
	}

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "begin_transaction",
			ConnectionID: input.ConnectionID,
			Driver:       conn.DriverName(),
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return newToolError(err), nil, err
	}

	txID := h.TxStore.Add(input.ConnectionID, conn.DriverName(), tx)

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         "begin_transaction",
		ConnectionID: input.ConnectionID,
		Driver:       conn.DriverName(),
		DurationMS:   time.Since(now).Milliseconds(),
		Success:      true,
	})

	resp := fmt.Sprintf(`{"tx_id":%q}`, txID)
	return newToolTextResult(resp), nil, nil
}

// CommitTransactionInput is the input for commit_transaction tool.
type CommitTransactionInput struct {
	TxID string `json:"tx_id"`
}

// CommitTransactionHandler handles the commit_transaction tool.
type CommitTransactionHandler struct {
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *CommitTransactionHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input CommitTransactionInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	if input.TxID == "" {
		err := fmt.Errorf("missing tx_id: tx_id is required")
		return newToolError(err), nil, err
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:   queryID,
		Timestamp: now,
		Tool:      "commit_transaction",
	})

	entry, ok := h.TxStore.Get(input.TxID)
	if !ok {
		err := fmt.Errorf("tx_id not found: no active transaction with id %q", input.TxID)
		h.Audit.Log(audit.AuditEntry{
			QueryID:    queryID,
			Timestamp:  time.Now(),
			Tool:       "commit_transaction",
			DurationMS: time.Since(now).Milliseconds(),
			Success:    false,
			Error:      err.Error(),
		})
		return newToolError(err), nil, err
	}

	if err := entry.Tx.Commit(); err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "commit_transaction",
			ConnectionID: entry.ConnID,
			Driver:       entry.Driver,
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return newToolError(err), nil, err
	}

	h.TxStore.Remove(input.TxID)
	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         "commit_transaction",
		ConnectionID: entry.ConnID,
		Driver:       entry.Driver,
		DurationMS:   time.Since(now).Milliseconds(),
		Success:      true,
	})

	return newToolTextResult(`{"committed":true}`), nil, nil
}

// RollbackTransactionInput is the input for rollback_transaction tool.
type RollbackTransactionInput struct {
	TxID string `json:"tx_id"`
}

// RollbackTransactionHandler handles the rollback_transaction tool.
type RollbackTransactionHandler struct {
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *RollbackTransactionHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input RollbackTransactionInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	if input.TxID == "" {
		err := fmt.Errorf("missing tx_id: tx_id is required")
		return newToolError(err), nil, err
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:   queryID,
		Timestamp: now,
		Tool:      "rollback_transaction",
	})

	entry, ok := h.TxStore.Get(input.TxID)
	if !ok {
		err := fmt.Errorf("tx_id not found: no active transaction with id %q", input.TxID)
		h.Audit.Log(audit.AuditEntry{
			QueryID:    queryID,
			Timestamp:  time.Now(),
			Tool:       "rollback_transaction",
			DurationMS: time.Since(now).Milliseconds(),
			Success:    false,
			Error:      err.Error(),
		})
		return newToolError(err), nil, err
	}

	if err := entry.Tx.Rollback(); err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "rollback_transaction",
			ConnectionID: entry.ConnID,
			Driver:       entry.Driver,
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return newToolError(err), nil, err
	}

	h.TxStore.Remove(input.TxID)
	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         "rollback_transaction",
		ConnectionID: entry.ConnID,
		Driver:       entry.Driver,
		DurationMS:   time.Since(now).Milliseconds(),
		Success:      true,
	})

	return newToolTextResult(`{"rolled_back":true}`), nil, nil
}
