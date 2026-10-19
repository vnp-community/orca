---
doc_id: CR-ORCA-004
product: orca
layer: backend, frontend
status: implemented
priority: P1
feature_ref: DS-005
created: 2026-10-19
completed_date: 2026-10-19
depends_on: CR-ORCA-001
---

# CR-ORCA-004 — orca-agent: Config Env Override + Executor Extension + Deploy Kit

| Field | Value |
|-------|-------|
| **CR ID** | CR-ORCA-004 |
| **Product** | orca (orca-agent & frontend) |
| **Priority** | P1 — Blocking: executor hiện không có `ExecuteLegacy` cho pis_* tools |
| **Status** | ✅ Implemented (2026-10-19) |
| **Effort** | 2 ngày |
| **Depends On** | CR-ORCA-001 |

---

## Vấn đề hiện tại (phát hiện khi đọc code thật)

1. **Executor thật**: `orca-agent/internal/mcp/executor.go` dùng `ExecuteLegacy(ctx, toolName, params)` — không phải Register pattern. PISTool trong CR-ORCA-001 cần integrate vào đúng executor path này.

2. **Config loading**: `config.go` dùng YAML only, không override từ env vars. `Bridges.ProductsAgentID` và `Bridges.DocAgentID` phải đọc được từ env `ORCA_BRIDGES_PRODUCTS_AGENT_ID` để deploy được qua docker-compose.

3. **Deploy kit**: `deploy/orca-agent/` đã có docker-compose nhưng **thiếu** env vars mới cho Bridges section.

4. **orca-agent config.yaml template**: Chưa có `bridges:` section trong example config.

---

## Acceptance Criteria

### AC-1: Config env var override

File: `orca-agent/internal/config/config.go` — thêm env override vào Load():

```go
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("config: read file %s: %w", path, err)
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("config: parse: %w", err)
    }
    // Env overrides — allow docker-compose env_file to override YAML
    if v := os.Getenv("ORCA_BRIDGES_PRODUCTS_AGENT_ID"); v != "" {
        cfg.Bridges.ProductsAgentID = v
    }
    if v := os.Getenv("ORCA_BRIDGES_DOC_AGENT_ID"); v != "" {
        cfg.Bridges.DocAgentID = v
    }
    if v := os.Getenv("ORCA_API_URL"); v != "" {
        cfg.Orca.OrcaURL = v
    }
    if cfg.Telemetry.MetricsAddr == "" {
        cfg.Telemetry.MetricsAddr = ":9102"
    }
    return &cfg, nil
}
```

### AC-2: Executor extension — pis_* dispatch

File: `orca-agent/internal/mcp/executor.go` — tìm `ExecuteLegacy` và thêm pis_* routing:

```go
// ExecuteLegacy dispatches a raw tool call (từ ws OnMessage).
// CR-ORCA-004: add pis_* tools routing.
func (e *Executor) ExecuteLegacy(ctx context.Context, toolName string, params map[string]any) (any, error) {
    // EXISTING routing for orca.* tools...
    switch toolName {
    case "orca.create_task", "orca.get_task_status", ...:
        // existing handlers

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

Executor struct extension:

```go
type Executor struct {
    orcaBaseURL string
    apiKey      string
    watcher     *TaskWatcher
    auditBuf    *audit.Buffer
    pisTool     *pis.PISTool  // CR-ORCA-004: optional — wired nếu ProductsAgentID set
}

func (e *Executor) WithPISTool(t *pis.PISTool) *Executor {
    e.pisTool = t
    return e
}
```

### AC-3: main.go — wire PISTool sau WS init

File: `orca-agent/cmd/orca-agent/main.go` — sau `wsClient = ws.NewClient(...)`:

```go
// CR-ORCA-001/004: wire PISTool nếu products-agent configured
if cfg.Bridges.ProductsAgentID != "" {
    pisTool := pis.NewPISTool(cfg.Bridges.ProductsAgentID, wsClient)
    executor.WithPISTool(pisTool)
    log.Printf("[orca-agent] PIS tools enabled (products-agent: %s)", cfg.Bridges.ProductsAgentID)
}
```

> **Note**: `pis.NewPISTool` cần `wsClient` (WS peer-invoke). Nhưng `wsClient` được tạo SAU executor — cần restructure thứ tự init hoặc dùng setter pattern. Xem AC-4.

### AC-4: Restructure init order trong main.go

Hiện tại: `executor → wsClient`. PISTool cần wsClient. Solution: dùng setter, init sau ws.NewClient:

```go
// 1. Create executor (without PISTool yet)
executor := mcp.NewExecutor(orcaURL, orcaKey, watcher, auditBuf)
catalog := append(mcp.OrcaToolCatalog(), pisTools...)  // merge catalog

