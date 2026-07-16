// Package init provides the interactive onboarding wizard for database-mcp.
package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"

	"github.com/agung-ap/database-mcp/internal/config"
	"github.com/agung-ap/database-mcp/internal/db"
	"github.com/agung-ap/database-mcp/internal/secrets"
	"golang.org/x/term"
)

var connIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// RunWizard runs the interactive setup wizard and returns an exit code.
func RunWizard() int {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "database-mcp init requires an interactive terminal.")
		fmt.Fprintf(os.Stderr, "Edit the config file directly instead: %s\n", config.DefaultConfigPath())
		return 1
	}

	fmt.Println("database-mcp setup")
	fmt.Println()

	cfgDir := config.ConfigDir()
	cfgPath := config.DefaultConfigPath()
	fmt.Printf("Config directory: %s\n\n", cfgDir)

	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		return 1
	}

	cfg := loadRawConfig(cfgPath)
	keyringAvailable := secrets.Available()
	if !keyringAvailable {
		fmt.Println("Note: no OS credential store detected (Keychain/Credential Manager/Secret Service). Passwords will be stored in plaintext.")
		fmt.Println()
	}

	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = "database-mcp"
	}

	for {
		var action string
		options := []huh.Option[string]{huh.NewOption("Add connection", "add")}
		if len(cfg.Connections) > 0 {
			options = append(options,
				huh.NewOption("Edit connection", "edit"),
				huh.NewOption("Delete connection", "delete"),
			)
		}
		options = append(options,
			huh.NewOption("Configure agents", "agents"),
			huh.NewOption("Save & exit", "save"),
		)

		menu := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().Title("database-mcp").Description(formatConnectionsSummary(cfg.Connections)),
				huh.NewSelect[string]().
					Title("What would you like to do?").
					Options(options...).
					Value(&action),
			),
		)
		if err := menu.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "\n%v\n", err)
			return 1
		}

		switch action {
		case "add":
			conn, ok := runConnectionForm(cfg.Connections, nil, keyringAvailable)
			if ok {
				cfg.Connections = append(cfg.Connections, conn)
				saveRawConfigOrWarn(cfgPath, cfg)
			}
		case "edit":
			id, ok := pickConnection(cfg.Connections, "Edit which connection?")
			if !ok {
				continue
			}
			idx := indexOfConnection(cfg.Connections, id)
			conn, ok := runConnectionForm(cfg.Connections, &cfg.Connections[idx], keyringAvailable)
			if ok {
				cfg.Connections[idx] = conn
				saveRawConfigOrWarn(cfgPath, cfg)
			}
		case "delete":
			id, ok := pickConnection(cfg.Connections, "Delete which connection?")
			if !ok {
				continue
			}
			var confirmed bool
			_ = huh.NewForm(huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Delete connection %q? This cannot be undone.", id)).
					Value(&confirmed),
			)).Run()
			if confirmed {
				idx := indexOfConnection(cfg.Connections, id)
				_ = secrets.Delete(id)
				cfg.Connections = append(cfg.Connections[:idx], cfg.Connections[idx+1:]...)
				saveRawConfigOrWarn(cfgPath, cfg)
			}
		case "agents":
			configureAgentsForm(binaryPath)
		case "save", "":
			fmt.Println()
			fmt.Println("Setup complete! Restart your agent to load the new MCP server.")
			fmt.Printf("Config: %s\n", cfgPath)
			return 0
		}
	}
}

func loadRawConfig(cfgPath string) config.Config {
	var cfg config.Config
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err == nil {
			fmt.Printf("Found existing config with %d connection(s).\n\n", len(cfg.Connections))
		}
	}
	return cfg
}

func saveRawConfigOrWarn(cfgPath string, cfg config.Config) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding config: %v\n", err)
		return
	}
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
	}
}

func formatConnectionsSummary(conns []config.Connection) string {
	if len(conns) == 0 {
		return "No connections configured yet."
	}
	var b strings.Builder
	for _, c := range conns {
		flags := ""
		if c.ReadOnly {
			flags = " [read_only]"
		}
		fmt.Fprintf(&b, "  - %s (%s)%s\n", c.ID, c.Driver, flags)
	}
	return b.String()
}

func indexOfConnection(conns []config.Connection, id string) int {
	for i, c := range conns {
		if c.ID == id {
			return i
		}
	}
	return -1
}

func pickConnection(conns []config.Connection, title string) (string, bool) {
	if len(conns) == 0 {
		return "", false
	}
	options := make([]huh.Option[string], 0, len(conns))
	for _, c := range conns {
		options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", c.ID, c.Driver), c.ID))
	}
	var id string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Options(options...).Value(&id),
	)).Run(); err != nil {
		return "", false
	}
	return id, true
}

