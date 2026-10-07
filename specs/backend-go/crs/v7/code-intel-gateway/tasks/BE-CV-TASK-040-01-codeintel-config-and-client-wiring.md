# BE-CV-TASK-040-01: Cấu hình `CodeIntelConfig`, dial `code-intel-service` có điều kiện, health, `ChannelDeps`

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/config/config_codeintel.go` (mới), `internal/config/config_codeintel_test.go` (mới), `internal/config/config.go`, `cmd/server/codeintel_wiring.go` (mới), `cmd/server/codeintel_wiring_test.go` (mới), `cmd/server/main.go`, `internal/adapter/wscompat/register_production.go`
**Depends on:** cổng G0 (stub `codeintelv1` của BE-CV-SOL-010/020 đã sinh). Nếu chưa có stub: làm phần config trước, phần dial sau.
**Status:** [x] DONE

---

## Context

- `config_mcp.go` là mẫu: slice cấu hình riêng + `loadMCP`. `OtherServiceAddrs` (`config.go:153-168`) bị dial hết ở `main.go:162-260` nên **không** thêm `code-intel-service` vào đó (SOL-040-foundation C1).
- Mẫu dial có điều kiện: `main.go:387-398` (`mcpConn`), có token: `cmd/server/mcp_governance_wiring.go:24-32`; test token trên cả unary và stream: `mcp_dial_token_test.go`.
- Hợp đồng: PQ-14 (6) `MaxCallRecvMsgSize(4 MiB)`, PQ-23 (tên biến), proto-and-data-map §3 mở đầu (service chặn mọi RPC khi `CODEINTEL_INTERNAL_CALLER_TOKEN` rỗng), §6.2.
- Chạy `gitnexus_impact` trên `RegisterProductionChannels` và `Config` trước khi sửa (quy ước dự án; expected: nhiều caller test, rủi ro thấp vì chỉ thêm field).

## Việc cần làm

1. `config_codeintel.go`: `type CodeIntelConfig struct{ServiceAddr string; MaxResponseBytes, MaxStreams int}`; `func loadCodeIntel() (CodeIntelConfig, error)` đọc `CODE_INTEL_SERVICE_ADDR` (mặc định `""`), `CODE_INTEL_MAX_RESPONSE_BYTES` (mặc định `2<<20`, hợp lệ `64<<10 ... 3<<20`), `CODE_INTEL_MAX_STREAMS` (mặc định `500`, hợp lệ `1 ... 10000`). Giá trị sai kiểu/ngoài khoảng => lỗi khởi động nêu tên biến, không nêu giá trị khác.
2. `config.go`: thêm `CodeIntel CodeIntelConfig` vào `Config`, gọi `loadCodeIntel()` trong `Load()`.
3. `cmd/server/codeintel_wiring.go`: `dialCodeIntelService(addr string) (*grpc.ClientConn, error)` theo SOL-040-foundation 2.1: insecure + `otelgrpc` + `WithDefaultCallOptions(MaxCallRecvMsgSize(4<<20))` + (khi `CODEINTEL_INTERNAL_CALLER_TOKEN` khác rỗng) `internalcaller.ClientInterceptor`/`StreamClientInterceptor`. Hàm `buildCodeIntelClients(cfg, logger)` trả `(core codeintelv1.CodeIntelServiceClient, quality codeintelv1.QualityGateServiceClient, conn *grpc.ClientConn, err)`; địa chỉ rỗng => cả hai `nil` (giá trị interface nil thật), log `Warn` một lần; địa chỉ có nhưng token rỗng => thêm `Warn` "internal caller token empty: code-intel-service will reject calls".
4. `main.go`: gọi `buildCodeIntelClients`; `defer conn.Close()` khi khác nil; `healthSrv.Register("code-intel-service", grpcConnHealthCheck(conn))` chỉ khi `conn != nil` (cạnh `mcpConn`, `main.go:569`); truyền `CodeIntel`, `QualityGate`, `CodeIntelLimits{MaxResponseBytes, MaxStreams}` vào `wscompat.ChannelDeps{...}` (`main.go:371`).
5. `register_production.go`: thêm ba trường vào `ChannelDeps`; `type CodeIntelLimits struct{MaxResponseBytes, MaxStreams int}` (giá trị 0 => mặc định 2 MiB / 500 trong `registerCodeIntelChannels`, để `ChannelDeps{}` rỗng vẫn hợp lệ); lời gọi `registerCodeIntelChannels(r, d)` ở cuối `RegisterProductionChannels` (hàm do TASK-040-07 tạo; ở task này tạo bản rỗng `func registerCodeIntelChannels(*Registry, ChannelDeps) {}` trong file mới `channels_codeintel_register.go` để biên dịch được).

## Kiểm thử

- `config_codeintel_test.go`: mặc định; ghi đè hợp lệ; `CODE_INTEL_MAX_RESPONSE_BYTES=abc`, `=0`, `=4194304` (vượt 3 MiB) => lỗi nêu tên biến; `CODE_INTEL_MAX_STREAMS=-1` => lỗi.
- `codeintel_wiring_test.go` theo `mcp_dial_token_test.go`: server gRPC giả có `internalcaller.Guard("s3cret", ...)` + `StreamGuard`; client dial với `CODEINTEL_INTERNAL_CALLER_TOKEN=s3cret` gọi được unary **và** stream; không có token => `PermissionDenied`/`Unauthenticated` (theo `internalcaller`). Test thứ hai: server giả trả message 5 MiB => client nhận `ResourceExhausted` (chứng minh `MaxCallRecvMsgSize` 4 MiB hiệu lực).
- `buildCodeIntelClients` với địa chỉ rỗng trả hai interface `== nil` (so sánh interface, không phải con trỏ).
- `register_production_test.go`: `ChannelDeps{}` rỗng vẫn gọi `RegisterProductionChannels` không panic.
- Lệnh: `cd backend-go/services/api-gateway && go test ./internal/config/... ./cmd/server/... ./internal/adapter/wscompat/ -run 'CodeIntel|RegisterProduction'`.

## Tiêu chí hoàn thành

- [x] Gateway khởi động với `CODE_INTEL_SERVICE_ADDR` rỗng; không dial, không panic; log cảnh báo.
- [x] Có địa chỉ: dial, health `code-intel-service` xuất hiện ở `/readyz`.
- [x] `MaxCallRecvMsgSize` 4 MiB và token nội bộ có test.
- [x] `ChannelDeps` rỗng hợp lệ.
- [x] Không thêm `max-lines` disable.

## Rủi ro và lưu ý

- Gán con trỏ nil kiểu cụ thể vào interface làm `client == nil` sai: chỉ gán sau khi dial thành công.
- Tên biến token chưa chốt (SOL-040-foundation Q2); đặt hằng `codeIntelInternalTokenEnv` một chỗ.
- Stub `codeintelv1` chưa tồn tại tới cổng G0; không tự viết stub tay.