// 2. Create ws client
wsClient = ws.NewClient(ws.Config{ ... OnMessage: executor.ExecuteLegacy ... })

// 3. Wire PISTool (needs wsClient for peer-invoke)
if cfg.Bridges.ProductsAgentID != "" {
    pisTool := pis.NewPISTool(cfg.Bridges.ProductsAgentID, wsClient)
    executor.WithPISTool(pisTool)
}
```

### AC-5: Tool catalog merge — pis_* vào OrcaToolCatalog

File: `orca-agent/internal/mcp/executor.go` (hoặc catalog.go) — thêm function:

```go
// PISToolCatalog returns 5 pis_* tool definitions to append to OrcaToolCatalog.
// CR-ORCA-001: called conditionally when ProductsAgentID is configured.
func PISToolCatalog() []Tool {
    strType := map[string]any{"type": "string"}
    return []Tool{
        {Name: "pis_query_feature_context", Description: "...", InputSchema: ...},
        {Name: "pis_query_crs", Description: "...", InputSchema: ...},
        {Name: "pis_query_hld", Description: "...", InputSchema: ...},
        {Name: "pis_query_adr", Description: "...", InputSchema: ...},
        {Name: "pis_kg_search", Description: "...", InputSchema: ...},
    }
}
```

### AC-6: PISTool — peer-invoke interface

File: `orca-agent/internal/pis/client.go` — `PeerCaller` interface phải match ws.Client:

```go
// PeerCaller is the interface PISTool uses for cross-agent calls.
// *ws.Client must implement this.
type PeerCaller interface {
    Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (any, error)
}
```

Verify: `ws.Client` trong orca-agent có method `Call(ctx, targetAgentID, toolName, args)` → nếu chưa có, cần thêm vào `internal/ws/client.go`.

### AC-7: deploy/orca-agent config template

File: `deploy/orca-agent/config/orca-agent.yaml` — thêm:

```yaml
# Bridges — peer agent IDs for cross-agent tool calls (CR-ORCA-004)
bridges:
  products_agent_id: ""    # Set via env: ORCA_BRIDGES_PRODUCTS_AGENT_ID
  doc_agent_id: ""         # Set via env: ORCA_BRIDGES_DOC_AGENT_ID
```

File: `deploy/orca-agent/docker-compose.yml` — thêm vào environment:

```yaml
environment:
  # ... existing ...
  - ORCA_BRIDGES_PRODUCTS_AGENT_ID=${ORCA_BRIDGES_PRODUCTS_AGENT_ID:-}
  - ORCA_BRIDGES_DOC_AGENT_ID=${ORCA_BRIDGES_DOC_AGENT_ID:-}
```

File: `deploy/orca-agent/.env.example` — tạo mới hoặc thêm:

```bash
# Existing
ORCA_BOOTSTRAP_TOKEN=<from workplace admin>
ORCA_API_URL=http://orca-web:9000
ORCA_API_KEY=<orca service key>

# CR-ORCA-004: PIKB KG bridge (optional — enables pis_* tools)
ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-prod
ORCA_BRIDGES_DOC_AGENT_ID=doc-agent-prod
```

### AC-8: Tests

```go
// orca-agent/internal/mcp/executor_test.go
func TestExecutor_PISTools_NilPISTool_ReturnsError(t *testing.T) { ... }
func TestExecutor_PISTools_Dispatched_ToProductsAgent(t *testing.T) { ... }

// orca-agent/internal/config/config_test.go
func TestLoad_EnvOverride_ProductsAgentID(t *testing.T) { ... }
```

---

## Files cần sửa

```
orca-agent/
├── internal/config/config.go               ← EDIT: env override for Bridges
├── internal/mcp/executor.go                ← EDIT: +pisTool field, +WithPISTool, +pis_* routing in ExecuteLegacy
├── internal/mcp/catalog_pis.go             ← NEW: PISToolCatalog() func
├── internal/pis/client.go                  ← EDIT: PeerCaller interface, verify ws.Client compat
├── internal/ws/client.go                   ← EDIT if needed: +Call() method for peer-invoke
├── cmd/orca-agent/main.go                  ← EDIT: init order + PISTool wire
├── internal/config/config_test.go          ← NEW/EDIT
└── internal/mcp/executor_test.go           ← NEW/EDIT

