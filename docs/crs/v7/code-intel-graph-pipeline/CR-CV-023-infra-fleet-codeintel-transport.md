# CR-CV-023 — `infra-fleet-service`: timeout `codeintel.*`, chuyển mã lỗi, nhận thông báo từ agent, `StreamCodeIntelEvents`, công bố `tools[]`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-023 |
| **Tên** | Thay đổi nhỏ ở `infra-fleet-service` để vận chuyển `codeintel.*`: timeout riêng, giữ mã lỗi agent, consumer cho thông báo `codeintel.indexChanged`/`codeintel.reindexProgress`, RPC `StreamCodeIntelEvents`, công bố `tools[]`/capabilities từ handshake |
| **Loại** | Feature (mở rộng lớp vận chuyển) |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | Hợp đồng thông báo/lỗi của CR-CV-001 và CR-CV-004 (agent); không phụ thuộc `code-intel-service` |
| **Mở khoá** | CR-CV-021 (mã lỗi, timeout, `GetAgentCapabilities`), CR-CV-024 (`StreamCodeIntelEvents`) |
| **Tác động** | `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `.../gen/go/orca/infrafleet/v1` (sinh), `infra-fleet-service/internal/{domain,usecase,adapter/grpc,adapter/agentwsserver,adapter/devserveragent}` |

---

## 1. Bối cảnh và vấn đề

Đã đọc code ngày 2026-10-05; phát hiện ở infra-fleet:

1. **Timeout.** `Client.Exec` gọi `sess.callWithTimeout(ctx, method, params, execTimeoutForMethod(method))` (`adapter/devserveragent/client.go:429`). `execTimeoutForMethod` chỉ có một ngoại lệ, `agent.execPrompt` = 15 phút (`client.go:403-418`); còn lại về `cfg.RequestTimeout` = 30 s (`config.go:57`, `session.go:1160-1170`). Truy vấn đồ thị nặng (nhiều lệnh CLI ~1,8 s mỗi lệnh, research 06 G6) dễ vượt 30 s.
2. **Mã lỗi agent bị mất.** `Exec` đã nhận diện `JSONRPCError` -32601 thành `domain.ErrAgentMethodNotFound` (`client.go:431-439`, `domain/agent_relay.go:21`), nhưng `RelayByDevServer.Execute` rồi gói **mọi** lỗi thành `KindInternal "INFRA_AGENT_EXEC_FAILED"` (`usecase/relay_by_dev_server.go:59-62`) và `apperrors.ToGRPCStatus` chỉ gửi `code: message`, nguyên nhân gốc chỉ log (`common/apperrors/apperrors.go:105-130`). `error.data.code` (ví dụ `CODEINTEL_INDEX_MISSING`, README v7 mục 3.3) và `error.data.candidates` không thể tới `code-intel-service`.
3. **Chiều agent → backend chỉ có bốn nhóm thông báo.** `session.routeNotification` (`session.go:434-466`) chỉ định tuyến `pty.data/exit/replay`, `agent.hook`, `browser.screencast*`, `fs.changed`, `agent.execPrompt.output`; mọi method khác rơi vào `default: return` (im lặng bỏ). `codeintel.indexChanged` hiện bị loại bỏ.
4. **`tools[]` không tới Go.** Agent gửi `tools: tools.map(t => t.name)` trong `agent.handshake` (`agent/src/relay/agent-session-handshake.ts:70`, tên `gitnexus`/`codegraph` được liệt kê chỉ khi binary tồn tại trong `PATH`, `agent-tool-registry.ts:185-206`), nhưng `agentwsserver.inboundHandshakeParams` (`adapter/agentwsserver/server.go:73-81`) **không có** trường `Tools`; `HandshakeInfo` (`devserveragent/session.go:47-53`) và `usecase.HandshakeInfo` (`usecase/ports.go:~85`) cũng không. `LastHandshakeInfo` (`client.go:473-493`) chỉ chuyển 4 trường. Vậy README v7 mục 1 ("Go lưu qua `LastHandshakeInfo`", research 01 mục 3) là **lệch code**: hiện không lưu `tools[]`.
5. **`StreamFileChanges` (mẫu) có bất thường về tenant.** Handler `adapter/grpc/server.go:689-693` gọi `s.streamFileChanges.Execute(stream.Context(), …)` trực tiếp, trong khi `Execute` yêu cầu `tenant.RequireTenantID` (`usecase/stream_file_changes.go:47-50`) và chỉ `ChainUnary` mới gắn tenant (`common/grpcmw/grpcmw.go:124-131`, `cmd/server/main.go:679`); các stream khác gọi `withTenantFromStreamMetadata` (`server.go:1393, 1456, 1500, 1545`, ghi chú "FLAGGED as a known gap" ở dòng ~1385-1392). Chưa chạy để xác nhận lỗi; nếu đúng, mẫu này không dùng trực tiếp được.
6. **Chưa có RPC công bố khả năng agent.** Tìm `HandshakeInfoFor`/`agent_version` trong `infrafleet.proto`: chỉ `ResolveConnectionResponse.node_version` (dòng ~578-582) và `DevServer.platform/arch/node_version/agent_version` (dòng ~508-515) lấy từ DB (cập nhật bởi `UpdateProvisionResult`, `usecase/ports.go:49`), không có `capabilities`/`tools` và không phản ánh phiên hiện tại.

## 2. Giải pháp đề xuất

### 2.1 Giữ mã lỗi agent cho `codeintel.*`

1. **domain (mới)** `internal/domain/agent_rpc_error.go`: `AgentRPCError{Code int; Message string; Data json.RawMessage}` với `Error()`/`Unwrap()` trả `ErrAgentMethodNotFound` khi `Code == -32601`.
2. **adapter** `devserveragent.Client.Exec`: nhánh `errors.As(err, &rpcErr)` hiện chỉ xử lý -32601 (`client.go:431-439`); thêm: mọi `*JSONRPCError` khác trả `&domain.AgentRPCError{Code, Message, Data}` thay vì giữ `*JSONRPCError` (domain không import adapter). Hành vi với các method khác không đổi về kiểu bên ngoài (chỉ đổi kiểu bọc); kiểm tra không còn nơi nào `errors.As(…, *JSONRPCError)` ngoài adapter (cần grep khi làm; chưa thực hiện).
3. **usecase** `RelayByDevServer.Execute` (và `Relay.Execute` để hai đường nhất quán): khi `strings.HasPrefix(in.Method, "codeintel.")` và `errors.As(err, *domain.AgentRPCError)`:
   - đọc `Data` thành `{code string}`; nếu `code` bắt đầu `CODEINTEL_` và nằm trong danh sách README v7 mục 3.3 (cộng `CODEINTEL_AGENT_UNSUPPORTED` do CR này sinh khi -32601): trả `apperrors.New(kind, code, cắt(Message, 300), err)` với `kind` theo bảng dưới;
   - ngược lại giữ hành vi cũ (`INFRA_AGENT_EXEC_FAILED`), tránh để agent chèn mã tuỳ ý.
   - Khi `errors.Is(err, domain.ErrAgentMethodNotFound)`: `KindFailedPrecondition`, mã `CODEINTEL_AGENT_UNSUPPORTED`.
   - Khi `errors.Is(err, context.DeadlineExceeded)` (timeout phía infra-fleet): `KindDeadlineExceeded`, `CODEINTEL_TIMEOUT`.

| `data.code` | `apperrors.Kind` |
|---|---|
| `CODEINTEL_TOOL_UNAVAILABLE`, `_INDEX_MISSING`, `_REPO_NOT_REGISTERED`, `_REINDEX_IN_PROGRESS`, `_OUTPUT_TOO_LARGE`, `CODEINTEL_AGENT_UNSUPPORTED` | `KindFailedPrecondition` |
| `_PATH_NOT_ALLOWED` | `KindPermissionDenied` |
| `_INVALID_PARAMS`, `_AMBIGUOUS_SYMBOL` | `KindInvalidArgument` |
| `_TIMEOUT` | `KindDeadlineExceeded` |
| `_TOOL_FAILED` | `KindInternal` |

4. **Dữ liệu có cấu trúc** (`candidates` của `AMBIGUOUS_SYMBOL`, cờ `retryable`): `ToGRPCStatus` không mang được. Trong handler `RelayByDevServer` (`server.go:660-680`), khi `errors.As(err, *domain.AgentRPCError)` và có `Data`: `grpc.SetTrailer(ctx, metadata.Pairs("x-orca-agent-error-data-bin", string(cắt(Data, 4096))))`. Khoá `-bin` để gRPC tự mã hoá base64. CR-CV-021 mục 2.3 đọc bằng `grpc.Trailer`. Lý do không dùng `status.WithDetails(errdetails.ErrorInfo)`: `errdetails` chưa dùng ở đâu trong `backend-go` (grep rỗng); trailer không cần phụ thuộc thêm. Tuỳ chọn khác (chưa chọn): đưa `data` vào message.
5. `RelayStream` ngoài phạm vi (collector chỉ dùng unary).

### 2.2 Timeout riêng cho `codeintel.*`

`execTimeoutForMethod` (`client.go:412`): thêm

```go
const codeIntelExecTimeout = 90 * time.Second   // ngân sách phía infra-fleet; agent phải hết hạn trước (CR-CV-001)
if strings.HasPrefix(method, "codeintel.") && method != "codeintel.status" && method != "codeintel.reindex" {
    return codeIntelExecTimeout
}
```

`codeintel.status` giữ mặc định 30 s (nhẹ, thăm dò HEAD của CR-CV-022 không được chờ lâu). `codeintel.reindex` giữ 30 s vì README v7 mục 3.2 mô tả là chạy nền, trả ngay `jobId` (tiến trình qua thông báo); nếu CR-CV-004 quyết định khác thì cập nhật dòng này. Hằng 90 s là đề xuất: lớn hơn timeout 60 s của `runToolCommand` mỗi lệnh (research 02 mục 2) và nhỏ hơn `CODEINTEL_COLLECT_TIMEOUT` 100 s của CR-CV-022; **agent phải tự hết hạn trước 90 s** để trả `CODEINTEL_TIMEOUT` có nghĩa thay vì để Go cắt (khi Go cắt, lệnh vẫn chạy tiếp trên agent vì không có cơ chế huỷ giữa chừng, `session.go:1195-1226`: chỉ `dropPending`).

Mở rộng test hiện có `TestClientExec_AgentExecPromptSurvivesLongerThanRequestTimeout` (`client_test.go:310`) bằng ca `codeintel.subgraph` tương tự và ca `codeintel.status` vẫn bị cắt ở `RequestTimeout`.

### 2.3 Consumer thông báo `codeintel.*` trong `devserveragent`

Quan sát cách nhận `pty.data`/`fs.changed`/`agent.hook`: `readLoop` → `routeNotification` (`session.go:434`) → hàm `route…` riêng giải mã `params`, copy danh sách subscriber dưới mutex riêng và **gửi không chặn** (bỏ khi đầy, `session.go:752-790`); người đăng ký gọi `subscribe…` **trước** khi gọi RPC để tránh mất thông báo (`session.go:839-851`). `agent.hook` là mẫu không khoá (`hookSubs`, `session.go:136-141`, `660-680`).

Thiết kế cho `codeintel.*` (mới):

- **Sổ đăng ký ở cấp `Client`, không ở cấp `session`.** Lý do: `StreamAgentHooks`/`StreamFileChanges` đều gọi `getOrCreateSession` (`agent_methods.go:101-118`, `client.go:724`) và với `direct-websocket` hàm này **lỗi nếu agent chưa từng kết nối** (`client.go:~228-240`: "the agent must dial in first"). Người đăng ký `code-intel-service` phải có khả năng đăng ký khi agent đang offline và nhận sự kiện khi nó vào. Do đó: `Client.codeIntelSubs map[devServerID][]chan rawCodeIntelEvent` (mutex riêng `codeIntelMu`), `Client.SubscribeCodeIntelEvents(devServerID)` không cần phiên.
- Mỗi `session` cần biết `devServerID` để định tuyến (hiện `newSession(host, cfg, logger)` không có, `session.go:198`): thêm field `devServerID string` và hook `onCodeIntel func(devServerID string, n JSONRPCNotification)` do `Client` gắn tại mọi nơi tạo/gắn session (`AttachInboundSession`, `AttachTransport`, `getOrCreateSession` các nhánh `connect`/`getOrProvisionSession`).
- `routeNotification`: thêm `case "codeintel.indexChanged", "codeintel.reindexProgress": s.routeCodeIntelNotification(n)`. Hàm này giải mã và gọi `onCodeIntel`. Cú pháp tham số theo README v7 mục 3.2: `indexChanged {workspaceRoot, tool, commit, indexedAt}`, `reindexProgress {jobId, stage, percent, message}`. Tham số hỏng/thiếu `workspaceRoot` (với `indexChanged`) hoặc `jobId` → bỏ, log mức debug.
- **Mất sự kiện khi rớt kết nối/đầy bộ đệm không được im lặng**: khác `pty.data`, mất một `indexChanged` làm cache sai vô thời hạn. Hai biện pháp:
  1. Khi `AttachInboundSession` (agent vào lại) hoặc `AttachTransport` gắn xong, `Client` phát sự kiện tổng hợp `Kind: resync` tới mọi subscriber của dev server đó (sự kiện trong lúc offline đã mất).
  2. Bộ đệm mỗi subscriber 64 (như `fs.changed`); nếu đầy thì **thay** bằng một sự kiện `Kind: overflow` duy nhất (đã đặt cờ; tương tự `FileChangeEvent.kind == "overflow"` mà `fs.changed` dùng, `session.go:~762-768`). `resync`/`overflow` đều có nghĩa "huỷ toàn bộ cache của dev server này".
- `domain/codeintel_event.go` (mới): `CodeIntelEvent{Kind, WorkspaceRoot, Tool, Commit, IndexedAt, JobID, Stage string; Percent int; Message string; ReceivedAt time.Time}` và cổng `usecase.DevServerAgentClient.SubscribeCodeIntelEvents(devServerID string) (<-chan CodeIntelEvent, func())`. Thêm vào interface `DevServerAgentClient` (`usecase/ports.go`) buộc mọi fake trong test (ví dụ `grpc/server_test.go:180,489`) phải cập nhật; thay vào đó **ưu tiên** một interface hẹp `CodeIntelEventSource` (tên cổng riêng, đúng tinh thần "narrow port" đã dùng ở `ports.go:~379-385`) để không đụng 16+ fake.

### 2.4 RPC `StreamCodeIntelEvents` (proto `orca.infrafleet.v1`)

Mẫu: `StreamFileChanges` (`infrafleet.proto:114`, message ở dòng 879-898, handler `server.go:689-714`, use case `usecase/stream_file_changes.go`).

```proto
// StreamCodeIntelEvents: server-streaming; kết thúc khi caller huỷ ctx.
rpc StreamCodeIntelEvents(StreamCodeIntelEventsRequest) returns (stream CodeIntelEvent);

