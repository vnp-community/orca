# CR-CV-040 — Kênh WS `codeIntel.*` của `api-gateway`, push, giới hạn kích thước và parity MCP

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-040 |
| **Tên** | Đăng ký nhóm kênh `codeIntel.*` trong `wscompat`, kênh push `codeIntel.subscribe`, kết nối gRPC tới `code-intel-service`, giới hạn kích thước, ánh xạ lỗi, loại trừ khỏi MCP |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large (6 đến 8 ngày: wiring, 26 kênh, view camelCase, stream, giới hạn, ánh xạ lỗi, test) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010 (proto `orca.codeintel.v1`, service chạy được), CR-CV-011, 012, 013 (quyền, ánh xạ), CR-CV-021 (collector); từng kênh cần RPC của CR-CV-031, 033, 034, 036, 037, 038 (kênh nào chưa có RPC thì trả `CODEINTEL_UNAVAILABLE`, xem 2.1) |
| **Mở khoá** | CR-CV-041, CR-CV-050 đến 062 (frontend), CR-CV-071, 072, 073 |
| **Tác động** | `backend-go/services/api-gateway` (`internal/config/config.go`, `cmd/server/main.go`, `internal/adapter/wscompat/register_production.go`, 5 file `channels_codeintel_*.go` mới, `internal/adapter/mcpserver/tools/excluded_channels.yaml`, `README.md`); `deploy/dev/docker-compose.yml` (biến môi trường) |

---

## 1. Bối cảnh và vấn đề

1. `code-intel-service` (CR-CV-010) chỉ nói gRPC. Frontend gọi qua WebSocket `wscompat`; không có kênh thì chưa UI nào dùng được (CR-CV-050 trở đi). Hiện không có file `channels_codeintel*.go` hay client `codeintelv1` nào trong `api-gateway` (đã liệt kê thư mục `wscompat`, 2026-10-05).
2. Mẫu đăng ký đã rõ và đã đọc:
   - `Registry.Register(channel, handler)` với `ChannelHandler func(ctx, Identity, []json.RawMessage) (any, error)` (`wscompat/registry.go:44,107`).
   - Kênh push: `Registry.RegisterStream(channel, StreamHandler)` trả `<-chan PushEvent` (`registry.go:115`, `push_bridge.go:18-28`); mẫu bọc một gRPC server-stream là `registerWorkspacePortsStreamChannel` và `registerNotificationStreamChannel` (`channels_push.go:50-116`).
   - Mỗi nhóm một file `channels_*.go`, client gRPC vào qua `ChannelDeps` và `RegisterProductionChannels` (`register_production.go:317-369`); mọi field nil vẫn hợp lệ vì đăng ký chỉ tạo closure.
   - Mẫu relay sang máy dev là `files.browseServerDir` (`channels_files.go`), nhưng với code-intel **gateway không relay**: `code-intel-service` gọi `RelayByDevServer` (quyết định D3).
3. `Registry.Dispatch` đã gắn `Identity` (gồm `Role`) lên metadata gRPC và đặt trần 60 s (`registry.go:172-201`); `handleInvoke` đặt `invokeTimeout` = 25 s (`handler.go:247`), `rpcTimeout` mỗi lệnh gọi gRPC của handler là 8 s (`channels.go:105`). Đường stream (`StreamHandlerFor`, `handler.go:228`) **không** đi qua `Dispatch` nên **không** có `AttachIdentity` tự động; `workspacePorts.subscribe` tự gắn (`channels_push.go:90`, thiếu `Role`).
4. Gateway không có OPA trước định tuyến (`api-gateway/README.md` dòng 83, 96; đã kiểm lại). Mọi RPC của `code-intel-service` phải tự kiểm quyền (CR-CV-013); gateway chỉ giữ ranh giới vào.
5. Kênh mới làm đỏ `TestChannelInventory` trong `mcpserver/tools/parity_test.go` nếu không có `ToolSpec` hoặc dòng loại trừ, và `TestToolParity` đỏ nếu một dòng loại trừ không khớp kênh nào (`parity_test.go:84-96`). Inventory dựng bằng `wscompat.RegisterProductionChannels(r, ChannelDeps{TaskActivityEnabled: true})` (`parity_test.go:14-19`), nên kênh `codeIntel.*` phải đăng ký **không điều kiện**.
6. Hạ tầng có giới hạn kích thước chưa ai ghi lại cho code-intel:
   - `gatewaygrpc.Dial` chỉ đặt credentials và stats handler, không đặt `MaxCallRecvMsgSize` (`adapter/grpc/dial.go:31-33`), nên phản hồi gRPC lớn hơn mặc định 4 MiB của grpc-go bị từ chối `ResourceExhausted` (mặc định thư viện, chưa chạy thử).
   - Không có `SetReadLimit` ở bất kỳ đâu trong `api-gateway` (grep 2026-10-05), nên giới hạn thông điệp **vào** là mặc định của `coder/websocket` v1.8.15 (theo tài liệu thư viện là 32768 byte; chưa kiểm chứng vì mã module không có trong cache). Ảnh hưởng `codeIntel.reviewState.save` và `codeIntel.c4.save`.
   - Khung RPC agent tối đa 16 MiB (`devserveragent/frame.go:24`) là ranh giới phía sau, không phải của gateway.

## 2. Giải pháp đề xuất

### 2.1 Wiring client

