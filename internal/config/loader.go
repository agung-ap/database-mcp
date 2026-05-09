package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Connection represents a single database connection config.
type Connection struct {
	Name               string `mapstructure:"name"`
	Engine             string `mapstructure:"engine"`
	Host               string `mapstructure:"host"`
	Port               int    `mapstructure:"port"`
	Username           string `mapstructure:"username"`
	Password           string `mapstructure:"password"`
	Database           string `mapstructure:"database"`
	SSLMode            string `mapstructure:"ssl_mode"`
	MaxConnections     int    `mapstructure:"max_connections"`
	MinConnections     int    `mapstructure:"min_connections"`
	ConnectTimeoutSecs int    `mapstructure:"connect_timeout_secs"`
}

// Config is the top-level server config.
type Config struct {
	Version           string       `mapstructure:"version"`
	DefaultConnection string       `mapstructure:"default_connection"`
	Databases         []Connection `mapstructure:"databases"`
}

// Load reads configuration from the given file path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &cfg, nil
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
