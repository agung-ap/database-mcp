package setup

import (
	"github.com/agp/db-mcp/internal/config"
)

// DatabaseConfig represents a single database configuration being set up.
type DatabaseConfig struct {
	Name     string
	Driver   string // "postgres", "mysql", "sqlite"
	Host     string
	Port     int
	Username string
	Password string
	Database string
	SSLMode  string
	FilePath string // for SQLite
}

// SetupOptions contains options for the setup wizard.
type SetupOptions struct {
	ConfigPath            string
	NoRegister            bool
	NonInteractive        bool
	DefaultAnswers        bool
	SkipConnectionTest    bool
	SkipAgentRegistration bool
}

// SetupResult contains the result of a successful setup.
type SetupResult struct {
	ConfigPath       string
	DatabasesAdded   int
	ConfigWritten    bool
	AgentsRegistered map[string]bool // agent name -> success
	SkillGenerated   bool
	Errors           []string
}

// ToConnectionConfig converts DatabaseConfig to config.Connection.
func (dc *DatabaseConfig) ToConnectionConfig() *config.Connection {
	return &config.Connection{
		Name:               dc.Name,
		Engine:             dc.Driver,
		Host:               dc.Host,
		Port:               dc.Port,
		Username:           dc.Username,
		Password:           dc.Password,
		Database:           dc.Database,
		SSLMode:            dc.SSLMode,
		FilePath:           dc.FilePath,
		MaxConnections:     10,
		MinConnections:     5,
		ConnectTimeoutSecs: 10,
	}
}
