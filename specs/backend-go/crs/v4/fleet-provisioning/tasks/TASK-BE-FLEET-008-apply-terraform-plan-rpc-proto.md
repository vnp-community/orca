# TASK-BE-FLEET-008: Proto + RPC `ApplyTerraformPlan` (server-streaming)

**Solution:** BE-FLEET-SOL-002 | **CR:** CR-FLEET-002
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-006 (`ApplyTerraformPlan` usecase phải tồn tại)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** proto/handler/wiring đúng theo sketch (bản MVP 1-event-cuối, đã ghi rõ giới hạn trong
> doc comment `ApplyTerraformPlanEvent`). `stream.Send`'s lỗi cũng được bọc qua `apperrors.New(...)` (task's
> sketch gốc `return stream.Send(...)` trực tiếp không bọc — cải thiện nhỏ để nhất quán với
> `BulkProvisionFleet`'s `sendErr` pattern, không đổi hành vi cốt lõi).
>
> **Gap thật phải lấp để `main.go` compile — KHÔNG có trong "Files cần sửa" gốc của task:** không task nào
> trong bộ 14 định nghĩa 1 adapter Go thật implement `usecase.TerraformRunner` (task 006 tự ghi "implementation
> thật... phụ thuộc TASK-BE-FLEET-007" nhưng không giao file cụ thể; task này đòi "wire... TerraformRunner
> implementation thật vào Server.New(...)" mà không liệt kê file adapter). Đã thêm
> `internal/adapter/devserveragent/terraform_runner.go` (`TerraformRunner` type, `NewTerraformRunner`) — tái
> dùng ĐÚNG cơ chế generic `DevServerAgentClient.Exec(ctx, devServer, method, params)` mà `usecase.Relay` đã
> dùng (KHÔNG tạo transport mới, KHÔNG đọc/lưu credential nào — gọi thẳng `terraform.apply` agent RPC từ
> TASK-BE-FLEET-007). Dùng interface hẹp cục bộ `agentExecutor` (chỉ `Exec`) thay vì
> `usecase.DevServerAgentClient` đầy đủ (15 method) để test không cần stub toàn bộ interface — `*devserveragent.Client`
> thật vẫn thoả mãn interface hẹp này tự nhiên. `var _ usecase.TerraformRunner = (*TerraformRunner)(nil)` khoá
> contract tại compile time.
>
> **Build cross-service:** additive-only — `go build ./...` PASS cho 6 service tiêu thụ khác (giống
> TASK-BE-FLEET-003).
>
> **Build/test thật đã chạy**: `buf generate --path orca/infrafleet/v1/infrafleet.proto` (từ `backend-go/proto/`)
> sạch, `git diff --stat` xác nhận chỉ `infrafleet.pb.go`/`infrafleet_grpc.pb.go` đổi thêm (auth files đã bị
> agent khác sửa TRƯỚC đó, xác nhận qua mtime). `go build ./...` sạch. `go test
> ./internal/adapter/grpc/... -run 'BulkProvisionFleet|ApplyTerraformPlan' -v` — 6/6 PASS. `go test
> ./internal/adapter/devserveragent/... -run TerraformRunner -v` — 5/5 PASS. `go test ./...` (toàn service) —
> PASS, không regress. `gofmt -l` sạch.

---

## Mục tiêu

Expose `ApplyTerraformPlan` usecase qua RPC server-streaming, tái dùng convention `StreamVmProvision`
(dòng 196 của `infrafleet.proto`) — stdout/stderr/result/error frame, giống `VmProvisionEvent`.

## Files cần sửa

1. `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (MODIFY)
2. `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (MODIFY)
3. `backend-go/services/infra-fleet-service/internal/adapter/grpc/server_test.go` (MODIFY hoặc file tương ứng)
4. `backend-go/services/infra-fleet-service/cmd/server/main.go` (MODIFY — wire `applyTerraformPlan` + `TerraformRunner` implementation thật vào `Server.New(...)`)

## Nội dung proto

```protobuf
rpc ApplyTerraformPlan(ApplyTerraformPlanRequest) returns (stream ApplyTerraformPlanEvent);

message ApplyTerraformPlanRequest {
  string control_dev_server_id = 1;
  string working_dir = 2;
  string vars_file = 3;
}

message ApplyTerraformPlanEvent {
  string type = 1;        // "stdout" | "stderr" | "result" | "error" — mirror VmProvisionEvent's Type convention
  string chunk = 2;
  string output_json = 3; // populated khi type == "result"
  string error_msg = 4;
}
```

**Không có event stdout/stderr thật ở usecase layer hiện tại (TASK-BE-FLEET-006):** `ApplyTerraformPlan.Execute`
(TASK-BE-FLEET-006) trả về kết quả sau khi xong, không có callback `emit` theo dòng như
`BulkProvisionFleet`. Nếu cần progress streaming thật (không chỉ 1 event "result" ở cuối), `ApplyTerraformPlan`
cần đổi chữ ký để nhận `emit func(chunk string, isStderr bool)` — **đây là điều chỉnh cần làm ở
TASK-BE-FLEET-006 nếu chưa merge, hoặc 1 task follow-up nếu đã merge**. Task này, ở phiên bản tối thiểu, có
thể chỉ gửi **đúng 1 event cuối cùng** (`type: "result"`) sau khi `Execute` trả về — chấp nhận được cho MVP,
ghi rõ đây là giới hạn đã biết, không phải thiếu sót ẩn.

```go
func (s *Server) ApplyTerraformPlan(req *infrafleetv1.ApplyTerraformPlanRequest, stream infrafleetv1.InfraFleetService_ApplyTerraformPlanServer) error {
	result, err := s.applyTerraformPlan.Execute(stream.Context(), usecase.ApplyTerraformPlanInput{
		ControlDevServerID: req.GetControlDevServerId(),
		WorkingDir:         req.GetWorkingDir(),
		VarsFile:           req.GetVarsFile(),
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	return stream.Send(&infrafleetv1.ApplyTerraformPlanEvent{
		Type:       "result",
		OutputJson: result.OutputJSON,
	})
}
```

## Test cases cần cover

- `TestServer_ApplyTerraformPlan_SendsResultEvent` — usecase trả `OutputJSON` khớp → event `type: "result"`
  có đúng `output_json`.
- `TestServer_ApplyTerraformPlan_UsecaseErrorReturnsGRPCStatus`.

## Verify

```bash
cd backend-go && buf generate --path orca/infrafleet/v1/infrafleet.proto
git diff --stat proto/gen/go/
cd backend-go/services/infra-fleet-service && go build ./... && go test ./internal/adapter/grpc/... -run ApplyTerraformPlan -v
```

## gitnexus

`impact({target: "Server", direction: "upstream", file_path: "backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go"})`
trước khi sửa constructor. `Server.ApplyTerraformPlan` — chạy `impact()` sau khi tạo.

## Blocking

TASK-BE-FLEET-014 (`DeployFleetDefinition`) gọi usecase `ApplyTerraformPlan` trực tiếp (không qua RPC này),
không phụ thuộc task này để build — nhưng RPC này cần tồn tại cho bất kỳ client (frontend/CLI) muốn gọi
`ApplyTerraformPlan` độc lập với `DeployFleetDefinition`.
