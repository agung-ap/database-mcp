package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ConnectionInfo is returned by list_connections.
type ConnectionInfo struct {
	ID     string `json:"id"`
	Driver string `json:"driver"`
}

// ListConnectionsHandler handles the list_connections tool (no inputs).
type ListConnectionsHandler struct {
	Connections []ConnectionInfo
	Audit       *audit.Logger
}

// Handle implements the list_connections tool.
func (h *ListConnectionsHandler) Handle(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{
		QueryID:   queryID,
		Timestamp: now,
		Tool:      "list_connections",
		Success:   true,
	})

	data, err := json.Marshal(h.Connections)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:    queryID,
			Timestamp:  time.Now(),
			Tool:       "list_connections",
			DurationMS: time.Since(now).Milliseconds(),
			Success:    false,
			Error:      err.Error(),
		})
		return newToolError(err), nil, err
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:    queryID,
		Timestamp:  time.Now(),
		Tool:       "list_connections",
		DurationMS: time.Since(now).Milliseconds(),
		Success:    true,
	})

	return newToolTextResult(string(data)), nil, nil
}

// TestConnectionInput is the input for test_connection tool.
type TestConnectionInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"required,description=Named connection alias from config"`
	Driver       string `json:"driver" jsonschema:"required,description=Database driver"`
}

// TestConnectionHandler handles the test_connection tool.
type TestConnectionHandler struct {
	Manager interface {
		Get(connectionID string) (db.Driver, error)
	}
	Audit *audit.Logger
}

// Handle implements the test_connection tool.
func (h *TestConnectionHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, input TestConnectionInput) (*mcp.CallToolResult, any, error) {
	queryID := uuid.NewString()
	now := time.Now()

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "test_connection",
		ConnectionID: input.ConnectionID,
		Driver:       input.Driver,
	})

	conn, err := h.Manager.Get(input.ConnectionID)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "test_connection",
			ConnectionID: input.ConnectionID,
			Driver:       input.Driver,
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return newToolError(err), nil, err
	}

	pingStart := time.Now()
	if err := conn.PingContext(ctx); err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "test_connection",
			ConnectionID: input.ConnectionID,
			Driver:       conn.DriverName(),
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return newToolError(err), nil, err
	}
	latencyMS := time.Since(pingStart).Milliseconds()

	result := fmt.Sprintf(`{"connection_id":%q,"driver":%q,"latency_ms":%d,"ok":true}`,
		input.ConnectionID, conn.DriverName(), latencyMS)

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         "test_connection",
		ConnectionID: input.ConnectionID,
		Driver:       conn.DriverName(),
		DurationMS:   time.Since(now).Milliseconds(),
		Success:      true,
	})

	return newToolTextResult(result), nil, nil
}
