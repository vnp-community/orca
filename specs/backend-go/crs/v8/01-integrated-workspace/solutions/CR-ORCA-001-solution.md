---
cr_id: CR-ORCA-001
solution_id: SOL-ORCA-001
product: orca
service: orca-agent
layer: backend
language: go
status: implemented
priority: P1
effort: 2–3 ngày
depends_on: CR-PIKB-001, CR-PIKB-002, CR-ORCA-004, CR-ORCA-005
created: 2026-10-19
---

# SOL-ORCA-001 — orca-agent: Expose pis_* Tools + Wire PISTool

## 1. Phân tích vấn đề

`orca-agent` đã có `internal/pis/client.go` (PISTool) và `internal/config/config.go` (`Bridges` section), nhưng PISTool chưa được kết nối vào luồng thực thi thực tế:

| Gap | Tác động |
|-----|---------|
| PISTool chưa đăng ký vào MCP executor | AI agent gọi `pis_*` → "unsupported tool" |
| `pis_*` chưa có trong tool catalog | MCP client không liệt kê tool |
| `main.go` chưa đọc `Bridges.ProductsAgentID` | PISTool không được khởi tạo |

---

## 2. Giải pháp kỹ thuật

### 2.1 Kiến trúc luồng

```
main.go
  ├─ Load config → cfg.Bridges.ProductsAgentID
  ├─ NewExecutor(...)
  ├─ ws.NewClient(...)  ← wsClient
  └─ if ProductsAgentID != "" → PISTool → executor.WithPISTool(pisTool)

Inbound AI call: pis_query_crs({crs_id: "CRS-045"})
  → ws.OnMessage → executor.ExecuteLegacy("pis_query_crs", params)
  → e.pisTool.Dispatch(ctx, "pis_query_crs", params)
  → wsClient.Call(ctx, "products-agent-prod", "pis_query_crs", params)
  → HTTP POST mcp-proxy → products-agent → pikb-catalog-service
```

### 2.2 Catalog — `internal/mcp/catalog_pis.go` (NEW)

```go
package mcp

// PISToolCatalog trả về 5 pis_* tool definitions để merge vào OrcaToolCatalog.
// CR-ORCA-001: chỉ được thêm khi ProductsAgentID được cấu hình.
func PISToolCatalog() []Tool {
    return []Tool{
        {
            Name:        "pis_query_feature_context",
            Description: "Query full feature context từ PIKB KG: HLD, CRS, ADR liên quan đến Feature ID.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "feature_id": map[string]any{"type": "string"},
                },
                "required": []string{"feature_id"},
            },
        },
        {
            Name:        "pis_query_crs",
            Description: "Query CRS node theo ID. Trả về status, layer, feature_ref, relationships với HLD/ADR.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "crs_id": map[string]any{"type": "string"},
                },
                "required": []string{"crs_id"},
            },
        },
        {
            Name:        "pis_query_hld",
            Description: "Query HLD node theo product, version, C4 level (L1–L4).",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "product_id": map[string]any{"type": "string"},
                    "version":    map[string]any{"type": "string"},
                    "level":      map[string]any{"type": "string", "description": "L1|L2|L3|L4"},
                },
                "required": []string{"product_id"},
            },
        },
        {
            Name:        "pis_query_adr",
            Description: "Query ADR node theo ID. Trả về status, supersedes, relationships.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "adr_id": map[string]any{"type": "string"},
                },
                "required": []string{"adr_id"},
            },
        },
        {
            Name:        "pis_kg_search",
            Description: "Full-text search trên Dev Knowledge KG. Filter theo type và product_id.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "q":          map[string]any{"type": "string"},
                    "type":       map[string]any{"type": "string"},
                    "product_id": map[string]any{"type": "string"},
                },
                "required": []string{"q"},
            },
        },
    }
}
```

### 2.3 Executor extension — `internal/mcp/executor.go` (EDIT)

```go
// Thêm field pisTool và method WithPISTool vào Executor struct.
// CR-ORCA-001: pisTool là optional — nil nếu ProductsAgentID không set.

type Executor struct {
    orcaBaseURL string
    apiKey      string
    watcher     *TaskWatcher
    auditBuf    *audit.Buffer
    pisTool     *pis.PISTool  // CR-ORCA-001: wired nếu ProductsAgentID configured
}

func (e *Executor) WithPISTool(t *pis.PISTool) *Executor {
    e.pisTool = t
    return e
}

// Thêm vào ExecuteLegacy() switch:
case "pis_query_feature_context",
    "pis_query_crs",
    "pis_query_hld",
    "pis_query_adr",
    "pis_kg_search":
    if e.pisTool == nil {
        return nil, fmt.Errorf("pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID")
    }
    return e.pisTool.Dispatch(ctx, toolName, params)
```

