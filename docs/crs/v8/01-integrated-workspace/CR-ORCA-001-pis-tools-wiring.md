---
doc_id: CR-ORCA-001
product: orca
layer: backend, frontend
status: implemented
priority: P1
feature_ref: DS-005
created: 2026-10-19
completed_date: 2026-10-19
depends_on: CR-PIKB-001,CR-PIKB-002
---

# CR-ORCA-001 — orca-agent: Expose pis_* Tools + Wire PISTool

| Field | Value |
|-------|-------|
| **CR ID** | CR-ORCA-001 |
| **Product** | orca (orca-agent & frontend) |
| **Priority** | P1 — Blocking: AI agent trong Orca không query được KG |
| **Status** | ✅ Implemented (2026-10-19) |
| **Effort** | 2–3 ngày |
| **Depends On** | CR-PIKB-001 (schema), CR-PIKB-002 (products-agent KG wired) |

---

## Mô tả yêu cầu

`orca-agent` đã implement (✅ 2026-10-19):
- `orca-agent/internal/pis/client.go` — PISTool với Dispatch method (peer-invoke products-agent)
- `orca-agent/internal/config/config.go` — `Bridges.ProductsAgentID`, `Bridges.DocAgentID`

**Còn thiếu**:
1. PISTool chưa được đăng ký vào orca-agent MCP executor.
2. `pis_*` tools chưa có trong orca-agent tool catalog.
3. `cmd/orca-agent/main.go` chưa đọc `cfg.Bridges.ProductsAgentID` và khởi tạo PISTool.

---

## Acceptance Criteria

### AC-1: PISTool catalog trong orca-agent

File: `orca-agent/internal/mcp/catalog.go` (hoặc tương đương) — thêm 5 pis_* tools:

```go
// TASK-MV2-003: pis_* tools cho PIKB KG query (BUG-MISSING-V2-003 fix)
{
    Name:        "pis_query_feature_context",
    Description: "Query full feature context từ PIKB KG: HLD, CRS, ADR liên quan đến Feature ID. Dùng để lấy context đầy đủ trước khi implement.",
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
    Description: "Query HLD node theo product, version, C4 level (L1-L4).",
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
    Description: "Full-text search trên Dev Knowledge KG. Filter theo type (crs|hld|adr|tdd|bug) và product_id.",
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
```

### AC-2: Wire PISTool vào orca-agent executor

File: `orca-agent/internal/mcp/executor.go` (hoặc tool_executor.go tương đương):

```go
// Thêm routing cho pis_* tools:
case "pis_query_feature_context",
     "pis_query_crs",
     "pis_query_hld",
     "pis_query_adr",
     "pis_kg_search":
    if e.pisTool == nil {
        return nil, fmt.Errorf("pis tools not configured (set ORCA_BRIDGES_PRODUCTS_AGENT_ID)")
    }
    return e.pisTool.Dispatch(ctx, toolName, params)
```

### AC-3: Executor struct extension

```go
type Executor struct {
    // ... existing fields ...
    pisTool *pis.PISTool  // optional — wired nếu ProductsAgentID configured
}

func (e *Executor) WithPISTool(t *pis.PISTool) *Executor {
    e.pisTool = t
    return e
}
```

### AC-4: main.go wiring

File: `orca-agent/cmd/orca-agent/main.go`:

```go
executor := mcp.NewExecutor(/* existing args */)

// Wire PISTool nếu products-agent ID configured
if cfg.Bridges.ProductsAgentID != "" {
    pisTool := pis.NewPISTool(cfg.Bridges.ProductsAgentID, wsClient)
    executor = executor.WithPISTool(pisTool)
    log.Printf("PIS tools enabled (products-agent: %s)", cfg.Bridges.ProductsAgentID)
}
```

### AC-5: deploy config

File: `deploy/orca-agent/.env.example` — thêm:

```bash
# PIKB KG integration (CR-ORCA-001)
ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-prod
ORCA_BRIDGES_DOC_AGENT_ID=doc-agent-prod
```

### AC-6: Tests

```go
// orca-agent/internal/mcp/executor_test.go
func TestExecutor_PISTools_DispatchToProductsAgent(t *testing.T) { ... }
func TestExecutor_PISTools_NotConfigured_Error(t *testing.T) { ... }
```

### AC-7: End-to-end smoke test

