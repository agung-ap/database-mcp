# db-mcp

A Model Context Protocol (MCP) server that gives AI agents safe, structured access to PostgreSQL and MySQL databases. Supports read-only queries, guarded mutations, schema inspection, table statistics, and multi-statement transactions.

## Quick Install

```bash
go install github.com/agung-ap/database-mcp@latest
```

Or build from source and install to your local bin:

```bash
git clone https://github.com/agp/db-mcp
cd db-mcp
go build -o ~/.local/bin/db-mcp .
```

## Quick Setup

Run the interactive wizard to configure your databases and supported agents:

```bash
db-mcp init
```

The wizard will:
1. Create `~/.config/db-mcp/connections.json` with your database connections
2. Optionally configure **Claude Code (CLI)**, **Claude Desktop (GUI)**, **OpenCode**, or **GitHub Copilot in VS Code**

## Supported Agents

| Agent | How `init` configures it |
|-------|--------------------------|
| Claude Code (CLI) | Runs `claude mcp add --scope user` (user-scoped, works in all projects) |
| Claude Desktop (GUI) | Writes `~/.claude/claude_desktop_config.json` |
| OpenCode | Writes `~/.config/opencode/opencode.json` |
| GitHub Copilot in VS Code | Writes `.vscode/mcp.json` in current directory |

## Manual Configuration

Config is stored at:
- **Linux / WSL / macOS**: `~/.config/db-mcp/connections.json`
- **Windows**: `%APPDATA%\db-mcp\connections.json`

Override with `MCP_DB_CONFIG_PATH=/path/to/connections.json`.

Example `connections.json`:

```json
{
  "connections": [
    {
      "id": "prod-pg",
      "driver": "postgres",
      "host": "localhost",
      "port": 5432,
      "database": "myapp",
      "user": "readonly",
      "password": "",
      "ssl_mode": "require",
      "pool": { "max_open": 10, "max_idle": 5, "conn_max_lifetime_minutes": 30 }
    },
    {
      "id": "staging-mysql",
      "driver": "mysql",
      "host": "localhost",
      "port": 3306,
      "database": "myapp_staging",
      "user": "admin",
      "password": "",
      "ssl_mode": "preferred"
    }
  ]
}
```

Set passwords via environment variables:
```bash
export MCP_DB_CONN_PROD_PG_DSN="host=localhost port=5432 dbname=myapp user=readonly password=secret sslmode=require"
```

### Manual agent config (Claude Code CLI)

```bash
claude mcp add db-mcp /path/to/db-mcp \
  --scope user \
  --env MCP_DB_CONFIG_PATH=~/.config/db-mcp/connections.json
```

### Manual agent config (Claude Desktop GUI)

Add to `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "db-mcp": {
      "command": "/path/to/db-mcp",
      "args": [],
      "env": {
        "MCP_DB_CONFIG_PATH": "/home/you/.config/db-mcp/connections.json"
      }
    }
  }
}
```

### Manual agent config (VS Code / Copilot)

Add `.vscode/mcp.json` to your workspace:

```json
{
  "servers": {
    "db-mcp": {
      "command": "db-mcp",
      "args": [],
      "type": "stdio"
    }
  }
}
```

## Available Tools (14)

### Meta
| Tool | Description |
|------|-------------|
| `list_connections` | List all configured connection IDs and drivers |
| `test_connection` | Ping a connection and return latency |

### Schema
| Tool | Description |
|------|-------------|
| `list_databases` | List databases visible to the user |
| `list_tables` | List tables in the connected database |
| `describe_table` | Return column definitions for a table |
| `list_indexes` | List indexes on a table |
| `list_foreign_keys` | List foreign key constraints on a table |

### Query
| Tool | Description |
|------|-------------|
| `execute_query` | Execute a read-only SELECT query, returns JSON rows |
| `execute_mutation` | Execute INSERT/UPDATE/DELETE — requires `confirm: true` |
| `explain_query` | Return the execution plan for a query |

### Stats
| Tool | Description |
|------|-------------|
| `get_table_stats` | Row count, size, live/dead tuples |

### Transactions
| Tool | Description |
|------|-------------|
| `begin_transaction` | Begin a transaction, returns `tx_id` |
| `commit_transaction` | Commit by `tx_id` |
| `rollback_transaction` | Roll back by `tx_id` |

## Security

- **Parameterized queries only** — user SQL is never interpolated into strings
- **Read-only enforcement** — `execute_query` wraps every query in a read-only transaction
- **Explicit confirm gate** — `execute_mutation` requires `confirm: true` before any DB write
- **Row limit** — results capped at 1000 rows (default 100)
- **Transaction TTL** — open transactions auto-rollback after 30 seconds (configurable)
- **Audit log** — every tool call logged pre/post to `~/.config/db-mcp/audit.log`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_DB_CONFIG_PATH` | `~/.config/db-mcp/connections.json` | Config file path |
| `MCP_DB_LOG_LEVEL` | `info` | Log level: debug/info/warn/error |
| `MCP_DB_AUDIT_LOG` | `~/.config/db-mcp/audit.log` | Audit log path |
| `MCP_DB_TRANSACTION_TTL_SECONDS` | `30` | Transaction auto-rollback TTL |
| `MCP_DB_CONN_<ID>_DSN` | — | DSN override for connection `<ID>` |

## Development

```bash
go build .                         # build binary
go test ./...                      # run unit tests
go test ./... -tags=integration    # run integration tests (needs Docker)
go vet ./...                       # static analysis
```
