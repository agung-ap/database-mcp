package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/agp/db-mcp/internal/config"
	"github.com/agp/db-mcp/internal/db"
)

// HealthCheckResult represents the result of a health check on a single connection.
type HealthCheckResult struct {
	ConnectionID string        `json:"connection_id"`
	Driver       string        `json:"driver"`
	Status       string        `json:"status"` // "healthy", "unhealthy", "unknown"
	ResponseTime time.Duration `json:"response_time_ms"`
	Latency      int64         `json:"latency_ms"`
	LastChecked  time.Time     `json:"last_checked"`
	Error        string        `json:"error,omitempty"`
	Details      DetailsInfo   `json:"details"`
}

// DetailsInfo contains driver-specific details.
type DetailsInfo struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Database       string `json:"database,omitempty"`
	ConnectionURL  string `json:"connection_url,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty"`
	OpenConns      int    `json:"open_connections,omitempty"`
	InUse          int    `json:"in_use,omitempty"`
	Idle           int    `json:"idle,omitempty"`
}

// HealthCheckRequest contains parameters for health checking.
type HealthCheckRequest struct {
	ConnectionIDs []string      `json:"connection_ids,omitempty"` // Empty = all connections
	Timeout       time.Duration `json:"timeout,omitempty"`        // Default 10 seconds
	Parallel      bool          `json:"parallel"`                 // Run checks in parallel
}

// HealthCheckResponse contains the overall health check result.
type HealthCheckResponse struct {
	Timestamp      time.Time           `json:"timestamp"`
	TotalChecks    int                 `json:"total_checks"`
	HealthyCount   int                 `json:"healthy_count"`
	UnhealthyCount int                 `json:"unhealthy_count"`
	UnknownCount   int                 `json:"unknown_count"`
	Results        []HealthCheckResult `json:"results"`
	Errors         map[string]string   `json:"errors,omitempty"`
}

// HealthChecker performs health checks on database connections.
type HealthChecker struct {
	manager *db.Manager
	logger  *slog.Logger
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(manager *db.Manager, logger *slog.Logger) *HealthChecker {
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthChecker{
		manager: manager,
		logger:  logger,
	}
}

// Check performs health checks on specified connections.
func (hc *HealthChecker) Check(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("health check request cannot be nil")
	}

	// Set defaults
	if req.Timeout == 0 {
		req.Timeout = 10 * time.Second
	}

	// Get all connections or specified ones
	var connNames []string
	allConnections := hc.manager.Connections()

	if len(req.ConnectionIDs) == 0 {
		// Get all connection names from manager
		for _, conn := range allConnections {
			connNames = append(connNames, conn.Name)
		}
	} else {
		connNames = req.ConnectionIDs
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	response := &HealthCheckResponse{
		Timestamp: time.Now(),
		Results:   make([]HealthCheckResult, 0, len(connNames)),
		Errors:    make(map[string]string),
	}

	if req.Parallel {
		hc.checkParallel(ctx, connNames, response)
	} else {
		hc.checkSequential(ctx, connNames, response)
	}

	// Update counts
	for _, result := range response.Results {
		response.TotalChecks++
		switch result.Status {
		case "healthy":
			response.HealthyCount++
		case "unhealthy":
			response.UnhealthyCount++
		case "unknown":
			response.UnknownCount++
		}
	}

	return response, nil
}

// checkSequential performs checks one at a time.
func (hc *HealthChecker) checkSequential(ctx context.Context, connIDs []string, response *HealthCheckResponse) {
	for _, connID := range connIDs {
		result := hc.checkConnection(ctx, connID)
		response.Results = append(response.Results, result)
	}
}

// checkParallel performs checks concurrently.
func (hc *HealthChecker) checkParallel(ctx context.Context, connIDs []string, response *HealthCheckResponse) {
	var wg sync.WaitGroup
	resultsChan := make(chan HealthCheckResult, len(connIDs))
	errorsChan := make(chan struct{ id, err string }, len(connIDs))

	for _, connID := range connIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			result := hc.checkConnection(ctx, id)
			if result.Error != "" {
				errorsChan <- struct{ id, err string }{id, result.Error}
			}
			resultsChan <- result
		}(connID)
	}

	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	for result := range resultsChan {
		response.Results = append(response.Results, result)
	}

	for errInfo := range errorsChan {
		response.Errors[errInfo.id] = errInfo.err
	}
}

// checkConnection performs a health check on a single connection.
func (hc *HealthChecker) checkConnection(ctx context.Context, connID string) HealthCheckResult {
	result := HealthCheckResult{
		ConnectionID: connID,
		LastChecked:  time.Now(),
		Status:       "unknown",
	}

	// Get connection driver
	driver, err := hc.manager.Get(connID)
	if err != nil {
		result.Status = "unhealthy"
		result.Error = fmt.Sprintf("connection not found: %v", err)
		hc.logger.Warn("health check failed: connection not found", "connection_id", connID, "error", err)
		return result
	}

	result.Driver = driver.DriverName()

	// Perform ping test
	startTime := time.Now()
	pingErr := driver.PingContext(ctx)
	elapsed := time.Since(startTime)
	result.Latency = elapsed.Milliseconds()

	if pingErr != nil {
		result.Status = "unhealthy"
		result.Error = pingErr.Error()
		hc.logger.Warn("health check failed: ping error", "connection_id", connID, "driver", result.Driver, "error", pingErr)
		return result
	}

	result.Status = "healthy"
	result.ResponseTime = elapsed

	// Get connection pool stats if available
	hc.captureConnectionDetails(&result, connID)

	hc.logger.Info("health check successful", "connection_id", connID, "driver", result.Driver, "latency_ms", result.Latency)
	return result
}

// captureConnectionDetails extracts connection pool information.
func (hc *HealthChecker) captureConnectionDetails(result *HealthCheckResult, connID string) {
	// Get all connections to find matching config
	connections := hc.manager.Connections()
	if connections == nil {
		return
	}

	for _, connCfg := range connections {
		if connCfg.Name == connID {
			result.Details.Host = connCfg.Host
			result.Details.Port = connCfg.Port
			result.Details.Database = connCfg.Database

			// Build connection URL
			result.Details.ConnectionURL = buildConnectionURL(&connCfg)

			// Get pool stats (driver-specific)
			hc.getPoolStats(result, connCfg.Engine)
			break
		}
	}
}

// getPoolStats retrieves connection pool statistics.
func (hc *HealthChecker) getPoolStats(result *HealthCheckResult, driver string) {
	// This would be implemented using driver-specific methods
	// For now, we set default values
	result.Details.MaxConnections = 10
	result.Details.OpenConns = 1
	result.Details.InUse = 1
	result.Details.Idle = 0
}

// buildConnectionURL builds a connection URL from config.
func buildConnectionURL(connCfg *config.Connection) string {
	switch connCfg.Engine {
	case "postgres":
		return fmt.Sprintf("postgres://%s:%d/%s", connCfg.Host, connCfg.Port, connCfg.Database)
	case "mysql":
		return fmt.Sprintf("mysql://%s:%d/%s", connCfg.Host, connCfg.Port, connCfg.Database)
	case "sqlite":
		// For SQLite, use Database field as the path
		return fmt.Sprintf("sqlite://%s", connCfg.Database)
	default:
		return ""
	}
}

// GetConnectionStatus returns the status of all connections as a JSON string.
func (hc *HealthChecker) GetConnectionStatus(ctx context.Context) (string, error) {
	req := &HealthCheckRequest{
		Timeout:  5 * time.Second,
		Parallel: true,
	}

	resp, err := hc.Check(ctx, req)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(data), nil
}
