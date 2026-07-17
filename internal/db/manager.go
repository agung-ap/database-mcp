package db

import (
	"fmt"
	"sync"

	"github.com/agung-ap/database-mcp/internal/config"
	"github.com/agung-ap/database-mcp/internal/db/mssql"
	"github.com/agung-ap/database-mcp/internal/db/mysql"
	"github.com/agung-ap/database-mcp/internal/db/postgres"
)

// Manager manages a set of named database connections.
type Manager struct {
	mu       sync.RWMutex
	drivers  map[string]Driver
	readOnly map[string]bool
	conns    []config.Connection
}

// NewManager creates a Manager from the given connection configs and opens all connections.
func NewManager(connections []config.Connection) (*Manager, error) {
	m := &Manager{
		drivers:  make(map[string]Driver),
		readOnly: make(map[string]bool),
		conns:    connections,
	}
	for _, conn := range connections {
		drv, err := openDriver(conn)
		if err != nil {
			return nil, fmt.Errorf("manager: open %q: %w", conn.ID, err)
		}
		m.drivers[conn.ID] = drv
		m.readOnly[conn.ID] = conn.ReadOnly
	}
	return m, nil
}

// Get returns the Driver for the given connection ID.
func (m *Manager) Get(connectionID string) (Driver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	drv, ok := m.drivers[connectionID]
	if !ok {
		return nil, fmt.Errorf("manager: unknown connection %q", connectionID)
	}
	return drv, nil
}

// IsReadOnly reports whether the given connection is configured as read-only.
// Unknown connection IDs report false; callers should have already validated
// the ID via Get.
func (m *Manager) IsReadOnly(connectionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readOnly[connectionID]
}

// Connections returns the list of configured connections (IDs and drivers).
func (m *Manager) Connections() []config.Connection {
	return m.conns
}

// CloseAll closes all managed connections.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, drv := range m.drivers {
		_ = drv.Close()
		delete(m.drivers, id)
	}
}

// Default pool limits applied when a connection's config leaves the
// corresponding field unset (<= 0). Without these, Go's default of
// "unlimited" open connections lets a single agent exhaust the database's
// connection slots.
const (
	defaultMaxOpen                = 5
	defaultMaxIdle                = 2
	defaultConnMaxLifetimeMinutes = 30
)

func resolvePool(p config.PoolConfig) config.PoolConfig {
	if p.MaxOpen <= 0 {
		p.MaxOpen = defaultMaxOpen
	}
	if p.MaxIdle <= 0 {
		p.MaxIdle = defaultMaxIdle
	}
	if p.ConnMaxLifetimeMinutes <= 0 {
		p.ConnMaxLifetimeMinutes = defaultConnMaxLifetimeMinutes
	}
	return p
}

func openDriver(conn config.Connection) (Driver, error) {
	dsn := conn.BuildDSN()
	pool := resolvePool(conn.Pool)

	switch conn.Driver {
	case "postgres":
		return postgres.New(dsn, postgres.PoolConfig{
			MaxOpen:                pool.MaxOpen,
			MaxIdle:                pool.MaxIdle,
			ConnMaxLifetimeMinutes: pool.ConnMaxLifetimeMinutes,
		})
	case "mysql":
		return mysql.New(dsn, mysql.PoolConfig{
			MaxOpen:                pool.MaxOpen,
			MaxIdle:                pool.MaxIdle,
			ConnMaxLifetimeMinutes: pool.ConnMaxLifetimeMinutes,
		})
	case "sqlserver":
		return mssql.New(dsn, mssql.PoolConfig{
			MaxOpen:                pool.MaxOpen,
			MaxIdle:                pool.MaxIdle,
			ConnMaxLifetimeMinutes: pool.ConnMaxLifetimeMinutes,
		})
	default:
		return nil, fmt.Errorf("unsupported driver %q", conn.Driver)
	}
}
