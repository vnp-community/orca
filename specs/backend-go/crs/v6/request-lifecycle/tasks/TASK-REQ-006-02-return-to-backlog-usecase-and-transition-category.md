# TASK-REQ-006-02: `TransitionInput.Category` và use case `ReturnRequestToBacklog`

**From Solution:** BE-REQ-SOL-006
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/transition_request.go`, `internal/usecase/transition_request_events.go`, `internal/usecase/transition_request_test.go`, `internal/usecase/status_write_guard_test.go` (sửa); `internal/usecase/return_request_to_backlog.go`, `internal/usecase/return_request_to_backlog_test.go` (mới)
**Depends on:** TASK-REQ-006-01, TASK-REQ-005-04 (`ApprovalCanceller`, `ExecutionGuard`)
**Status:** [x] DONE

---

## Context

`TransitionRequest` (TASK-REQ-003-03) chỉ nhận `Stage`, `Reason`. CR-REQ-006 mục 2.3 yêu cầu `category` và ghi `request_return_history`. Test kiến trúc (TASK-REQ-003-06) cần thêm tên trường `ReturnedCategory`. Cổng `ApprovalCanceller` và `ExecutionGuard` là no-op tới CR-REQ-009, 011. Từ chối Solution hoặc Plan (CR-REQ-007, 009) đi trigger `analysis_rejected` hoặc `plan_rejected` với `category=rejected`; lỗi thực thi (CR-REQ-013) gọi use case này với `actor_kind=system`.

## Việc cần làm

1. `transition_request.go`: `TransitionInput.Category domain.ReturnCategory`; khi đích là `request_backlog` bắt buộc hợp lệ (`REQUEST_RETURN_CATEGORY_INVALID`), ghi `r.ReturnedCategory`; rời backlog đặt rỗng. `statusChangedPayload` thêm `category`. Cập nhật mọi test cũ gọi `return_to_backlog`, `analysis_rejected`, `plan_rejected` để truyền `Category`.
2. `status_write_guard_test.go`: thêm `ReturnedCategory` vào tập tên bị cấm gán ngoài `transition_request.go`.
3. `return_request_to_backlog.go` theo SOL-006 mục D: `ReturnInput`, `ReturnRequestToBacklog{repo, transition, history ReturnHistoryRepository, canceller, guard, tx, outbox}`, `Execute`.
4. `InTx`: `repo.Get`; giao lặp (đã `request_backlog` cùng `returned_from_stage`): trả thành công, không ghi; stage khác: `REQUEST_TRANSITION_NOT_ALLOWED`; `ExpectedVersion` lệch: `REQUEST_VERSION_CONFLICT`; `StageForStatus` không chứa `in.Stage`: `REQUEST_RETURN_STAGE_INVALID`; `Reason` rỗng: `REQUEST_REASON_REQUIRED`; `executing` + `guard.HasActiveExecution`: `REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION`.
5. Chọn trigger: `rejected` + `awaiting_analysis_approval` thì `analysis_rejected`; `rejected` + `awaiting_plan_approval` thì `plan_rejected`; còn lại `return_to_backlog`.
6. Thứ tự: `transition.Execute(trigger, Stage, Category, Reason, ActorID, ActorKind)` → `history.Append(returned, stage, category, reason, actor)` → `canceller.CancelPending(requestID, "returned")` → outbox `orca.request.request.returned` `{request_id, stage, category, reason, actor_id}` (cùng giao dịch).
7. `Execute` ngoài cùng không tự thử lại CAS (lớp ngoài là RPC); trả `REQUEST_VERSION_CONFLICT` cho UI tải lại.

## Kiểm thử

- `TestReturn_FromEachStatus_ValidStage_GoesBacklog` (7 trạng thái, đúng `returned_from_stage`).
- `TestReturn_StageMismatch`, `_ExecutingWithoutPhaseRejectsPhaseStage`, `_ReasonRequired`, `_CategoryInvalid`.
- `TestReturn_Idempotent_SecondCallNoHistoryNoEvent`.
- `TestReturn_RejectedUsesRejectedTriggers` (spy trên `TransitionRequest`).
- `TestReturn_ExecutingActiveBlocked` (guard giả `true`).
- `TestReturn_CancelsPendingApproval`.
- `TestReturn_EmitsReturnedAndStatusChanged`.
- Test SOL-003 cập nhật xanh; `TestOnlyTransitionRequestWritesStatus` xanh sau khi thêm tên trường.
- Lệnh: `go test ./services/request-service/internal/usecase/... -run "Return|Transition|OnlyTransition"`.

## Tiêu chí hoàn thành

- [x] Mỗi trạng thái nguồn trả về với `stage` hợp lệ; `stage` sai bị từ chối.
- [x] Sau trả về có đủ `status`, `returned_from_stage`, `returned_category`, `return_reason`, một dòng lịch sử, hai sự kiện.
- [x] Gọi lặp: không dòng lịch sử, không sự kiện.
- [x] Mọi test cũ của `TransitionRequest` vẫn xanh sau thay đổi chữ ký.

## Rủi ro và lưu ý

- Đổi chữ ký `TransitionInput` ảnh hưởng mọi nơi gọi (SOL-004, 005 chưa gọi trigger backlog); tra bằng `gitnexus_impact` hoặc `codegraph` trước khi sửa.
- Người trả về và consumer từ chối Approval cạnh tranh: bên thua nhận `REQUEST_VERSION_CONFLICT`.
