---
task_id: T-P4-004
phase: 4
cr_id: CR-ORCA-002
solution_ref: solutions/CR-ORCA-002-solution.md
service: orca-agent
target_file: orca-agent/internal/task/handler_test.go
priority: P2
status: done
depends_on: T-P4-003
estimated_effort: 1–2 giờ
---

# T-P4-004 — Viết unit tests cho TaskHandler auto-context injection

## Target file

```
orca-agent/internal/task/handler_test.go   ← NEW FILE
```

## Instructions

1. **Tạo file** `orca-agent/internal/task/handler_test.go`

2. **Tạo mock PISTool** phù hợp với interface `pis.PISTool` (hoặc interface wrapper):
   ```go
   type mockDispatcher struct {
       calls      []string
       returnData map[string]json.RawMessage // toolName → returnData
       returnErr  error
   }

   func (m *mockDispatcher) Dispatch(ctx context.Context, toolName string, params map[string]any) (json.RawMessage, error) {
       m.calls = append(m.calls, toolName)
       if m.returnErr != nil {
           return nil, m.returnErr
       }
       if d, ok := m.returnData[toolName]; ok {
           return d, nil
       }
       return json.RawMessage(`{"ok":true}`), nil
   }
   ```

3. **Test: auto-inject với CRS**:
   ```go
   func TestTaskHandler_AutoContextInjection_WithCRS(t *testing.T) {
       mock := &mockDispatcher{
           returnData: map[string]json.RawMessage{
               "pis_query_crs": json.RawMessage(`{"id":"CRS-045","status":"pending"}`),
           },
       }
       handler := NewTaskHandler(mockToPISTool(mock)) // adapter nếu cần

       task := Task{ID: "T-100", KGContext: TaskKGContext{CRSID: "CRS-045"}}
       enriched, err := handler.HandleAssignment(context.Background(), task)

       require.NoError(t, err)
       assert.NotNil(t, enriched.CRSContext)
       assert.Contains(t, mock.calls, "pis_query_crs")
       assert.NotContains(t, mock.calls, "pis_query_feature_context") // không gọi nếu không có feature_ref
   }
   ```

4. **Test: skip khi không có CRS**:
   ```go
   func TestTaskHandler_AutoContextInjection_NoCRS_Skipped(t *testing.T) {
       mock := &mockDispatcher{}
       handler := NewTaskHandler(mockToPISTool(mock))

       task := Task{ID: "T-101"} // KGContext rỗng
       enriched, err := handler.HandleAssignment(context.Background(), task)

       require.NoError(t, err)
       assert.Nil(t, enriched.CRSContext)
       assert.Empty(t, mock.calls) // PISTool.Dispatch không được gọi
   }
   ```

5. **Test: system prompt format**:
   ```go
   func TestEnrichedTask_SystemPromptFormat(t *testing.T) {
       enriched := &EnrichedTask{
           Task: Task{
               ID:    "T-100",
               Title: "Implement auth service",
               KGContext: TaskKGContext{
                   CRSID:   "CRS-045",
                   Service: "auth-service",
               },
           },
           CRSContext: json.RawMessage(`{"id":"CRS-045"}`),
       }

       prompt := BuildSystemPrompt(enriched)

       assert.Contains(t, prompt, "T-100")
       assert.Contains(t, prompt, "CRS-045")
       assert.Contains(t, prompt, "// implements: CRS-045")
       assert.Contains(t, prompt, "auth-service")
   }
   ```

## Acceptance criteria

- [ ] `TestTaskHandler_AutoContextInjection_WithCRS` pass
- [ ] `TestTaskHandler_AutoContextInjection_NoCRS_Skipped` pass
- [ ] `TestEnrichedTask_SystemPromptFormat` pass
- [ ] KG fetch failure không gây test failure (best-effort)

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go test ./internal/task/... -v
```
