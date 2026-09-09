# CR-WF-003 — `ExecuteRequest.inputs` + `{{outputs.&lt;stepId&gt;.*}}` Interpolation + `action`/`parallel` Step Types

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WF-003 |
| **Tên** | Thêm run-time inputs, variable interpolation giữa các step, và 2 step type còn thiếu (`action`, `parallel`) |
| **Loại** | Feature |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-WF-002](./CR-WF-002-server-and-provider-resolution.md); **phối hợp thời điểm** với [`CR-FLOW-TASK-002`](../../v3/flow-task/CR-FLOW-TASK-002-workflow-as-task-execution-engine.md) — cả 2 CR đều đổi `ExecuteRequest`, nên gộp vào 1 migration proto nếu triển khai gần nhau |
| **Áp dụng thiết kế** | `specs/backend-go/bugs/logic-v1/BUG-WF-02-workflow-execution-partial.md`, `specs/backend-go/bugs/task-v1/BUG-TASKV1-006-workflow-target-resolution-and-step-types-gap.md` |
| **Tác động** | `backend-go/proto/orca/workflow/v1/workflow.proto`, `internal/usecase/execute.go`, `internal/domain/step.go`, `internal/usecase/wave_dispatcher.go` |

---

## 1. Vấn đề

```protobuf
// workflow.proto:84-89 — ExecuteRequest hiện tại
message ExecuteRequest {
  string template_id = 1;
  string project_id = 2;
  string root_trace_id = 3;
  string request_id = 4;
  // KHÔNG có `inputs` — xác nhận qua frontend's own comment (useWorkflow.ts:109-115):
  // "workflow.execute's real shape... no 'inputs' field exists on the RPC...
  //  inputs was silently dropped."
}
```

Không có `inputs` → `{{feature_description}}` (run-time input theo spec) không
có nơi nào để đọc. Không interpolation pass nào tồn tại → `{{outputs.
&lt;stepId&gt;.&lt;field&gt;}}` (tham chiếu output step trước) không bao giờ được thay
thế, step sau nhận nguyên văn chuỗi `"{{outputs.step1.result}}"` thay vì giá
trị thật.

`StepType` enum (`step.go:16-23`, `workflow.proto:54-61`) chỉ có 5 giá trị
(`agent|shell|notification|webhook|condition`) — thiếu `action` (dispatch
built-in git/github/jira actions theo spec) và `parallel` (fan-out nested
sub-step với `allSettled`/`allowPartialFailure` — khác cơ chế wave-level
parallelism đã có ở `wave_dispatcher.go:36-46`, vốn chỉ song song các step
**độc lập trong DAG**, không phải nested sub-step bên trong 1 step).

## 2. Giải pháp đề xuất

### 2.1 `ExecuteRequest.inputs`

```protobuf
message ExecuteRequest {
  string template_id = 1;
  string project_id = 2;
  string root_trace_id = 3;
  string request_id = 4;
  google.protobuf.Struct inputs = 5;      // MỚI — {{feature_description}} v.v.
  string origin_task_id = 6;              // MỚI, phối hợp CR-FLOW-TASK-002 — để trống nếu chạy độc lập
}
```

### 2.2 Interpolation pass — trước khi dispatch từng step

```go
// internal/usecase/interpolate.go (mới)
// Chạy TRƯỚC mỗi lần dispatch step (không phải 1 lần đầu execution) — vì
// {{outputs.<stepId>.*}} chỉ có giá trị SAU KHI stepId đó đã hoàn tất.
func Interpolate(raw string, inputs map[string]any, outputs map[string]StepOutput) string {
    return templatePattern.ReplaceAllStringFunc(raw, func(match string) string {
        path := extractPath(match) // "feature_description" hoặc "outputs.step1.result"
        if strings.HasPrefix(path, "outputs.") {
            return resolveStepOutput(outputs, path)
        }
        return fmt.Sprintf("%v", inputs[path])
    })
}
```

Gọi trong `wave_dispatcher.go` ngay trước khi build param cho từng step
executor (agent/shell/notification) — áp dụng lên toàn bộ field kiểu string
trong step config (prompt/command/params), không chỉ 1 field cố định.

### 2.3 `action` step type

```go
// step.go
const StepTypeAction StepType = "action"

type ActionStepConfig struct {
    Action string         `json:"action"` // "git.createBranch"|"github.createPR"|"jira.createIssue"...
    Params map[string]any `json:"params"`
}
```

Dispatch qua 1 registry built-in action → service tương ứng (git-gateway-service
cho `git.*`, issue-tracking-service cho `github.*`/`jira.*` — tái sử dụng
client gRPC đã có, không tự viết lại tích hợp Git/GitHub/Jira).

### 2.4 `parallel` step type — nested fan-out

```go
const StepTypeParallel StepType = "parallel"

type ParallelStepConfig struct {
    Steps                []Step `json:"steps"`
    AllowPartialFailure  bool   `json:"allowPartialFailure"`
}

// wave_dispatcher.go — executeStep() case StepTypeParallel:
results := make([]StepResult, len(cfg.Steps))
var wg sync.WaitGroup
for i, sub := range cfg.Steps {
    wg.Add(1)
    go func(i int, s Step) { defer wg.Done(); results[i] = e.executeStep(ctx, s) }(i, sub)
}
wg.Wait()
if !cfg.AllowPartialFailure && anyFailed(results) { return ErrParallelStepFailed }
```

## 3. Rủi ro / Không thuộc phạm vi

- **Bắt buộc phối hợp với CR-FLOW-TASK-002** — cả 2 CR đổi `ExecuteRequest`;
  nếu triển khai lệch thời điểm, migration proto sau phải additive (không
  được xoá/đổi field CR trước đã thêm).
- `action` step type's registry built-in action là danh sách mở — CR này chỉ
  định nghĩa cơ chế dispatch, KHÔNG cam kết implement toàn bộ action liệt kê
  trong spec (git/github/jira) — số lượng action cụ thể cần chốt riêng theo
  nhu cầu thực tế trước khi code từng action handler.
- `parallel` step type lồng nhau (parallel-trong-parallel) — CR này không
  cấm về mặt schema nhưng cũng không có test case cho độ sâu &gt; 1; nếu cần,
  làm rõ giới hạn độ sâu trong review trước khi merge.
- Không thuộc phạm vi: UI cho `action`/`parallel` step type — đó là CR-WF-006.

## Acceptance Criteria

- [ ] `ExecuteRequest.inputs` được đọc và dùng đúng cho `{{feature_description}}`-style
      placeholder (test: chạy 1 template có input, xác nhận step nhận đúng
      giá trị interpolate).
- [ ] `{{outputs.&lt;stepId&gt;.&lt;field&gt;}}` resolve đúng giá trị output của step đã
      hoàn tất trước đó trong cùng execution; step tham chiếu 1 stepId
      **chưa chạy** (lỗi thiết kế DAG) trả lỗi rõ ràng thay vì giá trị rỗng
      âm thầm.
- [ ] `action`/`parallel` xuất hiện trong `StepType` enum, `Valid()` chấp
      nhận cả 2.
- [ ] `parallel` step chạy đúng `allSettled` semantics — 1 sub-step fail
      không chặn các sub-step khác; `allowPartialFailure=false` khiến toàn
      bộ step cha fail nếu ≥1 sub-step fail.
- [ ] `origin_task_id` field tồn tại additive, không phá vỡ execution không
      đi qua Task (giá trị rỗng hợp lệ).
