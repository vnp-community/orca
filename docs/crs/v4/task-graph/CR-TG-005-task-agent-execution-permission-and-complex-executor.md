# CR-TG-005 — Task→Agent Execution: Permission Precheck, Real `ComplexExecutor`, Context &amp; Env Injection

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-005 |
| **Tên** | `ExecuteTask` precheck quyền + revert status khi fail + `ComplexExecutor` thật (không còn `stub-orchestration-exec:...`) + context/env injection đúng |
| **Loại** | Feature / Bugfix |
| **Priority** | P0 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-TG-001](./CR-TG-001-orcatask-data-model-widening.md) (field), [CR-TG-003](./CR-TG-003-task-access-control-team-scope-and-sharing.md) (permission precheck cần `ResolvePermission` đúng), [CR-TG-004](./CR-TG-004-orchestration-service-coordinator-run-lifecycle.md) (`StartCoordinatorRun` phải tồn tại trước khi `ComplexExecutor` gọi được) |
| **Áp dụng thiết kế** | [SOL-TG-04-task-agent-execution.md](../../../../specs/backend-go/bugs/logic-v1/solutions/SOL-TG-04-task-agent-execution.md) (498 dòng) |
| **Tác động** | `backend-go/services/task-service/internal/usecase/execute_task.go`, `internal/adapter/grpcclient/simple_executor.go`, `complex_executor.go` (thay `StubComplexExecutor`), `agent/src/relay/agent-print-mode-exec.ts` (env field, không đổi RPC contract) |

---

## 1. Vấn đề

1. **Không precheck permission** — `ExecuteTask.Execute`
   (`internal/usecase/execute_task.go:48-76`) chuyển task sang
   `StatusInProgress` **trước khi** kiểm tra độ phức tạp, và **không bao giờ
   gọi `ResolvePermission`** (chỉ gọi `tenant.RequireTenantID`,
   `repo.UpdateStatus`, `isComplex`, executors). Bất kỳ user nào biết `taskId`
   đều có thể Run Agent bất kể grant.
2. **Không revert status khi dispatch fail** — lỗi ở executor bị wrap và trả
   về nguyên trạng (`execute_task.go:72-74`), không có compensating write nào
   — task với dev server offline **kẹt vĩnh viễn ở `in_progress`**.
3. **`ComplexExecutor` là stub cứng**:
   ```go
   // internal/adapter/grpcclient/complex_executor.go:24-26
   func (s *StubComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, prompt string) (string, error) {
       return fmt.Sprintf("stub-orchestration-exec:%s:%s", taskID, requestID), nil
   }
   ```
   Mọi task có subtask/dependency (tức MỌI task đã qua AI decompose —
   CR-TG-002) "thành công" với 1 reference giả, **không thực sự chạy gì**.
4. **`SimpleExecutor` thật nhưng nghèo context** — `buildExecutePrompt`
   (`internal/adapter/grpcclient/simple_executor.go:187-193`) chỉ gửi
   **title trần**, không description/aiContext/parent/dependency context;
   không worktree reuse-or-create; không env-var injection
   (`agentExecPromptParams`, dòng 116-120, chỉ có `{Prompt, WorktreePath,
   StepID}` — thiếu `Env`); không completion callback (không auto-advance
   `review`, không ghi `actual_hours`); không batch/topological execution
   cho nhiều task cùng lúc.
5. **Env injection sai ngay cả khi có field** — `agent-print-mode-exec.ts:104-113`
   set `taskId: stepId ?? ''` khi build `ORCA_TASK_ID` — nếu `SimpleExecutor`
   gửi `StepID: requestID` (dòng 157), agent nhận **request ID, không phải
   task ID thật**. `ORCA_PROJECT_ID` còn tệ hơn: `buildAgentEnv` chỉ set nếu
   `projectId` được truyền vào, mà call site của `agent-print-mode-exec.ts`
   **không bao giờ truyền `projectId`**.

## 2. Giải pháp đề xuất (theo SOL-TG-04)

### 2.1 Permission precheck + status revert

```go
// execute_task.go
func (u *ExecuteTask) Execute(ctx context.Context, taskID, userID string) error {
    perm, err := u.grantResolver.ResolvePermission(ctx, taskID, userID, "execute")
    if err != nil || perm == nil || *perm < domain.PermissionExecute {
        return ErrTaskPermissionDenied // KHÔNG chuyển status trước khi biết chắc được phép
    }
    task, err := u.repo.Get(ctx, taskID)
    if err != nil { return err }
    if err := u.repo.UpdateStatus(ctx, taskID, domain.StatusInProgress); err != nil { return err }

    var execErr error
    if u.isComplex(task) {
        execErr = u.complex.Execute(ctx, task.TenantID, taskID, newRequestID(), u.buildPrompt(task))
    } else {
        execErr = u.simple.Execute(ctx, task)
    }
    if execErr != nil {
        _ = u.repo.UpdateStatus(ctx, taskID, domain.StatusBlocked) // compensating write — KHÔNG kẹt in_progress
        return fmt.Errorf("execute_task: dispatch failed, reverted to blocked: %w", execErr)
    }
    return nil
}
```

### 2.2 `ComplexExecutor` thật — gọi `orchestration-service.StartCoordinatorRun`

