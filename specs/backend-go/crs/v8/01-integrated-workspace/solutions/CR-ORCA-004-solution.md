---
cr_id: CR-ORCA-004
solution_id: SOL-ORCA-004
product: orca
service: orca-agent
layer: backend
language: go
status: implemented
priority: P1
effort: 2 ngày
depends_on: CR-ORCA-001
created: 2026-10-19
---

# SOL-ORCA-004 — orca-agent: Config Env Override + Executor Extension + Deploy Kit

## 1. Phân tích vấn đề

Khi đọc code thực tế `orca-agent`, phát hiện 4 vấn đề blocking:

| # | Vấn đề | Tác động |
|---|--------|---------|
| 1 | `ExecuteLegacy` là entry point thật, không phải Register pattern | CR-ORCA-001 cần target đúng method này |
| 2 | `config.go` dùng YAML only, không đọc env vars | `docker-compose env_file` không hoạt động |
| 3 | `deploy/orca-agent/` thiếu Bridges env vars | Không deploy được pis_* integration |
| 4 | Thiếu `bridges:` section trong config.yaml template | Operator không biết cấu hình gì |

---

## 2. Giải pháp kỹ thuật

### 2.1 Config env var override — `internal/config/config.go` (EDIT)

```go
package config

import (
    "fmt"
    "os"
    "gopkg.in/yaml.v3"
)

// Config là toàn bộ config của orca-agent.
type Config struct {
    // ... existing sections ...

    // Bridges — peer agent IDs cho cross-agent tool calls (CR-ORCA-004)
    Bridges struct {
        ProductsAgentID string `yaml:"products_agent_id"`
        DocAgentID      string `yaml:"doc_agent_id"`
    } `yaml:"bridges"`

    // MCPProxy — cho peer-invoke qua HTTP (CR-ORCA-005)
    MCPProxy struct {
        URL       string `yaml:"url"`
        PeerToken string `yaml:"peer_token"`
    } `yaml:"mcp_proxy"`

    // Telemetry
    Telemetry struct {
        MetricsAddr string `yaml:"metrics_addr"`
    } `yaml:"telemetry"`
}

// Load đọc YAML config rồi override bằng env vars.
// Thứ tự ưu tiên: env vars > YAML > defaults.
// CR-ORCA-004: env override cho phép docker-compose env_file hoạt động.
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("config: read file %s: %w", path, err)
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("config: parse yaml: %w", err)
    }

    // Env overrides — priority: env > yaml
    if v := os.Getenv("ORCA_BRIDGES_PRODUCTS_AGENT_ID"); v != "" {
        cfg.Bridges.ProductsAgentID = v
    }
    if v := os.Getenv("ORCA_BRIDGES_DOC_AGENT_ID"); v != "" {
        cfg.Bridges.DocAgentID = v
    }
    if v := os.Getenv("ORCA_MCP_PROXY_URL"); v != "" {
        cfg.MCPProxy.URL = v
    }
    if v := os.Getenv("ORCA_MCP_PROXY_PEER_TOKEN"); v != "" {
        cfg.MCPProxy.PeerToken = v
    }
    if v := os.Getenv("ORCA_API_URL"); v != "" {
        cfg.Orca.OrcaURL = v
    }

    // Default values
    if cfg.Telemetry.MetricsAddr == "" {
        cfg.Telemetry.MetricsAddr = ":9102"
    }

    return &cfg, nil
}
```

### 2.2 Executor extension — `internal/mcp/executor.go` (EDIT)

