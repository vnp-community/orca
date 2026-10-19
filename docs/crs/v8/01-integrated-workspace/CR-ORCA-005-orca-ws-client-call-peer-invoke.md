---
doc_id: CR-ORCA-005
product: orca
layer: backend, frontend
status: implemented
priority: P1
feature_ref: DS-005
created: 2026-10-19
completed_date: 2026-10-19
depends_on: CR-ORCA-004
---

# CR-ORCA-005 — orca-agent: ws.Client.Call() Peer-Invoke Method

| Field | Value |
|-------|-------|
| **CR ID** | CR-ORCA-005 |
| **Product** | orca (orca-agent & frontend) |
| **Priority** | P1 — Blocking: PISTool không compile nếu ws.Client không implement PeerCaller |
| **Status** | ✅ Implemented (2026-10-19) |
| **Effort** | 1 ngày |
| **Depends On** | CR-ORCA-004 |

---

## Vấn đề phát hiện khi đọc code thật

`orca-agent/internal/pis/client.go` định nghĩa:

```go
type PeerCaller interface {
    Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (json.RawMessage, error)
}
```

`pis.NewPISTool(productsAgentID, wsClient)` truyền `wsClient` (`*ws.Client`) như `PeerCaller`.

Nhưng `orca-agent/internal/ws/client.go` **không có `Call()` method** — chỉ có:
- `NewClient(cfg Config) *Client`
- `Run(ctx context.Context)`
- `Send(msg OutgoingMessage) error`
- `connect(ctx)` (private)

→ Code sẽ **fail compile** với `*ws.Client does not implement PeerCaller`.

---

## Acceptance Criteria

### AC-1: orca-agent ws.Client.Call() via mcp-proxy HTTP

**Approach**: Giống doc-agent's `invoke.PeerClient` — orca-agent thực hiện peer-invoke qua HTTP mcp-proxy, không qua WS (WS không hỗ trợ request-response).

File: `orca-agent/internal/ws/client.go` — thêm:

```go
// Call performs a peer-invoke to another agent via mcp-proxy.
// Implements pis.PeerCaller interface.
// Uses HTTP POST to /internal/v1/agent-peer/mcp/proxy, NOT the WS connection.
func (c *Client) Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (json.RawMessage, error) {
    if c.mcpProxyURL == "" {
        return nil, fmt.Errorf("peer-invoke not configured: set ORCA_MCP_PROXY_URL")
    }

    input, err := json.Marshal(args)
    if err != nil {
        return nil, fmt.Errorf("peer-invoke: marshal args: %w", err)
    }
    body, err := json.Marshal(map[string]any{
        "tool":  toolName,
        "input": json.RawMessage(input),
    })
    if err != nil {
        return nil, fmt.Errorf("peer-invoke: marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.mcpProxyURL+"/internal/v1/agent-peer/mcp/proxy",
        bytes.NewReader(body),
    )
    if err != nil {
        return nil, fmt.Errorf("peer-invoke: build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.peerToken)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("peer-invoke %s→%s: %w", toolName, targetAgentID, err)
    }
    defer resp.Body.Close()

    rawBody, _ := io.ReadAll(resp.Body)
    if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
        return nil, fmt.Errorf("peer-invoke: no grant for %s: %s", toolName, rawBody)
    }
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("peer-invoke %s HTTP %d: %s", toolName, resp.StatusCode, rawBody)
    }

    var envelope struct {
        Data json.RawMessage `json:"data"`
    }
    if err := json.Unmarshal(rawBody, &envelope); err != nil {
        return nil, fmt.Errorf("peer-invoke: decode response: %w", err)
    }
    return envelope.Data, nil
}
```

Thêm fields vào `Client` struct:

```go
type Client struct {
    cfg         Config
    conn        *websocket.Conn
    // Peer-invoke fields (CR-ORCA-005)
    mcpProxyURL string
    peerToken   string
    httpClient  *http.Client
}
```

### AC-2: Config extension — thêm MCPProxy section

File: `orca-agent/internal/config/config.go`:

