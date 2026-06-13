package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "host=localhost port=5432 dbname=myapp user=admin password=secret sslmode=disable", c.BuildDSN())
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
	assert.Equal(t, "admin:secret@tcp(localhost:3306)/myapp", c.BuildDSN())
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