message StreamCodeIntelEventsRequest {
  string dev_server_id = 1;   // bắt buộc; code-intel luôn đánh địa chỉ theo dev server (không có connection_id)
}
message CodeIntelEvent {
  string kind = 1;            // "index_changed" | "reindex_progress" | "resync" | "overflow"
  string workspace_root = 2;
  string tool = 3;            // "gitnexus" | "codegraph"
  string commit = 4;
  string indexed_at = 5;      // nguyên văn từ agent (ISO-8601)
  string job_id = 6;
  string stage = 7;
  int32 percent = 8;
  string message = 9;
  google.protobuf.Timestamp received_at = 10;  // thời điểm infra-fleet nhận, để chống đảo thứ tự ở CR-CV-024
}
```

- Use case `usecase/stream_code_intel_events.go` (mới): `tenant.RequireTenantID`; `DevServerRepository.Get(tenantID, devServerID)` để xác nhận quyền sở hữu (`NotFound` → `INFRA_DEV_SERVER_NOT_FOUND`); đăng ký qua `CodeIntelEventSource`; trả `(events, unsubscribe)`; **không** yêu cầu agent đang kết nối. Không gọi `fs.watch`-tương đương ở agent (agent chủ động gửi, README v7 D4); người dùng quyết định theo dõi file nằm ở CR-CV-004.
- Handler gRPC: **phải** bắt đầu bằng `ctx := withTenantFromStreamMetadata(stream.Context())` (`server.go:1545`) như `StreamExecOutput` (`server.go:1499-1500`); không bắt chước `StreamFileChanges` ở điểm này (mục 1 điểm 5).
- Phân quyền: `code-intel-service` là người gọi duy nhất; thêm `StreamCodeIntelEvents` vào `internalcaller.StreamGuard(token, …)` (`common/internalcaller/internalcaller.go:40`) nếu infra-fleet bật guard (chưa kiểm xem `main.go` có dùng `StreamGuard` không: `grep` ở `cmd/server/main.go:679` chỉ thấy `ChainUnary`+`StatsHandler`; ghi vào mục 7).
- Chặn đồng thời: mỗi `(tenant, dev server)` tối đa 4 luồng (tránh rò goroutine); `usecase/connection_stream_limiter.go` đã tồn tại với vai trò tương tự, kiểm tra tái dùng khi làm (chưa đọc kỹ).
- Sinh mã: `buf generate`; `buf breaking` xanh vì chỉ thêm RPC/message.

### 2.5 Công bố `tools[]` và capabilities

1. `agentwsserver.inboundHandshakeParams` thêm `Tools []string \`json:"tools"\``.
2. `devserveragent.HandshakeInfo` (`session.go:47`) thêm `Tools []string`; `agentwsserver/server.go:~216` điền `Tools: params.Tools`. `relay-websocket` (`runInitiatorHandshake`, `session.go:267-329`) nhận kết quả handshake từ agent; agent không gửi `tools` trong **kết quả** handshake (chỉ trong yêu cầu của chế độ direct, `agent-session-handshake.ts:57-71`), nên chế độ đó để `Tools` rỗng. Chấp nhận vì D2 (README v7) chỉ hỗ trợ `direct-websocket`.
3. `usecase.HandshakeInfo` (`usecase/ports.go:~85-90`) thêm `Capabilities []string`, `Tools []string`; sửa chuyển đổi trong `Client.LastHandshakeInfo` (`client.go:481-493`). Các điểm tiêu thụ `HandshakeInfo` hiện có (`UpdateProvisionResult`, `establish_connection.go:123`) bỏ qua trường mới; không đổi DB.
4. RPC mới **`GetAgentCapabilities(GetAgentCapabilitiesRequest{dev_server_id}) returns (GetAgentCapabilitiesResponse{connected, platform, arch, node_version, agent_version, capabilities[], tools[], session_id})`**: tenant-checked bằng `DevServerRepository.Get`; dữ liệu từ `LastHandshakeInfo` (bộ nhớ, không gọi agent, không dial); `connected=false` thì các trường còn lại rỗng. Dùng `usecase` mới `get_agent_capabilities.go`, kiểu như `IsDevServerConnected`. Lý do không mở rộng `IsDevServerConnected` hay `ResolveConnectionResponse`: đổi message đang dùng rộng rãi; RPC mới thuần cộng thêm.
5. `code-intel-service` dùng: `tools` không chứa `gitnexus` **và** `codegraph` → trả `CODEINTEL_TOOL_UNAVAILABLE` ngay, không gọi agent; chứa một trong hai → gọi `codeintel.status` để biết chỉ mục (`tools[]` chỉ nói *binary có trong PATH*, không nói chỉ mục tồn tại — `INDEX_MISSING` do agent trả).
6. `platform` (đã có) dùng cho `NormalizeRepoPath` (CR-CV-020 mục 2.6).