```go
type Config struct {
    // ... existing ...
    // MCPProxy — for peer-invoke to other agents (CR-ORCA-005)
    MCPProxy struct {
        URL       string `yaml:"url"`        // Env: ORCA_MCP_PROXY_URL
        PeerToken string `yaml:"peer_token"` // Env: ORCA_MCP_PROXY_PEER_TOKEN
    } `yaml:"mcp_proxy"`
}

// Load() — thêm env overrides:
if v := os.Getenv("ORCA_MCP_PROXY_URL"); v != "" {
    cfg.MCPProxy.URL = v
}
if v := os.Getenv("ORCA_MCP_PROXY_PEER_TOKEN"); v != "" {
    cfg.MCPProxy.PeerToken = v
}
```

### AC-3: main.go — wire mcpProxyURL + peerToken vào ws.Client

File: `orca-agent/cmd/orca-agent/main.go`:

```go
// Create ws client with peer-invoke support
wsClient = ws.NewClient(ws.Config{
    WSURL:  authResult.WebSocketURL,
    GetJWT: reg.GetCurrentJWT,
    // ... OnMessage, OnConnect ...
})
// Wire peer-invoke config (CR-ORCA-005)
wsClient.SetPeerConfig(cfg.MCPProxy.URL, cfg.MCPProxy.PeerToken)
```

Hoặc truyền qua Config struct:

```go
type Config struct { // ws.Config
    WSURL       string
    GetJWT      func() string
    OnMessage   func(...)
    OnConnect   func(...)
    MCPProxyURL  string  // CR-ORCA-005
    PeerToken   string   // CR-ORCA-005
}
```

### AC-4: deploy/orca-agent — thêm mcp_proxy config

File: `deploy/orca-agent/config/orca-agent.yaml`:

```yaml
# ... existing ...
# MCP Proxy peer-invoke (CR-ORCA-005)
mcp_proxy:
  url: ""          # Set via env: ORCA_MCP_PROXY_URL
  peer_token: ""   # Set via env: ORCA_MCP_PROXY_PEER_TOKEN
```

File: `deploy/orca-agent/docker-compose.yml` — thêm:

```yaml
environment:
  # ... existing ...
  - ORCA_MCP_PROXY_URL=${ORCA_MCP_PROXY_URL:-http://mcp-proxy:3012}
  - ORCA_MCP_PROXY_PEER_TOKEN=${ORCA_MCP_PROXY_PEER_TOKEN:-}
```

File: `deploy/orca-agent/.env.example` — thêm:

```bash
# MCP Proxy peer-invoke for pis_* tools (CR-ORCA-005)
ORCA_MCP_PROXY_URL=http://mcp-proxy:3012
ORCA_MCP_PROXY_PEER_TOKEN=<peer token from admin>
```

### AC-5: Verify compile

```bash
# Sau khi implement, verify compile:
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./...
# Expected: no errors
```

### AC-6: Tests

```go
// orca-agent/internal/ws/client_test.go
func TestClient_Call_PeerInvoke_Success(t *testing.T) {
    // Setup HTTP server mock returning JSON envelope
    // Call c.Call(ctx, "products-agent", "pis_query_crs", {...})
    // Assert result matches
}

func TestClient_Call_NoPeerConfig_Error(t *testing.T) {
    c := NewClient(Config{...})
    // mcpProxyURL = "" → expect error
    _, err := c.Call(ctx, "products-agent", "pis_query_crs", nil)
    assert.ErrorContains(t, err, "not configured")
}
```

---

## Files cần sửa

```
orca-agent/
├── internal/ws/client.go                    ← EDIT: +Call(), +mcpProxyURL/peerToken fields, +imports
├── internal/ws/client_test.go               ← NEW/EDIT
├── internal/config/config.go               ← EDIT: +MCPProxy struct + env overrides
└── cmd/orca-agent/main.go                  ← EDIT: wire peer config vào ws.Client

deploy/orca-agent/
├── config/orca-agent.yaml                   ← EDIT: +mcp_proxy section
├── docker-compose.yml                       ← EDIT: +ORCA_MCP_PROXY_* vars
└── .env.example                             ← EDIT: +mcp-proxy vars
```

---

## Trạng thái thực thi & Rà soát (Executed & Verified: 2026-10-19)

### 1. Checklist tiêu chí chấp nhận (Acceptance Criteria)

