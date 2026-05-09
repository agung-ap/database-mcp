# db-mcp — Database MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server written in Go that gives AI agents (Claude, Claude Code, and any MCP-compatible client) safe, structured access to PostgreSQL and MySQL databases.

---

## Features

- 14 tools covering schema inspection, read-only queries, guarded mutations, and transaction management
- Mandatory `confirm: true` gate on all write operations
- Read-only transaction enforcement for every `execute_query` call
- Server-side row limit clamping (max 1 000 rows)
- TTL-based auto-rollback for open transactions
- JSON-lines audit log written before and after every tool call
- Stdio transport — works as a subprocess for any MCP client

---

## Quick Start

### 1. Build

```bash
make build          # produces bin/mcp-db-server
# or
make check          # fmt + vet + lint + build in one command
```

### 2. Configure connections

Copy and edit `config/connections.yaml`:

```yaml
connections:
  - id: prod-pg
    driver: postgres
    host: localhost
    port: 5432
    database: mydb
    user: readonly_user
    password: ${PG_PASSWORD}
    ssl_mode: require
    pool:
      max_open: 10
      max_idle: 5
      conn_max_lifetime_minutes: 30

  - id: staging-mysql
    driver: mysql
    host: localhost
    port: 3306
    database: app_staging
    user: admin_user
    password: ${MYSQL_PASSWORD}
    ssl_mode: preferred
    pool:
      max_open: 5
      max_idle: 2
      conn_max_lifetime_minutes: 15
```

### 3. Configure `.mcp.json` for Claude Code / Claude Desktop

The `.mcp.json` file in your project root (or `~/.claude/claude_desktop_config.json` for Claude Desktop) tells Claude where to find the MCP server and how to connect to your databases.

#### Production Configuration

Add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "db-server": {
      "command": "/absolute/path/to/bin/mcp-db-server",
      "args": [],
      "env": {
        "MCP_DB_CONFIG_PATH": "/absolute/path/to/config/connections.yaml",
        "MCP_DB_AUDIT_LOG": "/absolute/path/to/audit.log",
        "MCP_DB_LOG_LEVEL": "info",
        "MCP_DB_CONN_PROD_PG_DSN": "postgres://user:pass@host:5432/dbname?sslmode=require",
        "MCP_DB_CONN_STAGING_MYSQL_DSN": "user:pass@tcp(host:3306)/dbname"
      }
    }
  }
}
```

**Configuration Details:**
- **`command`** — Absolute path to the compiled `bin/mcp-db-server` binary
- **`args`** — Leave empty (not used by this server)
- **`env`** — Environment variables passed to the subprocess:
  - `MCP_DB_CONFIG_PATH` — Path to `connections.yaml` (defines connection pools)
  - `MCP_DB_AUDIT_LOG` — Path to audit log file (JSON-lines format)
  - `MCP_DB_LOG_LEVEL` — Server log level: `debug`, `info`, `warn`, `error`
  - `MCP_DB_CONN_<ID>_DSN` — **DSN override** for connection `<id>` (takes priority over config file)

**Why use DSN overrides?**
- Keep credentials out of version control by using environment-specific `.mcp.json` files
- `<ID>` should be uppercased with `-` replaced by `_` (e.g., `prod-pg` → `PROD_PG`)

#### Local Development Configuration

For local testing with `make docker-up` (PostgreSQL on 5433, MySQL on 3307):

```json
{
  "mcpServers": {
    "db-server": {
      "command": "./bin/mcp-db-server",
      "args": [],
      "env": {
        "MCP_DB_CONFIG_PATH": "./config/connections.yaml",
        "MCP_DB_AUDIT_LOG": "./audit.log",
        "MCP_DB_LOG_LEVEL": "debug",
        "MCP_DB_CONN_PROD_PG_DSN": "postgres://testuser:testpass@localhost:5433/test_mcp?sslmode=disable",
        "MCP_DB_CONN_STAGING_MYSQL_DSN": "testuser:testpass@tcp(localhost:3307)/test_mcp"
      }
    }
  }
}
```

#### Claude Desktop Configuration

Add the same `"db-server"` block under `"mcpServers"` in `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "db-server": {
      "command": "/absolute/path/to/bin/mcp-db-server",
      "args": [],
      "env": {
        "MCP_DB_CONFIG_PATH": "/absolute/path/to/config/connections.yaml",
        "MCP_DB_AUDIT_LOG": "/absolute/path/to/audit.log"
      }
    }
  }
}
```

**After configuration:**
- Save the `.mcp.json` file
- **Restart Claude Code/Claude Desktop** for changes to take effect
- Claude will auto-discover all 14 tools on the next session start — no prompting needed

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `MCP_DB_CONFIG_PATH` | `./config/connections.yaml` | Path to connections config |
| `MCP_DB_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `MCP_DB_AUDIT_LOG` | `./audit.log` | Audit log file path |
| `MCP_DB_TRANSACTION_TTL_SECONDS` | `30` | Auto-rollback TTL for open transactions |
| `MCP_DB_CONN_<ID>_DSN` | — | DSN override for connection `<id>` (takes priority over config file, `<ID>` is uppercased with `-` replaced by `_`) |

