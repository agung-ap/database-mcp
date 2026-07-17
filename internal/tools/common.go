package tools

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DefaultRowLimit = 100
	MaxRowLimit     = 1000
)

// DBManager is the interface tool handlers use to look up connections.
type DBManager interface {
	Get(connectionID string) (db.Driver, error)
	IsReadOnly(connectionID string) bool
}

// querier is satisfied by both db.Driver and *sql.Tx, letting query/mutation
// handlers run against either a pooled connection or an open transaction
// through the same code path.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// textResult wraps a JSON string in a successful MCP tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// errResult returns a structured MCP tool error result.
func errResult(code, message, detail string) *mcp.CallToolResult {
	data, err := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}{Code: code, Message: message, Detail: detail})
	if err != nil {
		data = []byte(`{"code":"500","message":"internal error","detail":"failed to encode error"}`)
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}
}

// clampLimit enforces the row limit constraints.
func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultRowLimit
	}
	if limit > MaxRowLimit {
		return MaxRowLimit
	}
	return limit
}

// newUUID generates a random UUID v4 using only stdlib.
func newUUID() string {
	var b [16]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// scanAllRows scans up to limit rows from sql.Rows into a slice of maps.
// []byte values are converted to strings for JSON readability. The second
// return value reports whether more rows existed beyond the limit, so
// callers can tell a full result set from a truncated one.
func scanAllRows(rows *sql.Rows, limit int) ([]map[string]any, bool, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, false, err
	}
	var results []map[string]any
	for rows.Next() {
		if len(results) >= limit {
			return results, true, rows.Err()
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, false, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = vals[i]
			}
		}
		results = append(results, row)
	}
	return results, false, rows.Err()
}

// auditErr logs a failure and returns a tool error result.
func auditErr(logger *audit.Logger, queryID string, start time.Time, tool, connID, driver, code, msg string, err error) (*mcp.CallToolResult, error) {
	logger.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         tool,
		ConnectionID: connID,
		Driver:       driver,
		DurationMS:   time.Since(start).Milliseconds(),
		Success:      false,
		Error:        err.Error(),
	})
	return errResult(code, msg, err.Error()), nil
}

// auditSuccess logs a successful operation.
func auditSuccess(logger *audit.Logger, queryID string, start time.Time, tool, connID, driver, query string, paramCount int, rowsAffected int64) {
	logger.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         tool,
		ConnectionID: connID,
		Driver:       driver,
		Query:        query,
		ParamCount:   paramCount,
		DurationMS:   time.Since(start).Milliseconds(),
		RowsAffected: rowsAffected,
		Success:      true,
	})
}
