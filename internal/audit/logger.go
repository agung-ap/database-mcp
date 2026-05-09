package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

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

// New opens (or creates) the audit log file at the given path.
func New(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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
		return
	}
	_, _ = l.file.Write(append(data, '\n'))
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
