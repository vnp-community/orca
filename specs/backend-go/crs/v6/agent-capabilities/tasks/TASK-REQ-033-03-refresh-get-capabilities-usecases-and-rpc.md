# TASK-REQ-033-03: Use case `RefreshDevServerCapabilities`, `GetDevServerCapabilities`, RPC proto, kích hoạt sau handshake

**From Solution:** [BE-REQ-SOL-033](../solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md) mục 2.B, 2.E, 2.G, 2.J
**Priority:** P1
**Service/Area:** `infra-fleet-service` / proto, usecase, adapter grpc, config, wiring
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (sửa), `internal/usecase/refresh_dev_server_capabilities.go` (mới), `internal/usecase/get_dev_server_capabilities.go` (mới), `internal/adapter/grpc/server_capability.go` (mới), `internal/adapter/grpc/server.go` (sửa), `internal/adapter/devserveragent/client.go` (sửa, option `WithOnSessionAttached`), `internal/config/config.go` (sửa), `cmd/server/main.go` (sửa), và `_test.go`
**Depends on:** TASK-REQ-033-01 (store, domain), TASK-REQ-033-02 (`HandshakeInfo` mới)
**Status:** `[x] DONE`

---

## Context

Đã đọc ngày 2026-10-06:
- Mẫu use case "gọi method agent, method chưa có thì suy giảm": `internal/usecase/get_host_capabilities.go` (`agent.Exec(ctx, devServer, "host.capabilities", nil)`, `errors.Is(execErr, domain.ErrAgentMethodNotFound)`; dịch lỗi bằng `apperrors.New(kind, code, msg, cause)`). Lỗi tenant: `tenant.RequireTenantID(ctx)` trả `KindUnauthenticated`.
- `DevServerAgentClient` (`ports.go:411`): `Exec(ctx, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error)`, `IsConnected(devServerID) bool` (đọc map, không dial), `LastHandshakeInfo(devServerID)`.
- `ConnectionResolver` (`ports.go:296`): `ResolveConnection(ctx, tenantID, connectionID) (connected, devServer, conn, err)` và `ResolveConnectionByDevServer`. Dùng để quy `connection_id` thành `devServer`.
- `grpc.New(...)` ở `internal/adapter/grpc/server.go:156` nhận hơn 40 tham số vị trí; handler mẫu ở `server_emulator_host.go:107` (`GetHostCapabilities`). Thêm một use case là thêm một tham số và một trường struct; mọi nơi gọi `New` (main.go và `server_test.go`) phải sửa.
- `cmd/server/main.go` dòng 316 đến 333: `agentOpts` là slice `[]infradevserveragent.Option`; thêm option mới vào đó trước `infradevserveragent.New`. `main.go:451 getHostCapabilitiesUC := usecase.NewGetHostCapabilities(repo, agentClient)` là chỗ tham chiếu để thêm use case mới.
- Outbox của service: `infra.outbox_events` qua `EnqueueOutboxEvent` (`adapter/postgres/outbox.go`), ghi trực tiếp không cùng giao dịch nghiệp vụ; relay `common/outbox.NewRelay` đã chạy ở `main.go`. Subject mẫu `orca.infrafleet.dev_server.disconnected`; tên chuẩn của CR: `orca.infra.dev_server.capabilities_changed` (chênh `infra` và `infrafleet`: xem Rủi ro).
- Phạm vi quyền: không cần `requireAdmin`; chỉ tenant (hồ sơ không có bí mật).

## Việc cần làm

1. Proto: thêm `GetDevServerCapabilitiesRequest`, `DevServerCapabilityProfile` và `rpc GetDevServerCapabilities` đúng như solution mục 2.B:
   - số trường 1..9 như đã nêu
   - dùng `google.protobuf.Timestamp` nếu file đã import (kiểm tra import có sẵn, nếu chưa thì thêm). Chạy `buf generate` theo `Makefile` (`make proto`) và commit mã sinh nếu repo commit mã sinh (kiểm `proto/gen`).
