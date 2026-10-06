# BE-CV-SOL-023: `infra-fleet-service` vận chuyển `codeintel.*`/`quality.*`: timeout, mã lỗi, thông báo agent, `StreamCodeIntelEvents`, `GetAgentCapabilities`

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Là cổng đồng bộ **G2** (hợp đồng §7.1): mở khoá SOL-021, SOL-024 và `BE-CV-SOL-080-*`. Chạm code **đang chạy** của `infra-fleet-service` (dùng chung với `git-gateway-service`, `workflow-service`, `api-gateway`), nên mọi thay đổi phải **không đổi hành vi** với method ngoài `codeintel.`/`quality.`.

**CR:** [CR-CV-023](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-023-infra-fleet-codeintel-transport.md)
**Service:** `infra-fleet-service` (`internal/{domain,usecase,adapter/grpc,adapter/agentwsserver,adapter/devserveragent}`, `cmd/server/main.go`) · `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**TDD tham chiếu:** [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) (§3 API surface, §7 dependencies "connectionId resolution + relay dispatch flow", §9 security), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) ("Talking to the Dev Server Agent": giữ giao thức dây hiện có, Option A), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (adapter là nơi duy nhất ánh xạ lỗi domain sang mã dây), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (timeout mọi lời gọi ra ngoài, backpressure stream "drop/slow-consumer policy"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) ("Multi-tenancy isolation")
**Hợp đồng:** `CONTRACT-codeintel-proto-and-data-map.md` (PQ-02, PQ-13, PQ-17, PQ-18; §2.2, §2.3, §3.3, §10), `CONTRACT-codeintel-agent-rpc.md` (§1.3, §2.5, §3.2–3.4, §6)

---

## 0. Hợp đồng áp dụng

| Phán quyết / mục | Áp dụng |
|---|---|
| PQ-02 (chặng 1–2) | Agent giữ `error.data.code`; infra-fleet đổi thành `apperrors` có tiền tố `CODEINTEL_X: message≤300`; `error.data` đi trailer `x-orca-agent-error-data-bin` (JSON ≤ 4 KiB, hợp lệ hoá bằng bỏ phần tử) |
| PQ-13 | Bảng timeout Go: `codeintel.*` trừ `status\|reindex\|reindexStatus\|reindexCancel\|watch` = **90 s**; `quality.listProfiles` = 45 s; `quality.*` khác = 30 s (mặc định); `ai.complete` = 120 s (dòng do SOL-093 thêm) |
| PQ-17 | Chuyển **4** thông báo: `codeintel.indexChanged`, `codeintel.reindexProgress`, `quality.progress`, `quality.finished`, đều có `workspaceRoot`; `CodeIntelEvent` 20 field, `percent` là `optional int32`, `payload_json` ≤ 64 KiB |
| PQ-18 | `StreamCodeIntelEvents` giới hạn **16** luồng mỗi `(tenant, dev server)`; `GetAgentCapabilities` mới; `inboundHandshakeParams` và `HandshakeInfo` thêm `Tools []string` |
| §2.2 (proto) | Nội dung RPC/message thêm vào `infrafleet.proto`; tenant gắn bằng `withTenantFromStreamMetadata` ngay đầu handler |
| §3.3 (agent) | `CODEINTEL_AGENT_UNSUPPORTED` do infra-fleet sinh khi agent trả `-32601` cho method `codeintel.*`/`quality.*` |
| §3.4 (agent) | Chỉ khi `method` bắt đầu `codeintel.` hoặc `quality.`; danh sách cho phép + ánh xạ `Kind` |
| §10 | Danh sách việc ở `client.go`, `session.go`, `server.go` (agentwsserver), `relay_by_dev_server.go` |
| §8.3 (4) | `StreamCodeIntelEvents`, `GetAgentCapabilities` luôn kiểm `tenant` + `DevServerRepository.Get(tenant, id)` (test cô lập tenant) |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (với số dòng): `internal/usecase/{relay_by_dev_server.go (đủ 64 dòng), relay.go, stream_file_changes.go, is_dev_server_connected.go, connection_stream_limiter.go, ports.go (dòng 74–100 `HandshakeInfo`, 405–430 `DevServerAgentClient`)}`; `internal/adapter/devserveragent/{client.go (dòng 176–260, 296–340, 395–500), session.go (dòng 30–70, 125–215, 240–262, 425–470, 740–870, 1150–1235), jsonrpc.go, agent_methods.go (dòng 85–119)}`; `internal/adapter/agentwsserver/server.go` (dòng 60–100, 195–235); `internal/adapter/grpc/server.go` (dòng 215–330, 650–720, 1380–1400, 1538–1565) và `server_ephemeral_vm.go` (đầu file); `internal/adapter/portevents/broadcaster.go`; `internal/domain/agent_relay.go`; `cmd/server/main.go` (grep `EnsureStream`, `StreamGuard`, `grpc.NewServer`, dòng 384–389, 679); `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (dòng 96–133, 866–905; 94 RPC, 1639 dòng); `common/{apperrors,grpcmw,internalcaller,eventbus,outbox}`.

