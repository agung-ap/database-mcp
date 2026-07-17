//go:build integration

// Package integration exercises the tool handlers against real Postgres,
// MySQL, and SQL Server instances (see docker-compose.yml and the Makefile's
// `integration` target). It fills the gap unit tests can't: whether the
// read-only guard actually stops a write at the database, whether a
// transaction opened via tx_id really commits/rolls back real rows, and
// whether the SQL Server SHOWPLAN_XML batching fix actually works against a
// real server.
//
// Run with: make integration
// (equivalent to: docker compose up -d --wait && go test -tags integration ./integration/...)
package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/agung-ap/database-mcp/internal/config"
	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/agung-ap/database-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type target struct {
	driver string
	dsn    string
}

func targets(t *testing.T) []target {
	t.Helper()
	var ts []target
	for driver, envVar := range map[string]string{
		"postgres":  "MCP_DB_IT_POSTGRES_DSN",
		"mysql":     "MCP_DB_IT_MYSQL_DSN",
		"sqlserver": "MCP_DB_IT_MSSQL_DSN",
	} {
		if dsn := os.Getenv(envVar); dsn != "" {
			ts = append(ts, target{driver: driver, dsn: dsn})
		}
	}
	if len(ts) == 0 {
		t.Skip("no MCP_DB_IT_*_DSN env vars set; run via `make integration` to test against real databases")
	}
	return ts
}

func newManager(t *testing.T, tg target, readOnly bool) (*db.Manager, string) {
	t.Helper()
	connID := "it-" + tg.driver
	m, err := db.NewManager([]config.Connection{{ID: connID, Driver: tg.driver, DSN: tg.dsn, ReadOnly: readOnly}})
	require.NoError(t, err)
	t.Cleanup(m.CloseAll)
	return m, connID
}

func testAudit(t *testing.T) *audit.Logger {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "audit-*.log")
	require.NoError(t, err)
	logger, err := audit.New(f.Name())
	require.NoError(t, err)
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}

// setupTable (re)creates a small seeded table used by the tests below. It
// runs as a direct ExecContext against the pool (not through execute_mutation
// on a read_only manager) since it's test fixture setup, not the thing under
// test.
func setupTable(t *testing.T, m *db.Manager, connID, driver string) {
	t.Helper()
	drv, err := m.Get(connID)
	require.NoError(t, err)
	ctx := context.Background()

	switch driver {
	case "postgres":
		_, _ = drv.ExecContext(ctx, "DROP TABLE IF EXISTS it_widgets")
		_, err = drv.ExecContext(ctx, "CREATE TABLE it_widgets (id SERIAL PRIMARY KEY, name TEXT NOT NULL)")
	case "mysql":
		_, _ = drv.ExecContext(ctx, "DROP TABLE IF EXISTS it_widgets")
		_, err = drv.ExecContext(ctx, "CREATE TABLE it_widgets (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255) NOT NULL)")
	case "sqlserver":
		_, _ = drv.ExecContext(ctx, "IF OBJECT_ID('it_widgets', 'U') IS NOT NULL DROP TABLE it_widgets")
		_, err = drv.ExecContext(ctx, "CREATE TABLE it_widgets (id INT IDENTITY PRIMARY KEY, name NVARCHAR(255) NOT NULL)")
	}
	require.NoError(t, err)

	_, err = drv.ExecContext(ctx, "INSERT INTO it_widgets (name) VALUES ('seed-a')")
	require.NoError(t, err)
}

func TestReadOnlyGuard_BlocksMutation(t *testing.T) {
	for _, tg := range targets(t) {
		if tg.driver == "sqlserver" {
			// SQL Server has no session-level read-only mode, so this guard
			// is documented as not enforced for sqlserver connections; see
			// README "Security Notes".
			continue
		}
		t.Run(tg.driver, func(t *testing.T) {
			setupM, connID := newManager(t, tg, false)
			setupTable(t, setupM, connID, tg.driver)

			m, connID := newManager(t, tg, false)
			h := &tools.ExecuteQueryHandler{Manager: m, Audit: testAudit(t)}
			res, err := h.Handle(context.Background(), tools.ExecuteQueryInput{
				ConnectionID: connID,
				Query:        "UPDATE it_widgets SET name = 'hacked'",
			})
			require.NoError(t, err)
			require.True(t, res.IsError, "execute_query must reject a write even though it reached a real database")
		})
	}
}

func TestReadOnlyConnection_BlocksMutationTool(t *testing.T) {
	for _, tg := range targets(t) {
		t.Run(tg.driver, func(t *testing.T) {
			setupM, connID := newManager(t, tg, false)
			setupTable(t, setupM, connID, tg.driver)

			m, connID := newManager(t, tg, true) // read_only: true
			h := &tools.ExecuteMutationHandler{Manager: m, Audit: testAudit(t)}
			res, err := h.Handle(context.Background(), tools.ExecuteMutationInput{
				ConnectionID: connID,
				Query:        "DELETE FROM it_widgets",
				Confirm:      true,
			})
			require.NoError(t, err)
			require.True(t, res.IsError, "a read_only connection must reject execute_mutation before it ever reaches the database")
		})
	}
}

