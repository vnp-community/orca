# BE-REQ-SOL-043: `approval.approve` nhận `executionMode`: ghim chế độ chạy liên tục trong cùng transaction với quyết định duyệt Plan

> **📝 Chưa triển khai (0/6 task).** Soạn 2026-10-20 từ đọc mã. Mọi mô tả hiện trạng chưa được kiểm bằng chạy thật.

**CR:** [CR-REQ-043](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-043-approve-plan-and-run-all-steps.md)
**Service:** `request-service` · `api-gateway` (`wscompat`, `httpgateway`) · `orca-cli` · `proto/orca/request/v1`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (phân quyền, phân tách nhiệm vụ)
**Phụ thuộc solution:** [BE-REQ-SOL-038](./BE-REQ-SOL-038-continuous-execution-activity-feed-and-execution-summary.md) (chế độ `continuous`, ghim, `ExecutionStartPrecheck`), [BE-REQ-SOL-039](./BE-REQ-SOL-039-request-worktree-base-branch.md) (mẫu cờ tính năng `request.flowStatus.features`). Frontend: [FE-REQ-SOL-043](../../../../../frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-20 (đường dẫn dưới `backend-go/services/request-service/internal/` trừ khi ghi khác):

- `usecase/decide_approval.go` (`DecideApproval.Execute`, `:45-170`): đọc Approval để lấy request id (`Repo.Get`), `Authorizer.Prefetch`, rồi `StartPrecheck.Check(ctx, probe, userID)` cho `approve` ở trạng thái `pending` (`:75-79`); trong transaction: `LockRequest` (`:84`), `GetForUpdate` (`:88`), gọi lại cùng quyết định thì trả kết quả cũ (`:93-96`), hết hạn, `CanDecide` (`:111`), `ExpectedVersion` (`:114`), `ExpectedDigest` (`:117`), `req.Status == a.Stage` (`:120`), `a.Approve`/`Reject`, `Repo.UpdateDecision` (`:133`), subject handler (`:143`), `LockRequest` lại, `NewOutboxEvent(SubjectApprovalDecided, NewApprovalDecidedPayload(a, after, decision))` (`:158`). Struct có `Repo, Tx, Locker, Registry, Authorizer, Outbox, Expirer, StartPrecheck`; **không** có `Pinner`.
- `usecase/execution_mode_pin.go`: `ExecutionModePinner.PinExecutionModeIfUnset(ctx, requestID, mode) (ExecutionMode, error)`; `PinExecutionMode(ctx, pinner, resolver, req)` chỉ gọi từ `usecase/start_execution.go:38-44`, tức **sau** commit, từ consumer `eventbus/request_status_consumer.go:46-58`.
- Cài đặt ghim: `adapter/postgres/request_repository.go:171-193` và `adapter/mysql/request_repository.go:164`: `UPDATE ... SET execution_mode = $3 WHERE id AND tenant_id AND execution_mode IS NULL` rồi `SELECT`. Đi qua `scoped()` (`adapter/postgres/tx.go:82`, `adapter/mysql/tx.go:75`) dùng transaction trong `ctx` nếu có, nên gọi trong `DecideApproval` thì cuộn lại cùng transaction. Hiện chưa có test nào gọi nó bên trong một transaction của quyết định.
- `usecase/approval_execution_precheck.go` (`ExecutionStartPrecheck.Check`, `:21-53`): lọc `a.Stage == awaiting_plan_approval` và subject ∈ {`plan`, `task_list`, `pre_deploy`}; phân giải chế độ bằng `p.Modes.Mode(ctx, req)` (`:34`); chỉ khi `continuous` mới tới `Access.CheckReady`; lỗi `REQUEST_EXECUTE_FORBIDDEN` đổi thành `ErrExecutionActorNotAllowed` (`domain/execution_errors.go:33`). Nối ở `cmd/server/wire_execution.go:146-151` (sau `wire_approval.go`, vì `Access` cần task client).
- `domain/approval_events.go:27-62`: `ApprovalDecidedPayload` (không có chế độ), `NewApprovalDecidedPayload(a, r, decision)`.
- `usecase/audit_event_tap.go:25-42` (`auditFields`), `:111-129` (`approvalAudit`): metadata chỉ có `subject_type`, `stage`.
- `adapter/grpc/approval_server.go:49-75`: `decide(ctx, decision, id, comment, digest, version)`; `Approve` trả `ApproveResponse{Approval, RequestStatus}`.
- `proto/orca/request/v1/approval.proto`: `ApproveRequest` trường 1-4 (`:65-70`), `ApproveResponse` trường 1-2 (`:106-109`).
- Gateway: `wscompat/channels_approval.go:11-16` (`approvalDecisionArgs`, `approvalDecisionResult`), `:85-97` (`approval.approve`), `:139-147` (`decodeDecision` bắt buộc `expectedVersion >= 1` và `expectedDigest`); REST `httpgateway/request_routes.go:39,130-146`; `requestFeatures()` ở `wscompat/channels_request_base_ref.go:19` trả `[]string{requestFeatureBaseRef}`; MCP: `approval.approve` nằm trong `mcpserver/tools/excluded_channels.yaml:601`.
- CLI: `cmd/orca-cli/internal/command/approval_cmd.go:75-110` (`newApprovalDecideCmd`), `internal/apiclient/request.go:104-106` (`DecideApproval(ctx, id, decision, body)`).
- Khung `request.event` chỉ chuyển danh sách khoá cho phép (`wscompat/channels_request_stream.go:25-52`); `admin.queryAuditLog` trả `Target` không có metadata (`wscompat/channels_admin_audit.go:20-37`).
- Mã lỗi hiện có liên quan: `REQUEST_EXECUTION_MODE_INVALID` (`domain/execution_mode.go`, `ErrExecutionModeInvalid`), `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`, `REQUEST_APPROVAL_ALREADY_DECIDED` (`domain/approval_errors.go:8`), `REQUEST_APPROVAL_VERSION_CONFLICT`.

### Correction relative to the request brief

1. Số dòng: `completePhases` ở `evaluate_execution.go:249-290` (brief ghi 240-286); `ErrPhaseNotApproved` ở `start_phase.go:93` (brief ghi 92); các luồng ở `request_flow_registry.go:64-74` đúng.
2. Brief nêu "thêm vào payload `approval.decided`" cho audit: payload đi tới audit qua `AuditTap` (đọc `auditFields`), nên phải thêm khoá vào `auditFields` và `approvalAudit`, không chỉ vào payload.
3. Brief ghi acceptance "check `approval.decided` carries the mode". Qua WS **không kiểm được** (khung sự kiện lọc khoá, view audit không có metadata); chỉ kiểm được ở contract test đọc hàng outbox. Acceptance dùng phản hồi `executionMode` và `request.get`.
4. Gateway cũ bỏ qua tham số lạ (đã xảy ra với `baseRef`), nên cần cờ `approval.runAll` và trường `executionMode` ở kết quả để giao diện đối chiếu.
5. Claude giả (`tests/stages/e2e/orca-flows/fake_claude_cli/claude`) chỉ nhận bốn loại prompt (`:124-127`); prompt thực thi task là `unknown` và thất bại có chủ đích. Muốn chạy hết các Phase dưới stub phải mở rộng stub (task 06).

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `backend-go/`)

