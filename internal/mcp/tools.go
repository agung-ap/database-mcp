package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/agung-ap/database-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler aggregates shared dependencies for all tool registrations.
type Handler struct {
	Manager      *db.Manager
	Audit        *audit.Logger
	TxStore      *tools.TxStore
	QueryTimeout time.Duration // 0 means no per-call timeout
}

// wrap adapts a Handler.Handle method into the mcp.AddTool callback shape,
// bounding every call with the configured query timeout and recovering from
// panics so a single bad handler can't take down the whole server (and every
// other open transaction along with it).
func wrap[TIn any](h *Handler, name string, fn func(ctx context.Context, in TIn) (*mcp.CallToolResult, error)) func(context.Context, *mcp.CallToolRequest, TIn) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in TIn) (res *mcp.CallToolResult, out any, err error) {
		if h.QueryTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, h.QueryTimeout)
			defer cancel()
		}

		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered in tool handler", "tool", name, "panic", fmt.Sprintf("%v", r))
				res, out, err = panicResult(name, r), nil, nil
			}
		}()

		res, err = fn(ctx, in)
		return res, nil, err
	}
}

func panicResult(name string, r any) *mcp.CallToolResult {
	data, jsonErr := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}{Code: "500", Message: "internal error", Detail: fmt.Sprintf("panic in %s: %v", name, r)})
	if jsonErr != nil {
		data = []byte(`{"code":"500","message":"internal error","detail":"panic in tool handler"}`)
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}
}

// RegisterAll registers all 14 MCP tools on the given Server.
func (h *Handler) RegisterAll(srv *Server) {
	connInfos := make([]tools.ConnectionInfo, 0, len(h.Manager.Connections()))
	for _, c := range h.Manager.Connections() {
		connInfos = append(connInfos, tools.ConnectionInfo{ID: c.ID, Driver: c.Driver, ReadOnly: c.ReadOnly})
	}

	listConns := &tools.ListConnectionsHandler{Connections: connInfos, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_connections",
		Description: "List all configured database connection IDs and their drivers",
	}, wrap(h, "list_connections", listConns.Handle))

	testConn := &tools.TestConnectionHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "test_connection",
		Description: "Ping a database connection and return latency in milliseconds",
	}, wrap(h, "test_connection", testConn.Handle))

	listDBs := &tools.ListDatabasesHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_databases",
		Description: "List databases/schemas visible to the connected user",
	}, wrap(h, "list_databases", listDBs.Handle))

	listTbls := &tools.ListTablesHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_tables",
		Description: "List tables in the connected database",
	}, wrap(h, "list_tables", listTbls.Handle))

	descTbl := &tools.DescribeTableHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "describe_table",
		Description: "Return column definitions for a table",
	}, wrap(h, "describe_table", descTbl.Handle))

	listIdx := &tools.ListIndexesHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_indexes",
		Description: "List indexes on a table",
	}, wrap(h, "list_indexes", listIdx.Handle))

	listFKs := &tools.ListForeignKeysHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "list_foreign_keys",
		Description: "List foreign key constraints on a table",
	}, wrap(h, "list_foreign_keys", listFKs.Handle))

	execQuery := &tools.ExecuteQueryHandler{Manager: h.Manager, Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "execute_query",
		Description: "Execute a read-only SQL query and return results as JSON. Pass tx_id to run inside an open transaction instead of connection_id.",
	}, wrap(h, "execute_query", execQuery.Handle))

	execMut := &tools.ExecuteMutationHandler{Manager: h.Manager, Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "execute_mutation",
		Description: "Execute a write SQL statement (INSERT/UPDATE/DELETE) — requires confirm=true. Pass tx_id to run inside an open transaction instead of connection_id.",
	}, wrap(h, "execute_mutation", execMut.Handle))

	explainQ := &tools.ExplainQueryHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "explain_query",
		Description: "Return the query execution plan in JSON format",
	}, wrap(h, "explain_query", explainQ.Handle))

	tblStats := &tools.GetTableStatsHandler{Manager: h.Manager, Audit: h.Audit}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "get_table_stats",
		Description: "Return row count, size, and statistics for a table",
	}, wrap(h, "get_table_stats", tblStats.Handle))

	beginTx := &tools.BeginTransactionHandler{Manager: h.Manager, Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "begin_transaction",
		Description: "Begin a database transaction and return a tx_id",
	}, wrap(h, "begin_transaction", beginTx.Handle))

	commitTx := &tools.CommitTransactionHandler{Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "commit_transaction",
		Description: "Commit an open transaction by tx_id",
	}, wrap(h, "commit_transaction", commitTx.Handle))

	rollbackTx := &tools.RollbackTransactionHandler{Audit: h.Audit, TxStore: h.TxStore}
	mcp.AddTool(srv.s, &mcp.Tool{
		Name:        "rollback_transaction",
		Description: "Roll back an open transaction by tx_id",
	}, wrap(h, "rollback_transaction", rollbackTx.Handle))
}
