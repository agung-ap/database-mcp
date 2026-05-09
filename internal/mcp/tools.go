package mcp

import (
	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/agp/db-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler aggregates all tool handlers and their shared dependencies.
type Handler struct {
	Manager *db.Manager
	Audit   *audit.Logger
	TxStore *tools.TxStore
}

// RegisterAll registers all MCP tools on the given Server.
func (h *Handler) RegisterAll(srv *Server) {
	// --- Meta tools ---
	connInfos := make([]tools.ConnectionInfo, 0)
	for _, c := range h.Manager.Connections() {
		connInfos = append(connInfos, tools.ConnectionInfo{ID: c.Name, Driver: c.Engine})
	}

	listConnsTool := &mcp.Tool{
		Name:        "list_connections",
		Description: "List all configured database connection IDs and their drivers",
	}
	listConns := &tools.ListConnectionsHandler{Connections: connInfos, Audit: h.Audit}
	srv.AddTool(listConnsTool, listConns.Handle)

	testConnTool := &mcp.Tool{
		Name:        "test_connection",
		Description: "Ping a database connection and return latency",
	}
	testConn := &tools.TestConnectionHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, testConnTool, testConn.Handle)

	// --- Schema tools ---
	listDBsTool := &mcp.Tool{
		Name:        "list_databases",
		Description: "List databases/schemas visible to the connected user",
	}
	listDBs := &tools.ListDatabasesHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, listDBsTool, listDBs.Handle)

	listTblsTool := &mcp.Tool{
		Name:        "list_tables",
		Description: "List tables in the connected database",
	}
	listTbls := &tools.ListTablesHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, listTblsTool, listTbls.Handle)

	descTblTool := &mcp.Tool{
		Name:        "describe_table",
		Description: "Return column definitions for a table",
	}
	descTbl := &tools.DescribeTableHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, descTblTool, descTbl.Handle)

	listIdxTool := &mcp.Tool{
		Name:        "list_indexes",
		Description: "Return indexes defined on a table",
	}
	listIdx := &tools.ListIndexesHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, listIdxTool, listIdx.Handle)

	listFKTool := &mcp.Tool{
		Name:        "list_foreign_keys",
		Description: "Return foreign keys for a table",
	}
	listFK := &tools.ListForeignKeysHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, listFKTool, listFK.Handle)

	// --- Query tools ---
	execQueryTool := &mcp.Tool{
		Name:        "execute_query",
		Description: "Execute a read-only SQL query",
	}
	execQuery := &tools.ExecuteQueryHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, execQueryTool, execQuery.Handle)

	execMutTool := &mcp.Tool{
		Name:        "execute_mutation",
		Description: "Execute a SQL mutation (INSERT/UPDATE/DELETE). Must set confirm: true.",
	}
	execMut := &tools.ExecuteMutationHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, execMutTool, execMut.Handle)

	explainTool := &mcp.Tool{
		Name:        "explain_query",
		Description: "Return an EXPLAIN plan for a SQL query (read-only)",
	}
	explain := &tools.ExplainQueryHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, explainTool, explain.Handle)

	// --- Stats tools ---
	statsTool := &mcp.Tool{
		Name:        "get_table_stats",
		Description: "Return table statistics (row count, size, etc.)",
	}
	stats := &tools.GetTableStatsHandler{Manager: h.Manager, Audit: h.Audit}
	AddToolWithInput(srv, statsTool, stats.Handle)

	// --- Transaction tools ---
	beginTxTool := &mcp.Tool{
		Name:        "begin_transaction",
		Description: "Start a new database transaction",
	}
	beginTx := &tools.BeginTransactionHandler{Manager: h.Manager, TxStore: h.TxStore, Audit: h.Audit}
	AddToolWithInput(srv, beginTxTool, beginTx.Handle)

	commitTxTool := &mcp.Tool{
		Name:        "commit_transaction",
		Description: "Commit an open transaction",
	}
	commitTx := &tools.CommitTransactionHandler{TxStore: h.TxStore, Audit: h.Audit}
	AddToolWithInput(srv, commitTxTool, commitTx.Handle)

	rollbackTxTool := &mcp.Tool{
		Name:        "rollback_transaction",
		Description: "Rollback an open transaction",
	}
	rollbackTx := &tools.RollbackTransactionHandler{TxStore: h.TxStore, Audit: h.Audit}
	AddToolWithInput(srv, rollbackTxTool, rollbackTx.Handle)
}
