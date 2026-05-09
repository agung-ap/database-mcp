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

// GetTableStatsInput is the input for get_table_stats tool.
type GetTableStatsInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"required,description=Named connection alias from config"`
	Driver       string `json:"driver" jsonschema:"required,description=Database driver"`
	Table        string `json:"table" jsonschema:"required,description=Table name"`
	Schema       string `json:"schema" jsonschema:"description=Schema name (default: public for PostgreSQL)"`
}

// GetTableStatsHandler handles the get_table_stats tool.
type GetTableStatsHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *GetTableStatsHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input GetTableStatsInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	if input.ConnectionID == "" {
		err := fmt.Errorf("missing connection_id: connection_id is required")
		return newToolError(err), nil, err
	}
	if input.Table == "" {
		err := fmt.Errorf("missing table: table is required")
		return newToolError(err), nil, err
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "get_table_stats",
		ConnectionID: input.ConnectionID,
		Driver:       input.Driver,
	})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "get_table_stats",
			ConnectionID: input.ConnectionID,
			Driver:       input.Driver,
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return newToolError(err), nil, err
	}

	var stats TableStats
	stats.TableName = input.Table

	switch conn.DriverName() {
	case "postgres":
		schemaName := input.Schema
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

		rows, err := conn.QueryContext(ctx, q, schemaName, input.Table)
		if err != nil {
			h.Audit.Log(audit.AuditEntry{
				QueryID:      queryID,
				Timestamp:    time.Now(),
				Tool:         "get_table_stats",
				ConnectionID: input.ConnectionID,
				Driver:       conn.DriverName(),
				DurationMS:   time.Since(now).Milliseconds(),
				Success:      false,
				Error:        err.Error(),
			})
			return newToolError(err), nil, err
		}
		defer func() {
			if err := rows.Close(); err != nil {
				slog.Debug("failed to close rows", "error", err)
			}
		}()

		if rows.Next() {
			var lastAnalyze sql.NullTime
			if err := rows.Scan(&stats.LiveTuples, &stats.DeadTuples, &stats.SizeBytes, &lastAnalyze); err != nil {
				h.Audit.Log(audit.AuditEntry{
					QueryID:      queryID,
					Timestamp:    time.Now(),
					Tool:         "get_table_stats",
					ConnectionID: input.ConnectionID,
					Driver:       conn.DriverName(),
					DurationMS:   time.Since(now).Milliseconds(),
					Success:      false,
					Error:        err.Error(),
				})
				return newToolError(err), nil, err
			}
			stats.RowCount = stats.LiveTuples
			if lastAnalyze.Valid {
				s := lastAnalyze.Time.Format(time.RFC3339)
				stats.LastAnalyzed = &s
			}
		}
		if err := rows.Err(); err != nil {
			h.Audit.Log(audit.AuditEntry{
				QueryID:      queryID,
				Timestamp:    time.Now(),
				Tool:         "get_table_stats",
				ConnectionID: input.ConnectionID,
				Driver:       conn.DriverName(),
				DurationMS:   time.Since(now).Milliseconds(),
				Success:      false,
				Error:        err.Error(),
			})
			return newToolError(err), nil, err
		}

	case "mysql":
		q := `SELECT TABLE_ROWS, DATA_LENGTH + INDEX_LENGTH, CREATE_TIME
			FROM information_schema.TABLES
			WHERE TABLE_NAME = ? AND TABLE_SCHEMA = DATABASE()`

		rows, err := conn.QueryContext(ctx, q, input.Table)
		if err != nil {
			h.Audit.Log(audit.AuditEntry{
				QueryID:      queryID,
				Timestamp:    time.Now(),
				Tool:         "get_table_stats",
				ConnectionID: input.ConnectionID,
				Driver:       conn.DriverName(),
				DurationMS:   time.Since(now).Milliseconds(),
				Success:      false,
				Error:        err.Error(),
			})
			return newToolError(err), nil, err
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
				h.Audit.Log(audit.AuditEntry{
					QueryID:      queryID,
					Timestamp:    time.Now(),
					Tool:         "get_table_stats",
					ConnectionID: input.ConnectionID,
					Driver:       conn.DriverName(),
					DurationMS:   time.Since(now).Milliseconds(),
					Success:      false,
					Error:        err.Error(),
				})
				return newToolError(err), nil, err
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
			h.Audit.Log(audit.AuditEntry{
				QueryID:      queryID,
				Timestamp:    time.Now(),
				Tool:         "get_table_stats",
				ConnectionID: input.ConnectionID,
				Driver:       conn.DriverName(),
				DurationMS:   time.Since(now).Milliseconds(),
				Success:      false,
				Error:        err.Error(),
			})
			return newToolError(err), nil, err
		}

	default:
		err := fmt.Errorf("unsupported driver: %s", conn.DriverName())
		return newToolError(err), nil, err
	}

	data, _ := json.Marshal(stats)
	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         "get_table_stats",
		ConnectionID: input.ConnectionID,
		Driver:       conn.DriverName(),
		DurationMS:   time.Since(now).Milliseconds(),
		Success:      true,
	})

	return newToolTextResult(string(data)), nil, nil
}