```
AI agent gọi: pis_query_crs({crs_id: "CRS-045"})
  → orca-agent: PISTool.Dispatch("pis_query_crs", ...)
  → peer-invoke products-agent
  → products-agent: KGHandler.QueryCRS("CRS-045")
  → pikb-catalog-service: GET /api/v1/kg/crs/CRS-045
  → Response: {id: "CRS-045", status: "in-progress", ...}
```

---

## Files cần tạo/sửa

```
orca-agent/
├── cmd/orca-agent/main.go               ← EDIT: wire PISTool
├── internal/mcp/
│   ├── catalog.go                       ← EDIT: +5 pis_* tool definitions
│   ├── executor.go                      ← EDIT: pisTool field + routing
│   └── executor_test.go                 ← NEW/EDIT: PIS tool tests
└── deploy/orca-agent/.env.example       ← EDIT: add bridge env vars
```

---

## Trạng thái thực thi & Rà soát (Executed & Verified: 2026-10-19)

### 1. Checklist tiêu chí chấp nhận (Acceptance Criteria)

- [x] **AC-1 (PISTool catalog)**: 5 công cụ `pis_*` được định nghĩa trong `orca-agent/internal/mcp/catalog_pis.go` (`PISToolCatalog()`) gồm: `pis_query_feature_context`, `pis_query_crs`, `pis_query_hld`, `pis_query_adr`, `pis_kg_search`.
- [x] **AC-2 (Wire PISTool executor)**: Định tuyến 5 công cụ `pis_*` trong `ExecuteLegacy` (`internal/mcp/executor.go`) với kiểm tra nil trả về lỗi `"pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID"`.
- [x] **AC-3 (Executor extension)**: `Executor` struct có trường `pisTool *pis.PISTool` và method `WithPISTool(t *pis.PISTool) *Executor`.
- [x] **AC-4 (main.go wiring)**: Khởi tạo và liên kết `pis.NewPISTool` khi `cfg.Bridges.ProductsAgentID != ""` trong `cmd/orca-agent/main.go`.
- [x] **AC-5 (Deploy config)**: Cập nhật `deploy/orca-agent/.env.example` và `docker-compose.yml` với biến môi trường `ORCA_BRIDGES_PRODUCTS_AGENT_ID` và `ORCA_BRIDGES_DOC_AGENT_ID`.
- [x] **AC-6 (Tests)**: Unit test `TestExecutor_PISTools_DispatchToProductsAgent` và `TestExecutor_PISTools_NotConfigured_Error` pass (100%).
- [x] **AC-7 (End-to-end integration)**: Client dispatch thông qua peer-invoke products-agent, UI hiển thị badge và xử lý cấu hình thiếu.

### 2. Các tệp đã thực hiện (Backend & Frontend)

#### Backend (orca-agent):
- `backend/agents/bridges/dev-tools/orca-agent/internal/mcp/catalog_pis.go` (T-P3-001)
- `backend/agents/bridges/dev-tools/orca-agent/internal/mcp/executor.go` (T-P2-003)
- `backend/agents/bridges/dev-tools/orca-agent/internal/mcp/executor_pis_test.go` (T-P3-002: 6/6 tests PASS)
- `backend/agents/bridges/dev-tools/orca-agent/internal/pis/client.go` (T-P1-003)
- `backend/agents/bridges/dev-tools/orca-agent/cmd/orca-agent/main.go` (T-P3-003)

#### Frontend (Orca UI):
- `frontend/src/renderer/src/hooks/use-agent-tool-capabilities.ts` (T05: hook phát hiện 5 công cụ PIS từ agent catalog)
- `frontend/src/renderer/src/components/agent/kg-query-result-renderer.tsx` (T06: renderer kết quả query KG)
- `frontend/src/renderer/src/components/agent/agent-capability-badge.tsx` (T07: badge trạng thái PIKB KG)
- `frontend/src/renderer/src/components/agent/pis-tool-not-configured-error.tsx` (T08: banner hướng dẫn khi thiếu config)
- `frontend/src/renderer/src/components/settings/pikb-integration-settings.tsx` (T09: form cài đặt kết nối PIS)
- `frontend/src/renderer/src/hooks/__tests__/use-agent-tool-capabilities.test.ts` (T19a-2: 5/5 tests PASS)

### 3. Kết quả kiểm thử
- **Frontend Vitest**: `5/5 tests PASS` (setup, catalog detection, missing tools handling).
- **TypeScript compile**: `0 errors TS`.
- **Backend Go test**: `PASS` (`TestExecutor_PISTools_DispatchToProductsAgent`, `TestExecutor_PISTools_NotConfigured_Error`).

