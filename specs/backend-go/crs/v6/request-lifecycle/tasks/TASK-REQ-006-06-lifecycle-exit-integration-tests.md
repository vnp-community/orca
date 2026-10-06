# TASK-REQ-006-06: Test tích hợp trả backlog, mở lại, hủy, Request con (hai dialect)

**From Solution:** BE-REQ-SOL-006
**Priority:** P1
**Service:** `request-service`
**File:** `internal/adapter/contracttest/lifecycle_exit_contract.go` (mới), `internal/adapter/postgres/lifecycle_exit_integration_test.go`, `internal/adapter/mysql/lifecycle_exit_integration_test.go` (mới)
**Depends on:** TASK-REQ-006-05, TASK-REQ-002-06
**Status:** [ ] TODO

---

## Context

Các tiêu chí mục 4 của CR-REQ-006 cần DB thật: CHECK ghép `status`/`returned_category`, đồng thời trên `SpawnChildRequest`, rollback không để `request_links` mồ côi, cách ly tenant. Dùng `contracttest`, role không superuser (Postgres) và `ExecutionGuard` giả điều khiển được.

## Việc cần làm

1. `RunLifecycleExitContract(t, newEnv)` với kịch bản:
   - `ReturnFromEveryStatus` (dựng Request ở từng trạng thái bằng `TransitionRequest`, trả về với stage hợp lệ; kiểm cột, lịch sử, hai sự kiện).
   - `ReturnTwiceNoExtraRows`.
   - `BacklogCheckHeldEverywhere` (mọi chuyển giữ CHECK `(status='request_backlog') = (returned_category IS NOT NULL)`; chèn trực tiếp vi phạm bị DB từ chối).
   - `ReopenResetsColumnsAndKeepsSolutions`.
   - `ReopenTriggersClassificationEvent` (outbox có `status_changed` `to=classifying`, `trigger=reopen`).
   - `CancelFromBacklogAndAnalyzing`, `CancelCompletedRejected`, `CancelTwiceOK`.
   - `ActiveExecutionBlocksReturnAndCancel` (guard giả `true`).
   - `Spawn12ConcurrentSameKey` (một con, một `request_links`).
   - `SpawnChildLimit50` (50 link chèn sẵn, lần 51 bị chặn).
   - `SpawnDepthSix`.
   - `SpawnRollbackNoOrphanLink`.
   - `TenantIsolationLinksAndHistory`.
2. Hai file `_test.go` chỉ khởi môi trường và gọi hàm contract.
3. Chạy với `-race`; lặp `-count=3` cho `Spawn12ConcurrentSameKey`.

## Kiểm thử

- `go test -tags=integration -race ./services/request-service/internal/adapter/postgres/... -run LifecycleExit -v`
- `go test -tags=integration -race ./services/request-service/internal/adapter/mysql/... -run LifecycleExit -v`
- Migration `0005` up/down/up và kịch bản backfill đã có ở TASK-REQ-006-01.

## Tiêu chí hoàn thành

- [ ] Mọi kịch bản xanh với cả hai dialect.
- [ ] CHECK ghép giữ ở mọi chuyển; DB từ chối chèn vi phạm.
- [ ] 12 `SpawnChildRequest` đồng thời: một con, một link.
- [ ] Tenant B không đọc, không ghi `request_links`, `request_return_history` của tenant A.

## Rủi ro và lưu ý

- Dựng Request ở trạng thái `executing` cần đi qua cả luồng `TransitionRequest`; dùng `HappyPath` để lấy chuỗi trigger.
- MySQL có thể báo deadlock khi nhiều giao dịch cùng chạm `request_counters`; test cho phép thử lại tối đa 3 lần với lỗi 1213 ở phía gọi.
