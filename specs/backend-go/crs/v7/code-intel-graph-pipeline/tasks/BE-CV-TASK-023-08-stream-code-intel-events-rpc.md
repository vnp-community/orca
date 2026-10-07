# BE-CV-TASK-023-08: RPC `StreamCodeIntelEvents` (proto, use case, limiter 16, handler có tenant)

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P0
**Service:** `infra-fleet-service` · `proto`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (sửa: `rpc` cạnh `StreamFileChanges` dòng 114; `message` cạnh dòng ~879–898), stub sinh, `.../internal/usecase/stream_code_intel_events.go` (mới), `.../internal/usecase/code_intel_stream_limiter.go` (mới), `.../internal/adapter/grpc/server_code_intel.go` (sửa), `.../cmd/server/main.go` (sửa), test
**Depends on:** TASK-023-07, TASK-023-06 (để có `WithCodeIntel`)
**Status:** [x] DONE

---

## Context

Mẫu `StreamFileChanges` (`infrafleet.proto:114`, handler `server.go:689–714`, use case `stream_file_changes.go`), **nhưng** handler của nó không gắn tenant từ metadata stream (các stream khác như `AttachPty` thì có; `grpcmw.ChainUnary` chỉ gắn cho RPC đơn, `main.go:679`). Handler mới phải gọi `withTenantFromStreamMetadata` ngay đầu. PQ-18: 16 luồng/(tenant, dev server).

## Việc cần làm

1. Proto: `StreamCodeIntelEventsRequest{dev_server_id=1}`, `CodeIntelEvent` (20 field nguyên văn hợp đồng §2.3), `rpc StreamCodeIntelEvents(...) returns (stream CodeIntelEvent)`. `buf generate`.
2. `usecase/code_intel_stream_limiter.go`: `CodeIntelStreamLimiter` (khuôn `ConnectionStreamLimiter`: `Acquire(key) (release func(), err)`), khoá `tenantID|devServerID`, mặc định 16, cấu hình bởi biến `INFRA_CODEINTEL_MAX_STREAMS` (đề xuất; SOL-023 Q2). Lỗi: `apperrors.New(KindFailedPrecondition, "INFRA_CODEINTEL_STREAM_LIMIT", …)`.
3. `usecase/stream_code_intel_events.go`: `Execute(ctx, devServerID) (events <-chan domain.CodeIntelEvent, release func(), err)`: `RequireTenantID` → validate id → `DevServerRepository.Get` (`INFRA_DEV_SERVER_NOT_FOUND`) → `Acquire` → `CodeIntelEventSource.SubscribeCodeIntelEvents`; `release` gọi cả unsubscribe và limiter. **Không** yêu cầu agent đang kết nối.
4. Handler trong `server_code_intel.go`: `ctx := withTenantFromStreamMetadata(stream.Context())`; gọi `Execute(ctx, …)`; `defer release()`; vòng `for ev := range events` gửi `toProtoCodeIntelEvent`; kết thúc khi `stream.Context().Done()`. Ánh xạ `Percent *int` → `optional int32`, `received_at` → `timestamppb`.
5. `main.go`: dựng limiter và use case với `agentClient` làm `CodeIntelEventSource` (`*devserveragent.Client` cài `SubscribeCodeIntelEvents` từ TASK-023-07), truyền qua `WithCodeIntel`.
6. Không thêm `StreamGuard` (SOL-023 C8); ghi chú trong README service nếu có.

## Kiểm thử

- `buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'`.
- `cd backend-go/services/infra-fleet-service && go test ./internal/usecase/ ./internal/adapter/grpc/ -run 'StreamCodeIntelEvents|CodeIntelStreamLimiter' -race`.
- Ca: tenant A xin dev server của B → `NotFound`; không có metadata tenant → `Unauthenticated`; luồng thứ 17 cùng `(tenant, dev server)` bị từ chối, sau khi đóng một luồng mở lại được; huỷ ctx giải phóng limiter và unsubscribe (đếm `len(codeIntelSubs)`); `percent` vắng trên dây ↔ `HasPercent()==false` ở client; dev server chưa có phiên (direct-websocket) vẫn đăng ký được rồi nhận `resync` khi agent vào.
- Rò goroutine: đếm `runtime.NumGoroutine` trước/sau 100 vòng mở–đóng (ngưỡng dung sai nhỏ).

## Tiêu chí hoàn thành

- [x] Handler gắn tenant từ metadata stream và có test chứng minh.
- [x] Giới hạn 16 luồng/(tenant, dev server) có test.
- [x] Không phụ thuộc agent đang online khi đăng ký.
- [x] `buf breaking` xanh.

## Rủi ro và lưu ý

- Mỗi luồng giữ một goroutine; N replica code-intel-service × M dev server (SOL-024) chưa đo.
- Tên `StreamCodeIntelEvents*` trùng với `orca.codeintel.v1`: ở mã Go của code-intel-service dùng alias import (hợp đồng §2.2).
