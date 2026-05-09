# Task Plan: MCP Database Server Dependency Cleanup

**Goal**: Remove unused dependencies, replace community SDK with the official one, and slim the binary.  
**Target project**: `/Users/mekari/Documents/golang/database-mcp` (Go project)  
**Current binary**: `bin/mcp-db-server` — **14 MB**

---

## Audit Summary

Cross-referencing `go.mod` declarations against actual usage in `cmd/` and `internal/`:

| Dependency | Declared | Actually Used In Code | Action |
|---|---|---|---|
| `github.com/mark3labs/mcp-go` | ✅ | `internal/mcp/*.go` + all 11 tool files (163 usages) | **REPLACE** → official SDK |
| `github.com/lib/pq` | ✅ | `internal/db/postgres/driver.go` | **KEEP** |
| `github.com/go-sql-driver/mysql` | ✅ | `internal/db/mysql/driver.go` | **KEEP** |
| `github.com/google/uuid` | ✅ | `internal/tools/query.go` (query IDs) | **KEEP** |
| `github.com/spf13/viper` | ✅ | `internal/config/loader.go` | **REMOVE** → stdlib JSON |
| `github.com/stretchr/testify` | ✅ | 15 test files (`*_test.go`) | **KEEP** (test-only, no binary impact) |
| `github.com/AlecAivazis/survey/v2` | ✅ | `internal/setup/database_prompts.go` | **REMOVE** → stdlib stdin |

---

## Why stdlib is NOT enough for MCP

MCP (Model Context Protocol) is a **complex JSON-RPC 2.0 protocol** requiring:
- Message framing and session lifecycle management
- Tool schema declaration and request dispatching
- Stdio/SSE transport handling
- Protocol versioning (spec: `2025-11-25`)

**Stdlib provides none of this.** You always need an MCP SDK. The question is: *which one?*

---

## mark3labs/mcp-go vs Official SDK

| | `mark3labs/mcp-go` v0.18.0 (current) | `modelcontextprotocol/go-sdk` v1.6.0 (official) |
|---|---|---|
| **Maintained by** | Community (Ed Zynda) | **Anthropic + Google** ✅ |
| **Released** | Community project | **Official — May 8, 2026** ✅ |
| **MCP Spec** | v2024-11-05 | **v2025-11-25 (latest)** ✅ |
| **API style** | Builder pattern, verbose, stringly-typed | **Generics + struct tags, fully type-safe** ✅ |
| **Schema generation** | Manual (`WithString()`, `WithNumber()`, etc.) | **Auto-generated from struct tags** ✅ |
| **Input parsing** | Manual `req.GetString("field")` | **Automatic struct unmarshaling** ✅ |
| **Import path** | `github.com/mark3labs/mcp-go/mcp` | `github.com/modelcontextprotocol/go-sdk/mcp` ✅ |

> The official SDK README explicitly acknowledges `mcp-go` as the inspiration and predecessor.

---

## API Comparison

```go
// ❌ CURRENT: mark3labs/mcp-go — verbose, stringly-typed, manual schema
srv.AddTool(mcpgo.NewTool(
    "test_connection",
    mcpgo.WithDescription("Ping a database connection and return latency"),
    mcpgo.WithString("connection_id",
        mcpgo.Required(),
        mcpgo.Description("Named connection alias from config"),
    ),
    mcpgo.WithString("driver",
        mcpgo.Required(),
        mcpgo.Description("Database driver"),
    ),
), server.ToolHandlerFunc(testConn.Handle))

// handler signature
func (h *TestConnectionHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    connID := req.Params.Arguments["connection_id"].(string) // unsafe type assertion
    driver  := req.Params.Arguments["driver"].(string)       // can panic!
    ...
}

// ✅ OFFICIAL: modelcontextprotocol/go-sdk — type-safe, auto-schema, clean
type TestConnectionInput struct {
    ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
    Driver       string `json:"driver"        jsonschema:"Database driver"`
}

mcp.AddTool(server, &mcp.Tool{
    Name:        "test_connection",
    Description: "Ping a database connection and return latency",
}, func(ctx context.Context, req *mcp.CallToolRequest, in TestConnectionInput) (*mcp.CallToolResult, any, error) {
    // in.ConnectionID and in.Driver — fully typed, validated, no panics
})
```

---

## Before / After (`go.mod` diff)

