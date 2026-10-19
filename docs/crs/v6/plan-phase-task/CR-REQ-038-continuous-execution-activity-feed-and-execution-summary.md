# CR-REQ-038 — Thực thi liên tục sau một lần duyệt, nhật ký hoạt động và tóm tắt thực thi của Request

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-038 |
| **Tên** | Chế độ thực thi `continuous` (duyệt Plan một lần, các Phase tự chạy tuần tự), API nhật ký hoạt động `request.activity`, API tóm tắt thực thi `request.executionSummary`, thêm `engine` và `execution_mode` vào Request |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-16 |
| **Trạng thái** | ✅ Đã triển khai 2026-10-16 (8/8 task; quyết định B8 đến B12 đã chốt; e2e thật qua giao diện PASS với engine native, chưa chạy run OpenSpec sống; còn 3 câu hỏi mở ở mục 7) |
| **Phụ thuộc** | CR-REQ-003 (registry luồng), 009, 010 (Approval), 013 (`StartPhase`, `EvaluateExecution`, `task_run_outcomes`), 016 (kênh gateway), 026 (engine OpenSpec), 028 (Clarification), 030 (rủi ro, `impact.drift_detected`), 035 (quyền, audit) |
| **Mở khoá** | CR-REQ-037 (giao diện chi tiết Request: tab Execution, History, thang tiến độ) |
| **Tác động** | `backend-go/services/request-service` (`internal/domain/request_flow_registry.go`, `internal/usecase/{start_execution,start_phase,evaluate_execution,advance_execution}.go`, use case đọc mới, migration `0098` hai dialect, proto `request.proto`), `backend-go/services/api-gateway/internal/adapter/wscompat` (kênh mới), `mcp-service` (ToolSpec hoặc dòng loại trừ cho `parity_test`) |

> Nguồn: khảo sát request #1242 trên UI thật và đọc code ngày 2026-10-16 (mục 1). Frontend tương ứng: [CR-REQ-037](../request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md).

---

## 1. Bối cảnh và vấn đề