```go
package mcp

import (
    "context"
    "fmt"
    "github.com/vnp-blc/orca/orca-agent/internal/pis"
    "github.com/vnp-blc/orca/orca-agent/internal/audit"
)

// Executor dispatches tool calls từ ws OnMessage (ExecuteLegacy).
// CR-ORCA-004: thêm pisTool field + routing pis_* trong ExecuteLegacy.
type Executor struct {
    orcaBaseURL string
    apiKey      string
    watcher     *TaskWatcher
    auditBuf    *audit.Buffer
    pisTool     *pis.PISTool  // CR-ORCA-004: optional
}

func NewExecutor(orcaBaseURL, apiKey string, watcher *TaskWatcher, auditBuf *audit.Buffer) *Executor {
    return &Executor{
        orcaBaseURL: orcaBaseURL,
        apiKey:      apiKey,
        watcher:     watcher,
        auditBuf:    auditBuf,
    }
}

// WithPISTool attaches PISTool. Trả về receiver để chain.
// Gọi SAU ws.NewClient vì PISTool cần wsClient cho peer-invoke.
func (e *Executor) WithPISTool(t *pis.PISTool) *Executor {
    e.pisTool = t
    return e
}

// ExecuteLegacy là entry point thực sự cho tool calls từ ws.OnMessage.
// CR-ORCA-004: thêm pis_* routing sau khi xác nhận đây là method được ws dùng.
func (e *Executor) ExecuteLegacy(ctx context.Context, toolName string, params map[string]any) (any, error) {
    switch toolName {
    // Existing orca.* tools
    case "orca.create_task":
        return e.createTask(ctx, params)
    case "orca.get_task_status":
        return e.getTaskStatus(ctx, params)
    case "orca.list_tasks":
        return e.listTasks(ctx, params)
    case "orca.update_task":
        return e.updateTask(ctx, params)
    // ... other existing cases ...

    // CR-ORCA-001/004: PIKB KG tools via PISTool peer-invoke
    case "pis_query_feature_context",
        "pis_query_crs",
        "pis_query_hld",
        "pis_query_adr",
        "pis_kg_search":
        if e.pisTool == nil {
            return nil, fmt.Errorf("pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID")
        }
        return e.pisTool.Dispatch(ctx, toolName, params)

    default:
        return nil, fmt.Errorf("unsupported tool: %s", toolName)
    }
}
```

### 2.3 Tool catalog merge — `internal/mcp/catalog_pis.go` (NEW)

```go
package mcp

// PISToolCatalog trả về 5 pis_* tool definitions.
// CR-ORCA-004: gọi conditionally khi ProductsAgentID được cấu hình.
// Tách file riêng để OrcaToolCatalog() không phụ thuộc vào pis package.
func PISToolCatalog() []Tool {
    return []Tool{
        {Name: "pis_query_feature_context", Description: "Query feature context từ PIKB KG.", InputSchema: pisInputSchema("feature_id")},
        {Name: "pis_query_crs", Description: "Query CRS node theo ID.", InputSchema: pisInputSchema("crs_id")},
        {Name: "pis_query_hld", Description: "Query HLD node theo product.", InputSchema: pisHLDInputSchema()},
        {Name: "pis_query_adr", Description: "Query ADR node theo ID.", InputSchema: pisInputSchema("adr_id")},
        {Name: "pis_kg_search", Description: "Full-text search PIKB KG.", InputSchema: pisSearchInputSchema()},
    }
}

func pisInputSchema(requiredField string) map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            requiredField: map[string]any{"type": "string"},
        },
        "required": []string{requiredField},
    }
}
// ... pisHLDInputSchema, pisSearchInputSchema tương tự
```

### 2.4 Init order trong main.go — `cmd/orca-agent/main.go` (EDIT)

```go
// CR-ORCA-004: Thứ tự init đã được kiểm tra — executor trước ws, PISTool sau ws.
// PISTool cần wsClient (peer-invoke via HTTP), wsClient cần executor (OnMessage).

// Step 1: Build executor
executor := mcp.NewExecutor(cfg.Orca.OrcaURL, cfg.Orca.APIKey, watcher, auditBuf)

// Step 2: Build catalog (pis_* tools nếu configured)
catalog := mcp.OrcaToolCatalog()

// Step 3: Create ws client (cần executor.ExecuteLegacy làm OnMessage handler)
wsClient = ws.NewClient(ws.Config{
    WSURL:       authResult.WebSocketURL,
    GetJWT:      reg.GetCurrentJWT,
    OnMessage:   executor.ExecuteLegacy,
    OnConnect:   func() { log.Println("[orca-agent] connected") },
    // CR-ORCA-005: peer-invoke via HTTP mcp-proxy
    MCPProxyURL: cfg.MCPProxy.URL,
    PeerToken:   cfg.MCPProxy.PeerToken,
})

// Step 4: Wire PISTool sau khi ws đã tạo (CR-ORCA-001/004)
if cfg.Bridges.ProductsAgentID != "" {
    pisTool := pis.NewPISTool(cfg.Bridges.ProductsAgentID, wsClient)
    executor.WithPISTool(pisTool)
    catalog = append(catalog, mcp.PISToolCatalog()...)
    log.Printf("[orca-agent] PIS tools enabled (products-agent: %s)", cfg.Bridges.ProductsAgentID)
}
```