Example DSN overrides:

```bash
MCP_DB_CONN_PROD_PG_DSN=postgres://user:pass@host:5432/dbname?sslmode=require
MCP_DB_CONN_STAGING_MYSQL_DSN=user:pass@tcp(host:3306)/dbname
```

---

## Available Tools

### Meta

#### `list_connections`
Returns all configured connection IDs and their drivers.

```json
{}
```

#### `test_connection`
Pings a connection and returns latency.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres"
}
```

---

### Schema Inspection

#### `list_databases`
Lists databases/schemas visible to the connected user.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres"
}
```

#### `list_tables`
Lists tables in the connected database.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres",
  "database": "mydb"
}
```

#### `describe_table`
Returns column definitions for a table.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres",
  "table": "users",
  "schema": "public"
}
```

#### `list_indexes`
Returns indexes on a table.

```json
{
  "connection_id": "staging-mysql",
  "driver": "mysql",
  "table": "orders"
}
```

#### `list_foreign_keys`
Returns foreign key constraints for a table.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres",
  "table": "order_items",
  "schema": "public"
}
```

---

### Query Execution

#### `execute_query`
Runs a read-only SELECT. Wrapped in a read-only transaction — writes are rejected at the DB level.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres",
  "query": "SELECT id, email FROM users WHERE created_at > $1",
  "params": ["2024-01-01"],
  "limit": 50
}
```

- `limit` defaults to 100, maximum 1 000 — clamped server-side
- Returns a JSON array of row objects

#### `execute_mutation`
Runs INSERT / UPDATE / DELETE. **Requires `confirm: true`** — without it the request is rejected before any DB call is made.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres",
  "query": "UPDATE users SET status = $1 WHERE id = $2",
  "params": ["active", 42],
  "confirm": true
}
```

Returns `{"rows_affected": N}`.

#### `explain_query`
Returns the query plan without executing the query.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres",
  "query": "SELECT * FROM orders JOIN users ON orders.user_id = users.id",
  "params": []
}
```

---

### Table Statistics

#### `get_table_stats`
Returns row count, size, live/dead tuple counts, and last analyzed time.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres",
  "table": "users",
  "schema": "public"
}
```

---

### Transactions

Transactions are tracked server-side with a TTL (default 30 s). Expired transactions are auto-rolled back.

#### `begin_transaction`
Opens a transaction and returns a `tx_id`.

```json
{
  "connection_id": "prod-pg",
  "driver": "postgres"
}
```

Returns `{"tx_id": "uuid..."}`.

#### `commit_transaction`

```json
{ "tx_id": "uuid..." }
```

#### `rollback_transaction`

```json
{ "tx_id": "uuid..." }
```

---

## Using with Claude / Claude Code

### Setup Steps

1. **Build the server:**
   ```bash
   make build
   ```

2. **Create or edit your project's `.mcp.json`** (see [Configure `.mcp.json`](#3-configure-mcpjson-for-claude-code--claude-desktop) section above)

3. **Start Claude Code / Claude Desktop** with the project folder open
   - Claude automatically discovers all 14 tools from the MCP server
   - No additional prompting or configuration needed

### Using Tools in Claude

Once configured, Claude can call these tools naturally through conversation:

```
"List all tables in the prod-pg connection"
↓
Claude calls list_tables with connection_id=prod-pg

"Show me the schema for the orders table"
↓
Claude calls describe_table with table=orders

"How many active users are there?"
↓
Claude calls execute_query with SELECT count(*) FROM users WHERE active = true

"Archive inactive users from 2023"
↓
Claude calls execute_mutation with confirm=true (requires explicit human confirmation in the prompt)
```

### Safety Guarantees

✅ **Read operations are safe** — `execute_query` is wrapped in read-only transactions at the DB level  
✅ **Write operations require confirmation** — `execute_mutation` gates on `confirm: true`  
✅ **Audit trail** — Every tool call is logged with timestamp, user intent, parameters, and outcome  
✅ **Parameterized queries** — All user input is passed via bind parameters (no SQL injection)  
✅ **Row limits** — Results capped at 1,000 rows server-side

---

## Development

```bash
make test            # run unit tests
make test-race       # run with race detector
make docker-up       # start Postgres + MySQL test containers
make test-integration  # run integration tests (requires docker-up)
make test-coverage   # generate coverage.html
make fmt             # gofmt
make vet             # go vet
make lint            # golangci-lint
make check           # fmt + vet + lint + build in one command
make clean           # remove bin/ and coverage files
```

### Test seed data

Integration tests use:
- `testdata/postgres/seed.sql`
- `testdata/mysql/seed.sql`

Schema: `test_mcp` — tables: `users`, `orders`, `order_items`, `products`

---

## Security Notes

- **All user-supplied SQL parameters are passed via parameterized queries** — no string interpolation
- **`execute_query` is enforced read-only** at the transaction level (not just by trusting the SQL)
- **`execute_mutation` requires explicit `confirm: true`** — AI agents cannot accidentally mutate data
- **Audit log** records every tool call with intent (before) and outcome (after)
- Run the server with a read-only DB user for `execute_query` workloads; use a separate, more privileged user only for mutation connections
