package setup

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// AgentRegistry manages registration with various AI agents.
type AgentRegistry struct {
	serverPath string
	configPath string
}

// NewAgentRegistry creates a new agent registry.
func NewAgentRegistry(serverPath, configPath string) *AgentRegistry {
	return &AgentRegistry{
		serverPath: serverPath,
		configPath: configPath,
	}
}

// RegisterAll registers the MCP server with all selected agents.
func (ar *AgentRegistry) RegisterAll(agents map[string]bool) map[string]error {
	results := make(map[string]error)

	if agents["claude-desktop"] {
		err := ar.RegisterClaudeDesktop()
		results["claude-desktop"] = err
		if err != nil {
			slog.Error("failed to register with Claude Desktop", "error", err)
		} else {
			slog.Info("successfully registered with Claude Desktop")
		}
	}

	if agents["claude-code"] {
		err := ar.RegisterClaudeCode()
		results["claude-code"] = err
		if err != nil {
			slog.Error("failed to register with Claude Code", "error", err)
		} else {
			slog.Info("successfully registered with Claude Code")
		}
	}

	if agents["cursor"] {
		err := ar.RegisterCursor()
		results["cursor"] = err
		if err != nil {
			slog.Error("failed to register with Cursor", "error", err)
		} else {
			slog.Info("successfully registered with Cursor")
		}
	}

	return results
}

// RegisterClaudeDesktop registers the MCP server with Claude Desktop.
// Config location: ~/.claude/claude_desktop_config.json
// For testing, use RegisterClaudeDesktopAtPath to specify a custom path.
func (ar *AgentRegistry) RegisterClaudeDesktop() error {
	configPath := expandHome("~/.claude/claude_desktop_config.json")
	return ar.RegisterClaudeDesktopAtPath(configPath)
}

// RegisterClaudeDesktopAtPath registers with Claude Desktop at a specific path (for testing).
func (ar *AgentRegistry) RegisterClaudeDesktopAtPath(configPath string) error {
	// Read existing config or create new
	config := &ClaudeDesktopConfig{
		MCPServers: make(map[string]MCPServerConfig),
	}

	// Try to read existing config
	if fileExists(configPath) {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read Claude Desktop config: %w", err)
		}

		if err := json.Unmarshal(content, config); err != nil {
			return fmt.Errorf("failed to parse Claude Desktop config: %w", err)
		}
	}

	// Add db-mcp server
	config.MCPServers["db-mcp"] = MCPServerConfig{
		Command: ar.serverPath,
		Args:    []string{},
		Env: map[string]string{
			"MCP_DB_CONFIG_PATH": ar.configPath,
		},
	}

	// Write config
	if err := ar.saveClaudeDesktopConfig(configPath, config); err != nil {
		return err
	}

	return nil
}

// RegisterClaudeCode registers the MCP server with Claude Code.
// Config location: .mcp.json in project root
func (ar *AgentRegistry) RegisterClaudeCode() error {
	configPath := ".mcp.json"

	// Read existing config or create new
	config := &MCPClientConfig{
		MCPServers: make(map[string]MCPServerConfig),
	}

	// Try to read existing config
	if fileExists(configPath) {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read Claude Code config: %w", err)
		}

		if err := json.Unmarshal(content, config); err != nil {
			return fmt.Errorf("failed to parse Claude Code config: %w", err)
		}
	}

	// Add db-mcp server
	config.MCPServers["db-mcp"] = MCPServerConfig{
		Command: ar.serverPath,
		Args:    []string{},
		Env: map[string]string{
			"MCP_DB_CONFIG_PATH": ar.configPath,
		},
	}

	// Write config
	return ar.saveMCPClientConfig(configPath, config)
}

// RegisterCursor registers the MCP server with Cursor.
// Config location: .cursor/models/index.json (or similar)
func (ar *AgentRegistry) RegisterCursor() error {
	configDir := ".cursor"
	configPath := filepath.Join(configDir, "models", "index.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("failed to create Cursor config directory: %w", err)
	}

	// Read existing config or create new
	config := &MCPClientConfig{
		MCPServers: make(map[string]MCPServerConfig),
	}

	// Try to read existing config
	if fileExists(configPath) {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read Cursor config: %w", err)
		}

		if err := json.Unmarshal(content, config); err != nil {
			return fmt.Errorf("failed to parse Cursor config: %w", err)
		}
	}

	// Add db-mcp server
	config.MCPServers["db-mcp"] = MCPServerConfig{
		Command: ar.serverPath,
		Args:    []string{},
		Env: map[string]string{
			"MCP_DB_CONFIG_PATH": ar.configPath,
		},
	}

	// Write config
	return ar.saveMCPClientConfig(configPath, config)
}

// saveClaudeDesktopConfig saves the Claude Desktop configuration.
func (ar *AgentRegistry) saveClaudeDesktopConfig(path string, config *ClaudeDesktopConfig) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Backup existing config
	if fileExists(path) {
		backupPath := path + ".backup"
		content, err := os.ReadFile(path)
		if err == nil {
			_ = os.WriteFile(backupPath, content, 0o600)
		}
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with atomic approach
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to finalize config: %w", err)
	}

	slog.Info("Claude Desktop config updated", "path", path)
	return nil
}

// saveMCPClientConfig saves a generic MCP client configuration.
func (ar *AgentRegistry) saveMCPClientConfig(path string, config *MCPClientConfig) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Backup existing config
	if fileExists(path) {
		backupPath := path + ".backup"
		content, err := os.ReadFile(path)
		if err == nil {
			_ = os.WriteFile(backupPath, content, 0o600)
		}
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with atomic approach
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to finalize config: %w", err)
	}

	slog.Info("MCP client config updated", "path", path)
	return nil
}

// ClaudeDesktopConfig represents the Claude Desktop configuration structure.
type ClaudeDesktopConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPClientConfig represents a generic MCP client configuration.
type MCPClientConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig represents an MCP server configuration entry.
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}
