package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DBManager is the interface the schema handlers need to look up connections.
type DBManager interface {
	Get(connectionID string) (db.Driver, error)
}

// ListDatabasesInput is the input for list_databases tool.
type ListDatabasesInput struct {
	ConnectionID string `json:"connection_id"`
	Driver       string `json:"driver"`
}

// ListDatabasesHandler handles list_databases.
type ListDatabasesHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ListDatabasesHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input ListDatabasesInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_databases", ConnectionID: input.ConnectionID, Driver: input.Driver})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		return newToolError(err), nil, err
	}

	var query string
	switch conn.DriverName() {
	case "postgres":
		query = "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname"
	case "mysql":
		query = "SHOW DATABASES"
	default:
		err := fmt.Errorf("unsupported driver: %s", conn.DriverName())
		return newToolError(err), nil, err
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return newToolError(err), nil, err
	}
	defer func() {
		if e := rows.Close(); e != nil {
			slog.Debug("failed to close rows", "error", e)
		}
	}()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return newToolError(err), nil, err
		}
		databases = append(databases, name)
	}
	if err := rows.Err(); err != nil {
		return newToolError(err), nil, err
	}

	data, _ := json.Marshal(databases)
	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "list_databases", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true})
	return newToolTextResult(string(data)), nil, nil
}

// ListTablesInput is the input for list_tables tool.
type ListTablesInput struct {
	ConnectionID string `json:"connection_id"`
	Driver       string `json:"driver"`
	Database     string `json:"database"`
}

// ListTablesHandler handles list_tables.
type ListTablesHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

type tableInfo struct {
	Schema    string `json:"schema,omitempty"`
	TableName string `json:"table_name"`
	TableType string `json:"table_type,omitempty"`
}

func (h *ListTablesHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input ListTablesInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_tables", ConnectionID: input.ConnectionID, Driver: input.Driver})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		return newToolError(err), nil, err
	}

	var tables []tableInfo
	switch conn.DriverName() {
	case "postgres":
		q := `SELECT table_schema, table_name, table_type FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema, table_name`
		rows, err := conn.QueryContext(ctx, q)
		if err != nil {
			return newToolError(err), nil, err
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var t tableInfo
			if err := rows.Scan(&t.Schema, &t.TableName, &t.TableType); err != nil {
				return newToolError(err), nil, err
			}
			tables = append(tables, t)
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	case "mysql":
		rows, err := conn.QueryContext(ctx, "SHOW TABLES")
		if err != nil {
			return newToolError(err), nil, err
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return newToolError(err), nil, err
			}
			tables = append(tables, tableInfo{TableName: name})
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	default:
		err := fmt.Errorf("unsupported driver: %s", conn.DriverName())
		return newToolError(err), nil, err
	}

	data, _ := json.Marshal(tables)
	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "list_tables", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true})
	return newToolTextResult(string(data)), nil, nil
}

// DescribeTableInput is the input for describe_table tool.
type DescribeTableInput struct {
	ConnectionID string `json:"connection_id"`
	Driver       string `json:"driver"`
	Table        string `json:"table"`
	Schema       string `json:"schema"`
}

// DescribeTableHandler handles describe_table.
type DescribeTableHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

type columnInfo struct {
	ColumnName string `json:"column_name"`
	DataType   string `json:"data_type"`
	IsNullable bool   `json:"is_nullable"`
	ColumnKey  string `json:"column_key,omitempty"`
	Extra      string `json:"extra,omitempty"`
}

func (h *DescribeTableHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input DescribeTableInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "describe_table", ConnectionID: input.ConnectionID, Driver: input.Driver})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		return newToolError(err), nil, err
	}

	var columns []columnInfo
	schema := input.Schema
	if schema == "" && conn.DriverName() == "postgres" {
		schema = "public"
	}

	switch conn.DriverName() {
	case "postgres":
		q := `SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`
		rows, err := conn.QueryContext(ctx, q, schema, input.Table)
		if err != nil {
			return newToolError(err), nil, err
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var col columnInfo
			var nullable string
			if err := rows.Scan(&col.ColumnName, &col.DataType, &nullable); err != nil {
				return newToolError(err), nil, err
			}
			col.IsNullable = nullable == "YES"
			columns = append(columns, col)
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	case "mysql":
		q := fmt.Sprintf("DESCRIBE %s", input.Table)
		rows, err := conn.QueryContext(ctx, q)
		if err != nil {
			return newToolError(err), nil, err
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var col columnInfo
			var nullable string
			var key sql.NullString
			var extra sql.NullString
			if err := rows.Scan(&col.ColumnName, &col.DataType, &nullable, &key, nil, &extra); err != nil {
				return newToolError(err), nil, err
			}
			col.IsNullable = nullable == "YES"
			if key.Valid {
				col.ColumnKey = key.String
			}
			if extra.Valid {
				col.Extra = extra.String
			}
			columns = append(columns, col)
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	default:
		err := fmt.Errorf("unsupported driver: %s", conn.DriverName())
		return newToolError(err), nil, err
	}

	data, _ := json.Marshal(columns)
	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "describe_table", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true})
	return newToolTextResult(string(data)), nil, nil
}

// ListIndexesInput is the input for list_indexes tool.
type ListIndexesInput struct {
	ConnectionID string `json:"connection_id"`
	Driver       string `json:"driver"`
	Table        string `json:"table"`
	Schema       string `json:"schema"`
}

// ListIndexesHandler handles list_indexes.
type ListIndexesHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

