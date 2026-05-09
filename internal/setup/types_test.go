package setup

import (
	"testing"

	"github.com/agp/db-mcp/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseConfig_ToConnectionConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *DatabaseConfig
		expected *config.Connection
	}{
		{
			name: "PostgreSQL configuration",
			input: &DatabaseConfig{
				Name:     "prod-pg",
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Username: "admin",
				Password: "secret",
				Database: "mydb",
				SSLMode:  "require",
			},
			expected: &config.Connection{
				Name:                "prod-pg",
				Engine:              "postgres",
				Host:                "localhost",
				Port:                5432,
				Username:            "admin",
				Password:            "secret",
				Database:            "mydb",
				SSLMode:             "require",
				MaxConnections:      10,
				MinConnections:      5,
				ConnectTimeoutSecs:  10,
			},
		},
		{
			name: "MySQL configuration",
			input: &DatabaseConfig{
				Name:     "staging-mysql",
				Driver:   "mysql",
				Host:     "db.example.com",
				Port:     3306,
				Username: "user",
				Password: "pass",
				Database: "app_db",
				SSLMode:  "preferred",
			},
			expected: &config.Connection{
				Name:                "staging-mysql",
				Engine:              "mysql",
				Host:                "db.example.com",
				Port:                3306,
				Username:            "user",
				Password:            "pass",
				Database:            "app_db",
				SSLMode:             "preferred",
				MaxConnections:      10,
				MinConnections:      5,
				ConnectTimeoutSecs:  10,
			},
		},
		{
			name: "SQLite configuration",
			input: &DatabaseConfig{
				Name:     "local-sqlite",
				Driver:   "sqlite",
				FilePath: "/var/lib/myapp/data.sqlite3",
			},
			expected: &config.Connection{
				Name:                "local-sqlite",
				Engine:              "sqlite",
				FilePath:            "/var/lib/myapp/data.sqlite3",
				MaxConnections:      10,
				MinConnections:      5,
				ConnectTimeoutSecs:  10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToConnectionConfig()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetupResult_Structure(t *testing.T) {
	result := &SetupResult{
		ConfigPath:     "/home/user/.config/db-mcp/.databases.json",
		DatabasesAdded: 2,
		ConfigWritten:  true,
		AgentsRegistered: map[string]bool{
			"claude-desktop": true,
			"claude-code":    false,
		},
		SkillGenerated: false,
		Errors:         []string{},
	}

	assert.Equal(t, 2, result.DatabasesAdded)
	assert.True(t, result.ConfigWritten)
	assert.True(t, result.AgentsRegistered["claude-desktop"])
	assert.False(t, result.AgentsRegistered["claude-code"])
	assert.Equal(t, 0, len(result.Errors))
}

func TestSetupOptions_Defaults(t *testing.T) {
	opts := &SetupOptions{
		ConfigPath:            "/custom/path",
		SkipConnectionTest:    false,
		SkipAgentRegistration: false,
	}

	assert.Equal(t, "/custom/path", opts.ConfigPath)
	assert.False(t, opts.SkipConnectionTest)
	assert.False(t, opts.SkipAgentRegistration)
}
