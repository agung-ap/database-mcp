package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExecuteQueryInput is the input for the execute_query tool.
type ExecuteQueryInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
	Query        string `json:"query" jsonschema:"SQL SELECT query to execute"`
	Params       []any  `json:"params,omitempty" jsonschema:"Positional query parameters"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max rows to return (default 100, max 1000)"`
}

// ExecuteQueryHandler handles the execute_query tool.
type ExecuteQueryHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExecuteQueryHandler) Handle(ctx context.Context, in ExecuteQueryInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()
	limit := clampLimit(in.Limit)

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}
	if in.Query == "" {
		return errResult("400", "missing query", "query is required"), nil
	}

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, "", "503", "connection not found", err)
	}

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, conn.DriverName(), "503", "begin transaction failed", err)
	}
	defer func() { _ = tx.Rollback() }()

	switch conn.DriverName() {
	case "postgres":
		if _, err := tx.ExecContext(ctx, "SET TRANSACTION READ ONLY"); err != nil {
			return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, conn.DriverName(), "503", "set read only failed", err)
		}
	case "mysql":
		if _, err := tx.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
			return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, conn.DriverName(), "503", "set read only failed", err)
		}
	}

	rows, err := tx.QueryContext(ctx, in.Query, in.Params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
	}
	defer func() { _ = rows.Close() }()

	result, err := scanAllRows(rows, limit)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
	}
	if result == nil {
		result = []map[string]any{}
	}

	data, _ := json.Marshal(result)
	auditSuccess(h.Audit, queryID, now, "execute_query", in.ConnectionID, conn.DriverName(), in.Query, len(in.Params), int64(len(result)))
	return textResult(string(data)), nil
}

// ExecuteMutationInput is the input for the execute_mutation tool.
type ExecuteMutationInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
	Query        string `json:"query" jsonschema:"SQL mutation statement (INSERT/UPDATE/DELETE)"`
	Params       []any  `json:"params,omitempty" jsonschema:"Positional query parameters"`
	Confirm      bool   `json:"confirm" jsonschema:"Must be explicitly true to execute the mutation"`
}

// ExecuteMutationHandler handles the execute_mutation tool.
type ExecuteMutationHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExecuteMutationHandler) Handle(ctx context.Context, in ExecuteMutationInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	// Gate check BEFORE any DB call
	if !in.Confirm {
		return errResult("422", "mutation requires confirm: true", "set confirm=true explicitly to proceed"), nil
	}
	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}
	if in.Query == "" {
		return errResult("400", "missing query", "query is required"), nil
	}

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_mutation", in.ConnectionID, "", "503", "connection not found", err)
	}

	sqlResult, err := conn.ExecContext(ctx, in.Query, in.Params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_mutation", in.ConnectionID, conn.DriverName(), "503", "exec failed", err)
	}

	rowsAffected, _ := sqlResult.RowsAffected()
	auditSuccess(h.Audit, queryID, now, "execute_mutation", in.ConnectionID, conn.DriverName(), in.Query, len(in.Params), rowsAffected)
	return textResult(fmt.Sprintf(`{"rows_affected":%d}`, rowsAffected)), nil
}

// ExplainQueryInput is the input for the explain_query tool.
type ExplainQueryInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
	Query        string `json:"query" jsonschema:"SQL query to explain"`
	Params       []any  `json:"params,omitempty" jsonschema:"Positional query parameters"`
}

// ExplainQueryHandler handles the explain_query tool.
type ExplainQueryHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExplainQueryHandler) Handle(ctx context.Context, in ExplainQueryInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}
	if in.Query == "" {
		return errResult("400", "missing query", "query is required"), nil
	}

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, "", "503", "connection not found", err)
	}

	var explainQuery string
	switch conn.DriverName() {
	case "postgres":
		explainQuery = "EXPLAIN (FORMAT JSON) " + in.Query
	case "mysql":
		explainQuery = "EXPLAIN FORMAT=JSON " + in.Query
	case "sqlserver":
		explainQuery = "SET SHOWPLAN_XML ON; " + in.Query + "; SET SHOWPLAN_XML OFF;"
	default:
		return errResult("400", "unsupported driver", conn.DriverName()), nil
	}

	rows, err := conn.QueryContext(ctx, explainQuery, in.Params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "503", "explain failed", err)
	}
	defer func() { _ = rows.Close() }()

	var plan string
	if rows.Next() {
		if err := rows.Scan(&plan); err != nil {
			return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
		}
	}
	if err := rows.Err(); err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
	}

	auditSuccess(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), in.Query, len(in.Params), 0)
	return textResult(plan), nil
}
