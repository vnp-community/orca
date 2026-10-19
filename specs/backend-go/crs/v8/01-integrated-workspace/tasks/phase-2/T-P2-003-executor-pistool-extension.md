---
task_id: T-P2-003
phase: 2
cr_id: CR-ORCA-004
solution_ref: solutions/CR-ORCA-004-solution.md
service: orca-agent
target_file: orca-agent/internal/mcp/executor.go
priority: P1
status: done
depends_on: T-P1-003, T-P2-001
estimated_effort: 2 giờ
blocks: T-P3-001
---

# T-P2-003 — Extend Executor: thêm pisTool field + WithPISTool() + pis_* routing

## Context

`orca-agent/internal/mcp/executor.go` là file xử lý tool dispatch thực tế qua `ExecuteLegacy()`. Cần thêm:
1. `pisTool *pis.PISTool` field vào struct
2. `WithPISTool()` setter method
3. Routing case cho 5 `pis_*` tools trong `ExecuteLegacy()`

Thứ tự quan trọng: `WithPISTool()` phải là setter (không phải constructor arg) vì `PISTool` cần `ws.Client` được tạo **sau** `Executor`.

## Target file

```
orca-agent/internal/mcp/executor.go
```

## Instructions

1. **Đọc file hiện tại** để xác định:
   - `Executor` struct có những fields gì
   - `ExecuteLegacy()` có những case nào hiện tại
   - Import block hiện tại

2. **Thêm `pisTool` field** vào `Executor` struct:
   ```go
   pisTool *pis.PISTool  // CR-ORCA-004: optional, nil nếu ProductsAgentID chưa set
   ```
   Thêm vào cuối struct, sau các fields hiện có.

3. **Thêm import** `"github.com/vnp-blc/orca/orca-agent/internal/pis"` nếu chưa có.
   > **Lưu ý**: Kiểm tra module path thực tế trong `go.mod` của orca-agent

4. **Thêm `WithPISTool()` method**:
   ```go
   // WithPISTool attaches PISTool vào executor. Trả về receiver để chain.
   // CR-ORCA-004: phải gọi SAU ws.NewClient vì PISTool cần wsClient cho peer-invoke.
   func (e *Executor) WithPISTool(t *pis.PISTool) *Executor {
       e.pisTool = t
       return e
   }
   ```

5. **Thêm pis_* routing** vào `ExecuteLegacy()` switch, **trước** `default:` case:
   ```go
   // CR-ORCA-001/004: PIKB KG tools — peer-invoke qua PISTool đến products-agent
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

## Acceptance criteria

- [ ] `Executor` struct có field `pisTool *pis.PISTool`
- [ ] `WithPISTool(t *pis.PISTool) *Executor` method tồn tại
- [ ] `ExecuteLegacy()` có routing cho đúng 5 tools: `pis_query_feature_context`, `pis_query_crs`, `pis_query_hld`, `pis_query_adr`, `pis_kg_search`
- [ ] Khi `pisTool == nil`, trả về error có chứa `"ORCA_BRIDGES_PRODUCTS_AGENT_ID"`
- [ ] `GOWORK=off go build ./internal/mcp/...` thành công

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./internal/mcp/...
```
