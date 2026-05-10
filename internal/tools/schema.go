package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListDatabasesInput is the input for the list_databases tool.
type ListDatabasesInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
}

// ListDatabasesHandler handles list_databases.
type ListDatabasesHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ListDatabasesHandler) Handle(ctx context.Context, in ListDatabasesInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_databases", ConnectionID: in.ConnectionID})

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "list_databases", in.ConnectionID, "", "503", "connection not found", err)
	}

	var query string
	switch conn.DriverName() {
	case "postgres":
		query = "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname"
	case "mysql":
		query = "SHOW DATABASES"
	default:
		return errResult("400", "unsupported driver", conn.DriverName()), nil
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "list_databases", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
	}
	defer func() { _ = rows.Close() }()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return auditErr(h.Audit, queryID, now, "list_databases", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
		}
		databases = append(databases, name)
	}
	if err := rows.Err(); err != nil {
		return auditErr(h.Audit, queryID, now, "list_databases", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
	}

	data, _ := json.Marshal(databases)
	auditSuccess(h.Audit, queryID, now, "list_databases", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(string(data)), nil
}

// ListTablesInput is the input for the list_tables tool.
type ListTablesInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
}

type tableInfo struct {
	Schema    string `json:"schema,omitempty"`
	TableName string `json:"table_name"`
	TableType string `json:"table_type,omitempty"`
}

