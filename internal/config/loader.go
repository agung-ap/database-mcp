package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/agung-ap/database-mcp/internal/secrets"
)

// keyringGet is a package-level indirection over secrets.Get so tests can
// stub out the real OS credential store.
var keyringGet = secrets.Get

var supportedDrivers = map[string]bool{
	"postgres":  true,
	"mysql":     true,
	"sqlserver": true,
}

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
	ReadOnly bool       `json:"read_only"`
	Pool     PoolConfig `json:"pool"`
	// PasswordSource selects where the password comes from. Empty (default)
	// means use the Password field directly. "keyring" means look the
	// password up in the OS credential store (see internal/secrets), keyed
	// by this connection's ID; Password should be left empty in that case.
	PasswordSource string `json:"password_source,omitempty"`
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

	// Resolve each connection's password/DSN. Priority order:
	//   1. MCP_DB_CONN_<UPPER_ID>_DSN env var (full DSN override, wins outright)
	//   2. password_source: "keyring" (OS credential store lookup)
	//   3. the plaintext password field (kept for backward compatibility)
	for i := range cfg.Connections {
		conn := &cfg.Connections[i]

		envKey := fmt.Sprintf("MCP_DB_CONN_%s_DSN",
			strings.ToUpper(strings.ReplaceAll(conn.ID, "-", "_")))
		if dsn := os.Getenv(envKey); dsn != "" {
			conn.DSN = dsn
			continue
		}

		switch conn.PasswordSource {
		case "keyring":
			pw, err := keyringGet(conn.ID)
			if err != nil {
				return nil, fmt.Errorf("config: connection %q: %w", conn.ID, err)
			}
			conn.Password = pw
		case "":
			if conn.Password != "" {
				slog.Warn("connection stores its password in plaintext; consider migrating to the OS credential store",
					"connection_id", conn.ID, "hint", fmt.Sprintf("database-mcp secret set %s", conn.ID))
			}
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: %q: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks the loaded config for errors that would otherwise surface
// confusingly later (duplicate IDs silently colliding in a map, an unknown
// driver failing only that one connection while others start, etc). All
// problems found are returned together so a user can fix them in one pass.
func (cfg *Config) Validate() error {
	var errs []error
	seen := make(map[string]bool, len(cfg.Connections))

	for i, c := range cfg.Connections {
		if c.ID == "" {
			errs = append(errs, fmt.Errorf("connections[%d]: id is required", i))
			continue
		}
		if seen[c.ID] {
			errs = append(errs, fmt.Errorf("connection %q: duplicate id", c.ID))
		}
		seen[c.ID] = true

		if !supportedDrivers[c.Driver] {
			errs = append(errs, fmt.Errorf("connection %q: unsupported driver %q (want postgres, mysql, or sqlserver)", c.ID, c.Driver))
		}

		if c.DSN == "" {
			if c.Host == "" {
				errs = append(errs, fmt.Errorf("connection %q: host is required (or set a DSN override)", c.ID))
			}
			if c.Database == "" {
				errs = append(errs, fmt.Errorf("connection %q: database is required (or set a DSN override)", c.ID))
			}
		}
	}

	return errors.Join(errs...)
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
		// Built as a URL (rather than keyword/value pairs) so that
		// user/password/database values containing spaces, quotes, or '='
		// can't break out of the DSN or override connection options.
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(c.User, c.Password),
			Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
			Path:   "/" + c.Database,
		}
		q := url.Values{}
		q.Set("sslmode", sslMode)
		u.RawQuery = q.Encode()
		return u.String()
	case "mysql":
		cfg := mysqldriver.NewConfig()
		cfg.User = c.User
		cfg.Passwd = c.Password
		cfg.Net = "tcp"
		cfg.Addr = fmt.Sprintf("%s:%d", c.Host, c.Port)
		cfg.DBName = c.Database
		cfg.ParseTime = true
		switch c.SSLMode {
		case "", "disable":
			// no TLS
		default:
			cfg.TLSConfig = c.SSLMode
		}
		return cfg.FormatDSN()
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