// runConnectionForm walks the user through adding or editing a connection.
// existing is nil when adding. Returns the connection and whether it should
// be saved (false if the user backs out or the ping fails and they decline
// to save anyway).
func runConnectionForm(all []config.Connection, existing *config.Connection, keyringAvailable bool) (config.Connection, bool) {
	var conn config.Connection
	if existing != nil {
		conn = *existing
	}

	editingID := ""
	if existing != nil {
		editingID = existing.ID
	}

	// Step 1: ID + driver, since the default port and SSL options in step 2
	// depend on which driver was picked.
	idInput := huh.NewInput().
		Title("Connection ID").
		Description("e.g. prod-pg").
		Value(&conn.ID).
		Validate(func(s string) error {
			if !connIDPattern.MatchString(s) {
				return fmt.Errorf("use lowercase letters, numbers, - and _, starting with a letter or digit")
			}
			if s != editingID && indexOfConnection(all, s) >= 0 {
				return fmt.Errorf("connection %q already exists", s)
			}
			return nil
		})

	driverSelect := huh.NewSelect[string]().
		Title("Driver").
		Options(
			huh.NewOption("PostgreSQL", "postgres"),
			huh.NewOption("MySQL", "mysql"),
			huh.NewOption("SQL Server", "sqlserver"),
		).
		Value(&conn.Driver)

	step1 := huh.NewForm(huh.NewGroup(idInput, driverSelect))
	if existing != nil {
		// Editing: ID is fixed, only the driver could change (rare, but allowed).
		step1 = huh.NewForm(huh.NewGroup(driverSelect))
	}
	if err := step1.Run(); err != nil {
		return config.Connection{}, false
	}

	// Step 2: connection details, defaults derived from the chosen driver.
	defaultPort := map[string]int{"postgres": 5432, "mysql": 3306, "sqlserver": 1433}[conn.Driver]
	if conn.Port == 0 {
		conn.Port = defaultPort
	}
	if conn.Host == "" {
		conn.Host = "localhost"
	}

	portStr := strconv.Itoa(conn.Port)
	var password string
	storeInKeyring := keyringAvailable && conn.PasswordSource == "keyring"

	fields := []huh.Field{
		huh.NewInput().Title("Host").Value(&conn.Host),
		huh.NewInput().Title("Port").Value(&portStr).Validate(func(s string) error {
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 {
				return fmt.Errorf("enter a valid port number")
			}
			return nil
		}),
		huh.NewInput().Title("Database name").Value(&conn.Database).Validate(requiredField),
		huh.NewInput().Title("User").Value(&conn.User).Validate(requiredField),
		huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&password).
			Description("Leave blank to keep the current password (edit) or set one later via MCP_DB_CONN_<ID>_DSN"),
	}
	if keyringAvailable {
		fields = append(fields, huh.NewConfirm().
			Title("Store this password in the OS credential store?").
			Affirmative("Yes").Negative("No").
			Value(&storeInKeyring))
	}
	fields = append(fields,
		huh.NewSelect[string]().Title("SSL mode").Options(sslModeOptions(conn.Driver)...).Value(&conn.SSLMode),
		huh.NewConfirm().Title("Read-only connection? (blocks mutations and transactions)").
			Affirmative("Yes").Negative("No").Value(&conn.ReadOnly),
	)

	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return config.Connection{}, false
	}
	conn.Port, _ = strconv.Atoi(portStr)

	// Keep the plaintext password around for the connection test below; the
	// keyring/plaintext split only happens once the user actually saves,
	// since BuildDSN needs a real password to dial with regardless of where
	// it ends up stored.
	testPassword := password
	if testPassword == "" && existing != nil && existing.PasswordSource == "keyring" {
		if pw, err := secrets.Get(existing.ID); err == nil {
			testPassword = pw
		}
	} else if testPassword == "" {
		testPassword = conn.Password
	}

	// Step 3: optional advanced pool settings.
	var configurePool bool
	_ = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Configure advanced connection pool settings?").
			Affirmative("Yes").Negative("No, use defaults").Value(&configurePool),
	)).Run()
	if configurePool {
		maxOpen := strconv.Itoa(conn.Pool.MaxOpen)
		maxIdle := strconv.Itoa(conn.Pool.MaxIdle)
		lifetime := strconv.Itoa(conn.Pool.ConnMaxLifetimeMinutes)
		_ = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Max open connections (0 = default)").Value(&maxOpen).Validate(optionalIntField),
			huh.NewInput().Title("Max idle connections (0 = default)").Value(&maxIdle).Validate(optionalIntField),
			huh.NewInput().Title("Max connection lifetime, minutes (0 = default)").Value(&lifetime).Validate(optionalIntField),
		)).Run()
		conn.Pool.MaxOpen, _ = strconv.Atoi(maxOpen)
		conn.Pool.MaxIdle, _ = strconv.Atoi(maxIdle)
		conn.Pool.ConnMaxLifetimeMinutes, _ = strconv.Atoi(lifetime)
	}

	// Step 4: test the connection before saving. Build the test copy with
	// the plaintext password regardless of where it will end up stored.
	testConn := conn
	testConn.Password, testConn.PasswordSource = testPassword, ""

	fmt.Printf("\nTesting connection to %q...\n", conn.ID)
	if latency, err := testConnection(testConn); err != nil {
		fmt.Printf("Connection test failed: %v\n", err)
		var saveAnyway bool
		_ = huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Save this connection anyway?").
				Affirmative("Yes").Negative("No").Value(&saveAnyway),
		)).Run()
		if !saveAnyway {
			return config.Connection{}, false
		}
	} else {
		fmt.Printf("Connection OK (%s)\n", latency)
	}

	// Now that the connection is confirmed to be saved, decide where the
	// password lives.
	if password != "" {
		if storeInKeyring {
			if err := secrets.Set(conn.ID, password); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not store password in keyring: %v (falling back to plaintext)\n", err)
				conn.Password, conn.PasswordSource = password, ""
			} else {
				conn.Password, conn.PasswordSource = "", "keyring"
			}
		} else {
			conn.Password, conn.PasswordSource = password, ""
		}
	}

	return conn, true
}

