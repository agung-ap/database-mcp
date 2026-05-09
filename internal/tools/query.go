package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExecuteQueryInput is the input for execute_query tool.
type ExecuteQueryInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"required,description=Named connection alias from config"`
	Driver       string `json:"driver" jsonschema:"required,description=Database driver"`
	Query        string `json:"query" jsonschema:"required,description=SQL query to execute"`
	Params       []any  `json:"params" jsonschema:"description=Query parameters (optional)"`
	Limit        int    `json:"limit" jsonschema:"description=Max rows to return (default 100)"`
}

// ExecuteQueryHandler handles the execute_query tool.
type ExecuteQueryHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExecuteQueryHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input ExecuteQueryInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()
	limit := clampLimit(input.Limit)

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "execute_query",
		ConnectionID: input.ConnectionID,
		Driver:       input.Driver,
		Query:        input.Query,
		ParamCount:   len(input.Params),
	})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_query", ConnectionID: input.ConnectionID, Driver: input.Driver, DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}

	// Wrap in read-only transaction
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: false})
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Set read-only mode
	switch conn.DriverName() {
	case "postgres":
		_, _ = tx.ExecContext(ctx, "SET TRANSACTION READ ONLY")
	case "mysql":
		_, _ = tx.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY")
	}

	rows, err := tx.QueryContext(ctx, input.Query, input.Params...)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}
	defer func() {
		if e := rows.Close(); e != nil {
			slog.Debug("failed to close rows", "error", e)
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
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
			h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
			return newToolError(err), nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
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
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}

	if result == nil {
		result = []map[string]any{}
	}

	data, _ := json.Marshal(result)
	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true, RowsAffected: int64(rowCount)})
	return newToolTextResult(string(data)), nil, nil
}

// ExecuteMutationInput is the input for execute_mutation tool.
type ExecuteMutationInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"required,description=Named connection alias from config"`
	Driver       string `json:"driver" jsonschema:"required,description=Database driver"`
	Query        string `json:"query" jsonschema:"required,description=SQL mutation to execute"`
	Params       []any  `json:"params" jsonschema:"description=Query parameters (optional)"`
	Confirm      bool   `json:"confirm" jsonschema:"required,description=Must be true to execute mutation"`
}

// ExecuteMutationHandler handles the execute_mutation tool.
type ExecuteMutationHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExecuteMutationHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input ExecuteMutationInput) (*mcp.CallToolResult, any, error) {
	// Check confirm gate FIRST
	if !input.Confirm {
		err := fmt.Errorf("mutation requires confirm: true")
		return newToolError(err), nil, err
	}

	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "execute_mutation",
		ConnectionID: input.ConnectionID,
		Driver:       input.Driver,
		Query:        input.Query,
		ParamCount:   len(input.Params),
	})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_mutation", ConnectionID: input.ConnectionID, Driver: input.Driver, DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}

	sqlResult, err := conn.ExecContext(ctx, input.Query, input.Params...)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_mutation", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}

	rowsAffected, _ := sqlResult.RowsAffected()
	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "execute_mutation", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true, RowsAffected: rowsAffected})

	resp := fmt.Sprintf(`{"rows_affected":%d}`, rowsAffected)
	return newToolTextResult(resp), nil, nil
}

// ExplainQueryInput is the input for explain_query tool.
type ExplainQueryInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"required,description=Named connection alias from config"`
	Driver       string `json:"driver" jsonschema:"required,description=Database driver"`
	Query        string `json:"query" jsonschema:"required,description=SQL query to explain"`
	Params       []any  `json:"params" jsonschema:"description=Query parameters (optional)"`
}

// ExplainQueryHandler handles the explain_query tool.
type ExplainQueryHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ExplainQueryHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input ExplainQueryInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "explain_query",
		ConnectionID: input.ConnectionID,
		Driver:       input.Driver,
		Query:        input.Query,
		ParamCount:   len(input.Params),
	})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "explain_query", ConnectionID: input.ConnectionID, Driver: input.Driver, DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}

	var explainQuery string
	switch conn.DriverName() {
	case "postgres":
		explainQuery = "EXPLAIN (FORMAT JSON) " + input.Query
	case "mysql":
		explainQuery = "EXPLAIN FORMAT=JSON " + input.Query
	default:
		err := fmt.Errorf("unsupported driver: %s", conn.DriverName())
		return newToolError(err), nil, err
	}

	rows, err := conn.QueryContext(ctx, explainQuery, input.Params...)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "explain_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}
	defer func() {
		if e := rows.Close(); e != nil {
			slog.Debug("failed to close rows", "error", e)
		}
	}()

	var plan string
	if rows.Next() {
		if err := rows.Scan(&plan); err != nil {
			h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "explain_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
			return newToolError(err), nil, err
		}
	}
	if err := rows.Err(); err != nil {
		h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "explain_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: false, Error: err.Error()})
		return newToolError(err), nil, err
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "explain_query", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true})
	return newToolTextResult(plan), nil, nil
}
