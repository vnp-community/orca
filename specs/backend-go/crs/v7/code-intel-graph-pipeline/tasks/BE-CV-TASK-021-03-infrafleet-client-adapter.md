# BE-CV-TASK-021-03: Adapter `infrafleetclient` (Dial 16 MiB, tenant, gọi `RelayByDevServer`, giải mã kết quả)

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/adapter/infrafleetclient/{agent_codeintel_gateway.go,agent_rpc_caller.go,tenant_forwarding.go,agent_result_decoding.go}` (mới), `.../internal/config/config.go` (sửa) và test
**Depends on:** TASK-021-02, BE-CV-SOL-023
**Status:** [x] DONE

---

## Context

Mẫu: `git-gateway-service/.../grpcclient/resolver.go` (`Dial`), `tenant_forwarding.go`, `relay_executor.go`. Không có `MaxCallRecvMsgSize` ở bất kỳ đâu trong backend-go (PQ-14(6)).

## Việc cần làm

1. `Dial(addr)`: `grpc.NewClient`, insecure, `otelgrpc.NewClientHandler()`, `MaxCallRecvMsgSize(16<<20)`. Địa chỉ `INFRA_FLEET_SERVICE_ADDR`.
2. `withTenantMetadata` viết lại (không import chéo).
3. `Call`: `context.WithTimeout(95s)`, `RelayByDevServer`, `grpc.Trailer(&md)`; giải mã `result_json` (`UseNumber`) vào `RawCodeIntelResult`; phát hiện phong bì lỗi JSON-RPC trong kết quả → `CODEINTEL_TOOL_FAILED`; kết quả > `CODEINTEL_AGENT_MAX_RESULT_BYTES` (12 MiB) → `CODEINTEL_OUTPUT_TOO_LARGE`.
4. Semaphore `CODEINTEL_MAX_INFLIGHT_PER_DEVSERVER` (4).
5. Config: `INFRA_FLEET_SERVICE_ADDR`, `CODEINTEL_AGENT_CALL_TIMEOUT`, `CODEINTEL_MAX_INFLIGHT_PER_DEVSERVER`, `CODEINTEL_AGENT_MAX_RESULT_BYTES`.
6. Log không chứa `params_json`, `data`, đường dẫn tuyệt đối.

## Kiểm thử

- `go test ./internal/adapter/infrafleetclient/ -race` với `InfraFleetServiceClient` giả.
- Ca: kết quả 8 MiB đi qua; 13 MiB → OUTPUT_TOO_LARGE; JSON hỏng/thiếu `data` → RESULT_INVALID; thiếu tenant → lỗi; `params_json` bắt được không chứa khoá cấm; semaphore giới hạn 4.

## Tiêu chí hoàn thành

- [x] Mọi lời gọi mang metadata tenant.
- [x] Log sạch dữ liệu.
- [x] 8 MiB qua được.

## Rủi ro và lưu ý

- 12 MiB × song song tốn RAM (chưa đo).
- Không log `Data` (có thể chứa mã nguồn).
