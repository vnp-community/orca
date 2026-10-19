---
task_id: T-P1-001
phase: 1
cr_id: CR-ORCA-005
solution_ref: solutions/CR-ORCA-005-solution.md
service: orca-agent
target_file: orca-agent/internal/ws/client.go
priority: P1
status: done
estimated_effort: 2–3 giờ
blocks: T-P1-002, T-P2-003
---

# T-P1-001 — Extend ws.Client struct với peer-invoke fields

## Context

`orca-agent/internal/pis/client.go` định nghĩa `PeerCaller` interface yêu cầu method `Call(ctx, targetAgentID, toolName, args) (json.RawMessage, error)`. `ws.Client` được truyền vào `pis.NewPISTool()` nhưng **không có `Call()` method** → code fail compile ngay.

WS không hỗ trợ request-response (fire-and-forget), nên peer-invoke phải đi qua HTTP POST đến `mcp-proxy`. Cần thêm `mcpProxyURL`, `peerToken`, `httpClient` vào struct và implement `SetPeerConfig()` setter.

## Target file

```
orca-agent/internal/ws/client.go
```

## Instructions

1. **Đọc file hiện tại** `orca-agent/internal/ws/client.go` để xác định:
   - Struct `Client` hiện có những fields nào
   - Import block hiện tại có gì

2. **Thêm fields vào `Client` struct** (sau fields hiện có):
   ```go
   // Peer-invoke via HTTP mcp-proxy (CR-ORCA-005)
   // Tách khỏi WS vì WS không hỗ trợ request-response.
   mcpProxyURL string
   peerToken   string
   httpClient  *http.Client
   ```

3. **Thêm imports** nếu chưa có: `"bytes"`, `"encoding/json"`, `"fmt"`, `"io"`, `"net/http"`, `"time"`

4. **Thêm `SetPeerConfig()` method** vào cuối file:
   ```go
   // SetPeerConfig configures HTTP peer-invoke. Gọi sau NewClient, trước Run.
   // CR-ORCA-005: setter pattern — không thay đổi ws.Config để tránh breaking callers.
   func (c *Client) SetPeerConfig(mcpProxyURL, peerToken string) {
       c.mcpProxyURL = mcpProxyURL
       c.peerToken = peerToken
       if c.httpClient == nil {
           c.httpClient = &http.Client{Timeout: 30 * time.Second}
       }
   }
   ```

5. **Thêm `Call()` method** sau `SetPeerConfig()`:
   ```go
   // Call thực hiện peer-invoke đến agent khác qua HTTP mcp-proxy.
   // Implements pis.PeerCaller interface.
   // CR-ORCA-005: không dùng WS connection — WS không hỗ trợ request-response.
   func (c *Client) Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (json.RawMessage, error) {
       if c.mcpProxyURL == "" {
           return nil, fmt.Errorf("peer-invoke not configured: set ORCA_MCP_PROXY_URL")
       }

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
           return nil, fmt.Errorf("peer-invoke %s→%s: %w", toolName, targetAgentID, err)
       }
       defer resp.Body.Close()

       rawBody, _ := io.ReadAll(resp.Body)

       switch resp.StatusCode {
       case http.StatusOK:
           // continue
       case http.StatusForbidden, http.StatusUnauthorized:
           return nil, fmt.Errorf("peer-invoke: no grant for %s on %s: %s", toolName, targetAgentID, rawBody)
       default:
           return nil, fmt.Errorf("peer-invoke %s HTTP %d: %s", toolName, resp.StatusCode, rawBody)
       }

       var envelope struct {
           Data json.RawMessage `json:"data"`
       }
       if err := json.Unmarshal(rawBody, &envelope); err != nil {
           return rawBody, nil // fallback: return raw nếu không có envelope
       }
       return envelope.Data, nil
   }
   ```

## Acceptance criteria

- [ ] `Client` struct có 3 fields mới: `mcpProxyURL`, `peerToken`, `httpClient`
- [ ] `SetPeerConfig(url, token string)` method tồn tại
- [ ] `Call(ctx, targetAgentID, toolName, args)` method tồn tại và trả về `(json.RawMessage, error)`
- [ ] File compile được: `GOWORK=off go build ./internal/ws/...`

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./internal/ws/...
```
