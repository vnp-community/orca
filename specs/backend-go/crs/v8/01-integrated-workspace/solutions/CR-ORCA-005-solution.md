---
cr_id: CR-ORCA-005
solution_id: SOL-ORCA-005
product: orca
service: orca-agent
layer: backend
language: go
status: implemented
priority: P1
effort: 1 ngày
depends_on: CR-ORCA-004
created: 2026-10-19
---

# SOL-ORCA-005 — orca-agent: ws.Client.Call() Peer-Invoke Method

## 1. Phân tích vấn đề

`orca-agent/internal/pis/client.go` định nghĩa interface:

```go
type PeerCaller interface {
    Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (json.RawMessage, error)
}
```

`pis.NewPISTool(productsAgentID, wsClient)` truyền `*ws.Client` như `PeerCaller`, nhưng `ws.Client` **không có `Call()` method** — chỉ có `NewClient`, `Run`, `Send`, `connect`. Code sẽ **fail compile** ngay lập tức.

**Root cause**: WS connection là fire-and-forget (không có request-response). Peer-invoke cần HTTP round-trip → phải dùng HTTP POST đến mcp-proxy, không phải WS.

---

## 2. Giải pháp kỹ thuật

### 2.1 Approach: HTTP peer-invoke qua mcp-proxy

Giống pattern trong `doc-agent`'s `invoke.PeerClient`:

```
PISTool.Dispatch("pis_query_crs", params)
  → ws.Client.Call(ctx, "products-agent-prod", "pis_query_crs", params)
  → HTTP POST http://mcp-proxy:3012/internal/v1/agent-peer/mcp/proxy
      Body: {tool: "pis_query_crs", input: {...}}
      Header: Authorization: Bearer <peerToken>
  → mcp-proxy routes → products-agent
  → Response: {data: {...}}
```

**Lý do không dùng WS**: WS là pub/sub, không có correlation ID cho response. HTTP mcp-proxy đã có sẵn endpoint peer-invoke với request-response semantics.

### 2.2 ws.Client struct extension — `internal/ws/client.go` (EDIT)

```go
package ws

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    // ... existing imports ...
)

// Client quản lý WS connection và HTTP peer-invoke.
// CR-ORCA-005: thêm mcpProxyURL, peerToken, httpClient cho peer-invoke.
type Client struct {
    cfg         Config
    conn        *websocket.Conn

    // Peer-invoke via HTTP mcp-proxy (CR-ORCA-005)
    // Tách khỏi WS vì WS không hỗ trợ request-response.
    mcpProxyURL string
    peerToken   string
    httpClient  *http.Client
}

// SetPeerConfig configures HTTP peer-invoke. Gọi sau NewClient, trước Run.
// CR-ORCA-005: setter pattern để tránh phải pass vào ws.Config (phá vỡ interface hiện tại).
func (c *Client) SetPeerConfig(mcpProxyURL, peerToken string) {
    c.mcpProxyURL = mcpProxyURL
    c.peerToken = peerToken
    if c.httpClient == nil {
        c.httpClient = &http.Client{Timeout: 30 * time.Second}
    }
}

// Call thực hiện peer-invoke đến agent khác qua HTTP mcp-proxy.
// Implements pis.PeerCaller interface.
// CR-ORCA-005: không dùng WS connection — WS không hỗ trợ request-response.
func (c *Client) Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (json.RawMessage, error) {
    if c.mcpProxyURL == "" {
        return nil, fmt.Errorf("peer-invoke not configured: set ORCA_MCP_PROXY_URL")
    }

    // Build request body
    input, err := json.Marshal(args)
    if err != nil {
        return nil, fmt.Errorf("peer-invoke: marshal args: %w", err)
    }
    body, err := json.Marshal(map[string]any{
        "agent_id": targetAgentID,
        "tool":     toolName,
        "input":    json.RawMessage(input),
    })
    if err != nil {
        return nil, fmt.Errorf("peer-invoke: marshal request: %w", err)
    }

    // HTTP POST to mcp-proxy
    req, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.mcpProxyURL+"/internal/v1/agent-peer/mcp/proxy",
        bytes.NewReader(body),
    )
    if err != nil {
        return nil, fmt.Errorf("peer-invoke: build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    if c.peerToken != "" {
        req.Header.Set("Authorization", "Bearer "+c.peerToken)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("peer-invoke %s→%s/%s: %w", toolName, targetAgentID, toolName, err)
    }
    defer resp.Body.Close()

    rawBody, _ := io.ReadAll(resp.Body)

    switch resp.StatusCode {
    case http.StatusOK:
        // OK
    case http.StatusForbidden, http.StatusUnauthorized:
        return nil, fmt.Errorf("peer-invoke: no grant for %s on %s: %s", toolName, targetAgentID, rawBody)
    default:
        return nil, fmt.Errorf("peer-invoke %s HTTP %d: %s", toolName, resp.StatusCode, rawBody)
    }

    // Unwrap envelope {data: ...}
    var envelope struct {
        Data json.RawMessage `json:"data"`
    }
    if err := json.Unmarshal(rawBody, &envelope); err != nil {
        // Fallback: return raw body nếu không có envelope
        return rawBody, nil
    }
    return envelope.Data, nil
}
```

