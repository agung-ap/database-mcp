package tools

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
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

func (ts *TxStore) sweep() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ts.rollbackExpired()
	}
}

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
	id := newUUID()
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

// Remove deletes a transaction entry by ID.
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

// BeginTransactionInput is the input for the begin_transaction tool.
type BeginTransactionInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
}

// BeginTransactionHandler handles the begin_transaction tool.
type BeginTransactionHandler struct {
	Manager DBManager
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *BeginTransactionHandler) Handle(ctx context.Context, in BeginTransactionInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "begin_transaction", ConnectionID: in.ConnectionID})

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "begin_transaction", in.ConnectionID, "", "503", "connection not found", err)
	}

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return auditErr(h.Audit, queryID, now, "begin_transaction", in.ConnectionID, conn.DriverName(), "503", "begin transaction failed", err)
	}

	txID := h.TxStore.Add(in.ConnectionID, conn.DriverName(), tx)
	auditSuccess(h.Audit, queryID, now, "begin_transaction", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(fmt.Sprintf(`{"tx_id":%q}`, txID)), nil
}

// CommitTransactionInput is the input for the commit_transaction tool.
type CommitTransactionInput struct {
	TxID string `json:"tx_id" jsonschema:"Transaction ID returned by begin_transaction"`
}

// CommitTransactionHandler handles the commit_transaction tool.
type CommitTransactionHandler struct {
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *CommitTransactionHandler) Handle(ctx context.Context, in CommitTransactionInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.TxID == "" {
		return errResult("400", "missing tx_id", "tx_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "commit_transaction"})

	entry, ok := h.TxStore.Get(in.TxID)
	if !ok {
		return errResult("400", "tx_id not found", fmt.Sprintf("no active transaction with id %q", in.TxID)), nil
	}

	if err := entry.Tx.Commit(); err != nil {
		return auditErr(h.Audit, queryID, now, "commit_transaction", entry.ConnID, entry.Driver, "503", "commit failed", err)
	}

	h.TxStore.Remove(in.TxID)
	auditSuccess(h.Audit, queryID, now, "commit_transaction", entry.ConnID, entry.Driver, "", 0, 0)
	return textResult(`{"committed":true}`), nil
}

// RollbackTransactionInput is the input for the rollback_transaction tool.
type RollbackTransactionInput struct {
	TxID string `json:"tx_id" jsonschema:"Transaction ID returned by begin_transaction"`
}

// RollbackTransactionHandler handles the rollback_transaction tool.
type RollbackTransactionHandler struct {
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *RollbackTransactionHandler) Handle(ctx context.Context, in RollbackTransactionInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.TxID == "" {
		return errResult("400", "missing tx_id", "tx_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "rollback_transaction"})

	entry, ok := h.TxStore.Get(in.TxID)
	if !ok {
		return errResult("400", "tx_id not found", fmt.Sprintf("no active transaction with id %q", in.TxID)), nil
	}

	if err := entry.Tx.Rollback(); err != nil {
		return auditErr(h.Audit, queryID, now, "rollback_transaction", entry.ConnID, entry.Driver, "503", "rollback failed", err)
	}

	h.TxStore.Remove(in.TxID)
	auditSuccess(h.Audit, queryID, now, "rollback_transaction", entry.ConnID, entry.Driver, "", 0, 0)
	return textResult(`{"rolled_back":true}`), nil
}
