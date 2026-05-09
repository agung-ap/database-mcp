package setup

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// PromptDatabase prompts the user for a database configuration.
func PromptDatabase(index int) (*DatabaseConfig, error) {
	fmt.Printf("\n📦 Database %d Configuration\n", index+1)
	fmt.Println("═══════════════════════════════════════════")

	reader := bufio.NewReader(os.Stdin)

	// Prompt for name
	name, err := promptInput(reader, "Database name (alphanumeric, underscore):", fmt.Sprintf("db%d", index+1))
	if err != nil {
		return nil, fmt.Errorf("database name: %w", err)
	}

	// Validate name
	if err := validateDatabaseName(name); err != nil {
		return nil, err
	}

	// Prompt for driver
	driver, err := promptSelect(reader, "Database Engine:", []string{"postgres", "mysql", "sqlite"}, "postgres")
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
		if err := promptSQLDatabase(reader, cfg, driver); err != nil {
			return nil, err
		}
	case "sqlite":
		if err := promptSQLiteDatabase(reader, cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// promptSQLDatabase prompts for SQL database (PostgreSQL/MySQL) configuration.
func promptSQLDatabase(reader *bufio.Reader, cfg *DatabaseConfig, driver string) error {
	// Host
	host, err := promptInput(reader, "Host:", "localhost")
	if err != nil {
		return fmt.Errorf("host: %w", err)
	}
	cfg.Host = host

	// Port with default based on driver
	defaultPort := 5432
	if driver == "mysql" {
		defaultPort = 3306
	}
	portStr, err := promptInput(reader, "Port:", fmt.Sprintf("%d", defaultPort))
	if err != nil {
		return fmt.Errorf("port: %w", err)
	}

	if portStr == "" {
		cfg.Port = defaultPort
	} else {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port number: %w", err)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("port must be between 1-65535")
		}
		cfg.Port = port
	}

	// Username
	username, err := promptInput(reader, "Username:", "")
	if err != nil {
		return fmt.Errorf("username: %w", err)
	}

	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	cfg.Username = username

	// Password (hidden)
	password, err := promptPassword(reader, "Password (leave empty for no password):")
	if err != nil {
		return fmt.Errorf("password: %w", err)
	}
	cfg.Password = password

	// Database
	database, err := promptInput(reader, "Database name:", "")
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}

	if database == "" {
		return fmt.Errorf("database name cannot be empty")
	}
	cfg.Database = database

	// SSL Mode (PostgreSQL only)
	if driver == "postgres" {
		sslMode, err := promptSelect(reader, "SSL Mode:", []string{"disable", "allow", "prefer", "require"}, "prefer")
		if err != nil {
			return fmt.Errorf("ssl mode: %w", err)
		}
		cfg.SSLMode = sslMode
	} else {
		// MySQL
		sslMode, err := promptSelect(reader, "SSL Mode:", []string{"disabled", "preferred", "required"}, "preferred")
		if err != nil {
			return fmt.Errorf("ssl mode: %w", err)
		}
		cfg.SSLMode = sslMode
	}

	return nil
}

// promptSQLiteDatabase prompts for SQLite database configuration.
func promptSQLiteDatabase(reader *bufio.Reader, cfg *DatabaseConfig) error {
	filePath, err := promptInput(reader, "Database file path:", "./db.sqlite3")
	if err != nil {
		return fmt.Errorf("file path: %w", err)
	}

	if filePath == "" {
		filePath = "./db.sqlite3"
	}
	cfg.FilePath = filePath

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
	reader := bufio.NewReader(os.Stdin)
	answer, err := promptConfirm(reader, "Add another database?", false)
	return answer, err
}

// PromptAgentSelection prompts for agent registration preferences.
func PromptAgentSelection() (map[string]bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\nRegister with agents:")
	agents := []string{
		"Claude Desktop",
		"Claude Code",
		"Cursor",
	}

	selected := make(map[string]bool)
	defaults := map[string]bool{
		"Claude Desktop": true,
		"Claude Code":    true,
		"Cursor":         false,
	}

	for _, agent := range agents {
		answer, err := promptConfirm(reader, fmt.Sprintf("  %s? [y/n]", agent), defaults[agent])
		if err != nil {
			return nil, err
		}
		selected[agent] = answer
	}

	result := map[string]bool{
		"claude-desktop": false,
		"claude-code":    false,
		"cursor":         false,
	}

	for agent, isSelected := range selected {
		switch agent {
		case "Claude Desktop":
			result["claude-desktop"] = isSelected
		case "Claude Code":
			result["claude-code"] = isSelected
		case "Cursor":
			result["cursor"] = isSelected
		}
	}

	slog.Debug("agent selection", "selected", result)
	return result, nil
}

// PromptConfigPath prompts for the config file path.
func PromptConfigPath(recommended string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	configPath, err := promptInput(reader, "Config file path:", recommended)
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
	reader := bufio.NewReader(os.Stdin)
	answer, err := promptConfirm(reader, fmt.Sprintf("Retry database %d configuration?", dbIndex), true)
	if err != nil {
		return false
	}
	return answer
}

// Helper functions for prompting

// promptInput prompts for a text input with optional default.
func promptInput(reader *bufio.Reader, prompt string, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	text, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	text = strings.TrimSpace(text)
	if text == "" {
		text = defaultVal
	}

	return text, nil
}

// promptPassword prompts for a password input (echoed hidden).
func promptPassword(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt + ": ")

	// Get terminal file descriptor
	fd := int(syscall.Stdin)
	state, err := term.GetState(fd)
	if err != nil {
		// Fall back to regular input if terminal handling fails
		input, _ := reader.ReadString('\n')
		return strings.TrimSpace(input), nil
	}

	// Make raw mode to hide input
	_, _ = term.MakeRaw(fd)
	defer func() {
		_ = term.Restore(fd, state)
	}()

	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	fmt.Println() // New line after password input
	return strings.TrimSpace(password), nil
}

// promptSelect prompts for selection from options.
func promptSelect(reader *bufio.Reader, prompt string, options []string, defaultVal string) (string, error) {
	fmt.Println(prompt)
	for i, option := range options {
		marker := " "
		if option == defaultVal {
			marker = ">"
		}
		fmt.Printf(" %s [%d] %s\n", marker, i+1, option)
	}

	for {
		fmt.Print("Select [1]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			return defaultVal, nil
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(options) {
			fmt.Println("Invalid selection. Please try again.")
			continue
		}

		return options[idx-1], nil
	}
}

// promptConfirm prompts for a yes/no confirmation.
func promptConfirm(reader *bufio.Reader, prompt string, defaultVal bool) (bool, error) {
	defaultStr := "y"
	if !defaultVal {
		defaultStr = "n"
	}

	for {
		fmt.Printf("%s [%s/n]: ", prompt, defaultStr)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		input = strings.TrimSpace(strings.ToLower(input))
		if input == "" {
			return defaultVal, nil
		}

		if input == "y" || input == "yes" {
			return true, nil
		}
		if input == "n" || input == "no" {
			return false, nil
		}

		fmt.Println("Please enter 'y' or 'n'.")
	}
}
