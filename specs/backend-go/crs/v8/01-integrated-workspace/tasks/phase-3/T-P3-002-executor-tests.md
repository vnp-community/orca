---
task_id: T-P3-002
phase: 3
cr_id: CR-ORCA-001
solution_ref: solutions/CR-ORCA-001-solution.md
service: orca-agent
target_file: orca-agent/internal/mcp/executor_test.go
priority: P1
status: done
depends_on: T-P2-003
estimated_effort: 1–2 giờ
---

# T-P3-002 — Viết unit tests cho Executor pis_* routing

## Context

Sau T-P2-003, `ExecuteLegacy()` có routing cho `pis_*` tools. Cần tests verify:
1. Khi `pisTool` được wire → dispatch đến đúng PISTool method
2. Khi `pisTool == nil` → error message hướng dẫn rõ ràng

## Target file

```
orca-agent/internal/mcp/executor_test.go
```

## Instructions

1. **Tạo hoặc mở** `orca-agent/internal/mcp/executor_test.go`

2. **Tạo mock PISTool**:
   ```go
   type mockPISTool struct {
       lastTool   string
       lastParams map[string]any
       returnData json.RawMessage
       returnErr  error
   }

   func (m *mockPISTool) Dispatch(ctx context.Context, toolName string, params map[string]any) (json.RawMessage, error) {
       m.lastTool = toolName
       m.lastParams = params
       if m.returnErr != nil {
           return nil, m.returnErr
       }
       if m.returnData != nil {
           return m.returnData, nil
       }
       return json.RawMessage(`{"ok":true}`), nil
   }
   ```
   > **Lưu ý**: Mock phải implement interface phù hợp với cách `pisTool` được dùng trong executor. Nếu executor dùng `*pis.PISTool` trực tiếp, cần tạo interface wrapper hoặc dùng embed. Kiểm tra type thực tế trước khi viết mock.

3. **Test: dispatch đến PISTool khi được configure**:
   ```go
   func TestExecutor_PISTools_DispatchToProductsAgent(t *testing.T) {
       tools := []string{
           "pis_query_feature_context",
           "pis_query_crs",
           "pis_query_hld",
           "pis_query_adr",
           "pis_kg_search",
       }

       for _, toolName := range tools {
           t.Run(toolName, func(t *testing.T) {
               mock := &mockPISTool{}
               exec := NewExecutor("http://orca", "key", nil, nil)
               // Inject mock — cách inject phụ thuộc vào implementation thực tế
               exec.pisTool = mock  // hoặc exec.WithPISTool(mockAdapter)

               ctx := context.Background()
               _, err := exec.ExecuteLegacy(ctx, toolName, map[string]any{"q": "test"})

               assert.NoError(t, err)
               assert.Equal(t, toolName, mock.lastTool)
           })
       }
   }
   ```

4. **Test: error khi PISTool chưa configured**:
   ```go
   func TestExecutor_PISTools_NotConfigured_Error(t *testing.T) {
       exec := NewExecutor("http://orca", "key", nil, nil)
       // pisTool = nil (không wire)

       ctx := context.Background()
       _, err := exec.ExecuteLegacy(ctx, "pis_query_crs", map[string]any{"crs_id": "CRS-001"})

       require.Error(t, err)
       assert.Contains(t, err.Error(), "ORCA_BRIDGES_PRODUCTS_AGENT_ID")
   }
   ```

## Acceptance criteria

- [ ] `TestExecutor_PISTools_DispatchToProductsAgent` pass cho cả 5 tools
- [ ] `TestExecutor_PISTools_NotConfigured_Error` pass
- [ ] Test không dùng `time.Sleep` hay external network calls

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go test ./internal/mcp/... -run TestExecutor_PIS -v
```