Xác nhận đúng CR: (a) `execTimeoutForMethod` chỉ có ngoại lệ `agent.execPrompt` = 15 phút (`client.go` ~410–418), còn lại `cfg.RequestTimeout`; (b) `RelayByDevServer.Execute` và `Relay.Execute` đều bọc mọi lỗi `Exec` thành `INFRA_AGENT_EXEC_FAILED` (`relay_by_dev_server.go:59–62`, `relay.go` tương tự) và `apperrors.ToGRPCStatus` chỉ gửi `Code: Message`; (c) `routeNotification` chỉ có 5 nhánh (`pty.*`, `agent.hook`, `browser.screencast*`, `fs.changed`, `agent.execPrompt.output`), còn lại `default: return`; (d) `inboundHandshakeParams` (`server.go` 73–81) và `HandshakeInfo` (`session.go` 47–60) không có `Tools`; `usecase.HandshakeInfo` (`ports.go` 85–90) chỉ 4 trường; (e) `StreamFileChanges` handler (`server.go` 689–714) **không** gọi `withTenantFromStreamMetadata`, các stream khác thì có (`AttachPty` dòng ~1500); (f) với `direct-websocket`, `getInboundSession` trả lỗi "the agent must dial in first" nếu chưa có phiên (`client.go` ~228–240).

Đã kiểm thêm (đáp câu hỏi mở của CR): **`cmd/server/main.go` không dùng `internalcaller`/`StreamGuard`** (grep `StreamGuard|internalcaller` ra rỗng; `grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())` ở dòng 679 chỉ có interceptor unary). `ConnectionStreamLimiter` (`connection_stream_limiter.go`) khoá theo `connectionID` và thông điệp lỗi nêu "pty streams" nên **không tái dùng trực tiếp**.

### Correction relative to CR-CV-023 (Lệch giữa CR và hợp đồng)

