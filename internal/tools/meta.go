package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ConnectionInfo is returned by list_connections.
type ConnectionInfo struct {
	ID       string `json:"id"`
	Driver   string `json:"driver"`
	ReadOnly bool   `json:"read_only"`
}

// ListConnectionsInput is the input for the list_connections tool.
type ListConnectionsInput struct{}

// ListConnectionsHandler handles the list_connections tool.
type ListConnectionsHandler struct {
	Connections []ConnectionInfo
	Audit       *audit.Logger
}

func (h *ListConnectionsHandler) Handle(ctx context.Context, _ ListConnectionsInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	data, err := json.Marshal(h.Connections)
	if err != nil {
		return errResult("500", "failed to marshal connections", err.Error()), nil
	}

	auditSuccess(h.Audit, queryID, now, "list_connections", "", "", "", 0, 0)
	return textResult(string(data)), nil
}

// TestConnectionInput is the input for the test_connection tool.
type TestConnectionInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
}

// TestConnectionHandler handles the test_connection tool.
type TestConnectionHandler struct {
	Manager interface {
		Get(connectionID string) (db.Driver, error)
	}
	Audit *audit.Logger
}

func (h *TestConnectionHandler) Handle(ctx context.Context, in TestConnectionInput) (*mcp.CallToolResult, error) {
	queryID := newUUID()
	now := time.Now()

	if in.ConnectionID == "" {
		return errResult("400", "missing connection_id", "connection_id is required"), nil
	}

	conn, err := h.Manager.Get(in.ConnectionID)
	if err != nil {
		return auditErr(h.Audit, queryID, now, "test_connection", in.ConnectionID, "", "503", "connection not found", err)
	}

	pingStart := time.Now()
	if err := conn.PingContext(ctx); err != nil {
		return auditErr(h.Audit, queryID, now, "test_connection", in.ConnectionID, conn.DriverName(), "503", "ping failed", err)
	}
	latencyMS := time.Since(pingStart).Milliseconds()

	auditSuccess(h.Audit, queryID, now, "test_connection", in.ConnectionID, conn.DriverName(), "", 0, 0)
	return textResult(fmt.Sprintf(`{"connection_id":%q,"driver":%q,"latency_ms":%d,"ok":true}`,
		in.ConnectionID, conn.DriverName(), latencyMS)), nil
}
