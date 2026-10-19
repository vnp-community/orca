---
task_id: T-P2-002
phase: 2
cr_id: CR-ORCA-004
solution_ref: solutions/CR-ORCA-004-solution.md
service: orca-agent
target_file: orca-agent/internal/config/config_test.go
priority: P1
status: done
depends_on: T-P2-001
estimated_effort: 1 giờ
---

# T-P2-002 — Viết unit tests cho Config env override

## Context

Sau T-P2-001, cần test rằng các env vars thực sự override giá trị YAML. Đây là safety net để tránh regression khi config struct thay đổi.

## Target file

```
orca-agent/internal/config/config_test.go
```

## Instructions

1. **Tạo hoặc mở** `orca-agent/internal/config/config_test.go`

2. **Thêm helper** `writeTempConfig(t, yamlContent string) string`:
   ```go
   func writeTempConfig(t *testing.T, yamlContent string) string {
       t.Helper()
       f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
       require.NoError(t, err)
       _, err = f.WriteString(yamlContent)
       require.NoError(t, err)
       f.Close()
       return f.Name()
   }
   ```

3. **Thêm `TestLoad_EnvOverride_ProductsAgentID`**:
   ```go
   func TestLoad_EnvOverride_ProductsAgentID(t *testing.T) {
       tmp := writeTempConfig(t, "bridges:\n  products_agent_id: \"\"\n")
       t.Setenv("ORCA_BRIDGES_PRODUCTS_AGENT_ID", "products-agent-test")

       cfg, err := Load(tmp)

       require.NoError(t, err)
       assert.Equal(t, "products-agent-test", cfg.Bridges.ProductsAgentID)
   }
   ```

4. **Thêm `TestLoad_EnvOverride_MCPProxyURL`**:
   ```go
   func TestLoad_EnvOverride_MCPProxyURL(t *testing.T) {
       tmp := writeTempConfig(t, "mcp_proxy:\n  url: \"\"\n")
       t.Setenv("ORCA_MCP_PROXY_URL", "http://proxy:3012")

       cfg, err := Load(tmp)

       require.NoError(t, err)
       assert.Equal(t, "http://proxy:3012", cfg.MCPProxy.URL)
   }
   ```

5. **Thêm `TestLoad_YAMLValue_NoEnv_Preserved`** (kiểm tra YAML không bị overwrite khi không có env):
   ```go
   func TestLoad_YAMLValue_NoEnv_Preserved(t *testing.T) {
       tmp := writeTempConfig(t, "bridges:\n  products_agent_id: \"from-yaml\"\n")
       // Không set env var

       cfg, err := Load(tmp)

       require.NoError(t, err)
       assert.Equal(t, "from-yaml", cfg.Bridges.ProductsAgentID)
   }
   ```

## Acceptance criteria

- [ ] `TestLoad_EnvOverride_ProductsAgentID` pass
- [ ] `TestLoad_EnvOverride_MCPProxyURL` pass
- [ ] `TestLoad_YAMLValue_NoEnv_Preserved` pass
- [ ] Không có data race (tests dùng `t.Setenv` thay vì `os.Setenv` thủ công)

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go test ./internal/config/... -v
```
