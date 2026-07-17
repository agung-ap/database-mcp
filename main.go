package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
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

// version is set at build time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			os.Exit(dbinit.RunWizard())
		case "version":
			fmt.Println("database-mcp " + version)
			os.Exit(0)
		case "secret":
			os.Exit(runSecretCommand(os.Args[2:]))
		}
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
		auditPath = filepath.Join(config.ConfigDir(), "audit.log")
	}
	auditLogger, err := audit.New(auditPath)
	if err != nil {
		slog.Error("failed to open audit log", "path", auditPath, "error", err)
		os.Exit(1)
	}
	defer func() { _ = auditLogger.Close() }()

	// Transaction TTL
	ttlSeconds := 120
	if s := os.Getenv("MCP_DB_TRANSACTION_TTL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			ttlSeconds = n
		}
	}
	txStore := tools.NewTxStore(time.Duration(ttlSeconds) * time.Second)
	defer txStore.Stop()

	// Query timeout: bounds every tool call so a runaway query can't hang
	// the server indefinitely.
	timeoutSeconds := 30
	if s := os.Getenv("MCP_DB_QUERY_TIMEOUT_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			timeoutSeconds = n
		}
	}

	// Build and register MCP server
	srv := internalmcp.NewServer("database-mcp", version)
	h := &internalmcp.Handler{
		Manager:      manager,
		Audit:        auditLogger,
		TxStore:      txStore,
		QueryTimeout: time.Duration(timeoutSeconds) * time.Second,
	}
	h.RegisterAll(srv)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		slog.Info("received shutdown signal", "signal", sig)
		// Roll back open transactions and stop accepting new work before
		// tearing down connection pools, so in-flight queries aren't yanked
		// out from under active requests. Pool/audit cleanup happens via the
		// deferred CloseAll/Close calls once ServeStdio returns.
		txStore.RollbackAll()
		cancel()
	}()

	slog.Info("database-mcp server starting", "config", cfgPath, "connections", len(cfg.Connections))
	if err := srv.ServeStdio(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