| # | CR nói | Hợp đồng / mã thật | Xử lý |
|---|--------|--------------------|-------|
| C1 | Tối đa 4 luồng mỗi `(tenant, dev server)` | PQ-18: **16** (mỗi replica `code-intel-service` mở một luồng) | Hằng 16, cấu hình được |
| C2 | `CodeIntelEvent` 10 field, `percent int32`, `kind` 4 giá trị | Hợp đồng §2.3: 20 field, `optional int32 percent`, `kind` thêm `quality_progress`, `quality_finished` | Dùng nguyên văn hợp đồng |
| C3 | Chỉ chuyển `indexChanged`/`reindexProgress` | PQ-17: thêm `quality.progress`, `quality.finished`; `workspaceRoot` bắt buộc ở **mọi** thông báo | Bốn nhánh trong `routeNotification` |
| C4 | `codeintel.*` 90 s trừ `status`, `reindex` | PQ-13: trừ thêm `reindexStatus`, `reindexCancel`, `watch`; thêm `quality.listProfiles` 45 s | Bảng timeout theo method (2.C) |
| C5 | Chỉ `method` bắt đầu `codeintel.` | Hợp đồng §3.4: `codeintel.` **hoặc** `quality.`; danh sách mã thêm `PROFILE_UNKNOWN, ENV_NOT_READY, RUN_IN_PROGRESS, RUN_NOT_FOUND, RUN_CANCELLED`; `SYMBOL_NOT_FOUND, RUN_NOT_FOUND` → `KindNotFound`, `PROFILE_UNKNOWN` → `KindInvalidArgument` | Bảng mã ở 2.B |
| C6 | `Exec` trả `AgentRPCError` cho **mọi** `*JSONRPCError` | Hợp đồng §3.4: chỉ khi `method` thuộc hai nhóm trên | Giữ nguyên hành vi mọi method khác (giảm blast radius) |
| C7 | `HandshakeInfo` thêm `Tools`; `usecase.HandshakeInfo` thêm `Capabilities`, `Tools` | Đúng; `SessionID` cũng cần cho `GetAgentCapabilities` (`session.go` `HandshakeInfo.SessionID` có sẵn) | Thêm cả `SessionID` |
| C8 | Chưa chắc `StreamGuard` đã dùng (Q3) | Đã kiểm: **không** | Không thêm guard ở solution này; ghi vào rủi ro (dựa vào NetworkPolicy như RPC hiện có; token do CR-CV-013 của code-intel-service thi hành ở phía nó) |
| C9 | `quality.*` timeout không nêu | PQ-13: `listProfiles` 45 s, còn lại 30 s mặc định; `ai.complete` 120 s thuộc SOL-093 | Bảng dữ liệu, thêm `ai.complete` do SOL-093 |
| C10 | `payload orca.infra.agent.statusChanged` thêm `worktree_id`, `dev_server_id` (hợp đồng §2.2) | Thuộc CR-CV-080 | **Ngoài** solution này (`BE-CV-SOL-080-auto-refresh-index`) |

## 2. Giải pháp

### A. Cây file

```
backend-go/proto/orca/infrafleet/v1/infrafleet.proto                      (sửa: 2 RPC + 4 message, mục E)
backend-go/services/infra-fleet-service/internal/
  domain/agent_rpc_error.go                  (mới)  AgentRPCError
  domain/codeintel_event.go                  (mới)  CodeIntelEvent, các hằng Kind
  usecase/agent_rpc_error_mapping.go         (mới)  MapAgentExecError, AgentErrorDataForTrailer
  usecase/get_agent_capabilities.go          (mới)
  usecase/stream_code_intel_events.go        (mới)
  usecase/code_intel_stream_limiter.go       (mới)  tối đa 16 / (tenant, dev server)
  usecase/ports.go                           (sửa)  HandshakeInfo +SessionID,Capabilities,Tools; cổng CodeIntelEventSource
  usecase/relay_by_dev_server.go, relay.go   (sửa)  gọi MapAgentExecError
  adapter/devserveragent/exec_timeouts.go    (mới)  bảng timeout theo method
  adapter/devserveragent/client.go           (sửa)  execTimeoutForMethod -> bảng; Exec trả AgentRPCError; LastHandshakeInfo
  adapter/devserveragent/codeintel_events.go (mới)  Client.SubscribeCodeIntelEvents, sổ đăng ký, resync/overflow
  adapter/devserveragent/codeintel_notification_decoding.go (mới) giải mã 4 thông báo -> domain.CodeIntelEvent
  adapter/devserveragent/session.go          (sửa)  devServerID, onCodeIntel, onAttached, routeNotification, HandshakeInfo.Tools
  adapter/agentwsserver/server.go            (sửa)  inboundHandshakeParams.Tools -> HandshakeInfo.Tools
  adapter/grpc/server_code_intel.go          (mới)  handler GetAgentCapabilities, StreamCodeIntelEvents, WithCodeIntel
  adapter/grpc/server.go                     (sửa)  RelayByDevServer + Relay: SetTrailer
  cmd/server/main.go                         (sửa)  dựng use case, WithCodeIntel
```

