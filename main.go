package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/agung-ap/database-mcp/internal/audit"
	"github.com/agung-ap/database-mcp/internal/config"
	"github.com/agung-ap/database-mcp/internal/db"
	dbinit "github.com/agung-ap/database-mcp/internal/init"
	internalmcp "github.com/agung-ap/database-mcp/internal/mcp"
	"github.com/agung-ap/database-mcp/internal/tools"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		os.Exit(dbinit.RunWizard())
	}
	runServer()
}

func runServer() {
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
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Load config
	cfgPath := os.Getenv("MCP_DB_CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "path", cfgPath, "error", err)
		slog.Error("run 'database-mcp init' to set up your configuration")
		os.Exit(1)
	}

	manager, err := db.NewManager(cfg.Connections)
	if err != nil {
		slog.Error("failed to initialize db manager", "error", err)
		os.Exit(1)
	}
	defer manager.CloseAll()

	// Audit log
	auditPath := os.Getenv("MCP_DB_AUDIT_LOG")
	if auditPath == "" {
		auditPath = config.ConfigDir() + "/audit.log"
	}
	auditLogger, err := audit.New(auditPath)
	if err != nil {
		slog.Error("failed to open audit log", "path", auditPath, "error", err)
		os.Exit(1)
	}
	defer func() { _ = auditLogger.Close() }()

	// Transaction TTL
	ttlSeconds := 30
	if s := os.Getenv("MCP_DB_TRANSACTION_TTL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			ttlSeconds = n
		}
	}
	txStore := tools.NewTxStore(time.Duration(ttlSeconds) * time.Second)

	// Build and register MCP server
	srv := internalmcp.NewServer("database-mcp", "2.0.0")
	h := &internalmcp.Handler{
		Manager: manager,
		Audit:   auditLogger,
		TxStore: txStore,
	}
	h.RegisterAll(srv)

	// Graceful shutdown
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

	slog.Info("database-mcp server starting", "config", cfgPath, "connections", len(cfg.Connections))
	if err := srv.ServeStdio(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
