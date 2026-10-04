# TASK-023: Regression test for `InfraFleetDevServerLister`/`InfraFleetHostnameResolver` tenant forwarding

**Solution:** [SOL-011](../solutions/SOL-011-dev-server-lister-forward-tenant-metadata.md)
**Status:** [x] DONE (2026-09-15)

## Việc cần làm

Theo đúng pattern `profile_resolver_test.go` (TASK-017, BUG-008's regression test):

1. Fake `infrafleetv1.InfraFleetServiceClient` implement `ListDevServers`, ghi lại `ctx` nhận được.
2. Test `InfraFleetDevServerLister.Exists`: gọi với 1 `ctx` có tenant hợp lệ (qua `tenant.WithTenantID` hoặc tương đương cách project-service's `Execute` thường set) — assert `ctx` truyền vào `ListDevServers` có outbound metadata key `grpcmw.MetadataTenantID` đúng giá trị.
3. Test case `ctx` KHÔNG có tenant → `Exists` trả lỗi sớm (không gọi `ListDevServers` luôn), không panic.
4. Test tương tự cho `InfraFleetHostnameResolver.Hostname`.
5. Xác nhận test FAIL trên code trước fix (bằng `git stash`/checkout tạm bản cũ) trước khi coi task DONE — theo đúng convention TASK-017/TASK-021 đã làm.

## Verify

- [x] `go test ./services/project-service/internal/adapter/grpcclient/...` — pass (4/4 mới).
- [x] Test mới xác nhận fail nếu revert SOL-011's diff — đã chạy `git stash` tạm 2 file fix, cả 4 test fail đúng như dự kiến (`expected outbound call to carry gRPC metadata` / `expected an error when ctx carries no tenant`), sau đó `git stash pop` khôi phục lại fix.