- [x] **AC-1 (ws.Client.Call via mcp-proxy HTTP)**:
  - Bổ sung các trường `mcpProxyURL`, `peerToken`, `httpClient` vào struct `Client` trong `internal/ws/client.go`.
  - Triển khai method `Call(ctx, targetAgentID, toolName, args)` gửi HTTP POST tới `${mcpProxyURL}/internal/v1/agent-peer/mcp/proxy` với header `Authorization: Bearer <token>`.
  - Xử lý mã lỗi HTTP: 401/403 (unauthorized/no grant), non-200, và giải nén dữ liệu từ envelope `data`.
- [x] **AC-2 (Config extension - MCPProxy)**: Thêm struct `MCPProxy` vào `Config` và nạp ghi đè từ biến môi trường `ORCA_MCP_PROXY_URL`, `ORCA_MCP_PROXY_PEER_TOKEN`.
- [x] **AC-3 (main.go wiring)**: Phương thức `wsClient.SetPeerConfig(cfg.MCPProxy.URL, cfg.MCPProxy.PeerToken)` được gọi khi khởi tạo trong `cmd/orca-agent/main.go`.
- [x] **AC-4 (deploy/orca-agent config)**: Cập nhật `orca-agent.yaml`, `docker-compose.yml` (`ORCA_MCP_PROXY_URL=${ORCA_MCP_PROXY_URL:-http://mcp-proxy:3012}`) và `.env.example`.
- [x] **AC-5 (Verify compile)**: Build kiểm tra `GOWORK=off go build ./...` thành công không phát sinh lỗi interface.
- [x] **AC-6 (Tests)**:
  - Backend: 3 unit tests trong `internal/ws/client_test.go` (`TestClient_Call_PeerInvoke_Success`, `TestClient_Call_NoPeerConfig_Error`, `TestClient_Call_HTTP403_Error`) PASS 100%.
  - Frontend: 7 unit tests trong `use-mcp-proxy-status.test.ts` (100% PASS), tích hợp component `mcp-proxy-status-indicator.tsx`, `mcp-proxy-connection-test.tsx`.

### 2. Các tệp đã thực hiện (Backend & Frontend)

#### Backend (orca-agent & deploy):
- `backend/agents/bridges/dev-tools/orca-agent/internal/ws/client.go` (T-P1-001: +Call, +SetPeerConfig, +peer fields)
- `backend/agents/bridges/dev-tools/orca-agent/internal/ws/client_test.go` (T-P1-002: 3/3 tests PASS)
- `backend/agents/bridges/dev-tools/orca-agent/internal/pis/client.go` (T-P1-003: PeerCaller compatibility)
- `backend/agents/bridges/dev-tools/orca-agent/internal/config/config.go` (T-P2-001: +MCPProxy config struct & env vars)
- `backend/agents/bridges/dev-tools/orca-agent/cmd/orca-agent/main.go` (T-P3-003: SetPeerConfig wiring)
- `deploy/orca-agent/config/orca-agent.yaml`, `deploy/orca-agent/docker-compose.yml`, `deploy/orca-agent/.env.example` (T-P2-004)

#### Frontend (Orca UI):
- `frontend/src/renderer/src/types/mcp-proxy-types.ts` (T01: MCPProxyStatus, MCPProxyConfig, PeerInvokeError types)
- `frontend/src/renderer/src/hooks/use-mcp-proxy-status.ts` (T02: hook giám sát tình trạng mcp-proxy)
- `frontend/src/renderer/src/components/fleet/mcp-proxy-status-indicator.tsx` (T03: indicator trạng thái kết nối proxy)
- `frontend/src/renderer/src/components/settings/mcp-proxy-connection-test.tsx` (T04: panel test kết nối và đo độ trễ proxy)
- `frontend/src/renderer/src/hooks/__tests__/use-mcp-proxy-status.test.ts` (T19a-1: 7/7 tests PASS)

### 3. Kết quả kiểm thử
- **Frontend Vitest**: `7/7 tests PASS` (loading, healthy, unreachable, latency, polling, error mapping).
- **TypeScript compile**: `0 errors TS`.
- **Backend Go test**: `PASS` (`TestClient_Call_PeerInvoke_Success`, `TestClient_Call_NoPeerConfig_Error`, `TestClient_Call_HTTP403_Error`).

