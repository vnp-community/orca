# TASK-E2E-007: Áp dụng interceptor cho `project-service` và test lại hai sửa đổi cục bộ

**From Solution:** SOL-E2E-002 · **Resolves:** BUG-E2E-003
**Priority:** P0 · **Service/Area:** project-service
**Files:**
- backend-go/services/project-service/cmd/server/main.go
- backend-go/services/project-service/internal/adapter/grpcclient/dev_server_health_checker.go
- backend-go/services/project-service/internal/adapter/grpcclient/dev_server_relay.go
- backend-go/services/project-service/internal/adapter/grpcclient/*_test.go
**Depends on:** TASK-E2E-006
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`IsReachable` và `DevServerRelay.CreateConnection/Relay` đã được sửa bằng `withTenantMetadata` nhưng không có unit test, và các dial `grpc.NewClient` trong `main.go` vẫn thủ công.

## Việc cần làm

- [x] Dùng `grpcmw.ClientDialOptions()` cho mọi dial của service.
- [x] Viết test giả `InfraFleetServiceClient` kiểm tra metadata tenant có mặt trong `GetFleetHealth`, `CreateConnection`, `Relay`.
- [x] Sau khi test xanh, giữ hoặc xoá `withTenantMetadata` ở các adapter không còn cần (xoá từng cái, một PR nhỏ).

## Tiêu chí hoàn thành

- [x] Test chứng minh mất metadata thì đỏ.
- [x] e2e `DEV-04..06` xanh.

## Kiểm thử

Unit test adapter với fake client ghi lại metadata đến.

## Kết quả thực thi

Xong. project-service dùng `grpcmw.NewClient` ở mọi kết nối ra; bản vá cục bộ `withTenantMetadata` đã gỡ; test bufconn `infra_fleet_identity_test.go` chứng minh tenant tới infra-fleet. e2e DEV-04 PASS.
