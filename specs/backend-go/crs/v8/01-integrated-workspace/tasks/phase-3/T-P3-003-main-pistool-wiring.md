---
task_id: T-P3-003
phase: 3
cr_id: CR-ORCA-001
solution_ref: solutions/CR-ORCA-001-solution.md
service: orca-agent
target_file: orca-agent/cmd/orca-agent/main.go
priority: P1
status: done
depends_on: T-P1-001, T-P2-001, T-P2-003, T-P3-001
estimated_effort: 1–2 giờ
---

# T-P3-003 — Wire PISTool vào main.go với đúng init order

## Context

`main.go` phải khởi tạo các component theo đúng thứ tự:
1. `Executor` trước (không cần wsClient)
2. `ws.Client` sau (cần executor.ExecuteLegacy làm OnMessage handler)
3. `PISTool` cuối cùng (cần wsClient cho peer-invoke)

Nếu sai thứ tự → compile error hoặc nil pointer panic khi chạy.

## Target file

```
orca-agent/cmd/orca-agent/main.go
```

## Instructions

1. **Đọc toàn bộ `main.go`** để hiểu cấu trúc hiện tại:
   - Executor được tạo ở đâu?
   - ws.Client được tạo ở đâu?
   - Có `catalog` hay tool list nào được build không?

2. **Đảm bảo init order** theo pattern sau (tìm điểm chèn phù hợp):

   ```go
   // Step 1: Create executor (không cần wsClient)
   executor := mcp.NewExecutor(cfg.Orca.OrcaURL, cfg.Orca.APIKey, watcher, auditBuf)

   // Step 2: Build base catalog
   catalog := mcp.OrcaToolCatalog()

   // Step 3: Create ws client — OnMessage = executor.ExecuteLegacy
   wsClient = ws.NewClient(ws.Config{
       WSURL:     authResult.WebSocketURL,
       GetJWT:    reg.GetCurrentJWT,
       OnMessage: executor.ExecuteLegacy,
       // ... other existing fields ...
   })
   // CR-ORCA-005: configure peer-invoke via HTTP mcp-proxy
   wsClient.SetPeerConfig(cfg.MCPProxy.URL, cfg.MCPProxy.PeerToken)

   // Step 4: Wire PISTool SAU ws (CR-ORCA-001/004)
   if cfg.Bridges.ProductsAgentID != "" {
       pisTool := pis.NewPISTool(cfg.Bridges.ProductsAgentID, wsClient)
       executor.WithPISTool(pisTool)
       catalog = append(catalog, mcp.PISToolCatalog()...)
       log.Printf("[orca-agent] PIS tools enabled (products-agent: %s)", cfg.Bridges.ProductsAgentID)
   }
   ```

3. **Thêm imports** nếu chưa có: `"github.com/vnp-blc/orca/orca-agent/internal/pis"` (kiểm tra module path thực tế trong `go.mod`)

4. **Không xóa** bất kỳ logic hiện có nào — chỉ thêm vào / chỉnh thứ tự

## Acceptance criteria

- [ ] PISTool chỉ được tạo khi `cfg.Bridges.ProductsAgentID != ""`
- [ ] `wsClient.SetPeerConfig()` được gọi trước `ws.Client.Run()`
- [ ] `executor.WithPISTool(pisTool)` được gọi sau `ws.NewClient()`
- [ ] Khi `cfg.Bridges.ProductsAgentID != ""`, log message `"PIS tools enabled"` xuất hiện
- [ ] `GOWORK=off go build ./cmd/orca-agent/...` thành công
- [ ] `GOWORK=off go build ./...` thành công (full build)

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./...
# Expected: exit 0, no errors
```

## Smoke test (nếu có môi trường)

```bash
ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-test ./orca-agent --config config.yaml
# Expected log: "[orca-agent] PIS tools enabled (products-agent: products-agent-test)"
```
