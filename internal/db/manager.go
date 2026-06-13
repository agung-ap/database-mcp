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
	mu      sync.RWMutex
	drivers map[string]Driver
	conns   []config.Connection
}

// NewManager creates a Manager from the given connection configs and opens all connections.
func NewManager(connections []config.Connection) (*Manager, error) {
	m := &Manager{
		drivers: make(map[string]Driver),
		conns:   connections,
	}
	for _, conn := range connections {
		drv, err := openDriver(conn)
		if err != nil {
			return nil, fmt.Errorf("manager: open %q: %w", conn.ID, err)
		}
		m.drivers[conn.ID] = drv
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

func openDriver(conn config.Connection) (Driver, error) {
	dsn := conn.BuildDSN()

	switch conn.Driver {
	case "postgres":
		return postgres.New(dsn, postgres.PoolConfig{
			MaxOpen:                conn.Pool.MaxOpen,
			MaxIdle:                conn.Pool.MaxIdle,
			ConnMaxLifetimeMinutes: conn.Pool.ConnMaxLifetimeMinutes,
		})
	case "mysql":
		return mysql.New(dsn, mysql.PoolConfig{
			MaxOpen:                conn.Pool.MaxOpen,
			MaxIdle:                conn.Pool.MaxIdle,
			ConnMaxLifetimeMinutes: conn.Pool.ConnMaxLifetimeMinutes,
		})
	case "sqlserver":
		return mssql.New(dsn, mssql.PoolConfig{
			MaxOpen:                conn.Pool.MaxOpen,
			MaxIdle:                conn.Pool.MaxIdle,
			ConnMaxLifetimeMinutes: conn.Pool.ConnMaxLifetimeMinutes,
		})
	default:
		return nil, fmt.Errorf("unsupported driver %q", conn.Driver)
	}
}