2. `RefreshDevServerCapabilities` (`NewRefreshDevServerCapabilities(resolver ConnectionResolver, devServers DevServerStore, agent DevServerAgentClient, store CapabilityProfileStore, events OutboxEnqueuer, clock Clock)`); `Execute(ctx, tenantID, devServerID string) (domain.CapabilityProfile, error)` theo thứ tự 1 đến 6 của mục 2.E. Hàm phụ `buildHandshakeOnlyProfile(info HandshakeInfo, devServer domain.DevServer, now time.Time) domain.CapabilityProfile`: `profile = {"schemaVersion":1,"agent":{"buildVersion":...,"protocolVersion":...},"host":{"platform":...,"arch":...,"nodeVersion":...},"handshakeCapabilities":[...]}` và `source=handshake_only`.
3. Chống bão: trường `inflight singleflight.Group` (hoặc `map[string]*sync.Mutex` nếu repo không dùng `x/sync`; kiểm `go.mod` của service), khoá theo `tenantID+devServerID`; `lastAttempt map[string]time.Time` + `sync.Mutex` để áp `INFRA_CAPABILITY_REFRESH_MIN_INTERVAL`, trừ khi `Force=true` (RPC `refresh=true`).
4. Sau `Upsert`: nếu `existed && previousFingerprint != p.Fingerprint` hoặc `!existed` thì `events.EnqueueOutboxEvent(ctx, id, tenantID, "orca.infra.dev_server.capabilities_changed", now, 1, payload)`; payload JSON `{"dev_server_id","fingerprint","source","features"}`, không có `profile`.
5. `GetDevServerCapabilities` (`NewGetDevServerCapabilities(resolver, store, refresh *RefreshDevServerCapabilities, agent DevServerAgentClient, ttl time.Duration, clock Clock)`):
   - `Execute(ctx, in GetCapabilitiesInput{ConnectionID, DevServerID string; Refresh bool}) (GetCapabilitiesResult{Profile domain.CapabilityProfile; Connected bool; Found bool}, error)`: kiểm đúng một id (`INFRA_CAPABILITY_BAD_REQUEST`)
   - quy ra `devServerID`
   - `connected := agent.IsConnected(devServerID)`
   - nếu `connected` và (`Refresh` hoặc chưa có hồ sơ hoặc quá TTL) thì gọi Refresh, lỗi Refresh chỉ log và dùng hồ sơ cũ nếu có
   - không kết nối và chưa có hồ sơ thì `INFRA_CAPABILITY_PROFILE_NOT_FOUND`.
6. Handler `server_capability.go`: ánh xạ `result` sang `DevServerCapabilityProfile` (`profile_json` = `string(p.ProfileJSON)`, `degraded = p.Degraded()`, `connected`); dịch `apperrors` bằng hàm có sẵn của server (`apperrors.ToGRPCStatus`). Thêm vào `New(...)` và struct `Server`, sửa main.go và các test dùng `New`.
7. Kích hoạt: `devserveragent.WithOnSessionAttached(fn func(devServerID, tenantID string))` gọi ở cuối `attachTransport` (cả `AttachTransport` và `AttachInboundSession`); trong `main.go` đặt `fn` chạy `go func(){ ctx, cancel := context.WithTimeout(tenant.WithTenantID(context.Background(), tenantID), 10*time.Second); defer cancel(); _, _ = refreshUC.Execute(ctx, tenantID, devServerID) }()`. Cần biết `tenantID` của phiên: đọc cách `Client` ghi nhận tenant (nếu không có, tra từ `repo.GetDevServerByID` không cần tenant; ghi lựa chọn vào PR).
8. Config: `CapabilityProfileTTL time.Duration` từ `INFRA_CAPABILITY_PROFILE_TTL` (mặc định 24h), `CapabilityRefreshMinInterval` từ `INFRA_CAPABILITY_REFRESH_MIN_INTERVAL` (mặc định 5m); giá trị không parse được hoặc không dương thì rơi về mặc định (cùng kiểu `FleetPollInterval`).

## Kiểm thử

