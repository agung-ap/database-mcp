package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExecuteQueryInput is the input for the execute_query tool.
type ExecuteQueryInput struct {
	ConnectionID string `json:"connection_id,omitempty" jsonschema:"Named connection alias from config (omit when tx_id is set)"`
	Query        string `json:"query" jsonschema:"SQL SELECT query to execute"`
	Params       []any  `json:"params,omitempty" jsonschema:"Positional query parameters"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max rows to return (default 100, max 1000)"`
	TxID         string `json:"tx_id,omitempty" jsonschema:"Run inside an open transaction returned by begin_transaction"`
}

// ExecuteQueryHandler handles the execute_query tool.
type ExecuteQueryHandler struct {
	Manager DBManager
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *ExecuteQueryHandler) Handle(ctx context.Context, in ExecuteQueryInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()
	limit := clampLimit(in.Limit)

	if in.Query == "" {
		return errResult("400", "missing query", "query is required"), nil
	}

	var q querier
	var connID, driverName string

	if in.TxID != "" {
		entry, ok := h.TxStore.Touch(in.TxID)
		if !ok {
			return errResult("400", "tx_id not found", fmt.Sprintf("no active transaction with id %q", in.TxID)), nil
		}
		if in.ConnectionID != "" && in.ConnectionID != entry.ConnID {
			return errResult("400", "connection_id mismatch", fmt.Sprintf("tx_id %q belongs to connection %q", in.TxID, entry.ConnID)), nil
		}
		q, connID, driverName = entry.Tx, entry.ConnID, entry.Driver
	} else {
		if in.ConnectionID == "" {
			return errResult("400", "missing connection_id", "connection_id is required"), nil
		}
		conn, err := h.Manager.Get(in.ConnectionID)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, "", "503", "connection not found", err)
		}

		txOpts := &sql.TxOptions{}
		switch conn.DriverName() {
		case "postgres", "mysql":
			// ReadOnly is set on BEGIN itself, so the guard applies atomically and
			// the session is never left in a read-only state after the tx ends
			// (unlike a separate SET ... READ ONLY statement, which poisons the
			// pooled connection on mysql).
			txOpts.ReadOnly = true
		}

		tx, err := conn.BeginTx(ctx, txOpts)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "execute_query", in.ConnectionID, conn.DriverName(), "503", "begin transaction failed", err)
		}
		defer func() { _ = tx.Rollback() }()
		q, connID, driverName = tx, in.ConnectionID, conn.DriverName()
	}

	rows, err := q.QueryContext(ctx, in.Query, in.Params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", connID, driverName, "503", "query failed", err)
	}
	defer func() { _ = rows.Close() }()

	result, truncated, err := scanAllRows(rows, limit)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_query", connID, driverName, "500", "scan failed", err)
	}
	if result == nil {
		result = []map[string]any{}
	}

	envelope := struct {
		Rows      []map[string]any `json:"rows"`
		RowCount  int              `json:"row_count"`
		Truncated bool             `json:"truncated"`
	}{Rows: result, RowCount: len(result), Truncated: truncated}

	data, _ := json.Marshal(envelope)
	auditSuccess(h.Audit, queryID, now, "execute_query", connID, driverName, in.Query, len(in.Params), int64(len(result)))
	return textResult(string(data)), nil
}

// ExecuteMutationInput is the input for the execute_mutation tool.
type ExecuteMutationInput struct {
	ConnectionID string `json:"connection_id,omitempty" jsonschema:"Named connection alias from config (omit when tx_id is set)"`
	Query        string `json:"query" jsonschema:"SQL mutation statement (INSERT/UPDATE/DELETE)"`
	Params       []any  `json:"params,omitempty" jsonschema:"Positional query parameters"`
	Confirm      bool   `json:"confirm" jsonschema:"Must be explicitly true to execute the mutation"`
	TxID         string `json:"tx_id,omitempty" jsonschema:"Run inside an open transaction returned by begin_transaction"`
}

// ExecuteMutationHandler handles the execute_mutation tool.
type ExecuteMutationHandler struct {
	Manager DBManager
	Audit   *audit.Logger
	TxStore *TxStore
}