### 2.6 Đánh giá: event bus dùng chung có thay được `StreamCodeIntelEvents` không?

Đã đọc `adapter/eventbus/publisher.go` (stream `INFRA`, subject `orca.infra.terminal_session.*`, mô tả "best-effort, not outbox-backed"), `cmd/server/main.go:254-272` (stream `INFRAFLEET` `orca.infrafleet.>` + `outbox.Relay`, hiện dùng cho `orca.infrafleet.terminal.closed`) và `common/eventbus` (`Publisher`, `Consumer.Subscribe` bền/`SubscribeEphemeral`).

| Tiêu chí | NATS (infra-fleet publish, code-intel subscribe) | `StreamCodeIntelEvents` (gRPC) |
|---|---|---|
| Biết `tenant_id` khi nhận thông báo | **Không**: `session` chỉ biết `devServerID` (không có tenant trong `session.go`/`Client`); phải tra DB bằng truy vấn không theo tenant (kiểu `ListAllForPolling`, `usecase/ports.go:50-53`) trong đường nóng của `readLoop` | Có: người gọi gửi tenant trong metadata, `DevServerRepository.Get(tenant, id)` xác nhận |
| Độ bền khi `code-intel-service` tắt | Tốt nếu consumer bền (`Subscribe`, `eventbus.go:128`) | Mất (không bộ đệm); bù bằng `resync` khi nối lại (2.3, CR-CV-024) |
| Ghi DB infra-fleet cho mỗi thông báo (outbox) | Cần nếu muốn không mất; thêm tải và migration hai dialect cho thông báo tạm | Không |
| Số kết nối | Một subscription NATS toàn service | Một luồng gRPC mỗi dev server có binding |
| Nhất quán với mẫu hiện có | `INFRA`/`INFRAFLEET` đã có | `StreamFileChanges`, `StreamExecOutput`, `StreamPortForwardEvents` |
| Khớp README v7 (D4, mục 3.6) | Cần sửa README | Khớp |

