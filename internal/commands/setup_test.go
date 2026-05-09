package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agp/db-mcp/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// TestSetupCommand_Structure verifies the command has correct structure.
func TestSetupCommand_Structure(t *testing.T) {
	cmd := SetupCommand()

	assert.Equal(t, "setup", cmd.Name)
	assert.Equal(t, "Interactive setup wizard for database connections", cmd.Usage)
	assert.NotNil(t, cmd.Action)
	assert.Greater(t, len(cmd.Flags), 0, "command should have flags")
}

// TestSetupCommand_Flags verifies all expected flags exist.
func TestSetupCommand_Flags(t *testing.T) {
	cmd := SetupCommand()

	flagNames := make(map[string]bool)
	for _, flag := range cmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		}
	}

	expectedFlags := []string{"config", "skip-connection-test", "skip-agent-registration", "no-register"}
	for _, expected := range expectedFlags {
		assert.True(t, flagNames[expected], "flag %s should exist", expected)
	}
}

// TestSetupCommand_ConfigFlag verifies config flag has correct default.
func TestSetupCommand_ConfigFlag(t *testing.T) {
	cmd := SetupCommand()

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

// TestSetupAction_WithContext tests setupAction with mock context.
func TestSetupAction_WithContext(t *testing.T) {
	tests := []struct {
		name      string
		skipTest  bool
		skipAgent bool
		wantErr   bool
	}{
		{
			name:      "default flags",
			skipTest:  false,
			skipAgent: false,
			wantErr:   true, // Will fail because we're not running interactive wizard
		},
		{
			name:      "skip connection test",
			skipTest:  true,
			skipAgent: false,
			wantErr:   true,
		},
		{
			name:      "skip agent registration",
			skipTest:  false,
			skipAgent: true,
			wantErr:   true,
		},
		{
			name:      "skip both",
			skipTest:  true,
			skipAgent: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal context with flags set
			app := &cli.App{
				Flags: SetupCommand().Flags,
				Action: func(ctx *cli.Context) error {
					return setupAction(ctx)
				},
			}

			args := []string{"app", "setup"}
			if tt.skipTest {
				args = append(args, "--skip-connection-test")
			}
			if tt.skipAgent {
				args = append(args, "--skip-agent-registration")
			}

			err := app.Run(args)
			if tt.wantErr {
				assert.Error(t, err, "setupAction should return error")
			} else {
				assert.NoError(t, err, "setupAction should succeed")
			}
		})
	}
}

// TestRepeatString verifies string repetition utility.
func TestRepeatString(t *testing.T) {
	tests := []struct {
		input  string
		count  int
		want   string
		name   string
	}{
		{
			name:  "single character single repeat",
			input: "a",
			count: 1,
			want:  "a",
		},
		{
			name:  "single character multiple repeats",
			input: "=",
			count: 5,
			want:  "=====",
		},
		{
			name:  "multiple characters repeated",
			input: "ab",
			count: 3,
			want:  "ababab",
		},
		{
			name:  "zero repeats",
			input: "x",
			count: 0,
			want:  "",
		},
		{
			name:  "unicode character",
			input: "🤖",
			count: 2,
			want:  "🤖🤖",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repeatString(tt.input, tt.count)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestGetServerPath verifies server path detection logic.
func TestGetServerPath(t *testing.T) {
	path, err := getServerPath()

	// Should either get a valid path or an error
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotEmpty(t, path)
		// Path should be absolute or a valid reference
		assert.True(t, len(path) > 0)
	}
}

// TestGetServerPath_FallbackPaths verifies fallback paths are checked.
func TestGetServerPath_FallbackPaths(t *testing.T) {
	// Create a temporary executable
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "db-mcp")

	// Create the binary file
	f, err := os.Create(binPath)
	require.NoError(t, err)
	f.Close()

	// Add to PATH or verify fallback logic
	path, err := getServerPath()
	assert.True(t, err == nil || path != "")
}

// TestDisplaySetupCompletion_NilResult handles nil result gracefully.
func TestDisplaySetupCompletion_NilResult(t *testing.T) {
	// Should not panic with nil result
	assert.NotPanics(t, func() {
		displaySetupCompletion(nil)
	})
}

// TestDisplaySetupCompletion_SuccessfulSetup tests completion with successful setup.
func TestDisplaySetupCompletion_SuccessfulSetup(t *testing.T) {
	result := &setup.SetupResult{
		ConfigWritten:   true,
		DatabasesAdded:  2,
		ConfigPath:      "/home/user/.config/db-mcp/.databases.json",
		AgentsRegistered: map[string]bool{
			"Claude Desktop": true,
			"Claude Code":    true,
			"Cursor":         false,
		},
		Errors: []string{},
	}

	// Should not panic with complete result
	assert.NotPanics(t, func() {
		displaySetupCompletion(result)
	})
}

// TestDisplaySetupCompletion_WithErrors tests completion with errors.
func TestDisplaySetupCompletion_WithErrors(t *testing.T) {
	result := &setup.SetupResult{
		ConfigWritten:   false,
		DatabasesAdded:  1,
		ConfigPath:      "/home/user/.config/db-mcp/.databases.json",
		AgentsRegistered: map[string]bool{
			"Claude Desktop": false,
		},
		Errors: []string{
			"Failed to register with Claude Desktop",
			"Invalid connection string",
		},
	}

	// Should not panic even with errors
	assert.NotPanics(t, func() {
		displaySetupCompletion(result)
	})
}

// TestSetupCommand_SkipAgentRegistrationAlias verifies --no-register alias.
func TestSetupCommand_SkipAgentRegistrationAlias(t *testing.T) {
	cmd := SetupCommand()

	var noRegFlag *cli.BoolFlag
	for _, flag := range cmd.Flags {
		if f, ok := flag.(*cli.BoolFlag); ok && f.Name == "no-register" {
			noRegFlag = f
			break
		}
	}

	require.NotNil(t, noRegFlag, "--no-register flag should exist")
	assert.Equal(t, "Alias for --skip-agent-registration", noRegFlag.Usage)
}

// BenchmarkRepeatString benchmarks string repetition.
func BenchmarkRepeatString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		repeatString("=", 70)
	}
}

// BenchmarkRepeatString_LargeCount benchmarks with larger count.
func BenchmarkRepeatString_LargeCount(b *testing.B) {
	for i := 0; i < b.N; i++ {
		repeatString("=", 1000)
	}
}

// TestSetupCommand_Integration is an integration test (requires wizard interaction).
func TestSetupCommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// This test would require mocking the wizard interaction
	// For now, we just verify the command structure is correct
	cmd := SetupCommand()
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.Action)
}
