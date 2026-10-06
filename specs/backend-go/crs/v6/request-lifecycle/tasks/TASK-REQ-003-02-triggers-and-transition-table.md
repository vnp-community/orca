# TASK-REQ-003-02: Trigger, bảng chuyển `NextStatus` và đường chuẩn `HappyPath`

**From Solution:** BE-REQ-SOL-003
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/request_trigger.go`, `internal/domain/request_transition.go`, `internal/domain/request_flow_path.go` và `*_test.go` (mới)
**Depends on:** TASK-REQ-003-01
**Status:** [ ] TODO

---

## Context

Bảng chuyển: CR-REQ-003 mục 2.2 (16 trigger). `RequestStatus` có 11 giá trị (TASK-REQ-002-02). `ReturnStage` đã có. Hàm phải thuần (không I/O) để test table-driven bao phủ hết ma trận 11 nhân 16.

## Việc cần làm

1. `request_trigger.go`: `type Trigger string` và 16 hằng: `TriggerStartClassification="start_classification"`, `TriggerProposalReady`, `TriggerTypeConfirmed`, `TriggerAnalysisReady`, `TriggerAnalysisApproved`, `TriggerAnalysisRejected`, `TriggerAnalysisRevision`, `TriggerPlanReady`, `TriggerPlanApproved`, `TriggerPlanRejected`, `TriggerPlanRevision`, `TriggerExecutionFinished`, `TriggerReturnToBacklog`, `TriggerTypeChange`, `TriggerReopen`, `TriggerCancel`; `AllTriggers() []Trigger`.
2. `request_transition.go`: `func NextStatus(flow FlowDefinition, size RequestSize, from RequestStatus, trigger Trigger) (RequestStatus, error)`; dùng `switch trigger` rồi kiểm `from` thuộc tập cho phép, cài đúng bảng SOL-003 mục C:
   - `start_classification`: `new` thành `classifying`.
   - `proposal_ready`: `classifying` thành `awaiting_type_confirmation`.
   - `type_confirmed`: `awaiting_type_confirmation` thành `analyzing` nếu `AnalysisKind != none`, ngược lại `planning`.
   - `analysis_ready`: `analyzing` thành `awaiting_analysis_approval` nếu `AnalysisGate != ""`; `awaiting_plan_approval` nếu `PlanKind == single_task`; ngược lại `planning`.
   - `analysis_approved`, `analysis_rejected`, `analysis_revision` từ `awaiting_analysis_approval`.
   - `plan_ready` từ `planning`; `plan_approved`, `plan_rejected`, `plan_revision` từ `awaiting_plan_approval`.
   - `execution_finished`: `executing` thành `completed`.
   - `return_to_backlog`: từ 7 trạng thái (`classifying`, `awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`) thành `request_backlog`.
   - `type_change`: từ 6 trạng thái (`awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`) thành `awaiting_type_confirmation`.
   - `reopen`: `request_backlog` thành `classifying`.
   - `cancel`: mọi trạng thái trừ `completed`, `cancelled` thành `cancelled`.
   Cặp ngoài bảng: `ErrTransitionNotAllowed(from, trigger)` (`REQUEST_TRANSITION_NOT_ALLOWED`, `FailedPrecondition`; thêm vào `request_flow_errors.go`). `size` hiện chỉ dùng cho chuỗi chuẩn, giữ trong chữ ký để CR-REQ-013 mở rộng.
3. Hàm hỗ trợ `RequiresReason(trigger Trigger) bool` (đúng cho `analysis_rejected`, `plan_rejected`, `return_to_backlog`, `cancel`) và `TargetsBacklog(trigger) bool` để use case dùng.
4. `request_flow_path.go`: `HappyPath(flow FlowDefinition, size RequestSize) ([]RequestStatus, error)`: bắt đầu từ `new`, lần lượt áp trigger chuẩn (`start_classification`, `proposal_ready`, `type_confirmed`, rồi `analysis_ready`, `analysis_approved` nếu có `AnalysisKind`, `plan_ready`, `plan_approved`, `execution_finished` khi còn bước) và ghi các trạng thái đã qua tới `completed`. Quy tắc chọn trigger kế tiếp dựa vào trạng thái hiện tại và `flow`: tại `analyzing` dùng `analysis_ready`; tại `awaiting_analysis_approval` dùng `analysis_approved`; tại `planning` dùng `plan_ready`; tại `awaiting_plan_approval` dùng `plan_approved`; tại `executing` dùng `execution_finished`.

## Kiểm thử

- `TestNextStatus_FullMatrix`: ma trận 11 trạng thái nhân 16 trigger nhân vài loại đại diện (`change_request`, `hotfix`, `spike`, `task`); tập cặp hợp lệ viết tay, mọi cặp khác phải lỗi `REQUEST_TRANSITION_NOT_ALLOWED`.
- `TestNextStatus_PerType`: với từng loại, chuỗi trigger chuẩn đi từ `new` tới `completed`; `hotfix` không đi qua `awaiting_analysis_approval` hay `planning`; `spike` và `question` đi `analyzing`, `awaiting_analysis_approval`, `completed`; `security` đi `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`, `completed`.
- `TestTerminalStatesAcceptNothing`.
- `TestHappyPath_AllTypes` (độ dài và thứ tự khớp README mục 3.4).
- `TestRequiresReason`.
Lệnh: `go test ./services/request-service/internal/domain/...`.

## Tiêu chí hoàn thành

- [ ] Mọi cặp ngoài bảng bị từ chối (ma trận đầy đủ).
- [ ] `completed`, `cancelled` không nhận trigger.
- [ ] `HappyPath` đúng cho 11 loại.
- [ ] Không có I/O, không import ngoài stdlib và `common/apperrors`.

## Rủi ro và lưu ý

- `return_to_backlog` từ `classifying` hợp lệ, nhưng `new` không (Request vừa tạo chưa vào `classifying` thì chưa trả về được); khớp CR.
- `analysis_revision` (về `analyzing`) cần CR-REQ-007 gọi; để trigger sẵn, không có nguồn gọi ở feature này.