func requiredField(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("required")
	}
	return nil
}

func optionalIntField(s string) error {
	if s == "" {
		return nil
	}
	if _, err := strconv.Atoi(s); err != nil {
		return fmt.Errorf("enter a whole number")
	}
	return nil
}

func sslModeOptions(driver string) []huh.Option[string] {
	switch driver {
	case "mysql":
		return []huh.Option[string]{
			huh.NewOption("disable", "disable"),
			huh.NewOption("preferred", "preferred"),
			huh.NewOption("true (require TLS)", "true"),
			huh.NewOption("skip-verify", "skip-verify"),
		}
	case "sqlserver":
		return []huh.Option[string]{
			huh.NewOption("disable", "disable"),
			huh.NewOption("true", "true"),
			huh.NewOption("false", "false"),
		}
	default: // postgres
		return []huh.Option[string]{
			huh.NewOption("disable", "disable"),
			huh.NewOption("require", "require"),
			huh.NewOption("verify-ca", "verify-ca"),
			huh.NewOption("verify-full", "verify-full"),
		}
	}
}

// testConnection opens a throwaway connection using the existing driver
// plumbing and pings it with a short timeout.
func testConnection(conn config.Connection) (time.Duration, error) {
	manager, err := db.NewManager([]config.Connection{conn})
	if err != nil {
		return 0, err
	}
	defer manager.CloseAll()

	drv, err := manager.Get(conn.ID)
	if err != nil {
		return 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	if err := drv.PingContext(ctx); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

func configureAgentsForm(binaryPath string) {
	var claudeCode, claudeDesktop, openCode, vscode bool
	err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Claude Code (CLI)").Value(&claudeCode),
		huh.NewConfirm().Title("Claude Desktop (GUI app)").Value(&claudeDesktop),
		huh.NewConfirm().Title("OpenCode").Value(&openCode),
		huh.NewConfirm().Title("GitHub Copilot in VS Code (writes .vscode/mcp.json in current dir)").Value(&vscode),
	)).Run()
	if err != nil {
		return
	}

	if claudeCode {
		if err := WriteClaudeCodeConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not configure Claude Code: %v\n", err)
		} else {
			fmt.Println("  Claude Code configured (user scope)")
		}
	}
	if claudeDesktop {
		if err := WriteClaudeDesktopConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not write Claude Desktop config: %v\n", err)
		} else {
			fmt.Println("  Claude Desktop configured")
		}
	}
	if openCode {
		if err := WriteOpenCodeConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not write OpenCode config: %v\n", err)
		} else {
			fmt.Println("  OpenCode configured")
		}
	}
	if vscode {
		if err := WriteVSCodeConfig(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not write VS Code config: %v\n", err)
		} else {
			fmt.Println("  GitHub Copilot in VS Code configured (.vscode/mcp.json)")
		}
	}
}

// mergeJSON reads existing JSON from path (if any) and merges new key into it.
func mergeJSON(path string, key string, value any) error {
	obj := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &obj)
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 2 {
		sub, _ := obj[parts[0]].(map[string]any)
		if sub == nil {
			sub = make(map[string]any)
		}
		sub[parts[1]] = value
		obj[parts[0]] = sub
	} else {
		obj[key] = value
	}

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