`server_code_intel.go` là file riêng theo tiền lệ `server_ephemeral_vm.go` ("split out of server.go per this package's growing size") để `server.go` (đã dài) không phình thêm; `NewServer` có ~60 tham số vị trí nên **không** thêm tham số, dùng setter `func (s *Server) WithCodeIntel(streamUC *usecase.StreamCodeIntelEvents, capsUC *usecase.GetAgentCapabilities) *Server` (không phá lời gọi trong test).

### B. Giữ mã lỗi agent (PQ-02, hợp đồng agent §3.4)

```go
// domain/agent_rpc_error.go
type AgentRPCError struct { Code int; Message string; Data json.RawMessage }
func (e *AgentRPCError) Error() string { return e.Message }
func (e *AgentRPCError) Unwrap() error { if e.Code == -32601 { return ErrAgentMethodNotFound }; return nil }
```

`Client.Exec` (`client.go`): **trước** nhánh `-32601` hiện có, nếu `strings.HasPrefix(method, "codeintel.") || strings.HasPrefix(method, "quality.")` và `errors.As(err, &*JSONRPCError)` thì trả `&domain.AgentRPCError{Code, Message, Data}` (domain không import adapter). Mọi method khác không đổi.

`usecase.MapAgentExecError(method string, err error) error` (dùng chung `RelayByDevServer.Execute` và `Relay.Execute`; chỉ thay nhánh `INFRA_AGENT_EXEC_FAILED`):

1. `method` không thuộc hai nhóm → giữ `apperrors.New(KindInternal, "INFRA_AGENT_EXEC_FAILED", …, err)` (hành vi cũ).
2. `errors.Is(err, domain.ErrAgentMethodNotFound)` → `KindFailedPrecondition`, `CODEINTEL_AGENT_UNSUPPORTED`.
3. `errors.Is(err, context.DeadlineExceeded)` → `KindDeadlineExceeded`, `CODEINTEL_TIMEOUT`.
4. `errors.As(err, &*AgentRPCError)`: đọc `Data` thành `{code string}`; nếu `code` ∈ danh sách cho phép thì `apperrors.New(kind, code, cắtRune(Message, 300), err)` (`err` được giữ làm cause để handler lấy `Data`), ngược lại bước 1. Loại ký tự điều khiển khỏi `Message`.

| `data.code` | `apperrors.Kind` (hợp đồng §3.4) |
|---|---|
| `TOOL_UNAVAILABLE, INDEX_MISSING, REPO_NOT_REGISTERED, REINDEX_IN_PROGRESS, OUTPUT_TOO_LARGE, AGENT_UNSUPPORTED, ENV_NOT_READY, RUN_IN_PROGRESS, RUN_CANCELLED` | `KindFailedPrecondition` |
| `PATH_NOT_ALLOWED` | `KindPermissionDenied` |
| `INVALID_PARAMS, AMBIGUOUS_SYMBOL, PROFILE_UNKNOWN` | `KindInvalidArgument` |
| `SYMBOL_NOT_FOUND, RUN_NOT_FOUND` | `KindNotFound` |
| `TIMEOUT` | `KindDeadlineExceeded` |
| `TOOL_FAILED` | `KindInternal` |

(Mọi mã có tiền tố `CODEINTEL_`.) `common/apperrors` hiện có đủ các `Kind` trên (đã đọc `apperrors.go`: `KindNotFound … KindDeadlineExceeded`); **không** cần `KindResourceExhausted/KindUnavailable` (CR-CV-010 thêm sau, ở collector).

