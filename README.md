# database-mcp

Give your AI agent direct, safe access to your PostgreSQL and MySQL databases — query data, inspect schemas, run transactions, and more, all from within Claude Code or any MCP-compatible agent.

## Features

- **Query databases** — run SELECT queries and get results as JSON
- **Inspect schemas** — list tables, columns, indexes, and foreign keys
- **Run mutations** — INSERT, UPDATE, DELETE with an explicit confirmation gate
- **Transactions** — begin, commit, or roll back multi-step operations
- **Explain queries** — see the execution plan before running expensive queries
- **Table stats** — row counts, size, live/dead tuples at a glance
- **Multiple connections** — manage several databases (Postgres or MySQL) from one config
- **Audit log** — every tool call is logged automatically

---

## Install

```bash
go install github.com/agung-ap/database-mcp@latest
```

---

## Setup

### 1. Create the config file

The config lives at `~/.config/database-mcp/connections.json`. Create it manually or copy the example below.

```json
{
  "connections": [
    {
      "id": "my-db",
      "driver": "postgres",
      "host": "localhost",
      "port": 5432,
      "database": "myapp",
      "user": "myuser",
      "password": "mypassword",
      "ssl_mode": "disable",
      "read_only": false
    }
  ]
}
```

**Supported drivers:** `postgres`, `mysql`, `sqlserver`

**`ssl_mode` options (PostgreSQL):** `disable`, `require`, `verify-ca`, `verify-full`

**`ssl_mode` options (MySQL):** `disable`, `preferred`, `true` (require TLS), `skip-verify`

**`ssl_mode` for `sqlserver`:** maps to the `encrypt` connection option (e.g. `disable`, `true`, `false`)

**`read_only`:** when `true`, `execute_mutation` and `begin_transaction` are rejected outright for this connection — use it for production/reporting databases you never want an agent to write to.

> **Note:** `execute_query` always begins a read-only transaction (`sql.TxOptions{ReadOnly: true}`) for `postgres` and `mysql`, so a stray write can't slip through and the pooled connection is never left in a read-only state afterwards. SQL Server has no equivalent session-level read-only mode, so this guard is not enforced for `sqlserver` connections — use `read_only: true` on the connection if you need a hard guarantee there.

> **Tip:** Keep passwords out of the config. Two options:
> - Full DSN override via environment variable (takes priority over everything else):
>   ```bash
>   export MCP_DB_CONN_MY_DB_DSN="postgres://myuser:secret@localhost:5432/myapp?sslmode=disable"
>   ```
> - Store the password in your OS's credential store (macOS Keychain, Windows Credential Manager, or the Linux/BSD Secret Service) and reference it from the config — see [Storing passwords securely](#storing-passwords-securely) below.

### 2. Add to Claude Code

```bash
claude mcp add database-mcp /path/to/database-mcp \
  --scope user \
  --env MCP_DB_CONFIG_PATH=~/.config/database-mcp/connections.json
```

### 3. Add to Claude Desktop

Add to `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "database-mcp": {
      "command": "/path/to/database-mcp",
      "args": [],
      "env": {
        "MCP_DB_CONFIG_PATH": "/home/you/.config/database-mcp/connections.json"
      }
    }
  }
}
```

### 4. Add to VS Code / GitHub Copilot

Create `.vscode/mcp.json` in your workspace:

```json
{
  "servers": {
    "database-mcp": {
      "command": "database-mcp",
      "args": [],
      "type": "stdio"
    }
  }
}
```

---

## Available Tools (14)

### Connection
| Tool | What it does |
|------|-------------|
| `list_connections` | List all configured connections and their drivers |
| `test_connection` | Ping a connection and return latency |

### Schema
| Tool | What it does |
|------|-------------|
| `list_databases` | List databases visible to the user |
| `list_tables` | List tables in the connected database |
| `describe_table` | Show column definitions for a table |
| `list_indexes` | Show indexes on a table |
| `list_foreign_keys` | Show foreign key constraints on a table |

### Query
| Tool | What it does |
|------|-------------|
| `execute_query` | Run a SELECT query, returns `{"rows": [...], "row_count": N, "truncated": bool}` (max 1000 rows). Pass `tx_id` instead of `connection_id` to run inside an open transaction. |
| `execute_mutation` | Run INSERT/UPDATE/DELETE — requires `confirm: true`. Rejected outright on `read_only` connections. Pass `tx_id` instead of `connection_id` to run inside an open transaction. |
| `explain_query` | Return the execution plan for a query (works on Postgres, MySQL, and SQL Server) |

### Stats
| Tool | What it does |
|------|-------------|
| `get_table_stats` | Row count, size, live/dead tuples |

### Transactions
| Tool | What it does |
|------|-------------|
| `begin_transaction` | Begin a transaction, returns a `tx_id` |
| `commit_transaction` | Commit by `tx_id` |
| `rollback_transaction` | Roll back by `tx_id` |

---

## Configuration Reference