| Việc | File |
|---|---|
| Thêm `"code-intel-service": commonconfig.StringEnv("CODE_INTEL_SERVICE_ADDR", "")` vào `OtherServiceAddrs` | `internal/config/config.go` (cạnh `task-service`) |
| Chỉ dial khi địa chỉ khác rỗng, theo mẫu MCP (`cmd/server/main.go:387-398`): `gatewaygrpc.Dial(addr)` rồi `codeintelv1.NewCodeIntelServiceClient(conn)`; địa chỉ rỗng thì giữ client `nil` và log một cảnh báo | `cmd/server/main.go` |
| Đặt tường minh `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4<<20))` cho kết nối này (hiện đã là mặc định, ghi ra để không ai nâng vô ý) | `cmd/server/main.go` hoặc hàm dial riêng `dialCodeIntelService` |
| `healthSrv.Register("code-intel-service", grpcConnHealthCheck(conn))` khi đã dial | `cmd/server/main.go` (cạnh dòng 558) |
| Thêm `CodeIntel codeintelv1.CodeIntelServiceClient` vào `ChannelDeps`; gọi `registerCodeIntelChannels(r, d.CodeIntel)` ở cuối `RegisterProductionChannels` | `wscompat/register_production.go` |
| Truyền `CodeIntel: codeIntelClient` vào `ChannelDeps{...}` | `cmd/server/main.go:371` |
| Biến `CODE_INTEL_SERVICE_ADDR` cho `api-gateway` | `deploy/dev/docker-compose.yml` (khối env chung chứa `TASK_SERVICE_ADDR`, dòng 65) |

Mọi handler bắt đầu bằng kiểm tra `client == nil` và trả `CODEINTEL_UNAVAILABLE: code-intel service not configured`. Lưu ý: `codeintelv1.CodeIntelServiceClient` là interface, nên gán một con trỏ nil của kiểu cụ thể sẽ không bằng `nil`; chỉ gán khi đã dial.

Kênh nào chưa có RPC thật ở `code-intel-service` (ví dụ `contractDiff` trước CR-CV-038): service trả `Unimplemented`; gateway ánh xạ thành `CODEINTEL_UNAVAILABLE` (xem 2.8). Không khai báo RPC chưa có message (xem v6 CR-REQ-001, README v7 3.6).

### 2.2 Tệp mới trong `wscompat` (mới)

| File | Nội dung |
|---|---|
| `channels_codeintel.go` | `registerCodeIntelChannels` và các kênh đọc: `status`, `structure`, `architecture`, `dataFlows`, `dataFlow`, `erd`, `storage`, `subgraph`, `impact`, `symbol`, `routes`, `changeOverlay`, `readingOrder`, `findings`, `contractDiff` |
| `channels_codeintel_write.go` | kênh ghi: `reindex`, `reindexStatus`, `dismissFinding`, `reviewState.get/save`, `c4.get/save`, `bindRepo`, `settings.get/set` |
| `channels_codeintel_stream.go` | `codeIntel.subscribe` |
| `channels_codeintel_args.go` | giải mã chặt `args[0]`, kiểm giới hạn tham số, ánh xạ lỗi, kiểm kích thước phản hồi |
| `channels_codeintel_views.go` | struct JSON camelCase cho phản hồi (không trả message proto trực tiếp) |
| `channels_codeintel_test.go`, `..._stream_test.go`, `..._args_test.go` | fake client theo mẫu `fakeOrchestrationClient` (`channels_orchestration_test.go:15`) |

Quy tắc view: JSON mặc định của message proto ra snake_case (ghi chú BUG-023 ở `channels.go`), nên luôn qua struct view camelCase. Các cây dữ liệu lớn (`ArchitectureGraph`, `SymbolGraph`, `ErdModel`...) đi qua `payloadJson` (xem 2.6) để gateway không phải mirror từng kiểu. Slice nil được `normalizeNilSlices` chuẩn hoá thành `[]` (`registry.go:227`) nhưng chỉ ở mức đầu; view tự bảo đảm slice lồng không `null`.

### 2.3 Dialect và quy tắc `args` theo vị trí

`ChannelHandler` nhận `args []json.RawMessage` đúng như mảng `args` của envelope `invoke`. Có hai dialect đi tới cùng handler (`session_dialect.go:57-69`):

| Dialect | Vào | Thành `args` |
|---|---|---|
| native (`rpc-client.ts`) | `{"id","type":"invoke","channel","args":[...]}` | nguyên mảng |
| session-client (`WebSessionClient`) | `{"id","authToken","method","params"}` | `[params]`; `params` vắng hoặc `null` thành `{}` (`emptyParams`) |

Quy tắc cho mọi kênh `codeIntel.*`:

1. Chỉ đọc `args[0]`, bắt buộc là một object JSON (`{}` hợp lệ khi mọi trường tuỳ chọn). Nếu `len(args) > 1` thì từ chối `CODEINTEL_INVALID_PARAMS: expected a single params object`; không im lặng bỏ qua phần dư, vì dialect session-client không bao giờ sinh nó và client native gửi dư là lỗi lập trình.
2. Giải mã bằng `json.Decoder` với `DisallowUnknownFields` (hàm mới `decodeCodeIntelArgs[T]` trong `channels_codeintel_args.go`). `decodeArg[T]` của `registry.go:267` dùng `json.Unmarshal` lỏng nên **không** dùng ở đây: một client gửi thêm `workspaceRoot` hay `repo` sẽ bị bỏ qua thay vì bị báo lỗi (quyết định G1). Lỗi giải mã trả `CODEINTEL_INVALID_PARAMS` kèm **tên trường**, không kèm giá trị.
3. Với dialect session-client, lỗi ra dạng `{"ok":false,"error":{"code":"internal","message":"CODEINTEL_X: ..."}}`: `code` cố định `internal` (`session_dialect.go:94-101`), mã thật nằm ở tiền tố `message`. Frontend phải đọc tiền tố (CR-CV-050).
4. `send` (fire-and-forget, `handleSend`) gọi cùng `Dispatch` nhưng bỏ phản hồi và chỉ log lỗi (`handler.go:397-416`). Kênh ghi `codeIntel.*` gọi bằng `send` sẽ thất bại im lặng. Frontend phải dùng `invoke`; gateway không thể chặn ở mức kênh (không biết kiểu gọi). Ghi nhận là rủi ro.
5. Push: `PushEvent.Args` là slice vì client native spread vào callback `on(channel, handler)`; dialect session-client mất tên kênh (chỉ có `requestId`) và nhận `Args[0]` (`push_bridge.go:98-113`). Vì vậy mỗi khung push của code-intel có trường `event` (`"changed"` hoặc `"reindexProgress"`) ở trong payload, để cả hai dialect phân biệt được.

