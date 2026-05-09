package setup

import (
	"fmt"
	"net"
	"strconv"
)

// ValidateConnection validates a database connection configuration.
func ValidateConnection(cfg *DatabaseConfig) error {
	if err := validateName(cfg.Name); err != nil {
		return err
	}

	switch cfg.Driver {
	case "postgres", "mysql":
		if err := validateSQLConnection(cfg); err != nil {
			return err
		}
	case "sqlite":
		if err := validateSQLiteConnection(cfg); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}

	return nil
}

// validateName validates a database name.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	if len(name) > 50 {
		return fmt.Errorf("database name cannot exceed 50 characters")
	}

	for _, ch := range name {
		isLower := ch >= 'a' && ch <= 'z'
		isUpper := ch >= 'A' && ch <= 'Z'
		isDigit := ch >= '0' && ch <= '9'
		isUnderscore := ch == '_'
		if !isLower && !isUpper && !isDigit && !isUnderscore {
			return fmt.Errorf("database name must contain only alphanumeric characters and underscore")
		}
	}

	return nil
}

// validateSQLConnection validates SQL database connection config.
func validateSQLConnection(cfg *DatabaseConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1-65535")
	}

	if cfg.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if cfg.Database == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	return nil
}

// validateSQLiteConnection validates SQLite connection config.
func validateSQLiteConnection(cfg *DatabaseConfig) error {
	if cfg.FilePath == "" {
		return fmt.Errorf("SQLite file path cannot be empty")
	}

	return nil
}

// IsValidHostPort checks if a host:port combination is valid.
func IsValidHostPort(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	_, err := net.ResolveTCPAddr("tcp", addr)
	return err == nil
}

// IsValidPort checks if a port number is valid.
func IsValidPort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %w", err)
	}

	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1-65535")
	}

	return port, nil
}
