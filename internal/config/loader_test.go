package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandHome(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with tilde",
			input:    "~/.config/db-mcp/config.json",
			expected: filepath.Join(homeDir, ".config/db-mcp/config.json"),
		},
		{
			name:     "without tilde",
			input:    "/etc/db-mcp/config.yaml",
			expected: "/etc/db-mcp/config.yaml",
		},
		{
			name:     "relative path",
			input:    ".databases.json",
			expected: ".databases.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandHome(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_PASSWORD", "secret123")
	defer os.Unsetenv("TEST_PASSWORD")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with env var",
			input:    "password: ${TEST_PASSWORD}",
			expected: "password: secret123",
		},
		{
			name:     "without env var",
			input:    "password: hardcoded",
			expected: "password: hardcoded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnvVars(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_config*.yaml")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing file",
			path:     tmpFile.Name(),
			expected: true,
		},
		{
			name:     "non-existing file",
			path:     "/nonexistent/path/to/file.yaml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileExists(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadFromPathYAML(t *testing.T) {
	yamlContent := `version: "1.0"
default_connection: "test_db"
databases:
  - name: "test_db"
    engine: "postgres"
    host: "localhost"
    port: 5432
    username: "testuser"
    password: "testpass"
    database: "testdb"
    ssl_mode: "disable"
`

	tmpFile, err := os.CreateTemp("", "test_config*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	err = os.WriteFile(tmpFile.Name(), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	cfg, usedPath, err := LoadFromPath(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, tmpFile.Name(), usedPath)
	assert.NotNil(t, cfg)
	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, "test_db", cfg.DefaultConnection)
	assert.Len(t, cfg.Databases, 1)
	assert.Equal(t, "test_db", cfg.Databases[0].Name)
	assert.Equal(t, "postgres", cfg.Databases[0].Engine)
	assert.Equal(t, "localhost", cfg.Databases[0].Host)
	assert.Equal(t, 5432, cfg.Databases[0].Port)
}

func TestLoadFromPathJSON(t *testing.T) {
	jsonContent := `{
  "version": "1.0",
  "default_connection": "test_db",
  "databases": [
    {
      "name": "test_db",
      "engine": "mysql",
      "host": "db.example.com",
      "port": 3306,
      "username": "testuser",
      "password": "testpass",
      "database": "testdb",
      "ssl_mode": "require"
    }
  ]
}`

	tmpFile, err := os.CreateTemp("", "test_config*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	err = os.WriteFile(tmpFile.Name(), []byte(jsonContent), 0o644)
	require.NoError(t, err)

	cfg, usedPath, err := LoadFromPath(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, tmpFile.Name(), usedPath)
	assert.NotNil(t, cfg)
	assert.Equal(t, "1.0", cfg.Version)
	assert.Len(t, cfg.Databases, 1)
	assert.Equal(t, "mysql", cfg.Databases[0].Engine)
	assert.Equal(t, 3306, cfg.Databases[0].Port)
}

func TestLoadFromPathEnvVarSubstitution(t *testing.T) {
	os.Setenv("DB_PASSWORD", "secret_password")
	os.Setenv("DB_USERNAME", "admin")
	defer os.Unsetenv("DB_PASSWORD")
	defer os.Unsetenv("DB_USERNAME")

	yamlContent := `version: "1.0"
databases:
  - name: "test_db"
    engine: "postgres"
    host: "localhost"
    port: 5432
    username: "${DB_USERNAME}"
    password: "${DB_PASSWORD}"
    database: "testdb"
`

	tmpFile, err := os.CreateTemp("", "test_config*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	err = os.WriteFile(tmpFile.Name(), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	cfg, _, err := LoadFromPath(tmpFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "admin", cfg.Databases[0].Username)
	assert.Equal(t, "secret_password", cfg.Databases[0].Password)
}

func TestLoadFromPathMissingFile(t *testing.T) {
	cfg, _, err := LoadFromPath("/nonexistent/path/to/file.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config: read")
}

func TestBackwardCompatibilityLoad(t *testing.T) {
	yamlContent := `version: "1.0"
default_connection: "test_db"
databases:
  - name: "test_db"
    engine: "postgres"
    host: "localhost"
    port: 5432
    username: "testuser"
    password: "testpass"
    database: "testdb"
`

	tmpFile, err := os.CreateTemp("", "test_config*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	err = os.WriteFile(tmpFile.Name(), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	cfg, err := Load(tmpFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "1.0", cfg.Version)
	assert.Len(t, cfg.Databases, 1)
}

func TestConnectionBuildDSNPostgres(t *testing.T) {
	conn := &Connection{
		Engine:             "postgres",
		Host:               "localhost",
		Port:               5432,
		Username:           "user",
		Password:           "pass",
		Database:           "mydb",
		SSLMode:            "require",
		ConnectTimeoutSecs: 10,
	}

	dsn := conn.BuildDSN()
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=user")
	assert.Contains(t, dsn, "password=pass")
	assert.Contains(t, dsn, "dbname=mydb")
	assert.Contains(t, dsn, "sslmode=require")
	assert.Contains(t, dsn, "connect_timeout=10")
}

func TestConnectionBuildDSNMySQL(t *testing.T) {
	conn := &Connection{
		Engine:   "mysql",
		Host:     "db.example.com",
		Port:     3306,
		Username: "admin",
		Password: "secret",
		Database: "appdb",
	}

	dsn := conn.BuildDSN()
	assert.Equal(t, "admin:secret@tcp(db.example.com:3306)/appdb", dsn)
}
