# TASK-REQ-013-06: Use case và consumer `ReportTaskOutcome`, phản hồi ngược lên Phase, Plan, Request

**From Solution:** BE-REQ-SOL-013
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/report_task_outcome.go` (mới), `internal/domain/task_outcome_class.go` (mới), `internal/adapter/eventbus/task_outcome_consumer.go` (mới), `internal/adapter/grpc/server_task_outcome.go` (mới), `proto/orca/request/v1/request_execution.proto`, `internal/usecase/report_task_outcome_test.go` (mới), `internal/adapter/eventbus/testdata/statuschanged_*.json` (mới)
**Depends on:** TASK-REQ-013-02 (payload thật), TASK-REQ-013-03, TASK-REQ-013-04, CR-REQ-006 (`ReturnToBacklog`), CR-REQ-003 (`execution_finished`)
**Status:** `[ ] TODO`

---

## Context

- Nguồn sự kiện: subject `orca.task.task.statuschanged` trên stream `TASK` (task-service tạo stream, `cmd/server/main.go` dòng ~409). Payload sau SOL-011/013: `task_id, project_id, worktree_id, previous_status, new_status, task_type, parent_id, request_id, cause, execution_link_id, engine, error_message`. Tenant ở `eventbus.Event.TenantID`, id khử trùng ở `Event.ID` (`common/eventbus/eventbus.go`).
- Consumer bền: `Subscribe(ctx, streamName, consumerName, subject, fn)` (dòng 128), khác `SubscribeEphemeral` của gateway. Khuôn tham khảo `task-service/internal/adapter/eventbus/consumer.go` (`Consumer.Start` chạy từng binding, giải mã payload, gọi use case, trả lỗi để giao lại).
- Bảng phân loại (SOL-013 2.5) là hợp đồng: viết thành hàm thuần `ClassifyTaskOutcome(taskType, cause, newStatus string) (Outcome, bool)` trong `task_outcome_class.go` để test table-driven.
- Hoàn tất: `TypePolicy.CompletionChecks` và `OnCompleted` (SOL-014) là cổng, mặc định rỗng/no-op ở task này.
- Solution không ghi thêm trạng thái sau thực thi (CR Q2).

## Việc cần làm

1. `task_outcome_class.go`: `ClassifyTaskOutcome` theo bảng: lá `execute_claim/in_progress` → `started`; lá `execution_completed/review` → `succeeded` (hành động: tự `done` nếu bật); lá `user_update/done` → `succeeded`; lá `execution_failed` hoặc `recovery` → `failed`; lá `cancelled` → `cancelled`; `phase` `derived/done` → `phase_done`; `plan` `derived/done` → `plan_done`; ngoài bảng thì `ok=false` (bỏ qua, chỉ log).
2. `report_task_outcome.go`: `ReportTaskOutcome.Execute(ctx, in ReportTaskOutcomeInput) error` trong `TxRunner.InTx`:
   1. `processed_events.TryInsert(event_id)`: đã có thì trả `nil`.
   2. Đọc Request theo `request_id`; không có thì log Warn và trả `nil` (`request_id` mồ côi). Request không `executing` thì chỉ ghi `task_run_outcomes` và dừng.
   3. Ghi `task_run_outcomes`; thực hiện hành động (ngoài tx nếu gọi task-service): `UpdateTask(done)` khi `AutoCompleteTasks`; `AdvanceExecution(parent)` khi lá `done`; lỗi: nếu `CountFailed < MaxTaskAttempts` thì `AdvanceExecution` chạy lại, ngược lại `ReturnToBacklog(stage=task, category=other, reason="Task \"<title>\" lỗi sau N lần: <error_message>")`; `phase_done`: outbox `orca.request.phase.completed {request_id, phase_task_id, task_count}`, còn Phase kế thì `OpenApproval(subject_type=phase)`, hết Phase thì hoàn tất; `plan_done`: hoàn tất; mọi lá `cancelled` thì `ReturnToBacklog(stage=phase|plan, other)`.
   4. Hoàn tất: `CompletionChecks`; có `failed` thì `ReturnToBacklog(stage=task)`; `missing` thì chờ; đạt thì `TransitionRequest(execution_finished, ExpectedFrom=executing)`, outbox `orca.request.request.completed`, rồi `OnCompleted`.
3. Tách phần ghi DB (trong tx) khỏi phần gọi `task-service`/`TransitionRequest` (sau commit) để tx ngắn; mất giữa chừng thì đối soát (task 07) chạy lại.
4. `task_outcome_consumer.go`: đăng ký bền `orca.task.task.statuschanged`, bỏ bản ghi `request_id` rỗng sớm; đặt tenant context từ `Event.TenantID`; gọi use case; lỗi tạm thời trả lỗi để giao lại, lỗi không thể xử lý (payload hỏng) log và ack.
5. `server_task_outcome.go`: RPC nội bộ `ReportTaskOutcome(ReportTaskOutcomeRequest{event_id, request_id, task_id, task_type, previous_status, new_status, cause, execution_link_id, error_message, occurred_at})` là vỏ mỏng quanh cùng use case; chỉ nhận từ service nội bộ (dùng `common/internalcaller` nếu có cơ chế; chưa kiểm chứng).
6. `testdata/statuschanged_*.json`: bản sao payload thật từ task-service (cùng file của TASK-REQ-013-02) để test hợp đồng hai phía.

## Kiểm thử

- `TestClassifyTaskOutcome_Table` (đủ bảng và dòng ngoài bảng).
- `TestReportTaskOutcome_Duplicate_ProcessedOnce`, `_OrphanRequest_Ignored`, `_RequestNotExecuting_OnlyRecords`, `_SuccessAutoCompletes`, `_SuccessManualWhenFlagOff`, `_FailureRetriesThenBacklog`, `_PhaseDone_OpensNextPhaseApprovalOnce`, `_LastPhaseDone_CompletesRequest`, `_CompletionCheckMissing_Waits`, `_CompletionCheckFailed_ReturnsBacklog`, `_AllChildrenCancelled_ReturnsBacklog`.
- `TestTaskOutcomeConsumer_DecodesRealPayload` (dùng `testdata`).
- Integration hai dialect: `processed_events` giao lặp; consumer đọc NATS thật cho vòng đời một task.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'TaskOutcome|ClassifyTaskOutcome' -v`.

## Tiêu chí hoàn thành

- [ ] Sự kiện giao hai lần (cùng `Event.ID`) cho cùng kết quả một lần.
- [ ] Task lỗi lần 1 được chạy lại; hết `MaxTaskAttempts` thì Request vào `request_backlog` với `returned_from_stage=task`, lý do chứa `error_message`.
- [ ] Task `review` được đặt `done` (cờ bật) và dependent chuyển `open` rồi được chạy.
- [ ] Phase `done` phát `phase.completed`, Approval Phase kế tạo đúng một lần; hết Phase thì `completed` và có `request.completed`.
- [ ] Payload từ task-service thật parse được (test hợp đồng trên `testdata`).

## Rủi ro và lưu ý

- `UpdateTask(done)` hàng loạt phát `orca.task.task.completed` nên `notification-service` có thể đẩy thông báo hàng loạt; cần quyết định cờ bỏ thông báo (chưa chốt).
- RPC nội bộ chưa có kiểm danh tính service gọi (cùng vấn đề `ReportTaskExecutionResult`).
- Thứ tự sự kiện không đảm bảo giữa các task: logic dựa trạng thái hiện thời đọc lại từ task-service, không dựa thứ tự nhận.