```
proto/orca/request/v1/approval.proto                                              (sửa) ApproveRequest.execution_mode=5, ApproveResponse.execution_mode=3 + mã sinh
services/request-service/
  internal/domain/approval_execution_mode.go                                      (mới) ParseApprovalExecutionMode, AppliesToExecutionModeChoice, 2 lỗi mới
  internal/domain/approval_execution_mode_test.go                                 (mới)
  internal/domain/approval_events.go                                              (sửa) ApprovalDecidedPayload.ExecutionMode
  internal/usecase/decide_approval.go                                             (sửa) Input.ExecutionMode, Pinner, kiểm tra, ghim, payload, kết quả
  internal/usecase/decide_approval_execution_mode.go                              (mới) pinRequestedMode: tách logic ghim để decide_approval.go không phình
  internal/usecase/approval_execution_precheck.go                                 (sửa) Check(..., requested)
  internal/usecase/audit_event_tap.go                                             (sửa) execution_mode vào metadata
  internal/adapter/grpc/approval_server.go                                        (sửa) truyền và trả execution_mode
  cmd/server/wire_approval.go                                                     (sửa) Decide.Pinner = stores.executionModes
  internal/adapter/contracttest/approval_flow_contract.go, execution_flow_contract.go  (sửa)
services/api-gateway/internal/adapter/
  wscompat/channels_approval.go                                                   (sửa) executionMode vào args và kết quả
  wscompat/channels_request_base_ref.go hoặc file giữ requestFeatures()           (sửa) + "approval.runAll"
  httpgateway/request_routes.go                                                   (sửa) body executionMode
cmd/orca-cli/internal/{command/approval_cmd.go,apiclient/request.go}              (sửa) --execution-mode
tests/stages/e2e/orca-flows/{suite_approve_run_all.py,run_acceptance.py,fake_claude_cli/claude,fake_claude_cli/test_fake_claude_cli.py}  (mới/sửa)
```