type indexInfo struct {
	IndexName  string `json:"index_name"`
	ColumnName string `json:"column_name"`
	IsUnique   bool   `json:"is_unique"`
	IsPrimary  bool   `json:"is_primary"`
}

func (h *ListIndexesHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input ListIndexesInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_indexes", ConnectionID: input.ConnectionID, Driver: input.Driver})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		return newToolError(err), nil, err
	}

	var indexes []indexInfo
	schema := input.Schema
	if schema == "" && conn.DriverName() == "postgres" {
		schema = "public"
	}

	switch conn.DriverName() {
	case "postgres":
		q := `
			SELECT i.relname, a.attname,
				ix.indisunique, ix.indisprimary
			FROM pg_index ix
			JOIN pg_class t  ON t.oid  = ix.indrelid
			JOIN pg_class i  ON i.oid  = ix.indexrelid
			JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
			JOIN pg_namespace n ON n.oid = t.relnamespace
			WHERE t.relname = $1 AND n.nspname = $2
			ORDER BY i.relname, a.attnum`
		rows, queryErr := conn.QueryContext(ctx, q, input.Table, schema)
		if queryErr != nil {
			return newToolError(queryErr), nil, queryErr
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var idx indexInfo
			if err := rows.Scan(&idx.IndexName, &idx.ColumnName, &idx.IsUnique, &idx.IsPrimary); err != nil {
				return newToolError(err), nil, err
			}
			indexes = append(indexes, idx)
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	case "mysql":
		q := fmt.Sprintf("SHOW INDEX FROM %s", input.Table)
		rows, err := conn.QueryContext(ctx, q)
		if err != nil {
			return newToolError(err), nil, err
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var tableName, nonUnique int
			var keyName string
			var seqInIndex int
			var columnName string
			var collation sql.NullString
			var cardinality sql.NullInt64
			var subPart sql.NullString
			var packed sql.NullString
			var null string
			var indexType string
			var comment string
			var indexComment string
			var visible string
			var expression sql.NullString

			if err := rows.Scan(&tableName, &nonUnique, &keyName, &seqInIndex, &columnName, &collation, &cardinality, &subPart, &packed, &null, &indexType, &comment, &indexComment, &visible, &expression); err != nil {
				return newToolError(err), nil, err
			}
			indexes = append(indexes, indexInfo{IndexName: keyName, ColumnName: columnName, IsUnique: nonUnique == 0})
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	default:
		err := fmt.Errorf("unsupported driver: %s", conn.DriverName())
		return newToolError(err), nil, err
	}

	data, _ := json.Marshal(indexes)
	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "list_indexes", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true})
	return newToolTextResult(string(data)), nil, nil
}

// ListForeignKeysInput is the input for list_foreign_keys tool.
type ListForeignKeysInput struct {
	ConnectionID string `json:"connection_id"`
	Driver       string `json:"driver"`
	Table        string `json:"table"`
	Schema       string `json:"schema"`
}

// ListForeignKeysHandler handles list_foreign_keys.
type ListForeignKeysHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

type fkInfo struct {
	ConstraintName   string `json:"constraint_name"`
	ColumnName       string `json:"column_name"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
}

func (h *ListForeignKeysHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input ListForeignKeysInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_foreign_keys", ConnectionID: input.ConnectionID, Driver: input.Driver})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		return newToolError(err), nil, err
	}

	var fks []fkInfo
	// For PostgreSQL, default schema is 'public' if not specified
	// (This is implicitly used when table names are unqualified)

	switch conn.DriverName() {
	case "postgres":
		q := `
			SELECT kcu.constraint_name, kcu.column_name, ccu.table_name, ccu.column_name
			FROM information_schema.key_column_usage kcu
			JOIN information_schema.referential_constraints rc
				ON kcu.constraint_name = rc.constraint_name
				AND kcu.constraint_schema = rc.constraint_schema
			JOIN information_schema.constraint_column_usage ccu
				ON rc.unique_constraint_name = ccu.constraint_name
				AND rc.unique_constraint_schema = ccu.constraint_schema
			WHERE kcu.table_name = $1 AND kcu.table_schema = $2`
		schema := input.Schema
		if schema == "" {
			schema = "public"
		}
		rows, err := conn.QueryContext(ctx, q, input.Table, schema)
		if err != nil {
			return newToolError(err), nil, err
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var fk fkInfo
			if err := rows.Scan(&fk.ConstraintName, &fk.ColumnName, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
				return newToolError(err), nil, err
			}
			fks = append(fks, fk)
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	case "mysql":
		q := `SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL`
		rows, err := conn.QueryContext(ctx, q, input.Table)
		if err != nil {
			return newToolError(err), nil, err
		}
		defer func() {
			if e := rows.Close(); e != nil {
				slog.Debug("failed to close rows", "error", e)
			}
		}()
		for rows.Next() {
			var fk fkInfo
			if err := rows.Scan(&fk.ConstraintName, &fk.ColumnName, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
				return newToolError(err), nil, err
			}
			fks = append(fks, fk)
		}
		if err := rows.Err(); err != nil {
			return newToolError(err), nil, err
		}
	default:
		err := fmt.Errorf("unsupported driver: %s", conn.DriverName())
		return newToolError(err), nil, err
	}

	data, _ := json.Marshal(fks)
	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: time.Now(), Tool: "list_foreign_keys", ConnectionID: input.ConnectionID, Driver: conn.DriverName(), DurationMS: time.Since(now).Milliseconds(), Success: true})
	return newToolTextResult(string(data)), nil, nil
}