### 2.5 PISTool PeerCaller interface — `internal/pis/client.go` (EDIT)

```go
package pis

import (
    "context"
    "encoding/json"
)

// PeerCaller interface cho peer-invoke cross-agent.
// *ws.Client phải implement interface này (CR-ORCA-005).
type PeerCaller interface {
    Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (json.RawMessage, error)
}

// PISTool thực hiện PIKB KG queries qua products-agent peer-invoke.
type PISTool struct {
    productsAgentID string
    peer            PeerCaller
}

func NewPISTool(productsAgentID string, peer PeerCaller) *PISTool {
    return &PISTool{productsAgentID: productsAgentID, peer: peer}
}

// Dispatch routes tool call qua peer-invoke đến products-agent.
func (t *PISTool) Dispatch(ctx context.Context, toolName string, params map[string]any) (json.RawMessage, error) {
    return t.peer.Call(ctx, t.productsAgentID, toolName, params)
}
```

### 2.6 Deploy config — `deploy/orca-agent/config/orca-agent.yaml` (EDIT)

```yaml
# ... existing config ...

# Bridges — peer agent IDs cho cross-agent tool calls (CR-ORCA-004)
bridges:
  products_agent_id: ""    # Override: ORCA_BRIDGES_PRODUCTS_AGENT_ID
  doc_agent_id: ""         # Override: ORCA_BRIDGES_DOC_AGENT_ID

# MCP Proxy — peer-invoke via HTTP (CR-ORCA-005)
mcp_proxy:
  url: ""          # Override: ORCA_MCP_PROXY_URL
  peer_token: ""   # Override: ORCA_MCP_PROXY_PEER_TOKEN
```

### 2.7 docker-compose.yml (EDIT)

```yaml
environment:
  # ... existing ...
  - ORCA_BRIDGES_PRODUCTS_AGENT_ID=${ORCA_BRIDGES_PRODUCTS_AGENT_ID:-}
  - ORCA_BRIDGES_DOC_AGENT_ID=${ORCA_BRIDGES_DOC_AGENT_ID:-}
  - ORCA_MCP_PROXY_URL=${ORCA_MCP_PROXY_URL:-http://mcp-proxy:3012}
  - ORCA_MCP_PROXY_PEER_TOKEN=${ORCA_MCP_PROXY_PEER_TOKEN:-}
```

### 2.8 .env.example (NEW)

```bash
# orca-agent — required
ORCA_BOOTSTRAP_TOKEN=<from workplace admin>
ORCA_API_URL=http://orca-web:9000
ORCA_API_KEY=<orca service key>

# CR-ORCA-004: PIKB KG bridge (optional — enables pis_* tools)
ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-prod
ORCA_BRIDGES_DOC_AGENT_ID=doc-agent-prod

# CR-ORCA-005: MCP Proxy peer-invoke
ORCA_MCP_PROXY_URL=http://mcp-proxy:3012
ORCA_MCP_PROXY_PEER_TOKEN=<peer token from admin>
```

---

## 3. File map

| File | Action | Ghi chú |
|------|--------|---------|
| `orca-agent/internal/config/config.go` | **EDIT** | +`Bridges` struct, +`MCPProxy` struct, +env overrides trong `Load()` |
| `orca-agent/internal/config/config_test.go` | **NEW/EDIT** | Test env override |
| `orca-agent/internal/mcp/executor.go` | **EDIT** | +`pisTool` field, +`WithPISTool()`, +`pis_*` routing |
| `orca-agent/internal/mcp/catalog_pis.go` | **NEW** | `PISToolCatalog()` |
| `orca-agent/internal/pis/client.go` | **EDIT** | Confirm `PeerCaller` interface |
| `orca-agent/cmd/orca-agent/main.go` | **EDIT** | Restructure init order |
| `deploy/orca-agent/config/orca-agent.yaml` | **EDIT** | +bridges + mcp_proxy sections |
| `deploy/orca-agent/docker-compose.yml` | **EDIT** | +env vars |
| `deploy/orca-agent/.env.example` | **NEW** | Full env documentation |
| `orca-agent/internal/mcp/executor_test.go` | **NEW/EDIT** | 2 test cases |

