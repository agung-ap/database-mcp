// Package init provides the interactive onboarding wizard for database-mcp.
package init

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agung-ap/database-mcp/internal/config"
)

// RunWizard runs the interactive setup wizard and returns an exit code.
func RunWizard() int {
	fmt.Println("Welcome to database-mcp setup!")
	fmt.Println()

	cfgDir := config.ConfigDir()
	cfgPath := config.DefaultConfigPath()
	fmt.Printf("Config directory: %s\n", cfgDir)
	fmt.Println()

	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		return 1
	}

	// Load existing config or start fresh
	var cfg config.Config
	if data, err := os.ReadFile(cfgPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
		fmt.Printf("Found existing config with %d connection(s).\n\n", len(cfg.Connections))
	}

	reader := bufio.NewReader(os.Stdin)

	// Add connections
	for promptBool(reader, "Add a database connection?", len(cfg.Connections) == 0) {
		conn := promptConnection(reader)
		cfg.Connections = append(cfg.Connections, conn)
		fmt.Printf("Connection %q added.\n\n", conn.ID)
	}

	if len(cfg.Connections) == 0 {
		fmt.Println("No connections configured. Exiting.")
		return 0
	}

	// Save connections.json
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding config: %v\n", err)
		return 1
	}
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}
	fmt.Printf("Config saved to %s\n\n", cfgPath)

	// Detect binary path
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = "database-mcp"
	}

	// Configure agents
	fmt.Println("Which agents would you like to configure?")
	configureAgents(reader, binaryPath)

	fmt.Println()
	fmt.Println("Setup complete! Restart your agent to load the new MCP server.")
	fmt.Printf("\nTo add more connections later, edit: %s\n", cfgPath)
	fmt.Println("To test a connection, run: database-mcp test <connection-id>")
	return 0
}

func promptConnection(reader *bufio.Reader) config.Connection {
	var conn config.Connection
	conn.ID = promptString(reader, "Connection ID (e.g. prod-pg)", "")
	conn.Driver = promptChoice(reader, "Driver", []string{"postgres", "mysql", "sqlserver"})
	conn.Host = promptStringDefault(reader, "Host", "localhost")

	switch conn.Driver {
	case "postgres":
		conn.Port = promptInt(reader, "Port", 5432)
	case "mysql":
		conn.Port = promptInt(reader, "Port", 3306)
	case "sqlserver":
		conn.Port = promptInt(reader, "Port", 1433)
	}

	conn.Database = promptString(reader, "Database name", "")
	conn.User = promptString(reader, "User", "")
	conn.Password = promptString(reader, "Password (leave blank to set via env)", "")

	sslDefault := "disable"
	if conn.Driver == "postgres" {
		sslDefault = "require"
	}
	conn.SSLMode = promptStringDefault(reader, "SSL mode [require/prefer/disable]", sslDefault)
	return conn
}

func configureAgents(reader *bufio.Reader, binaryPath string) {
	fmt.Println()
	if promptBool(reader, "  Claude Code (CLI)", true) {
		if err := WriteClaudeCodeConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not configure Claude Code: %v\n", err)
		} else {
			fmt.Println("  ✓ Claude Code configured (user scope)")
		}
	}

	if promptBool(reader, "  Claude Desktop (GUI app)", false) {
		if err := WriteClaudeDesktopConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not write Claude Desktop config: %v\n", err)
		} else {
			fmt.Println("  ✓ Claude Desktop configured")
		}
	}

	if promptBool(reader, "  OpenCode", true) {
		if err := WriteOpenCodeConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not write OpenCode config: %v\n", err)
		} else {
			fmt.Println("  ✓ OpenCode configured")
		}
	}

	if promptBool(reader, "  GitHub Copilot in VS Code (writes .vscode/mcp.json in current dir)", false) {
		if err := WriteVSCodeConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not write VS Code config: %v\n", err)
		} else {
			fmt.Println("  ✓ GitHub Copilot in VS Code configured (.vscode/mcp.json)")
		}
	}
}

// --- prompt helpers ---

func promptString(r *bufio.Reader, label, defaultVal string) string {
	for {
		if defaultVal != "" {
			fmt.Printf("  %s [%s]: ", label, defaultVal)
		} else {
			fmt.Printf("  %s: ", label)
		}
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && defaultVal != "" {
			return defaultVal
		}
		if line != "" {
			return line
		}
		fmt.Println("  (required)")
	}
}

func promptStringDefault(r *bufio.Reader, label, defaultVal string) string {
	fmt.Printf("  %s [%s]: ", label, defaultVal)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func promptInt(r *bufio.Reader, label string, defaultVal int) int {
	for {
		fmt.Printf("  %s [%d]: ", label, defaultVal)
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return defaultVal
		}
		n, err := strconv.Atoi(line)
		if err == nil && n > 0 {
			return n
		}
		fmt.Println("  (enter a valid port number)")
	}
}

func promptBool(r *bufio.Reader, label string, defaultYes bool) bool {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	fmt.Printf("  %s %s: ", label, hint)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

func promptChoice(r *bufio.Reader, label string, choices []string) string {
	for {
		fmt.Printf("  %s [%s]: ", label, strings.Join(choices, "/"))
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		for _, c := range choices {
			if line == c {
				return c
			}
		}
		fmt.Printf("  (choose one of: %s)\n", strings.Join(choices, ", "))
	}
}

// mergeJSON reads existing JSON from path (if any) and merges new key into it.
func mergeJSON(path string, key string, value any) error {
	obj := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &obj)
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 2 {
		sub, _ := obj[parts[0]].(map[string]any)
		if sub == nil {
			sub = make(map[string]any)
		}
		sub[parts[1]] = value
		obj[parts[0]] = sub
	} else {
		obj[key] = value
	}

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
