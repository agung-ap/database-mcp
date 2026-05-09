package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agp/db-mcp/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWriter(t *testing.T) {
	writer := NewConfigWriter("/tmp/config.json")
	assert.NotNil(t, writer)
	assert.Equal(t, "/tmp/config.json", writer.configPath)
}

func TestNewConfigWriter_ExpandHome(t *testing.T) {
	writer := NewConfigWriter("~/.config/db-mcp/.databases.json")
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".config/db-mcp/.databases.json")
	assert.Equal(t, expectedPath, writer.configPath)
}

func TestConfigWriter_Write_SingleDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	writer := NewConfigWriter(configPath)
	databases := []DatabaseConfig{
		{
			Name:     "test-db",
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "admin",
			Password: "secret",
			Database: "testdb",
			SSLMode:  "require",
		},
	}

	err := writer.Write(databases)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, configPath)

	// Verify content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = json.Unmarshal(content, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, "test-db", cfg.DefaultConnection)
	assert.Len(t, cfg.Databases, 1)
	assert.Equal(t, "test-db", cfg.Databases[0].Name)
	assert.Equal(t, "postgres", cfg.Databases[0].Engine)
}

func TestConfigWriter_Write_MultipleDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	writer := NewConfigWriter(configPath)
	databases := []DatabaseConfig{
		{
			Name:     "prod-db",
			Driver:   "postgres",
			Host:     "prod.example.com",
			Port:     5432,
			Username: "user",
			Database: "proddb",
		},
		{
			Name:     "staging-db",
			Driver:   "mysql",
			Host:     "staging.example.com",
			Port:     3306,
			Username: "user",
			Database: "stagingdb",
		},
	}

	err := writer.Write(databases)
	require.NoError(t, err)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = json.Unmarshal(content, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "prod-db", cfg.DefaultConnection)
	assert.Len(t, cfg.Databases, 2)
}

func TestConfigWriter_Write_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested", "dir", "config.json")

	writer := NewConfigWriter(configPath)
	databases := []DatabaseConfig{
		{
			Name:     "test",
			Driver:   "sqlite",
			FilePath: ":memory:",
		},
	}

	err := writer.Write(databases)
	require.NoError(t, err)

	assert.FileExists(t, configPath)
}

func TestConfigWriter_Write_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	writer := NewConfigWriter(configPath)
	databases := []DatabaseConfig{
		{
			Name:     "test",
			Driver:   "sqlite",
			FilePath: "./test.db",
		},
	}

	err := writer.Write(databases)
	require.NoError(t, err)

	info, err := os.Stat(configPath)
	require.NoError(t, err)

	// File permissions should be 0o600 (user read-write only)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestConfigWriter_Write_BackupExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create initial config file
	err := os.WriteFile(configPath, []byte("old config"), 0o644)
	require.NoError(t, err)

	// Write new config
	writer := NewConfigWriter(configPath)
	databases := []DatabaseConfig{
		{
			Name:     "test",
			Driver:   "postgres",
			Port:     5432,
			Host:     "localhost",
			Username: "user",
			Database: "test",
		},
	}

	err = writer.Write(databases)
	require.NoError(t, err)

	// Check that backup was created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	backupFound := false
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".bak" {
			backupFound = true
			break
		}
	}
	assert.True(t, backupFound, "backup file should be created")
}

func TestExpandHome_WithTilde(t *testing.T) {
	path := expandHome("~/.config/test.json")
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".config/test.json")
	assert.Equal(t, expectedPath, path)
}

func TestExpandHome_WithoutTilde(t *testing.T) {
	path := expandHome("/etc/config.json")
	assert.Equal(t, "/etc/config.json", path)
}

func TestExpandHome_Empty(t *testing.T) {
	path := expandHome("")
	assert.Equal(t, "", path)
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// File doesn't exist yet
	assert.False(t, fileExists(testFile))

	// Create file
	err := os.WriteFile(testFile, []byte("test"), 0o644)
	require.NoError(t, err)

	// File exists now
	assert.True(t, fileExists(testFile))
}

func TestConfigWriter_SQLiteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	writer := NewConfigWriter(configPath)
	databases := []DatabaseConfig{
		{
			Name:     "sqlite-local",
			Driver:   "sqlite",
			FilePath: "./myapp.db",
		},
	}

	err := writer.Write(databases)
	require.NoError(t, err)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = json.Unmarshal(content, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "sqlite", cfg.Databases[0].Engine)
	assert.Equal(t, "./myapp.db", cfg.Databases[0].FilePath)
}

func TestConfigWriter_EmptyDatabases(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	writer := NewConfigWriter(configPath)
	databases := []DatabaseConfig{}

	err := writer.Write(databases)
	require.NoError(t, err)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = json.Unmarshal(content, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "", cfg.DefaultConnection)
	assert.Len(t, cfg.Databases, 0)
}