**Kết luận:** giữ `StreamCodeIntelEvents` (khớp README v7 và tenant-an-toàn); event bus **có** dùng chung được về kỹ thuật nhưng buộc phải tra tenant trong đường nóng và thêm outbox, không đáng cho thông báo "làm mới cache" vốn tự phục hồi bằng `resync`. Xem lại nếu số dev server > vài trăm (mỗi luồng gRPC một goroutine) — Q2.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Sổ đăng ký ở `Client`, không ở `session` | Phải đăng ký được khi agent offline; session `direct-websocket` chỉ tồn tại sau lần kết nối đầu |
| `resync`/`overflow` thay cho im lặng | Mất một `indexChanged` làm cache sai mãi; khác `pty.data` |
| Chỉ ánh xạ mã lỗi cho `method` bắt đầu `codeintel.` và `data.code` thuộc danh sách | Không đổi hành vi các service khác; không để agent chèn mã tuỳ ý |
| Trailer `x-orca-agent-error-data-bin` | Không thêm phụ thuộc `errdetails`; `ToGRPCStatus` không mang dữ liệu có cấu trúc |
| `GetAgentCapabilities` là RPC mới | Cộng thêm, không đổi message dùng rộng |
| Timeout 90 s trừ `status`/`reindex` | `status` phải nhẹ; `reindex` chạy nền |
| Interface hẹp `CodeIntelEventSource` | Không buộc 16+ fake của `DevServerAgentClient` đổi |

