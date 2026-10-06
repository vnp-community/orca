# BE-CV-TASK-012-04: Client gRPC tới project/infra-fleet/git-gateway kèm chuyển tiếp danh tính

**From Solution:** BE-CV-SOL-012-target-resolution-and-bindings
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpcclient/{dial.go,identity_forwarding.go,project_client.go,infra_fleet_client.go,git_gateway_client.go}` và `_test.go` (mới); `internal/usecase/ports.go` (thêm cổng `ProjectDirectory`, `DevServerDirectory`, `OnDiskWorktreeDetector`)
**Depends on:** BE-CV-TASK-010-07, BE-CV-TASK-012-01
**Status:** [ ] TODO

---

## Context

Mẫu dial: `task-service/cmd/server/main.go` (`grpc.NewClient` + insecure + `otelgrpc.NewClientHandler()`). `project-service` yêu cầu user (`PROJECT_NO_USER` nếu thiếu) và role (admin toàn cục) qua metadata; chuyển đủ 4 khoá `grpcmw.MetadataTenantID/UserID/Role/ClientIP`. PQ-14 (6): `MaxCallRecvMsgSize(16 MiB)` tới infra-fleet.

## Việc cần làm

1. `identity_forwarding.go`: `withIdentity(ctx) context.Context` đọc `tenant.TenantID/UserID/Role/ClientIP` và `metadata.AppendToOutgoingContext` đủ 4 khoá (chỉ khoá có giá trị). Cùng interceptor client unary.
2. `dial.go`: hàm `Dial(addr string, extra ...grpc.DialOption)`; dial lười, lỗi không làm service chết; infra-fleet thêm `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16<<20))`.
3. Cổng (ở `usecase/ports.go`): `ProjectDirectory{GetProject, ListRepos, ListWorktrees, ListMembers}`; `DevServerDirectory{ListDevServers, IsDevServerConnected}`; `OnDiskWorktreeDetector{DetectWorktrees(ctx, repoID) ([]string, error)}`. **Không** có `GetRepo`/`GetWorktree` trong cổng (để biên dịch chặn dùng nhầm).
4. Hiện thực ba client; ánh xạ `status.Code` hạ nguồn: `Unavailable`/`DeadlineExceeded` → `apperrors.KindUnavailable` `CODEINTEL_UNAVAILABLE`; `PermissionDenied`/`NotFound` từ `GetProject` → `CODEINTEL_NOT_AUTHORIZED`; khác → `Internal`.
5. Chỉ trả trường cần (`id`, `url`, `dev_server_id`, `path`, `repo_id`, `approval_status`, `mode`, `kind`, `platform`, `role`) vào struct domain, không để proto lọt vào usecase.

## Kiểm thử

- Unit với gRPC server giả (`bufconn`): xác nhận bốn khoá metadata tới nơi; thiếu user thì không thêm khoá; `Unavailable` ánh xạ đúng; message > 4 MiB nhận được từ fake infra-fleet (cần 16 MiB).
- `go test ./services/code-intel-service/internal/adapter/grpcclient/...`

## Tiêu chí hoàn thành

- [ ] Mọi lời gọi mang đủ 4 khoá.
- [ ] Cổng không có `GetRepo`/`GetWorktree`.
- [ ] Ánh xạ lỗi theo PQ-03.

## Rủi ro và lưu ý

- Danh tính là metadata tin cậy theo mạng; không thay thế mTLS (SOL-013).
