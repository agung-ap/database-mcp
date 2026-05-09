package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/db"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

// ConnectionInfo is returned by list_connections.
type ConnectionInfo struct {
	ID     string `json:"id"`
	Driver string `json:"driver"`
}

// ListConnectionsHandler handles the list_connections tool.
type ListConnectionsHandler struct {
	Connections []ConnectionInfo
	Audit       *audit.Logger
}

// Handle implements the list_connections tool.
func (h *ListConnectionsHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return toolError("500", "failed to marshal connections", err.Error()), nil
	}

	h.Audit.Log(audit.AuditEntry{
		QueryID:    queryID,
		Timestamp:  time.Now(),
		Tool:       "list_connections",
		DurationMS: time.Since(now).Milliseconds(),
		Success:    true,
	})

	return mcp.NewToolResultText(string(data)), nil
}

// TestConnectionHandler handles the test_connection tool.
type TestConnectionHandler struct {
	Manager interface {
		Get(connectionID string) (db.Driver, error)
	}
	Audit *audit.Logger
}

// Handle implements the test_connection tool.
func (h *TestConnectionHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID := uuid.NewString()
	now := time.Now()

	connID, ok := req.Params.Arguments["connection_id"].(string)
	if !ok || connID == "" {
		return toolError("400", "missing connection_id", "connection_id is required"), nil
	}
	driver, _ := req.Params.Arguments["driver"].(string)

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    now,
		Tool:         "test_connection",
		ConnectionID: connID,
		Driver:       driver,
	})

	conn, err := h.Manager.Get(connID)
	if err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "test_connection",
			ConnectionID: connID,
			Driver:       driver,
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return toolError("503", "connection not found", err.Error()), nil
	}

	pingStart := time.Now()
	if err := conn.PingContext(ctx); err != nil {
		h.Audit.Log(audit.AuditEntry{
			QueryID:      queryID,
			Timestamp:    time.Now(),
			Tool:         "test_connection",
			ConnectionID: connID,
			Driver:       conn.DriverName(),
			DurationMS:   time.Since(now).Milliseconds(),
			Success:      false,
			Error:        err.Error(),
		})
		return toolError("503", "ping failed", err.Error()), nil
	}
	latencyMS := time.Since(pingStart).Milliseconds()

	result := fmt.Sprintf(`{"connection_id":%q,"driver":%q,"latency_ms":%d,"ok":true}`,
		connID, conn.DriverName(), latencyMS)

	h.Audit.Log(audit.AuditEntry{
		QueryID:      queryID,
		Timestamp:    time.Now(),
		Tool:         "test_connection",
		ConnectionID: connID,
		Driver:       conn.DriverName(),
		DurationMS:   time.Since(now).Milliseconds(),
		Success:      true,
	})

	return mcp.NewToolResultText(result), nil
}