Không có migration: cột `requests.execution_mode` và `PinExecutionModeIfUnset` đã có từ `0098_execution_mode`. Kiểm lại số migration mới nhất khi triển khai (hiện `0100_analysis_runs_revision_index`).

### 2.2 Miền

`domain/approval_execution_mode.go`:

- `ParseApprovalExecutionMode(raw string) (ExecutionMode, error)`: cắt khoảng trắng và hạ chữ thường; rỗng → `""` (không chọn); `continuous` → `ExecutionContinuous`; mọi giá trị khác, kể cả `step_by_step`, → `ErrExecutionModeInvalid` (mã `REQUEST_EXECUTION_MODE_INVALID` đã có). Hàm riêng, không dùng `ParseExecutionMode` (hàm đó nhận cả `step_by_step` cho cài đặt dự án).
- `ExecutionModeChoiceApplies(a Approval) bool`: `a.Stage == string(RequestStatusAwaitingPlanApproval)` và `a.SubjectType` ∈ {`SubjectPlan`, `SubjectTaskList`, `SubjectPreDeploy`}. Dùng chung với `ExecutionStartPrecheck` (thay hai `switch` trùng lặp) để hai nơi không lệch.
- `ErrExecutionModeNotApplicable()` (`FailedPrecondition`, `REQUEST_EXECUTION_MODE_NOT_APPLICABLE`), `ErrExecutionModeAlreadyPinned(pinned ExecutionMode)` (`FailedPrecondition`, `REQUEST_EXECUTION_MODE_ALREADY_PINNED`, thông điệp nói chế độ đang ghim; không lộ dữ liệu khác).

`ApprovalDecidedPayload.ExecutionMode string `json:"execution_mode,omitempty"``. `NewApprovalDecidedPayload` giữ chữ ký; `DecideApproval` đặt trường sau khi tạo (struct trả về theo giá trị).

### 2.3 `DecideApproval`

`DecideApprovalInput.ExecutionMode string` (thô từ proto). `DecideApprovalResult.ExecutionMode domain.ExecutionMode`. Thứ tự (các bước có sẵn giữ nguyên, bước mới đánh dấu **+**):

1. Chuẩn hoá `Decision`; **+** `mode, err := ParseApprovalExecutionMode(in.ExecutionMode)`; `mode != "" && Decision == reject` → `ErrExecutionModeInvalid`.
2. Đọc `probe`; **+** nếu `mode != ""` và `!ExecutionModeChoiceApplies(probe)` → `ErrExecutionModeNotApplicable` (sớm, trước khi gọi task-service).
3. `StartPrecheck.Check(ctx, probe, userID, mode)` (**+** tham số, task 03).
4. Trong transaction: khoá, ca "đã quyết định" trả `ExecutionMode: req.ExecutionMode` (chế độ đang ghim, không đọc lại tham số), hết hạn, `CanDecide`, version, digest, stage mismatch (đã có).
5. **+** `pinRequestedMode`: nếu `mode != ""`: `got, err := Pinner.PinExecutionModeIfUnset(ctx, req.ID, mode)`; `got != mode` → `ErrExecutionModeAlreadyPinned(got)` (transaction cuộn lại, không ghi quyết định); `Pinner == nil` mà `mode != ""` → lỗi cấu hình `Internal` (không bỏ qua im lặng; `requireWired` ở `wire_approval.go` cũng phải bắt thiếu `Pinner`). Đặt `res.ExecutionMode = got`.
6. `a.Approve`, `UpdateDecision`, handler, `LockRequest` lại (đã có).
7. **+** Payload: `p := NewApprovalDecidedPayload(...)`; `p.ExecutionMode = string(mode)` (chỉ khi `Decision == approve`); phát outbox.
8. `res.ExecutionMode` = chế độ ghim sau cùng (`after.ExecutionMode`, vì `after` đọc lại sau handler) để phản hồi luôn đúng sự thật.

