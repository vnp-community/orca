# TASK-REQ-003-01: Registry luồng `FlowDefinition` và `FlowFor`

**From Solution:** BE-REQ-SOL-003
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/request_flow_registry.go`, `internal/domain/request_flow_registry_test.go`, `internal/domain/request_flow_errors.go` (mới)
**Depends on:** TASK-REQ-002-02 (kiểu `RequestType`, `RequestSize`)
**Status:** [ ] TODO

---

## Context

CR-REQ-003 mục 2.1 có bảng 11 loại. README v6 mục 3.4 là nguồn gốc (bảng "Luồng theo loại"). README không có subject `diagnosis`, nên giai đoạn phân tích của `bug`, `security`, `performance` dùng gate subject `solution`. `request_type` luôn là cổng đầu và không nằm trong registry. `RequestType`, `RequestSize` đã có từ TASK-REQ-002-02. Chưa có mã registry nào trong repo.

## Việc cần làm

1. Khai báo kiểu `AnalysisKind` (`none|solution|diagnosis|findings|answer`), `PlanKind` (`none|plan|task_list|single_task`), `PhaseRule` (`never|always|when_size_L`), `GateSubject` (`""`, `solution`, `findings`, `answer`, `plan`, `task_list`, `pre_deploy`, `phase`) kèm hằng.
2. `type FlowDefinition struct { Type RequestType; HumanConfirmRequired bool; AnalysisKind AnalysisKind; AnalysisGate GateSubject; PlanKind PlanKind; PhaseRule PhaseRule; StartGate GateSubject; ExecutionGates []GateSubject; CompletesAfterAnalysis bool }`.
3. Bảng `flowDefinitions` (biến gói, khởi tạo trong `init` hoặc literal) theo CR mục 2.1: `change_request` (solution, gate solution, plan, always, start gate plan, execution gates [phase]); `bug` (diagnosis, gate solution, plan, when_size_L, start plan); `hotfix` (diagnosis, gate rỗng, single_task, never, start pre_deploy, HumanConfirm); `task` (none, task_list, never, start task_list); `spike` (findings, gate findings, none, never, CompletesAfterAnalysis); `question` (answer, gate answer, none, never, CompletesAfterAnalysis); `refactor` (solution, gate solution, plan, when_size_L, start plan); `security` (diagnosis, gate solution, plan, never, start pre_deploy, HumanConfirm); `performance` (diagnosis, gate solution, plan, never, start plan); `docs` (none, task_list, never, start task_list); `ops_request` (none, plan, never, start plan, execution gates [pre_deploy]).
4. `FlowFor(t RequestType) (FlowDefinition, error)`: trả bản sao (sao chép slice `ExecutionGates`); loại ngoài bảng trả `ErrFlowUnknownType(t)` (`REQUEST_FLOW_UNKNOWN_TYPE`, `KindInvalidArgument`) trong `request_flow_errors.go`.
5. `(f FlowDefinition) PhasesFor(size RequestSize) bool`: `always` đúng, `when_size_L` đúng khi `size == L`, `never` sai; size rỗng coi là không phải L.
6. Hàm phụ cho test hợp đồng: `AllFlowTypes() []RequestType` (đúng 11, thứ tự README).
7. Chưa thêm trigger, bảng chuyển (task 02).

## Kiểm thử

Tên test: `TestFlowFor_AllElevenMatchTable` (so từng trường với bảng mong đợi viết tay trong test), `TestFlowFor_UnknownType`, `TestFlowFor_ReturnsCopy` (sửa `ExecutionGates` của kết quả không đổi lần gọi sau), `TestPhasesFor` (3 luật nhân 3 size nhân size rỗng), `TestFlowTypesCoverAllRequestTypes` (`AllFlowTypes()` bằng `AllRequestTypes()`).
Lệnh: `go test ./services/request-service/internal/domain/... -run Flow`.

## Tiêu chí hoàn thành

- [ ] 11 định nghĩa khớp bảng CR mục 2.1, kiểm bằng test.
- [ ] Loại lạ trả `REQUEST_FLOW_UNKNOWN_TYPE`.
- [ ] `PhasesFor` đúng ba luật; size rỗng là không phải L.
- [ ] Package `domain` vẫn không import adapter hay proto.

## Rủi ro và lưu ý

- Bảng chép tay từ CR; một lệch gây sai luồng thật. Test `TestFlowFor_AllElevenMatchTable` phải viết độc lập với bảng trong mã (không dùng lại biến của mã).
- Q1 (cổng `pre_deploy` chiếm `awaiting_plan_approval`) chưa được xác nhận; đừng nhúng giả định này ở nơi khác ngoài `StartGate`.
