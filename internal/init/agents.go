package init

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agp/db-mcp/internal/config"
)

// WriteClaudeCodeConfig registers db-mcp with the Claude Code CLI via `claude mcp add`.
// Uses --scope user so the server is available in all projects, not just the current one.
func WriteClaudeCodeConfig(binaryPath string) error {
	cmd := exec.Command("claude", "mcp", "add", "db-mcp", binaryPath,
		"--scope", "user",
		"--env", "MCP_DB_CONFIG_PATH="+config.DefaultConfigPath(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// WriteClaudeDesktopConfig configures Claude Desktop with db-mcp.
// Config file: ~/.claude/claude_desktop_config.json
func WriteClaudeDesktopConfig(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(home, ".claude", "claude_desktop_config.json")

	serverEntry := map[string]any{
		"command": binaryPath,
		"args":    []string{},
		"env": map[string]string{
			"MCP_DB_CONFIG_PATH": config.DefaultConfigPath(),
		},
	}
	return mergeJSON(cfgPath, "mcpServers.db-mcp", serverEntry)
}

// WriteOpenCodeConfig configures OpenCode with db-mcp.
// Config file: ~/.config/opencode/opencode.json (XDG convention)
func WriteOpenCodeConfig(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cfgDir := os.Getenv("XDG_CONFIG_HOME")
	if cfgDir == "" {
		cfgDir = filepath.Join(home, ".config")
	}
	cfgPath := filepath.Join(cfgDir, "opencode", "opencode.json")

	serverEntry := map[string]any{
		"type":    "local",
		"command": []string{binaryPath},
		"enabled": true,
	}
	return mergeJSON(cfgPath, "mcp.db-mcp", serverEntry)
}

// WriteVSCodeConfig configures GitHub Copilot in VS Code with db-mcp.
// Config file: .vscode/mcp.json in the current working directory.
func WriteVSCodeConfig(binaryPath string) error {
	cfgPath := filepath.Join(".vscode", "mcp.json")

	serverEntry := map[string]any{
		"command": binaryPath,
		"args":    []string{},
		"type":    "stdio",
	}
	return mergeJSON(cfgPath, "servers.db-mcp", serverEntry)
}
