# TASK-REQ-013-02: `task-service` phát `statuschanged` với `cause` cho task có `request_id` ở mọi nhánh thực thi

**From Solution:** BE-REQ-SOL-013
**Priority:** P0
**Service:** `task-service`
**File:** `internal/usecase/task_run_events.go` (mới), `internal/usecase/execute_task.go`, `internal/usecase/report_execution_result.go`, `internal/usecase/execution_lease.go`, `internal/usecase/update_task.go` (payload), `internal/usecase/task_run_events_test.go` (mới)
**Depends on:** TASK-REQ-013-01, TASK-REQ-011-06
**Status:** `[ ] TODO`

---

## Context

- Các điểm cần phát (số dòng ngày 2026-10-06): claim `execute_task.go` dòng 229/236; hoàn tác dispatch lỗi dòng 250, 262, 316; `dispatchDirectAgentAsync` hoàn tác dòng 376 (lỗi `simple.Execute`, biến `err` có nội dung lỗi) và `CompleteExecution` dòng 385 (thành công, `review`); `report_execution_result.go` nhánh thành công dòng 85 và thất bại dòng 100; vòng phục hồi `execution_lease.go` dòng 201 (`release`, lý do `reason`).
- Engine 1 chạy trong goroutine dùng `dispatchCtx` (tenant, user), có `task` đầy đủ (kể cả `RequestID` từ SOL-011).
- Payload `taskStatusChangedPayload` (`update_task.go` dòng 181) đã có `task_type`, `parent_id`, `request_id`, `cause` từ TASK-REQ-011-05; cần thêm `execution_link_id`, `engine`, `error_message`.
- Subject không đổi: `orca.task.task.statuschanged`. Không phát `orca.task.task.completed` ở đây (đã có ở `UpdateTask`; notification không nên nhận thêm thông báo đẩy từ run).
- Chỉ phát khi `task.RequestID != ""` để task thường không thêm dòng outbox.

## Việc cần làm

1. `task_run_events.go`: 
   ```go
   type RunCause string // execute_claim, execution_completed, execution_failed, recovery
   func newRunStatusEvent(t domain.Task, prev, next domain.Status, cause RunCause, linkID string, engine domain.ExecutionEngine, errMsg string, now time.Time) (domain.OutboxEvent, bool)
   ```
   Trả `false` khi `t.RequestID == ""`. `errMsg` cắt tối đa 1024 byte (không cắt giữa ký tự UTF-8). `ID = uuid.NewString()`.
2. Mở rộng `taskStatusChangedPayload` (struct dùng chung): `ExecutionLinkID string json:"execution_link_id,omitempty"`, `Engine`, `ErrorMessage` cùng `omitempty`.
3. `ExecuteTask`: truyền `events` vào `claimer.ClaimForExecution` (cause `execute_claim`); với đường không có claimer (`UpdateStatus(in_progress)`) chuyển sang `ReleaseExecution`-kiểu hoặc giữ nhưng không phát nếu không thể cùng tx (ghi rõ trong comment và trong Rủi ro). Các nhánh hoàn tác dòng 250, 262, 316, 376 đổi sang `ReleaseExecution(..., events)` với `execution_failed` kèm `error_message`; `dispatchDirectAgentAsync` thành công dùng `CompleteExecution(..., events)` với `execution_completed`.
4. `ReportTaskExecutionResult`: thành công qua `CompleteExecution(..., events)` (`execution_completed`, `link.Engine`, `link.ID`), thất bại qua `ReleaseExecution(..., events)` (`execution_failed`, `in.ErrorMessage`). Cần `Get` task (đã có) để biết `RequestID`.
5. `RecoverInterruptedExecutions.release`: truyền event `recovery` (task đã đọc trong `domain.ExpiredRun`/`StuckTask`: kiểm xem có `RequestID`; nếu thiếu, thêm field vào `domain.ExpiredRun` và vào truy vấn `ListExpiredRuns` hai dialect, hoặc `Get` task trước khi phát).
6. Chạy `gitnexus_impact` cho `ExecuteTask.Execute`, `dispatchDirectAgentAsync`, `ReportTaskExecutionResult.Execute`; báo blast radius.
7. Thêm test hợp đồng trên payload cho consumer `request-service` (TASK-REQ-013-06): file `testdata/statuschanged_execution_failed.json`.

## Kiểm thử

- `TestNewRunStatusEvent_NoRequestID_NoEvent`, `_TruncatesErrorMessageUTF8`, `_IncludesLinkAndEngine`.
- `TestExecuteTask_RequestTask_EmitsClaimEvent`, `_DispatchFailure_EmitsExecutionFailedWithError`, `_PlainTask_NoOutboxRows` (đếm outbox trước và sau bằng nhau).
- `TestReportExecutionResult_Success_EmitsCompleted`, `_Failure_EmitsFailedWithMessage`, `_StaleCallback_NoEvent`.
- `TestRecovery_Release_EmitsRecoveryCause`.
- Integration hai dialect: vòng đời task có `request_id`: `execute_claim` rồi `execution_completed` là hai dòng outbox, đúng thứ tự `occurred_at`.
- `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/usecase/... && go test -tags=integration ./services/task-service/internal/adapter/... -run 'Execution|Outbox' -v`.

## Tiêu chí hoàn thành

- [ ] Task có `request_id` phát đúng một `statuschanged` cho mỗi chuyển trạng thái ở bốn `cause`, cùng transaction với ghi status.
- [ ] Task không có `request_id` không phát thêm gì (so số dòng outbox trước/sau).
- [ ] `error_message` có mặt ở `execution_failed` và bị cắt đúng 1 KB.
- [ ] Payload cũ vẫn parse được bởi `api-gateway` (`workspace_events.go`) và `notification-service` (trường mới `omitempty`).
- [ ] Không mất hành vi hoàn tác về `previous_status` (CR-TG-008).

## Rủi ro và lưu ý

- Đường `UpdateStatus(in_progress)` không claimer (cấu hình không bật lease) không cùng tx được; production luôn bật claimer (`WithExecutionClaim`): kiểm `cmd/server/main.go`.
- `error_message` có thể chứa đường dẫn nội bộ hoặc bí mật từ agent: không log ở mức Info, cân nhắc lọc bí mật (hỏi chủ sở hữu bảo mật).
- Subject `statuschanged` có consumer `api-gateway` ephemeral: lượng sự kiện tăng, theo dõi.