Trailer: trong handler `RelayByDevServer` và `Relay` (`server.go`), khi lỗi trả về chứa `*domain.AgentRPCError` có `Data`: `grpc.SetTrailer(ctx, metadata.Pairs("x-orca-agent-error-data-bin", string(AgentErrorDataForTrailer(data, 4096))))`. `AgentErrorDataForTrailer`: nếu ≤ 4096 byte trả nguyên; ngược lại **bỏ phần tử** (mảng lớn nhất bỏ phần tử cuối trước, rồi các khoá không phải `code`), không bao giờ cắt giữa chuỗi; kết quả luôn là JSON hợp lệ chứa `code`. `-bin` để gRPC tự mã hoá base64. `RelayStream` ngoài phạm vi.

### C. Timeout (PQ-13)

`adapter/devserveragent/exec_timeouts.go` (thay thân `execTimeoutForMethod`, `0` = dùng `cfg.RequestTimeout` như hiện nay):

```go
var execTimeoutOverrides = map[string]time.Duration{
    "agent.execPrompt":     15 * time.Minute, // giữ nguyên
    "quality.listProfiles": 45 * time.Second,
    // "ai.complete": 120 * time.Second  -> thêm bởi BE-CV-SOL-093
}
const codeIntelReadTimeout = 90 * time.Second
func execTimeoutForMethod(method string) time.Duration {
    if d, ok := execTimeoutOverrides[method]; ok { return d }
    if strings.HasPrefix(method, "codeintel.") {
        switch method {
        case "codeintel.status", "codeintel.reindex", "codeintel.reindexStatus", "codeintel.reindexCancel", "codeintel.watch":
            return 0
        }
        return codeIntelReadTimeout
    }
    return 0 // quality.run|runStatus|cancel|results|coverage: 30 s mặc định
}
```
Agent luôn tự hết hạn trước (25 s; `detectChanges`/`structuralFacts` 55 s). Khi Go cắt, lệnh vẫn có thể chạy tiếp trên agent vì `callWithTimeout` chỉ `dropPending` (đã đọc `session.go` ~1195–1226); đó là hạn chế đã biết, không thuộc phạm vi.

### D. Thông báo agent → `StreamCodeIntelEvents`

- **Sổ đăng ký ở `Client`**, không ở `session`: `Client.codeIntelSubs map[devServerID][]*codeIntelSub` với mutex riêng. Lý do: phải đăng ký được khi agent chưa từng kết nối (với `direct-websocket`, `getInboundSession` lỗi nếu chưa có phiên; `StreamFileChanges`/`StreamAgentHooks` đều đi qua `getOrCreateSession` nên không dùng được).
- `session` thêm `devServerID string`, `onCodeIntel func(devServerID string, n JSONRPCNotification)`, `onAttached func(devServerID string)`; `Client` gắn tại **cả bốn** nơi gọi `newSession` (`client.go` ~211, ~267, ~305, ~327: `getOrDialSession`, `getOrProvisionSession`, `AttachTransport`, `AttachInboundSession`).
- `routeNotification` thêm `case "codeintel.indexChanged", "codeintel.reindexProgress", "quality.progress", "quality.finished": s.routeCodeIntelNotification(n)`. Giải mã (`codeintel_notification_decoding.go`): thiếu/hỏng `workspaceRoot` (hoặc `jobId` với `reindexProgress`, `runId` với `quality.*`) → bỏ, log mức debug (không log `params`). Ánh xạ (hợp đồng agent §6): `indexChanged` → `kind=index_changed` + `tool, commit, indexedAt→indexed_at, reason, headCommit→head_commit, stale, indexScope→index_scope, mergeBase→merge_base, trigger`; `reindexProgress` → `kind=reindex_progress` + `job_id, tool, stage, percent (null→không đặt), message, outcome, error_code`; `quality.progress` → `kind=quality_progress` + `run_id, stage, percent, message`; `quality.finished` → `kind=quality_finished` + `run_id, head_commit, error_code`. `payload_json` = `params` gốc nếu ≤ 64 KiB, ngược lại rỗng (consumer gọi `runStatus`/`reindexStatus` bù). **`state` của `reindexProgress` chỉ có trong `payload_json`** (hợp đồng không có field `state`: xem mục Lệch ở báo cáo).
- **Không im lặng khi mất sự kiện** (CR mục 2.3): (1) `onAttached` (gọi cuối `attachTransport`) phát `kind=resync` (không `workspace_root`) tới mọi subscriber của dev server; (2) bộ đệm mỗi subscriber 64; gửi không chặn; khi đầy đặt cờ `dropped` và, sau khi consumer rút cạn, phát **đúng một** `kind=overflow` rồi xoá cờ. `resync`/`overflow` nghĩa là "huỷ cache của dev server này".
- Cổng hẹp ở `usecase/ports.go` (không thêm vào `DevServerAgentClient` để không phải sửa các fake ở `grpc/server_test.go:180,489`, `usecase/*_test.go`, `devserveragent/terraform_runner_test.go`):

