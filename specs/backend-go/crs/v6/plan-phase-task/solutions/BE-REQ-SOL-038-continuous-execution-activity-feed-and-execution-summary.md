# BE-REQ-SOL-038: Thực thi liên tục sau một lần duyệt, nhật ký hoạt động, tóm tắt thực thi, `engine` và `execution_mode` trên Request

> **✅ Đã triển khai (8/8 task, 2026-10-16).** Kiểm chứng: unit test, hợp đồng trên Postgres 16 và MySQL 8 (testcontainers), parity MCP, triển khai trên stack local và e2e thật qua giao diện (`run_ui_flow.py --real continuous`, PASS). Chưa chạy: run OpenSpec sống trên repo có knowledge hub. Sai khác so với bản soạn: không thêm chỉ mục mới (task 01); biến môi trường là `REQUEST_DEFAULT_EXECUTION_MODE`; `execution_mode` của Request chưa ghim được điền theo cài đặt dự án để màn hình mô tả được thẻ duyệt (task 04); số lần thử chỉ đếm lần chạy dùng ngân sách thử lại (task 05, 06).

**CR:** [CR-REQ-038](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md)
**Service:** `request-service` · `proto/orca/request/v1` · `api-gateway` (`wscompat`) · `mcp-service` (parity)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần, use case đọc), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, chỉ mục, RLS), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (quyền đọc, quyền thực thi)
**Phụ thuộc solution:** [BE-REQ-SOL-013](./BE-REQ-SOL-013-phase-execution-and-feedback-loop.md) (`StartPhase`, `EvaluateExecution`, `task_run_outcomes`), [BE-REQ-SOL-014](./BE-REQ-SOL-014-type-specific-execution-policies.md) (cổng `pre_deploy`). Frontend dùng kết quả: [FE-REQ-SOL-037](../../../../../frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-16: `internal/domain/request_flow_registry.go` (`flowDefinitions`, `FlowFor`, `PhasesFor`, `HasExecutionGate`), `internal/domain/engine_settings.go` (`ProjectEngineSettings`, `EffectiveEngine`, `EngineDefaults`), `internal/domain/openspec_profile.go`, `internal/domain/execution_tree.go` (`NextReadyPhase`), `internal/usecase/{start_execution,start_phase,evaluate_execution,advance_execution,ports_execution}.go`, `internal/config/config.go`, `proto/orca/request/v1/request.proto` (`Request` trường 1 đến 31, `GetRequestFlowResponse`, `GetProjectEngineSettings`, `SetProjectEngineSettings`), `internal/adapter/grpc/{request_mapper,server_engine_settings,server_execution}.go`, `api-gateway/.../wscompat/{channels_request_flow,channels_request_engine}.go`, migration mới nhất `0097_openspec_change_sync_progress` ở `postgres` và `mysql`.

- `change_request` là loại duy nhất có `ExecutionGates: [GatePhase]`. `ops_request` có `GatePreDeploy` trong `executing`. `bug`, `refactor` (cỡ L) có Phase nhưng không có cổng Phase.
- `StartExecution.Execute` trả `started=false` cho mọi luồng có Phase (`flow.PhasesFor(req.Size)`) và chỉ gọi `Evaluate.Run`; Phase đầu và các Phase sau chờ `StartPhase` của người. `StartPhaseInput.Internal` đã có nhưng chưa có đường gọi nào đặt `Internal: true` cho Phase.
- `EvaluateExecution.completePhases` (dòng ~245 đến 300): khi `flow.HasExecutionGate(GatePhase)`, mở Approval `phase` cho Phase sẵn sàng kế tiếp bằng `e.Open.Execute`. Khi không có cổng thì chỉ ghi `phase_done`.
- `GetProjectEngineSettings` và `SetProjectEngineSettings` đã có (gateway: `request.engineSettings`, `request.engineSettingsSet`); bảng `project_engine_settings` đã có `engine`, `min_version`, `solution_agent` (migration `0095`).
- `Request` proto dừng ở trường 31; không có `engine`, `execution_mode`. `GetRequestFlowResponse` không có hồ sơ OpenSpec. Gateway có `request.flow` nhưng frontend chưa dùng.
- `task_run_outcomes` có `outcome` (`started|succeeded|failed|cancelled|phase_done|plan_done`), `cause`, `error_message`, `occurred_at`; chưa có RPC nào đọc cho UI. `REQUEST_MAX_TASK_ATTEMPTS` mặc định 2 (`config.go`).
- Các nguồn của nhật ký đã có bảng: `request_type_history`, `request_return_history`, `approvals`, `decisions`/`decision_history`, `clarifications`, `solutions`, `analysis_runs`, `classification_runs`, `request_revisions`, `phase_starts`, `task_run_outcomes`.

### Correction relative to CR-REQ-038

1. CR viết "mở rộng RPC cài đặt engine": đã xác nhận `Get/SetProjectEngineSettings` tồn tại, nên chỉ thêm trường `execution_mode`, không thêm RPC mới.
2. CR nêu cột `request_revisions` như một nguồn "request.created": bản ghi `cause=created` có `snapshot` (nội dung Request); `ListRequestActivity` **không** được trả `snapshot`, chỉ `summary` ngắn (task 05).
3. CR giả định `phase_starts` chỉ ghi người bấm; `StartPhase` có nhánh `Internal` cần `ActorID` rõ ràng, nên `phase_starts.started_by` của lần tự chạy là người duyệt Plan (task 03).
4. CR chưa nêu `StartExecution` còn có nhánh dùng `Evaluate.Run` cho luồng có Phase để mở cổng đầu tiên; với `continuous` nhánh này phải thay bằng tự bắt đầu Phase đầu (task 03).

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `backend-go/services/request-service/`)

