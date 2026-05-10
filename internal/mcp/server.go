package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the official MCP server.
type Server struct {
	s *mcp.Server
}

// NewServer creates a new MCP server instance with the given name and version.
func NewServer(name, version string) *Server {
	s := mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)
	return &Server{s: s}
}

// ServeStdio starts the server using stdio transport (blocks until ctx is done or stdin closes).
func (srv *Server) ServeStdio(ctx context.Context) error {
	return srv.s.Run(ctx, &mcp.StdioTransport{})
}
