# TASK-REQ-011-05: `DeriveContainerStatus`, `SyncContainerStatus` và CAS `UpdateContainerStatus`

**From Solution:** BE-REQ-SOL-011
**Priority:** P0
**Service:** `task-service`
**File:** `internal/domain/container_status.go` (mới), `internal/usecase/sync_container_status.go` (mới), `internal/usecase/ports.go`, `internal/adapter/postgres/container_status.go` (mới), `internal/adapter/mysql/container_status.go` (mới), `internal/usecase/update_task.go` (struct payload)
**Depends on:** TASK-REQ-011-02
**Status:** `[ ] TODO`

---

## Context

- `domain/task.go` `SetStatus` (dòng 232): từ chối `in_progress` (`ErrCannotSetInProgress`) và mọi chuyển khỏi `done/cancelled`. Phải giữ nguyên; giá trị suy ra đi đường repository, như `ExecuteTask` làm với `UpdateStatus`.
- `Repository.Update` (postgres dòng 425): mẫu transaction "UPDATE task + INSERT `task.outbox_events`" (`id, tenant_id, subject, occurred_at, version, payload`); MySQL có bản tương đương. `UpdateContainerStatus` bám cùng khuôn.
- `usecase/recalculate_progress.go` `RecalculateProgress.Execute(ctx, rootID) (int, error)`: cascade tiến độ sẵn có từ v4 (BE-SOL-001 đã triển khai); chỉ gọi lại, không viết lại.
- `taskStatusChangedPayload` ở `update_task.go` dòng 181 (5 trường). Subject `orca.task.task.statuschanged`; `api-gateway/.../wscompat/workspace_events.go` và `notification-service` đang nghe subject này và bỏ qua trường lạ.
- Container `cancelled` không bao giờ bị suy ra lại; container `done` được phép mở lại khi có con mới chưa xong (log Warn, theo CR; đang là câu hỏi mở Q1).

## Việc cần làm

1. `domain/container_status.go`: `func DeriveContainerStatus(children []Status) (Status, bool)` theo bảng của solution (thứ tự ưu tiên: không con thì giữ nguyên; toàn `cancelled`; toàn `done`; có `in_progress`; toàn {done,review} có review; có {done,review} lẫn {open,blocked}; toàn `blocked`; còn lại `open`). Bỏ con `cancelled` trước khi xét.
2. `ports.go` `TaskRepository` thêm: `ListChildStatuses(ctx, tenantID, parentID string) ([]domain.Status, error)` và `UpdateContainerStatus(ctx, tenantID, id string, from, to domain.Status, events []domain.OutboxEvent) (changed bool, err error)`.
3. Adapter: `ListChildStatuses` = `SELECT status FROM tasks WHERE tenant_id = ? AND parent_id = ?`. `UpdateContainerStatus` = một transaction: `UPDATE ... SET status = to, updated_at = now WHERE tenant_id = ? AND id = ? AND status = from AND task_type IN ('plan','phase')`, `RowsAffected()==0` thì trả `changed=false` không lỗi; có thì INSERT outbox.
4. `usecase/update_task.go`: `taskStatusChangedPayload` thêm `TaskType string json:"task_type,omitempty"`, `ParentID`, `RequestID`, `Cause` (`omitempty`). `UpdateTask` điền `Cause = "user_update"` và các trường mới (đổi nhỏ, test hiện có không được vỡ).
5. `usecase/sync_container_status.go`: `type SyncContainerStatus struct{ repo TaskRepository; progress *RecalculateProgress }`; `Execute(ctx, changedTaskID string) error`:
   - `Get` task; `ParentID` rỗng hoặc parent không phải container thì dừng.
   - `ListChildStatuses` → `DeriveContainerStatus`; `ok=false` hoặc bằng trạng thái hiện tại thì lên cấp trên.
   - Parent đang `cancelled` thì bỏ qua. Parent `done` mà trạng thái mới khác `done` thì `slog.Warn`.
   - `UpdateContainerStatus` kèm một `OutboxEvent{Subject:"orca.task.task.statuschanged"}` (payload có `task_type`, `parent_id`, `request_id` của container, `previous_status`, `new_status`, `cause:"derived"`); mất CAS thì đọc lại và thử tối đa 3 lần.
   - `progress.Execute(ctx, parentID)` best-effort (log lỗi).
   - Lặp với parent làm "task vừa đổi" đến khi parent không phải container. Không phát `orca.task.task.completed` cho container.
6. Thêm constructor `NewSyncContainerStatus(repo, progress)`; chưa nối vào đâu (task 06).
7. Cập nhật các fake `TaskRepository` trong `fakes_test.go` và `adapter/grpc/server_test.go`.

## Kiểm thử

- `domain/container_status_test.go`: `TestDeriveContainerStatus` table-driven đủ 8 dòng cộng không con, toàn `cancelled`, một con `cancelled` lẫn `done` (=> `done`).
- `usecase/sync_container_status_test.go`: `TestSync_PhaseThenPlan_TwoLevels`, `TestSync_LostCAS_Retries`, `TestSync_NoChildren_NoWrite`, `TestSync_CancelledContainer_Untouched`, `TestSync_EmitsStatusChangedWithCauseDerived`.
- Integration hai dialect: `TestRepository_UpdateContainerStatus_CAS_8Goroutines` (đúng một người thắng), `_WritesOutboxSameTx` (chèn lỗi outbox thì status không đổi), `_RejectsNonContainerRow`.
- `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/domain/... ./services/task-service/internal/usecase/... && go test -tags=integration ./services/task-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [ ] Bảng suy ra đúng từng dòng (test table-driven xanh).
- [ ] `UpdateContainerStatus` là CAS, một dòng outbox mỗi lần đổi, cùng transaction.
- [ ] `SetStatus` không đổi; không đường nào đặt container `in_progress` qua `UpdateTask`.
- [ ] Container `done` mở lại có log Warn; container `cancelled` không bị suy ra.
- [ ] Payload cũ vẫn parse được bởi consumer cũ (trường mới `omitempty`).

## Rủi ro và lưu ý

- `UpdateTask` ghi lại toàn hàng (kể cả `status`) nên có thể đè trạng thái suy ra khi đua; task 06 thêm đối soát.
- Hai dialect: `LIMIT`/`IN` khác nhau nhưng truy vấn ở đây đơn giản; chú ý MySQL dùng `?` và CHAR(36).
- Đặt tên file theo khái niệm (`container_status`), không dùng `helpers`.
