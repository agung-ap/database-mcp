package commands

import (
	"testing"

	"github.com/agp/db-mcp/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// TestStatusCommand_Structure verifies the command has correct structure.
func TestStatusCommand_Structure(t *testing.T) {
	cmd := StatusCommand()

	assert.Equal(t, "status", cmd.Name)
	assert.Equal(t, "Check health of all configured database connections", cmd.Usage)
	assert.NotNil(t, cmd.Action)
	assert.Greater(t, len(cmd.Flags), 0, "command should have flags")
}

// TestStatusCommand_Flags verifies all expected flags exist.
func TestStatusCommand_Flags(t *testing.T) {
	cmd := StatusCommand()

	flagNames := make(map[string]bool)
	for _, flag := range cmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.IntFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		case *cli.StringSliceFlag:
			flagNames[f.Name] = true
		}
	}

	expectedFlags := []string{"config", "format", "timeout", "parallel", "only"}
	for _, expected := range expectedFlags {
		assert.True(t, flagNames[expected], "flag %s should exist", expected)
	}
}

// TestStatusCommand_ConfigFlag verifies config flag defaults.
func TestStatusCommand_ConfigFlag(t *testing.T) {
	cmd := StatusCommand()

	var configFlag *cli.StringFlag
	for _, flag := range cmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "config" {
			configFlag = f
			break
		}
	}

	require.NotNil(t, configFlag, "config flag should exist")
	assert.Equal(t, "~/.config/db-mcp/.databases.json", configFlag.Value)
}

// TestStatusCommand_FormatFlag verifies format flag defaults.
func TestStatusCommand_FormatFlag(t *testing.T) {
	cmd := StatusCommand()

	var formatFlag *cli.StringFlag
	for _, flag := range cmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "format" {
			formatFlag = f
			break
		}
	}

	require.NotNil(t, formatFlag, "format flag should exist")
	assert.Equal(t, "table", formatFlag.Value)
}

// TestStatusCommand_TimeoutFlag verifies timeout flag.
func TestStatusCommand_TimeoutFlag(t *testing.T) {
	cmd := StatusCommand()

	var timeoutFlag *cli.IntFlag
	for _, flag := range cmd.Flags {
		if f, ok := flag.(*cli.IntFlag); ok && f.Name == "timeout" {
			timeoutFlag = f
			break
		}
	}

	require.NotNil(t, timeoutFlag, "timeout flag should exist")
	assert.Equal(t, 10, timeoutFlag.Value)
}

// TestStatusCommand_ParallelFlag verifies parallel flag defaults to true.
func TestStatusCommand_ParallelFlag(t *testing.T) {
	cmd := StatusCommand()

	var parallelFlag *cli.BoolFlag
	for _, flag := range cmd.Flags {
		if f, ok := flag.(*cli.BoolFlag); ok && f.Name == "parallel" {
			parallelFlag = f
			break
		}
	}

	require.NotNil(t, parallelFlag, "parallel flag should exist")
	assert.True(t, parallelFlag.Value)
}

// TestFormatTableHeader verifies table header formatting.
func TestFormatTableHeader(t *testing.T) {
	header := formatTableHeader()

	assert.NotEmpty(t, header)
	assert.Contains(t, header, "Connection")
	assert.Contains(t, header, "Driver")
	assert.Contains(t, header, "Status")
	assert.Contains(t, header, "Latency")
	assert.Contains(t, header, "Details")
}

// TestFormatTableRow_Healthy verifies healthy row formatting.
func TestFormatTableRow_Healthy(t *testing.T) {
	result := &tools.HealthCheckResult{
		ConnectionID: "prod-pg",
		Driver:       "postgres",
		Status:       "healthy",
		Latency:      5,
		Details: tools.DetailsInfo{
			ConnectionURL: "postgres://localhost:5432/proddb",
		},
	}

	row := formatTableRow(result)

	assert.NotEmpty(t, row)
	assert.Contains(t, row, "prod-pg")
	assert.Contains(t, row, "postgres")
	assert.Contains(t, row, "healthy")
	assert.Contains(t, row, "5ms")
}