```go
// BEFORE
require (
    github.com/AlecAivazis/survey/v2 v2.3.7       // ← Phase 1: REMOVE
    github.com/go-sql-driver/mysql v1.9.3
    github.com/google/uuid v1.6.0
    github.com/lib/pq v1.10.9
    github.com/mark3labs/mcp-go v0.18.0            // ← Phase 3: REPLACE
    github.com/spf13/viper v1.18.2                 // ← Phase 2: REMOVE
    github.com/stretchr/testify v1.9.0
)

// AFTER (all phases complete)
require (
    github.com/go-sql-driver/mysql v1.9.3
    github.com/google/uuid v1.6.0
    github.com/lib/pq v1.10.9
    github.com/modelcontextprotocol/go-sdk v1.6.0  // ← official SDK
    github.com/stretchr/testify v1.9.0
)
// NOTE: encoding/json is stdlib (free). yaml.v3 stays as transitive from go-sdk/testify.
```

---

## Phased Implementation Plan

### Phase 1: Remove `survey/v2` — stdin Prompts

**Files affected**: `internal/setup/database_prompts.go`

**Changes**:
1. Replace `survey.Input{}`, `survey.Select{}`, `survey.Confirm{}` with `bufio.Scanner` + `fmt.Fprintf`
2. Remove `github.com/AlecAivazis/survey/v2` from `go.mod`
3. Run `go mod tidy`

**Dropped transitive deps**: `Netflix/go-expect`, `creack/pty`, `hinshun/vt10x`, `kballard/go-shellquote`

**Expected impact**:
- Binary size: **−1.2 MB**
- Build time: **−1-2s**
- Risk: **LOW** — setup wizard UX change only; core MCP server unaffected

---

### Phase 2: Remove `viper` — JSON Config via stdlib

**Files affected**: `internal/config/loader.go`, `config/connections.yaml`

**Changes**:
1. Rename `config/connections.yaml` → `config/connections.json`
2. Update struct tags: `yaml:"..."` → `json:"..."`
3. Replace viper loader with `os.ReadFile` + `json.Unmarshal`
4. Remove `github.com/spf13/viper` from `go.mod`
5. Run `go mod tidy`

**Code change**:
```go
// BEFORE (viper)
viper.SetConfigFile(cfgPath)
viper.ReadInConfig()
viper.Unmarshal(&cfg)

// AFTER (stdlib — zero deps)
data, err := os.ReadFile(cfgPath)
json.Unmarshal(data, &cfg)
```

**Config format change**:
```yaml
# BEFORE (connections.yaml)
connections:
  - id: prod-pg
    driver: postgres
    host: localhost
    port: 5432
```
```json
// AFTER (connections.json)
{
  "connections": [
    { "id": "prod-pg", "driver": "postgres", "host": "localhost", "port": 5432 }
  ]
}
```

**Dropped transitive deps**: `spf13/cast`, `spf13/afero`, `spf13/pflag`, `hashicorp/hcl`, `magiconair/properties`, `mitchellh/mapstructure`, `sagikazarmark/locafero`, `sourcegraph/conc`, `fsnotify`, and more (~12 indirect deps)

**Expected impact**:
- Binary size: **−800 KB**
- Build time: **−1s**
- Risk: **LOW** — stdlib JSON is simpler and faster than viper; struct tags trivial to update

---

### Phase 3: Replace `mark3labs/mcp-go` → Official SDK ⭐ (Highest Value)

**Files affected**: `internal/mcp/server.go`, `internal/mcp/tools.go`, all 11 tool handler files (163 total usages)

**Why this matters most**:
- `mark3labs/mcp-go` is now a **legacy community project** — the official SDK is its successor
- Official SDK is **type-safe** via generics — eliminates unsafe `.(string)` type assertions that can panic
- **Auto-schema generation** — removes ~300 lines of `WithString()`/`WithNumber()` boilerplate from `tools.go`
- Stays current with **MCP spec v2025-11-25** (mark3labs is on v2024-11-05)

**Changes**:

#### 3a. Update `go.mod`
```bash
go get github.com/modelcontextprotocol/go-sdk@latest
go mod edit -droprequire github.com/mark3labs/mcp-go
go mod tidy
```

#### 3b. Rewrite `internal/mcp/server.go`
```go
// BEFORE
import "github.com/mark3labs/mcp-go/server"
s := server.NewMCPServer("db-server", "1.0.0")
s.ServeStdio()

// AFTER
import "github.com/modelcontextprotocol/go-sdk/mcp"
s := mcp.NewServer(&mcp.Implementation{Name: "db-server", Version: "1.0.0"}, nil)
s.Run(ctx, &mcp.StdioTransport{})
```