```go
type CodeIntelEventSource interface {
    SubscribeCodeIntelEvents(devServerID string) (<-chan domain.CodeIntelEvent, func())
}
```

### E. Proto `infrafleet.proto` (additive)

Theo hợp đồng §2.2/§2.3 nguyên văn: `rpc StreamCodeIntelEvents(StreamCodeIntelEventsRequest) returns (stream CodeIntelEvent)` (mẫu `StreamFileChanges`, dòng 114), `message StreamCodeIntelEventsRequest { string dev_server_id = 1; }`, `message CodeIntelEvent` (20 field), `rpc GetAgentCapabilities(GetAgentCapabilitiesRequest) returns (GetAgentCapabilitiesResponse)`; `GetAgentCapabilitiesRequest{dev_server_id=1}`; `GetAgentCapabilitiesResponse{connected=1, platform=2, arch=3, node_version=4, agent_version=5, capabilities=6, tools=7, session_id=8}` (số theo thứ tự hợp đồng; chủ sở hữu CR chốt). Tên trùng có chủ ý với `orca.codeintel.v1.StreamCodeIntelEventsRequest` (khác package): mã Go đặt alias `fleetv1`/`codeintelv1` (hợp đồng §2.2).

### F. Công bố `tools[]`/capabilities

`inboundHandshakeParams` thêm `Tools []string \`json:"tools"\``; `agentwsserver` điền `HandshakeInfo.Tools`; `usecase.HandshakeInfo` thêm `SessionID, Capabilities, Tools`; `Client.LastHandshakeInfo` chuyển đủ. `relay-websocket` không có `tools` trong **kết quả** handshake nên `Tools` rỗng (D2: chỉ `direct-websocket` ở MVP). `GetAgentCapabilities`: `tenant.RequireTenantID` → `DevServerRepository.Get(tenant, id)` (`NotFound` → `INFRA_DEV_SERVER_NOT_FOUND`) → `LastHandshakeInfo` + `IsConnected`; `connected=false` thì các trường còn lại rỗng; không dial.

### G. `StreamCodeIntelEvents` (use case + handler)

`usecase.StreamCodeIntelEvents.Execute(ctx, devServerID)`: `RequireTenantID`; `DevServerRepository.Get`; `CodeIntelStreamLimiter.Acquire(tenant|devServerID)` (16; vượt → `KindFailedPrecondition`, mã `INFRA_CODEINTEL_STREAM_LIMIT`, đổi sang `KindResourceExhausted` khi CR-CV-010 thêm); `SubscribeCodeIntelEvents` **không** yêu cầu agent đang kết nối. Handler (`server_code_intel.go`) **bắt đầu** bằng `ctx := withTenantFromStreamMetadata(stream.Context())` (không bắt chước `StreamFileChanges`), `defer unsubscribe(); defer release()`, gửi `stream.Send(toProtoCodeIntelEvent(ev))`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Chỉ ánh xạ lỗi cho `codeintel.`/`quality.` và mã trong danh sách cho phép | Không đổi hành vi service khác; agent không chèn mã tuỳ ý |
| `AgentRPCError` chỉ cho hai nhóm method | Giảm blast radius so với CR (C6) |
| Trailer `-bin` thay vì `errdetails` | `errdetails` chưa dùng ở `backend-go`; `ToGRPCStatus` không mang dữ liệu có cấu trúc (PQ-02) |
| Cổng `CodeIntelEventSource` hẹp + `WithCodeIntel` | Không đụng ~60 tham số `NewServer` và 16+ fake |
| `resync`/`overflow` thay cho im lặng | Mất một `indexChanged` làm cache sai vô thời hạn (khác `pty.data`) |
| Bảng timeout dạng dữ liệu | `ai.complete` (SOL-093) chỉ thêm một dòng |
| Limiter riêng (16) | `ConnectionStreamLimiter` khoá theo `connectionID`, nghĩa "pty" |
| Giữ `StreamCodeIntelEvents` (gRPC), không dùng NATS | Tra tenant trong đường nóng của `readLoop` và cần outbox (CR mục 2.6; hợp đồng §5 "Đường không qua NATS") |

