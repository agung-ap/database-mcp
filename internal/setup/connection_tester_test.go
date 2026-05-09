package setup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConnectionTester_DefaultTimeout(t *testing.T) {
	tester := NewConnectionTester(0)
	assert.NotNil(t, tester)
	assert.Equal(t, 5*time.Second, tester.timeout)
}

func TestNewConnectionTester_CustomTimeout(t *testing.T) {
	customTimeout := 10 * time.Second
	tester := NewConnectionTester(customTimeout)
	assert.Equal(t, customTimeout, tester.timeout)
}

func TestBuildPostgresDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   *DatabaseConfig
		expected string
	}{
		{
			name: "with ssl mode",
			config: &DatabaseConfig{
				Username: "admin",
				Password: "secret",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				SSLMode:  "require",
			},
			expected: "postgres://admin:secret@localhost:5432/testdb?sslmode=require",
		},
		{
			name: "without ssl mode defaults to disable",
			config: &DatabaseConfig{
				Username: "user",
				Password: "pass",
				Host:     "db.example.com",
				Port:     5432,
				Database: "proddb",
				SSLMode:  "",
			},
			expected: "postgres://user:pass@db.example.com:5432/proddb?sslmode=disable",
		},
		{
			name: "with ssl prefer",
			config: &DatabaseConfig{
				Username: "admin",
				Password: "secret",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				SSLMode:  "prefer",
			},
			expected: "postgres://admin:secret@localhost:5432/testdb?sslmode=prefer",
		},
		{
			name: "with special characters in password",
			config: &DatabaseConfig{
				Username: "admin",
				Password: "p@ssw0rd!",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				SSLMode:  "disable",
			},
			expected: "postgres://admin:p@ssw0rd!@localhost:5432/testdb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildPostgresDSN(tt.config)
			assert.Equal(t, tt.expected, dsn)
		})
	}
}

func TestBuildMySQLDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   *DatabaseConfig
		expected string
	}{
		{
			name: "basic mysql config",
			config: &DatabaseConfig{
				Username: "root",
				Password: "password",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
			},
			expected: "root:password@tcp(localhost:3306)/testdb",
		},
		{
			name: "with remote host",
			config: &DatabaseConfig{
				Username: "user",
				Password: "pass",
				Host:     "db.example.com",
				Port:     3306,
				Database: "appdb",
			},
			expected: "user:pass@tcp(db.example.com:3306)/appdb",
		},
		{
			name: "with custom port",
			config: &DatabaseConfig{
				Username: "admin",
				Password: "secret",
				Host:     "localhost",
				Port:     3307,
				Database: "mydb",
			},
			expected: "admin:secret@tcp(localhost:3307)/mydb",
		},
		{
			name: "empty password",
			config: &DatabaseConfig{
				Username: "user",
				Password: "",
				Host:     "localhost",
				Port:     3306,
				Database: "db",
			},
			expected: "user:@tcp(localhost:3306)/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildMySQLDSN(tt.config)
			assert.Equal(t, tt.expected, dsn)
		})
	}
}

func TestBuildSQLiteDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   *DatabaseConfig
		expected string
	}{
		{
			name: "local file path",
			config: &DatabaseConfig{
				FilePath: "/var/lib/app/data.sqlite3",
			},
			expected: "/var/lib/app/data.sqlite3",
		},
		{
			name: "relative path",
			config: &DatabaseConfig{
				FilePath: "./db.sqlite3",
			},
			expected: "./db.sqlite3",
		},
		{
			name: "in-memory database",
			config: &DatabaseConfig{
				FilePath: ":memory:",
			},
			expected: ":memory:",
		},
		{
			name: "home directory path",
			config: &DatabaseConfig{
				FilePath: "~/.config/myapp/db.sqlite3",
			},
			expected: "~/.config/myapp/db.sqlite3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildSQLiteDSN(tt.config)
			assert.Equal(t, tt.expected, dsn)
		})
	}
}

func TestConnectionTester_UnsupportedDriver(t *testing.T) {
	tester := NewConnectionTester(5 * time.Second)
	cfg := &DatabaseConfig{
		Name:   "invalid",
		Driver: "oracle",
	}

	_, _, err := tester.TestConnection(cfg)
	require.Error(t, err)
	assert.Equal(t, "unsupported driver: oracle", err.Error())
}

func TestConnectionTester_SQLiteMemory(t *testing.T) {
	// SQLite in-memory databases should work if drivers are available
	tester := NewConnectionTester(5 * time.Second)
	cfg := &DatabaseConfig{
		Name:     "test-memory",
		Driver:   "sqlite",
		FilePath: ":memory:",
	}

	success, msg, err := tester.TestConnection(cfg)
	// We don't assert success here because sqlite driver might not be installed
	// Just verify the function doesn't panic
	assert.NotNil(t, msg)
	_ = err
	_ = success
}

func TestConnectionTester_InvalidPostgresConnection(t *testing.T) {
	tester := NewConnectionTester(2 * time.Second)
	cfg := &DatabaseConfig{
		Name:     "invalid-pg",
		Driver:   "postgres",
		Host:     "localhost",
		Port:     9999, // Port that's likely not running
		Username: "user",
		Database: "testdb",
	}

	success, msg, err := tester.TestConnection(cfg)
	// Should fail gracefully
	assert.Nil(t, err)
	assert.False(t, success)
	assert.NotEmpty(t, msg)
}

func TestConnectionTester_InvalidMySQLConnection(t *testing.T) {
	tester := NewConnectionTester(2 * time.Second)
	cfg := &DatabaseConfig{
		Name:     "invalid-mysql",
		Driver:   "mysql",
		Host:     "localhost",
		Port:     9998, // Port that's likely not running
		Username: "user",
		Database: "testdb",
	}

	success, msg, err := tester.TestConnection(cfg)
	// Should fail gracefully
	assert.Nil(t, err)
	assert.False(t, success)
	assert.NotEmpty(t, msg)
}
