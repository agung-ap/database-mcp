package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateName_Valid(t *testing.T) {
	tests := []string{
		"db1",
		"my_database",
		"MyDatabase",
		"db_2024",
		"prod_pg_v2",
		"a",
		"Z",
		"0",
		"test_db_123_xyz",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateName(name)
			assert.NoError(t, err, "name %q should be valid", name)
		})
	}
}

func TestValidateName_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		nameInput string
		errMsg    string
	}{
		{
			name:      "empty name",
			nameInput: "",
			errMsg:    "database name cannot be empty",
		},
		{
			name:      "exceeds 50 chars",
			nameInput: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz",
			errMsg:    "database name cannot exceed 50 characters",
		},
		{
			name:      "special character dash",
			nameInput: "my-database",
			errMsg:    "database name must contain only alphanumeric characters and underscore",
		},
		{
			name:      "special character space",
			nameInput: "my database",
			errMsg:    "database name must contain only alphanumeric characters and underscore",
		},
		{
			name:      "special character dot",
			nameInput: "my.database",
			errMsg:    "database name must contain only alphanumeric characters and underscore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.nameInput)
			require.Error(t, err)
			assert.Equal(t, tt.errMsg, err.Error())
		})
	}
}

func TestValidateSQLConnection_Valid(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "admin",
		Database: "mydb",
	}

	err := validateSQLConnection(cfg)
	assert.NoError(t, err)
}

func TestValidateSQLConnection_InvalidHost(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "",
		Port:     5432,
		Username: "admin",
		Database: "mydb",
	}

	err := validateSQLConnection(cfg)
	require.Error(t, err)
	assert.Equal(t, "host cannot be empty", err.Error())
}

func TestValidateSQLConnection_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"above max", 65536},
		{"high above max", 99999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &DatabaseConfig{
				Host:     "localhost",
				Port:     tt.port,
				Username: "admin",
				Database: "mydb",
			}

			err := validateSQLConnection(cfg)
			require.Error(t, err)
			assert.Equal(t, "port must be between 1-65535", err.Error())
		})
	}
}

func TestValidateSQLConnection_InvalidUsername(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "",
		Database: "mydb",
	}

	err := validateSQLConnection(cfg)
	require.Error(t, err)
	assert.Equal(t, "username cannot be empty", err.Error())
}

func TestValidateSQLConnection_InvalidDatabase(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "admin",
		Database: "",
	}

	err := validateSQLConnection(cfg)
	require.Error(t, err)
	assert.Equal(t, "database name cannot be empty", err.Error())
}

func TestValidateSQLiteConnection_Valid(t *testing.T) {
	cfg := &DatabaseConfig{
		FilePath: "/var/lib/db.sqlite3",
	}

	err := validateSQLiteConnection(cfg)
	assert.NoError(t, err)
}

func TestValidateSQLiteConnection_InvalidFilePath(t *testing.T) {
	cfg := &DatabaseConfig{
		FilePath: "",
	}

	err := validateSQLiteConnection(cfg)
	require.Error(t, err)
	assert.Equal(t, "SQLite file path cannot be empty", err.Error())
}

func TestValidateConnection_PostgreSQL(t *testing.T) {
	cfg := &DatabaseConfig{
		Name:     "prod_pg",
		Driver:   "postgres",
		Host:     "localhost",
		Port:     5432,
		Username: "admin",
		Database: "mydb",
	}

	err := ValidateConnection(cfg)
	assert.NoError(t, err)
}

func TestValidateConnection_MySQL(t *testing.T) {
	cfg := &DatabaseConfig{
		Name:     "staging_mysql",
		Driver:   "mysql",
		Host:     "db.example.com",
		Port:     3306,
		Username: "user",
		Database: "app_db",
	}

	err := ValidateConnection(cfg)
	assert.NoError(t, err)
}

func TestValidateConnection_SQLite(t *testing.T) {
	cfg := &DatabaseConfig{
		Name:     "local_sqlite",
		Driver:   "sqlite",
		FilePath: "./db.sqlite3",
	}

	err := ValidateConnection(cfg)
	assert.NoError(t, err)
}

func TestValidateConnection_UnsupportedDriver(t *testing.T) {
	cfg := &DatabaseConfig{
		Name:   "invalid",
		Driver: "oracle",
	}

	err := ValidateConnection(cfg)
	require.Error(t, err)
	assert.Equal(t, "unsupported driver: oracle", err.Error())
}

func TestValidateConnection_InvalidName(t *testing.T) {
	cfg := &DatabaseConfig{
		Name:   "my-db",
		Driver: "postgres",
	}

	err := ValidateConnection(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database name must contain only alphanumeric characters and underscore")
}

func TestIsValidPort_Valid(t *testing.T) {
	tests := []struct {
		portStr string
		want    int
	}{
		{"5432", 5432},
		{"3306", 3306},
		{"1", 1},
		{"65535", 65535},
	}

	for _, tt := range tests {
		t.Run(tt.portStr, func(t *testing.T) {
			port, err := IsValidPort(tt.portStr)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, port)
		})
	}
}

func TestIsValidPort_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		portStr string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"above max", "65536"},
		{"float", "54.32"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := IsValidPort(tt.portStr)
			assert.Error(t, err)
		})
	}
}

func TestIsValidHostPort(t *testing.T) {
	// localhost should always be valid
	result := IsValidHostPort("localhost", 5432)
	assert.True(t, result)

	// 127.0.0.1 should be valid
	result = IsValidHostPort("127.0.0.1", 3306)
	assert.True(t, result)

	// Invalid port should be invalid
	result = IsValidHostPort("localhost", 99999)
	assert.False(t, result)
}