## 4. Phụ thuộc chéo khu vực

| Hướng | Solution | Quan hệ |
|---|---|---|
| Cùng khu vực, sau | `BE-CV-SOL-021-agent-collector` (đọc mã lỗi + trailer, dùng `GetAgentCapabilities`), `BE-CV-SOL-024-event-distribution` (đọc `StreamCodeIntelEvents`), `BE-CV-SOL-080-auto-refresh-index`, `BE-CV-SOL-093-ai-review-summary` (dòng `ai.complete`) | cổng G2 (§7.1) |
| Cùng khu vực, không phụ thuộc | `BE-CV-SOL-010`, `BE-CV-SOL-020` | song song (đợt 1, §7.3) |
| Agent | `AG-CV-SOL-001-codeintel-agent-foundation` (`error.data.code`, capability `codeintel`, `tools[]`), `AG-CV-SOL-004-reindex-and-index-notifications` (`indexChanged`, `reindexProgress`), `AG-CV-SOL-081-quality-runner-core` (`quality.progress/finished` có `workspaceRoot`) | tên method/tham số theo agent contract §6; đối chiếu bằng test với tệp vàng G1 |
| Frontend | — | không có |

## 5. Kiểm thử

- **Unit:** bảng ánh xạ mã lỗi (mỗi dòng một ca) + mã lạ → `INFRA_AGENT_EXEC_FAILED`; method ngoài hai nhóm không đổi; `AgentErrorDataForTrailer` (≤ 4 KiB, hợp lệ, giữ `code`, không cắt giữa chuỗi); `execTimeoutForMethod` (bảng đủ 12 ca: `codeintel.subgraph`=90 s, `status/reindex/reindexStatus/reindexCancel/watch`=0, `quality.listProfiles`=45 s, `quality.run`=0, `agent.execPrompt`=15 phút); giải mã bốn thông báo (hợp lệ, thiếu `workspaceRoot`, JSON hỏng, `percent:null`, `params` > 64 KiB).
- **`devserveragent` với `fakeAgent`** (mẫu `client_test.go:310` `TestClientExec_AgentExecPromptSurvivesLongerThanRequestTimeout`, `startFakeAgent`): timeout `codeintel.subgraph` vượt `RequestTimeout`, `codeintel.status` vẫn bị cắt; thông báo tới đúng subscriber; đăng ký trước khi agent kết nối rồi nhận `resync`; 200 thông báo không đọc → ≤ 64 + đúng một `overflow` và `readLoop` không bị chặn (`-race`).
- **gRPC in-process (`adapter/grpc/server_test.go`):** `RelayByDevServer` ra trailer + message `CODEINTEL_INDEX_MISSING: …`; `-32601` → `CODEINTEL_AGENT_UNSUPPORTED`; `StreamCodeIntelEvents` tenant A trên dev server của B → `NotFound`, thiếu metadata tenant → `Unauthenticated` (chứng minh gọi `withTenantFromStreamMetadata`), luồng thứ 17 bị từ chối; `GetAgentCapabilities` online/offline.
- **Hợp đồng:** `buf lint`, `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` gọi trực tiếp (không `make proto-lint`, có `|| true`).
- **Hồi quy:** `cd backend-go/services/infra-fleet-service && go test ./... -race` giữ nguyên kết quả cũ.
- **Hai dialect/cô lập tenant:** solution không thêm bảng; tenant được kiểm bằng test ở trên.
- Chưa chạy test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- Đổi kiểu lỗi `Exec` có thể phá chỗ `errors.As(…, *JSONRPCError)` ở ngoài adapter: **chưa grep hết** (TASK-023-01 làm đầu tiên); `Relay.Execute`, `ScanWorkspacePorts`, `EmulatorRelay`… dùng `Exec`.
- `grpc.SetTrailer` và khoá `-bin` chưa thử qua `otelgrpc`/interceptor thật ở phía client (SOL-021 kiểm).
- `StreamCodeIntelEvents` **không** có `StreamGuard`/token ở infra-fleet (đã kiểm: `main.go` không dùng `internalcaller`); bảo vệ dựa vào NetworkPolicy và tenant metadata như các RPC khác. Nếu sau này bật guard thì thêm `/orca.infrafleet.v1.InfraFleetService/StreamCodeIntelEvents` vào `StreamGuard`.
- Thông báo `codeintel.*` chỉ tới sau khi **backend gọi bất kỳ method nhóm này** (agent contract §6: "notifier hiện hành"); infra-fleet không ép điều đó: SOL-024 phải gọi `codeintel.watch`+`status` sau mỗi `resync`.
- Hạn chế `StreamFileChanges` (chưa chạy xác nhận lỗi tenant) chỉ được **suy** từ đọc code; thiết kế vẫn an toàn nếu hoá ra không lỗi.
- Một goroutine gRPC mỗi dev server mỗi replica (SOL-024) × tối đa 16 luồng/dev server: chưa đo; nếu lớn thì CR-024 Q2.
- `relay-ssh` (Part B) không có `tools/*` và chưa phát `codeintel.*` (CR-006, O-5): đơn giản không có sự kiện.
- Tần suất `reindexProgress` tối đa do agent giới hạn (≤ 1/giây/job, agent §6.2); infra-fleet không giới hạn thêm.