### 2.4 Danh sách kênh chốt cuối

Chốt từ README mục 3.7 (23 kênh) cộng 3 kênh mới (đánh dấu **mới**), tổng 26: 25 unary, 1 stream. Tất cả nhận một object ở `args[0]`, tên trường camelCase. `worktreeId` luôn là id Orca (không phải đường dẫn); tên trường proto cuối cùng do CR sở hữu RPC chốt, cột "Tham số" ghi những gì gateway kiểm. "Lớp quyền" là mức **service** thi hành (CR-CV-013); gateway không tự kiểm.

| Kênh | Tham số (camelCase) | RPC đích (3.6) | Lớp quyền | Timeout kênh | Ghi chú |
|---|---|---|---|---|---|
| `codeIntel.status` | `worktreeId` | `GetIndexStatus` | read | 8 s | Không yêu cầu cờ bật cho phần trạng thái cờ (xem 2.7) |
| `codeIntel.reindex` | `worktreeId`, `mode` ∈ `incremental`,`full` | `RequestReindex` | write (quyền chạy reindex, O3) | 8 s | Trả ngay `{jobId, status}`; tiến trình qua push. `CODEINTEL_REINDEX_IN_PROGRESS` |
| `codeIntel.reindexStatus` | `jobId` | `GetReindexJob` | read | 8 s | Dự phòng khi mất push |
| `codeIntel.structure` | `worktreeId`, `path?`, `depth?` ≤ 3, `limit?`, `pageToken?` | `GetStructure` | read | 20 s | |
| `codeIntel.architecture` | `worktreeId`, `level?`, `focus?` | `GetArchitecture` | read | 20 s | C4, ≤ 500 cụm / 5 000 cạnh (README 3.2) |
| `codeIntel.dataFlows` | `worktreeId`, `limit?` ≤ 100, `pageToken?` | `ListDataFlows` | read | 20 s | |
| `codeIntel.dataFlow` | `worktreeId`, `flowId`, `format?` | `GetDataFlow` | read | 20 s | ≤ 200 bước |
| `codeIntel.erd` | `worktreeId`, `service?`, `atCommit?` | `GetErd` | read | 20 s | |
| `codeIntel.storage` | `worktreeId` | `GetStorageMap` | read | 20 s | Không lộ secret (CR-CV-035, 072) |
| `codeIntel.subgraph` | `worktreeId`, `center`, `depth?` ≤ 3, `kinds?`, `limit?` ≤ 1 500 | `GetSubgraph` | read | 20 s | |
| `codeIntel.impact` | `worktreeId`, `target`, `direction`, `depth?` | `GetImpact` | read | 20 s | ≤ 300 nút; `CODEINTEL_AMBIGUOUS_SYMBOL` kèm `candidates` |
| `codeIntel.symbol` | `worktreeId`, `key` hoặc (`name`, `file`) | `GetSymbol` | read (lộ mã nguồn) | 20 s | ≤ 200 KiB mã; `file` qua `CleanWorktreePath` (2.5) |
| `codeIntel.routes` | `worktreeId`, `limit?`, `pageToken?` | `GetRouteMap` | read | 20 s | |
| `codeIntel.changeOverlay` | `worktreeId`, `base?`, `head?` | `GetChangeOverlay` | read | 20 s | Mặc định merge-base (O7) |
| `codeIntel.readingOrder` | `worktreeId`, `base?`, `head?` | `GetReadingOrder` | read | 20 s | |
| `codeIntel.findings` | `worktreeId`, `kinds?`, `severity?`, `limit?` ≤ 200, `pageToken?` | `ListFindings` | read | 20 s | |
| `codeIntel.dismissFinding` | `worktreeId`, `findingKey`, `reason` ≤ 500 | `DismissFinding` | write | 8 s | |
| `codeIntel.contractDiff` | `worktreeId`, `base?`, `head?` | `GetContractDiff` | read | 20 s | P2 (CR-CV-038) |
| `codeIntel.reviewState.get` | `worktreeId`, `baseCommit?`, `headCommit?` | `GetReviewState` | read | 8 s | O6: lưu ở backend theo `(worktree, commit)` |
| `codeIntel.reviewState.save` | `worktreeId`, `baseCommit`, `headCommit`, `readingProgress`, `notes`, `expectedVersion` | `SaveReviewState` | write | 8 s | Giới hạn 28 KiB (2.6); `CODEINTEL_VERSION_CONFLICT` |
| `codeIntel.c4.get` | `worktreeId`, `container?` | `GetC4Overrides` | read | 8 s | |
| `codeIntel.c4.save` | `worktreeId`, `container`, `document`, `expectedVersion` | `SaveC4Overrides` | write | 8 s | Giới hạn 28 KiB; `document` là YAML text |
| `codeIntel.bindRepo` | `projectId`, `worktreeId` | `BindRepo` | write | 8 s | Không nhận đường dẫn hay tên repo (O4); service tự suy ra |
| `codeIntel.settings.get` (**mới**) | không | `GetSettings` (CR-CV-073, **mới**) | thành viên tenant | 8 s | `{enabled}`; luôn trả được kể cả khi cờ tắt |
| `codeIntel.settings.set` (**mới**) | `enabled` | `SetSettings` (CR-CV-073, **mới**) | admin | 8 s | Service kiểm `Identity.Role` |
| `codeIntel.subscribe` (**mới**, stream) | `worktreeId?` | `StreamCodeIntelEvents` | read | stream | Mở push `codeIntel.changed`, `codeIntel.reindexProgress` (2.5) |