`Pinner` thêm vào struct `DecideApproval` và được nối trong `wire_approval.go` bằng `stores.executionModes` (đã có ở `store_wiring.go:35,87,136`).

### 2.4 `ExecutionStartPrecheck`

`Check(ctx, a, approverID string, requested domain.ExecutionMode) error`. Mode hiệu lực = `requested` nếu khác rỗng, ngược lại `p.Modes.Mode(ctx, req)`. Phần còn lại giữ nguyên (người không được duyệt nhận lỗi duyệt thường, không phải gợi ý quyền task; lỗi tạm của task-service không bị đổi thành từ chối). Dùng `ExecutionModeChoiceApplies` thay `switch` nội bộ. Cập nhật mọi chỗ gọi `Check` (một chỗ ở `decide_approval.go`) và `approval_execution_precheck_test.go`.

### 2.5 Audit

`auditFields.ExecutionMode string `json:"execution_mode"``; trong `approvalAudit`, nếu khác rỗng thì `md["execution_mode"]`. Không sinh bản ghi mới: vẫn một bản ghi `approval.approve` cho mỗi quyết định.

### 2.6 gRPC server và gateway

- `approval_server.go`: `decide(...)` nhận thêm `executionMode`; `Approve` truyền `req.GetExecutionMode()` và trả `ApproveResponse.ExecutionMode`; `Reject` truyền `""` (proto `RejectRequest` không có trường; nếu sau này có thì usecase đã từ chối).
- `channels_approval.go`: `approvalDecisionArgs.ExecutionMode string `json:"executionMode"``; `approval.approve` chuyển xuống; **không** kiểm giá trị ở gateway; kết quả `approvalDecisionResult.ExecutionMode string `json:"executionMode,omitempty"``; `approval.reject` trả `REQUEST_EXECUTION_MODE_INVALID: executionMode applies to approve only` (mẫu `requestInputError{msg: "CODE: ..."}` như `request.engineSettingsSet`) khi khoá có mặt và khác rỗng, không gọi service. `decodeDecision` giữ yêu cầu `expectedVersion`/`expectedDigest`.
- `request_routes.go` `decide`: body thêm `ExecutionMode string `json:"executionMode"`` và đưa vào args của cả hai kênh khi khác rỗng; kênh `reject` từ chối.
- `requestFeatures()`: thêm hằng `requestFeatureApproveRunAll = "approval.runAll"` cạnh `requestFeatureBaseRef`; "gateway thêm tên, không bao giờ bỏ".
- CLI: cờ `--execution-mode` (chỉ nhận `continuous`; để rỗng là mặc định) trên `orca approval approve`, bị từ chối ở `reject`; truyền vào `Document{"executionMode": ...}` của `DecideApproval`.
- MCP: không đổi; test `parity_test` và `excluded_channels.yaml` phải vẫn xanh (dòng loại trừ đã có, lý do "agent không được duyệt").

### 2.7 Mã lỗi

| Mã | Loại | Khi nào |
|---|---|---|
| `REQUEST_EXECUTION_MODE_INVALID` (có sẵn) | InvalidArgument | giá trị khác rỗng/`continuous`; kèm `reject` |
| `REQUEST_EXECUTION_MODE_NOT_APPLICABLE` (mới) | FailedPrecondition | Approval không phải `plan`/`task_list`/`pre_deploy` ở `awaiting_plan_approval` |
| `REQUEST_EXECUTION_MODE_ALREADY_PINNED` (mới) | FailedPrecondition | Request đã ghim chế độ khác `continuous` |
| `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` (có sẵn) | FailedPrecondition | người duyệt không có quyền execute |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| S1 | Ghim ngay trong transaction của `DecideApproval`, trước handler | Quyết định và chế độ cùng thành công hoặc cùng cuộn lại; consumer `StartExecution` thấy chế độ đã ghim |
| S2 | Ghim khác chế độ yêu cầu thì từ chối cả quyết định | Không bỏ lựa chọn đã xác nhận; không sửa pin sẵn có (B2 của SOL-038) |
| S3 | Hàm phân tích riêng chỉ nhận `continuous`/rỗng | Đúng quyết định của người yêu cầu; `ParseExecutionMode` phục vụ cài đặt dự án |
| S4 | `ExecutionModeChoiceApplies` dùng chung với precheck | Một nơi định nghĩa "Approval khởi đầu run" |
| S5 | Payload mang lựa chọn, không mang mặc định phân giải | Mục đích là truy vết con người |
| S6 | Gateway không kiểm giá trị | Danh sách giá trị chỉ ở miền |
| S7 | Phản hồi trả chế độ đang ghim, kể cả ca gọi lại | Giao diện đối chiếu và hiển thị đúng sự thật; idempotent |
| S8 | Không thêm RPC "đổi chế độ cho Request đang chạy" | Ngoài phạm vi (CR mục 2.10) |
| S9 | Không migration | Cột và ghim đã có |

