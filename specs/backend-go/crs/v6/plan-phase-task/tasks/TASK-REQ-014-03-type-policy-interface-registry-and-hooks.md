# TASK-REQ-014-03: Interface `TypePolicy`, registry, `noopPolicy` và nối hook vào `GeneratePlan`, `AdvanceExecution`, `ReportTaskOutcome`

**From Solution:** BE-REQ-SOL-014
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/type_policy.go` (mới), `type_policy_registry.go` (mới), `type_policy_noop.go` (mới), `internal/usecase/generate_plan.go`, `commit_plan.go`, `advance_execution.go`, `report_task_outcome.go` (sửa), `internal/domain/type_policy_registry_test.go` (mới)
**Depends on:** TASK-REQ-012-05, TASK-REQ-013-04, TASK-REQ-013-06
**Status:** `[ ] TODO`

---

## Context

- SOL-012 task 05 và SOL-013 task 04, 06 để sẵn cổng rỗng/`nil` cho chính sách theo loại; task này định nghĩa interface thật và thay cổng rỗng bằng `PolicyFor(req.Type)`.
- Quy ước repo: tên file theo khái niệm (`type_policy_hotfix.go`), không dùng `helpers`/`utils`/`common`/`misc`; không thêm `max-lines` disable.
- Domain thuần Go. `TaskRef` là kiểu domain gồm `ID`, `Title`, `Labels`, `ParentID`, `Status` (ánh xạ từ `TaskView` ở grpcclient).
- Loại không có chính sách chạy y như trước (tiêu chí chấp nhận: `bug`, `task`, `docs`, `change_request`).

## Việc cần làm

1. `type_policy.go`:
   ```go
   type GateRequirement struct{ SubjectType, SubjectID, Stage string }
   type CheckVerdict struct{ Kind CheckKind; Stage string; Status VerdictStatus; Summary string } // passed|failed|missing
   type FollowUp struct{ TypeHint RequestType; Title, Body, LinkReason, ClientRequestID string }
   type TypePolicy interface { /* bốn hàm ở SOL-014 2.2 */ }
   ```
2. `type_policy_noop.go`: `noopPolicy` trả `nil`/rỗng cho cả bốn hàm.
3. `type_policy_registry.go`: `PolicyFor(t RequestType) TypePolicy` dùng map dựng bởi hàm `NewPolicyRegistry()` (không dùng `init`) và truyền vào use case để test thay thế được; loại chưa có thì `noopPolicy`.
4. Nối hook (chỉ thay cổng, không thêm logic loại):
   - `GeneratePlan`/`CommitPlan`: sau `ValidateProposal` gọi `policy.PlanPreconditions(req, proposal)`; lỗi trả nguyên mã `REQUEST_*` của chính sách.
   - `AdvanceExecution`: trước `Execute` từng task gọi `policy.PreExecutionGate(req, taskRef)`; có `GateRequirement` thì `OpenApproval` (idempotent) và bỏ qua task.
   - `ReportTaskOutcome` bước hoàn tất: `checks := RequestCheckRepository.ListByRequest`; `verdicts := policy.CompletionChecks(req, checks)`; có `failed` thì `ReturnToBacklog(stage=task, category=infeasible|other, reason=tóm tắt)`; có `missing` thì chờ; sau `execution_finished` và `request.completed` gọi `policy.OnCompleted(req)` và thực hiện từng `FollowUp` (task 07 làm hiện thực hotfix).
5. Chọn `category` khi `failed`: `infeasible` nếu `Kind` thuộc `perf_*`, ngược lại `other` (khớp SOL-013 2.6).
6. Chạy `gitnexus_impact` cho bốn use case trước khi sửa (nếu chỉ mục có mặt; `request-service` mới nên có thể chưa được lập chỉ mục: ghi chú).

## Kiểm thử

- `TestPolicyFor_UnknownType_ReturnsNoop`, `_AllKnownTypesRegistered` (11 loại: không panic, chính sách đúng loại).
- `TestGeneratePlan_PolicyPreconditionError_Propagates`, `TestAdvanceExecution_PolicyGate_OpensApprovalOnce`, `TestReportTaskOutcome_CompletionFailed_ReturnsBacklogWithCategory`, `_CompletionMissing_Waits`, `_OnCompletedFollowUps_SpawnedOnce`.
- Hồi quy: các test của TASK-REQ-012-05 và 013-04/06 chạy với `noopPolicy` vẫn xanh.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'Policy|GeneratePlan|AdvanceExecution|TaskOutcome' -v`.

## Tiêu chí hoàn thành

- [ ] Loại khác (`bug`, `task`, `docs`, `change_request`) chạy y như trước (test hồi quy với `noopPolicy`).
- [ ] Mỗi điểm hook gọi đúng hàm chính sách của loại.
- [ ] Registry chứa đủ loại chính sách (sau tasks 04 đến 07) và test chống thiếu.
- [ ] Không có `switch req.Type` trong use case chung.

## Rủi ro và lưu ý

- `request-service` mới nên chỉ mục GitNexus có thể không có; xác nhận bằng grep.
- Nếu CR-REQ-029 (ReadinessGate) thêm cổng cùng điểm `AdvanceExecution`, thứ tự gọi (policy trước hay readiness trước) cần chốt; không quyết ở task này.
- Interface lớn dễ thành "dumping ground"; mỗi loại một file, không gom chung.