`ListRepoBindings` (README 3.6) không có kênh: `status` đã trả thông tin ràng buộc của một worktree và chưa có màn hình nào cần liệt kê. Nếu CR-CV-012 hoặc 050 cần, thêm `codeIntel.bindings` ở đây.

Timeout: mỗi handler bọc `context.WithTimeout` (8 s hoặc 20 s) trên cùng ctx của `Dispatch`; cả hai nhỏ hơn `invokeTimeout` = 25 s để lỗi là của ta và nhỏ hơn 30 s `INVOKE_TIMEOUT_MS` phía `rpc-client.ts` (theo chú thích ở `handler.go:241-247`). 20 s là giá trị đề xuất vì collector có thể gọi agent nhiều lần (~1,8 s mỗi lần gọi CLI, README mục 1); service **phải** tôn trọng deadline của ctx và khi hết hạn thì trả `CODEINTEL_TIMEOUT` trong khi vẫn hoàn tất việc nền để lần gọi lại trúng cache (singleflight, CR-CV-022). Hằng số mới `codeIntelReadTimeout = 20 * time.Second` đặt cạnh các hằng ở `channels_codeintel_args.go`; bổ sung test `TestCodeIntelTimeouts_ShorterThanInvokeTimeout` theo mẫu `TestRPCTimeoutConstant_ShorterThanInvokeTimeout` (`channels_test.go:1212`).

### 2.5 Push `codeIntel.subscribe`

Theo mẫu `registerWorkspacePortsStreamChannel` (`channels_push.go:81-116`), khác ba chỗ.

```go
r.RegisterStream("codeIntel.subscribe", func(ctx, id, args) (<-chan PushEvent, error) {
    in, err := decodeCodeIntelArgs[subscribeArgs](args)        // worktreeId tuỳ chọn
    ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
    stream, err := client.StreamCodeIntelEvents(ctx, &codeintelv1.StreamCodeIntelEventsRequest{WorktreeId: in.WorktreeID})
    // goroutine: Recv -> PushEvent{Channel: "codeIntel.changed" | "codeIntel.reindexProgress", Args: []any{view}}
})
```

| Điểm | Quyết định | Lý do |
|---|---|---|
| Identity | Gắn `AttachIdentity` **có `Role`** trong handler | Đường stream không qua `Dispatch` (xem 1.3) |
| Quyền và tenant | Service lọc theo tenant lấy từ metadata và kiểm quyền đọc `worktreeId` **trước** khi mở stream; lỗi quyền trả ở ack (lỗi của `sh()` được ghi bằng `writeDialectError`, `handler.go:371-376`). Gateway không lọc lại theo nội dung khung | Một điểm thi hành; không có `tenantId` trong khung |
| Nội dung khung | Chỉ metadata nhẹ, không graph, không đường dẫn tuyệt đối: `changed` = `{event:"changed", worktreeId, tool, commit, indexedAt, stale, resync?}`; `reindexProgress` = `{event:"reindexProgress", jobId, worktreeId, stage, percent, message}` | D4: không đẩy cả graph; `workspaceRoot` không rời backend |
| Đóng stream | Khi `Recv` lỗi (service khởi động lại, mất kết nối) phát **một** khung `codeIntel.changed` với `resync:true` rồi đóng kênh | `pipePush` (native) chỉ trở về khi kênh đóng, client không biết; `resync` buộc UI tải lại. Dialect session-client còn nhận khung `end` (`push_bridge.go:81-87`) |
| Áp lực ngược | Kênh `out` không đệm như mẫu; chặn ở `stream.Recv` nhờ cơ chế flow control gRPC. Service chịu trách nhiệm gộp (coalesce) khung `changed` dồn dập | Tránh bộ nhớ vô hạn ở gateway |
| Số luồng | Một kết nối WS mở tối đa **1** `codeIntel.subscribe` (lần hai thay thế lần đầu: huỷ ctx cũ); mỗi replica tối đa `CODE_INTEL_MAX_STREAMS` (mặc định đề xuất 500, quá thì `CODEINTEL_RATE_LIMITED`) | Mỗi subscribe giữ một gRPC stream tới service; chưa đo tải (mục 6) |

Chưa có cơ chế fan-out dùng chung ở gateway (mỗi socket một gRPC stream, giống `workspacePorts.subscribe`). Phương án khác (gateway giữ một stream theo tenant rồi fan-out nội bộ) phức tạp hơn; chưa cần ở MVP.

Parity MCP: kênh stream không thể thành tool; dòng loại trừ ở 2.9 bao phủ.

### 2.6 Giới hạn kích thước và phân trang