deploy/orca-agent/
├── config/orca-agent.yaml                  ← EDIT: +bridges section
├── docker-compose.yml                      ← EDIT: +ORCA_BRIDGES_* env
└── .env.example                            ← NEW: with all env vars documented
```

---

## Trạng thái thực thi & Rà soát (Executed & Verified: 2026-10-19)

### 1. Checklist tiêu chí chấp nhận (Acceptance Criteria)

- [x] **AC-1 (Config env var override)**: `Load()` trong `internal/config/config.go` nạp biến môi trường ghi đè YAML: `ORCA_BRIDGES_PRODUCTS_AGENT_ID`, `ORCA_BRIDGES_DOC_AGENT_ID`, `ORCA_API_URL`, fallback port `:9102`.
- [x] **AC-2 (Executor extension)**: `ExecuteLegacy` định tuyến 5 công cụ `pis_*` với kiểm tra nil trả về lỗi `"pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID"`. Bổ sung `WithPISTool()` setter pattern.
- [x] **AC-3 (main.go wiring)**: Khởi tạo `pis.NewPISTool(cfg.Bridges.ProductsAgentID, wsClient)` khi `cfg.Bridges.ProductsAgentID != ""` và gán vào executor.
- [x] **AC-4 (Restructure init order)**: Sắp xếp thứ tự khởi tạo hợp lý: `NewExecutor` → `ws.NewClient` → `pis.NewPISTool(productsAgentID, wsClient)` → `executor.WithPISTool(pisTool)`.
- [x] **AC-5 (Tool catalog merge)**: Hàm `PISToolCatalog()` trả về 5 định nghĩa công cụ `pis_*` với đầy đủ JSON schema.
- [x] **AC-6 (PISTool PeerCaller interface)**: Khai báo interface `PeerCaller` tương thích với method `Call()` trên `*ws.Client`.
- [x] **AC-7 (deploy config template)**: Cập nhật `deploy/orca-agent/config/orca-agent.yaml`, `docker-compose.yml` và `.env.example` với đầy đủ các cấu hình bridge.
- [x] **AC-8 (Tests)**:
  - Backend: `config_test.go` (4/4 tests PASS), `executor_pis_test.go` (6/6 tests PASS).
  - Frontend: `use-mcp-proxy-status.test.ts` (7/7 tests PASS).

### 2. Các tệp đã thực hiện (Backend & Frontend)

#### Backend (orca-agent & deploy):
- `backend/agents/bridges/dev-tools/orca-agent/internal/config/config.go` (T-P2-001)
- `backend/agents/bridges/dev-tools/orca-agent/internal/config/config_test.go` (T-P2-002: 4/4 tests PASS)
- `backend/agents/bridges/dev-tools/orca-agent/internal/mcp/executor.go` (T-P2-003)
- `backend/agents/bridges/dev-tools/orca-agent/internal/mcp/catalog_pis.go` (T-P3-001)
- `backend/agents/bridges/dev-tools/orca-agent/internal/mcp/executor_pis_test.go` (T-P3-002: 6/6 tests PASS)
- `backend/agents/bridges/dev-tools/orca-agent/cmd/orca-agent/main.go` (T-P3-003)
- `deploy/orca-agent/config/orca-agent.yaml`, `deploy/orca-agent/docker-compose.yml`, `deploy/orca-agent/.env.example` (T-P2-004)

#### Frontend (Orca UI):
- `frontend/src/renderer/src/components/fleet/agent-bridge-config-display.tsx` (T10a: xem cấu hình bridge sản phẩm/tài liệu)
- `frontend/src/renderer/src/components/fleet/agent-pis-health-check.tsx` (T10b: kiểm tra tình trạng kết nối PIS)
- `frontend/src/renderer/src/components/fleet/agent-env-config-display.tsx` (T10c: xem biến môi trường với mặt nạ bảo mật)
- `frontend/src/renderer/src/components/onboarding/pikb-bridge-step.tsx` (T10d: bước cài đặt bridge trong onboarding)

### 3. Kết quả kiểm thử
- **Frontend Vitest**: `PASS` (tích hợp UI views kết nối proxy và settings).
- **TypeScript compile**: `0 errors TS`.
- **Backend Go test**: `PASS` (`TestLoad_EnvOverride_ProductsAgentID`, `TestLoad_EnvOverride_DocAgentID`, `TestExecutor_PISTools_DispatchToProductsAgent`).

