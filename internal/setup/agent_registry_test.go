package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentRegistry(t *testing.T) {
	serverPath := "/usr/local/bin/db-mcp"
	configPath := "/home/user/.config/db-mcp/.databases.json"

	registry := NewAgentRegistry(serverPath, configPath)
	assert.NotNil(t, registry)
	assert.Equal(t, serverPath, registry.serverPath)
	assert.Equal(t, configPath, registry.configPath)
}

func TestAgentRegistry_RegisterAll_Empty(t *testing.T) {
	registry := NewAgentRegistry("/bin/db-mcp", "/config.json")
	agents := map[string]bool{
		"claude-desktop": false,
		"claude-code":    false,
		"cursor":         false,
	}

	results := registry.RegisterAll(agents)
	assert.Empty(t, results)
}

func TestAgentRegistry_RegisterClaudeDesktopAtPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude", "claude_desktop_config.json")

	serverPath := "/usr/local/bin/db-mcp"
	dbConfigPath := "/home/user/.config/db-mcp/.databases.json"
	registry := NewAgentRegistry(serverPath, dbConfigPath)

	// Register at custom path
	err := registry.RegisterClaudeDesktopAtPath(configPath)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, configPath)

	// Verify content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config ClaudeDesktopConfig
	err = json.Unmarshal(content, &config)
	require.NoError(t, err)

	assert.Contains(t, config.MCPServers, "db-mcp")
	assert.Equal(t, serverPath, config.MCPServers["db-mcp"].Command)
	assert.Equal(t, dbConfigPath, config.MCPServers["db-mcp"].Env["MCP_DB_CONFIG_PATH"])
}

func TestAgentRegistry_RegisterClaudeCode(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tmpDir)

	serverPath := "/usr/local/bin/db-mcp"
	registry := NewAgentRegistry(serverPath, "/home/user/.config/db-mcp/.databases.json")

	// Register
	err := registry.RegisterClaudeCode()
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, ".mcp.json")

	// Verify content
	content, err := os.ReadFile(".mcp.json")
	require.NoError(t, err)

	var config MCPClientConfig
	err = json.Unmarshal(content, &config)
	require.NoError(t, err)

	assert.Contains(t, config.MCPServers, "db-mcp")
	assert.Equal(t, serverPath, config.MCPServers["db-mcp"].Command)
}

func TestAgentRegistry_RegisterCursor(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tmpDir)

	serverPath := "/usr/local/bin/db-mcp"
	registry := NewAgentRegistry(serverPath, "/home/user/.config/db-mcp/.databases.json")

	// Register
	err := registry.RegisterCursor()
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, ".cursor/models/index.json")

	// Verify content
	content, err := os.ReadFile(".cursor/models/index.json")
	require.NoError(t, err)

	var config MCPClientConfig
	err = json.Unmarshal(content, &config)
	require.NoError(t, err)

	assert.Contains(t, config.MCPServers, "db-mcp")
	assert.Equal(t, serverPath, config.MCPServers["db-mcp"].Command)
}

func TestAgentRegistry_RegisterClaudeDesktopAtPath_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude", "claude_desktop_config.json")

	// Create initial config with other servers
	dir := filepath.Dir(configPath)
	err := os.MkdirAll(dir, 0o700)
	require.NoError(t, err)

	initialConfig := ClaudeDesktopConfig{
		MCPServers: map[string]MCPServerConfig{
			"other-server": {
				Command: "/bin/other",
				Env: map[string]string{
					"OTHER_VAR": "value",
				},
			},
		},
	}

	initialData, err := json.Marshal(initialConfig)
	require.NoError(t, err)
	err = os.WriteFile(configPath, initialData, 0o600)
	require.NoError(t, err)

	// Register db-mcp
	serverPath := "/usr/local/bin/db-mcp"
	registry := NewAgentRegistry(serverPath, "/config.json")

	err = registry.RegisterClaudeDesktopAtPath(configPath)
	require.NoError(t, err)

	// Verify both servers are present
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config ClaudeDesktopConfig
	err = json.Unmarshal(content, &config)
	require.NoError(t, err)

	assert.Contains(t, config.MCPServers, "db-mcp")
	assert.Contains(t, config.MCPServers, "other-server")
	assert.Equal(t, "/bin/other", config.MCPServers["other-server"].Command)
}

func TestAgentRegistry_RegisterClaudeCode_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tmpDir)

	// Create initial config
	initialConfig := MCPClientConfig{
		MCPServers: map[string]MCPServerConfig{
			"other-server": {
				Command: "/bin/other",
			},
		},
	}

	initialData, err := json.Marshal(initialConfig)
	require.NoError(t, err)
	err = os.WriteFile(".mcp.json", initialData, 0o600)
	require.NoError(t, err)

	// Register db-mcp
	serverPath := "/usr/local/bin/db-mcp"
	registry := NewAgentRegistry(serverPath, "/config.json")

	err = registry.RegisterClaudeCode()
	require.NoError(t, err)

	// Verify both servers are present
	content, err := os.ReadFile(".mcp.json")
	require.NoError(t, err)

	var config MCPClientConfig
	err = json.Unmarshal(content, &config)
	require.NoError(t, err)

	assert.Contains(t, config.MCPServers, "db-mcp")
	assert.Contains(t, config.MCPServers, "other-server")
}