## 4. Tiêu chí chấp nhận

- [ ] `execTimeoutForMethod("codeintel.subgraph") == 90s`; `("codeintel.status") == 0`; `("codeintel.reindex") == 0`; `("agent.execPrompt")` không đổi (test bảng).
- [ ] Agent giả trả JSON-RPC error `{code:-32000, data:{code:"CODEINTEL_INDEX_MISSING"}}` cho `codeintel.overview`: gRPC `RelayByDevServer` trả `FailedPrecondition` với message bắt đầu `CODEINTEL_INDEX_MISSING:`; cho method không phải `codeintel.*` vẫn ra `INFRA_AGENT_EXEC_FAILED` như hiện tại.
- [ ] `CODEINTEL_AMBIGUOUS_SYMBOL` kèm `candidates`: trailer `x-orca-agent-error-data-bin` chứa JSON ≤ 4 KiB.
- [ ] Agent -32601 cho `codeintel.status`: `FailedPrecondition` `CODEINTEL_AGENT_UNSUPPORTED`.
- [ ] Agent giả gửi `codeintel.indexChanged {workspaceRoot, tool, commit, indexedAt}` → subscriber của dev server đó nhận đúng một `CodeIntelEvent{kind:"index_changed"}`; subscriber của dev server khác không nhận.
- [ ] Đăng ký **trước khi** agent kết nối (direct-websocket) thành công; khi agent vào, subscriber nhận `resync`.
- [ ] Gửi 200 thông báo không đọc: subscriber nhận một `overflow` cuối cùng, `readLoop` không bị chặn (race detector `-race`).
- [ ] `StreamCodeIntelEvents` với tenant A cho dev server của tenant B trả `NotFound`; thiếu tenant metadata trả `Unauthenticated` (chứng minh gọi `withTenantFromStreamMetadata`).
- [ ] Handshake có `tools:["gitnexus","codegraph","git"]` → `GetAgentCapabilities` trả đúng `tools`, `capabilities`, `platform`; dev server offline → `connected=false`.
- [ ] `buf lint`, `buf breaking` xanh; test hiện có của infra-fleet (`go test ./...`) không đổi kết quả; không có tên file `helpers/utils/common/misc`; không `max-lines` disable mới.

