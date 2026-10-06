# BE-CV-TASK-012-07: Consumer bền `orca.project.worktree.deleted` với dedup `processed_events`

**From Solution:** BE-CV-SOL-012-target-resolution-and-bindings
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/eventbus/worktree_deleted_consumer.go`, `internal/usecase/handle_worktree_deleted.go` (+ `_test.go`), `cmd/server/main.go` (sửa)
**Depends on:** BE-CV-TASK-011-07, 011-09, 010-07
**Status:** [ ] TODO

---

## Context

Hợp đồng §5: consumer bền, stream `PROJECT`, durable `code-intel-service-worktree-deleted`. Payload (đã đọc `project-service/internal/usecase/lifecycle_events.go`): `worktree_id`, `project_id` (+ trường khác, bỏ qua). `eventbus.Consumer.Subscribe(ctx, streamName, consumerName, subject, fn)` (`common/eventbus/eventbus.go:128`); `awaitStream` chờ stream của project-service.

## Việc cần làm

1. Use case `HandleWorktreeDeleted.Execute(ctx, eventID, projectID, worktreeID)`: gắn tenant từ `Event.TenantID` vào ctx (`tenant.WithTenantID`); trong **một** transaction nếu có thể (hoặc hai bước idempotent): `MarkProcessed(eventID, subject)`; nếu `firstTime` → `DeleteByWorktreeID(projectID, worktreeID)` (+ `SnapshotRepository.DeleteByBinding` cho binding bị xoá — hoặc để bảo trì mồ côi dọn).
2. Consumer: phân tích JSON chặt (field lạ bỏ qua; thiếu `worktree_id`/`project_id` → log WARN và ack, không retry vô hạn); lỗi DB → trả lỗi để JetStream giao lại.
3. `main.go`: khởi consumer sau khi bus kết nối, chỉ khi bus sẵn sàng; lỗi đăng ký log WARN, không thoát.
4. Bỏ qua `orca.project.worktree.created`.

## Kiểm thử

- Unit: payload hợp lệ/thiếu trường; lần hai cùng `event_id` không xoá lại; tenant sai không xoá binding tenant khác.
- Integration (`common/testutil.StartNATS` + DB hai dialect): phát hai lần cùng `event_id` → binding xoá một lần; `c4_overrides`/`finding_dismissals` còn.
- `go test ./services/code-intel-service/... -run WorktreeDeleted`

## Tiêu chí hoàn thành

- [ ] Idempotent theo `(tenant_id, event_id)`.
- [ ] Consumer không làm service thoát khi bus lỗi.

## Rủi ro và lưu ý

- Stream `PROJECT` có thể chưa tồn tại lúc khởi động (`awaitStream`); chạy consumer trong goroutine.
- `orca.codeintel.>` chia sẻ stream của chính service: consumer lọc subject (§5).