func TestAgentRegistry_CreateBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude", "claude_desktop_config.json")

	// Create directory and initial config
	dir := filepath.Dir(configPath)
	err := os.MkdirAll(dir, 0o700)
	require.NoError(t, err)

	initialConfig := ClaudeDesktopConfig{
		MCPServers: map[string]MCPServerConfig{
			"old-server": {Command: "/bin/old"},
		},
	}

	initialData, err := json.Marshal(initialConfig)
	require.NoError(t, err)
	err = os.WriteFile(configPath, initialData, 0o600)
	require.NoError(t, err)

	// Register new server
	registry := NewAgentRegistry("/bin/db-mcp", "/config.json")

	err = registry.RegisterClaudeDesktopAtPath(configPath)
	require.NoError(t, err)

	// Verify backup exists
	backupPath := configPath + ".backup"
	assert.FileExists(t, backupPath)

	// Verify backup content
	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err)

	var backupConfig ClaudeDesktopConfig
	err = json.Unmarshal(backupContent, &backupConfig)
	require.NoError(t, err)

	assert.Contains(t, backupConfig.MCPServers, "old-server")
}

func TestMCPServerConfig_Structure(t *testing.T) {
	config := MCPServerConfig{
		Command: "/usr/local/bin/db-mcp",
		Args:    []string{"--flag", "value"},
		Env: map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
		},
	}

	assert.Equal(t, "/usr/local/bin/db-mcp", config.Command)
	assert.Len(t, config.Args, 2)
	assert.Len(t, config.Env, 2)
	assert.Equal(t, "value1", config.Env["VAR1"])
}

func TestClaudeDesktopConfig_JSONMarshaling(t *testing.T) {
	config := ClaudeDesktopConfig{
		MCPServers: map[string]MCPServerConfig{
			"db-mcp": {
				Command: "/bin/db-mcp",
				Env: map[string]string{
					"MCP_DB_CONFIG_PATH": "/config.json",
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	// Unmarshal back
	var unmarshaled ClaudeDesktopConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, config, unmarshaled)
}

func TestMCPClientConfig_JSONMarshaling(t *testing.T) {
	config := MCPClientConfig{
		MCPServers: map[string]MCPServerConfig{
			"db-mcp": {
				Command: "/bin/db-mcp",
				Env: map[string]string{
					"MCP_DB_CONFIG_PATH": "/config.json",
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	// Unmarshal back
	var unmarshaled MCPClientConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, config, unmarshaled)
}

func TestAgentRegistry_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tmpDir)

	registry := NewAgentRegistry("/bin/db-mcp", "/config.json")

	// Register with Claude Code
	err := registry.RegisterClaudeCode()
	require.NoError(t, err)

	// Check file permissions (should be 0o600)
	info, err := os.Stat(".mcp.json")
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestAgentRegistry_CreateNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tmpDir)

	registry := NewAgentRegistry("/bin/db-mcp", "/config.json")

	// Register with Cursor (creates nested directories)
	err := registry.RegisterCursor()
	require.NoError(t, err)

	// Verify nested directories were created
	assert.DirExists(t, ".cursor")
	assert.DirExists(t, ".cursor/models")
	assert.FileExists(t, ".cursor/models/index.json")
}

func TestAgentRegistry_RegisterAll_MultipleAgents(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tmpDir)

	// Create .claude directory manually
	err := os.MkdirAll(".claude", 0o700)
	require.NoError(t, err)

	registry := NewAgentRegistry("/bin/db-mcp", "/config.json")

	// For this test, we use a custom path for Claude Desktop
	claudeConfigPath := filepath.Join(tmpDir, ".claude", "claude_desktop_config.json")
	err = registry.RegisterClaudeDesktopAtPath(claudeConfigPath)
	require.NoError(t, err)

	// Register the other agents normally
	err = registry.RegisterClaudeCode()
	require.NoError(t, err)

	err = registry.RegisterCursor()
	require.NoError(t, err)

	// Verify all files created
	assert.FileExists(t, ".claude/claude_desktop_config.json")
	assert.FileExists(t, ".mcp.json")
	assert.FileExists(t, ".cursor/models/index.json")
}

func TestAgentRegistry_EnvVariableInConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	_ = os.Chdir(tmpDir)

	dbConfigPath := "/home/user/.config/db-mcp/.databases.json"
	serverPath := "/usr/local/bin/db-mcp"
	registry := NewAgentRegistry(serverPath, dbConfigPath)

	// Register
	err := registry.RegisterClaudeCode()
	require.NoError(t, err)

	// Verify config contains correct env var
	content, err := os.ReadFile(".mcp.json")
	require.NoError(t, err)

	var config MCPClientConfig
	err = json.Unmarshal(content, &config)
	require.NoError(t, err)

	serverConfig := config.MCPServers["db-mcp"]
	assert.Equal(t, dbConfigPath, serverConfig.Env["MCP_DB_CONFIG_PATH"])
}