## 5. Kiểm thử

- **Unit:** bảng ánh xạ mã lỗi; `execTimeoutForMethod`; giải mã tham số thông báo (hợp lệ, thiếu trường, JSON hỏng); `GetAgentCapabilities` với `LastHandshakeInfo` giả.
- **`devserveragent` với `fakeAgent`** (`client_test.go:310` có sẵn mẫu `startFakeAgent`, `responseDelay`): timeout; thông báo `codeintel.*`; đăng ký trước khi kết nối; `resync`; `overflow` (đầy 64).
- **gRPC in-process (`adapter/grpc/server_test.go`):** `RelayByDevServer` ra trailer; `StreamCodeIntelEvents` tenant.
- **Hợp đồng:** `buf breaking` so với `main`.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy `StreamFileChanges` để xác nhận lỗi tenant (mục 1 điểm 5); nếu hoá ra có cơ chế gắn tenant khác ngoài `ChainUnary`, thiết kế 2.4 vẫn an toàn (gọi hàm thừa vô hại).
- Đổi `Exec` trả `*domain.AgentRPCError` có thể phá chỗ nào đó `errors.As(…, *JSONRPCError)` ở ngoài adapter; chưa grep hết (cần làm đầu tiên).
- `grpc.SetTrailer` và `-bin` chưa thử qua `otelgrpc`/interceptor thực tế.
- Phụ thuộc agent gửi `error.data.code` đúng tên (CR-CV-001); nếu agent đổi tên mã, ánh xạ rơi về `INFRA_AGENT_EXEC_FAILED` — suy giảm nhưng không hỏng.
- `tools[]` chỉ có từ lúc handshake; agent cài/gỡ `gitnexus` sau đó không cập nhật cho đến khi nối lại. `codeintel.status` là nguồn sự thật; `tools[]` chỉ chặn sớm.
- Thông báo hàng loạt (reindex chạy nhiều `reindexProgress` mỗi giây): mỗi sự kiện chỉ giải mã nhỏ nên rẻ, nhưng tần suất tối đa chưa đặt (CR-CV-004 cần giới hạn phía agent, ví dụ ≤ 2 sự kiện/giây).
- Việc agent `relay-ssh` (Part B) không có `tools/*` và chưa hỗ trợ `codeintel.*` (CR-CV-006): `StreamCodeIntelEvents` không chặn theo mode như `StreamFileChanges` (`client.go:721-723`); vì sổ đăng ký ở `Client`, dev server `relay-ssh` đơn giản không phát sự kiện.

