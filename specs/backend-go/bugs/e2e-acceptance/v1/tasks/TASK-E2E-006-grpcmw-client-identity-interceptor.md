# TASK-E2E-006: Thêm `ClientIdentityInterceptor` vào `common/grpcmw`

**From Solution:** SOL-E2E-002 · **Resolves:** BUG-E2E-003, BUG-E2E-006, BUG-E2E-008
**Priority:** P0 · **Service/Area:** common/grpcmw
**Files:**
- backend-go/common/grpcmw/client_identity.go (mới)
- backend-go/common/grpcmw/client_identity_test.go (mới)
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Mỗi adapter tự gắn metadata tenant/user. Hai adapter bị sót gây `INFRA_NO_TENANT`; tác vụ nền mất user. Cần một nơi duy nhất.

## Việc cần làm

- [x] `ClientIdentityInterceptor()` (unary) và `ClientIdentityStreamInterceptor()`: đọc `tenant.TenantID/UserID/Role/ActorType` từ `ctx`; thêm metadata còn thiếu bằng đúng khoá phía nhận đọc (`MetadataTenantID`, `MetadataUserID`, … trong cùng package).
- [x] Không ghi đè khoá đã có trong outgoing metadata.
- [x] Không có tenant trong `ctx`: không bịa, để phía nhận từ chối (fail-closed).
- [x] `ClientDialOptions()` trả `[]grpc.DialOption{interceptors, otelgrpc stats handler}`.
- [x] Comment giải thích vì sao không ghi đè và vì sao fail-closed (G9).

## Tiêu chí hoàn thành

- [x] Bảng test: có/không tenant; đã có sẵn metadata; user rỗng; actor `agent`.
- [x] Không phụ thuộc service cụ thể.

## Kiểm thử

Unit test bảng; test stream.

## Kết quả thực thi

Xong. `common/grpcmw/client_identity.go` (`ClientIdentityInterceptor`, `NewClient`) + test: không ghi đè metadata có sẵn, không bịa tenant, luôn chuyển actor type không phải user.