### 2.3 Config extension — `internal/config/config.go` (EDIT)

```go
// MCPProxy section đã được thêm trong CR-ORCA-004.
// CR-ORCA-005: chỉ cần verify env overrides đã có:
//   ORCA_MCP_PROXY_URL → cfg.MCPProxy.URL
//   ORCA_MCP_PROXY_PEER_TOKEN → cfg.MCPProxy.PeerToken
```

### 2.4 main.go — wire peer config vào ws.Client (EDIT)

```go
// Tùy chọn A: SetPeerConfig setter (ít breaking hơn)
wsClient = ws.NewClient(ws.Config{
    WSURL:     authResult.WebSocketURL,
    GetJWT:    reg.GetCurrentJWT,
    OnMessage: executor.ExecuteLegacy,
})
// CR-ORCA-005: wire peer-invoke config sau NewClient
wsClient.SetPeerConfig(cfg.MCPProxy.URL, cfg.MCPProxy.PeerToken)

// Tùy chọn B: truyền qua ws.Config (nếu muốn immutable)
wsClient = ws.NewClient(ws.Config{
    WSURL:       authResult.WebSocketURL,
    GetJWT:      reg.GetCurrentJWT,
    OnMessage:   executor.ExecuteLegacy,
    MCPProxyURL: cfg.MCPProxy.URL,   // CR-ORCA-005
    PeerToken:   cfg.MCPProxy.PeerToken, // CR-ORCA-005
})
```

> **Khuyến nghị**: Tùy chọn A (setter) ít thay đổi `ws.Config` struct hơn, tránh breaking callers khác.

### 2.5 Deploy config additions — `deploy/orca-agent/` (EDIT)

`config/orca-agent.yaml`:
```yaml
mcp_proxy:
  url: ""          # Override: ORCA_MCP_PROXY_URL
  peer_token: ""   # Override: ORCA_MCP_PROXY_PEER_TOKEN
```

`docker-compose.yml`:
```yaml
environment:
  - ORCA_MCP_PROXY_URL=${ORCA_MCP_PROXY_URL:-http://mcp-proxy:3012}
  - ORCA_MCP_PROXY_PEER_TOKEN=${ORCA_MCP_PROXY_PEER_TOKEN:-}
```

`.env.example`:
```bash
# MCP Proxy peer-invoke for pis_* tools (CR-ORCA-005)
ORCA_MCP_PROXY_URL=http://mcp-proxy:3012
ORCA_MCP_PROXY_PEER_TOKEN=<peer token from admin>
```

---

## 3. File map