## 7. Câu hỏi mở

- **Q1.** `CodeIntelEvent` không có field `state` cho `reindexProgress` (chỉ có trong `payload_json`): có thêm `string state = 21` (additive) không? Đề nghị chủ hợp đồng (SOL-024 cần để biết job kết thúc).
- **Q2.** Biến môi trường cho trần 16 luồng (`INFRA_CODEINTEL_MAX_STREAMS`, đề xuất) chưa có trong hợp đồng §6.2.
- **Q3.** Có thêm `codeintel.indexChanged` vào audit/metrics của infra-fleet không (tên chốt ở CR-071)?
- **Q4.** Khi `KindResourceExhausted` có (CR-010), đổi mã giới hạn luồng sang `ResourceExhausted`.

## 8. Tham chiếu

- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/{relay_by_dev_server.go,relay.go,stream_file_changes.go,is_dev_server_connected.go,connection_stream_limiter.go,ports.go}`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/{client.go,session.go,jsonrpc.go,agent_methods.go,client_test.go}`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/grpc/{server.go,server_ephemeral_vm.go,server_test.go}`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/portevents/broadcaster.go` (mẫu broadcaster "drop on full")
- `/opt/repos/orca/backend-go/services/infra-fleet-service/cmd/server/main.go` (dòng 384–389, 679)
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `/opt/repos/orca/backend-go/common/apperrors/apperrors.go`, `common/grpcmw/grpcmw.go`, `common/internalcaller/internalcaller.go`
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-*.md` (PQ-02/13/17/18; agent §3.4, §6)
- `/opt/repos/orca/docs/crs/v7/code-intel-graph-pipeline/CR-CV-023-infra-fleet-codeintel-transport.md`