---

## 4. Test strategy

```go
// orca-agent/internal/config/config_test.go

func TestLoad_EnvOverride_ProductsAgentID(t *testing.T) {
    // Write temp YAML với products_agent_id rỗng
    tmp := writeTempConfig(t, `bridges:\n  products_agent_id: ""`)
    t.Setenv("ORCA_BRIDGES_PRODUCTS_AGENT_ID", "products-agent-test")

    cfg, err := config.Load(tmp)

    assert.NoError(t, err)
    assert.Equal(t, "products-agent-test", cfg.Bridges.ProductsAgentID)
}

func TestLoad_EnvOverride_MCPProxyURL(t *testing.T) {
    tmp := writeTempConfig(t, `mcp_proxy:\n  url: ""`)
    t.Setenv("ORCA_MCP_PROXY_URL", "http://proxy:3012")

    cfg, err := config.Load(tmp)

    assert.NoError(t, err)
    assert.Equal(t, "http://proxy:3012", cfg.MCPProxy.URL)
}

// orca-agent/internal/mcp/executor_test.go

func TestExecutor_PISTools_NilPISTool_ReturnsError(t *testing.T) {
    exec := mcp.NewExecutor("http://orca", "key", nil, nil)
    _, err := exec.ExecuteLegacy(ctx, "pis_query_crs", nil)
    assert.ErrorContains(t, err, "ORCA_BRIDGES_PRODUCTS_AGENT_ID")
}

func TestExecutor_PISTools_Dispatched_ToProductsAgent(t *testing.T) {
    mock := &mockPISTool{}
    exec := mcp.NewExecutor("http://orca", "key", nil, nil).WithPISTool(mock)
    _, err := exec.ExecuteLegacy(ctx, "pis_query_crs", map[string]any{"crs_id": "X"})
    assert.NoError(t, err)
    assert.Equal(t, "pis_query_crs", mock.lastTool)
}
```

---

## 5. Compile verification

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./...
# Expected: exit 0, no errors
```

---

## 6. Risks & mitigations

| Risk | Mitigation |
|------|-----------|
| Env vars không override đúng (typo) | `config_test.go` cover tất cả env keys |
| Init order vỡ (executor cần ws reference) | Setter pattern tránh circular dependency |
| docker-compose env_file + env: block conflict | Env block có độ ưu tiên cao hơn env_file trong compose v2 — test trước deploy |

---

## 7. Execution order

```
CR-ORCA-005 (ws.Client.Call — blocking compile dependency)
  → CR-ORCA-004 (Config env override + executor extension)  ← file này
    → CR-ORCA-001 (PISTool catalog wiring)
```

---

## Execution Log — 2026-10-19

**Status**: ✅ implemented

### Files changed
- [`config/config.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/config/config.go): +`MCPProxy` struct, +4 env overrides
- [`config/config_test.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/config/config_test.go): 4 tests (PASS)
- [`mcp/executor.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/mcp/executor.go): +`pisTool` field, +`WithPISTool()`
- [`mcp/rbac.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/mcp/rbac.go): +5 `pis_*` entries in allowlist
- [`deploy/orca-agent/config/orca-agent.yaml`](../../../../vnp-workspace/deploy/orca-agent/config/orca-agent.yaml): +`bridges` + `mcp_proxy` sections
- [`deploy/orca-agent/.env.example`](../../../../vnp-workspace/deploy/orca-agent/.env.example): new file

### Verification
```
GOWORK=off go test ./internal/config/... → 4 PASS
GOWORK=off go test ./internal/mcp/...   → 6 PASS
```