```
migrations/{postgres,mysql}/0098_execution_mode_and_activity_indexes.{up,down}.sql   (mới, kiểm số lúc làm)
internal/domain/execution_mode.go            ExecutionMode, ParseExecutionMode, FlowDefinition.EffectiveExecutionGates(mode)   (mới)
internal/domain/execution_summary.go         ExecutionSummary, NeedsYouItem, FailedTask, RunningTask, HandoverState   (mới)
internal/domain/request_activity.go          ActivityEntry, ActivityKind, ActivitySeverity, ActivityFilter, ActivityCursor   (mới)
internal/domain/failure_cause.go             FailureCause (timed_out|model_error|verify_failed|env_defect|spec_defect|agent_error|cancelled|unknown)   (mới)
internal/domain/engine_settings.go           thêm ExecutionMode vào ProjectEngineSettings, EngineDefaults   (sửa)
internal/domain/request.go                   thêm Engine, ExecutionMode   (sửa)
internal/usecase/start_execution.go          nhánh continuous tự bắt đầu Phase đầu   (sửa)
internal/usecase/evaluate_execution.go       completePhases: continuous thì StartPhase Internal thay cho mở Approval   (sửa)
internal/usecase/start_phase.go              ghi phase_starts.trigger   (sửa)
internal/usecase/approval_artifact_handler.go   kiểm quyền thực thi (ExecutionAccess.Ensure) khi OnApproved của Plan, ghim mode   (sửa)
internal/usecase/list_request_activity.go    (mới)
internal/usecase/get_execution_summary.go    (mới)
internal/adapter/{postgres,mysql}/request_activity.go     UNION các nguồn   (mới)
internal/adapter/{postgres,mysql}/task_run_outcomes.go    thêm ListByRequest, CountAttempts   (sửa)
internal/adapter/{postgres,mysql}/phase_starts.go         cột trigger   (sửa)
internal/adapter/{postgres,mysql}/project_engine_settings_repository.go   cột execution_mode   (sửa)
internal/adapter/grpc/request_mapper.go      map engine, execution_mode   (sửa)
internal/adapter/grpc/server_activity.go     ListRequestActivity, GetExecutionSummary   (mới)
proto/orca/request/v1/request.proto          trường mới và hai RPC   (sửa)
api-gateway/.../wscompat/channels_request_activity.go     request.activity, request.executionSummary   (mới)
mcp-service/.../tools/                        ToolSpec request_activity, request_execution_summary hoặc dòng loại trừ parity   (sửa)
```

