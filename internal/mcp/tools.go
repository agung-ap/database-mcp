package mcp

import (
	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/agp/db-mcp/internal/tools"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Handler aggregates all tool handlers and their shared dependencies.
type Handler struct {
	Manager *db.Manager
	Audit   *audit.Logger
	TxStore *tools.TxStore
}

// RegisterAll registers all 14 MCP tools on the given Server.
func (h *Handler) RegisterAll(srv *Server) {
	// --- Meta tools ---
	connInfos := make([]tools.ConnectionInfo, 0)
	for _, c := range h.Manager.Connections() {
		connInfos = append(connInfos, tools.ConnectionInfo{ID: c.Name, Driver: c.Engine})
	}

	listConns := &tools.ListConnectionsHandler{Connections: connInfos, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"list_connections",
		mcpgo.WithDescription("List all configured database connection IDs and their drivers"),
	), server.ToolHandlerFunc(listConns.Handle))

	testConn := &tools.TestConnectionHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"test_connection",
		mcpgo.WithDescription("Ping a database connection and return latency"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
	), server.ToolHandlerFunc(testConn.Handle))

	// --- Schema tools ---
	listDBs := &tools.ListDatabasesHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"list_databases",
		mcpgo.WithDescription("List databases/schemas visible to the connected user"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
	), server.ToolHandlerFunc(listDBs.Handle))

	listTbls := &tools.ListTablesHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"list_tables",
		mcpgo.WithDescription("List tables in the connected database"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("database",
			mcpgo.Description("Database name (optional)"),
		),
	), server.ToolHandlerFunc(listTbls.Handle))

	descTbl := &tools.DescribeTableHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"describe_table",
		mcpgo.WithDescription("Return column definitions for a table"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("table",
			mcpgo.Required(),
			mcpgo.Description("Table name"),
		),
		mcpgo.WithString("schema",
			mcpgo.Description("Schema name (optional, default: public for postgres)"),
		),
	), server.ToolHandlerFunc(descTbl.Handle))

	listIdx := &tools.ListIndexesHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"list_indexes",
		mcpgo.WithDescription("List indexes on a table"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("table",
			mcpgo.Required(),
			mcpgo.Description("Table name"),
		),
		mcpgo.WithString("schema",
			mcpgo.Description("Schema name (optional)"),
		),
	), server.ToolHandlerFunc(listIdx.Handle))

	listFKs := &tools.ListForeignKeysHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"list_foreign_keys",
		mcpgo.WithDescription("List foreign key constraints on a table"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("table",
			mcpgo.Required(),
			mcpgo.Description("Table name"),
		),
		mcpgo.WithString("schema",
			mcpgo.Description("Schema name (optional)"),
		),
	), server.ToolHandlerFunc(listFKs.Handle))

	// --- Query tools ---
	execQuery := &tools.ExecuteQueryHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"execute_query",
		mcpgo.WithDescription("Execute a read-only SQL query and return results as JSON"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("query",
			mcpgo.Required(),
			mcpgo.Description("SQL query to execute"),
		),
	), server.ToolHandlerFunc(execQuery.Handle))

	execMut := &tools.ExecuteMutationHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"execute_mutation",
		mcpgo.WithDescription("Execute a write SQL statement (INSERT/UPDATE/DELETE) — requires confirm=true"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("query",
			mcpgo.Required(),
			mcpgo.Description("SQL mutation statement"),
		),
	), server.ToolHandlerFunc(execMut.Handle))

	explainQ := &tools.ExplainQueryHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"explain_query",
		mcpgo.WithDescription("Return the query execution plan in JSON format"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("query",
			mcpgo.Required(),
			mcpgo.Description("SQL query to explain"),
		),
	), server.ToolHandlerFunc(explainQ.Handle))

	// --- Stats tool ---
	tblStats := &tools.GetTableStatsHandler{Manager: h.Manager, Audit: h.Audit}
	srv.AddTool(mcpgo.NewTool(
		"get_table_stats",
		mcpgo.WithDescription("Return row count, size, and other statistics for a table"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
		mcpgo.WithString("table",
			mcpgo.Required(),
			mcpgo.Description("Table name"),
		),
		mcpgo.WithString("schema",
			mcpgo.Description("Schema name (optional)"),
		),
	), server.ToolHandlerFunc(tblStats.Handle))

	// --- Transaction tools ---
	beginTx := &tools.BeginTransactionHandler{Manager: h.Manager, Audit: h.Audit, TxStore: h.TxStore}
	srv.AddTool(mcpgo.NewTool(
		"begin_transaction",
		mcpgo.WithDescription("Begin a database transaction and return a tx_id"),
		mcpgo.WithString("connection_id",
			mcpgo.Required(),
			mcpgo.Description("Named connection alias from config"),
		),
		mcpgo.WithString("driver",
			mcpgo.Required(),
			mcpgo.Description("Database driver"),
		),
	), server.ToolHandlerFunc(beginTx.Handle))

	commitTx := &tools.CommitTransactionHandler{Audit: h.Audit, TxStore: h.TxStore}
	srv.AddTool(mcpgo.NewTool(
		"commit_transaction",
		mcpgo.WithDescription("Commit an open transaction by tx_id"),
		mcpgo.WithString("tx_id",
			mcpgo.Required(),
			mcpgo.Description("Transaction ID returned by begin_transaction"),
		),
	), server.ToolHandlerFunc(commitTx.Handle))

	rollbackTx := &tools.RollbackTransactionHandler{Audit: h.Audit, TxStore: h.TxStore}
	srv.AddTool(mcpgo.NewTool(
		"rollback_transaction",
		mcpgo.WithDescription("Roll back an open transaction by tx_id"),
		mcpgo.WithString("tx_id",
			mcpgo.Required(),
			mcpgo.Description("Transaction ID returned by begin_transaction"),
		),
	), server.ToolHandlerFunc(rollbackTx.Handle))
}
