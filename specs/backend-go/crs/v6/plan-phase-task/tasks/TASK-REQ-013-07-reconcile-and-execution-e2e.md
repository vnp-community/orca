# TASK-REQ-013-07: `ReconcileExecutingRequests`, wiring và kiểm thử đầu cuối thực thi

**From Solution:** BE-REQ-SOL-013
**Priority:** P1
**Service:** `request-service` (kèm kiểm thử `task-service`)
**File:** `backend-go/services/request-service/internal/usecase/reconcile_executing_requests.go` (mới), `internal/adapter/postgres/executing_requests.go`, `internal/adapter/mysql/executing_requests.go` (mới), `cmd/server/main.go`, `internal/config/config.go`, `internal/usecase/execution_flow_integration_test.go` (mới)
**Depends on:** TASK-REQ-013-04, 05, 06
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./... && go test -tags integration ./internal/adapter/{postgres,mysql,eventbus}` trong `request-service`; E2E với task-service giả qua bufconn)

---

## Context

- Sự kiện at-least-once nhưng có thể mất: `ReleaseUnlinkedInProgress` là update hàng loạt không phát sự kiện; NATS có thể chưa kết nối lúc khởi động (task-service cho phép chạy khi stream chưa sẵn sàng, `cmd/server/main.go` dòng ~409); consumer có thể tắt. Đối soát là lưới an toàn (SOL-013 2.8).
- Khoá cho nhiều replica: Postgres `FOR UPDATE SKIP LOCKED`; MySQL cần ≥ 8.0.1 cho `SKIP LOCKED` hoặc `GET_LOCK` (CR-DB-002; chưa kiểm chứng TiDB). Dùng `common/dbcapability` để rẽ nhánh.
- Cấu hình: `REQUEST_RECONCILE_INTERVAL=60s`, `REQUEST_DISPATCH_RETRY_WINDOW=15m`, `REQUEST_MAX_PARALLEL_TASKS=1`, `REQUEST_MAX_TASK_ATTEMPTS=2`, `REQUEST_AUTO_COMPLETE_TASKS=true` vào `config.Config` (nhúng `commonconfig.Base`).
- Mẫu vòng nền: `RecoverInterruptedExecutions.RunRecoveryLoop` (`task-service/internal/usecase/execution_lease.go` dòng 216).

## Việc cần làm

1. `executing_requests.go` (mỗi dialect): `ListStaleExecuting(ctx, quietFor time.Duration, limit int) ([]domain.Request, error)` chọn Request `status='executing'` mà `MAX(task_run_outcomes.occurred_at)` (hoặc `requests.updated_at`) cũ hơn `quietFor`, khoá `SKIP LOCKED` (Postgres) hoặc nhận khoá `GET_LOCK` theo Request (MySQL).
2. `reconcile_executing_requests.go`: `ReconcileExecutingRequests.Execute(ctx) (int, error)`: với mỗi Request, `ListTasks(request_ids=[id])`, rồi chạy lại bước 3 của 2.5 theo trạng thái hiện thời: task `open` không có run thì `AdvanceExecution`; dispatch lỗi tạm thời quá `DispatchRetryWindow` thì `ReturnToBacklog(stage=task, category=blocked_dependency, reason="Không chạy được: <mã lỗi>")`; phase `done` mà chưa có `phase_done` thì ghi và phát `phase.completed` rồi mở Approval kế; mọi lá `done` thì hoàn tất. Dùng đúng hàm của use case 06 (không nhân đôi logic): tách hàm `evaluateExecutionState(ctx, request)` dùng chung.
3. `RunReconcileLoop(ctx, interval)` giống `RunRecoveryLoop`; khởi chạy ở `main.go` sau khi dựng usecase; tắt khi `ctx` huỷ.
4. Thêm cấu hình vào `config.go` kèm giá trị mặc định và kiểm hợp lệ.
5. E2E: dựng task-service thật (DB test) và request-service với `PlanGenerator` giả: `change_request` từ Plan đã duyệt → `StartPhase` → task chạy bằng `SimpleExecutor` giả → `review` → `done` → Phase `done` → Approval Phase kế → hết Phase → `completed`; một task lỗi cố ý (executor giả trả lỗi 2 lần) → `request_backlog` stage `task`.
6. Kịch bản mất sự kiện: tắt consumer 2 phút (hoặc xoá tay dòng outbox), chạy `Reconcile`, Request về trạng thái đúng trong một chu kỳ.
7. Kịch bản `ReleaseUnlinkedInProgress`: task `in_progress` không link quá grace về `open`, đối soát chạy lại dispatch.

## Kiểm thử

- Unit: `TestReconcile_ReDispatchesOpenTaskWithoutRun`, `_ReturnsBacklogAfterRetryWindow`, `_EmitsPhaseCompletedOnce`, `_CompletesRequestWhenAllDone`, `_SkipsRecentActivity`.
- Integration hai dialect: khoá không cho hai replica xử lý cùng Request (hai goroutine, đếm lời gọi `Execute` giả).
- E2E: `TestExecutionFlow_ChangeRequest_ToCompleted`, `TestExecutionFlow_TaskFailsTwice_ToBacklog`, `TestExecutionFlow_LostEvents_ReconcileFixes`, `TestExecutionFlow_SecondTaskSameWorktree`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... && go test -tags=integration ./services/request-service/... ./services/task-service/internal/adapter/... -run 'Reconcile|ExecutionFlow' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Sau khi mất sự kiện, đối soát đưa Request về trạng thái đúng trong một chu kỳ (`REQUEST_RECONCILE_INTERVAL`).
- [x] Hai replica không xử lý trùng một Request.
- [x] Toàn bộ tiêu chí mục 4 của CR-REQ-013 có test tương ứng.
- [x] Hai dialect cùng kết quả; cấu hình mới có mặc định và kiểm hợp lệ.
- [x] Task thứ hai của Plan chạy trong cùng `worktree_id` (kiểm trên `GetTask`).

## Rủi ro và lưu ý

- Đối soát và consumer có thể đua; thiết kế idempotent (`processed_events`, CAS `TransitionRequest`, `phase_starts`) đã chịu; test đua bắt buộc.
- E2E cần cả hai service: có thể chậm hoặc không chạy được trên CI hiện tại; ghi rõ phần chưa chạy.
- Chu kỳ 60 giây và cửa sổ 15 phút là giá trị đề xuất, chưa đo.

## Ghi chú triển khai

Lệch so với task và điểm chưa kiểm chứng: xem `IMPLEMENTATION-NOTES.md` mục "Đợt 3, phần request-service (exec)".
