# TASK-REQ-003-03: Use case `TransitionRequest`, sự kiện `status_changed`, `InTransaction`

**From Solution:** BE-REQ-SOL-003
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/transition_request.go`, `internal/usecase/transition_request_events.go`, `internal/usecase/transition_request_test.go`, `internal/usecase/ports.go` (sửa: thêm `InTransaction`), `internal/adapter/postgres/tx.go`, `internal/adapter/mysql/tx.go` (sửa)
**Depends on:** TASK-REQ-003-02, TASK-REQ-002-04, TASK-REQ-002-05, TASK-REQ-001-04
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: go test ./internal/domain ./internal/usecase -run Transition; go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql (Postgres 16 và MySQL 8.0 thật))

---

## Context

`TxRunner.InTx` tham gia giao dịch của ctx khi lồng (TASK-REQ-001-04). `RequestRepository.Update(ctx, r, expectedVersion)` là CAS (TASK-REQ-002-04/05), trả `REQUEST_VERSION_CONFLICT`. `usecase.NewOutboxEvent` sinh `OutboxEvent` (TASK-REQ-001-04). Mẫu idempotent: `task-service/internal/usecase/report_execution_result.go`. Subject: `orca.request.request.status_changed`, `orca.request.request.completed` (README v6 mục 8 điểm 4; không có `request.reopened`/`cancelled`).

## Việc cần làm

1. `ports.go`: thêm vào `TxRunner` phương thức `InTransaction(ctx context.Context) bool`; adapter cài bằng kiểm khóa `txKey` trong ctx (Postgres) hoặc khoá tương ứng (MySQL).
2. `transition_request.go`: kiểu `TransitionInput`, `TransitionResult`, `TransitionRequest{repo RequestRepository; tx TxRunner; outbox OutboxWriter; clock func() time.Time}`, `NewTransitionRequest(...)`.
3. `once(ctx, in) (TransitionResult, error)` theo SOL-003 mục D: `tenant.RequireTenantID`; `repo.Get` (`REQUEST_NOT_FOUND`); xử lý `ExpectedFrom` (đích của (`*ExpectedFrom`, `Trigger`) bằng `domain.NextStatus` với `flow` của `type` hiện tại; trùng `status` thì `Applied=false`); `FlowFor(r.Type)` (nếu `type` rỗng thì dùng `FlowDefinition{}` rỗng, đủ cho trigger không cần flow: `start_classification`, `proposal_ready`, `return_to_backlog`, `cancel`, `reopen`, `type_change`; trigger cần flow mà `type` rỗng trả `REQUEST_TYPE_NOT_SET`); `NextStatus`; tiền điều kiện (`REQUEST_REASON_REQUIRED` qua `domain.RequiresReason`; `type_confirmed` cần `type`); ghi `Status`, `ReturnedFromStage = in.Stage` khi đích là `request_backlog` (ngược lại xoá), `ReturnReason = in.Reason` khi vào `request_backlog` (xoá khi rời); `repo.Update(ctx, r, r.Version)`; `InsertOutboxEvent`.
4. `Execute`: nếu `tx.InTransaction(ctx)` thì gọi `once` thẳng; ngược lại vòng tối đa 3 lần `tx.InTx(ctx, func(ctx) error { res, err = once(ctx, in); return err })`, thử lại khi lỗi là `REQUEST_VERSION_CONFLICT`, sau 3 lần trả lỗi đó.
5. `transition_request_events.go`: `statusChangedPayload(...)` trả JSON `{request_id, project_id, number, from, to, trigger, type, actor_id, actor_kind, stage, reason, version, at}` (RFC3339, UTC); `completedPayload` cho `request.completed` (`{request_id, project_id, number, type, at}`).
6. Khi `Applied=false` không ghi outbox, không `Update`.
7. Không ghi `returned_category` ở đây (cột chưa có; TASK-REQ-006-02 mở rộng `TransitionInput.Category`).

## Kiểm thử

Unit với repo, outbox, tx giả (đặt cạnh test: `request_repository_fake_test.go`, `outbox_writer_fake_test.go`):
- `TestTransition_HappyPath_WritesStatusChangedEvent` (payload đủ trường).
- `TestTransition_Completed_EmitsTwoEvents`.
- `TestTransition_IdempotentRedelivery`: `ExpectedFrom` = trạng thái cũ, hiện đã ở đích: `Applied=false`, 0 sự kiện mới.
- `TestTransition_StateStale`: `ExpectedFrom` lệch và đích khác: `REQUEST_STATE_STALE`.
- `TestTransition_ReasonRequired` (cho `return_to_backlog`, `cancel`, `analysis_rejected`).
- `TestTransition_TypeNotSet` (`type_confirmed` khi `type` rỗng).
- `TestTransition_LeavingBacklogClearsStageAndReason`.
- `TestTransition_RetriesOnVersionConflictWhenOutermost` (repo giả trả conflict hai lần rồi thành công) và `TestTransition_NoRetryWhenNested` (`InTransaction=true`: conflict trả ngay).
Lệnh: `go test ./services/request-service/internal/usecase/... -run Transition`.

## Tiêu chí hoàn thành

- [x] Mọi tiêu chí unit ở trên xanh.
- [x] `InTransaction` có ở hai adapter, test nhỏ xác nhận `false` ngoài `InTx`, `true` bên trong.
- [x] Ghi `status` và `outbox` cùng một `InTx`.
- [x] Không file nào khác `transition_request.go` gán `.Status` (xem TASK-REQ-003-06).

## Rủi ro và lưu ý

- Thử lại phải mở giao dịch mới mỗi lần (REPEATABLE READ MySQL); `InTx` ngoài cùng đảm bảo điều đó, đừng lặp trong một giao dịch.
- Trigger không cần flow nhưng `type` rỗng: kiểm cẩn thận, tránh `FlowFor("")` trả `REQUEST_FLOW_UNKNOWN_TYPE` làm `return_to_backlog`/`cancel` ở `classifying` thất bại.