### 2.4 main.go wiring — `cmd/orca-agent/main.go` (EDIT)

```go
// Thứ tự init: executor trước ws, PISTool sau ws (PISTool cần wsClient).
executor := mcp.NewExecutor(orcaURL, orcaKey, watcher, auditBuf)
catalog := mcp.OrcaToolCatalog()

wsClient = ws.NewClient(ws.Config{...})
wsClient.SetPeerConfig(cfg.MCPProxy.URL, cfg.MCPProxy.PeerToken)

// CR-ORCA-001: Wire PISTool sau khi ws đã tạo
if cfg.Bridges.ProductsAgentID != "" {
    pisTool := pis.NewPISTool(cfg.Bridges.ProductsAgentID, wsClient)
    executor.WithPISTool(pisTool)
    catalog = append(catalog, mcp.PISToolCatalog()...)
    log.Printf("[orca-agent] PIS tools enabled (products-agent: %s)", cfg.Bridges.ProductsAgentID)
}
```

### 2.5 Deploy config — `deploy/orca-agent/.env.example`

```bash
# CR-ORCA-001: PIKB KG bridge (optional — enables pis_* tools)
ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-prod
ORCA_BRIDGES_DOC_AGENT_ID=doc-agent-prod
```

---

## 3. File map

| File | Action | Ghi chú |
|------|--------|---------|
| `orca-agent/internal/mcp/catalog_pis.go` | **NEW** | `PISToolCatalog()` — 5 tool defs |
| `orca-agent/internal/mcp/executor.go` | **EDIT** | `+pisTool` field, `+WithPISTool()`, `+pis_*` routing |
| `orca-agent/internal/mcp/executor_test.go` | **NEW/EDIT** | 2 test cases |
| `orca-agent/cmd/orca-agent/main.go` | **EDIT** | Wire PISTool sau ws init |
| `deploy/orca-agent/.env.example` | **EDIT** | +ORCA_BRIDGES_* |

---

## 4. Test strategy

```go
// orca-agent/internal/mcp/executor_test.go

func TestExecutor_PISTools_DispatchToProductsAgent(t *testing.T) {
    mockPIS := &mockPISTool{}
    exec := mcp.NewExecutor("http://orca", "key", nil, nil).WithPISTool(mockPIS)

    tools := []string{
        "pis_query_crs", "pis_query_hld", "pis_query_adr",
        "pis_kg_search", "pis_query_feature_context",
    }
    for _, tool := range tools {
        _, err := exec.ExecuteLegacy(ctx, tool, map[string]any{"q": "test"})
        assert.NoError(t, err)
        assert.Equal(t, tool, mockPIS.lastTool)
    }
}

func TestExecutor_PISTools_NotConfigured_Error(t *testing.T) {
    exec := mcp.NewExecutor("http://orca", "key", nil, nil) // no PISTool
    _, err := exec.ExecuteLegacy(ctx, "pis_query_crs", map[string]any{"crs_id": "CRS-001"})
    assert.ErrorContains(t, err, "ORCA_BRIDGES_PRODUCTS_AGENT_ID")
}
```

---

## 5. Risks & mitigations

| Risk | Mitigation |
|------|-----------|
| `ws.Client` chưa có `Call()` → compile fail | Phụ thuộc CR-ORCA-005 phải implement trước |
| PISTool init order phức tạp | Setter pattern `WithPISTool()` giải quyết circular dependency |
| ProductsAgentID không set | Graceful skip: không wire, `pis_*` không expose trong catalog |

---

## 6. Execution order

```
CR-ORCA-005 (ws.Client.Call)
  → CR-ORCA-004 (Config env override + init order)
    → CR-ORCA-001 (PISTool catalog + executor wiring)  ← file này
```

---

## Execution Log — 2026-10-19

**Status**: ✅ implemented

### Files changed
- [`mcp/catalog_pis.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/mcp/catalog_pis.go): new — 5 `pis_*` tool definitions
- [`mcp/executor.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/mcp/executor.go): `pis_*` dispatch routing
- [`mcp/executor_pis_test.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/mcp/executor_pis_test.go): 6 tests (PASS)
- [`cmd/orca-agent/main.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/cmd/orca-agent/main.go): PISTool conditional wiring
- [`pis/client.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/pis/client.go): unified `pis_*` names, +`UpdateCRSStatus()`, +`pis_query_adr`

### Bugs found and fixed
- `pis/client.go` used `pis.query_*` (dot) → fixed to `pis_query_*` (underscore)
- RBAC allowlist missing `pis_*` tools → added

### Verification
```
GOWORK=off go test ./internal/mcp/... -run TestExecutor_PIS → 6 PASS
GOWORK=off go build ./... → EXIT 0
```
