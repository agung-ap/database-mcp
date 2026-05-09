// Package mcp provides the MCP protocol handler using stdio transport.
package mcp

import (
	"context"
	"os"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolHandler is the function signature for tool handlers.
type ToolHandler = server.ToolHandlerFunc

// Server wraps the MCP server with its handlers registered.
type Server struct {
	s *server.MCPServer
}

// NewServer creates a new MCP server instance with the given name and version.
func NewServer(name, version string) *Server {
	s := server.NewMCPServer(name, version)
	return &Server{s: s}
}

// AddTool registers a tool with its handler on the underlying MCP server.
func (srv *Server) AddTool(tool mcpgo.Tool, handler server.ToolHandlerFunc) {
	srv.s.AddTool(tool, handler)
}

// ServeStdio starts the server using stdio transport (blocks until ctx is done or stdin closes).
func (srv *Server) ServeStdio(ctx context.Context) error {
	stdioServer := server.NewStdioServer(srv.s)
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}