```go
// complex_executor.go — thay StubComplexExecutor
type ComplexExecutor struct {
    orchestration orchestrationv1.OrchestrationServiceClient // CR-TG-004's RPC mới
}

func (e *ComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, prompt string) (string, error) {
    resp, err := e.orchestration.StartCoordinatorRun(ctx, &orchestrationv1.StartCoordinatorRunRequest{
        TenantId: tenantID, TaskId: taskID, RequestId: requestID,
    })
    if err != nil { return "", fmt.Errorf("complex_executor: start_coordinator_run: %w", err) }
    return resp.GetRunId(), nil
}
```

### 2.3 `ReportTaskExecutionResult` — callback hoàn tất

```protobuf
// task.proto — service-to-service RPC, orchestration-service gọi ngược lại khi CompleteCoordinatorRun/FailCoordinatorRun
rpc ReportTaskExecutionResult(ReportTaskExecutionResultRequest) returns (google.protobuf.Empty);
```

Cùng RPC này được CR-FLOW-TASK-002 tái sử dụng cho Engine 3 (Workflow) — thêm
field `engine` để usecase phân biệt nguồn log, KHÔNG tạo RPC riêng cho từng
engine.

### 2.4 Context preamble + env injection đúng

```go
// simple_executor.go — buildExecutePrompt() mở rộng
func (e *SimpleExecutor) buildExecutePrompt(task domain.Task, deps []domain.Task) string {
    var b strings.Builder
    fmt.Fprintf(&b, "# Task: %s\n\n%s\n", task.Title, task.Description)
    if task.AIContext != "" { fmt.Fprintf(&b, "\n## Context\n%s\n", task.AIContext) }
    if len(deps) > 0 { fmt.Fprintf(&b, "\n## Đã hoàn thành (dependency)\n%s\n", summarize(deps)) }
    if task.PromptTemplate != "" { fmt.Fprintf(&b, "\n## Hướng dẫn cụ thể\n%s\n", task.PromptTemplate) }
    return b.String()
}

// agentExecPromptParams — thêm Env, sửa StepID→dùng đúng taskID
type agentExecPromptParams struct {
    Prompt       string
    WorktreePath string
    StepID       string            // giữ nguyên cho idempotency key, KHÔNG dùng làm task ID
    Env          map[string]string // MỚI
}
// Execute() — set rõ ràng, không lẫn với StepID:
params.Env = map[string]string{"ORCA_TASK_ID": task.ID, "ORCA_PROJECT_ID": task.ProjectID}
```

Phía `agent/` **không cần đổi RPC contract** (`agent.execPrompt` đã nhận `env`
— `agent-print-mode-exec.ts:44-56`) — chỉ cần backend-go thật sự gửi field
này và không còn lẫn `stepId`/`taskId`.

### 2.5 Worktree reuse-or-create + batch execution

`SimpleExecutor` kiểm tra `task.WorktreeID` đã set chưa trước khi tạo worktree
mới (tránh tạo trùng khi retry); với `ComplexExecutor`, batch execution
(chạy nhiều subtask độc lập song song) do vòng lặp nền của CR-TG-004 đảm
nhiệm — CR này không tự triển khai batching riêng ở tầng `task-service`.

## 3. Rủi ro / Không thuộc phạm vi

- **Cứng phụ thuộc CR-TG-004** — không thể làm `ComplexExecutor` thật nếu
  `StartCoordinatorRun` chưa tồn tại; nếu 2 CR phải ship độc lập, có thể tạm
  giữ `StubComplexExecutor` cho riêng phần này và merge phần `SimpleExecutor`
  + permission precheck trước (chia nhỏ PR, không chặn toàn bộ CR).
- Không giải quyết streaming PTY output liên tục — đó là CR-TG-006 (SOL-TG-04
  tự flag phần này là "deliberately left undesigned").
- Không tự thêm field `taskId`/`projectId` tường minh vào `agent.execPrompt`'s
  RPC contract — dùng `env` map sẵn có là đủ, tránh proto churn không cần
  thiết ở `agent/`.
- Không thuộc phạm vi: sửa `agent-print-mode-exec.ts:76-94`'s giới hạn
  "chỉ hỗ trợ `claude` cho one-shot exec" — đây là giới hạn thật của agent/,
  cần 1 CR riêng nếu muốn hỗ trợ `gemini`/`codex`/`opencode` cho Run Agent.

## Acceptance Criteria

- [ ] `ExecuteTask` gọi `ResolvePermission` với `action="execute"` TRƯỚC khi
      đổi status; user không đủ quyền nhận `TASK_PERMISSION_DENIED`, task
      giữ nguyên status cũ.
- [ ] Dispatch fail (giả lập dev server offline) → task tự revert về
      `blocked`, không kẹt `in_progress` (test có giả lập lỗi executor).
- [ ] `ComplexExecutor` gọi thật `StartCoordinatorRun`, không còn chuỗi
      `stub-orchestration-exec:` ở đâu trong codebase.
- [ ] `ReportTaskExecutionResult` được gọi đúng 1 lần khi coordinator run kết
      thúc (thành công hoặc thất bại), idempotent nếu retry.
- [ ] `buildExecutePrompt` gửi đủ description/aiContext/dependency-summary/
      promptTemplate — có test snapshot prompt output.
- [ ] `agentExecPromptParams.Env` chứa đúng `ORCA_TASK_ID` (task ID thật,
      không phải request/step ID) và `ORCA_PROJECT_ID`; test xác nhận agent
      process nhận đúng 2 biến này (integration test giả lập agent/).
- [ ] Worktree không bị tạo trùng khi `ExecuteTask` retry trên task đã có
      `WorktreeID`.
