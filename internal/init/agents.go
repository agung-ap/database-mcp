package init

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/agung-ap/database-mcp/internal/config"
)

// claudeDesktopConfigPath returns the platform-specific location of Claude
// Desktop's config file.
func claudeDesktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	default:
		cfgDir := os.Getenv("XDG_CONFIG_HOME")
		if cfgDir == "" {
			cfgDir = filepath.Join(home, ".config")
		}
		return filepath.Join(cfgDir, "Claude", "claude_desktop_config.json"), nil
	}
}

// WriteClaudeCodeConfig registers database-mcp with the Claude Code CLI via `claude mcp add`.
// Uses --scope user so the server is available in all projects, not just the current one.
func WriteClaudeCodeConfig(binaryPath string) error {
	// binaryPath comes from os.Executable() in the caller — it's the current
	// binary's own path, not user-controlled input. We validate it anyway to
	// catch programming errors early, then suppress the gosec G204 warning
	// (subprocess with variable args) since the value is proven safe.
	if !filepath.IsAbs(binaryPath) && binaryPath != "database-mcp" {
		return fmt.Errorf("invalid binary path: %q", binaryPath)
	}
	//nolint:gosec // binaryPath validated above; sourced from os.Executable()
	cmd := exec.Command("claude", "mcp", "add", "database-mcp", binaryPath,
		"--scope", "user",
		"--env", "MCP_DB_CONFIG_PATH="+config.DefaultConfigPath(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// WriteClaudeDesktopConfig configures Claude Desktop with database-mcp.
func WriteClaudeDesktopConfig(binaryPath string) error {
	cfgPath, err := claudeDesktopConfigPath()
	if err != nil {
		return err
	}

	serverEntry := map[string]any{
		"command": binaryPath,
		"args":    []string{},
		"env": map[string]string{
			"MCP_DB_CONFIG_PATH": config.DefaultConfigPath(),
		},
	}
	return mergeJSON(cfgPath, "mcpServers.database-mcp", serverEntry)
}

// WriteOpenCodeConfig configures OpenCode with database-mcp.
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
	return mergeJSON(cfgPath, "mcp.database-mcp", serverEntry)
}

// WriteVSCodeConfig configures GitHub Copilot in VS Code with database-mcp.
// Config file: .vscode/mcp.json in the current working directory.
func WriteVSCodeConfig(binaryPath string) error {
	cfgPath := filepath.Join(".vscode", "mcp.json")

	serverEntry := map[string]any{
		"command": binaryPath,
		"args":    []string{},
		"type":    "stdio",
	}
	return mergeJSON(cfgPath, "servers.database-mcp", serverEntry)
}
