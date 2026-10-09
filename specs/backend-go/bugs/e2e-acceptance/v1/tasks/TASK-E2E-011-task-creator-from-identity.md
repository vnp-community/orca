# TASK-E2E-011: `CreateTask` suy người tạo từ danh tính, không tin `creator_id` của client

**From Solution:** SOL-E2E-002 · **Resolves:** BUG-E2E-006
**Priority:** P1 · **Service/Area:** task-service, api-gateway
**Files:**
- backend-go/services/task-service/internal/adapter/grpc/server_create_task.go
- backend-go/services/task-service/internal/usecase/create_task.go
- backend-go/services/api-gateway/internal/adapter/wscompat/channels.go (đã truyền CreatorId)
- backend-go/services/task-service/internal/adapter/grpc/*_test.go
**Depends on:** TASK-E2E-006
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Task chỉ nhận grant `owner` khi `CreatorID` khác rỗng. Gateway từng không truyền. Quy tắc phải nằm ở miền (G1, G3), không phụ thuộc client nhớ truyền.

## Việc cần làm

- [x] Trong gRPC server: lấy `tenant.UserID(ctx)`; dùng làm `CreatorID`. Chỉ chấp nhận `creator_id` từ request khi lời gọi mang token nội bộ (AIApply, tạo từ nguồn).
- [x] Giữ gateway truyền `CreatorId` (tương thích đường nội bộ).
- [x] Test: client công khai gửi `creator_id` giả bị bỏ qua; lời gọi nội bộ được phép.
- [x] Thêm test gateway: `task.create` truyền `CreatorId` từ `Identity`.

## Tiêu chí hoàn thành

- [x] e2e `TSK-07` xanh với task mới.
- [x] Không thể giả mạo chủ task bằng tham số.

## Kiểm thử

Unit test server + gateway; `TSK-07`.

## Kết quả thực thi

Xong. `toCreateTaskInput` suy CreatorID từ người gọi đã xác thực; gateway truyền `CreatorId`; test đơn vị. e2e TSK-01 PASS.
