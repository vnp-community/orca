# TASK-E2E-014: Cài `ResolveProjectAccess` và sửa phân loại lỗi của `GetProject`

**From Solution:** SOL-E2E-003 · **Resolves:** BUG-E2E-009, OBS-A
**Priority:** P1 · **Service/Area:** project-service
**Files:**
- backend-go/services/project-service/internal/usecase/resolve_project_access.go (mới)
- backend-go/services/project-service/internal/adapter/grpc/server_*.go
- backend-go/services/project-service/internal/usecase/get_project.go
**Depends on:** TASK-E2E-013
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`GetProject` id lạ trả `Internal: PROJECT_MEMBERSHIP_LOOKUP_FAILED`; người gọi không phân biệt được "không có".

## Việc cần làm

- [x] Usecase: tra cứu project theo `(tenant, id)`; `exists=false` khi không có (không phải lỗi); `role` từ bảng thành viên.
- [x] Guard `internalcaller` (G4); tenant từ metadata (G3).
- [x] `GetProject`: id không tồn tại → `NotFound`; có nhưng không có quyền → `PermissionDenied` (giữ).
- [x] Test: tồn tại + owner/member/không thành viên; không tồn tại; project của tenant khác.

## Tiêu chí hoàn thành

- [x] `project.get` id lạ trả `NotFound`.
- [x] Không rò tồn tại giữa các tenant.

## Kiểm thử

Unit test usecase + server; thêm test hợp đồng ở TASK-E2E-017.

## Kết quả thực thi

Xong. `usecase/resolve_project_access.go` + test, handler gRPC, guard `internalcaller` (`PROJECT_INTERNAL_CALLER_TOKEN`). Bổ sung vòng v2: id không phải UUID là NotFound ở repo Postgres (BUG-V2-006); e2e NFC-03/08 PASS. Test tích hợp repo chưa chạy được ở máy này (thiếu CLI `migrate`).
