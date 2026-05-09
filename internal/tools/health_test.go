package tools

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/agp/db-mcp/internal/config"
	"github.com/agp/db-mcp/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDriver implements the db.Driver interface for testing.
type MockDriver struct {
	driverName string
	latency    time.Duration
	pingErr    error
}

func (m *MockDriver) QueryContext(ctx context.Context, query string, args ...any) (any, error) {
	return nil, nil
}

func (m *MockDriver) ExecContext(ctx context.Context, query string, args ...any) (any, error) {
	return nil, nil
}

func (m *MockDriver) BeginTx(ctx context.Context, opts any) (any, error) {
	return nil, nil
}

func (m *MockDriver) PingContext(ctx context.Context) error {
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return m.pingErr
}

func (m *MockDriver) Close() error {
	return nil
}

func (m *MockDriver) DriverName() string {
	return m.driverName
}

// TestHealthChecker_NewHealthChecker verifies instantiation.
func TestHealthChecker_NewHealthChecker(t *testing.T) {
	connections := []config.Connection{
		{
			Name:   "test-pg",
			Engine: "postgres",
			Host:   "localhost",
			Port:   5432,
		},
	}

	manager, err := db.NewManager(connections)
	require.NoError(t, err)

	checker := NewHealthChecker(manager, slog.Default())
	assert.NotNil(t, checker)
	assert.NotNil(t, checker.manager)
	assert.NotNil(t, checker.logger)
}

// TestHealthChecker_NewHealthChecker_NilLogger uses default logger.
func TestHealthChecker_NewHealthChecker_NilLogger(t *testing.T) {
	connections := []config.Connection{
		{
			Name:   "test-pg",
			Engine: "postgres",
			Host:   "localhost",
			Port:   5432,
		},
	}

	manager, err := db.NewManager(connections)
	require.NoError(t, err)

	checker := NewHealthChecker(manager, nil)
	assert.NotNil(t, checker.logger)
}

// TestHealthCheckRequest_Defaults tests default values.
func TestHealthCheckRequest_Defaults(t *testing.T) {
	req := &HealthCheckRequest{
		Timeout:  0,
		Parallel: false,
	}

	assert.Equal(t, time.Duration(0), req.Timeout)
	assert.False(t, req.Parallel)
}

// TestHealthCheckResult_StatusValues tests status field.
func TestHealthCheckResult_StatusValues(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"healthy", "healthy"},
		{"unhealthy", "unhealthy"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HealthCheckResult{
				Status: tt.status,
			}
			assert.Equal(t, tt.status, result.Status)
		})
	}
}

// TestHealthCheckResponse_CounterFields tests counter initialization.
func TestHealthCheckResponse_CounterFields(t *testing.T) {
	resp := &HealthCheckResponse{
		Timestamp:      time.Now(),
		TotalChecks:    0,
		HealthyCount:   0,
		UnhealthyCount: 0,
		UnknownCount:   0,
		Results:        []HealthCheckResult{},
		Errors:         make(map[string]string),
	}

	assert.Equal(t, 0, resp.TotalChecks)
	assert.Equal(t, 0, resp.HealthyCount)
	assert.Equal(t, 0, resp.UnhealthyCount)
	assert.Equal(t, 0, resp.UnknownCount)
	assert.Len(t, resp.Results, 0)
	assert.Len(t, resp.Errors, 0)
}

// TestBuildConnectionURL tests URL construction.
func TestBuildConnectionURL(t *testing.T) {
	tests := []struct {
		name   string
		conn   *config.Connection
		expect string
	}{
		{
			name: "postgres",
			conn: &config.Connection{
				Engine:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
			},
			expect: "postgres://localhost:5432/testdb",
		},
		{
			name: "mysql",
			conn: &config.Connection{
				Engine:   "mysql",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
			},
			expect: "mysql://localhost:3306/testdb",
		},
		{
			name: "sqlite",
			conn: &config.Connection{
				Engine:   "sqlite",
				Database: "/path/to/db.sqlite",
			},
			expect: "sqlite:///path/to/db.sqlite",
		},
		{
			name: "unknown engine",
			conn: &config.Connection{
				Engine:   "unknown",
				Host:     "localhost",
				Database: "testdb",
			},
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildConnectionURL(tt.conn)
			assert.Equal(t, tt.expect, url)
		})
	}
}

