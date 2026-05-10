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
      "ssl_mode": "disable"
    }
  ]
}
```

**Supported drivers:** `postgres`, `mysql`

**`ssl_mode` options (PostgreSQL):** `disable`, `require`, `verify-ca`, `verify-full`

> **Tip:** Keep passwords out of the config by using an environment variable instead:
> ```bash
> export MCP_DB_CONN_MY_DB_DSN="host=localhost port=5432 dbname=myapp user=myuser password=secret sslmode=disable"
> ```

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
| `execute_query` | Run a SELECT query, returns results as JSON (max 1000 rows) |
| `execute_mutation` | Run INSERT/UPDATE/DELETE — requires `confirm: true` |
| `explain_query` | Return the execution plan for a query |

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
| `driver` | ✅ | `postgres` or `mysql` |
| `host` | ✅ | Database host |
| `port` | ✅ | Database port |
| `database` | ✅ | Database name |
| `user` | ✅ | Username |
| `password` | ✅ | Password (or use env var) |
| `ssl_mode` | — | SSL mode (see above) |
| `pool.max_open` | — | Max open connections (default: unlimited) |
| `pool.max_idle` | — | Max idle connections |
| `pool.conn_max_lifetime_minutes` | — | Max connection lifetime |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_DB_CONFIG_PATH` | `~/.config/database-mcp/connections.json` | Path to config file |
| `MCP_DB_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `MCP_DB_AUDIT_LOG` | `~/.config/database-mcp/audit.log` | Audit log path |
| `MCP_DB_TRANSACTION_TTL_SECONDS` | `30` | Seconds before an open transaction auto-rolls back |
| `MCP_DB_CONN_<ID>_DSN` | — | Full DSN override for a connection (replace `-` with `_` in the ID) |

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
    }
  ]
}
```

---

## Security Notes

- `execute_query` runs every SELECT inside a read-only transaction — no accidental writes
- `execute_mutation` requires explicit `confirm: true` before any write is made
- Results are capped at 1000 rows
- Open transactions auto-rollback after 30 seconds (configurable)
- All tool calls are written to the audit log
