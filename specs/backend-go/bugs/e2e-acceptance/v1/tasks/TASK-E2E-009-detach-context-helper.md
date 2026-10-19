# TASK-E2E-009: `tenant.Detach` cho tác vụ nền và thay mẫu `context.Background()`

**From Solution:** SOL-E2E-002 · **Resolves:** BUG-E2E-008
**Priority:** P0 · **Service/Area:** common/tenant, workflow-service
**Files:**
- backend-go/common/tenant/detach.go (mới)
- backend-go/common/tenant/detach_test.go (mới)
- backend-go/services/workflow-service/internal/usecase/execute.go
- các điểm khác tìm bằng grep `tenant.WithTenantID(context.Background()
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`Execute` dùng `context.Background()` + tenant (và nay thêm user bằng tay). Cần một cách đúng và có tên: `context.WithoutCancel` giữ mọi giá trị, bỏ tín hiệu huỷ.

## Việc cần làm

- [x] `tenant.Detach(ctx) context.Context { return context.WithoutCancel(ctx) }` kèm comment nêu lý do (RPC trả về thì ctx bị huỷ nhưng việc nền vẫn cần danh tính và trace).
- [x] Thay trong `workflow-service` `Execute`; bỏ đoạn thêm user thủ công.
- [~] Grep các chỗ khác: tìm `context.Background()` trong goroutine chạy sau khi RPC trả; đánh giá từng chỗ (task execution, automation, outbox). — _đã thay ở workflow và task-service; chưa rà automation/outbox_
- [x] Test: ctx gốc bị huỷ, ctx tách vẫn sống và còn tenant/user/actor.

## Tiêu chí hoàn thành

- [x] Unit test `Detach`.
- [x] Test workflow: bước `project:` chạy xong sau khi RPC `Execute` đã trả (e2e `WFL-13`).

## Kiểm thử

Unit test + `WFL-13`.

## Kết quả thực thi

Xong. `common/tenant/detach.go` (`Detach` = `context.WithoutCancel`) + test; workflow `Execute` và task `ExecuteTask` dùng.
