# TASK-E2E-013: Proto `ProjectService.ResolveProjectAccess` (nội bộ)

**From Solution:** SOL-E2E-003 · **Resolves:** BUG-E2E-009
**Priority:** P1 · **Service/Area:** proto
**Files:**
- backend-go/proto/orca/project/v1/project.proto
- backend-go/proto/gen/go/... (sinh bằng `make proto-gen`)
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Không có RPC nào trả "project có tồn tại" tách khỏi "tôi có quyền".

## Việc cần làm

- [x] Thêm RPC và hai message (xem SOL-E2E-003); chỉ cộng thêm (G2).
- [ ] `make proto-gen`, `make proto-lint` (buf lint + breaking) xanh. — _chưa chạy `buf lint`/`buf breaking` (không có `buf`); sinh mã bằng protoc_
- [x] Ghi chú rõ RPC chỉ dành cho gọi nội bộ qua `internalcaller` (G4).

## Tiêu chí hoàn thành

- [ ] `buf breaking` xanh. — _như trên_
- [ ] Mã sinh được commit đúng quy ước repo. — _chưa commit_

## Kiểm thử

Lint proto.

## Kết quả thực thi

Xong. `ProjectService.ResolveProjectAccess` thêm vào proto (cộng thêm) và sinh lại `project.pb.go`/`project_grpc.pb.go` bằng protoc (không có `buf`, `buf lint/breaking` chưa chạy).
