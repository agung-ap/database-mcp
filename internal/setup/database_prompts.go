package setup

import (
	"fmt"
	"log/slog"

	"github.com/AlecAivazis/survey/v2"
)

// PromptDatabase prompts the user for a database configuration.
func PromptDatabase(index int) (*DatabaseConfig, error) {
	fmt.Printf("\n📦 Database %d Configuration\n", index+1)
	fmt.Println("═══════════════════════════════════════════")

	var name string
	err := survey.AskOne(&survey.Input{
		Message: "Database name (alphanumeric, underscore):",
		Default: fmt.Sprintf("db%d", index+1),
	}, &name)
	if err != nil {
		return nil, fmt.Errorf("database name: %w", err)
	}

	// Validate name
	if err := validateDatabaseName(name); err != nil {
		return nil, err
	}

	// Prompt for driver
	var driver string
	err = survey.AskOne(&survey.Select{
		Message: "Database Engine:",
		Options: []string{"postgres", "mysql", "sqlite"},
		Default: "postgres",
	}, &driver)
	if err != nil {
		return nil, fmt.Errorf("driver selection: %w", err)
	}

	cfg := &DatabaseConfig{
		Name:   name,
		Driver: driver,
	}

	// Prompt for driver-specific fields
	switch driver {
	case "postgres", "mysql":
		if err := promptSQLDatabase(cfg, driver); err != nil {
			return nil, err
		}
	case "sqlite":
		if err := promptSQLiteDatabase(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// promptSQLDatabase prompts for SQL database (PostgreSQL/MySQL) configuration.
func promptSQLDatabase(cfg *DatabaseConfig, driver string) error {
	// Host
	err := survey.AskOne(&survey.Input{
		Message: "Host:",
		Default: "localhost",
	}, &cfg.Host)
	if err != nil {
		return fmt.Errorf("host: %w", err)
	}

	// Port with default based on driver
	defaultPort := 5432
	if driver == "mysql" {
		defaultPort = 3306
	}
	var portStr string
	err = survey.AskOne(&survey.Input{
		Message: "Port:",
		Default: fmt.Sprintf("%d", defaultPort),
	}, &portStr)
	if err != nil {
		return fmt.Errorf("port: %w", err)
	}

	if portStr == "" {
		cfg.Port = defaultPort
	} else {
		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			return fmt.Errorf("invalid port number: %w", err)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("port must be between 1-65535")
		}
		cfg.Port = port
	}

	// Username
	err = survey.AskOne(&survey.Input{
		Message: "Username:",
	}, &cfg.Username)
	if err != nil {
		return fmt.Errorf("username: %w", err)
	}

	if cfg.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Password (hidden)
	err = survey.AskOne(&survey.Password{
		Message: "Password (leave empty for no password):",
	}, &cfg.Password)
	if err != nil {
		return fmt.Errorf("password: %w", err)
	}

	// Database
	err = survey.AskOne(&survey.Input{
		Message: "Database name:",
	}, &cfg.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}

	if cfg.Database == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	// SSL Mode (PostgreSQL only)
	if driver == "postgres" {
		err = survey.AskOne(&survey.Select{
			Message: "SSL Mode:",
			Options: []string{"disable", "allow", "prefer", "require"},
			Default: "prefer",
		}, &cfg.SSLMode)
		if err != nil {
			return fmt.Errorf("ssl mode: %w", err)
		}
	} else {
		// MySQL
		err = survey.AskOne(&survey.Select{
			Message: "SSL Mode:",
			Options: []string{"disabled", "preferred", "required"},
			Default: "preferred",
		}, &cfg.SSLMode)
		if err != nil {
			return fmt.Errorf("ssl mode: %w", err)
		}
	}

	return nil
}

// promptSQLiteDatabase prompts for SQLite database configuration.
func promptSQLiteDatabase(cfg *DatabaseConfig) error {
	err := survey.AskOne(&survey.Input{
		Message: "Database file path:",
		Default: "./db.sqlite3",
	}, &cfg.FilePath)
	if err != nil {
		return fmt.Errorf("file path: %w", err)
	}

	if cfg.FilePath == "" {
		cfg.FilePath = "./db.sqlite3"
	}

	return nil
}

// validateDatabaseName validates a database configuration name.
func validateDatabaseName(name string) error {
	if name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	if len(name) > 50 {
		return fmt.Errorf("database name cannot exceed 50 characters")
	}

	// Check for valid characters (alphanumeric and underscore only)
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return fmt.Errorf("database name must be alphanumeric or underscore only")
		}
	}

	return nil
}

// PromptAddAnother asks if the user wants to add another database.
func PromptAddAnother() (bool, error) {
	var addAnother bool
	err := survey.AskOne(&survey.Confirm{
		Message: "Add another database?",
		Default: false,
	}, &addAnother)
	return addAnother, err
}

// PromptAgentSelection prompts for agent registration preferences.
func PromptAgentSelection() (map[string]bool, error) {
	var selected []string
	err := survey.AskOne(&survey.MultiSelect{
		Message: "Register with agents:",
		Options: []string{
			"Claude Desktop",
			"Claude Code",
			"Cursor",
		},
		Default: []string{
			"Claude Desktop",
			"Claude Code",
		},
	}, &selected)
	if err != nil {
		return nil, fmt.Errorf("agent selection: %w", err)
	}

	result := map[string]bool{
		"claude-desktop": false,
		"claude-code":    false,
		"cursor":         false,
	}

	for _, agent := range selected {
		switch agent {
		case "Claude Desktop":
			result["claude-desktop"] = true
		case "Claude Code":
			result["claude-code"] = true
		case "Cursor":
			result["cursor"] = true
		}
	}

	slog.Debug("agent selection", "selected", result)
	return result, nil
}

// PromptConfigPath prompts for the config file path.
func PromptConfigPath(recommended string) (string, error) {
	var configPath string
	err := survey.AskOne(&survey.Input{
		Message: "Config file path:",
		Default: recommended,
	}, &configPath)
	if err != nil {
		return "", fmt.Errorf("config path: %w", err)
	}

	if configPath == "" {
		configPath = recommended
	}

	return configPath, nil
}

// PromptRetryConnection prompts the user if they want to retry a failed connection.
func PromptRetryConnection(dbIndex int) bool {
	var retry bool
	err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("Retry database %d configuration?", dbIndex),
		Default: true,
	}, &retry)
	if err != nil {
		return false
	}
	return retry
}