## 4. Phụ thuộc và thứ tự

01 (proto và miền) → 02 (`DecideApproval`, audit) và 03 (precheck) song song sau 01 → 04 (gateway, REST, CLI, cờ; cần 01 và 02) → 05 (contract hai dialect và luồng nhiều Phase; cần 02, 03) → 06 (e2e).

## 5. Kiểm thử

- Unit miền: bảng `ParseApprovalExecutionMode` (rỗng, `continuous`, `Continuous`, `" continuous "`, `step_by_step`, `""` toàn khoảng trắng, rác dài), `ExecutionModeChoiceApplies` cho từng subject và stage.
- Usecase có fake: hợp lệ thì ghim, payload và kết quả; kèm `reject`; subject/stage sai; đã ghim `step_by_step`; đã ghim `continuous` (idempotent); handler lỗi cuộn lại (fake pinner ghi nhận không còn ghim); nhấp đúp; hai quyết định đồng thời (một thắng); `Pinner == nil`; precheck với chế độ yêu cầu ở dự án `step_by_step`.
- Contract hai dialect: `DecideApproval` thật với repository thật; ghim cùng commit với quyết định; lỗi handler giữ cột `NULL`; hàng outbox `approval.decided` có `execution_mode`; ca không chọn thì khoá vắng; luồng ba Phase chạy hết sau một lần duyệt với fake task service (`execution_flow_contract.go`).
- Gateway: `wscompat` cho `approval.approve` (chuyển/ không chuyển tham số, kết quả), `approval.reject` kèm tham số bị từ chối, `request.flowStatus.features`; `httpgateway`; `parity_test` MCP; CLI.
- E2E: `suite_approve_run_all.py` (`ARA-xx`) với claude giả; hạn chế đã nêu ở mục 1 (payload không đọc được qua WS; ca thiếu quyền cần tài khoản thứ hai).

## 6. Rủi ro và điểm chưa kiểm chứng

- **Blast radius**: chữ ký `ExecutionStartPrecheck.Check` và `DecideApproval` đổi; chạy `gitnexus_impact` cho `DecideApproval.Execute`, `ExecutionStartPrecheck.Check`, `NewApprovalDecidedPayload`, `approvalAudit` và báo trước khi sửa.
- **Ghim trong transaction chưa từng được thử** (mới đọc mã `scoped()`); contract test phải chứng minh cuộn lại trên cả Postgres và MySQL.
- **Chặn tại tầng gateway cũ**: cờ `approval.runAll` chỉ có ở gateway mới; frontend mới trên gateway cũ phải ẩn nút.
- **Số trường proto** 5 và 3 có thể bị CR khác giữ; kiểm bằng `buf` khi triển khai. CR-REQ-044 cùng thư mục chạm luồng Plan nhưng chưa đọc để loại trừ chồng lấn trên `approval.approve`.
- **Notification-service giải mã `approval.decided`** chưa được đọc để chắc khoá lạ vô hại.
- **Claude giả** chưa nhận prompt thực thi; task-service chưa kiểm chứng việc chấp nhận `done` không có thay đổi tệp.

## 7. Câu hỏi mở

Như CR-REQ-043 mục 7 (năm câu). Mặc định tạm: #1771 và Request đã ghim `step_by_step` không được hỗ trợ; Inbox không có lựa chọn; không ghi vào History; chỉ nhận `continuous`; công tắc dự án chỉ admin.

## 8. Tham chiếu

- Các file ở mục 1; [CR-REQ-043](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-043-approve-plan-and-run-all-steps.md); [CR-REQ-038](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md); [`CONTRACT-request-ui-api.md`](../../gateway-and-mcp/CONTRACT-request-ui-api.md) dòng 198, 408 (người điều phối cập nhật)
