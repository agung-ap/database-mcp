package setup

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/agp/db-mcp/internal/config"
)

// ConfigWriter handles writing configuration to disk with backups and atomic writes.
type ConfigWriter struct {
	configPath string
}

// NewConfigWriter creates a new config writer.
func NewConfigWriter(configPath string) *ConfigWriter {
	return &ConfigWriter{
		configPath: expandHome(configPath),
	}
}

// Write writes the configuration to disk.
// It creates directories as needed, backs up existing configs, and uses atomic writes.
func (cw *ConfigWriter) Write(databases []DatabaseConfig) error {
	// Create directory if doesn't exist
	dir := filepath.Dir(cw.configPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	// Backup existing config if it exists
	if fileExists(cw.configPath) {
		if err := cw.backupConfig(); err != nil {
			slog.Warn("failed to backup existing config", "error", err)
			// Don't fail, just log warning
		}
	}

	// Convert DatabaseConfig to config.Connection
	var conns []config.Connection
	for _, db := range databases {
		conns = append(conns, *db.ToConnectionConfig())
	}

	// Build config
	cfg := &config.Config{
		Version:           "1.0",
		DefaultConnection: "",
		Databases:         conns,
	}

	if len(conns) > 0 {
		cfg.DefaultConnection = conns[0].Name
	}

	// Write using atomic write (temp file + rename)
	return cw.atomicWrite(cfg)
}

// backupConfig backs up the existing configuration file.
func (cw *ConfigWriter) backupConfig() error {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := cw.configPath + "." + timestamp + ".bak"

	slog.Info("backing up existing config", "backup", backupPath)

	content, err := os.ReadFile(cw.configPath)
	if err != nil {
		return fmt.Errorf("failed to read existing config: %w", err)
	}

	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

// atomicWrite performs an atomic write using a temporary file and rename.
func (cw *ConfigWriter) atomicWrite(cfg *config.Config) error {
	// Determine format from extension
	ext := filepath.Ext(cw.configPath)
	var content []byte
	var err error

	switch ext {
	case ".json":
		content, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	default:
		// Default to JSON if no extension
		content, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	}

	// Write to temporary file
	dir := filepath.Dir(cw.configPath)
	tmpFile, err := os.CreateTemp(dir, ".tmp-config-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	_ = tmpFile.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, cw.configPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Ensure proper permissions
	if err := os.Chmod(cw.configPath, 0o600); err != nil {
		slog.Warn("failed to set config file permissions", "error", err)
	}

	slog.Info("config written successfully", "path", cw.configPath)
	return nil
}

// expandHome expands ~ to the user's home directory.
func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
