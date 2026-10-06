# TASK-REQ-006-03: Use case `ReopenRequest` và `CancelRequest`

**From Solution:** BE-REQ-SOL-006
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/reopen_request.go`, `internal/usecase/cancel_request.go` và `*_test.go` (mới)
**Depends on:** TASK-REQ-006-02
**Status:** [ ] TODO

---

## Context

CR-REQ-006 mục 2.4, 2.5. `reopen` chuyển `request_backlog` về `classifying`; consumer của SOL-005 (lọc `to=classifying`) sẽ chạy phân loại lại. `cancel` hợp lệ từ mọi trạng thái trừ `completed`, `cancelled`; `TransitionRequest` không có trigger từ trạng thái cuối nên huỷ lặp phải được xử lý trong use case (SOL-006 C6). `classification_attempts` đặt 0 khi mở lại (C4). Không có sự kiện `request.reopened`, `request.cancelled` (README mục 8 điểm 4): chỉ `status_changed` kèm `trigger`.

## Việc cần làm

1. `ReopenRequest.Execute(ctx, ReopenInput{RequestID, Note, ExpectedVersion, ActorID}) (domain.Request, error)`: `InTx`: `repo.Get`; `status != request_backlog` thì `REQUEST_REOPEN_NOT_ALLOWED`; `ExpectedVersion` lệch thì `REQUEST_VERSION_CONFLICT`; đặt `ClassificationAttempts=0` và `repo.Update(CAS)`; `transition.Execute(reopen, ExpectedFrom=request_backlog, ActorKind=user)`; `history.Append(reopened, reason=Note, user)`. Solution, Plan, Task không đụng.
2. `CancelRequest.Execute(ctx, CancelInput{RequestID, Reason, ExpectedVersion, ActorID}) (CancelResult, error)`: `InTx`: `repo.Get`; `status == cancelled` thì trả thành công `Applied=false`; `status == completed` thì `REQUEST_CANCEL_NOT_ALLOWED`; `Reason` rỗng thì `REQUEST_REASON_REQUIRED`; `executing` + `guard.HasActiveExecution` thì `REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`; `transition.Execute(cancel, ...)`; `history.Append(cancelled, reason)`; `canceller.CancelPending(requestID, "cancelled")`.
3. `Execute` của hai use case không lan sang Request con, không gọi `task-service` (Q5 của SOL-006).
4. Ghi một dòng chú thích trong mã về lý do không có sự kiện `reopened`/`cancelled` riêng (README mục 8 điểm 4).

## Kiểm thử

- `TestReopen_FromBacklog_GoesClassifying_ClearsReturnColumns_AttemptsReset`.
- `TestReopen_FromOtherStatus_Rejected` (từng trạng thái trừ backlog).
- `TestReopen_KeepsSolutionsAndTasks`.
- `TestReopen_WritesHistoryReopened`.
- `TestCancel_FromBacklog_FromAnalyzing_OK`, `_Completed_Rejected`, `_Twice_SuccessNoApplied`.
- `TestCancel_ExecutingActive_Blocked`.
- `TestCancel_ReasonRequired`, `_CancelsPendingApproval`, `_DoesNotTouchChildren` (con ở `analyzing` vẫn `analyzing`).
- Integration (từng dialect) ở TASK-REQ-006-06.
- Lệnh: `go test ./services/request-service/internal/usecase/... -run "Reopen|Cancel"`.

## Tiêu chí hoàn thành

- [ ] Mở lại: `classifying`, ba cột trả về NULL hoặc rỗng, lịch sử `reopened`, `attempts=0`.
- [ ] Hủy từ backlog và từ `analyzing` thành công, hủy `completed` bị từ chối, hủy hai lần thành công.
- [ ] Đang có Task chạy (guard giả `true`): hủy bị chặn.
- [ ] Không có sự kiện nào ngoài `status_changed` (kiểm bằng danh sách subject trong outbox giả).

## Rủi ro và lưu ý

- Mở lại nhiều lần lặp chi phí AI (Q4 của SOL-006); chưa có giới hạn.
- `completed` không mở lại được ở v6; UI cần giải thích.
