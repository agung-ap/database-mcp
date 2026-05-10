package tools

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DefaultRowLimit = 100
	MaxRowLimit     = 1000
)

// DBManager is the interface tool handlers use to look up connections.
type DBManager interface {
	Get(connectionID string) (db.Driver, error)
}

// textResult wraps a JSON string in a successful MCP tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// errResult returns a structured MCP tool error result.
func errResult(code, message, detail string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf(`{"code":"%s","message":"%s","detail":"%s"}`, code, message, detail),
		}},
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
// []byte values are converted to strings for JSON readability.
func scanAllRows(rows *sql.Rows, limit int) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	for rows.Next() && len(results) < limit {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
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
	return results, rows.Err()
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
