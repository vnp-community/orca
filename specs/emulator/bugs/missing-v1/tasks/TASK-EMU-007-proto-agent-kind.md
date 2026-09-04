# TASK-EMU-007: Thêm `AgentKind` vào `infrafleet.proto`

**Solution:** [SOL-EMU-005](../solutions/SOL-EMU-005-backend-go-agent-kind.md)
**Priority:** P2
**Status:** `[x]` DONE — toolchain (`buf`/`protoc`/`protoc-gen-go`/
`protoc-gen-go-grpc`) có sẵn trong môi trường, verify build/vet/test xanh
trong pass này.

## Việc đã làm

1. `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`: thêm
   `enum AgentKind {AGENT_KIND_UNSPECIFIED=0, AGENT_KIND_DEV_SERVER=1,
   AGENT_KIND_MOBILE_EMULATOR=2}`; field `AgentKind kind = 8;` trên
   `DevServer` (field 7 cuối cùng trước đó là `group_id`, xác nhận trước khi
   thêm — không giả định số field trong task doc gốc); field
   `AgentKind kind = 5;` trên `RegisterDevServerRequest` (field 4 cuối cùng
   trước đó là `ssh_target_id`). Ngoài phạm vi mô tả gốc của task, còn thêm
   filter `AgentKind kind` (field 1) vào `ListDevServersRequest` và
   (field 3) vào `ListDevServersForUserRequest` — cần thiết để mục 4 bên
   dưới ("filter kind") thực sự có đường dây từ RPC request, không chỉ ở
   usecase layer.
2. Regenerate: `cd backend-go/proto && buf generate` — chạy sạch, idempotent
   (xác nhận ở Bước 0 trước khi sửa .proto: chạy trên trạng thái .proto hiện
   tại (đã có sẵn thay đổi uncommitted từ trước ở `project.proto`) không tạo
   diff thêm). Sau khi sửa `infrafleet.proto`/`project.proto`, `buf generate`
   chỉ đổi đúng `infrafleet.pb.go`, `project.pb.go`, `project_grpc.pb.go` —
   không có `infrafleet_grpc.pb.go` diff (không đổi RPC signature nào, chỉ
   thêm field lên message có sẵn).
3. Migration `backend-go/services/infra-fleet-service/migrations/0011_dev_server_kind.{up,down}.sql`
   — backfill `kind = 'dev_server'` (DEFAULT, áp dụng cho mọi hàng cũ, cùng
   pattern với migration 0008's `approval_status`).
4. `ListDevServers`/`ListDevServersForUser` (usecase + gRPC handler): thêm
   filter `kind` — rỗng/`AGENT_KIND_UNSPECIFIED` = không filter (trả tất cả,
   hành vi y hệt trước khi field này tồn tại). Filter lọc trong usecase
   layer (không đổi `DevServerRepository.List`'s signature).
5. `domain.DevServer` (infra-fleet-service): thêm field `Kind AgentKind`
   (`dev_server` | `mobile_emulator`). `NewDevServer` mặc định
   `AgentKindDevServer`; `RegisterDevServer` usecase override khi caller gửi
   kind hợp lệ khác — back-compat cho agent/ build cũ chưa gửi field này
   (map về `AGENT_KIND_UNSPECIFIED` → `""` → giữ default `dev_server`).
6. `internal/adapter/postgres/repository.go`: thêm cột `kind` vào mọi
   SELECT/INSERT/RETURNING đụng tới `infra.dev_servers`
   (`Register`/`Get`/`List`/`UpdateApprovalStatus`/`AssignGroup`/
   `scanDevServerRow`/`FindBySshTarget`/`FindByHostAndMode`/
   `ListAllDevServers`).
7. `internal/adapter/grpc/server.go`: `toDomainAgentKind`/`toProtoAgentKind`
   converters (cùng pattern `toDomainConnectionMode`/`toProtoConnectionMode`);
   `RegisterDevServer`/`ListDevServers`/`ListDevServersForUser` handlers nối
   field `kind` cả hai chiều; `toProtoDevServer` trả `Kind`.
8. Test mới: `TestListDevServers_FiltersByKind`,
   `TestListDevServersForUser_FiltersByKind` (usecase layer) — xác nhận
   `kind=""` trả tất cả (back-compat) và `kind` cụ thể lọc đúng.

## Verify (chạy thật trong pass này, có bằng chứng)

**Bước 0 — toolchain, chạy trên .proto TRƯỚC khi sửa:**
```
$ which buf protoc protoc-gen-go protoc-gen-go-grpc
/home/ubuntu/go/bin/buf            (buf 1.72.0)
/usr/bin/protoc                    (libprotoc 3.21.12)
/home/ubuntu/go/bin/protoc-gen-go
/home/ubuntu/go/bin/protoc-gen-go-grpc

$ cd backend-go/proto && buf generate
(exit 0, no output)
$ git status --porcelain backend-go/proto/   # trước và sau buf generate — không đổi
 M backend-go/proto/gen/go/orca/project/v1/project.pb.go
 M backend-go/proto/gen/go/orca/project/v1/project_grpc.pb.go
 M backend-go/proto/orca/project/v1/project.proto
```
→ idempotent, toolchain hoạt động đúng. Tiếp tục sửa `.proto`.

**Sau khi sửa proto + regenerate:**
```
$ cd backend-go/proto && buf generate && go build ./...
(exit 0, no output cả hai lệnh)
```

**`go build`/`go vet`/`go test` cho `infra-fleet-service` VÀ mọi service
import `infrafleetv1`** (`ai-provider-service`, `api-gateway`,
`git-gateway-service`, `infra-fleet-service`, `project-service`,
`task-service`, `workflow-service` — tìm bằng
`grep -rl infrafleetv1 backend-go/services --include=*.go`):
```
$ for svc in ai-provider-service api-gateway git-gateway-service \
             infra-fleet-service project-service task-service workflow-service; do
    (cd backend-go/services/$svc && go build ./... && go vet ./... && go test ./...)
  done
```
Kết quả: build/vet sạch (không output) cho cả 7 service; `go test ./...`
— toàn bộ package có test đều `ok` (infra-fleet-service:
`adapter/agentwsserver`, `adapter/devserveragent`, `adapter/sshconn`,
`adapter/sshrelay`, `domain`, `usecase`; các service còn lại tương tự,
không có FAIL nào).

## Ghi chú

- `infra-fleet-service`'s `adapter/grpc` và `adapter/postgres` không có test
  file (`[no test files]`) — hành vi trước pass này, không phải gap do pass
  này tạo ra; SQL mới chỉ verify qua build (không có DB thật trong sandbox
  để chạy integration test).
