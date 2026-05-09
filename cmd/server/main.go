package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/agp/db-mcp/internal/audit"
	"github.com/agp/db-mcp/internal/config"
	"github.com/agp/db-mcp/internal/db"
	internalmcp "github.com/agp/db-mcp/internal/mcp"
	"github.com/agp/db-mcp/internal/tools"
)

func main() {
	// Configure log level
	logLevel := os.Getenv("MCP_DB_LOG_LEVEL")
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Load config
	cfgPath := os.Getenv("MCP_DB_CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "./config/connections.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize DB manager
	manager, err := db.NewManager(cfg.Databases)
	if err != nil {
		slog.Error("failed to initialize db manager", "error", err)
		os.Exit(1)
	}
	defer manager.CloseAll()

	// Initialize audit logger
	auditPath := os.Getenv("MCP_DB_AUDIT_LOG")
	if auditPath == "" {
		auditPath = "./audit.log"
	}
	auditLogger, err := audit.New(auditPath)
	if err != nil {
		slog.Error("failed to open audit log", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := auditLogger.Close(); err != nil {
			slog.Error("failed to close audit log", "error", err)
		}
	}()

	// Initialize transaction store
	ttlSeconds := 30
	if s := os.Getenv("MCP_DB_TRANSACTION_TTL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			ttlSeconds = n
		}
	}
	txStore := tools.NewTxStore(time.Duration(ttlSeconds) * time.Second)

	// Build MCP server
	srv := internalmcp.NewServer("db-mcp-server", "1.0.0")
	h := &internalmcp.Handler{
		Manager: manager,
		Audit:   auditLogger,
		TxStore: txStore,
	}
	h.RegisterAll(srv)

	// Handle shutdown signals
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		slog.Info("received shutdown signal", "signal", sig)
		txStore.RollbackAll()
		manager.CloseAll()
		cancel()
	}()

	slog.Info("starting MCP db server", "config", cfgPath)
	if err := srv.ServeStdio(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