// TestFormatTableRow_Unhealthy verifies unhealthy row formatting.
func TestFormatTableRow_Unhealthy(t *testing.T) {
	result := &tools.HealthCheckResult{
		ConnectionID: "staging-mysql",
		Driver:       "mysql",
		Status:       "unhealthy",
		Latency:      0,
		Error:        "connection refused",
		Details: tools.DetailsInfo{
			Host: "staging-db.example.com",
			Port: 3306,
		},
	}

	row := formatTableRow(result)

	assert.NotEmpty(t, row)
	assert.Contains(t, row, "staging-mysql")
	assert.Contains(t, row, "mysql")
	assert.Contains(t, row, "unhealthy")
}

// TestFormatTableRow_Unknown verifies unknown status row formatting.
func TestFormatTableRow_Unknown(t *testing.T) {
	result := &tools.HealthCheckResult{
		ConnectionID: "test-sqlite",
		Driver:       "sqlite",
		Status:       "unknown",
		Latency:      0,
	}

	row := formatTableRow(result)

	assert.NotEmpty(t, row)
	assert.Contains(t, row, "test-sqlite")
	assert.Contains(t, row, "unknown")
}

// TestTruncateString verifies string truncation.
func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
		name   string
	}{
		{
			name:   "no truncation needed",
			input:  "short",
			maxLen: 20,
			want:   "short",
		},
		{
			name:   "truncation needed",
			input:  "this is a very long connection string that needs truncation",
			maxLen: 20,
			want:   "this is a very lo...",
		},
		{
			name:   "exact length",
			input:  "exact",
			maxLen: 5,
			want:   "exact",
		},
		{
			name:   "single character",
			input:  "a",
			maxLen: 10,
			want:   "a",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestDisplayStatusTable_NotPanic verifies table display doesn't panic.
func TestDisplayStatusTable_NotPanic(t *testing.T) {
	resp := &tools.HealthCheckResponse{
		TotalChecks:    3,
		HealthyCount:   2,
		UnhealthyCount: 1,
		UnknownCount:   0,
		Results: []tools.HealthCheckResult{
			{
				ConnectionID: "conn1",
				Driver:       "postgres",
				Status:       "healthy",
				Latency:      5,
				Details: tools.DetailsInfo{
					ConnectionURL: "postgres://localhost:5432/db1",
				},
			},
			{
				ConnectionID: "conn2",
				Driver:       "mysql",
				Status:       "unhealthy",
				Latency:      0,
				Error:        "connection failed",
			},
		},
		Errors: map[string]string{
			"conn2": "connection refused",
		},
	}

	assert.NotPanics(t, func() {
		displayStatusTable(resp)
	})
}

// TestDisplayStatusJSON_NotPanic verifies JSON display doesn't panic.
func TestDisplayStatusJSON_NotPanic(t *testing.T) {
	resp := &tools.HealthCheckResponse{
		TotalChecks:    1,
		HealthyCount:   1,
		UnhealthyCount: 0,
		Results: []tools.HealthCheckResult{
			{
				ConnectionID: "test-conn",
				Driver:       "postgres",
				Status:       "healthy",
				Latency:      10,
			},
		},
		Errors: make(map[string]string),
	}

	assert.NotPanics(t, func() {
		displayStatusJSON(resp)
	})
}

// TestStatusCommand_WithFlags tests command with flags.
func TestStatusCommand_WithFlags(t *testing.T) {
	cmd := StatusCommand()
	app := &cli.App{
		Flags:  cmd.Flags,
		Action: cmd.Action,
	}

	// Test with JSON format flag
	args := []string{"app", "--format", "json"}
	err := app.Run(args)
	// Will fail because config doesn't exist, but command should handle it gracefully
	assert.Error(t, err)
}

// BenchmarkFormatTableRow benchmarks row formatting.
func BenchmarkFormatTableRow(b *testing.B) {
	result := &tools.HealthCheckResult{
		ConnectionID: "bench-conn",
		Driver:       "postgres",
		Status:       "healthy",
		Latency:      5,
		Details: tools.DetailsInfo{
			ConnectionURL: "postgres://localhost:5432/benchdb",
		},
	}

	for i := 0; i < b.N; i++ {
		formatTableRow(result)
	}
}

// BenchmarkTruncateString benchmarks string truncation.
func BenchmarkTruncateString(b *testing.B) {
	longStr := "this is a very long connection string that needs truncation for display"
	for i := 0; i < b.N; i++ {
		truncateString(longStr, 35)
	}
}
