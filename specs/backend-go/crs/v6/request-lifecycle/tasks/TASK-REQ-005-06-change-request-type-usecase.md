# TASK-REQ-005-06: Use case `ChangeRequestType` và `ListRequestTypeHistory`

**From Solution:** BE-REQ-SOL-005
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/change_request_type.go`, `internal/usecase/list_request_type_history.go` và `*_test.go` (mới)
**Depends on:** TASK-REQ-005-05, TASK-REQ-005-01
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -race ./internal/domain/... ./internal/usecase/... ./internal/adapter/... và go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql -run "Intake|Classification|Migration|Schema"`)

---

## Context

CR-REQ-005 mục 2.4; bảng đường đổi ở `domain.ChangeTypeAllowed`. Tập `status` cho phép: `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`. `TransitionRequest(type_change)` đưa về `awaiting_type_confirmation` và, qua bảng chuyển, cũng hợp lệ từ chính `awaiting_type_confirmation` nhưng ở đó người dùng dùng `ConfirmRequestType` nên use case này từ chối. `ExecutionGuard` no-op trả `false` tới CR-REQ-011. **Giữ phần đã làm:** không động tới `solutions`, Plan, Task.

## Việc cần làm

1. `ChangeInput{RequestID, NewType, Size, Urgency, Reason string; ExpectedVersion int64; ActorID string}`; `ChangeRequestType{repo, history, transition, approvals canceller, guard, tx, outbox}`.
2. `Execute`: `Reason` rỗng thì `REQUEST_REASON_REQUIRED`; `InTx`: `repo.Get`; `ExpectedVersion` lệch thì `REQUEST_VERSION_CONFLICT`; `status == awaiting_type_confirmation` thì `REQUEST_TRANSITION_NOT_ALLOWED`; `status` ngoài tập thì `REQUEST_TRANSITION_NOT_ALLOWED`; `ChangeTypeAllowed(r.Type, newType)`; khi `executing`, `guard.HasActiveExecution` đúng thì `REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION`.
3. Cập nhật `Type=newType`, `TypeSource=human`, `Size`/`Urgency` nếu gửi (nếu loại mới yêu cầu size theo `PhaseRule` mà không có size cũ cũng không gửi: chưa kiểm ở đây, `ConfirmRequestType` sau đó sẽ kiểm); `repo.Update(CAS)`; `history.Append(from=cũ, to=mới, user, reason)`; `canceller.CancelPending(requestID, "type_changed")`; `transition.Execute(type_change, ExpectedFrom=status hiện tại, ActorKind=user)`; outbox `orca.request.request.type_changed` `{request_id, from, to, actor_id, reason}`.
4. `list_request_type_history.go`: `Execute(ctx, requestID) ([]RequestTypeChange, error)`: `tenant.RequireTenantID`; xác nhận Request tồn tại (`REQUEST_NOT_FOUND`); `history.List` theo `at, id`.
5. Sau đổi loại, Request ở `awaiting_type_confirmation`: người dùng xác nhận lại bằng `ConfirmRequestType`; không tự gọi AI lại (CR không yêu cầu; ghi trong Câu hỏi nếu muốn).

## Kiểm thử

- `TestChange_BugToChangeRequest_FromAnalyzing_KeepsSolutionsAndTasks` (repo fake có Solution; không bị xoá hoặc đổi).
- `TestChange_SpikeToChangeRequest_UseChild`, `_HotfixAnyTarget_UseChild`, `_BugToDocs_NotAllowed`, `_SameType_Unchanged`, `_SecurityToHotfix_Allowed`.
- `TestChange_FromAwaitingTypeConfirmation_Rejected`, `_FromNew_Rejected`, `_FromCompleted_Rejected`.
- `TestChange_ReasonRequired`, `_StaleVersion`.
- `TestChange_ExecutingWithActiveExecution_Blocked` (guard giả `true`).
- `TestChange_CancelsPendingApproval` (canceller giả được gọi với `type_changed`).
- `TestListTypeHistory_OrderAfterAIUserChange` (AI đề xuất, người sửa, đổi loại: đúng thứ tự `at`).
- Lệnh: `go test ./services/request-service/internal/usecase/... -run "ChangeRequestType|TypeHistory"`.

## Tiêu chí hoàn thành

- [x] `bug` sang `change_request` từ `analyzing`: status `awaiting_type_confirmation`, Solution và Task cũ còn nguyên, có dòng lịch sử và `type_changed`.
- [x] `spike` sang `change_request` trả `REQUEST_TYPE_CHANGE_USE_CHILD`; `bug` sang `docs` trả `REQUEST_TYPE_CHANGE_NOT_ALLOWED`.
- [x] Approval `pending` bị huỷ qua cổng.
- [x] `ListRequestTypeHistory` đúng thứ tự.

## Rủi ro và lưu ý

- `ExecutionGuard` giả: đổi loại từ `executing` không chặn Task đang chạy cho tới CR-REQ-011; ghi vào README service.
- Đổi sang loại cần `size` (bug, refactor) rồi xác nhận mà quên `size`: bắt ở `Confirm` (`REQUEST_SIZE_REQUIRED`).

## Ghi chú triển khai

- `ChangeKeepsSolutions` kiểm bằng DB thật (hàng `solutions` còn nguyên, version 1).
- `ExecutionGuard` vẫn là no-op tới CR-REQ-011.