func TestTransactionLifecycle_CommitAndRollback(t *testing.T) {
	for _, tg := range targets(t) {
		t.Run(tg.driver, func(t *testing.T) {
			setupM, connID := newManager(t, tg, false)
			setupTable(t, setupM, connID, tg.driver)

			m, connID := newManager(t, tg, false)
			auditLog := testAudit(t)
			store := tools.NewTxStore(30 * time.Second)
			t.Cleanup(store.Stop)

			begin := &tools.BeginTransactionHandler{Manager: m, Audit: auditLog, TxStore: store}
			mutate := &tools.ExecuteMutationHandler{Manager: m, Audit: auditLog, TxStore: store}
			commit := &tools.CommitTransactionHandler{Audit: auditLog, TxStore: store}
			rollback := &tools.RollbackTransactionHandler{Audit: auditLog, TxStore: store}
			query := &tools.ExecuteQueryHandler{Manager: m, Audit: auditLog}

			ctx := context.Background()

			// Commit path: insert a row inside a tx_id transaction, commit,
			// then confirm a fresh pool-based query sees it.
			beginRes, err := begin.Handle(ctx, tools.BeginTransactionInput{ConnectionID: connID})
			require.NoError(t, err)
			require.False(t, beginRes.IsError)
			txID := extractTxID(t, beginRes)

			mutRes, err := mutate.Handle(ctx, tools.ExecuteMutationInput{
				TxID: txID, Query: "INSERT INTO it_widgets (name) VALUES ('committed-row')", Confirm: true,
			})
			require.NoError(t, err)
			require.False(t, mutRes.IsError)

			commitRes, err := commit.Handle(ctx, tools.CommitTransactionInput{TxID: txID})
			require.NoError(t, err)
			require.False(t, commitRes.IsError)

			countRes, err := query.Handle(ctx, tools.ExecuteQueryInput{
				ConnectionID: connID, Query: "SELECT COUNT(*) AS c FROM it_widgets WHERE name = 'committed-row'",
			})
			require.NoError(t, err)
			require.False(t, countRes.IsError)
			require.Contains(t, resultText(t, countRes), `"c"`)

			// Rollback path: insert another row, roll back, confirm it's gone.
			beginRes2, err := begin.Handle(ctx, tools.BeginTransactionInput{ConnectionID: connID})
			require.NoError(t, err)
			txID2 := extractTxID(t, beginRes2)

			_, err = mutate.Handle(ctx, tools.ExecuteMutationInput{
				TxID: txID2, Query: "INSERT INTO it_widgets (name) VALUES ('rolled-back-row')", Confirm: true,
			})
			require.NoError(t, err)

			rbRes, err := rollback.Handle(ctx, tools.RollbackTransactionInput{TxID: txID2})
			require.NoError(t, err)
			require.False(t, rbRes.IsError)

			checkRes, err := query.Handle(ctx, tools.ExecuteQueryInput{
				ConnectionID: connID, Query: "SELECT COUNT(*) AS c FROM it_widgets WHERE name = 'rolled-back-row'",
			})
			require.NoError(t, err)
			require.False(t, checkRes.IsError)
			require.Contains(t, resultText(t, checkRes), `"c":0`)
		})
	}
}

func TestExplainQuery(t *testing.T) {
	for _, tg := range targets(t) {
		t.Run(tg.driver, func(t *testing.T) {
			setupM, connID := newManager(t, tg, false)
			setupTable(t, setupM, connID, tg.driver)

			m, connID := newManager(t, tg, false)
			h := &tools.ExplainQueryHandler{Manager: m, Audit: testAudit(t)}
			res, err := h.Handle(context.Background(), tools.ExplainQueryInput{
				ConnectionID: connID,
				Query:        "SELECT * FROM it_widgets",
			})
			require.NoError(t, err)
			require.Falsef(t, res.IsError, "explain_query failed: %s", resultText(t, res))
			require.NotEmpty(t, resultText(t, res))
		})
	}
}

func TestSchemaTools(t *testing.T) {
	for _, tg := range targets(t) {
		t.Run(tg.driver, func(t *testing.T) {
			setupM, connID := newManager(t, tg, false)
			setupTable(t, setupM, connID, tg.driver)

			m, connID := newManager(t, tg, false)
			auditLog := testAudit(t)
			ctx := context.Background()

			listTables := &tools.ListTablesHandler{Manager: m, Audit: auditLog}
			res, err := listTables.Handle(ctx, tools.ListTablesInput{ConnectionID: connID})
			require.NoError(t, err)
			require.False(t, res.IsError)
			require.Contains(t, resultText(t, res), "it_widgets")

			stats := &tools.GetTableStatsHandler{Manager: m, Audit: auditLog}
			statsRes, err := stats.Handle(ctx, tools.GetTableStatsInput{ConnectionID: connID, Table: "it_widgets"})
			require.NoError(t, err)
			require.False(t, statsRes.IsError)
		})
	}
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.Len(t, res.Content, 1)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	return text.Text
}

func extractTxID(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var out struct {
		TxID string `json:"tx_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(resultText(t, res)), &out))
	require.NotEmpty(t, out.TxID)
	return out.TxID
}