## 7. Câu hỏi mở

- **Q1.** `codeintel.reindex` trả ngay (nền) hay chặn? Quyết định mục 2.2 dòng `reindex` (CR-CV-004).
- **Q2.** Số dev server đồng thời có binding: nếu lớn, chuyển sang NATS (mục 2.6)? Ngưỡng chưa đo.
- **Q3.** `internalcaller.StreamGuard` có đang được dùng ở infra-fleet? Chưa xác nhận trong `cmd/server/main.go`; nếu chưa thì `StreamCodeIntelEvents` dựa vào NetworkPolicy như các RPC còn lại.
- **Q4.** `GetAgentCapabilities` có nên trả luôn `hasCodeIntel` (agent có `codeintel.*` không) suy từ `capabilities` (agent liệt kê `codeintel` trong `capabilities[]`)? Cần CR-CV-001 thêm giá trị vào `buildCapabilities` (`agent-session-capabilities.ts`).

## 8. Tham chiếu

- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (dòng 228-240, 403-452, 473-493, 720-780), `session.go` (dòng 47-60, 136-141, 198, 434-466, 557-680, 752-870, 1160-1226), `agent_methods.go` (dòng 99-118), `client_test.go` (dòng 300-340)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go` (dòng 73-81, 150-225)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go`, `relay.go`, `stream_file_changes.go`, `ports.go` (dòng ~50-53, ~79-90, ~405-430)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (dòng 660-714, 1385-1392, 1499-1512, 1545-1558)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/eventbus/publisher.go`, `/opt/repos/orca/backend-go/services/infra-fleet-service/cmd/server/main.go` (dòng 254-272, 679)
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (dòng 103-114, 866-905)
- `/opt/repos/orca/backend-go/common/apperrors/apperrors.go` (dòng 97-130), `common/grpcmw/grpcmw.go` (dòng 58-131), `common/internalcaller/internalcaller.go`, `common/eventbus/eventbus.go`
- `/opt/repos/orca/agent/src/relay/agent-session-handshake.ts` (dòng 57-75), `agent-tool-registry.ts` (dòng 185-206)
- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.2, 3.3, 3.6, 7), `/opt/repos/orca/docs/research/view-code/01-agent-backend-connection.md`, `07-architecture-decisions.md` (D4)
- CR-CV-021, CR-CV-022, CR-CV-024 cùng folder