| Hướng | Giới hạn | Nơi thi hành |
|---|---|---|
| Tham số vào (`reviewState.save`, `c4.save`) | tổng `args[0]` ≤ **28 KiB** (kiểm `len(args[0])` trước khi giải mã); dưới giới hạn đọc mặc định 32 KiB của WS (1.6) | gateway, lỗi `CODEINTEL_INVALID_PARAMS: params too large` |
| Chuỗi và mảng trong tham số | `reason` ≤ 500, `findingKey` ≤ 256, `kinds` ≤ 32 phần tử, `key` ≤ 1 024, `document` ≤ 24 KiB | gateway |
| Phạm vi số | `depth` 1..3, `limit` trong ngân sách README 3.2 (`subgraph` ≤ 1 500, `dataFlows` ≤ 100, `findings` ≤ 200); ngoài khoảng thì **từ chối**, không kẹp ngầm | gateway (tầng 1) và agent/service (tầng 2) |
| Phản hồi từ service | `proto.Size(resp)` > `CODE_INTEL_MAX_RESPONSE_BYTES` (mặc định **2 MiB**; riêng `symbol` 320 KiB cho 200 KiB mã nguồn cộng khung) thì trả `CODEINTEL_RESPONSE_TOO_LARGE`, **không** cắt ngầm | gateway; việc cắt có `truncated`/`totalCount` là của agent và service (README mục 6) |
| Kênh gRPC | `MaxCallRecvMsgSize` 4 MiB tường minh (2.1); 2 MiB của cap nằm dưới nó có chủ ý | gateway |
| Phân trang | Danh sách dùng `limit` + `pageToken` mờ do service cấp; gateway không diễn giải token, chỉ giới hạn độ dài ≤ 512 và trả `nextPageToken` nguyên văn | gateway |

Phản hồi cấu trúc lớn đi qua một trường `payloadJson` (chuỗi JSON do service dựng từ schema chuẩn) hay field có kiểu là quyết định của CR-CV-020/050; CR này chỉ yêu cầu gateway **không** nhân đôi bộ nhớ (không giải mã rồi mã hoá lại cả cây) khi chỉ cần chuyển tiếp. Cap kích thước áp trên `proto.Size`, không trên JSON cuối.

### 2.7 Kiểm quyền và cờ `code_intel_enabled`

| Hạng mục | Quyết định |
|---|---|
| Quyền | Gateway chỉ bảo đảm `Identity.TenantID` và `Identity.UserID` không rỗng (không thì `CODEINTEL_NOT_FOUND`, xem ghi chú dưới) rồi chuyển qua metadata (`AttachIdentity`). Quyền đọc worktree/project, quyền reindex, quyền ghi trạng thái review do `code-intel-service` thi hành (CR-CV-013); mỗi RPC phải có test từ chối ở service. CR này **không** coi gateway là lớp bảo vệ |
| Giả mạo | Không kênh nào nhận `tenantId`, `userId`, `deviceId`, `role`; test gửi giá trị giả và khẳng định RPC nhận giá trị từ `Identity` (như tiêu chí v6 CR-REQ-016) |
| `DeviceID` | Phiên thiết bị di động (`Identity.DeviceID` khác rỗng) **bị từ chối** ở mọi kênh `codeIntel.*` ngoài `settings.get`, cho tới khi CR-CV-062 mở màn tóm tắt chỉ đọc. Lý do: `Identity.DeviceID` chỉ có ở JWT thiết bị (`registry.go:22-28`); không mở bề mặt mới cho mobile ngoài kế hoạch |
| Cờ | Thi hành ở `code-intel-service` bằng interceptor gRPC (CR-CV-073): cờ tắt thì mọi RPC ngoài `GetSettings` trả `CODEINTEL_DISABLED`. Gateway không giữ bản sao cờ nên không lệch; chỉ ánh xạ lỗi (2.8). Kênh vẫn **đăng ký** khi cờ tắt (quyết định G3), nên cờ không làm đổi inventory MCP |
| Không có `CODE_INTEL_ENABLED` ở gateway | Khác `MCP_ENABLED` (cờ tiến trình của gateway, `config_mcp.go`): ở đây không có việc gì gateway phải bật/tắt riêng; địa chỉ rỗng đã đủ để "tắt" ở mức triển khai |

Ghi chú: mã khi `Identity` rỗng không thể xảy ra trên đường WS đã xác thực (`resolveIdentity`, `handler.go:120`); giữ kiểm tra như phòng thủ, với thông điệp trung tính để không thành oracle. Mã cụ thể cho từ chối quyền do CR-CV-013 định nghĩa; gateway chuyển nguyên văn nếu khớp mẫu `CODEINTEL_*` (2.8).

