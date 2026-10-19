---
task_id: T-P1-002
phase: 1
cr_id: CR-ORCA-005
solution_ref: solutions/CR-ORCA-005-solution.md
service: orca-agent
target_file: orca-agent/internal/ws/client_test.go
priority: P1
status: done
depends_on: T-P1-001
estimated_effort: 1–2 giờ
---

# T-P1-002 — Viết unit tests cho ws.Client.Call()

## Context

Sau khi T-P1-001 thêm `Call()` method, cần verify hai behavior:
1. Happy path: HTTP server mock trả về JSON envelope → `Call()` unwrap `data` field đúng
2. Error path: `mcpProxyURL` rỗng → error message rõ ràng hướng dẫn set `ORCA_MCP_PROXY_URL`

## Target file

```
orca-agent/internal/ws/client_test.go
```

## Instructions

1. **Tạo hoặc mở** `orca-agent/internal/ws/client_test.go`

2. **Thêm test `TestClient_Call_PeerInvoke_Success`**:
   ```go
   func TestClient_Call_PeerInvoke_Success(t *testing.T) {
       srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           // Verify request format
           assert.Equal(t, http.MethodPost, r.Method)
           assert.Equal(t, "/internal/v1/agent-peer/mcp/proxy", r.URL.Path)
           assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
           assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

           var body map[string]any
           require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
           assert.Equal(t, "products-agent", body["agent_id"])
           assert.Equal(t, "pis_query_crs", body["tool"])

           // Return JSON envelope
           w.Header().Set("Content-Type", "application/json")
           w.WriteHeader(http.StatusOK)
           json.NewEncoder(w).Encode(map[string]any{
               "data": map[string]any{"id": "CRS-045", "status": "pending"},
           })
       }))
       defer srv.Close()

       c := NewClient(Config{WSURL: "ws://ignored"})
       c.SetPeerConfig(srv.URL, "test-token")

       ctx := context.Background()
       result, err := c.Call(ctx, "products-agent", "pis_query_crs", map[string]any{"crs_id": "CRS-045"})

       require.NoError(t, err)
       assert.Contains(t, string(result), "CRS-045")
       assert.Contains(t, string(result), "pending")
   }
   ```

3. **Thêm test `TestClient_Call_NoPeerConfig_Error`**:
   ```go
   func TestClient_Call_NoPeerConfig_Error(t *testing.T) {
       c := NewClient(Config{WSURL: "ws://ignored"})
       // mcpProxyURL rỗng → expect error hướng dẫn set env var

       ctx := context.Background()
       _, err := c.Call(ctx, "products-agent", "pis_query_crs", nil)

       require.Error(t, err)
       assert.Contains(t, err.Error(), "not configured")
       assert.Contains(t, err.Error(), "ORCA_MCP_PROXY_URL")
   }
   ```

4. **Thêm test `TestClient_Call_HTTP403_Error`** (bonus):
   ```go
   func TestClient_Call_HTTP403_Error(t *testing.T) {
       srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           w.WriteHeader(http.StatusForbidden)
           w.Write([]byte("no grant for pis_query_crs"))
       }))
       defer srv.Close()

       c := NewClient(Config{WSURL: "ws://ignored"})
       c.SetPeerConfig(srv.URL, "bad-token")

       _, err := c.Call(context.Background(), "products-agent", "pis_query_crs", nil)

       require.Error(t, err)
       assert.Contains(t, err.Error(), "no grant")
   }
   ```

5. Đảm bảo imports: `"context"`, `"encoding/json"`, `"net/http"`, `"net/http/httptest"`, `"testing"`, `"github.com/stretchr/testify/assert"`, `"github.com/stretchr/testify/require"`

## Acceptance criteria

- [ ] `TestClient_Call_PeerInvoke_Success` pass
- [ ] `TestClient_Call_NoPeerConfig_Error` pass
- [ ] `TestClient_Call_HTTP403_Error` pass
- [ ] Không có test nào skip hoặc dùng `t.Skip()`

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go test ./internal/ws/... -run TestClient_Call -v
```
