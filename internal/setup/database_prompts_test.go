package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDatabaseName_Valid(t *testing.T) {
	tests := []string{
		"db1",
		"my_database",
		"MyDatabase",
		"db_2024",
		"test_DB_123",
		"a",
		"Z",
		"0",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateDatabaseName(name)
			assert.NoError(t, err, "name %q should be valid", name)
		})
	}
}

func TestValidateDatabaseName_Invalid(t *testing.T) {
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
			errMsg:    "database name must be alphanumeric or underscore only",
		},
		{
			name:      "special character space",
			nameInput: "my database",
			errMsg:    "database name must be alphanumeric or underscore only",
		},
		{
			name:      "special character dot",
			nameInput: "my.database",
			errMsg:    "database name must be alphanumeric or underscore only",
		},
		{
			name:      "special character at",
			nameInput: "my@database",
			errMsg:    "database name must be alphanumeric or underscore only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDatabaseName(tt.nameInput)
			require.Error(t, err)
			assert.Equal(t, tt.errMsg, err.Error())
		})
	}
}

func TestValidateDatabaseName_Boundary50Chars(t *testing.T) {
	// 50 chars should be valid
	name50 := "12345678901234567890123456789012345678901234567890"
	err := validateDatabaseName(name50)
	assert.NoError(t, err)
	assert.Len(t, name50, 50)

	// 51 chars should fail
	name51 := "123456789012345678901234567890123456789012345678901"
	err = validateDatabaseName(name51)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot exceed 50 characters")
}
