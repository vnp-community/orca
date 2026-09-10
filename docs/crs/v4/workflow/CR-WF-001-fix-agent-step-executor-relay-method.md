# CR-WF-001 — Fix Agent Step Executor: `agent.exec` → `agent.execPrompt`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WF-001 |
| **Tên** | Sửa `agent_step_executor.go` gọi đúng relay method + param shape cho step loại `agent` |
| **Loại** | Bugfix (P0 hotfix) |
| **Priority** | P0 — mọi workflow step loại `agent` thất bại ngay hôm nay |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | Không — làm trước tiên trong series |
| **Áp dụng thiết kế** | `specs/backend-go/bugs/logic-v1/solutions/SOL-PRF-04-profile-aware-agent-execution.md` (đoạn "relay method là `agent.execPrompt`, không phải `agent.exec`", dòng 56-76 + code mẫu dòng 320-362), `specs/backend-go/bugs/logic-v1/tasks/TASK-PRF-04-06-workflow-service-agent-executor-env-injection.md` |
| **Tác động** | `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go` |

---

## 1. Vấn đề

```go
// agent_step_executor.go:12-34,58-62 — code hiện tại tự thú trong comment:
// dòng 17-22: "'agent.exec' as the example method for agent steps, and this
// executor found 'agent.exec' IS a real agent RPC — just a different one:
// a generic exec with no prompt/model/trustPreset concept — and had to
// switch to 'agent.execPrompt'" [comment ghi lại phát hiện nhưng CHƯA sửa]
const agentExecMethod = "agent.exec"
// ...
relay(ctx, e.client, cfg.ConnectionID, agentExecMethod, agentExecParams{
    Prompt: cfg.Prompt, WorktreePath: cfg.WorktreePath, TrustPreset: cfg.TrustPreset,
})
```

Handler thật ở `agent/` cho `agent.exec` (`agent/src/relay/agent-rpc-dispatch-agent-exec.ts:72`)
yêu cầu `{binary(required), args?, cwd?, stdin?, env?, timeoutMs?}` — **hoàn
toàn khác** shape `{prompt, worktreePath, trustPreset}` mà
`agent_step_executor.go` đang gửi. Method đúng là `agent.execPrompt`
(`agent-rpc-dispatch-agent-exec.ts:176`), nhận đúng
`{prompt, worktreePath, trustPreset?, model?, accountId?, stepId?, env?,
timeoutMs?}` — **đã được dùng đúng** ở `task-service.SimpleExecutor`
(`backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`),
chứng minh RPC đúng đã sẵn có và hoạt động, chỉ `workflow-service` gọi sai.

`shell_step_executor.go`/`notification_step_executor.go` xác nhận **đúng**
(`shell.exec`/`notification.send` khớp shape) — bug chỉ khoanh vùng ở agent
executor.

## 2. Giải pháp đề xuất

```go
// agent_step_executor.go
const agentExecMethod = "agent.execPrompt" // sửa từ "agent.exec"

type agentExecParams struct {
    Prompt       string            `json:"prompt"`
    WorktreePath string            `json:"worktreePath"`
    TrustPreset  string            `json:"trustPreset,omitempty"`
    Model        string            `json:"model,omitempty"`
    AccountID    string            `json:"accountId,omitempty"`
    StepID       string            `json:"stepId,omitempty"`
    Env          map[string]string `json:"env,omitempty"`
}
```

Đây là **fix tối thiểu** (đổi tên method + field struct) tách riêng khỏi phạm
vi lớn hơn của `SOL-PRF-04` (profile-aware injection đầy đủ, `accountId`/
`model` resolve theo priority chain — thuộc [`CR-WF-002`](./CR-WF-002-server-and-provider-resolution.md)).
CR này chỉ đảm bảo **bước `agent` chạy được**, không đảm bảo chọn đúng
provider/model tối ưu.

## 3. Rủi ro / Không thuộc phạm vi

- Không giải quyết resolve provider/model — đó là CR-WF-002; CR này gửi
  `Model`/`AccountID` rỗng nếu chưa resolve (hành vi fallback mặc định của
  `agent.execPrompt` khi thiếu 2 field này, không phải lỗi mới).
- Không đổi `agent/`'s RPC contract — `agent.execPrompt` đã đúng sẵn, chỉ sửa
  phía gọi.
- Không thuộc phạm vi: `env` injection đúng ngữ nghĩa (`ORCA_TASK_ID` thật vs
  step ID) — đó là vấn đề riêng ở `task-service` (xem
  [`docs/crs/v4/task-graph/CR-TG-005`](../task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md)),
  không lặp lại ở đây cho `workflow-service`'s phiên bản tương tự (nếu tồn
  tại, xử lý trong CR-WF-002 khi thêm resolver).

## Acceptance Criteria

- [ ] `agentExecMethod` = `"agent.execPrompt"`, param struct khớp đúng shape
      handler thật yêu cầu.
- [ ] Test: 1 workflow có step loại `agent` chạy thành công end-to-end (giả
      lập `agent/` trong integration test), không còn lỗi
      `UNKNOWN_METHOD`/shape-mismatch.
- [ ] `shell_step_executor.go`/`notification_step_executor.go` không bị đụng
      tới (đã đúng, chỉ xác nhận qua regression test).
- [ ] `gitnexus_impact({target: "agent_step_executor"})` xác nhận không có
      caller nào khác phụ thuộc vào shape cũ của `agentExecParams`.
