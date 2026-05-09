package setup

import (
	"fmt"
	"log/slog"
	"time"
)

// Wizard orchestrates the interactive database setup flow.
type Wizard struct {
	options       *SetupOptions
	tester        *ConnectionTester
	configWriter  *ConfigWriter
	result        *SetupResult
}

// NewWizard creates a new setup wizard.
func NewWizard(options *SetupOptions) *Wizard {
	if options == nil {
		options = &SetupOptions{
			ConfigPath: expandHome("~/.config/db-mcp/.databases.json"),
		}
	}

	return &Wizard{
		options: options,
		tester:  NewConnectionTester(5 * time.Second),
		configWriter: NewConfigWriter(options.ConfigPath),
		result: &SetupResult{
			ConfigPath:       options.ConfigPath,
			AgentsRegistered: make(map[string]bool),
		},
	}
}

// Run executes the setup wizard.
func (w *Wizard) Run() (*SetupResult, error) {
	slog.Info("starting setup wizard")

	// Display welcome message
	displayWelcome()

	// Collect database configurations
	databases, err := w.promptDatabases()
	if err != nil {
		return w.result, fmt.Errorf("database configuration: %w", err)
	}

	if len(databases) == 0 {
		fmt.Println("\n❌ No databases configured. Setup cancelled.")
		return w.result, nil
	}

	w.result.DatabasesAdded = len(databases)

	// Test connections if not skipped
	if !w.options.SkipConnectionTest {
		fmt.Println("\n🔍 Testing connections...")
		for i, db := range databases {
			fmt.Printf("  Testing %s (%s)... ", db.Name, db.Driver)
			success, msg, err := w.tester.TestConnection(&db)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return w.result, fmt.Errorf("connection test failed: %w", err)
			}
			if !success {
				fmt.Printf("❌ %s\n", msg)
				// Ask if user wants to continue
				retry, err := promptRetry(i + 1)
				if err != nil {
					return w.result, err
				}
				if retry {
					newDB, err := PromptDatabase(i)
					if err != nil {
						return w.result, err
					}
					databases[i] = *newDB
					i-- // Retry this database
				} else {
					return w.result, fmt.Errorf("connection test failed for %s", db.Name)
				}
			} else {
				fmt.Printf("✅ %s\n", msg)
			}
		}
	}

	// Write config
	fmt.Println("\n💾 Writing configuration...")
	if err := w.configWriter.Write(databases); err != nil {
		return w.result, fmt.Errorf("failed to write config: %w", err)
	}
	w.result.ConfigWritten = true
	fmt.Printf("✅ Config saved to %s\n", w.options.ConfigPath)

	// Agent registration if not skipped
	if !w.options.SkipAgentRegistration {
		fmt.Println("\n🤖 Agent Registration")
		if err := w.promptAgentRegistration(); err != nil {
			slog.Warn("agent registration failed", "error", err)
			// Don't fail setup on agent registration errors
		}
	}

	// Display completion message
	displayCompletion(w.result)

	return w.result, nil
}

// promptDatabases prompts the user for database configurations.
func (w *Wizard) promptDatabases() ([]DatabaseConfig, error) {
	var databases []DatabaseConfig

	for i := 0; ; i++ {
		db, err := PromptDatabase(i)
		if err != nil {
			return nil, err
		}

		// Validate configuration
		if err := ValidateConnection(db); err != nil {
			fmt.Printf("❌ Validation error: %v\n", err)
			continue
		}

		databases = append(databases, *db)

		// Ask if user wants to add another
		if i >= 9 { // Max 10 databases
			fmt.Println("\n⚠️  Maximum 10 databases allowed")
			break
		}

		addAnother, err := PromptAddAnother()
		if err != nil {
			return nil, err
		}

		if !addAnother {
			break
		}
	}

	return databases, nil
}

// promptAgentRegistration prompts for agent registration preferences.
func (w *Wizard) promptAgentRegistration() error {
	agents, err := PromptAgentSelection()
	if err != nil {
		return err
	}

	// Store results
	for agent, selected := range agents {
		w.result.AgentsRegistered[agent] = selected
	}

	return nil
}

// promptRetry asks if the user wants to retry a failed connection.
func promptRetry(dbIndex int) (bool, error) {
	retry := PromptRetryConnection(dbIndex)
	return retry, nil
}

// displayWelcome displays the welcome message.
func displayWelcome() {
	fmt.Print("\n╔════════════════════════════════════════════════════════════════╗\n")
	fmt.Println("║         Welcome to db-mcp Setup Wizard! 🚀                     ║")
	fmt.Print("╚════════════════════════════════════════════════════════════════╝\n")
	fmt.Println("\nThis wizard will help you configure database connections.")
	fmt.Print("Setup typically takes < 2 minutes.\n")
}

// displayCompletion displays the completion message.
func displayCompletion(result *SetupResult) {
	fmt.Print("\n╔════════════════════════════════════════════════════════════════╗\n")
	fmt.Println("║              Setup Complete! ✅                                ║")
	fmt.Print("╚════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  ✓ %d database(s) configured\n", result.DatabasesAdded)
	fmt.Printf("  ✓ Config saved to %s\n", result.ConfigPath)

	if len(result.AgentsRegistered) > 0 {
		fmt.Println("\n🤖 Agent Registration:")
		for agent, success := range result.AgentsRegistered {
			if success {
				fmt.Printf("  ✓ %s\n", agent)
			} else {
				fmt.Printf("  ✗ %s\n", agent)
			}
		}
	}

	fmt.Println("\n📝 Next Steps:")
	fmt.Println("  1. Restart Claude Desktop to load db-mcp")
	fmt.Println("  2. Check connection status: db-mcp --status")
	fmt.Print("  3. Access your databases via Claude!\n")
}