Đường dẫn trong tham số: các trường `path` (structure) và `file` (symbol) là đường dẫn **tương đối gốc repo** (README 3.4). Gateway chạy `tools.CleanWorktreePath`-tương đương (logic ở `mcpserver/tools/sensitive_path_rules.go:24-40`: từ chối `\`, tiền tố `/`, `%XX` còn dư, `..`, chuẩn hoá NFKC) ở `decodeCodeIntelArgs`; giá trị bị từ chối trả `CODEINTEL_PATH_NOT_ALLOWED`. Vì gói `mcpserver/tools` import `wscompat` (`spec.go:15`), `wscompat` **không** được import ngược lại; thay vào đó chuyển logic kiểm đường dẫn sang một gói không phụ thuộc (đề xuất `internal/adapter/pathsafety`, tên theo AGENTS.md: gọi theo khái niệm) và cho cả hai dùng. Đây là điểm cần quyết định khi triển khai (Q3). Service kiểm lại độc lập (CR-CV-030, 072).

### 2.8 Ánh xạ lỗi

Mẫu có sẵn: `mcpChannelError` trong `channels_mcp.go:85-119` (bóc `rpc error: code = ... desc =`, giữ thông điệp có mã, còn lại ánh xạ theo `status.Code`). Hàm mới `mapCodeIntelError(err)` (cùng ý với `mcpChannelError`) trong `channels_codeintel_args.go`:

1. Lấy `status.FromError(err)`; nếu có thì dùng `st.Message()`; bóc tiền tố `rpc error: code = X desc =`; cắt khoảng trắng, **cắt về một dòng ≤ 200 ký tự**.
2. Nếu khớp `^CODEINTEL_[A-Z0-9_]+: ` thì trả nguyên (mã do service hoặc agent đặt; service giữ tiền tố `CODEINTEL_` trong `apperrors`, README 3.3). `CODEINTEL_AMBIGUOUS_SYMBOL` mang `candidates`: service nhét danh sách (≤ 10) dạng JSON gọn **sau** dấu `|` cuối thông điệp, hoặc dùng `status.Details`; cách mã hoá chốt cùng CR-CV-021/050 (Q2). Gateway không diễn giải.
3. Không khớp thì ánh xạ theo `status.Code`:

| `status.Code` (không có mã `CODEINTEL_`) | Mã ra WS |
|---|---|
| `Unavailable`, `Unimplemented`, kết nối `nil` | `CODEINTEL_UNAVAILABLE` |
| `DeadlineExceeded`, `context.DeadlineExceeded` | `CODEINTEL_TIMEOUT` |
| `NotFound`, `PermissionDenied` | `CODEINTEL_NOT_FOUND` (không phân biệt "không có" và "không phải của bạn") |
| `InvalidArgument` | `CODEINTEL_INVALID_PARAMS` |
| `FailedPrecondition` | `CODEINTEL_TOOL_FAILED` |
| `ResourceExhausted` | `CODEINTEL_RATE_LIMITED` (hạn mức đồng thời của CR-CV-013) hoặc `CODEINTEL_RESPONSE_TOO_LARGE` nếu thông điệp nhắc kích thước |
| `Aborted`, `AlreadyExists` | `CODEINTEL_VERSION_CONFLICT` (khi `expectedVersion` cũ) |
| còn lại | `CODEINTEL_INTERNAL: internal error` |

Mã mới ngoài README 3.3 (cần chốt khi duyệt, README mục 8): `CODEINTEL_UNAVAILABLE`, `CODEINTEL_DISABLED` (cờ tắt, do service), `CODEINTEL_NOT_FOUND`, `CODEINTEL_RESPONSE_TOO_LARGE`, `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_RATE_LIMITED`, `CODEINTEL_INTERNAL`. Các mã `CODEINTEL_TOOL_UNAVAILABLE`, `INDEX_MISSING`, `REPO_NOT_REGISTERED`, `PATH_NOT_ALLOWED`, `INVALID_PARAMS`, `AMBIGUOUS_SYMBOL`, `TIMEOUT`, `REINDEX_IN_PROGRESS`, `OUTPUT_TOO_LARGE`, `TOOL_FAILED` đã có (3.3) và được chuyển nguyên.

Điều kiện `CODEINTEL_REPO_NOT_REGISTERED` và `CODEINTEL_PATH_NOT_ALLOWED` của agent chạy qua `infra-fleet-service` rồi qua service nên đến gateway đã là thông điệp `CODEINTEL_*` của service; kiểm chứng đường truyền `error.data.code` của JSON-RPC agent qua `RelayByDevServer` thuộc CR-CV-021/023 (chưa kiểm chứng ở đây).

`scrubSensitiveChannelError` (`registry.go:198`) chỉ áp cho kênh trong `sensitiveArgChannels` (`channel_args_redaction.go:16`); kênh code-intel **không** thêm vào đó (tham số không phải secret), nhưng `mapCodeIntelError` tự bảo đảm thông điệp một dòng, không chứa mã nguồn, không chứa đường dẫn tuyệt đối (lọc chuỗi bắt đầu bằng `/` hoặc `X:\` dài ≥ 3 ký tự phân đoạn thì thay bằng `<path>`), và không chứa nội dung tham số.

### 2.9 Loại trừ khỏi MCP và parity

Thêm **một** dòng vào `mcpserver/tools/excluded_channels.yaml` (mẫu các dòng `files.*`, `workspacePorts.subscribe` ở dòng 391-455; `Match` hỗ trợ `.*` trải qua mọi cấp, `excluded.go:59-71`):

```yaml
- pattern: "codeIntel.*"
  category: code-intel-v1
  reason: "Code intelligence views are not MCP tools in v1 (README v7 O2): results carry repository source text, reindex runs analyzers on a dev server, and an LLM must not drive them without governance (CR-CV-041)."
```

`LoadExclusions` đòi `reason` ≥ 20 ký tự (`excluded.go:38-41`). Phải nằm **cùng PR** với việc đăng ký kênh: dòng loại trừ không khớp kênh nào làm `TestToolParity` đỏ ("exclusion ... matches no registered channel"), kênh không có dòng làm `TestChannelInventory` đỏ. CR-CV-041, nếu được duyệt, **thay** dòng này bằng danh sách tường minh các kênh còn loại trừ (một kênh vừa có `ToolSpec` vừa bị loại trừ cũng làm đỏ: "both exposed and excluded", `parity_test.go:56-58`). `golden` `tools_list.golden.json` không đổi vì không có tool mới.

### 2.10 Quan sát tại gateway (tối thiểu)

CR-CV-071 sở hữu metrics và span. CR này chỉ yêu cầu: không log `args` hay kết quả (đang đúng cho mọi kênh: "currently log and trace no args at all", `channel_args_redaction.go:11-15`); không đưa mã nguồn hay đường dẫn vào thông điệp lỗi (2.8); span `otelgrpc` của client gRPC đã có sẵn từ `gatewaygrpc.Dial` (`dial.go:32`).

### 2.11 Tài liệu

Cập nhật `api-gateway/README.md`: thêm `code-intel-service` vào danh sách downstream và 26 kênh vào bảng kênh; thêm biến `CODE_INTEL_SERVICE_ADDR`, `CODE_INTEL_MAX_RESPONSE_BYTES`, `CODE_INTEL_MAX_STREAMS`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Giải mã tham số chặt, từ chối khoá lạ và `len(args) > 1` | Chặn client nhét `workspaceRoot`, `repo`, `args`; `decodeArg` hiện tại bỏ qua khoá lạ |
| D2 | Gateway không relay tới agent, không đọc `fs.*` | D3 README: `code-intel-service` là nơi duy nhất nói với `infra-fleet-service` |
| D3 | Từ chối thay vì kẹp ngầm `limit`/`depth` ngoài ngân sách | UI biết mình xin quá; kết quả không im lặng khác điều xin |
| D4 | Không cắt phản hồi ở gateway, chỉ chặn bằng `CODEINTEL_RESPONSE_TOO_LARGE` | Cắt đúng ngữ nghĩa (`truncated`, `totalCount`) cần biết cấu trúc, thuộc agent và service |
| D5 | Timeout đọc 20 s, ghi/trạng thái 8 s, đều < 25 s | Giữ nguyên bất biến `rpcTimeout < invokeTimeout < INVOKE_TIMEOUT_MS`; xem 2.4 |
| D6 | `codeIntel.subscribe` là `RegisterStream` (không phải `RegisterStreamChannel`) | Không cần ack mang dữ liệu; mẫu `workspacePorts.subscribe` |
| D7 | Phát `resync:true` khi stream đứt | Client native không nhận tín hiệu đóng kênh (xem 2.5) |
| D8 | Thông điệp lỗi một dòng, không chứa mã nguồn hay đường dẫn tuyệt đối | Mã nguồn của khách hàng không được lọt vào log, báo lỗi UI hay telemetry |
| D9 | Loại trừ MCP bằng một mẫu `codeIntel.*` | Gọn, và CR-CV-041 tự thu hẹp khi cần |
| D10 | Từ chối phiên thiết bị di động (`DeviceID`) cho tới CR-CV-062 | Không mở bề mặt mobile ngoài kế hoạch |

## 4. Tiêu chí chấp nhận

- [ ] `CODE_INTEL_SERVICE_ADDR` được đọc; gateway khởi động được khi để trống, mọi kênh `codeIntel.*` trả `CODEINTEL_UNAVAILABLE` (không panic vì client `nil`).
- [ ] 26 kênh ở 2.4 có trong `Registry.Channels()`: 25 `ChannelUnary` và 1 `ChannelStream` (`codeIntel.subscribe`); test inventory liệt kê đúng tên.
- [ ] Không kênh nào chấp nhận `tenantId`, `userId`, `devServerId`, `workspaceRoot`, `repo`, `args`, `command`, `cypher`: gửi vào trả `CODEINTEL_INVALID_PARAMS` nêu tên trường; test gửi giá trị giả cho `tenantId` và khẳng định RPC nhận giá trị từ `Identity`.
- [ ] `len(args) > 1`, `args[0]` không phải object, `args` rỗng với kênh có trường bắt buộc đều bị từ chối; dialect session-client với `params` vắng cho kênh không tham số vẫn chạy.
- [ ] `path`/`file` chứa `..`, `/abs`, `\`, `%2e%2e`, dạng full-width `．．` bị từ chối `CODEINTEL_PATH_NOT_ALLOWED`; dùng chung logic kiểm của `sensitive_path_rules.go` mà không tạo import vòng.
- [ ] `limit`, `depth` ngoài ngân sách bị từ chối; không có kẹp ngầm.
- [ ] `reviewState.save`/`c4.save` với `args[0]` > 28 KiB bị từ chối trước khi giải mã.
- [ ] Phản hồi proto > 2 MiB (hoặc 320 KiB với `symbol`) trả `CODEINTEL_RESPONSE_TOO_LARGE`; test với fake client trả message lớn.
- [ ] Kết quả danh sách rỗng là `[]`, không `null`; mọi view JSON là camelCase, không có khoá snake_case (test quét).
- [ ] Ánh xạ lỗi theo bảng 2.8, gồm `Unimplemented` thành `CODEINTEL_UNAVAILABLE`, `PermissionDenied` thành `CODEINTEL_NOT_FOUND`; thông điệp một dòng ≤ 200 ký tự; test với lỗi chứa đường dẫn `/home/dev/...` khẳng định đường dẫn bị che.
- [ ] `codeIntel.subscribe`: ack thành công rồi nhận `codeIntel.changed` và `codeIntel.reindexProgress` đúng tên kênh (native) và có `event` trong payload (session-client); stream đứt phát một khung `resync:true` rồi đóng; quá hạn mức stream trả `CODEINTEL_RATE_LIMITED`; lần subscribe thứ hai trên cùng socket thay thế lần đầu và huỷ gRPC stream cũ (không rò goroutine).
- [ ] Stream không rò sự kiện tenant khác: fake service phát khung của tenant B vào stream của tenant A thì (do service lọc) test ở tầng service đỏ; ở gateway test khẳng định metadata gRPC mang đúng `TenantID`.
- [ ] Phiên `Identity.DeviceID != ""` bị từ chối ở mọi kênh ngoài `settings.get`.
- [ ] `go test ./internal/adapter/mcpserver/tools/...` xanh với một dòng `codeIntel.*` trong `excluded_channels.yaml` và kênh đã đăng ký; xoá dòng thì `TestChannelInventory` đỏ.
- [ ] Không thêm `max-lines` disable (AGENTS.md); mỗi file `channels_codeintel_*.go` nhỏ hơn ngưỡng của `config/max-lines-baseline.txt`.
- [ ] `README.md` của `api-gateway` ghi `code-intel-service`, 26 kênh và ba biến môi trường.

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `channels_codeintel_args_test.go` | `decodeCodeIntelArgs`: khoá lạ, dư `args`, `params` vắng (session-client), giới hạn số, đường dẫn độc hại, tham số quá lớn |
| `channels_codeintel_test.go` | từng kênh với fake `CodeIntelServiceClient`: ánh xạ tham số, camelCase, `[]`, timeout (ctx deadline đúng 8 s hoặc 20 s), cap kích thước, `client == nil` |
| `channels_codeintel_errors_test.go` | bảng 2.8; che đường dẫn; một dòng; không rò nội dung tham số |
| `channels_codeintel_stream_test.go` | ack + hai loại khung, `resync`, thay thế subscribe, huỷ ctx, giới hạn stream, hai dialect (mẫu `push_bridge_test.go`, `channels_task_activity_test.go`) |
| `channels_codeintel_identity_test.go` | không kênh nào nhận `tenantId`/`userId`; `DeviceID` bị từ chối; metadata gRPC có `Role` |
| `registry_channels_test.go` (sửa) | inventory có 26 kênh mới |
| `mcpserver/tools/parity_test.go` (chạy lại) | xanh nhờ dòng loại trừ; đỏ khi bỏ |
| `register_production_test.go` (sửa) | `ChannelDeps{}` rỗng vẫn đăng ký đủ |
| Tích hợp (CR-CV-073) | WS thật tới service thật: `tests/` kiểu `api_websocket_channels.py` |

Chưa chạy: toàn bộ danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Giới hạn đọc WebSocket**: giá trị mặc định 32 KiB của `coder/websocket` v1.8.15 chưa kiểm chứng (không có mã module trong cache để đọc); nếu thực tế khác thì sửa giới hạn 28 KiB. Nâng giới hạn bằng `conn.SetReadLimit` ảnh hưởng mọi kênh của `/ws`, cần quyết định riêng (Q1).
- **`MaxCallRecvMsgSize`** mặc định 4 MiB là kiến thức về grpc-go, chưa chạy thử trên kết nối này.
- **Độ trễ**: kênh đọc lạnh có thể vượt 20 s trên repo cỡ Orca (CLI ~1,8 s mỗi lần, DB CodeGraph ~1,2 GB); phụ thuộc cách service gộp truy vấn (CR-CV-021) và đo ở CR-CV-071.
- **Mỗi socket một gRPC stream** (`codeIntel.subscribe`): chưa đo tải; mặc định 500 stream/replica là đề xuất, không có số đo.
- **`send` thất bại im lặng** cho kênh ghi (2.3, mục 4): chỉ giảm được bằng quy ước phía frontend.
- **Import vòng** nếu dùng lại `CleanWorktreePath` từ `mcpserver/tools` (2.7): cần tách gói; chưa chọn tên cuối cùng.
- **Phiên session-client** cố định mã lỗi `internal`: người dùng web cần đọc tiền tố `message`; chưa kiểm chứng `WebSessionClient` hiện hiển thị gì.
- Gateway không có OPA: nếu một RPC của `code-intel-service` thiếu kiểm quyền thì lộ ngay qua WS (và `symbol` lộ mã nguồn). Cần test quyền ở phía service (CR-CV-013, 072).
- `auditclient.Append(ctx, tenantID, actorID, action, target, outcome, ip)` (`common/auditclient/client.go:35`) không có `actor_type`/`target_type`: nếu CR-CV-013 muốn ghi audit đọc mã nguồn kiểu "agent hay người", cần mở rộng client này (đã kiểm lại 2026-10-05: vẫn thiếu).
- SSH và remote: gateway không chạm máy dev; mọi truy cập đi qua `code-intel-service` → `infra-fleet-service`, nên giữ nguyên hành vi SSH hiện có (AGENTS.md).

## 7. Câu hỏi mở

1. Có nâng giới hạn đọc của `/ws` (ví dụ 256 KiB) để `reviewState.save` không bị 28 KiB bó không? Ảnh hưởng toàn bộ kênh.
2. `CODEINTEL_AMBIGUOUS_SYMBOL` mang `candidates` qua `status.Details` hay nhúng JSON vào thông điệp? Envelope lỗi WS chỉ có `message` (`envelope.go:63-67`); cần CR-CV-021/050 thống nhất.
3. Tách logic kiểm đường dẫn khỏi `mcpserver/tools` sang gói riêng (tên đề xuất `pathsafety`), hay nhân đôi một hàm nhỏ trong `wscompat`?
4. Cần kênh `codeIntel.bindings` (liệt kê ràng buộc, `ListRepoBindings`) không? README 3.6 có RPC nhưng 3.7 không có kênh.
5. README v7 mục 3.5 cần bảng `tenant_settings` (CR-CV-073), 3.6 cần `GetSettings`/`SetSettings`, 3.7 cần `codeIntel.subscribe` và `codeIntel.settings.*`, 3.3 cần các mã ở 2.8.
6. Tên trường chính xác (`worktreeId` so với `worktree` kiểu selector của `git.*`, `channels_git.go:51-53`) nên thống nhất với CR-CV-050; CR này dùng `worktreeId` theo README 3.x.

## 8. Tham chiếu

- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/register_production.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/registry_channels.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/session_dialect.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/push_bridge.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/envelope.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_files.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_task_activity.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_mcp.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channel_args_redaction.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/worktree_target_resolver.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_orchestration_test.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/push_bridge_test.go`
- `backend-go/services/api-gateway/internal/adapter/grpc/dial.go`, `backend-go/services/api-gateway/internal/config/config.go`, `backend-go/services/api-gateway/internal/config/config_mcp.go`, `backend-go/services/api-gateway/cmd/server/main.go`, `backend-go/services/api-gateway/README.md`
- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/parity_test.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/sensitive_path_rules.go`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/frame.go`, `backend-go/common/auditclient/client.go`, `backend-go/common/apperrors/apperrors.go`
- `deploy/dev/docker-compose.yml`
- `docs/crs/v7/README.md` mục 3.3, 3.6, 3.7; `docs/research/view-code/03-command-and-data-flow.md` mục 4; `docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md`
