package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

// ExecuteQueryHandler handles the execute_query tool.
type ExecuteQueryHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExecuteQueryHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()
	args := req.Params.Arguments

	connID := getStringArg(args, "connection_id")
	driverName := getStringArg(args, "driver")
	query := getStringArg(args, "query")
	params := getSliceArg(args, "params")
	limit := clampLimit(getIntArg(args, "limit", DefaultRowLimit))

	if connID == "" {
		return toolError("400", "missing connection_id", "connection_id is required"), nil
	}
	if query == "" {
		return toolError("400", "missing query", "query is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "execute_query",
		ConnectionID: connID,
		Driver:       driverName,
		Query:        query,
		ParamCount:   len(params),
	})

	conn, err := h.Manager.Get(connID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", connID, driverName, "503", "connection not found", err)
	}

	// Wrap in a read-only transaction
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), "503", "begin transaction failed", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Set read-only mode
	switch conn.DriverName() {
	case "postgres":
		if _, err := tx.ExecContext(ctx, "SET TRANSACTION READ ONLY"); err != nil {
			return auditErr(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), "503", "set read only failed", err)
		}
	case "mysql":
		if _, err := tx.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
			return auditErr(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), "503", "set read only failed", err)
		}
	}

	rows, err := tx.QueryContext(ctx, query, params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), "503", "query failed", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), "500", "columns failed", err)
	}

	var result []map[string]any
	rowCount := 0
	for rows.Next() && rowCount < limit {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return auditErr(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), "500", "scan failed", err)
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			// Convert []byte to string for readability
			if b, ok := vals[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = vals[i]
			}
		}
		result = append(result, row)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), "500", "rows error", err)
	}

	if result == nil {
		result = []map[string]any{}
	}

	data, _ := json.Marshal(result)
	auditSuccess(h.Audit, queryID, now, "execute_query", connID, conn.DriverName(), query, len(params), int64(rowCount))
	return mcp.NewToolResultText(string(data)), nil
}

// ExecuteMutationHandler handles the execute_mutation tool.
type ExecuteMutationHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExecuteMutationHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()
	args := req.Params.Arguments

	// Check confirm gate FIRST, before any DB call
	confirm := getBoolArg(args, "confirm")
	if !confirm {
		return toolError("422", "mutation requires confirm: true", "set confirm=true explicitly to proceed"), nil
	}

	connID := getStringArg(args, "connection_id")
	driverName := getStringArg(args, "driver")
	query := getStringArg(args, "query")
	params := getSliceArg(args, "params")

	if connID == "" {
		return toolError("400", "missing connection_id", "connection_id is required"), nil
	}
	if query == "" {
		return toolError("400", "missing query", "query is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "execute_mutation",
		ConnectionID: connID,
		Driver:       driverName,
		Query:        query,
		ParamCount:   len(params),
	})

	conn, err := h.Manager.Get(connID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_mutation", connID, driverName, "503", "connection not found", err)
	}

	sqlResult, err := conn.ExecContext(ctx, query, params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_mutation", connID, conn.DriverName(), "503", "exec failed", err)
	}

	rowsAffected, _ := sqlResult.RowsAffected()

	auditSuccess(h.Audit, queryID, now, "execute_mutation", connID, conn.DriverName(), query, len(params), rowsAffected)

	resp := fmt.Sprintf(`{"rows_affected":%d}`, rowsAffected)
	return mcp.NewToolResultText(resp), nil
}

// ExplainQueryHandler handles the explain_query tool.
type ExplainQueryHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExplainQueryHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()
	args := req.Params.Arguments

	connID := getStringArg(args, "connection_id")
	driverName := getStringArg(args, "driver")
	query := getStringArg(args, "query")
	params := getSliceArg(args, "params")

	if connID == "" {
		return toolError("400", "missing connection_id", "connection_id is required"), nil
	}
	if query == "" {
		return toolError("400", "missing query", "query is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "explain_query",
		ConnectionID: connID,
		Driver:       driverName,
		Query:        query,
		ParamCount:   len(params),
	})

	conn, err := h.Manager.Get(connID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", connID, driverName, "503", "connection not found", err)
	}

	var explainQuery string
	switch conn.DriverName() {
	case "postgres":
		explainQuery = "EXPLAIN (FORMAT JSON) " + query
	case "mysql":
		explainQuery = "EXPLAIN FORMAT=JSON " + query
	default:
		return toolError("400", "unsupported driver", conn.DriverName()), nil
	}

	rows, err := conn.QueryContext(ctx, explainQuery, params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", connID, conn.DriverName(), "503", "explain failed", err)
	}
	defer rows.Close()

	var plan string
	if rows.Next() {
		if err := rows.Scan(&plan); err != nil {
			return auditErr(h.Audit, queryID, now, "explain_query", connID, conn.DriverName(), "500", "scan failed", err)
		}
	}
	if err := rows.Err(); err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", connID, conn.DriverName(), "500", "rows error", err)
	}

	auditSuccess(h.Audit, queryID, now, "explain_query", connID, conn.DriverName(), query, len(params), 0)
	return mcp.NewToolResultText(plan), nil
}
