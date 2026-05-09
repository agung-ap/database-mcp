package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
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

// GetTableStatsHandler handles the get_table_stats tool.
type GetTableStatsHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *GetTableStatsHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()
	args := req.Params.Arguments

	connID := getStringArg(args, "connection_id")
	driverName := getStringArg(args, "driver")
	tableName := getStringArg(args, "table")
	schemaName := getStringArg(args, "schema")

	if connID == "" {
		return toolError("400", "missing connection_id", "connection_id is required"), nil
	}
	if tableName == "" {
		return toolError("400", "missing table", "table is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "get_table_stats",
		ConnectionID: connID,
		Driver:       driverName,
	})

	conn, err := h.Manager.Get(connID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "get_table_stats", connID, driverName, "503", "connection not found", err)
	}

	var stats TableStats
	stats.TableName = tableName

	switch conn.DriverName() {
	case "postgres":
		if schemaName == "" {
			schemaName = "public"
		}
		stats.Schema = schemaName

		q := `SELECT
				s.n_live_tup,
				s.n_dead_tup,
				pg_total_relation_size(c.oid),
				s.last_analyze
			FROM pg_stat_user_tables s
			JOIN pg_class c ON c.relname = s.relname
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE s.schemaname = $1 AND s.relname = $2`

		rows, err := conn.QueryContext(ctx, q, schemaName, tableName)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "get_table_stats", connID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				slog.Debug("failed to close rows", "error", err)
			}
		}()

		if rows.Next() {
			var lastAnalyze sql.NullTime
			if err := rows.Scan(&stats.LiveTuples, &stats.DeadTuples, &stats.SizeBytes, &lastAnalyze); err != nil {
				return auditErr(h.Audit, queryID, now, "get_table_stats", connID, conn.DriverName(), "500", "scan failed", err)
			}
			stats.RowCount = stats.LiveTuples
			if lastAnalyze.Valid {
				s := lastAnalyze.Time.Format(time.RFC3339)
				stats.LastAnalyzed = &s
			}
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "get_table_stats", connID, conn.DriverName(), "500", "rows error", err)
		}

	case "mysql":
		q := `SELECT TABLE_ROWS, DATA_LENGTH + INDEX_LENGTH, CREATE_TIME
			FROM information_schema.TABLES
			WHERE TABLE_NAME = ? AND TABLE_SCHEMA = DATABASE()`

		rows, err := conn.QueryContext(ctx, q, tableName)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "get_table_stats", connID, conn.DriverName(), "503", "query failed", err)
		}

		defer func() {
			if err := rows.Close(); err != nil {
				slog.Debug("failed to close rows", "error", err)
			}
		}()

		if rows.Next() {
			var rowCount sql.NullInt64
			var sizeBytes sql.NullInt64
			var createTime sql.NullTime
			if err := rows.Scan(&rowCount, &sizeBytes, &createTime); err != nil {
				return auditErr(h.Audit, queryID, now, "get_table_stats", connID, conn.DriverName(), "500", "scan failed", err)
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
			return auditErr(h.Audit, queryID, now, "get_table_stats", connID, conn.DriverName(), "500", "rows error", err)
		}

	default:
		return toolError("400", "unsupported driver", conn.DriverName()), nil
	}

	data, _ := json.Marshal(stats)
	auditSuccess(h.Audit, queryID, now, "get_table_stats", connID, conn.DriverName(), "", 0, 0)
	return mcp.NewToolResultText(string(data)), nil
}
