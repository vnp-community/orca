# BE-REQ-SOL-008: Chẩn đoán, Findings, Answer bằng agent chỉ đọc (`AgentReadonlyRunner`)

> **✅ Đã triển khai (kiểm chứng 2026-10-08)** ở `request-service`: use case, repository hai DB, worker có lease, handler Approval và gRPC đã nối vào `main.go`. Hai điểm chờ hợp nhất với đợt Approval (mở Approval thật, bộ hợp đồng handler) ghi ở [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md). Phần thay đổi ở agent (TypeScript) thuộc CR-REQ-033.

**CR:** [CR-REQ-008](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md)
**Service:** `request-service` (mới) · đọc `git-gateway-service` (RPC có sẵn) và `infra-fleet-service` (Relay)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (dữ liệu không tin cậy, che bí mật), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (Relay), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md), [`services/task-service`](../../../../tdd/services/task-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: CR-REQ-008, `task-service/internal/adapter/grpcclient/simple_executor.go` (doc comment: `agent.execPrompt` nhận `stepId, prompt, worktreePath, trustPreset, ...`; `SimpleExecutor` đặt `trustPreset: "full"`; trả `TASK_EXECUTE_NO_WORKTREE_PATH` khi `worktreePath` rỗng), `proto/orca/infrafleet/v1/infrafleet.proto` (`ResolveConnectionResponse{connected, dev_server, repo_path, worktree_id}`, `RelayRequest`), `proto/orca/gitgateway/v1/gitgateway.proto` (`GetStatus`), SOL-007 trong thư mục này (bảng `analysis_runs`, lease, handler `solution`). `request-service` chưa tồn tại: mọi file "(mới)".

**Correction relative to CR-REQ-008**
1. **`GetStatus` không trả `HEAD`.** `GetStatusResponse` chỉ có `files[]` (`path`, `state`) và `branch`; `GetStatusRequest` chỉ nhận `worktree_id` (không nhận đường dẫn). Hai hệ quả: (a) kiểm "sau chạy" chỉ khả thi khi `ResolveConnection` trả `worktree_id` khác rỗng; rỗng thì không kiểm được và run phải ghi `repo_check=skipped` (xem quyết định 3); (b) v1 so sánh `(branch, danh sách file+state)`, chưa so `HEAD`. Thêm `HEAD` cần RPC mới ở `git-gateway-service` (ngoài phạm vi, câu hỏi mở 1). Lệnh Git không thêm, tuân `guides/reference/git-compatibility.md`.
2. **Số migration:** không có migration mới ở solution này (dùng `analysis_runs` của SOL-007). Nếu cần cột `repo_check` thì cộng vào migration của SOL-007 hoặc một migration riêng đọc số tiếp theo lúc làm (task 02).
3. `trustPreset` mặc định `"default"` đã khớp với `simple_executor.go` ("giá trị khác `full` là no-op"): hành vi thực của agent với `default` chưa kiểm chứng.

## 2. Giải pháp

### A. Cây thư mục (mới, trong `backend-go/services/request-service/`)

```
internal/domain/diagnosis_document.go, findings_document.go, answer_document.go   # struct + Validate()
internal/usecase/
    run_agent_readonly_analysis.go     # worker mode=agent_readonly
    analysis_document_validation.go    # chọn validator theo kind
    analysis_secret_redaction.go       # [REDACTED]
    findings_approval_handler.go, answer_approval_handler.go
    solution_prompt.go (sửa, của SOL-007) # nhánh prompt theo kind
    ports.go (sửa)                     # AgentPromptRunner, RepoStateProbe, AnalysisConcurrencyGate
internal/adapter/grpcclient/agent_prompt_relay.go, repo_state_probe.go
internal/adapter/postgres|mysql/analysis_run_repository.go (sửa: CountRunning)
```

### B. Registry và `GenerateSolution` (không RPC mới)

`GenerateSolution` của SOL-007 rẽ nhánh theo `FlowFor(type).Analysis`:

| Loại | `kind` | Mode mặc định | Cổng |
|---|---|---|---|
| `bug`, `performance`, `security` | `diagnosis` | `agent_readonly` | `solution` |
| `hotfix` | `diagnosis` (timeout 300 000 ms) | `agent_readonly` | không; Solution tự `approved` |
| `spike` | `findings` | `agent_readonly` | `findings` |
| `question` | `answer` | `agent_readonly` | `answer` |

`analysis_mode=COMPLETE` ghi đè (câu trả lời chỉ văn bản) và đi qua `AICompleter` của SOL-007. Không hạ cấp âm thầm khi agent lỗi.

### C. Ports

```go
type AgentPromptRunner interface {
    // Relay agent.execPrompt; không có worktree, cwd = repoPath.
    ExecPrompt(ctx context.Context, connectionID string, in AgentPromptInput) (AgentPromptResult, error)
}
type AgentPromptInput struct { StepID, Prompt, RepoPath string; TimeoutMS int; Env map[string]string } // trustPreset cố định "default" trong adapter
type AgentPromptResult struct { Stdout, Stderr string; ExitCode int; TimedOut bool }

type RepoStateProbe interface { // git-gateway GetStatus(worktree_id)
    Snapshot(ctx context.Context, worktreeID string) (RepoSnapshot, error)
}
type RepoSnapshot struct { Branch string; Files []FileState } // FileState{Path, State}; Equal() so sánh sau khi sắp xếp
type AnalysisConcurrencyGate interface { // đếm run running theo (tenant, project) trong transaction
    TryAcquire(ctx context.Context, tenantID, projectID string, max int) (release func(), err error)
}
```
`agent_prompt_relay.go` đặt `params_json = {"stepId":run_id,"prompt":...,"worktreePath":repo_path,"trustPreset":"default","env":{ORCA_REQUEST_ID,ORCA_PROJECT_ID},"timeoutMs":N}`. Không bao giờ gửi token Orca hay credential vào `env`. `trustPreset` không nhận từ ngoài: hằng số `trustPresetReadonly = "default"` kèm test chặn `"full"`.

### D. `RunAgentReadonlyAnalysis.Execute(ctx, runID)`

1. `ResolveConnection`: `connected=false` thì `REQUEST_ANALYSIS_NO_CONNECTION`; `repo_path==""` thì `REQUEST_ANALYSIS_NO_REPO_PATH`.
2. Cổng đồng thời: đếm run `agent_readonly` `running` của `(tenant, project)`; `>= REQUEST_AGENT_READONLY_MAX_PER_PROJECT` (mặc định đề xuất 2) thì `REQUEST_ANALYSIS_BUSY` và run `failed` (chặn ở bước `GenerateSolution`, trước khi chèn run, để không để lại dòng `failed` vô ích). Đếm và chèn run cùng một transaction; trên MySQL đếm bằng `SELECT COUNT(*) ... FOR UPDATE` trên các dòng `running` của project (không có partial index).
3. `before := probe.Snapshot(worktree_id)` nếu `worktree_id != ""`, nếu không đánh dấu `repoCheck=skipped` và log cảnh báo.
4. Dựng prompt theo `kind` (khối `<request>` là dữ liệu, chỉ dẫn chỉ đọc, "đúng một JSON, không code fence", `prior_artifacts`).
5. `ExecPrompt`. `TimedOut` thì `REQUEST_ANALYSIS_TIMEOUT`; `ExitCode != 0` thì `REQUEST_ANALYSIS_AGENT_FAILED`.
6. `after := probe.Snapshot(...)`; `!before.Equal(after)` thì run `failed` `REQUEST_ANALYSIS_REPO_MODIFIED`, bỏ kết quả, phát sự kiện nội bộ `analysis.repo_modified` (log + metric; thông báo admin là việc của task 04, dùng kênh có sẵn, xem câu hỏi mở 4).
7. Trích JSON (bộ trích của SOL-007), `ValidateByKind`, một lần thử lại với lỗi cụ thể (`attempt=2`), vẫn sai thì `REQUEST_ANALYSIS_INVALID_OUTPUT`.
8. Che bí mật (`RedactSecrets`: PEM private key, `ghp_[A-Za-z0-9]{36}`, `AKIA[0-9A-Z]{16}`, JWT `eyJ...\..*\..*`, `password\s*[=:]\s*\S+`) trên `excerpt`, `answer_markdown`, `raw_output` trước khi lưu.
9. Transaction persist: Solution `proposed`; nếu `FlowFor.GateSubject != ""` thì `OpenApproval`; `TransitionRequest(analysis_ready)`. `hotfix`: Solution `approved` ngay, outbox `solution.approved{auto:true, mode}`, không Approval, đích `awaiting_plan_approval` (do registry).

### E. Schema tài liệu

Giữ nguyên JSON ở CR-REQ-008 mục 2.4 (`schema_version: 1`, tối đa 64 KB, `confidence` thuộc `[0,1]`, `evidence` tối đa 20 mục, `excerpt` tối đa 600). Cột `options` chứa tài liệu; `chosen_option` NULL; `content_ref` rỗng. `ValidateByKind`:

```go
func (d DiagnosisDocument) Validate() error // root_cause.statement bắt buộc; severity thuộc tập; fix_directions.id duy nhất
func (d FindingsDocument) Validate() error  // >=1 findings; mỗi finding có evidence hoặc nằm trong unknowns
func (d AnswerDocument) Validate() error    // 1<=len(answer_markdown)<=8000
```
`suggest_escalate_to_change_request` chỉ hiển thị; không đổi `requests.type`. `measurements[]` chỉ chừa chỗ (đo thật thuộc CR-REQ-014).

### F. Handler Approval

`findings_approval_handler.go`, `answer_approval_handler.go` cùng khuôn `SolutionApprovalHandler`: `ValidateForRequest` kiểm đúng `kind` và `proposed`, digest `DigestOptions(options, nil)`; `OnApproved` ghi `approved`, outbox `solution.approved`, `TransitionRequest(analysis_approved)`; registry `CompletesAfterAnalysis=true` đưa `spike`/`question` sang `completed` (không Plan, không Task). `OnRejected` về backlog `returned_from_stage=analysis`. Cả hai đăng ký tại composition root; thiếu thì service không khởi động (SOL-009).

### G. Giao diện với agent (không sửa ở đây, thuộc CR-REQ-033)

Backend gọi `agent.execPrompt` với tham số hiện có. Nếu CR-REQ-033 thêm tham số chỉ đọc (dự kiến `readOnly: true` hoặc `allowedTools`), `agent_prompt_relay.go` chỉ cần thêm một khoá vào `params_json`, bật bằng cờ `REQUEST_AGENT_READONLY_USE_AGENT_FLAG` (mặc định tắt) để không gửi tham số lạ tới agent cũ. Tên tham số do CR-REQ-033 chốt; solution này không giả định. Hợp đồng không đổi: kết quả vẫn `{stdout, stderr, exitCode, timedOut, stepId}`.

### H. Lỗi, sự kiện

Mã `REQUEST_ANALYSIS_{NO_CONNECTION,NO_REPO_PATH,BUSY,AGENT_FAILED,TIMEOUT,REPO_MODIFIED,INVALID_OUTPUT,KIND_NOT_ALLOWED}`. Sự kiện dùng lại `solution.proposed` và `solution.approved`, thêm `mode`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Không dùng `ExecuteTask` | Cần Task, worktree, đổi status; vứt stdout |
| 2 | Chạy trên `repo_path`, không tạo worktree | Tránh `worktree.created` kích hoạt đồng bộ Jira sai |
| 3 | Kiểm sau chạy chỉ khi có `worktree_id`; ghi `repo_check=skipped` khi không | `GetStatus` cần `worktree_id`; không bịa chống |
| 4 | `trustPreset` là hằng, không tham số | Không để đường nào đặt `full` |
| 5 | Che bí mật trước khi lưu, không tin tuyệt đối | Giảm thiểu, độ phủ mẫu chưa kiểm chứng |
| 6 | Đếm đồng thời trong transaction | MySQL không có partial index |

## 4. Phụ thuộc và thứ tự

Cần SOL-007 (bảng run, lease, `GenerateSolution`, handler `solution`), SOL-009 (`OpenApproval`, `SubjectHandler`), CR-REQ-003 (registry với `GateSubject`, `CompletesAfterAnalysis`). Mở khoá CR-REQ-014 và CR-REQ-006. Ghi tiến: CR-REQ-033 (agent chỉ đọc), CR-REQ-028 (Clarification) có thể thêm trạng thái trước `analyzing`; không phụ thuộc.

## 5. Kiểm thử

- Unit domain: ba `Validate` (bảng đúng/sai), `RedactSecrets` (mẫu dương/âm, tiếng Việt có dấu).
- Unit usecase (fake Relay, probe, gate): thành công, exit khác 0, timeout, repo bị sửa, `worktree_id` rỗng (skipped), quá giới hạn, mất lease, JSON sai rồi đúng, hotfix tự duyệt, `kind` không hợp lệ.
- Test chặn: adapter không bao giờ đặt `trustPreset="full"`, `env` không chứa khoá nhạy cảm; fake git-gateway ghi nhận 0 lời gọi tạo worktree.
- Integration hai DB: đếm đồng thời đúng (MySQL trong transaction), test repository của SOL-007 với `mode=agent_readonly`.
- Hợp đồng: golden JSON cho ba schema; test `SubjectHandler` findings/answer.
- Thủ công, chưa kiểm chứng: chạy `agent.execPrompt` thật với `trustPreset=default` trên dev server cục bộ và SSH.

## 6. Rủi ro và điểm chưa kiểm chứng

- Không có chế độ chỉ đọc thật: agent có shell trên repo gốc, Request đến từ nguồn ngoài. Giảm thiểu: người xác nhận loại trước, prompt, kiểm sau chạy, che bí mật. Chờ CR-REQ-033.
- `repo_path` có thể bẩn sẵn; so sánh trước và sau, không so với "sạch".
- `trustPreset=default` có thể chặn mọi công cụ hoặc treo chờ quyền (chưa kiểm chứng).
- Chi phí và timeout (600 000 ms, hotfix 300 000 ms) là đề xuất.

## 7. Câu hỏi mở

1. Thêm `HEAD` vào kiểm sau chạy: cần RPC mới ở `git-gateway-service` hay chấp nhận `(branch, files)` ở v1?
2. Chấp nhận mức chỉ đọc "quy trình" cho v1 hay chặn đến khi CR-REQ-033 xong (khuyến nghị: chấp nhận kèm cảnh báo).
3. `GetAnalysisRun` hoặc stream tiến độ cho UI (README 3.6 không có); hiện dùng `ListSolutionsResponse.runs`.
4. Thông báo admin khi `REPO_MODIFIED`: dùng sự kiện nào của `notification-service` (không có trong hợp đồng 3.7).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/execute_task.go`
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `proto/orca/gitgateway/v1/gitgateway.proto`
- `/opt/repos/orca/guides/reference/git-compatibility.md`
- `/opt/repos/orca/specs/backend-go/crs/v6/solution-analysis/solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md`
