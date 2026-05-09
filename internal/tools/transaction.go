package tools

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
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

// BeginTransactionHandler handles the begin_transaction tool.
type BeginTransactionHandler struct {
	Manager DBManager
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *BeginTransactionHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()
	args := req.Params.Arguments

	connID := getStringArg(args, "connection_id")
	driverName := getStringArg(args, "driver")

	if connID == "" {
		return toolError("400", "missing connection_id", "connection_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "begin_transaction",
		ConnectionID: connID,
		Driver:       driverName,
	})

	conn, err := h.Manager.Get(connID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "begin_transaction", connID, driverName, "503", "connection not found", err)
	}

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return auditErr(h.Audit, queryID, now, "begin_transaction", connID, conn.DriverName(), "503", "begin transaction failed", err)
	}

	txID := h.TxStore.Add(connID, conn.DriverName(), tx)

	auditSuccess(h.Audit, queryID, now, "begin_transaction", connID, conn.DriverName(), "", 0, 0)

	resp := fmt.Sprintf(`{"tx_id":%q}`, txID)
	return mcp.NewToolResultText(resp), nil
}

// CommitTransactionHandler handles the commit_transaction tool.
type CommitTransactionHandler struct {
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *CommitTransactionHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()
	args := req.Params.Arguments

	txID := getStringArg(args, "tx_id")
	if txID == "" {
		return toolError("400", "missing tx_id", "tx_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:   queryID,
		Timestamp: now,
		Tool:      "commit_transaction",
	})

	entry, ok := h.TxStore.Get(txID)
	if !ok {
		return toolError("400", "tx_id not found", fmt.Sprintf("no active transaction with id %q", txID)), nil
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
		return toolError("503", "commit failed", err.Error()), nil
	}

	h.TxStore.Remove(txID)
	auditSuccess(h.Audit, queryID, now, "commit_transaction", entry.ConnID, entry.Driver, "", 0, 0)

	return mcp.NewToolResultText(`{"committed":true}`), nil
}

// RollbackTransactionHandler handles the rollback_transaction tool.
type RollbackTransactionHandler struct {
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *RollbackTransactionHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()
	args := req.Params.Arguments

	txID := getStringArg(args, "tx_id")
	if txID == "" {
		return toolError("400", "missing tx_id", "tx_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:   queryID,
		Timestamp: now,
		Tool:      "rollback_transaction",
	})

	entry, ok := h.TxStore.Get(txID)
	if !ok {
		return toolError("400", "tx_id not found", fmt.Sprintf("no active transaction with id %q", txID)), nil
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
		return toolError("503", "rollback failed", err.Error()), nil
	}

	h.TxStore.Remove(txID)
	auditSuccess(h.Audit, queryID, now, "rollback_transaction", entry.ConnID, entry.Driver, "", 0, 0)

	return mcp.NewToolResultText(`{"rolled_back":true}`), nil
}
