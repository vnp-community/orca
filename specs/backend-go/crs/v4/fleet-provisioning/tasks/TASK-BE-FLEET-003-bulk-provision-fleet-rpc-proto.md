# TASK-BE-FLEET-003: Proto + RPC `BulkProvisionFleet` (server-streaming)

**Solution:** BE-FLEET-SOL-001 | **CR:** CR-FLEET-001
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-001 (`BulkProvisionFleet` usecase phải tồn tại)
**Status:** ✅ DONE (2026-09-09)

> **ADDENDUM (phát hiện lúc làm TASK-BE-FLEET-014):** handler `BulkProvisionFleet` viết ở đây ban đầu có 1
> **race condition thật** — `emit` callback (đọc/ghi `sendErr` + gọi `stream.Send`) không có mutex, trong khi
> `BulkProvisionFleet.Execute` gọi `emit` từ N goroutine đồng thời (đúng thiết kế concurrency semaphore của
> chính usecase này) — gRPC stream không an toàn khi `Send` đồng thời từ nhiều goroutine. Phát hiện bằng `go
> test -race` khi làm TASK-BE-FLEET-014 (lần đầu tiên `-race` được chạy trên package `adapter/grpc` trong bộ
> task này). Đã sửa: thêm `sync.Mutex` bọc quanh `sendErr`/`stream.Send` trong CHÍNH handler này (không chỉ
> handler mới `DeployFleetDefinition`). Xem TASK-BE-FLEET-014's "Kết quả thực tế" cho chi tiết đầy đủ. `go
> test ./... -race -count=1` PASS sau khi sửa (chạy lặp lại 3 lần, sạch cả 3).

> **Kết quả thực tế:** proto thêm đúng `FleetSpecServerProto`/`BulkProvisionFleetRequest`/`BulkProvisionFleetEvent`
> + `rpc BulkProvisionFleet(...) returns (stream BulkProvisionFleetEvent)`, đặt cạnh `CreateSshTarget` (không
> phải cạnh `StreamVmProvision` dòng 196 như task gợi ý — khu vực đó đang được 1 agent KHÁC sửa song song cho
> CR-EVM (`SuspendEphemeralVmWorkspace` trở xuống), tránh xa để giảm rủi ro merge conflict).
>
> **Sai khác quan trọng so với sketch của task:** `FleetSpecServerProto.kind` dùng type **`AgentKind`** (enum
> proto có sẵn), KHÔNG phải `string` như task viết — đọc `RegisterDevServerRequest.kind` thật (dòng ~347,
> `AgentKind kind = 5`) xác nhận field đó vốn đã là enum, không phải string; task tự mô tả sai. Handler dùng
> lại `toDomainAgentKind`/`toProtoAgentKind` đã có sẵn trong `server.go` (không viết lại).
>
> **`tenant_id` field:** xác nhận đúng theo task — kiểm tra `ListDevServersRequest`/`StreamFileChangesRequest`/
> `GetFleetConnectivitySummaryRequest` (RPC mới nhất trong file) xác nhận convention hiện tại đã bỏ `tenant_id`
> khỏi request proto (chỉ 2 RPC cũ hơn — `CreateSshTargetRequest`/`GetFleetHealthRequest` — còn giữ field này,
> coi là legacy) → `BulkProvisionFleetRequest` theo đúng convention MỚI, không có `tenant_id`.
>
> **`buf generate` chạy từ `backend-go/proto/` (không phải `backend-go/`)** — `buf.gen.yaml` thật nằm ở
> `backend-go/proto/buf.gen.yaml`, task's lệnh mẫu `cd backend-go && buf generate` thiếu 1 cấp thư mục, đã
> điều chỉnh. `git diff --stat backend-go/proto/gen/go/orca/infrafleet/` xác nhận CHỈ
> `infrafleet.pb.go`/`infrafleet_grpc.pb.go` đổi — không đụng `auth.pb.go`/`auth_grpc.pb.go` (2 file đó đã bị
> agent KHÁC sửa TRƯỚC KHI task này chạy, xác nhận bằng mtime: `auth.pb.go` cũ hơn thời điểm chạy
> `buf generate` của task này, không phải do task này gây ra).
>
> **`cmd/server/main.go` wiring:** thực hiện GỘP với TASK-BE-FLEET-001 (xem ghi chú ở đó) — thêm
> `deleteSshTargetUC`/`bulkProvisionFleetUC` construction + truyền vào `infragrpc.New(...)` đúng vị trí tham số
> mới (`createSshTargetUC` → `bulkProvisionFleetUC` → `getFleetHealthUC`).
>
> **Build cross-service:** additive-only proto change (không sửa/xoá field nào có sẵn) — `go build ./...` PASS
> cho toàn bộ 6 service khác import `infrafleetv1` (`api-gateway`, `git-gateway-service`, `workflow-service`,
> `task-service`, `project-service`, `ai-provider-service`) — xác nhận không phá client nào.
>
> **Test:** `TestServer_BulkProvisionFleet_UsecaseErrorReturnsGRPCStatus` (tên gốc trong task) đổi tên/tách
> thành 2 test vì lý do thật: `usecase.BulkProvisionFleet.Execute` KHÔNG BAO GIỜ trả lỗi tổng quát (luôn
> `return result, nil`) — lỗi missing-tenant chỉ xuất hiện dưới dạng event `FAILED` per-server, không phải lỗi
> RPC top-level. Viết `TestServer_BulkProvisionFleet_MissingTenant_AllServersFailedNoTopLevelError` (xác nhận
> hành vi thật) + `TestServer_BulkProvisionFleet_StreamSendFails_ReturnsGRPCStatus` (nhánh lỗi tổng quát THẬT
> duy nhất còn có thể xảy ra — `stream.Send` fail) để vẫn cover đúng ý định gốc của task (nhánh lỗi chung trả
> gRPC status).
>
> **Build/test thật đã chạy**: `buf generate --path orca/infrafleet/v1/infrafleet.proto` (từ `backend-go/proto/`)
> sạch. `go build ./...` sạch (service này + 6 service tiêu thụ khác). `go test ./internal/adapter/grpc/... -run
> BulkProvisionFleet -v` — 4/4 PASS. `go test ./...` (toàn service) — PASS, không regress. `gofmt -l` sạch.

---

## Mục tiêu

Expose `BulkProvisionFleet` usecase qua 1 RPC server-streaming mới, tái dùng convention đã có ở
`StreamVmProvision` (dòng 196 của `infrafleet.proto`) cho "N bước tuần tự cần progress".

## Files cần sửa

1. `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (MODIFY — thêm message + rpc)
2. `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (MODIFY — handler mới)
3. `backend-go/services/infra-fleet-service/internal/adapter/grpc/server_test.go` (MODIFY hoặc file test tương ứng — kiểm tra convention test file hiện có cho `server.go` trước khi quyết định thêm file mới hay nối vào file cũ)
4. `backend-go/services/infra-fleet-service/cmd/server/main.go` (MODIFY — truyền `bulkProvisionFleet` vào `Server.New(...)`)

## Nội dung proto

```protobuf
// backend-go/proto/orca/infrafleet/v1/infrafleet.proto — thêm cạnh RegisterDevServer/CreateSshTarget
message FleetSpecServerProto {
  string host = 1;
  string user_name = 2;
  string vault_ssh_role = 3;
  string kind = 4; // domain.AgentKind trên wire dưới dạng string — mirror RegisterDevServerRequest's kind field hiện có
}