func (h *ExecuteMutationHandler) Handle(ctx context.Context, in ExecuteMutationInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	// Gate check BEFORE any DB call
	if !in.Confirm {
		return errResult("422", "mutation requires confirm: true", "set confirm=true explicitly to proceed"), nil
	}
	if in.Query == "" {
		return errResult("400", "missing query", "query is required"), nil
	}

	var q querier
	var connID, driverName string

	if in.TxID != "" {
		// begin_transaction already rejected read-only connections, so a
		// tx_id in the store is guaranteed to belong to a writable connection.
		entry, ok := h.TxStore.Touch(in.TxID)
		if !ok {
			return errResult("400", "tx_id not found", fmt.Sprintf("no active transaction with id %q", in.TxID)), nil
		}
		if in.ConnectionID != "" && in.ConnectionID != entry.ConnID {
			return errResult("400", "connection_id mismatch", fmt.Sprintf("tx_id %q belongs to connection %q", in.TxID, entry.ConnID)), nil
		}
		q, connID, driverName = entry.Tx, entry.ConnID, entry.Driver
	} else {
		if in.ConnectionID == "" {
			return errResult("400", "missing connection_id", "connection_id is required"), nil
		}
		conn, err := h.Manager.Get(in.ConnectionID)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "execute_mutation", in.ConnectionID, "", "503", "connection not found", err)
		}
		if h.Manager.IsReadOnly(in.ConnectionID) {
			return auditErr(h.Audit, queryID, now, "execute_mutation", in.ConnectionID, conn.DriverName(), "403",
				"connection is read-only", fmt.Errorf("connection %q is configured as read_only: mutations are not permitted", in.ConnectionID))
		}
		q, connID, driverName = conn, in.ConnectionID, conn.DriverName()
	}

	sqlResult, err := q.ExecContext(ctx, in.Query, in.Params...)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "execute_mutation", connID, driverName, "503", "exec failed", err)
	}

	rowsAffected, _ := sqlResult.RowsAffected()
	auditSuccess(h.Audit, queryID, now, "execute_mutation", connID, driverName, in.Query, len(in.Params), rowsAffected)
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
		return h.explainMSSQL(ctx, conn, in, queryID, now)
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

// explainMSSQL implements explain_query for SQL Server, which requires
// SET SHOWPLAN_XML ON, the query itself, and SET SHOWPLAN_XML OFF to each run
// as their own batch on the same session — a single semicolon-joined
// statement (as previously attempted) is rejected by the driver. A dedicated
// *sql.Conn keeps all three batches pinned to one backend connection.
func (h *ExplainQueryHandler) explainMSSQL(ctx context.Context, conn db.Driver, in ExplainQueryInput, queryID string, now time.Time) (*mcp.CallToolResult, error) {
	sqlConn, err := conn.Conn(ctx)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "503", "get connection failed", err)
	}
	defer func() { _ = sqlConn.Close() }()

	if _, err := sqlConn.ExecContext(ctx, "SET SHOWPLAN_XML ON"); err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "503", "set showplan_xml on failed", err)
	}

	rows, err := sqlConn.QueryContext(ctx, in.Query, in.Params...)
	if err != nil {
		_, _ = sqlConn.ExecContext(ctx, "SET SHOWPLAN_XML OFF")
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "503", "explain failed", err)
	}

	var plan string
	if rows.Next() {
		if scanErr := rows.Scan(&plan); scanErr != nil {
			rows.Close()
			_, _ = sqlConn.ExecContext(ctx, "SET SHOWPLAN_XML OFF")
			return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "500", "scan failed", scanErr)
		}
	}
	rowsErr := rows.Err()
	rows.Close()

	if _, err := sqlConn.ExecContext(ctx, "SET SHOWPLAN_XML OFF"); err != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "503", "set showplan_xml off failed", err)
	}
	if rowsErr != nil {
		return auditErr(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), "500", "rows error", rowsErr)
	}

	auditSuccess(h.Audit, queryID, now, "explain_query", in.ConnectionID, conn.DriverName(), in.Query, len(in.Params), 0)
	return textResult(plan), nil
}
