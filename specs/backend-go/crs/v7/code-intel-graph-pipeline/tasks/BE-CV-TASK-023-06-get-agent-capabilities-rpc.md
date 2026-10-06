# BE-CV-TASK-023-06: RPC `GetAgentCapabilities` (proto, use case, handler)

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P1
**Service:** `infra-fleet-service` · `proto`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (sửa: thêm `rpc` cạnh `IsDevServerConnected`, dòng ~133, và message cạnh dòng ~900–905), stub sinh, `.../internal/usecase/get_agent_capabilities.go` (mới), `.../internal/adapter/grpc/server_code_intel.go` (mới, `WithCodeIntel`, handler), test
**Depends on:** TASK-023-05
**Status:** [ ] TODO

---

## Context

PQ-18: `GetAgentCapabilities` trả `capabilities[]`, `tools[]`, `platform`, `arch`, `node_version`, `agent_version`, `session_id`, `connected`. Dùng `LastHandshakeInfo` (bộ nhớ, không dial, không gọi agent), mẫu use case `IsDevServerConnected` (`is_dev_server_connected.go`, đã đọc).

## Việc cần làm

1. Proto: `rpc GetAgentCapabilities(GetAgentCapabilitiesRequest) returns (GetAgentCapabilitiesResponse);` cùng message (SOL-023 mục 2.E; số field 1–8 theo thứ tự hợp đồng). Chạy `cd backend-go/proto && buf generate`.
2. `usecase.GetAgentCapabilities.Execute(ctx, devServerID)`: `tenant.RequireTenantID` (lỗi → `INFRA_NO_TENANT`), id rỗng → `INFRA_NO_DEV_SERVER` (`KindInvalidArgument`), `DevServerRepository.Get` lỗi → `INFRA_DEV_SERVER_NOT_FOUND`, `IsConnected`; nếu connected thì `LastHandshakeInfo`; trả struct domain.
3. `server_code_intel.go`: `func (s *Server) WithCodeIntel(streamUC *usecase.StreamCodeIntelEvents, capsUC *usecase.GetAgentCapabilities) *Server`; handler `GetAgentCapabilities`; `toProtoAgentCapabilities`. Server nhúng `UnimplementedInfraFleetServiceServer` (kiểm khi biên dịch) nên chưa gọi `WithCodeIntel` thì RPC trả `Unimplemented`.
4. `main.go`: dựng use case (`usecase.NewGetAgentCapabilities(repo, agentClient)`), gọi `.WithCodeIntel(...)` trên `Server` (cùng chỗ dựng ở dòng ~384–389 và đăng ký ở ~685).

## Kiểm thử

- `buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (trong `backend-go/proto`).
- `go test ./internal/usecase/ ./internal/adapter/grpc/ -run GetAgentCapabilities -race`.
- Ca: online có tools/capabilities/platform/session; offline → `connected=false` và các trường rỗng; dev server của tenant khác → `NotFound`; thiếu tenant → `Unauthenticated`; `connected=true` nhưng `LastHandshakeInfo` không có (race) → trả `connected=true` với trường rỗng, không lỗi.

## Tiêu chí hoàn thành

- [ ] RPC cộng thêm thuần (không đổi message dùng chung `IsDevServerConnected`, `ResolveConnectionResponse`).
- [ ] Cô lập tenant được test.
- [ ] `buf breaking` xanh.

## Rủi ro và lưu ý

- Hợp đồng chưa có `hasCodeIntel` (CR Q4): collector suy từ `capabilities` chứa `codeintel` (SOL-021).
- Hai task proto (06 và 08) cùng sửa `infrafleet.proto` và stub: nếu làm song song, rebase và sinh lại stub thay vì sửa tay.