// TestDetailsInfo_Initialization tests Details structure.
func TestDetailsInfo_Initialization(t *testing.T) {
	details := DetailsInfo{
		Host:           "localhost",
		Port:           5432,
		Database:       "testdb",
		ConnectionURL:  "postgres://localhost:5432/testdb",
		MaxConnections: 10,
		OpenConns:      5,
		InUse:          3,
		Idle:           2,
	}

	assert.Equal(t, "localhost", details.Host)
	assert.Equal(t, 5432, details.Port)
	assert.Equal(t, "testdb", details.Database)
	assert.Equal(t, 10, details.MaxConnections)
	assert.Equal(t, 5, details.OpenConns)
	assert.Equal(t, 3, details.InUse)
	assert.Equal(t, 2, details.Idle)
}

// TestHealthCheckResult_ErrorField tests error handling.
func TestHealthCheckResult_ErrorField(t *testing.T) {
	tests := []struct {
		name      string
		result    HealthCheckResult
		expectErr bool
	}{
		{
			name: "with error",
			result: HealthCheckResult{
				Status: "unhealthy",
				Error:  "connection refused",
			},
			expectErr: true,
		},
		{
			name: "healthy without error",
			result: HealthCheckResult{
				Status: "healthy",
				Error:  "",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.result.Error != ""
			assert.Equal(t, tt.expectErr, hasError)
		})
	}
}

// TestHealthCheckResponse_ResultsAppend tests appending results.
func TestHealthCheckResponse_ResultsAppend(t *testing.T) {
	resp := &HealthCheckResponse{
		Timestamp: time.Now(),
		Results:   make([]HealthCheckResult, 0),
		Errors:    make(map[string]string),
	}

	result1 := HealthCheckResult{
		ConnectionID: "conn1",
		Status:       "healthy",
	}
	result2 := HealthCheckResult{
		ConnectionID: "conn2",
		Status:       "unhealthy",
	}

	resp.Results = append(resp.Results, result1, result2)

	assert.Len(t, resp.Results, 2)
	assert.Equal(t, "conn1", resp.Results[0].ConnectionID)
	assert.Equal(t, "conn2", resp.Results[1].ConnectionID)
}

// TestHealthCheckResponse_ErrorsMap tests error map handling.
func TestHealthCheckResponse_ErrorsMap(t *testing.T) {
	resp := &HealthCheckResponse{
		Errors: make(map[string]string),
	}

	resp.Errors["conn1"] = "failed to connect"
	resp.Errors["conn2"] = "timeout"

	assert.Len(t, resp.Errors, 2)
	assert.Equal(t, "failed to connect", resp.Errors["conn1"])
	assert.Equal(t, "timeout", resp.Errors["conn2"])
}

// TestHealthChecker_CaptureConnectionDetails tests detail extraction.
func TestHealthChecker_CaptureConnectionDetails(t *testing.T) {
	connections := []config.Connection{
		{
			Name:     "test-pg",
			Engine:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Database: "testdb",
		},
	}

	manager, err := db.NewManager(connections)
	require.NoError(t, err)

	checker := NewHealthChecker(manager, slog.Default())

	result := &HealthCheckResult{
		ConnectionID: "test-pg",
	}

	checker.captureConnectionDetails(result, "test-pg")

	assert.Equal(t, "localhost", result.Details.Host)
	assert.Equal(t, 5432, result.Details.Port)
	assert.Equal(t, "testdb", result.Details.Database)
	assert.Equal(t, "postgres://localhost:5432/testdb", result.Details.ConnectionURL)
}

// TestHealthChecker_GetPoolStats tests pool stats extraction.
func TestHealthChecker_GetPoolStats(t *testing.T) {
	manager, err := db.NewManager([]config.Connection{})
	require.NoError(t, err)

	checker := NewHealthChecker(manager, slog.Default())

	result := &HealthCheckResult{}
	checker.getPoolStats(result, "postgres")

	assert.Equal(t, 10, result.Details.MaxConnections)
	assert.Equal(t, 1, result.Details.OpenConns)
	assert.Equal(t, 1, result.Details.InUse)
	assert.Equal(t, 0, result.Details.Idle)
}

// BenchmarkBuildConnectionURL benchmarks URL building.
func BenchmarkBuildConnectionURL(b *testing.B) {
	conn := &config.Connection{
		Engine:   "postgres",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
	}

	for i := 0; i < b.N; i++ {
		buildConnectionURL(conn)
	}
}

// BenchmarkHealthCheckResult benchmarks result creation.
func BenchmarkHealthCheckResult(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = HealthCheckResult{
			ConnectionID: "test-conn",
			Status:       "healthy",
			Latency:      5,
		}
	}
}
