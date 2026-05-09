package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DefaultRowLimit = 100
	MaxRowLimit     = 1000
)

// clampLimit enforces the row limit constraints.
func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultRowLimit
	}
	if limit > MaxRowLimit {
		return MaxRowLimit
	}
	return limit
}

// newToolError returns a structured MCP tool error.
func newToolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("error: %v", err),
			},
		},
		IsError: true,
	}
}

// newToolTextResult returns a successful text result.
func newToolTextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}
}