#### 3c. Rewrite `internal/mcp/tools.go` — Tool Registration
```go
// BEFORE (~50 lines per tool, stringly-typed)
srv.AddTool(mcpgo.NewTool(
    "execute_query",
    mcpgo.WithDescription("Execute a read-only SQL query"),
    mcpgo.WithString("connection_id", mcpgo.Required(), mcpgo.Description("...")),
    mcpgo.WithString("driver",        mcpgo.Required(), mcpgo.Description("...")),
    mcpgo.WithString("query",         mcpgo.Required(), mcpgo.Description("...")),
    mcpgo.WithNumber("limit",                           mcpgo.Description("...")),
), server.ToolHandlerFunc(h.ExecuteQuery))

// AFTER (~5 lines per tool, auto-schema from struct)
mcp.AddTool(server, &mcp.Tool{
    Name:        "execute_query",
    Description: "Execute a read-only SQL query",
}, h.ExecuteQuery) // input type inferred from generic
```

#### 3d. Rewrite each tool handler — typed inputs
```go
// BEFORE — unsafe string assertions, can panic
func (h *QueryHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    connID := req.Params.Arguments["connection_id"].(string)
    query  := req.Params.Arguments["query"].(string)
    limit  := int(req.Params.Arguments["limit"].(float64))
    ...
}

// AFTER — fully typed, validated by SDK before handler is called
type ExecuteQueryInput struct {
    ConnectionID string `json:"connection_id" jsonschema:"Named connection alias from config"`
    Driver       string `json:"driver"        jsonschema:"Database driver (postgres|mysql)"`
    Query        string `json:"query"         jsonschema:"SQL query to execute"`
    Limit        int    `json:"limit"         jsonschema:"Max rows to return (default 100)"`
}

func (h *QueryHandler) Execute(ctx context.Context, req *mcp.CallToolRequest, in ExecuteQueryInput) (*mcp.CallToolResult, any, error) {
    // in.ConnectionID, in.Query, in.Limit — all typed, never panic
    ...
}
```

**Dropped dependency**: `github.com/mark3labs/mcp-go` (entire package)

**Expected impact**:
- Binary size: **−300-500 KB**
- Build time: **−1s**
- Code reduction: **~300+ lines** removed from tools.go boilerplate
- Safety improvement: **eliminates all type-assertion panics** on tool inputs
- Risk: **MEDIUM** — 163 usages to update; bounded, mechanical changes

---

## Expected Outcome

| Metric | Before | After Phase 1 | After Phase 1+2 | After All 3 Phases |
|---|---|---|---|---|
| Binary size | **14 MB** | ~12.8 MB | ~12 MB | **~11.5 MB** |
| Direct deps | 7 | 6 | 5 | **5** |
| MCP spec version | 2024-11-05 | 2024-11-05 | 2024-11-05 | **2025-11-25** ✅ |
| Type-safe tool inputs | ❌ | ❌ | ❌ | **✅** |
| Auto schema gen | ❌ | ❌ | ❌ | **✅** |
| Build time (clean) | ~8s | ~6-7s | ~5-6s | **~4-5s** |

---

## Verification Steps

**Phase 1**:
- `go mod tidy && go mod verify`
- `make build` — must compile
- `make test` — all tests pass
- Smoke: `./bin/mcp-db-server setup` — prompts work with stdin

**Phase 2**:
- `make build && make test`
- Rename config: `mv config/connections.yaml config/connections.json`
- Smoke: `./bin/mcp-db-server status` — config loads correctly

**Phase 3**:
- `go mod tidy && go mod verify`
- `make build` — no compilation errors
- `make test` — all 15 test files pass
- Integration: `make docker-up && make test-integration` — all 14 tools work correctly
- MCP protocol smoke test: connect via Claude Desktop and verify tool listing

---

## Risk Assessment

| Phase | Change | Risk | Mitigation |
|-------|--------|------|-----------|
| 1 | Remove survey/v2 | **LOW** | Only setup wizard UX; core server unaffected |
| 2 | viper → stdlib JSON | **LOW** | stdlib JSON is simpler; struct tags trivially updated |
| 3 | mark3labs → official SDK | **MEDIUM** | 163 usages; bounded mechanical changes; full test suite validates |

---

## Notes

- **Why not stdlib for MCP?** MCP is a stateful JSON-RPC 2.0 protocol with tool registration, schema validation, and session lifecycle. Stdlib has none of this.
- **Phase 3 is the most valuable** — it future-proofs the project with the officially maintained SDK, eliminates type-assertion panics, and removes ~300 lines of boilerplate
- **yaml.v3** stays as a transitive dep from `go-sdk` and `testify` — cannot be fully eliminated without removing both
- **testify** stays — test-only, zero binary impact, improves test readability
- All phases are independent and can be shipped separately
