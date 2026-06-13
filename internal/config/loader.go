package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PoolConfig holds connection pool configuration.
type PoolConfig struct {
	MaxOpen                int `json:"max_open"`
	MaxIdle                int `json:"max_idle"`
	ConnMaxLifetimeMinutes int `json:"conn_max_lifetime_minutes"`
}

// Connection represents a single database connection config.
type Connection struct {
	ID       string     `json:"id"`
	Driver   string     `json:"driver"`
	Host     string     `json:"host"`
	Port     int        `json:"port"`
	Database string     `json:"database"`
	User     string     `json:"user"`
	Password string     `json:"password"`
	SSLMode  string     `json:"ssl_mode"`
	Pool     PoolConfig `json:"pool"`
	// DSN overrides all individual fields when set (not stored in JSON).
	DSN string `json:"-"`
}

// Config is the top-level server config.
type Config struct {
	Connections []Connection `json:"connections"`
}

// ConfigDir returns the platform-appropriate config directory for database-mcp.
//
//	Windows: %APPDATA%\database-mcp
//	Linux / WSL / macOS: $XDG_CONFIG_HOME/database-mcp or ~/.config/database-mcp
func ConfigDir() string {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "database-mcp")
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "database-mcp")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "database-mcp")
}

// DefaultConfigPath returns the default path for connections.json.
func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), "connections.json")
}

// Load reads configuration from the given JSON file path (and environment overrides).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}

	// Apply DSN overrides from environment: MCP_DB_CONN_<UPPER_ID>_DSN
	for i, conn := range cfg.Connections {
		envKey := fmt.Sprintf("MCP_DB_CONN_%s_DSN",
			strings.ToUpper(strings.ReplaceAll(conn.ID, "-", "_")))
		if dsn := os.Getenv(envKey); dsn != "" {
			cfg.Connections[i].DSN = dsn
		}
	}

	return &cfg, nil
}

// BuildDSN constructs a DSN from the connection fields (if DSN is not already set).
func (c *Connection) BuildDSN() string {
	if c.DSN != "" {
		return c.DSN
	}
	switch c.Driver {
	case "postgres":
		sslMode := c.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
			c.Host, c.Port, c.Database, c.User, c.Password, sslMode)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			c.User, c.Password, c.Host, c.Port, c.Database)
	case "sqlserver":
		u := &url.URL{
			Scheme: "sqlserver",
			User:   url.UserPassword(c.User, c.Password),
			Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		}
		q := url.Values{}
		q.Set("database", c.Database)
		if c.SSLMode != "" {
			q.Set("encrypt", c.SSLMode)
		}
		u.RawQuery = q.Encode()
		return u.String()
	default:
		return ""
	}
}
