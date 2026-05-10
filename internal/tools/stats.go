package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TableStats holds statistics for a table.
type TableStats struct {
	TableName    string  `json:"table_name"`
	Schema       string  `json:"schema,omitempty"`
	RowCount     int64   `json:"row_count"`
	SizeBytes    int64   `json:"size_bytes,omitempty"`
	LastAnalyzed *string `json:"last_analyzed,omitempty"`
	LiveTuples   int64   `json:"live_tuples,omitempty"`
	DeadTuples   int64   `json:"dead_tuples,omitempty"`
}

// GetTableStatsInput is the input for the get_table_stats tool.
type GetTableStatsInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
	Table        string `json:"table" jsonschema:"Table name"`
	Schema       string `json:"schema,omitempty" jsonschema:"Schema name (optional)"`
}

// GetTableStatsHandler handles the get_table_stats tool.
type GetTableStatsHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *GetTableStatsHandler) Handle(ctx context.Context, in GetTableStatsInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}
	if in.Table == "" {
		return errResult("400", "missing table", "table is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "get_table_stats", ConnectionID: in.ConnectionID})

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, "", "503", "connection not found", err)
	}

	stats := TableStats{TableName: in.Table}

	switch conn.DriverName() {
	case "postgres":
		schema := in.Schema
		if schema == "" {
			schema = "public"
		}
		stats.Schema = schema
		q := `SELECT s.n_live_tup, s.n_dead_tup, pg_total_relation_size(c.oid), s.last_analyze
			FROM pg_stat_user_tables s
			JOIN pg_class c ON c.relname = s.relname
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE s.schemaname = $1 AND s.relname = $2`
		rows, err := conn.QueryContext(ctx, q, schema, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		if rows.Next() {
			var lastAnalyze sql.NullTime
			if err := rows.Scan(&stats.LiveTuples, &stats.DeadTuples, &stats.SizeBytes, &lastAnalyze); err != nil {
				return auditErr(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			stats.RowCount = stats.LiveTuples
			if lastAnalyze.Valid {
				s := lastAnalyze.Time.Format(time.RFC3339)
				stats.LastAnalyzed = &s
			}
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	case "mysql":
		q := `SELECT TABLE_ROWS, DATA_LENGTH + INDEX_LENGTH, CREATE_TIME
			FROM information_schema.TABLES
			WHERE TABLE_NAME = ? AND TABLE_SCHEMA = DATABASE()`
		rows, err := conn.QueryContext(ctx, q, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		if rows.Next() {
			var rowCount, sizeBytes sql.NullInt64
			var createTime sql.NullTime
			if err := rows.Scan(&rowCount, &sizeBytes, &createTime); err != nil {
				return auditErr(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			if rowCount.Valid {
				stats.RowCount = rowCount.Int64
			}
			if sizeBytes.Valid {
				stats.SizeBytes = sizeBytes.Int64
			}
			if createTime.Valid {
				s := createTime.Time.Format(time.RFC3339)
				stats.LastAnalyzed = &s
			}
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	default:
		return errResult("400", "unsupported driver", conn.DriverName()), nil
	}

	data, _ := json.Marshal(stats)
	auditSuccess(h.Audit, queryID, now, "get_table_stats", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(string(data)), nil
}