// ListTablesHandler handles list_tables.
type ListTablesHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ListTablesHandler) Handle(ctx context.Context, in ListTablesInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_tables", ConnectionID: in.ConnectionID})

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "list_tables", in.ConnectionID, "", "503", "connection not found", err)
	}

	var tables []tableInfo
	switch conn.DriverName() {
	case "postgres":
		q := `SELECT table_schema, table_name, table_type FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema, table_name`
		rows, err := conn.QueryContext(ctx, q)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "list_tables", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var t tableInfo
			if err := rows.Scan(&t.Schema, &t.TableName, &t.TableType); err != nil {
				return auditErr(h.Audit, queryID, now, "list_tables", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			tables = append(tables, t)
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "list_tables", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	case "mysql":
		rows, err := conn.QueryContext(ctx, "SHOW TABLES")
		if err != nil {
			return auditErr(h.Audit, queryID, now, "list_tables", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return auditErr(h.Audit, queryID, now, "list_tables", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			tables = append(tables, tableInfo{TableName: name})
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "list_tables", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	default:
		return errResult("400", "unsupported driver", conn.DriverName()), nil
	}

	data, _ := json.Marshal(tables)
	auditSuccess(h.Audit, queryID, now, "list_tables", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(string(data)), nil
}

// DescribeTableInput is the input for the describe_table tool.
type DescribeTableInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
	Table        string `json:"table" jsonschema:"Table name"`
	Schema       string `json:"schema,omitempty" jsonschema:"Schema name (default: public for postgres)"`
}

type columnInfo struct {
	Name       string `json:"column_name"`
	OrdinalPos int    `json:"ordinal_position"`
	Default    string `json:"column_default,omitempty"`
	Nullable   string `json:"is_nullable"`
	DataType   string `json:"data_type"`
	MaxLength  *int   `json:"character_maximum_length,omitempty"`
}

// DescribeTableHandler handles describe_table.
type DescribeTableHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *DescribeTableHandler) Handle(ctx context.Context, in DescribeTableInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}
	if in.Table == "" {
		return errResult("400", "missing table", "table is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "describe_table", ConnectionID: in.ConnectionID})

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "describe_table", in.ConnectionID, "", "503", "connection not found", err)
	}

	var columns []columnInfo
	switch conn.DriverName() {
	case "postgres":
		schema := in.Schema
		if schema == "" {
			schema = "public"
		}
		q := `SELECT column_name, ordinal_position, COALESCE(column_default,''), is_nullable, data_type, character_maximum_length
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2
			ORDER BY ordinal_position`
		rows, err := conn.QueryContext(ctx, q, schema, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "describe_table", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var c columnInfo
			var maxLen sql.NullInt64
			if err := rows.Scan(&c.Name, &c.OrdinalPos, &c.Default, &c.Nullable, &c.DataType, &maxLen); err != nil {
				return auditErr(h.Audit, queryID, now, "describe_table", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			if maxLen.Valid {
				v := int(maxLen.Int64)
				c.MaxLength = &v
			}
			columns = append(columns, c)
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "describe_table", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	case "mysql":
		q := `SELECT COLUMN_NAME, ORDINAL_POSITION, COALESCE(COLUMN_DEFAULT,''), IS_NULLABLE, DATA_TYPE, CHARACTER_MAXIMUM_LENGTH
			FROM information_schema.COLUMNS
			WHERE TABLE_NAME = ? AND TABLE_SCHEMA = DATABASE()
			ORDER BY ORDINAL_POSITION`
		rows, err := conn.QueryContext(ctx, q, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "describe_table", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var c columnInfo
			var maxLen sql.NullInt64
			if err := rows.Scan(&c.Name, &c.OrdinalPos, &c.Default, &c.Nullable, &c.DataType, &maxLen); err != nil {
				return auditErr(h.Audit, queryID, now, "describe_table", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			if maxLen.Valid {
				v := int(maxLen.Int64)
				c.MaxLength = &v
			}
			columns = append(columns, c)
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "describe_table", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	default:
		return errResult("400", "unsupported driver", conn.DriverName()), nil
	}

	data, _ := json.Marshal(columns)
	auditSuccess(h.Audit, queryID, now, "describe_table", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(string(data)), nil
}

// ListIndexesInput is the input for the list_indexes tool.
type ListIndexesInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
	Table        string `json:"table" jsonschema:"Table name"`
	Schema       string `json:"schema,omitempty" jsonschema:"Schema name (optional)"`
}

type indexInfo struct {
	SchemaName string `json:"schema_name,omitempty"`
	TableName  string `json:"table_name"`
	IndexName  string `json:"index_name"`
	ColumnName string `json:"column_name,omitempty"`
	IsUnique   bool   `json:"is_unique"`
	IndexDef   string `json:"index_def,omitempty"`
}

// ListIndexesHandler handles list_indexes.
type ListIndexesHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ListIndexesHandler) Handle(ctx context.Context, in ListIndexesInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}
	if in.Table == "" {
		return errResult("400", "missing table", "table is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_indexes", ConnectionID: in.ConnectionID})

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "list_indexes", in.ConnectionID, "", "503", "connection not found", err)
	}

	var indexes []indexInfo
	switch conn.DriverName() {
	case "postgres":
		schema := in.Schema
		if schema == "" {
			schema = "public"
		}
		q := `SELECT schemaname, tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = $1 AND tablename = $2 ORDER BY indexname`
		rows, err := conn.QueryContext(ctx, q, schema, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "list_indexes", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var idx indexInfo
			if err := rows.Scan(&idx.SchemaName, &idx.TableName, &idx.IndexName, &idx.IndexDef); err != nil {
				return auditErr(h.Audit, queryID, now, "list_indexes", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			indexes = append(indexes, idx)
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "list_indexes", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	case "mysql":
		q := `SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS WHERE TABLE_NAME = ? AND TABLE_SCHEMA = DATABASE() ORDER BY INDEX_NAME, SEQ_IN_INDEX`
		rows, err := conn.QueryContext(ctx, q, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "list_indexes", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var idx indexInfo
			var nonUnique int
			if err := rows.Scan(&idx.TableName, &idx.IndexName, &idx.ColumnName, &nonUnique); err != nil {
				return auditErr(h.Audit, queryID, now, "list_indexes", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			idx.IsUnique = nonUnique == 0
			indexes = append(indexes, idx)
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "list_indexes", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	default:
		return errResult("400", "unsupported driver", conn.DriverName()), nil
	}

	data, _ := json.Marshal(indexes)
	auditSuccess(h.Audit, queryID, now, "list_indexes", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(string(data)), nil
}

// ListForeignKeysInput is the input for the list_foreign_keys tool.
type ListForeignKeysInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
	Table        string `json:"table" jsonschema:"Table name"`
	Schema       string `json:"schema,omitempty" jsonschema:"Schema name (optional)"`
}

type foreignKeyInfo struct {
	ConstraintName     string `json:"constraint_name"`
	TableSchema        string `json:"table_schema,omitempty"`
	TableName          string `json:"table_name"`
	ColumnName         string `json:"column_name"`
	ForeignTableSchema string `json:"foreign_table_schema,omitempty"`
	ForeignTableName   string `json:"foreign_table_name"`
	ForeignColumnName  string `json:"foreign_column_name"`
	UpdateRule         string `json:"update_rule,omitempty"`
	DeleteRule         string `json:"delete_rule,omitempty"`
}

// ListForeignKeysHandler handles list_foreign_keys.
type ListForeignKeysHandler struct {
	Manager DBManager
	Audit   *audit.Logger
}

func (h *ListForeignKeysHandler) Handle(ctx context.Context, in ListForeignKeysInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}
	if in.Table == "" {
		return errResult("400", "missing table", "table is required"), nil
	}

	h.Audit.Log(audit.AuditEntry{QueryID: queryID, Timestamp: now, Tool: "list_foreign_keys", ConnectionID: in.ConnectionID})

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, "", "503", "connection not found", err)
	}

	var fks []foreignKeyInfo
	switch conn.DriverName() {
	case "postgres":
		schema := in.Schema
		if schema == "" {
			schema = "public"
		}
		q := `SELECT
				kcu.constraint_name, kcu.table_schema, kcu.table_name, kcu.column_name,
				ccu.table_schema, ccu.table_name, ccu.column_name, rc.update_rule, rc.delete_rule
			FROM information_schema.key_column_usage kcu
			JOIN information_schema.referential_constraints rc
				ON kcu.constraint_name = rc.constraint_name AND kcu.constraint_schema = rc.constraint_schema
			JOIN information_schema.constraint_column_usage ccu
				ON rc.unique_constraint_name = ccu.constraint_name AND rc.unique_constraint_schema = ccu.constraint_schema
			WHERE kcu.table_schema = $1 AND kcu.table_name = $2
			ORDER BY kcu.constraint_name`
		rows, err := conn.QueryContext(ctx, q, schema, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var fk foreignKeyInfo
			if err := rows.Scan(&fk.ConstraintName, &fk.TableSchema, &fk.TableName, &fk.ColumnName,
				&fk.ForeignTableSchema, &fk.ForeignTableName, &fk.ForeignColumnName,
				&fk.UpdateRule, &fk.DeleteRule); err != nil {
				return auditErr(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			fks = append(fks, fk)
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	case "mysql":
		q := `SELECT
				kcu.CONSTRAINT_NAME, kcu.TABLE_SCHEMA, kcu.TABLE_NAME, kcu.COLUMN_NAME,
				kcu.REFERENCED_TABLE_SCHEMA, kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME,
				rc.UPDATE_RULE, rc.DELETE_RULE
			FROM information_schema.KEY_COLUMN_USAGE kcu
			JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
				ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
			WHERE kcu.TABLE_NAME = ? AND kcu.TABLE_SCHEMA = DATABASE()
				AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
			ORDER BY kcu.CONSTRAINT_NAME`
		rows, err := conn.QueryContext(ctx, q, in.Table)
		if err != nil {
			return auditErr(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, conn.DriverName(), "503", "query failed", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var fk foreignKeyInfo
			if err := rows.Scan(&fk.ConstraintName, &fk.TableSchema, &fk.TableName, &fk.ColumnName,
				&fk.ForeignTableSchema, &fk.ForeignTableName, &fk.ForeignColumnName,
				&fk.UpdateRule, &fk.DeleteRule); err != nil {
				return auditErr(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, conn.DriverName(), "500", "scan failed", err)
			}
			fks = append(fks, fk)
		}
		if err := rows.Err(); err != nil {
			return auditErr(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, conn.DriverName(), "500", "rows error", err)
		}
	default:
		return errResult("400", "unsupported driver", conn.DriverName()), nil
	}

	data, _ := json.Marshal(fks)
	auditSuccess(h.Audit, queryID, now, "list_foreign_keys", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(string(data)), nil
}
