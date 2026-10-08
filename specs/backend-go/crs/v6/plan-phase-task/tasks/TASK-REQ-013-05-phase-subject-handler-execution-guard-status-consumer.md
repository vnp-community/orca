# TASK-REQ-013-05: `SubjectHandler` cho `phase`, `ExecutionGuard` và consumer `request.status_changed` tự khởi động thực thi

**From Solution:** BE-REQ-SOL-013
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/phase_subject_handler.go` (mới), `internal/usecase/execution_guard.go` (mới), `internal/adapter/eventbus/request_status_consumer.go` (mới), `cmd/server/main.go`, `*_test.go` (mới)
**Depends on:** TASK-REQ-013-04, CR-REQ-009 (`SubjectHandler`), CR-REQ-005 (`ExecutionGuard` port, mặc định `false`), CR-REQ-006 (`ReturnToBacklog`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./... && go test -tags integration ./internal/adapter/{postgres,mysql,eventbus}` trong `request-service`)

---

## Context

- CR-REQ-009 mục 2.4 giao handler `phase` cho CR-REQ-012/013; CR này nhận. Service không khởi động nếu thiếu handler cho `subject_type` trong CHECK.
- `ExecutionGuard` là cổng do CR-REQ-005 định nghĩa (mặc định `false` = chưa có chặn); CR-REQ-006 dùng để chặn trả backlog hoặc huỷ khi còn task chạy. Hiện thực ở đây bằng `ListTasks(request_ids=[id], task_types=[task,bug,feature])` rồi kiểm có `in_progress`.
- Consumer subject nội bộ `orca.request.request.status_changed` (README v6 mục 8 điều 4: subject theo mẫu `orca.request.<entity>.<event>`, kèm `trigger`; không có `request.reopened/cancelled`). Dùng khuôn consumer bền của `task-service/internal/adapter/eventbus/consumer.go` (`commoneventbus.Consumer.Subscribe(ctx, stream, consumerName, subject, fn)`) và `processed_events` để khử trùng.
- Plan không Phase, `task_list`, `hotfix` tự chạy khi Request vào `executing` (SOL-013 2.3).

## Việc cần làm

1. `phase_subject_handler.go`: 
   - `ValidateForRequest`: Request ở `executing`; Phase thuộc Plan của Request (`ListTasks task_types=[phase]`, `request_id`, `parent_id=plan_task_id`); `digest` = SHA-256 của `(id, title)` các task con của Phase và cạnh `depends_on` giữa chúng (cùng cách mã hoá cố định của SOL-012 task 06).
   - `OnApproved`: không đổi trạng thái Request; chỉ ghi (sự kiện `approval.decided` đủ); người bấm `StartPhase` sau.
   - `OnRejected`: `ReturnToBacklog(stage=phase, category=rejected, reason=comment, actor_kind=system)` trong cùng transaction (chỉ ghi DB request-service).
   - `OnClosedWithoutDecision`: không làm gì.
2. `execution_guard.go`: `TaskExecutionGuard.HasActiveExecution(ctx, requestID) (bool, error)` bằng `TaskClient.ListTasks`; lỗi gọi task-service trả lỗi (không coi là `false`) để không trả backlog khi không biết (fail closed).
3. `request_status_consumer.go`: đăng ký consumer bền `request-service-status-start` trên subject `orca.request.request.status_changed`; payload có `to` và `type` (khớp CR-REQ-003); `to=executing` và loại có Plan không Phase (`PlanShapeFor` ≠ Plan+Phase) thì gọi `StartExecution`. Khử trùng `processed_events`; lỗi tạm thời trả error để JetStream giao lại.
4. Đăng ký handler `phase` ở `main.go` (bảng `ApprovalRegistry`); kiểm service từ chối khởi động khi thiếu.
5. Tránh vòng: consumer này và consumer outcome (task 06) cùng gọi `AdvanceExecution`; thiết kế idempotent đã đủ, chỉ cần test song song.

## Kiểm thử

- `TestPhaseSubjectHandler_Validate_WrongPhase`, `_DigestChangesOnNewTask`, `_OnRejected_ReturnsToBacklogPhase`, `_OnApproved_NoStateChange`.
- `TestExecutionGuard_ActiveTask_True`, `_NoActive_False`, `_TaskServiceDown_ReturnsError`.
- `TestStatusConsumer_ExecutingStartsPlanWithoutPhase`, `_ChangeRequestNotAutoStarted`, `_DuplicateDelivery_StartsOnce`, `_HotfixStartsSingleTask`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/eventbus/... -run 'PhaseSubject|ExecutionGuard|StatusConsumer' -v`.

## Tiêu chí hoàn thành

- [x] Phase bị từ chối đưa Request về backlog stage `phase`, `category=rejected`.
- [x] `task`, `docs`, `bug` S/M và `hotfix` tự chạy khi vào `executing` mà không cần `StartPhase`; `change_request` không tự chạy.
- [x] `ExecutionGuard` chặn khi còn task `in_progress` và chặn khi không hỏi được task-service.
- [x] Giao lặp sự kiện không chạy hai lần.

## Rủi ro và lưu ý

- Tên consumer bền và stream `REQUEST` do CR-REQ-001 quyết; dùng tên theo đó khi viết.
- Fail closed của `ExecutionGuard` có thể làm kẹt Request khi task-service sập lâu; đối soát (task 07) vẫn chạy khi hồi phục.
- Payload `status_changed` cần có `type` và `to`; nếu CR-REQ-003 chưa phát `type`, đọc Request từ DB trong handler.

## Ghi chú triển khai

Lệch so với task và điểm chưa kiểm chứng: xem `IMPLEMENTATION-NOTES.md` mục "Đợt 3, phần request-service (exec)".