| File | Action | Ghi chú |
|------|--------|---------|
| `orca-agent/internal/ws/client.go` | **EDIT** | +`Call()`, +`mcpProxyURL/peerToken/httpClient` fields, +`SetPeerConfig()` |
| `orca-agent/internal/ws/client_test.go` | **NEW/EDIT** | 2 test cases cho `Call()` |
| `orca-agent/internal/config/config.go` | **EDIT** | Verify `MCPProxy` struct + env overrides (đã có trong CR-ORCA-004) |
| `orca-agent/cmd/orca-agent/main.go` | **EDIT** | Gọi `wsClient.SetPeerConfig(...)` sau `ws.NewClient` |
| `deploy/orca-agent/config/orca-agent.yaml` | **EDIT** | +`mcp_proxy` section |
| `deploy/orca-agent/docker-compose.yml` | **EDIT** | +`ORCA_MCP_PROXY_*` env |
| `deploy/orca-agent/.env.example` | **EDIT** | +mcp-proxy vars |

---

## 4. Test strategy

```go
// orca-agent/internal/ws/client_test.go

func TestClient_Call_PeerInvoke_Success(t *testing.T) {
    // Mock HTTP server trả về JSON envelope
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "/internal/v1/agent-peer/mcp/proxy", r.URL.Path)
        assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

        var body map[string]any
        json.NewDecoder(r.Body).Decode(&body)
        assert.Equal(t, "products-agent", body["agent_id"])
        assert.Equal(t, "pis_query_crs", body["tool"])

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]any{
            "data": map[string]any{"id": "CRS-045", "status": "pending"},
        })
    }))
    defer srv.Close()

    c := ws.NewClient(ws.Config{WSURL: "ws://ignored"})
    c.SetPeerConfig(srv.URL, "test-token")

    result, err := c.Call(ctx, "products-agent", "pis_query_crs", map[string]any{"crs_id": "CRS-045"})

    assert.NoError(t, err)
    assert.Contains(t, string(result), "CRS-045")
}

func TestClient_Call_NoPeerConfig_Error(t *testing.T) {
    c := ws.NewClient(ws.Config{WSURL: "ws://ignored"})
    // mcpProxyURL = "" → expect error

    _, err := c.Call(ctx, "products-agent", "pis_query_crs", nil)
    assert.ErrorContains(t, err, "not configured")
    assert.ErrorContains(t, err, "ORCA_MCP_PROXY_URL")
}
```

---

## 5. Compile verification

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./...
# Expected: exit 0

# Verify interface compliance (compile-time check)
# Thêm vào ws/client.go:
var _ pis.PeerCaller = (*Client)(nil)
```

---

## 6. Interface compliance check

Thêm vào `internal/ws/client.go` để compile-time verify:

```go
// Compile-time assertion: *Client phải implement pis.PeerCaller.
// CR-ORCA-005: nếu signature thay đổi, compile fail ngay lập tức.
import "github.com/vnp-blc/orca/orca-agent/internal/pis"
var _ pis.PeerCaller = (*Client)(nil)
```

---

## 7. Risks & mitigations

| Risk | Mitigation |
|------|-----------|
| mcp-proxy endpoint path khác với giả định | Kiểm tra doc-agent implementation để confirm path `/internal/v1/agent-peer/mcp/proxy` |
| Response envelope format khác | Fallback: nếu unmarshal envelope fail, trả về raw body |
| Timeout quá ngắn / dài | Default 30s; configurable qua `ORCA_MCP_PROXY_TIMEOUT` nếu cần |
| Import cycle: ws → pis | Dùng `var _ pis.PeerCaller = (*Client)(nil)` trong test file, không trong production code |

---

## 8. Execution order

```
CR-ORCA-005 (ws.Client.Call — này)  ← PHẢI LÀM ĐẦU TIÊN
  → CR-ORCA-004 (Config env override + executor init order)
    → CR-ORCA-001 (PISTool catalog + executor wiring)
      → CR-ORCA-002 (Task KGContext + auto-inject)
        → CR-ORCA-003 (Quality gate + CRS status update)
```

---

## Execution Log — 2026-10-19

**Status**: ✅ implemented

### Files changed
- [`ws/client.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/ws/client.go): +`Call()`, +`SetPeerConfig()`, +3 peer-invoke fields
- [`ws/client_test.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/ws/client_test.go): 3 tests (PASS)

### Verification
```
GOWORK=off go test ./internal/ws/... → 3 PASS
```