### connections.json fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | ✅ | Unique name for this connection |
| `driver` | ✅ | `postgres`, `mysql`, or `sqlserver` |
| `host` | ✅ | Database host (unless a DSN override is set) |
| `port` | ✅ | Database port |
| `database` | ✅ | Database name (unless a DSN override is set) |
| `user` | ✅ | Username |
| `password` | — | Password in plaintext (use `password_source` or a DSN override to avoid this) |
| `password_source` | — | Set to `"keyring"` to read the password from the OS credential store instead of the `password` field — see [Storing passwords securely](#storing-passwords-securely) |
| `ssl_mode` | — | SSL mode (see above) |
| `read_only` | — | When `true`, blocks `execute_mutation` and `begin_transaction` on this connection |
| `pool.max_open` | — | Max open connections (default: 5) |
| `pool.max_idle` | — | Max idle connections (default: 2) |
| `pool.conn_max_lifetime_minutes` | — | Max connection lifetime (default: 30) |

Config is validated on load: unknown drivers, duplicate IDs, and missing `host`/`database` (when no DSN override is set) are reported together so you can fix them in one pass.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_DB_CONFIG_PATH` | `~/.config/database-mcp/connections.json` | Path to config file |
| `MCP_DB_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `MCP_DB_AUDIT_LOG` | `~/.config/database-mcp/audit.log` | Audit log path (created `0600`) |
| `MCP_DB_TRANSACTION_TTL_SECONDS` | `120` | Seconds of inactivity before an open transaction auto-rolls back (each `execute_query`/`execute_mutation` on the tx resets the timer) |
| `MCP_DB_QUERY_TIMEOUT_SECONDS` | `30` | Max time any single tool call may run before it's cancelled |
| `MCP_DB_CONN_<ID>_DSN` | — | Full DSN override for a connection (replace `-` with `_` in the ID); takes priority over `password_source` and `password` |

---

## Storing passwords securely

Passwords can be kept out of `connections.json` entirely by storing them in your OS's native credential store (macOS Keychain, Windows Credential Manager, or the Linux/BSD Secret Service API — via [zalando/go-keyring](https://github.com/zalando/go-keyring)):

```bash
database-mcp secret set my-db      # prompts for the password (hidden input)
database-mcp secret delete my-db   # removes it
```

Then set `"password_source": "keyring"` on that connection in `connections.json` and remove the plaintext `password` field. `database-mcp init` (see below) offers to do this for you automatically when adding or editing a connection.

If no credential store is available (e.g. headless Linux without GNOME Keyring/KWallet running), database-mcp falls back to the plaintext `password` field and logs a warning suggesting migration.

---

## Multiple Databases Example

```json
{
  "connections": [
    {
      "id": "prod-pg",
      "driver": "postgres",
      "host": "prod.example.com",
      "port": 5432,
      "database": "myapp",
      "user": "readonly",
      "password": "",
      "ssl_mode": "require"
    },
    {
      "id": "staging-mysql",
      "driver": "mysql",
      "host": "staging.example.com",
      "port": 3306,
      "database": "myapp_staging",
      "user": "admin",
      "password": "secret"
    },
    {
      "id": "reporting-mssql",
      "driver": "sqlserver",
      "host": "reporting.example.com",
      "port": 1433,
      "database": "myapp_reporting",
      "user": "sa",
      "password": "secret",
      "ssl_mode": "true"
    }
  ]
}
```

---

## Security Notes

- `execute_query` runs every SELECT inside a read-only transaction (`postgres`/`mysql`) — no accidental writes, and the pooled connection is never left read-only afterwards
- `execute_mutation` requires explicit `confirm: true` before any write is made
- Connections marked `read_only: true` reject `execute_mutation` and `begin_transaction` outright, regardless of `confirm`
- Results are capped at 1000 rows; responses include `row_count`/`truncated` so an agent can tell a full result from a truncated one
- Every tool call is bounded by a query timeout (`MCP_DB_QUERY_TIMEOUT_SECONDS`, default 30s) so a runaway query can't hang the server
- Open transactions auto-rollback after a period of inactivity (`MCP_DB_TRANSACTION_TTL_SECONDS`, default 120s)
- Passwords can be stored in the OS credential store instead of plaintext — see [Storing passwords securely](#storing-passwords-securely)
- DSNs are built via URL/config-struct encoding (not string concatenation), so passwords containing spaces, quotes, or `=` can't inject extra connection options
- Connection pools default to 5 max-open / 2 max-idle connections per connection with a 30-minute max lifetime, so one agent can't exhaust the database's connection slots
- The audit log is created `0600` (owner-only) since it contains full query text
- All tool calls are written to the audit log

---

## Development

Run the unit tests:

```bash
go test ./...
```

Integration tests run the tool handlers against real Postgres, MySQL, and SQL Server instances via Docker Compose:

```bash
make integration
```

This starts the databases, runs `go test -tags integration ./integration/...`, and tears them down. It verifies things a mock can't — that the read-only guard actually blocks a write at the database, that a `tx_id` transaction really commits/rolls back rows, and that `explain_query` works against all three engines.

Check the installed version:

```bash
database-mcp version
```