Người dùng chạy Request `change_request` (#1242, 7 Phase, 28 task) và gặp ba vấn đề mà backend là một phần nguyên nhân:

1. **Phải duyệt và bấm "Start phase" cho từng Phase.** Kết quả khảo sát code:
   - `domain/request_flow_registry.go`: chỉ `change_request` có `ExecutionGates: [GatePhase]`. `ops_request` có `GatePreDeploy` trong lúc `executing`. Các loại có Phase theo cỡ (`bug`, `refactor` khi `size=L`) không có cổng Phase.
   - `usecase/start_execution.go`: mọi luồng có Phase (`flow.PhasesFor(size)`) **luôn chờ người gọi `StartPhase`** cho từng Phase, kể cả khi không có cổng duyệt. Chỉ Plan không Phase, task list và hotfix tự chạy.
   - `usecase/start_phase.go` có `StartPhaseInput.Internal` ("bỏ qua kiểm tra người gọi") nhưng không có chỗ nào trong code chạy (ngoài test) đặt `Internal: true` cho một Phase; mới chỉ dùng cho nhánh Plan không Phase.
   - `usecase/evaluate_execution.go` (`completePhases`): khi một Phase xong và luồng có cổng Phase, nó **mở Approval cho Phase kế tiếp** và dừng, chờ người.
   Trên #1242: 7/7 Phase phải duyệt riêng, người dùng không thể "duyệt một lần rồi để agent làm hết".
2. **Không có API lịch sử.** Tab History của frontend chỉ gọi `request.typeHistory` nên chỉ thấy một dòng đổi loại, trong khi dữ liệu đã nằm ở nhiều bảng: `request_return_history`, `approvals`, `decision_history`, `task_run_outcomes`, `phase_starts`, `clarifications`, `classification_runs`, `analysis_runs`, `solutions`, `request_revisions`.
3. **Frontend không biết đủ thông tin để hiển thị tiến độ thực thi và lỗi.**
   - Số lần thử và nguyên nhân lỗi từng lần chỉ có trong `task_run_outcomes` (`outcome`, `cause`, `error_message`, `occurred_at`); chưa có RPC đọc. Trên #1242, task #TG-1037 lỗi 2 lần nhưng UI hiện "Open", banner nói `TIMED_OUT` còn log cuối của agent là `unrecognized_model` (chưa rõ lần nào lỗi do gì).
   - `Request` (proto `request.proto`, trường 1 đến 31) không có `engine` hay `execution_mode`. Frontend chỉ biết engine qua `request.planProposal` lúc sinh plan. `EffectiveEngine(pinned, settings, profile)` ở `domain/engine_settings.go` quyết định engine; `OpenSpecProfileFor`: `change_request`, `refactor` = `Full`; `bug`, `security`, `performance` = `Light`; còn lại `None` (luôn native).
   - Frontend không dùng kênh `request.flow` (đã có ở `api-gateway/.../channels_request_flow.go`) mà giữ bảng tĩnh `REQUEST_FLOW_REGISTRY` đã **lệch** với `FlowFor` (xem CR-REQ-037, mục 2.2).

Hiện trạng code (đọc 2026-10-16): migration mới nhất của `request-service` là `0097_openspec_change_sync_progress` ở cả Postgres và MySQL; tên bảng `project_engine_settings` đã có cột `solution_agent` (migration `0095`), là tiền lệ cho một cờ "cách dự án này chạy Request".

## 2. Giải pháp đề xuất

### 2.1 Chế độ thực thi `step_by_step` và `continuous`

Thêm khái niệm `execution_mode` ∈ {`step_by_step` (hành vi hiện tại), `continuous`}.

| | `step_by_step` (giữ nguyên) | `continuous` (mới) |
|---|---|---|
| Duyệt Plan | `StartGate` (plan, task_list, pre_deploy) như hiện tại | như hiện tại, và **là lần duyệt duy nhất** cho phần thực thi |
| Cổng Phase của `change_request` | mở Approval `phase` cho từng Phase | **không mở**; Phase kế tiếp tự bắt đầu |
| Bắt đầu Phase (luồng có Phase) | người gọi `StartPhase` | hệ thống gọi `StartPhase{Internal: true}` cho Phase đầu khi vào `executing`, và cho Phase sẵn sàng kế tiếp khi Phase trước xong |
| Cổng `pre_deploy` trong `executing` (`ops_request`) | giữ | **giữ**, vẫn dừng chờ người |
| Cổng `pre_deploy` làm `StartGate` (`hotfix`, `security`) | giữ | giữ |

Cách làm:

- `domain/execution_mode.go` mới: kiểu `ExecutionMode` và `FlowDefinition.EffectiveExecutionGates(mode)`: bỏ `GatePhase` khi `continuous`. `HasExecutionGate(GatePhase)` ở `start_phase.go:91` và `evaluate_execution.go:256` đọc qua hàm này với mode đã ghim của Request.
- **Ghim mode theo Request** lúc Request vào `executing` (cột `requests.execution_mode`), giống cách ghim engine, để đổi cài đặt giữa chừng không làm một Request đang chạy đổi luật. Request cũ (cột rỗng) coi như `step_by_step`.
- Nguồn cài đặt: cột `execution_mode` mới ở `project_engine_settings` (mặc định `step_by_step`); biến môi trường `REQUEST_DEFAULT_EXECUTION_MODE` làm mặc định khi dự án chưa có cài đặt (cùng mẫu `EngineDefaults`, `engine_settings_defaults.go`). Cài đặt **theo dự án** (đã chốt, D-B10): mở rộng RPC cài đặt engine hiện có của `project_engine_settings` thêm trường `execution_mode`, mặc định `step_by_step`. Người duyệt **không** chọn chế độ trên thẻ duyệt Plan trong v6.
- `StartExecution.Execute`: với `continuous` và `flow.PhasesFor(size)` thì **tự bắt đầu Phase đầu**; hiện nhánh này trả `started=false` và chờ người.
- `EvaluateExecution.completePhases`: khi `continuous`, thay vì `e.Open.Execute(... SubjectPhase ...)` thì gọi `StartPhase{Internal: true, ActorID: <người duyệt Plan>}` cho `nextReadyPhase`. Vẫn tôn trọng cạnh `depends_on` (`NextReadyPhase`) và chỉ có một Phase chạy tại một thời điểm.
- **Người thực thi** là người đã duyệt Approval khởi đầu (`approvals.decided_by` của `StartGate`); task-service kiểm quyền `execute` theo người này. Kiểm quyền **ngay lúc duyệt**: nếu người duyệt không đủ quyền thực thi cây Plan thì từ chối duyệt với lỗi mới `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` thay vì để Request kẹt giữa chừng.
- `phase_starts` thêm cột `trigger` ∈ {`manual`, `auto`} để lịch sử phân biệt "người bấm" và "tự chạy" (mục 2.3).

**Điều kiện dừng** (không cần cơ chế mới, chỉ phải đảm bảo vẫn hoạt động khi `continuous`):

| Tình huống | Cơ chế hiện có | Kết quả |
|---|---|---|
| Agent cần thông tin | Clarification, `awaiting_information` (CR-REQ-028) | Dừng chờ người trả lời, rồi chạy tiếp |
| Task lỗi quá `REQUEST_MAX_TASK_ATTEMPTS` | `ReturnToBacklog` | Request về `request_backlog`; Phase kế tiếp không tự bắt đầu |
| Lệch kế hoạch | `impact.drift_detected` (CR-REQ-030, 036) | Phase tạm dừng chờ người xem lại |
| Cổng `pre_deploy` trong `executing` | `ExecutionGates` | Mở Approval, dừng |
| Người dùng huỷ | `CancelRequest` | Dừng; không bắt đầu Phase mới |

**Rủi ro khi bỏ cổng Phase (đã chốt, D-B8).** Hiện từng Phase mang `ApprovalRiskGateSection` (chấp nhận rủi ro theo Phase, CR-REQ-036). Khi `continuous`, chấp nhận rủi ro của **mọi Phase** được thu thập ngay trên Approval của Plan (một lần, gồm danh sách phát hiện của tất cả Phase). Phase vượt ngưỡng sau khi chạy (`drift`) vẫn dừng chờ người. Không có cổng lai theo mức rủi ro trong v6.

**Tạm dừng và dừng chạy (ngoài phạm vi v6, đã chốt D-B9).** Chỉ có "Dừng" bằng `CancelRequest` hiện có. `PauseExecution` và `ResumeExecution` (cờ `requests.execution_paused`; `AdvanceExecution` không dispatch task mới khi cờ bật) để dành cho CR sau; không thêm cột `execution_paused` ở migration `0098`.

### 2.2 `request.activity`: nhật ký hoạt động

RPC `ListRequestActivity` (gateway `request.activity`) trả sự kiện đã lưu, mới nhất trước, phân trang bằng con trỏ `(occurred_at, id)`. **Không cần kho sự kiện mới**: đây là read model gộp (UNION) các bảng có sẵn, mỗi nguồn ánh xạ sang một `kind`.

| Nguồn | `kind` | Nội dung chính |
|---|---|---|
| `request_revisions` (`cause=created`) | `request.created` | người tạo, nguồn |
| `classification_runs`, `request_type_history` | `request.classified`, `request.type_changed` | từ loại nào sang loại nào, lý do, AI hay người |
| `solutions`, `analysis_runs`, `decision_history` | `analysis.generated`, `analysis.decided` | phiên bản, phương án chọn, lý do |
| `clarifications` | `clarification.asked`, `clarification.answered` | số câu hỏi, người trả lời |
| `approvals` | `approval.requested`, `approval.decided` | đối tượng (`subject_type`), người quyết định, kết quả, lý do |
| `phase_starts` | `phase.started` | Phase, `trigger` (`manual`/`auto`) |
| `task_run_outcomes` | `task.started`, `task.succeeded`, `task.failed`, `task.cancelled`, `phase.done`, `plan.done` | task, lần thử, nguyên nhân, `error_message` |
| `request_return_history` | `request.returned`, `request.reopened`, `request.cancelled` | giai đoạn, nhóm lý do (`category`), lý do, người |

Trường chung của mỗi dòng: `id`, `kind`, `occurred_at`, `actor_kind` (`user|ai|system|agent`), `actor_id`, `subject_type`, `subject_id`, `summary` (ngắn, không chứa nội dung request), `attempt` (nếu có), `severity` (`info|attention|error`), `details` (JSON giới hạn kích thước, mặc định 4 KB như `request_audit_outbox.metadata_json`).

- Bộ lọc: `kinds[]`, `severity`, `actor_kind`, `subject_id`, `since`. Nhóm lọc UI "Needs attention" = `severity != info`.
- Không trả `body`, `snapshot` của `request_revisions` hay nội dung log của agent; chỉ tham chiếu (`subject_id`) để frontend gọi RPC chi tiết có kiểm quyền.
- Kiểm quyền: cùng quyền đọc Request (`GetRequest`); không mở rộng quyền (CR-REQ-035). Dòng của đối tượng người xem không đọc được bị loại.
- Hiệu năng: mỗi nguồn truy vấn theo `(tenant_id, request_id, occurred_at)`; cần kiểm chỉ mục hai dialect (xem mục 6). Giới hạn `page_size` ≤ 100.
- Bảng `outbox_events` **không dùng** làm nguồn vì có thể bị dọn sau khi giao (chưa kiểm chứng chính sách dọn).

### 2.3 `request.executionSummary`: tóm tắt thực thi

RPC `GetExecutionSummary` (gateway `request.executionSummary`), để frontend poll rẻ và nhất quán thay vì tự ghép từ nhiều RPC:

```
ExecutionSummary {
  request_id, status, engine, execution_mode,
  progress { phases_total, phases_done, tasks_total, tasks_done, tasks_failed, tasks_running }
  current_phase_id            // rỗng khi luồng không có Phase
  running[]  { task_id, phase_id, attempt, max_attempts, started_at }
  failed[]   { task_id, phase_id, attempts, max_attempts, last_cause, last_error, last_at }
  needs_you[] { kind: approval|clarification|failed_task|pre_deploy|drift,
                ref_id, subject_type, since, due_at }
  handover   { engine=openspec: state: none|archiving|ready|failed, archive_attempts, summary_ref }  // rỗng với native
  next_phase_id
}
```

- `attempt` và `failed[]` suy ra từ `task_run_outcomes` (đếm `started` theo task; `last_cause`, `last_error` từ dòng `failed` mới nhất). `max_attempts` từ `REQUEST_MAX_TASK_ATTEMPTS`.
- `needs_you[]`: Approval `pending` của Request mà người xem được phép quyết (tái dùng bộ lọc của `ListPendingForUser`), Clarification mở, task lỗi làm Request về backlog, cổng `pre_deploy`, `drift` đang mở.
- `handover` đọc từ `openspec_changes` (đã có `sync_progress`, migration `0097`); chỉ tính cho Request có engine `openspec`. Cần đối soát tên trường archive thật trước khi chốt (câu hỏi mở 2, mục 7).
- **Chuẩn hoá nguyên nhân lỗi**: `task_run_outcomes.cause` hiện là chuỗi tự do. Thống nhất tập mã `timed_out`, `model_error`, `verify_failed`, `env_defect`, `spec_defect`, `agent_error`, `cancelled`, `unknown` và ghi ở nơi phát hiện lỗi. Việc đầu tiên: tìm vì sao #1242 ghi `execution failed: retryable TIMED_OUT` trong khi log cuối là `unrecognized_model` (có thể lần 1 và lần 2 khác nguyên nhân, hoặc cause bị ghi đè).

### 2.4 Trường mới trên `Request` và `request.flow`

- `Request` thêm `string engine = 32` (engine hiệu lực: `EffectiveEngine(...)`, `native` nếu profile `None`) và `string execution_mode = 33` (rỗng khi chưa ghim; giá trị hiệu lực khi đã `executing`). Số trường cần kiểm tra lại lúc triển khai (CR-REQ-027 dùng 27 đến 31).
- `GetRequestFlowResponse` thêm `string open_spec_profile` (`full|light|none`) để frontend biết loại nào *có thể* chạy OpenSpec; engine thật lấy từ `Request.engine`.
- Kênh gateway mới (`wscompat`): `request.activity`, `request.executionSummary`; `request.get` trả hai trường mới. `parity_test.go` của MCP đỏ nếu thêm kênh mà không có `ToolSpec` hoặc dòng loại trừ (README v6 mục 8, điểm 13): thêm tool `request_activity`, `request_execution_summary` (chỉ đọc) hoặc dòng loại trừ có lý do.

### 2.5 Migration `0098` (Postgres và MySQL)

- `request.project_engine_settings`: thêm `execution_mode TEXT NOT NULL DEFAULT 'step_by_step'` (CHECK hai giá trị).
- `request.requests`: thêm `execution_mode TEXT NULL` (ghim lúc vào `executing`).
- `request.phase_starts`: thêm `trigger TEXT NOT NULL DEFAULT 'manual'` (CHECK `manual|auto`).
- Chỉ mục phục vụ 2.2 (kiểm chứng bằng `EXPLAIN`): `(tenant_id, request_id, occurred_at DESC)` hoặc `(tenant_id, request_id, at DESC)` trên `task_run_outcomes`, `request_return_history`, `approvals` (cột `updated_at`), `clarifications`.
- Migration thuận chiều; mặc định giữ nguyên hành vi (`step_by_step`).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| B1 | Thêm chế độ, **không đổi** hành vi mặc định | Dự án đang chạy luồng cũ; đổi mặc định cần rollout riêng (CR-REQ-025) |
| B2 | Ghim `execution_mode` theo Request khi vào `executing` | Đổi cài đặt giữa chừng không được đổi luật của Request đang chạy (cùng lý do ghim engine) |
| B3 | Người duyệt Plan là người thực thi, kiểm quyền lúc duyệt | Task-service kiểm quyền `execute` theo người; lỗi quyền giữa chừng làm Request kẹt khó hiểu |
| B4 | Nhật ký là read model UNION, không có bảng sự kiện mới | Dữ liệu đã nằm ở các bảng; tránh ghi đôi và lệch |
| B5 | Tóm tắt thực thi tính ở backend | Frontend không có `task_run_outcomes`; ghép nhiều RPC dễ lệch và tốn lượt gọi |
| B6 | `engine` và `execution_mode` trên `Request`, không suy từ loại ở frontend | `bug`, `security`, `performance` có profile `Light`; engine phụ thuộc cài đặt dự án |
| B7 | Cổng `pre_deploy` và Clarification vẫn dừng ở mọi chế độ | Đây là quyết định của người, không phải thủ tục theo Phase |
| B8 | Khi `continuous`, chấp nhận rủi ro của mọi Phase thu ở Approval của Plan; `drift` vẫn dừng Phase | Giữ một lần duyệt; không cổng lai theo mức rủi ro trong v6 (chốt 2026-10-16) |
| B9 | Không làm Pause/Resume trong v6; chỉ "Dừng" bằng `CancelRequest` | Giảm phạm vi; cờ và RPC để dành CR sau (chốt 2026-10-16) |
| B10 | `execution_mode` cài đặt theo dự án, cùng chỗ với engine, mặc định `step_by_step`; người duyệt không chọn trên thẻ duyệt | Một nơi cài đặt; đổi mặc định cần rollout riêng (chốt 2026-10-16) |
| B11 | Người duyệt Plan là người thực thi, kiểm quyền lúc duyệt; bị gỡ quyền giữa chừng thì task kế tiếp lỗi quyền và Request về backlog (`category=blocked_dependency`); không dùng tài khoản hệ thống | Giữ truy vết theo người; task-service kiểm quyền theo người (chốt 2026-10-16) |
| B12 | `request.activity` chỉ có sự kiện cấp Request và task; log và câu hỏi của agent mở riêng bằng RPC chi tiết | Tránh lộ nội dung và phình dữ liệu (chốt 2026-10-16) |

## 4. Tiêu chí chấp nhận

- [ ] Dự án `continuous`: duyệt Plan `change_request` 3 Phase một lần thì Phase 1, 2, 3 tự chạy tuần tự, không mở Approval `phase`, không cần `StartPhase` thủ công; `phase_starts.trigger = auto`.
- [ ] Dự án `step_by_step` (mặc định) hành vi y hệt hiện tại, test hiện có vẫn xanh.
- [ ] Phase kế tiếp không bắt đầu khi Phase trước còn chạy hoặc `depends_on` chưa xong.
- [ ] Task lỗi vượt `REQUEST_MAX_TASK_ATTEMPTS` đưa Request về backlog và không Phase nào mới tự bắt đầu.
- [ ] Người duyệt không có quyền `execute` bị từ chối với `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`; không có Request nào kẹt vì lý do này.
- [ ] Người duyệt bị gỡ quyền giữa chừng: task kế tiếp lỗi quyền, Request về `request_backlog` với `category=blocked_dependency`, không Phase mới tự bắt đầu.
- [ ] `continuous`: Approval của Plan chứa danh sách phát hiện rủi ro của mọi Phase; không còn `ApprovalRiskGateSection` riêng cho từng Phase.
- [ ] Đổi `execution_mode` của dự án giữa lúc Request đang `executing` không đổi hành vi Request đó.
- [ ] `ops_request` vẫn dừng ở cổng `pre_deploy` trong `executing` khi `continuous`.
- [ ] `ListRequestActivity` trả đủ các `kind` ở bảng 2.2 theo thứ tự thời gian, phân trang ổn định (không lặp, không sót dòng cùng thời điểm), lọc theo `kinds` và `severity`, không trả `body` hay log agent.
- [ ] Người xem không có quyền đọc đối tượng không thấy dòng liên quan trong `activity` và `needs_you[]`.
- [ ] `GetExecutionSummary` của #1242 (dữ liệu thật) trả `tasks_total=28`, `tasks_done=27`, `failed[0].attempts=2`, `last_cause` khác rỗng.
- [ ] `Request.engine` đúng cho `change_request` (openspec theo cài đặt), `task` (native); `bug` với cài đặt dự án `openspec` trả `openspec`.
- [ ] Kênh mới có `ToolSpec` hoặc dòng loại trừ; `parity_test` xanh.
- [ ] Migration chạy xuôi trên cả Postgres và MySQL; Request cũ vẫn đọc được.

## 5. Kiểm thử

- Unit: `EffectiveExecutionGates` cho 11 loại × 2 chế độ; chọn Phase kế tiếp theo `depends_on`; ghim mode; đếm lần thử và `last_cause`; ánh xạ `kind`/`severity` của từng nguồn; con trỏ phân trang với nhiều dòng cùng `occurred_at`.
- Contract test hai dialect (`internal/adapter/contracttest`): UNION activity trên Postgres và MySQL cho cùng dữ liệu; `phase_starts.trigger`; chỉ mục.
- Luồng: `execution_flow_contract` thêm kịch bản `continuous` nhiều Phase, lỗi task giữa chừng, `drift`, `ops_request` `pre_deploy`, người duyệt thiếu quyền.
- Gateway: kênh mới qua `wscompat` test; `parity_test` MCP.
- E2E (CR-REQ-025): một Request `change_request` 3 Phase chạy thật với agent thật trên stack local. Lưu ý khi chạy: token CLI hết hạn sau khoảng 10 phút nên một lần sinh plan dài có thể chết `UNAUTHENTICATED` dù run vẫn tiếp tục; sau khi redeploy một service phải khởi động lại các service gọi nó. Chưa chạy; kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Bỏ cổng Phase làm mất điểm kiểm soát rủi ro theo Phase.** Đã chọn thu rủi ro ở Plan (B8); người duyệt phải xem danh sách phát hiện của mọi Phase trước khi duyệt một lần, nên UI cần làm rõ (CR-REQ-037, mục 2.5). Phát hiện rủi ro mới chỉ xuất hiện sau khi chạy (drift) vẫn dừng Phase.
- **Người duyệt là người thực thi**: quyền của một người dùng tác động lên cả Plan dài giờ sau. Đã chọn xử lý bằng B11 (lỗi quyền, về backlog); chưa kiểm chứng task-service trả lỗi quyền đủ rõ để phân biệt với lỗi task thường.
- **Hiệu năng UNION nhiều bảng** chưa đo; Request dài có hàng trăm dòng `task_run_outcomes`. Cần `EXPLAIN` trên cả hai dialect và giới hạn cửa sổ thời gian.
- **`cause` hiện là chuỗi tự do**: dữ liệu cũ không map được sang bộ mã mới; chấp nhận `unknown` cho dữ liệu cũ.
- **`outbox_events` có thể bị dọn**: nếu về sau muốn đưa sự kiện không có bảng nguồn (ví dụ `agent.question`), cần quyết định lưu bền riêng.
- Số trường proto 32 và 33 và số migration 0098 có thể đã bị CR khác giữ khi triển khai; kiểm tra lại.
- Chưa chạy hệ thống; mọi mô tả hành vi dựa trên đọc code ngày 2026-10-16.

## 7. Quyết định đã chốt và câu hỏi còn mở

**Đã chốt (người yêu cầu, 2026-10-16)** theo đề xuất trong bản nháp: B8 (rủi ro thu ở Plan), B9 (không Pause/Resume), B10 (`execution_mode` theo dự án, người duyệt không chọn), B11 (người duyệt Plan là người thực thi, gỡ quyền thì về backlog), B12 (activity chỉ cấp Request và task). Cách hiểu B9 là suy luận từ câu "đồng ý với đề xuất"; cần xác nhận khi duyệt.

**Còn mở:**

1. Người duyệt thứ hai (rủi ro nghiêm trọng, CR-REQ-036) có tính là "người duyệt Plan" để kiểm quyền thực thi không? Mặc định tạm: chỉ người quyết định Approval cuối cùng.
2. Bước archive và bàn giao OpenSpec diễn ra ở trạng thái nào (`executing` hay `completed`) và đọc tiến độ từ cột nào của `openspec_changes`? Sơ đồ luồng của người yêu cầu ghi ở `completed`; cần đối soát với code trước khi chốt `handover` ở 2.3.
3. Tên mã nguyên nhân lỗi (`timed_out`, `model_error`, ...) có khớp phân loại `Failure.class` của CR-REQ-029 không? Cần người nắm CR-REQ-029 rà.

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| README v6 mục 3.4 | Ghi chú: cổng `phase` của `change_request` phụ thuộc `execution_mode` |
| CR-REQ-003 | `FlowDefinition` thêm `EffectiveExecutionGates(mode)`; `GetRequestFlowResponse` thêm `open_spec_profile` |
| CR-REQ-013 | `StartExecution` và `completePhases` có nhánh `continuous`; `phase_starts.trigger`; `StartPhase` dùng `Internal` cho Phase |
| CR-REQ-009, 010 | Không mở Approval `phase` khi `continuous` |
| CR-REQ-016 | Thêm kênh `request.activity`, `request.executionSummary`; `request.get` trả `engine`, `execution_mode` |
| CR-REQ-017 | `ToolSpec` hoặc dòng loại trừ cho hai kênh mới |
| CR-REQ-021 (frontend) | Cổng và nút Phase chỉ còn khi `step_by_step` (xem CR-REQ-037) |
| CR-REQ-026 | `Request.engine` là nơi lộ engine hiệu lực; `openspec_changes` cấp dữ liệu bàn giao |
| CR-REQ-035 | Nhật ký và tóm tắt tuân quyền đọc Request; audit ghi `phase.started` tự động |

## 9. Tham chiếu

- `backend-go/services/request-service/internal/domain/request_flow_registry.go`, `engine_settings.go`, `openspec_profile.go`, `execution_tree.go`
- `backend-go/services/request-service/internal/usecase/start_execution.go`, `start_phase.go`, `evaluate_execution.go`, `advance_execution.go`
- `backend-go/services/request-service/migrations/{postgres,mysql}/0050_phase_execution`, `0080_security_compliance`, `0095_project_solution_agent`, `0097_openspec_change_sync_progress`
- `backend-go/proto/orca/request/v1/request.proto` (`Request`, `GetRequestFlowResponse`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_flow.go`
- [CR-REQ-013](./CR-REQ-013-phase-execution-and-feedback-loop.md), [CR-REQ-014](./CR-REQ-014-type-specific-execution-policies.md), [CR-REQ-037](../request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md)
- Mới: `internal/domain/execution_mode.go`, `internal/usecase/{list_request_activity,get_execution_summary}.go`, migration `0098_execution_mode_and_activity_indexes`
