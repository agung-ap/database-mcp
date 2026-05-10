package mcp

import (
	"context"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/agp/db-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler aggregates shared dependencies for all tool registrations.
type Handler struct {
	Manager *db.Manager
	Audit   *audit.Logger
	TxStore *tools.TxStore
}

// RegisterAll registers all 14 MCP tools on the given Server.
func (h *Handler) RegisterAll(srv *Server) {
	connInfos := make([]tools.ConnectionInfo, 0, len(h.Manager.Connections()))
	for _, c := range h.Manager.Connections() {
		connInfos = append(connInfos, tools.ConnectionInfo{ID: c.ID, Driver: c.Driver})
	}

	listConns := &tools.ListConnectionsHandler{Connections: connInfos, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_connections",
		Description: "List all configured database connection IDs and their drivers",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ListConnectionsInput) (*mcp.CallToolResult, any, error) {
		res, err := listConns.Handle(ctx, in)
		return res, nil, err
	})

	testConn := &tools.TestConnectionHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "test_connection",
		Description: "Ping a database connection and return latency in milliseconds",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.TestConnectionInput) (*mcp.CallToolResult, any, error) {
		res, err := testConn.Handle(ctx, in)
		return res, nil, err
	})

	listDBs := &tools.ListDatabasesHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_databases",
		Description: "List databases/schemas visible to the connected user",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ListDatabasesInput) (*mcp.CallToolResult, any, error) {
		res, err := listDBs.Handle(ctx, in)
		return res, nil, err
	})

	listTbls := &tools.ListTablesHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_tables",
		Description: "List tables in the connected database",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ListTablesInput) (*mcp.CallToolResult, any, error) {
		res, err := listTbls.Handle(ctx, in)
		return res, nil, err
	})

	descTbl := &tools.DescribeTableHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "describe_table",
		Description: "Return column definitions for a table",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.DescribeTableInput) (*mcp.CallToolResult, any, error) {
		res, err := descTbl.Handle(ctx, in)
		return res, nil, err
	})

	listIdx := &tools.ListIndexesHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_indexes",
		Description: "List indexes on a table",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ListIndexesInput) (*mcp.CallToolResult, any, error) {
		res, err := listIdx.Handle(ctx, in)
		return res, nil, err
	})

	listFKs := &tools.ListForeignKeysHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_foreign_keys",
		Description: "List foreign key constraints on a table",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ListForeignKeysInput) (*mcp.CallToolResult, any, error) {
		res, err := listFKs.Handle(ctx, in)
		return res, nil, err
	})

	execQuery := &tools.ExecuteQueryHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "execute_query",
		Description: "Execute a read-only SQL query and return results as JSON",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ExecuteQueryInput) (*mcp.CallToolResult, any, error) {
		res, err := execQuery.Handle(ctx, in)
		return res, nil, err
	})

	execMut := &tools.ExecuteMutationHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "execute_mutation",
		Description: "Execute a write SQL statement (INSERT/UPDATE/DELETE) — requires confirm=true",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ExecuteMutationInput) (*mcp.CallToolResult, any, error) {
		res, err := execMut.Handle(ctx, in)
		return res, nil, err
	})

	explainQ := &tools.ExplainQueryHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "explain_query",
		Description: "Return the query execution plan in JSON format",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.ExplainQueryInput) (*mcp.CallToolResult, any, error) {
		res, err := explainQ.Handle(ctx, in)
		return res, nil, err
	})

	tblStats := &tools.GetTableStatsHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "get_table_stats",
		Description: "Return row count, size, and statistics for a table",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.GetTableStatsInput) (*mcp.CallToolResult, any, error) {
		res, err := tblStats.Handle(ctx, in)
		return res, nil, err
	})

	beginTx := &tools.BeginTransactionHandler{Manager: h.Manager, Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "begin_transaction",
		Description: "Begin a database transaction and return a tx_id",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.BeginTransactionInput) (*mcp.CallToolResult, any, error) {
		res, err := beginTx.Handle(ctx, in)
		return res, nil, err
	})

	commitTx := &tools.CommitTransactionHandler{Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "commit_transaction",
		Description: "Commit an open transaction by tx_id",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.CommitTransactionInput) (*mcp.CallToolResult, any, error) {
		res, err := commitTx.Handle(ctx, in)
		return res, nil, err
	})

	rollbackTx := &tools.RollbackTransactionHandler{Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "rollback_transaction",
		Description: "Roll back an open transaction by tx_id",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tools.RollbackTransactionInput) (*mcp.CallToolResult, any, error) {
		res, err := rollbackTx.Handle(ctx, in)
		return res, nil, err
	})
}
