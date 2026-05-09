package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/agp/db-mcp/internal/setup"
	"github.com/urfave/cli/v2"
)

// SetupCommand returns the setup CLI command.
func SetupCommand() *cli.Command {
	return &cli.Command{
		Name:      "setup",
		Usage:     "Interactive setup wizard for database connections",
		UsageText: "db-mcp setup [options]",
		Description: `Run the interactive setup wizard to configure database connections.

The wizard will:
  1. Prompt for database configurations (PostgreSQL, MySQL, SQLite)
  2. Test each connection for validity
  3. Save configuration to file
  4. Register db-mcp with AI agents (Claude Desktop, Claude Code, Cursor)`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Configuration file path",
				Value:   "~/.config/db-mcp/.databases.json",
			},
			&cli.BoolFlag{
				Name:    "skip-connection-test",
				Usage:   "Skip connection testing for each database",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "skip-agent-registration",
				Usage:   "Skip agent registration step",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "no-register",
				Usage:   "Alias for --skip-agent-registration",
				Value:   false,
			},
		},
		Action: setupAction,
	}
}

// setupAction handles the setup command execution.
func setupAction(c *cli.Context) error {
	slog.Info("starting db-mcp setup wizard")

	// Get configuration options
	configPath := c.String("config")
	skipConnTest := c.Bool("skip-connection-test")
	skipAgentReg := c.Bool("skip-agent-registration") || c.Bool("no-register")

	// Determine server binary path
	serverPath, err := getServerPath()
	if err != nil {
		slog.Warn("could not determine server path for agent registration", "error", err)
		serverPath = "db-mcp"
	}

	// Create setup options
	opts := &setup.SetupOptions{
		ConfigPath:            configPath,
		SkipConnectionTest:    skipConnTest,
		SkipAgentRegistration: skipAgentReg,
	}

	// Create and run wizard
	wizard := setup.NewWizard(opts)
	result, err := wizard.Run()
	if err != nil {
		slog.Error("setup wizard failed", "error", err)
		fmt.Printf("\n❌ Setup failed: %v\n", err)
		return err
	}

	// Register with agents if not skipped
	if !skipAgentReg && result.ConfigWritten {
		fmt.Println("\n🤖 Registering with AI agents...")
		registry := setup.NewAgentRegistry(serverPath, configPath)
		registrationResults := registry.RegisterAll(result.AgentsRegistered)

		for agent, err := range registrationResults {
			if err != nil {
				slog.Warn("agent registration failed", "agent", agent, "error", err)
				fmt.Printf("  ⚠️  %s: %v\n", agent, err)
			} else {
				slog.Info("agent registered successfully", "agent", agent)
				fmt.Printf("  ✅ %s\n", agent)
			}
		}
	}

	// Show completion message
	displaySetupCompletion(result)

	return nil
}

// getServerPath tries to determine the path to the db-mcp binary.
func getServerPath() (string, error) {
	// Try to get executable path
	exe, err := os.Executable()
	if err == nil {
		// Resolve symlinks
		resolved, err := filepath.EvalSymlinks(exe)
		if err == nil {
			return resolved, nil
		}
		return exe, nil
	}

	// Fallback: try common locations
	commonPaths := []string{
		"/usr/local/bin/db-mcp",
		"/opt/db-mcp/db-mcp",
		"./bin/mcp-db-server",
		"./db-mcp",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("could not determine server path")
}

// displaySetupCompletion shows the setup completion summary.
func displaySetupCompletion(result *setup.SetupResult) {
	if result == nil {
		return
	}

	separator := repeatString("═", 70)
	fmt.Println("\n" + separator)
	if result.ConfigWritten {
		fmt.Println("✅ Setup completed successfully!")
	} else {
		fmt.Println("⚠️  Setup completed with warnings")
	}
	fmt.Println(separator)

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  • %d database(s) configured\n", result.DatabasesAdded)
	fmt.Printf("  • Config: %s\n", result.ConfigPath)

	if len(result.AgentsRegistered) > 0 {
		fmt.Println("\n🤖 Registered agents:")
		for agent, success := range result.AgentsRegistered {
			symbol := "✓"
			if !success {
				symbol = "✗"
			}
			fmt.Printf("  %s %s\n", symbol, agent)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Println("\n⚠️  Errors:")
		for _, errMsg := range result.Errors {
			fmt.Printf("  • %s\n", errMsg)
		}
	}

	fmt.Println("\n📝 Next steps:")
	if result.ConfigWritten {
		fmt.Println("  1. Test connections: db-mcp status --config " + result.ConfigPath)
		fmt.Println("  2. Start the server: mcp-db-server")
		fmt.Println("  3. Restart Claude Desktop or reload in Claude Code/Cursor")
	} else {
		fmt.Println("  1. Run setup again: db-mcp setup")
	}
	fmt.Println()
}

// repeatString repeats a string count times.
func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
