package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/agp/db-mcp/internal/config"
	"github.com/agp/db-mcp/internal/db"
	"github.com/agp/db-mcp/internal/tools"
	"github.com/urfave/cli/v2"
)

// StatusCommand returns the status CLI command.
func StatusCommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Check health of all configured database connections",
		UsageText: "db-mcp status [options]",
		Description: `Check the health and connectivity status of all configured database connections.

The status command will:
  1. Load database configuration from file
  2. Perform connectivity tests on each connection
  3. Display results in table or JSON format
  4. Return exit code 0 if all healthy, 1 if any unhealthy`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Configuration file path",
				Value:   "~/.config/db-mcp/.databases.json",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Usage:   "Output format: table or json",
				Value:   "table",
			},
			&cli.IntFlag{
				Name:    "timeout",
				Aliases: []string{"t"},
				Usage:   "Health check timeout in seconds",
				Value:   10,
			},
			&cli.BoolFlag{
				Name:    "parallel",
				Aliases: []string{"p"},
				Usage:   "Run health checks in parallel",
				Value:   true,
			},
			&cli.StringSliceFlag{
				Name:    "only",
				Usage:   "Check only specific connections (comma-separated)",
				Value:   cli.NewStringSlice(),
			},
		},
		Action: statusAction,
	}
}

// statusAction handles the status command execution.
func statusAction(c *cli.Context) error {
	slog.Info("checking database connection status")

	// Get configuration options
	configPath := c.String("config")
	format := c.String("format")
	timeout := time.Duration(c.Int("timeout")) * time.Second
	parallel := c.Bool("parallel")
	only := c.StringSlice("only")

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load configuration", "config_path", configPath, "error", err)
		fmt.Printf("❌ Error: Failed to load configuration from %s\n", configPath)
		fmt.Printf("   %v\n", err)
		return err
	}

	if len(cfg.Databases) == 0 {
		fmt.Println("⚠️  No database connections configured")
		return nil
	}

	// Create database manager
	manager, err := db.NewManager(cfg.Databases)
	if err != nil {
		slog.Error("failed to create database manager", "error", err)
		fmt.Printf("❌ Error: Failed to initialize database manager\n")
		fmt.Printf("   %v\n", err)
		return err
	}
	defer manager.CloseAll()

	// Create health checker
	healthChecker := tools.NewHealthChecker(manager, slog.Default())

	// Prepare health check request
	req := &tools.HealthCheckRequest{
		ConnectionIDs: only,
		Timeout:       timeout,
		Parallel:      parallel,
	}

	// Perform health checks
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()

	resp, err := healthChecker.Check(ctx, req)
	if err != nil {
		slog.Error("health check failed", "error", err)
		fmt.Printf("❌ Error: Health check failed\n")
		fmt.Printf("   %v\n", err)
		return err
	}

	// Display results based on format
	switch format {
	case "json":
		displayStatusJSON(resp)
	case "table":
		displayStatusTable(resp)
	default:
		fmt.Printf("❌ Error: Unknown format '%s' (use 'table' or 'json')\n", format)
		return fmt.Errorf("unknown format: %s", format)
	}

	// Return appropriate exit code
	if resp.UnhealthyCount > 0 {
		return fmt.Errorf("some connections are unhealthy")
	}

	return nil
}

// displayStatusTable displays health check results in table format.
func displayStatusTable(resp *tools.HealthCheckResponse) {
	fmt.Println("\n" + repeatString("═", 100))
	fmt.Println("Database Connection Status")
	fmt.Println(repeatString("═", 100))

	// Summary
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  • Checked:     %d\n", resp.TotalChecks)
	fmt.Printf("  • Healthy:     %d ✅\n", resp.HealthyCount)
	fmt.Printf("  • Unhealthy:   %d ❌\n", resp.UnhealthyCount)
	if resp.UnknownCount > 0 {
		fmt.Printf("  • Unknown:     %d ⚠️\n", resp.UnknownCount)
	}

	// Results table
	fmt.Println("\nDetailed Results:")
	fmt.Println()
	fmt.Println(formatTableHeader())
	fmt.Println(repeatString("─", 100))

	for _, result := range resp.Results {
		fmt.Println(formatTableRow(&result))
	}

	// Errors
	if len(resp.Errors) > 0 {
		fmt.Println("\nErrors:")
		for connID, errMsg := range resp.Errors {
			fmt.Printf("  • %s: %s\n", connID, errMsg)
		}
	}

	fmt.Println()
	fmt.Println(repeatString("═", 100))
}

// displayStatusJSON displays health check results in JSON format.
func displayStatusJSON(resp *tools.HealthCheckResponse) {
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Printf("❌ Error: Failed to marshal response to JSON\n")
		fmt.Printf("   %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// formatTableHeader returns the table header.
func formatTableHeader() string {
	return fmt.Sprintf("%-25s | %-10s | %-15s | %-10s | %-35s",
		"Connection", "Driver", "Status", "Latency", "Details")
}

// formatTableRow formats a single status result as a table row.
func formatTableRow(result *tools.HealthCheckResult) string {
	status := result.Status
	switch result.Status {
	case "healthy":
		status = "✅ " + status
	case "unhealthy":
		status = "❌ " + status
	case "unknown":
		status = "⚠️  " + status
	}

	latency := fmt.Sprintf("%dms", result.Latency)
	details := result.Details.ConnectionURL
	if details == "" {
		details = result.Details.Host
		if result.Details.Port > 0 {
			details = fmt.Sprintf("%s:%d", details, result.Details.Port)
		}
	}

	return fmt.Sprintf("%-25s | %-10s | %-15s | %-10s | %-35s",
		result.ConnectionID,
		result.Driver,
		status,
		latency,
		truncateString(details, 35))
}

// truncateString truncates a string to maximum length with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