message BulkProvisionFleetRequest {
  repeated FleetSpecServerProto servers = 1;
  int32 concurrency = 2; // 0 = dùng default 5
}

message BulkProvisionFleetEvent {
  string host = 1;
  enum Status {
    PENDING = 0;
    SUCCEEDED = 1;
    FAILED = 2;
  }
  Status status = 2;
  string dev_server_id = 3;
  string error = 4;
}

service InfraFleetService {
  // ...rpc hiện có...
  rpc BulkProvisionFleet(BulkProvisionFleetRequest) returns (stream BulkProvisionFleetEvent);
}
```

**Không có field `tenant_id` trong `BulkProvisionFleetRequest`** — khác với draft ban đầu của CR gốc (CR
từng đề xuất `string tenant_id = 1`). Quyết định: bỏ hẳn field này khỏi request proto để loại trừ hoàn toàn
khả năng 1 client cố tình/vô tình gửi `tenant_id` khác — `tenantID` chỉ có thể đến từ
`tenant.RequireTenantID(ctx)` (xác thực qua gRPC metadata/interceptor), không có đường nào khác. Nếu convention
message khác trong cùng file luôn có `tenant_id` field cho mục đích logging, kiểm tra lại 1-2 RPC mới nhất
khác (`GetFleetConnectivitySummary`, ...) xem có giữ field đó không, và làm nhất quán — không tự quyết ngược
lại convention nếu đã có tiền lệ rõ ràng.

## Chạy `buf generate`

```bash
cd backend-go && buf generate --path orca/infrafleet/v1/infrafleet.proto
git diff --stat proto/gen/go/  # xác nhận CHỈ infrafleet.pb.go/infrafleet_grpc.pb.go đổi thêm, không phá service khác
```

## `adapter/grpc/server.go` — handler

```go
func (s *Server) BulkProvisionFleet(req *infrafleetv1.BulkProvisionFleetRequest, stream infrafleetv1.InfraFleetService_BulkProvisionFleetServer) error {
	servers := make([]usecase.FleetSpecServer, 0, len(req.GetServers()))
	for _, sp := range req.GetServers() {
		servers = append(servers, usecase.FleetSpecServer{
			Host:         sp.GetHost(),
			UserName:     sp.GetUserName(),
			VaultSSHRole: sp.GetVaultSshRole(),
			Kind:         domain.AgentKind(sp.GetKind()),
		})
	}
	spec := usecase.FleetSpec{Servers: servers}

	var sendErr error
	_, err := s.bulkProvisionFleet.Execute(stream.Context(), spec, int(req.GetConcurrency()), func(r usecase.BulkProvisionServerResult) {
		if sendErr != nil {
			return // đã lỗi 1 lần, không cố gửi tiếp — stream có thể đã đóng
		}
		status := infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED
		if r.Status == "FAILED" {
			status = infrafleetv1.BulkProvisionFleetEvent_FAILED
		}
		sendErr = stream.Send(&infrafleetv1.BulkProvisionFleetEvent{
			Host: r.Host, Status: status, DevServerId: r.DevServerID, Error: r.Error,
		})
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	if sendErr != nil {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "INFRA_BULK_PROVISION_STREAM_SEND_FAILED", "failed to stream event", sendErr))
	}
	return nil
}
```

**Xác nhận tên method thật của `apperrors.ToGRPCStatus`/`apperrors.New`** trước khi khoá code — đã dùng
đúng tên này ở `create_ssh_target.go`/`register_dev_server.go`, giữ nguyên convention.

## Wiring `main.go`

```go
srv := grpcadapter.NewServer(
	// ...tham số hiện có...
	bulkProvisionFleet,
)
```

Xác nhận chữ ký thật của `Server.New(...)`/hàm khởi tạo `Server` trước khi thêm tham số — đọc lại
`adapter/grpc/server.go`'s constructor hiện có, không đoán tên tham số.

## Test cases cần cover

- `TestServer_BulkProvisionFleet_StreamsEventPerServer` — fake `bulkProvisionFleet` usecase gọi `emit` N lần,
  xác nhận `stream.Send` được gọi đúng N lần với đúng `Status`.
- `TestServer_BulkProvisionFleet_MapsFailedStatusCorrectly` — 1 server `FAILED` → event có
  `Status: BulkProvisionFleetEvent_FAILED`, `Error` khớp.
- `TestServer_BulkProvisionFleet_UsecaseErrorReturnsGRPCStatus` — usecase `Execute` trả lỗi tổng quát (không
  phải per-server) → RPC trả gRPC status lỗi tương ứng qua `apperrors.ToGRPCStatus`.

## Verify

```bash
cd backend-go && buf generate --path orca/infrafleet/v1/infrafleet.proto
cd backend-go/services/infra-fleet-service && go build ./... && go test ./internal/adapter/grpc/... -run BulkProvisionFleet -v
gofmt -l internal/adapter/grpc/server.go
```

## gitnexus

`impact({target: "Server", direction: "upstream", file_path: "backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go"})`
trước khi sửa constructor `Server.New(...)`/struct `Server` — xác nhận không phá caller nào khác (mirror
`TASK-BE-STORAGE-003`'s gitnexus note). `Server.BulkProvisionFleet` (method mới) — chạy `impact()` sau khi
tạo, trước khi task khác thêm caller.

## Blocking

Không có task nào trong bộ này phụ thuộc trực tiếp vào RPC này (frontend cutover là phạm vi CR-RBAC-001,
ngoài bộ task này) — nhưng TASK-BE-FLEET-014 (`DeployFleetDefinition`) gọi lại usecase `BulkProvisionFleet`
trực tiếp (không qua RPC này), nên không có phụ thuộc ngược.
