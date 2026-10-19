---
task_id: T-P5-005
phase: 5
cr_id: CR-ORCA-003
solution_ref: solutions/CR-ORCA-003-solution.md
service: orca-agent
target_file: orca-agent/cmd/orca-agent/main.go
priority: P2
status: done
depends_on: T-P5-003, T-P3-003
estimated_effort: 1 giờ
---

# T-P5-005 — Wire CompletionHook vào main.go + full build verify

## Context

Đây là task cuối cùng: wire `CompletionHook` vào main.go và verify toàn bộ project compile được. Sau task này, tất cả 5 CRs đã được implement.

## Target file

```
orca-agent/cmd/orca-agent/main.go
```

## Instructions

1. **Đọc lại main.go** (sau khi đã có các thay đổi từ T-P3-003)

2. **Thêm wire CompletionHook** sau khi `pisTool` được tạo:
   ```go
   // CR-ORCA-003: wire CompletionHook sau khi pisTool và gitVerifier đã sẵn sàng
   var completionHook *task.CompletionHook
   if pisTool != nil {
       // gitVerifier: có thể là nil nếu chưa implement gitVerify tool
       // Khi nil → convention check bị skip (graceful degradation)
       completionHook = task.NewCompletionHook(pisTool, gitVerifier)
       log.Println("[orca-agent] CompletionHook enabled (quality gate active)")
   }
   ```

3. **Tạo TaskHandler** với CompletionHook (nếu chưa có):
   ```go
   taskHandler := task.NewTaskHandler(pisTool)
   if completionHook != nil {
       taskHandler.SetCompletionHook(completionHook)
   }
   ```
   > **Lưu ý**: Nếu `TaskHandler` chưa có `SetCompletionHook()` method, thêm vào `handler.go`

4. **Đảm bảo imports** đúng:
   ```go
   "github.com/vnp-blc/orca/orca-agent/internal/task"
   ```

5. **Verify toàn bộ build**:
   ```bash
   GOWORK=off go build ./...
   ```

## Phase 5 full integration checklist

Sau khi hoàn thành T-P5-005, verify toàn bộ Phase 5:

- [ ] `pikb-catalog-service`: `PATCH /api/v1/kg/crs/{id}/status` hoạt động (T-P5-001)
- [ ] `products-agent`: `pis_update_crs_status` tool trong catalog (T-P5-002)
- [ ] `orca-agent/internal/task/completion_hook.go` tồn tại (T-P5-003)
- [ ] Tất cả CompletionHook tests pass (T-P5-004)
- [ ] `orca-agent` full build: `GOWORK=off go build ./...` → exit 0

## Final acceptance criteria — toàn bộ 5 phases

- [ ] **Phase 1**: `ws.Client.Call()` implement + tests pass
- [ ] **Phase 2**: Config env override hoạt động + deploy kit đầy đủ
- [ ] **Phase 3**: PISTool wired + executor routing + main.go init order đúng
- [ ] **Phase 4**: Task KGContext domain + auto-context injection + tests pass
- [ ] **Phase 5**: Quality gate + CRS status update + CompletionHook wire

## Final build command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./...
GOWORK=off go test ./... -count=1
# Expected: all PASS, exit 0
```
