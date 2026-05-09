// Package mcp provides the MCP protocol handler using stdio transport.
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server implementation.
type Server struct {
	s *mcp.Server
}

// NewServer creates a new MCP server instance with the given name and version.
func NewServer(name, version string) *Server {
	impl := &mcp.Implementation{
		Name:    name,
		Version: version,
	}
	s := mcp.NewServer(impl, nil)
	return &Server{s: s}
}

// AddTool registers a tool with no input parameters.
// Use AddToolWithInput for tools that accept input parameters.
func (srv *Server) AddTool(tool *mcp.Tool, handler func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, any, error)) {
	mcp.AddTool[struct{}, any](srv.s, tool, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return handler(ctx, req)
	})
}

// AddToolWithInput registers a tool with typed input parameters.
// In and Out are the input and output types for the tool.
func AddToolWithInput[In, Out any](srv *Server, tool *mcp.Tool, handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)) {
	mcp.AddTool[In, Out](srv.s, tool, handler)
}

// ServeStdio starts the server using stdio transport (blocks until ctx is done or stdin closes).
func (srv *Server) ServeStdio(ctx context.Context) error {
	return srv.s.Run(ctx, &mcp.StdioTransport{})
}
