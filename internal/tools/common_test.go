package tools

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrResult_ProducesValidJSONForHostileInput(t *testing.T) {
	res := errResult("500", `message with "quotes" and \backslash`, "line one\nline two")
	require.Len(t, res.Content, 1)

	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)

	var decoded struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &decoded), "errResult must always produce valid JSON even when inputs contain quotes/newlines")
	assert.Equal(t, "500", decoded.Code)
	assert.Contains(t, decoded.Message, "quotes")
	assert.Contains(t, decoded.Detail, "line two")
}

func TestClampLimit(t *testing.T) {
	assert.Equal(t, DefaultRowLimit, clampLimit(0))
	assert.Equal(t, DefaultRowLimit, clampLimit(-5))
	assert.Equal(t, 50, clampLimit(50))
	assert.Equal(t, MaxRowLimit, clampLimit(999999))
}