### 2.2 Chế độ thực thi

```go
type ExecutionMode string // "step_by_step" | "continuous"

func (f FlowDefinition) EffectiveExecutionGates(m ExecutionMode) []GateSubject // bỏ GatePhase khi continuous
```

- Nguồn: `project_engine_settings.execution_mode` (mặc định `step_by_step`), mặc định môi trường `REQUEST_DEFAULT_EXECUTION_MODE` cho dự án chưa có cài đặt (`EngineDefaults`).
- **Ghim** lúc Request vào `executing` (`requests.execution_mode`); mọi nơi đọc cổng Phase (`start_phase.go`, `evaluate_execution.go`, `backlog_gate.go`, `approval_subject_stage.go`) đọc qua `EffectiveExecutionGates` với mode đã ghim. Request chưa ghim (dữ liệu cũ) coi là `step_by_step`.
- `StartExecution.Execute` với `continuous` và `PhasesFor(size)`: gọi `StartPhase{Internal:true, ActorID: <người duyệt Plan>}` cho Phase đầu sẵn sàng (`NextReadyPhase`), rồi `Evaluate.Run`.
- `EvaluateExecution.completePhases` với `continuous`: sau khi ghi `phase_done`, nếu không Phase nào đang chạy thì chọn `nextReadyPhase` và gọi `StartPhase{Internal:true}` thay cho `e.Open.Execute(... SubjectPhase ...)`. Vẫn tôn trọng `depends_on`.
- Người thực thi: `approvals.decided_by` của Approval khởi đầu (`StartGate`). Lúc duyệt, kiểm quyền thực thi cây Plan (qua `ExecutionAccess`/`task-service`); thiếu quyền thì từ chối với `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` và không đổi trạng thái.
- Thu rủi ro: Approval của Plan khi `continuous` mang danh sách phát hiện rủi ro của mọi Phase (đã chốt B8). Phần UI ở FE-REQ-SOL-037; backend chỉ cần `impact.*` trả theo Phase và `approval.approve` nhận `acceptedFindingIds` bao gồm mọi Phase (CR-REQ-030 đã có trường).
- `phase_starts.trigger` ∈ {`manual`, `auto`}.

### 2.3 `ListRequestActivity`

UNION các nguồn theo bảng CR-REQ-038 mục 2.2; mỗi nguồn là một truy vấn có giới hạn và sắp theo `(occurred_at DESC, id)`, hợp nhất ở tầng use case bằng k-way merge (mỗi nguồn trả tối đa `page_size + 1`), tránh một `UNION ALL` khổng lồ khác nhau giữa hai dialect. Con trỏ mã hoá `(occurred_at, source, id)`. Loại dòng của đối tượng người xem không đọc được. Không trả `body`, `snapshot` hay log agent. `details` giới hạn 4 KB.

### 2.4 `GetExecutionSummary`

Tính trong một lần gọi: cây thực thi từ `task-service` (`ListTasks` theo `RequestIDs`, như `StartPhase`), `task_run_outcomes` (đếm lần thử, dòng `failed` mới nhất), Approval `pending` và Clarification mở của Request (lọc theo quyền người xem), `openspec_changes` cho `handover` khi engine `openspec`. `failure_cause` chuẩn hoá bằng `domain.ClassifyFailureCause(outcome)`; dữ liệu cũ không map được thì `unknown`.

### 2.5 Mã lỗi mới

`REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` (FailedPrecondition), `REQUEST_DEFAULT_EXECUTION_MODE_INVALID` (InvalidArgument, khi `SetProjectEngineSettings` nhận giá trị lạ), `REQUEST_ACTIVITY_CURSOR_INVALID` (InvalidArgument).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| `EffectiveExecutionGates` trong domain, không rẽ nhánh theo mode ở use case | Đổi luật ở một nơi; test 11 loại × 2 mode |
| Ghim `execution_mode` ở `requests`, không đọc cài đặt dự án mỗi lần | Đổi cài đặt không đổi Request đang chạy |
| Tự bắt đầu Phase dùng `StartPhase{Internal:true}` sẵn có | Một đường thực thi duy nhất, giữ `phase_starts` làm claim idempotent |
| K-way merge ở use case cho nhật ký | Hai dialect khác nhau ở `UNION`/`ORDER BY`/kiểu thời gian; truy vấn từng nguồn đơn giản và dùng chỉ mục |
| `GetExecutionSummary` đọc, không ghi | Có thể gọi lặp 5 giây một lần mà không tác dụng phụ |
| Không thêm Pause/Resume, không thêm cột `execution_paused` | CR-038 B9 |

## 4. Phụ thuộc và thứ tự

Cần SOL-013 (đã làm), SOL-014 (`pre_deploy`), CR-REQ-030 (rủi ro theo Phase), CR-REQ-035 (quyền đọc Request). Thứ tự task: 01 migration và repository; 02 domain và cài đặt; 03 thực thi liên tục; 04 trường Request và flow; 05 nhật ký; 06 tóm tắt thực thi; 07 gateway và MCP; 08 kiểm thử tích hợp và e2e. 05 và 06 song song sau 01; 04 song song sau 02.

## 5. Kiểm thử

- **Unit:** `EffectiveExecutionGates` (11 loại × 2 mode); ghim và đọc mode; ánh xạ `ActivityKind`/`severity` từng nguồn; k-way merge và con trỏ (nhiều dòng cùng `occurred_at`); đếm lần thử và `last_cause`; `ClassifyFailureCause`.
- **Integration hai dialect:** repository nhật ký (UNION từng nguồn), `phase_starts.trigger`, `project_engine_settings.execution_mode`, chỉ mục (`EXPLAIN`).
- **Luồng (`internal/adapter/contracttest/execution_flow_contract.go`):** kịch bản `continuous` 3 Phase, lỗi task giữa chừng, `drift`, `ops_request` với `pre_deploy`, người duyệt thiếu quyền, đổi mode giữa chừng.
- **Gateway và MCP:** test kênh mới; `parity_test.go`.
- Lệnh dự kiến: `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'ExecutionMode|Activity|ExecutionSummary|Continuous' -v` (chưa chạy).

## 6. Rủi ro và điểm chưa kiểm chứng

- Hiệu năng nhật ký với Request có hàng trăm dòng `task_run_outcomes` chưa đo; cần `EXPLAIN` hai dialect.
- Số migration `0098`, trường proto 32 và 33 có thể bị CR khác dùng; kiểm lại lúc làm.
- `ExecutionAccess` chưa rõ có kiểm "quyền thực thi" đủ ở thời điểm duyệt (task 03 phải đọc lại trước khi sửa).
- Chuẩn hoá nguyên nhân lỗi phụ thuộc nơi ghi `cause` hiện tại; chưa biết vì sao #1242 ghi `TIMED_OUT` trong khi log cuối là `unrecognized_model`.
- `openspec_changes` có thể chưa có trường archive đủ cho `handover` (câu hỏi mở 2 của CR).

## 7. Câu hỏi mở

- Người duyệt thứ hai (rủi ro nghiêm trọng) có tính là người duyệt Plan để kiểm quyền? (CR mục 7, câu 1)
- Archive và bàn giao OpenSpec xảy ra ở `executing` hay `completed`; cột nào của `openspec_changes`? (CR mục 7, câu 2)
- Bộ mã `failure_cause` có khớp `Failure.class` của CR-REQ-029? (CR mục 7, câu 3)

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md`; `CR-REQ-013`, `CR-REQ-014`
- `backend-go/services/request-service/internal/{domain,usecase}/` các file nêu ở mục 1; `backend-go/proto/orca/request/v1/request.proto`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_flow.go`, `channels_request_engine.go`
- [BE-REQ-SOL-013](./BE-REQ-SOL-013-phase-execution-and-feedback-loop.md), [BE-REQ-SOL-014](./BE-REQ-SOL-014-type-specific-execution-policies.md)
