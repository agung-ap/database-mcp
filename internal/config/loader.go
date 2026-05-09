package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Connection represents a single database connection config.
type Connection struct {
	Name               string `json:"name"`
	Engine             string `json:"engine"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	Database           string `json:"database"`
	SSLMode            string `json:"ssl_mode"`
	MaxConnections     int    `json:"max_connections"`
	MinConnections     int    `json:"min_connections"`
	ConnectTimeoutSecs int    `json:"connect_timeout_secs"`
	// SQLite-specific
	FilePath string `json:"file_path"`
}

// Config is the top-level server config.
type Config struct {
	Version           string       `json:"version"`
	DefaultConnection string       `json:"default_connection"`
	Databases         []Connection `json:"databases"`
}

// Loader handles multi-path configuration loading with fallback chain.
type Loader struct {
	searchPaths []string
	usedPath    string
}

// NewLoader creates a new configuration loader with fallback paths.
func NewLoader() *Loader {
	return &Loader{
		searchPaths: getDefaultSearchPaths(),
	}
}

// getDefaultSearchPaths returns the default configuration search paths in priority order.
// 1. $MCP_DB_CONFIG_PATH (environment override - highest priority)
// 2. .databases.json (project-local)
// 3. ~/.config/db-mcp/.databases.json (user home)
// 4. ./config/connections.json (legacy fallback)
func getDefaultSearchPaths() []string {
	paths := []string{}

	// 1. Environment override (highest priority)
	if envPath := os.Getenv("MCP_DB_CONFIG_PATH"); envPath != "" {
		paths = append(paths, envPath)
	}

	// 2. Project-local path
	paths = append(paths, ".databases.json")

	// 3. User home path
	if homeDir, err := os.UserHomeDir(); err == nil {
		configDir := filepath.Join(homeDir, ".config", "db-mcp")
		paths = append(paths, filepath.Join(configDir, ".databases.json"))
	}

	// 4. Legacy fallback
	paths = append(paths, "./config/connections.json")

	return paths
}

// Load reads configuration from the default search paths (with fallback chain).
// Returns the config, the path that was used, and any error.
func (l *Loader) Load() (*Config, string, error) {
	for _, path := range l.searchPaths {
		expandedPath := expandHome(path)
		if !fileExists(expandedPath) {
			continue
		}

		cfg, err := l.loadFromPath(expandedPath)
		if err != nil {
			// Log but continue to next path
			slog.Debug("config: failed to load from path, trying next", "path", expandedPath, "error", err)
			continue
		}

		l.usedPath = expandedPath
		slog.Debug("config: loaded from", "path", expandedPath)
		return cfg, expandedPath, nil
	}

	return nil, "", fmt.Errorf("config: no configuration found in any search path (searched: %v)", l.searchPaths)
}

// LoadFromPath reads configuration from a specific file path.
// Returns the config, the path used, and any error.
func LoadFromPath(path string) (*Config, string, error) {
	expandedPath := expandHome(path)
	loader := &Loader{
		searchPaths: []string{expandedPath},
	}
	cfg, err := loader.loadFromPath(expandedPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, expandedPath, nil
}

// loadFromPath is the internal method that loads from a specific path.
func (l *Loader) loadFromPath(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	// Expand environment variables in content
	contentStr := expandEnvVars(string(content))

	var cfg Config
	if err := json.Unmarshal([]byte(contentStr), &cfg); err != nil {
		return nil, fmt.Errorf("config: parse JSON from %q: %w", path, err)
	}

	return &cfg, nil
}

// UsedPath returns the path that was actually loaded (after Load was called).
func (l *Loader) UsedPath() string {
	return l.usedPath
}

// Load is a convenience function for backward compatibility.
// It loads from MCP_DB_CONFIG_PATH or defaults to ./config/connections.json.
func Load(path string) (*Config, error) {
	cfg, _, err := LoadFromPath(path)
	return cfg, err
}

// expandHome expands ~ to the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

// expandEnvVars replaces ${VAR_NAME} with environment variable values.
func expandEnvVars(s string) string {
	return os.ExpandEnv(s)
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// BuildDSN constructs a DSN from the connection fields.
func (c *Connection) BuildDSN() string {
	switch c.Engine {
	case "postgres":
		sslMode := c.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
			c.Host, c.Port, c.Database, c.Username, c.Password, sslMode)
		if c.ConnectTimeoutSecs > 0 {
			dsn += fmt.Sprintf(" connect_timeout=%d", c.ConnectTimeoutSecs)
		}
		return dsn
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			c.Username, c.Password, c.Host, c.Port, c.Database)
	default:
		return ""
	}
}
