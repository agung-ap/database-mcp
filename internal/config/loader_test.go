package config

import (
	"errors"
	"net/url"
	"os"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDSN_Postgres(t *testing.T) {
	c := Connection{
		Driver:   "postgres",
		Host:     "localhost",
		Port:     5432,
		Database: "myapp",
		User:     "admin",
		Password: "secret",
	}
	assert.Equal(t, "postgres://admin:secret@localhost:5432/myapp?sslmode=disable", c.BuildDSN())
}

func TestBuildDSN_MySQL(t *testing.T) {
	c := Connection{
		Driver:   "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "myapp",
		User:     "admin",
		Password: "secret",
	}
	assert.Equal(t, "admin:secret@tcp(localhost:3306)/myapp?parseTime=true", c.BuildDSN())
}

func TestBuildDSN_SQLServer(t *testing.T) {
	c := Connection{
		Driver:   "sqlserver",
		Host:     "localhost",
		Port:     1433,
		Database: "myapp",
		User:     "sa",
		Password: "secret",
		SSLMode:  "disable",
	}
	assert.Equal(t, "sqlserver://sa:secret@localhost:1433?database=myapp&encrypt=disable", c.BuildDSN())
}

func TestBuildDSN_Override(t *testing.T) {
	c := Connection{
		Driver: "postgres",
		DSN:    "custom-dsn",
	}
	assert.Equal(t, "custom-dsn", c.BuildDSN())
}

func TestBuildDSN_Postgres_SpecialCharsDontInjectOptions(t *testing.T) {
	c := Connection{
		Driver:   "postgres",
		Host:     "localhost",
		Port:     5432,
		Database: "myapp",
		User:     "admin",
		Password: "p@ss word' sslmode=disable",
	}
	dsn := c.BuildDSN()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	pw, _ := u.User.Password()
	assert.Equal(t, "p@ss word' sslmode=disable", pw, "password must round-trip exactly, not leak into other DSN options")
	assert.Equal(t, "disable", u.Query().Get("sslmode"), "sslmode must remain the configured value, not be overridden by password content")
}

func TestBuildDSN_MySQL_SpecialChars(t *testing.T) {
	c := Connection{
		Driver:   "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "myapp",
		User:     "admin",
		Password: "p@ss:word/?",
	}
	dsn := c.BuildDSN()
	cfg, err := mysqldriver.ParseDSN(dsn)
	require.NoError(t, err)
	assert.Equal(t, "p@ss:word/?", cfg.Passwd)
	assert.Equal(t, "myapp", cfg.DBName)
}

func TestValidate_DuplicateID(t *testing.T) {
	cfg := Config{Connections: []Connection{
		{ID: "a", Driver: "postgres", Host: "h", Database: "d"},
		{ID: "a", Driver: "mysql", Host: "h", Database: "d"},
	}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate id")
}

func TestValidate_UnsupportedDriver(t *testing.T) {
	cfg := Config{Connections: []Connection{
		{ID: "a", Driver: "oracle", Host: "h", Database: "d"},
	}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported driver")
}

func TestValidate_MissingHostAndDatabase(t *testing.T) {
	cfg := Config{Connections: []Connection{
		{ID: "a", Driver: "postgres"},
	}}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
	assert.Contains(t, err.Error(), "database is required")
}

func TestValidate_DSNOverrideSkipsHostDatabaseCheck(t *testing.T) {
	cfg := Config{Connections: []Connection{
		{ID: "a", Driver: "postgres", DSN: "postgres://x"},
	}}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_OK(t *testing.T) {
	cfg := Config{Connections: []Connection{
		{ID: "a", Driver: "postgres", Host: "h", Database: "d"},
		{ID: "b", Driver: "mysql", Host: "h", Database: "d"},
	}}
	assert.NoError(t, cfg.Validate())
}

func TestLoad_KeyringPasswordSource(t *testing.T) {
	orig := keyringGet
	keyringGet = func(connID string) (string, error) {
		assert.Equal(t, "prod-pg", connID)
		return "from-keyring", nil
	}
	t.Cleanup(func() { keyringGet = orig })

	dir := t.TempDir()
	path := dir + "/connections.json"
	require.NoError(t, os.WriteFile(path, []byte(`{
		"connections": [
			{"id": "prod-pg", "driver": "postgres", "host": "h", "database": "d", "password_source": "keyring"}
		]
	}`), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Len(t, cfg.Connections, 1)
	assert.Equal(t, "from-keyring", cfg.Connections[0].Password)
}

func TestLoad_KeyringLookupFailure(t *testing.T) {
	orig := keyringGet
	keyringGet = func(connID string) (string, error) {
		return "", errors.New("no password stored")
	}
	t.Cleanup(func() { keyringGet = orig })

	dir := t.TempDir()
	path := dir + "/connections.json"
	require.NoError(t, os.WriteFile(path, []byte(`{
		"connections": [
			{"id": "prod-pg", "driver": "postgres", "host": "h", "database": "d", "password_source": "keyring"}
		]
	}`), 0o600))

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod-pg")
}