- Refresh: `TestRefresh_ProbeSuccess_UpsertsAndEmitsOnFirstProfile`, `_FingerprintUnchanged_NoEvent`, `_FingerprintChanged_OneEvent`, `_MethodNotFound_BuildsHandshakeOnly` (cờ `degraded`), `_AgentTimeout_KeepsOldProfile`, `_NotConnected_ReturnsStored`, `_ConcurrentCallsCoalesce` (8 goroutine, một `Exec`), `_MinIntervalSkips`, `_ForceBypassesMinInterval`.
- Get: `TestGet_ExactlyOneID`, `_TTLExpiredRefreshes`, `_FreshProfileNoRefresh`, `_NotConnectedNoProfileNotFound`, `_ConnectionIDResolves`, `_TenantFromContextOnly` (tenant khác không thấy).
- Handler: `TestServer_GetDevServerCapabilities_MapsFields` trong `server_test.go` kiểu hiện có.
- Wiring: `TestWithOnSessionAttached_CalledAfterHandshake_NotBlocking` (callback ngủ 2 giây, handshake vẫn xong trong dưới 1 giây).
- Hợp đồng: `cd backend-go/proto && buf lint && buf breaking --against '../../.git#branch=main'` chạy trực tiếp (không dùng `make proto-lint` vì `|| true` nuốt lỗi, xem CR-REQ-030 mục 1).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/infra-fleet-service/...` và bản `-tags=integration`.

## Tiêu chí hoàn thành

- [x] `GetDevServerCapabilities` trả `source=probe` với agent mới và `source=handshake_only`, `degraded=true` với agent cũ (agent giả trả -32601).
- [x] Sự kiện `capabilities_changed` chỉ phát khi fingerprint đổi hoặc hồ sơ mới.
- [x] Không có Refresh nào chặn handshake; tám cuộc gọi đồng thời tạo đúng một `Exec`.
- [x] `buf lint` và `buf breaking` xanh.
- [x] `grpc.New` và mọi test gọi nó biên dịch; `go vet ./services/infra-fleet-service/...` sạch.
- [x] Không biến môi trường hay giá trị bí mật nào xuất hiện trong log hay sự kiện (test quét chuỗi mẫu).

## Thứ tự thực hiện gợi ý

1. Proto và mã sinh trước (commit riêng, chạy `buf lint` + `buf breaking`), để các commit sau biên dịch được.
2. `RefreshDevServerCapabilities` với test dùng fake (chưa có RPC); sau đó `GetDevServerCapabilities`.
3. Handler gRPC và sửa `grpc.New(...)`/`main.go`/`server_test.go` trong **một** commit (vì đổi chữ ký là phá biên dịch ở nhiều nơi).
4. Cuối cùng option `WithOnSessionAttached` và wiring kích hoạt sau handshake; kiểm callback không chặn handshake.

## Kiểm tra thủ công sau khi xong

- `grpcurl` gọi `GetDevServerCapabilities` với `dev_server_id` của một dev server đang kết nối (agent cũ), kỳ vọng hồ sơ `handshake_only`.
- Ngắt kết nối dev server rồi gọi lại: kỳ vọng hồ sơ cũ kèm `connected=false`, không có lần dial mới (xem log).
- Gọi liên tiếp 10 lần `refresh=true` từ tài khoản thường: kỳ vọng chỉ một lần `agent.capabilities` được gửi (khoảng tối thiểu 5 phút).

## Rủi ro và lưu ý

- Subject: CR viết `orca.infra.dev_server.capabilities_changed`, nhưng subject có sẵn của service là `orca.infrafleet.dev_server.disconnected`. Chốt một tiền tố và ghi lại; nếu chọn `orca.infrafleet.` thì sửa solution, không sửa CR.
- `grpc.New` chỉ có tham số vị trí: sai thứ tự gây lỗi biên dịch khó đọc; thêm tham số ở cuối danh sách.
- Probe ngay khi vừa attach có thể gặp agent đang khởi động; chấp nhận lỗi và để lần gọi theo yêu cầu làm mới.
- `refresh=true` có thể bị lạm dụng thành bão probe; giới hạn theo `lastAttempt` cho caller không phải admin (chưa có kiểm admin ở v1: quyết định cuối thuộc người duyệt).
