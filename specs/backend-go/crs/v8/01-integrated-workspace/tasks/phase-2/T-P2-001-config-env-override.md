---
task_id: T-P2-001
phase: 2
cr_id: CR-ORCA-004
solution_ref: solutions/CR-ORCA-004-solution.md
service: orca-agent
target_file: orca-agent/internal/config/config.go
priority: P1
status: done
depends_on: T-P1-001, T-P1-003
estimated_effort: 2 giờ
blocks: T-P2-003, T-P2-004
---

# T-P2-001 — Thêm Bridges + MCPProxy vào Config struct + env overrides

## Context

`orca-agent/internal/config/config.go` hiện chỉ parse YAML, không đọc env vars. Điều này khiến `docker-compose env_file` không có tác dụng với `ORCA_BRIDGES_PRODUCTS_AGENT_ID` và `ORCA_MCP_PROXY_URL`. Cần thêm 2 struct mới vào `Config` và thêm env override logic vào hàm `Load()`.

## Target file

```
orca-agent/internal/config/config.go
```

## Instructions

1. **Đọc file hiện tại** để hiểu cấu trúc `Config` struct và hàm `Load()`

2. **Thêm `Bridges` struct** vào `Config`:
   ```go
   // Bridges — peer agent IDs cho cross-agent tool calls (CR-ORCA-004)
   Bridges struct {
       ProductsAgentID string `yaml:"products_agent_id"`
       DocAgentID      string `yaml:"doc_agent_id"`
   } `yaml:"bridges"`
   ```

3. **Thêm `MCPProxy` struct** vào `Config`:
   ```go
   // MCPProxy — HTTP endpoint cho peer-invoke qua mcp-proxy (CR-ORCA-005)
   MCPProxy struct {
       URL       string `yaml:"url"`
       PeerToken string `yaml:"peer_token"`
   } `yaml:"mcp_proxy"`
   ```

4. **Thêm env override logic** vào hàm `Load()`, sau khi `yaml.Unmarshal`:
   ```go
   // Env overrides — priority: env > yaml (CR-ORCA-004)
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
       cfg.Orca.OrcaURL = v  // Điều chỉnh field name theo struct thực tế
   }
   ```
   > **Lưu ý**: Kiểm tra tên field thực tế trong struct (có thể là `OrcaURL`, `URL`, hoặc khác)

5. **Đảm bảo import** `"os"` đã có trong file

## Acceptance criteria

- [ ] `Config.Bridges.ProductsAgentID` và `Config.Bridges.DocAgentID` tồn tại
- [ ] `Config.MCPProxy.URL` và `Config.MCPProxy.PeerToken` tồn tại
- [ ] `Load()` override từ env vars với đúng 4 keys: `ORCA_BRIDGES_PRODUCTS_AGENT_ID`, `ORCA_BRIDGES_DOC_AGENT_ID`, `ORCA_MCP_PROXY_URL`, `ORCA_MCP_PROXY_PEER_TOKEN`
- [ ] `GOWORK=off go build ./internal/config/...` thành công

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./internal/config/...
```
