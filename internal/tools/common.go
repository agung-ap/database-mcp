package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	DefaultRowLimit = 100
	MaxRowLimit     = 1000
)

// toolError returns a structured MCP tool error with a code, message, and detail.
func toolError(code, message, detail string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf(
		`{"code":"%s","message":"%s","detail":"%s"}`,
		code, message, detail,
	))
}

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

// getStringArg extracts a string argument from the request arguments map.
func getStringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// getIntArg extracts an int argument from the request arguments map (handles float64 from JSON).
func getIntArg(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	}
	return defaultVal
}

// getBoolArg extracts a bool argument from the request arguments map.
func getBoolArg(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

// getSliceArg extracts a []any argument from the request arguments map.
func getSliceArg(args map[string]any, key string) []any {
	v, _ := args[key].([]any)
	return v
}
