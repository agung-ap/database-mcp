package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxLogSizeBytes is the size at which the audit log is rotated to a single
// backup generation (audit.log -> audit.log.1) on the next New() call.
const maxLogSizeBytes = 50 * 1024 * 1024

// AuditEntry represents a single audit log record.
type AuditEntry struct {
	QueryID      string    `json:"query_id"`
	Timestamp    time.Time `json:"timestamp"`
	Tool         string    `json:"tool"`
	ConnectionID string    `json:"connection_id"`
	Driver       string    `json:"driver"`
	Query        string    `json:"query,omitempty"`
	ParamCount   int       `json:"param_count,omitempty"`
	DurationMS   int64     `json:"duration_ms,omitempty"`
	RowsAffected int64     `json:"rows_affected,omitempty"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
}

// Logger writes JSON-lines audit entries to a file.
type Logger struct {
	mu   sync.Mutex
	file *os.File
}

// New opens (or creates) the audit log file at the given path, creating the
// parent directory if necessary. The file (and its containing directory) are
// restricted to the owner since audit entries include full query text.
func New(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("audit: mkdir %q: %w", filepath.Dir(path), err)
	}

	if info, err := os.Stat(path); err == nil && info.Size() > maxLogSizeBytes {
		rotated := path + ".1"
		_ = os.Remove(rotated)
		if err := os.Rename(path, rotated); err != nil {
			slog.Warn("audit: failed to rotate log", "path", path, "error", err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: open %q: %w", path, err)
	}
	return &Logger{file: f}, nil
}

// Log writes a single AuditEntry as a JSON line.
func (l *Logger) Log(entry AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		slog.Error("audit: failed to marshal entry", "error", err)
		return
	}
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		slog.Error("audit: failed to write entry", "error", err)
	}
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
