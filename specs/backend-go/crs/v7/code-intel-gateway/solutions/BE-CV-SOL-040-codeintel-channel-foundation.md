# BE-CV-SOL-040-codeintel-channel-foundation: Nền kênh `codeIntel.*` ở `api-gateway`: wiring, giải mã chặt, lỗi, mã hoá JSON, giới hạn, đọc WS, parity MCP

> **Proposed.** Chưa triển khai, chưa chạy test nào. Solution đầu tiên của CR-CV-040: sau khi xong, **cả 46 tên kênh đã đăng ký vô điều kiện** (G3) và trả `CODEINTEL_UNAVAILABLE` cho tới khi ba solution kênh nối RPC thật. Ba solution còn lại của CR-CV-040 dùng các khối ở đây, không tự dựng lại.

**CR:** [CR-CV-040](../../../../../../docs/crs/v7/code-intel-gateway/CR-CV-040-api-gateway-codeintel-channels.md)
**Service:** `api-gateway` (`internal/config`, `cmd/server`, `internal/adapter/wscompat`, `internal/adapter/pathsafety` (mới), `internal/adapter/mcpserver/tools`) · `deploy/dev/docker-compose.yml`
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md) (gateway là cạnh, không logic nghiệp vụ), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (adapter không import ngược usecase), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (AuthN/AuthZ, multi-tenancy, input validation), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "API Gateway responsibilities", deadline bắt buộc), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (health, không log nội dung), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md) mục 2 (pure edge), mục 9 (security)

---

## Hợp đồng áp dụng

| Nguồn | Mục | Nội dung solution hiện thực |
|---|---|---|
| `CONTRACT-codeintel-ui-api.md` | §1 U1–U9 | một object ở `args[0]`, giải mã chặt, không `send`, định danh chỉ từ session, camelCase, mã lỗi ở tiền tố `message`, cờ tắt, từ chối phiên thiết bị, chuỗi không tin cậy |
| | §2.1–2.5 | `WorktreeSel` phẳng, `CodeIntelEnvelope`, bảng lỗi hợp nhất (PQ-03), timeout/giới hạn gateway, quyền mức service |
| | §3 | danh mục **46 kênh** (45 unary + 1 stream) là đầu vào của `codeIntelChannelCatalog` |
| | §5, §6 | `CODE_INTEL_SERVICE_ADDR` rỗng => mọi kênh `CODEINTEL_UNAVAILABLE`; `CODE_INTEL_MAX_STREAMS` |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-02 | bốn chặng mã lỗi; hàm `codeIntelChannelError` (mẫu `mcpChannelError`) |
| | PQ-03 | tập mã hợp nhất; `NotFound|PermissionDenied` => `CODEINTEL_NOT_AUTHORIZED` |
| | PQ-04 | `{projectId, worktreeId}` trên mọi kênh gắn worktree |
| | PQ-12 | phong bì phẳng, `etag`, `notModified` (encoder ở TASK-040-05, 06) |
| | PQ-13 | gateway đọc 20 s, ghi/trạng thái 8 s; hậu tố `{"retryAfterMs":3000,"inProgress":true}` khi `DeadlineExceeded` do gateway |
| | PQ-14 | `conn.SetReadLimit(320<<10)`, giới hạn `args[0]` theo kênh, phản hồi ≤ 2 MiB (`symbol` 320 KiB), `MaxCallRecvMsgSize(4 MiB)` |
| | PQ-23 | tên biến `CODE_INTEL_SERVICE_ADDR`, `CODE_INTEL_MAX_RESPONSE_BYTES`, `CODE_INTEL_MAX_STREAMS` |
| | PQ-24, PQ-01 | gateway **không** giữ cờ; chỉ chuyển `CODEINTEL_DISABLED`/`CODEINTEL_QUALITY_GATE_DISABLED` |
| | PQ-32 | quy tắc chữ hoa/thường enum trong bộ mã hoá |
| | §3 (mở đầu), §6.2, §7.1 G3, §8.3, §9 O-2/O-16, §10 (dòng `wscompat`, `excluded_channels.yaml`) | token nội bộ, env, cổng G3, kiểm tra chéo, điểm mở, việc ở code hiện có |

## Lệch giữa CR và hợp đồng

| # | CR-CV-040 nói | Hợp đồng nói | Theo |
|---|---|---|---|
| L1 | 26 kênh (25 unary + 1 stream); `quality.*` không có | 46 kênh (45 + 1) với 20 kênh `quality.*` (PQ-27) | hợp đồng |
| L2 | kênh chỉ nhận `worktreeId` (2.4) | `{projectId, worktreeId}` (PQ-04) | hợp đồng |
| L3 | `NotFound|PermissionDenied` => `CODEINTEL_NOT_FOUND` (2.8) | => `CODEINTEL_NOT_AUTHORIZED`; `NOT_FOUND` chỉ cho id con (job, run, waiver, turn) và Identity rỗng (UI-API 2.3, PQ-03 điểm 5) | hợp đồng |
| L4 | trần `args[0]` 28 KiB (dưới mặc định 32 KiB của thư viện), `findingKey` ≤ 256, `document` ≤ 24 KiB | `SetReadLimit(320<<10)`; `reviewState.save` ≤ 256 KiB, `c4.save` ≤ 96 KiB (`document` ≤ 64 KiB), `findingKey` ≤ 128, còn lại ≤ 16 KiB (UI-API 2.4, PQ-14) | hợp đồng |
| L5 | `candidates` qua `status.Details` hoặc JSON sau `|` (Q2) | luôn hậu tố `" \| {json}"` trong `message`, không `status.Details` (PQ-02) | hợp đồng |
| L6 | hàm tên `mapCodeIntelError` | `codeIntelChannelError` | hợp đồng |
| L7 | kết quả lớn qua `payloadJson` hoặc field có kiểu "do CR-020/050 chốt" (2.2) | proto có kiểu (ResultMeta + `data` kiểu thật, proto-and-data-map 2.1/2.3); **không nêu cách ra JSON** (enum, `int64`, `Timestamp`, `optional`) | giải pháp tại 2.5 và Câu hỏi mở Q1 |
| L8 | chỉ ba biến môi trường | thêm: gateway phải dial với token nội bộ vì mọi RPC `code-intel-service` bị chặn khi token rỗng (proto-and-data-map §3 mở đầu, `CODEINTEL_INTERNAL_CALLER_TOKEN`) | hợp đồng, tên biến chốt ở 2.1 |
| L9 | `CODE_INTEL_SERVICE_ADDR` thêm vào `OtherServiceAddrs` | `main.go` dial mọi phần tử `OtherServiceAddrs` ngay (không điều kiện) | tách `config_codeintel.go` theo mẫu `config_mcp.go`, dial có điều kiện |

## Phụ thuộc chéo khu vực

| Hướng | Solution | Điều kiện |
|---|---|---|
| BE trước | `BE-CV-SOL-010-scaffold-code-intel-service` | proto `codeintel.proto`/`codeintel_common.proto` sinh stub (cổng G0); service name `code-intel-service:9090` trong compose; `apperrors` Kind mới (không dùng ở gateway) |
| BE trước | `BE-CV-SOL-020-canonical-graph-model` | `WorktreeSelector`, `ResultMeta`, `SymbolRef` để mã hoá envelope |
| BE trước (token) | `BE-CV-SOL-013-agent-call-gate-and-quotas` / `BE-CV-SOL-010` | tên và cách đặt `CODEINTEL_INTERNAL_CALLER_TOKEN` ở phía service |
| BE sau | `BE-CV-SOL-040-codeintel-view-channels`, `-write-and-stream-channels`, `-quality-channels` | dùng runner, args, lỗi, encoder, catalog |
| BE sau | `BE-CV-SOL-041-mcp-codeintel-tools` | thay dòng loại trừ; dùng `pathsafety` |
| FE | `FE-CV-SOL-050-types-and-runtime-bridge` | đọc tiền tố `message`; không phụ thuộc code ở đây nhưng cần cùng bảng mã (UI-API 2.3) |
| AG | không có | gateway không chạm agent |

Thứ tự §7.1: cổng **G3** = solution này (phần nền) xong trước FE-CV-SOL-050 dùng kênh thật.

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06, toàn bộ trong `backend-go/services/api-gateway/` trừ khi ghi khác): `internal/adapter/wscompat/{registry.go (1-290), handler.go (100-260, 340-411), push_bridge.go, channels_push.go (40-188), channels_mcp.go (60-140), channels_mcp_approval.go (100-115), register_production.go, session_dialect.go (50-125), envelope.go (55-100), channel_args_redaction.go (1-40), registry_channels.go, file_watch_stream_registry.go (1-70), channels.go (95-110), channels_test.go (1207-1224)}`, `internal/adapter/grpc/dial.go`, `internal/config/{config.go (96-185), config_mcp.go (1-50)}`, `cmd/server/{main.go (160-265, 355-460, 540-575, 640-670), mcp_governance_wiring.go (24-32), mcp_dial_token_test.go}`, `internal/adapter/mcpserver/tools/{all_specs.go, parity_test.go (1-200), excluded.go (1-80), excluded_channels.yaml (đầu, 385-420), sensitive_path_rules.go, sensitive_path_rules_test.go (tên test), files_sensitive_guard.go, executor.go (379-421)}`, `README.md` (mục), `deploy/dev/docker-compose.yml` (60-75), `backend-go/common/internalcaller/internalcaller.go` (tên hàm), `backend-go/go.mod`/`api-gateway/go.mod` dòng `coder/websocket v1.8.15`.

Xác nhận đúng:
- Chưa có file `channels_codeintel*.go`, client `codeintelv1`, thư mục `proto/orca/codeintel` (`ls backend-go/proto/orca` không có `codeintel`).
- `ChannelHandler func(ctx, Identity, args []json.RawMessage) (any, error)`; `Dispatch` (`registry.go:172`) gắn `AttachIdentity` **có `Role`** và trần `dispatchRPCTimeout = 60 s` (`:164`); `handler.go:247` `invokeTimeout = 25 s`.
- `decodeArg[T]` (`registry.go:267`) dùng `json.Unmarshal` lỏng, **không** từ chối khoá lạ.
- `normalizeNilSlices` (`registry.go:227`) trả nguyên `proto.Message`, chỉ sửa slice cấp đầu của struct giá trị; nên view phải tự bảo đảm `[]`.
- Đường stream (`handler.go:228` `StreamHandlerFor` => `handleSubscribe`) **không** qua `Dispatch` nên không có `AttachIdentity`; mẫu `workspacePorts.subscribe` (`channels_push.go:81`) gắn tay và **thiếu `Role`** (`usecase.Identity{TenantID, UserID}`).
- `writeDialectError` (`session_dialect.go:94`) ghi `code:"internal"` + `err.Error()`; native ghi `ErrorMessage{Message: err.Error()}` (`envelope.go`).
- `websocket.Accept` ở `handler.go:113`; **không** có `SetReadLimit` ở bất kỳ file Go nào của `api-gateway` (`grep -rn SetReadLimit` trên `api-gateway` không có kết quả; `grep ReadLimit` chỉ trả `embeddedReadLimit` ở `prompts/provider.go`, không liên quan).
- `gatewaygrpc.Dial` (`dial.go:31`) chỉ đặt credentials + `otelgrpc`; không `MaxCallRecvMsgSize`. MCP dial có token: `dialMCPService` (`mcp_governance_wiring.go:24`) thêm `internalcaller.ClientInterceptor`/`StreamClientInterceptor` khi `MCP_INTERNAL_CALLER_TOKEN` khác rỗng.
- `main.go:162-260` dial mọi giá trị `cfg.OtherServiceAddrs[...]` bằng `gatewaygrpc.Dial` không điều kiện (khác `mcpClient` ở `:387-398`, chỉ dial khi có địa chỉ).
- `parity_test.go:13-19` dựng inventory bằng `RegisterProductionChannels(r, ChannelDeps{TaskActivityEnabled: true})` + `RegisterMcpChannels(r, McpChannelDeps{})`; `TestChannelInventory` (`:38`) đỏ khi kênh không có `ToolSpec` lẫn dòng loại trừ hoặc có cả hai; `TestToolParity` (`:79`) đỏ khi dòng loại trừ không khớp kênh nào.
- `Exclusion.Match` (`excluded.go:51`): mẫu `codeIntel.*` khớp mọi cấp (nhánh `HasSuffix(".*")` ở cuối hàm), kể cả `codeIntel.reviewState.get`, `codeIntel.quality.turn.record`.
- `CleanWorktreePath` nằm trong `package tools` (`sensitive_path_rules.go:27`); `mcpserver/resources/uri.go:117,122` gọi `tools.CleanWorktreePath`; `tools` import `wscompat` (`spec.go:15`) nên `wscompat` **không** được import `tools`.
- Bất biến thời gian hiện có: `rpcTimeout (8 s) < invokeTimeout (25 s)` và biên ghi >= 5 s (`channels_test.go:1212`).
- `channels_mcp_approval.go:109` là tiền lệ đọc `id.DeviceID`.

### Correction relative to CR-CV-040

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | "dial khi địa chỉ khác rỗng theo mẫu MCP" và "thêm vào `OtherServiceAddrs`" | `OtherServiceAddrs` được dial hết, không điều kiện (`main.go:162-260`); dial địa chỉ rỗng bằng `grpc.NewClient("")` là hành vi chưa kiểm chứng | tách `CodeIntelConfig` (mẫu `config_mcp.go`) và hàm `dialCodeIntelService`, chỉ gọi khi địa chỉ khác rỗng |
| C2 | `MaxCallRecvMsgSize(4<<20)` "đặt tường minh, hiện đã là mặc định" | `Dial` không đặt; mặc định grpc-go 4 MiB là kiến thức thư viện (chưa chạy thử) | đặt tường minh trong `dialCodeIntelService`, test bằng server giả trả 5 MiB |
| C3 | handler stream "gắn `AttachIdentity` có `Role`" như điểm khác biệt | đúng, nhưng còn thiếu một điều CR không nêu: `handleSubscribe` truyền **ctx kết nối** (không timeout) và ack ghi trước khi biết stream mở được; lỗi quyền chỉ lộ ở `Recv` đầu | xử lý ở `BE-CV-SOL-040-codeintel-write-and-stream-channels` |
| C4 | `decodeCodeIntelArgs` "lỗi nêu tên trường" | `DisallowUnknownFields` của `encoding/json` trả `json: unknown field "x"`; tên trường nằm trong chuỗi lỗi chung | quét khoá cấp một trước, báo tên trường bằng mã `CODEINTEL_INVALID_PARAMS` + `data.field` |
| C5 | "`wscompat` không được import ngược; đề xuất `internal/adapter/pathsafety`" | đúng (`spec.go:15`) | thực hiện; `tools` giữ hàm bọc để `resources/uri.go` không đổi |

---

## 2. Giải pháp

### 2.1 Wiring client và cấu hình (mới)

`internal/config/config_codeintel.go` (mới, theo `config_mcp.go`):

```go
// CodeIntelConfig is the api-gateway slice of the code-intel feature (CR-CV-040).
type CodeIntelConfig struct {
    ServiceAddr      string // CODE_INTEL_SERVICE_ADDR; "" => never dial, every channel answers CODEINTEL_UNAVAILABLE
    MaxResponseBytes int    // CODE_INTEL_MAX_RESPONSE_BYTES, default 2<<20 (PQ-14)
    MaxStreams       int    // CODE_INTEL_MAX_STREAMS, default 500 per replica (PQ-23)
}
```
`Config` thêm trường `CodeIntel CodeIntelConfig`; `Load()` gọi `loadCodeIntel()`; giá trị sai (không phải số nguyên dương) trả lỗi khởi động giống `loadMCP`. `MaxResponseBytes` bị kẹp tối đa **3 MiB** (dưới `MaxCallRecvMsgSize` 4 MiB): cao hơn thì lỗi cấu hình.

`cmd/server/codeintel_wiring.go` (mới):

```go
func dialCodeIntelService(addr string) (*grpc.ClientConn, error) {
    opts := []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
        grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4 << 20)), // PQ-14: explicit, so nobody raises it by accident
    }
    if tok := os.Getenv("CODEINTEL_INTERNAL_CALLER_TOKEN"); tok != "" {
        opts = append(opts, grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(tok)),
            grpc.WithChainStreamInterceptor(internalcaller.StreamClientInterceptor(tok)))
    }
    return grpc.NewClient(addr, opts...)
}
```
`main.go`: nếu `cfg.CodeIntel.ServiceAddr != ""` dial rồi dựng **hai** client (`codeintelv1.NewCodeIntelServiceClient`, `codeintelv1.NewQualityGateServiceClient`); ngược lại để hai biến interface ở giá trị `nil` thật và log một cảnh báo. Không gán con trỏ nil kiểu cụ thể vào interface. `healthSrv.Register("code-intel-service", grpcConnHealthCheck(conn))` chỉ khi đã dial (mẫu `mcpConn`, `main.go:569`). `ChannelDeps` thêm `CodeIntel codeintelv1.CodeIntelServiceClient`, `QualityGate codeintelv1.QualityGateServiceClient`, `CodeIntelLimits CodeIntelLimits{MaxResponseBytes, MaxStreams int}`; `RegisterProductionChannels` gọi `registerCodeIntelChannels(r, d)` ở **cuối** (sau `RegisterMobileChannels`), vô điều kiện.

### 2.2 `pathsafety` (mới)

`internal/adapter/pathsafety/worktree_path.go`: chuyển nguyên `ErrUnsafePath`, `CleanWorktreePath`, `checkPathForm`, `doubleEncoded` từ `tools/sensitive_path_rules.go` (không đổi hành vi; NFKC trên `golang.org/x/text/unicode/norm` đã có trong `go.mod`). `tools/sensitive_path_rules.go` còn `IsSensitivePath`, `containsPrivateKey` và hai khai báo mỏng `var ErrUnsafePath = pathsafety.ErrUnsafePath`, `func CleanWorktreePath(raw string) (string, error) { return pathsafety.CleanWorktreePath(raw) }` để `resources/uri.go` và `files_sensitive_guard.go` không đổi. Test `TestCleanWorktreePath_RejectsTraversalAndTricks` chuyển sang `pathsafety`. Đây là quyết định O-16 (tên gói cụ thể, không `utils`).

Lưu ý hành vi: `CleanWorktreePath` từ chối chuỗi rỗng, `.` và dấu `/` cuối; gateway chỉ gọi khi trường **có mặt và khác rỗng**; "gốc repo" = bỏ trường.

### 2.3 Cây file `wscompat` (mới, giữ mỗi file < 400 dòng)

```
channels_codeintel_catalog.go        # codeIntelChannelSpec + var codeIntelChannelCatalog (46) + hằng timeout/giới hạn
channels_codeintel_args.go           # decodeCodeIntelArgs[A], codeIntelParamError, forbidden keys, validators dùng chung
channels_codeintel_args_selector.go  # codeIntelSelector (projectId, worktreeId), refs git, enum, RFC 3339
channels_codeintel_errors.go         # codeIntelChannelError, scrub, one-line, candidates trim
channels_codeintel_wire_encoder.go   # protoreflect => camelCase JSON (enum, int64, Timestamp, optional, nullable)
channels_codeintel_wire_enums.go     # bảng ghi đè enum => chuỗi dây (PQ-32)
channels_codeintel_envelope.go       # encodeEnvelope(ResultMeta, view, data, nextPageToken)
channels_codeintel_runner.go         # registerCodeIntelUnary[A], guards, timeout, trần phản hồi
channels_codeintel_register.go       # registerCodeIntelChannels, placeholder cho kênh chưa nối
channels_codeintel_*_test.go
```
`handler.go` thêm `conn.SetReadLimit(codeIntelWSReadLimit)`.

### 2.4 Catalog 46 kênh và runner

```go
type codeIntelChannelSpec struct {
    Name         string        // "codeIntel.reviewState.get"
    Stream       bool          // chỉ "codeIntel.subscribe"
    Timeout      time.Duration // 8 s | 20 s | 24 s (quality.summary); 0 cho stream
    MaxArgsBytes int           // 16<<10 mặc định; 256<<10, 96<<10, 8<<10 theo UI-API 2.4
    AllowDevice  bool          // true chỉ cho codeIntel.settings.get (U8)
    Quality      bool          // dùng QualityGateService
    MaxResponse  int           // 0 = cfg.MaxResponseBytes; symbol = 320<<10
    RPC          string        // tài liệu + test: "CodeIntelService/GetSymbol"
}
```
Bảng đầy đủ ở [`tasks/README.md`](../tasks/README.md) mục "Kênh -> solution -> task".

Runner (generic, một đường cho cả 45 kênh unary):

```go
type codeIntelArgs interface{ validate() error } // trả *codeIntelParamError

func registerCodeIntelUnary[A codeIntelArgs](r *Registry, d codeIntelDeps, name string,
    call func(ctx context.Context, c codeIntelCaller, id Identity, in A) (any, error)) {
    spec := mustCatalogSpec(name)
    r.Register(name, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        if !d.configured(spec) { return nil, errCodeIntelUnavailable }              // 1. client nil
        if id.TenantID == "" || id.UserID == "" { return nil, errCodeIntelNotFound } // 2. phòng thủ
        if id.DeviceID != "" && !spec.AllowDevice { return nil, errCodeIntelNotAuthorized } // 3. U8
        in, err := decodeCodeIntelArgs[A](spec, args)                                 // 4. cỡ, khoá, kiểu, validate
        if err != nil { return nil, err }
        cctx, cancel := context.WithTimeout(ctx, spec.Timeout); defer cancel()        // 5. 8/20/24 s
        out, err := call(cctx, d.caller(), id, in)
        if err != nil { return nil, codeIntelChannelError(err) }
        return out, nil                                                               // 6. đã qua encoder + trần
    })
}
```
Thứ tự 1 trước 2–4 để gateway chưa cấu hình luôn trả `CODEINTEL_UNAVAILABLE` (U7/6: client ẩn tính năng, không toast). `call` nhận kết quả proto, kiểm `proto.Size(resp)` so với `spec.MaxResponse` rồi gọi encoder (mục 2.5) và trả `json.RawMessage` (không bị `normalizeNilSlices` đụng tới vì không phải `proto.Message`, và slice byte không nil).

`registerCodeIntelChannels(r, d)`: chạy lần lượt `registerCodeIntelViewChannels`, `registerCodeIntelStateChannels`, `registerCodeIntelSubscribe`, `registerCodeIntelQualityChannels` (do ba solution kia thêm); sau đó với **mỗi kênh của catalog chưa có trong `r.Channels()`** đăng ký placeholder: unary trả `CODEINTEL_UNAVAILABLE: channel not wired`, stream trả lỗi cùng mã. Nhờ đó mỗi PR của solution sau chỉ thay một nhóm, và `TestCodeIntelChannelInventory` luôn thấy 46 tên.

### 2.5 Mã hoá JSON từ proto (điểm hợp đồng thiếu)

Hợp đồng chốt shape TS (UI-API §4) và proto có kiểu (proto-and-data-map §2.1, §2.3) nhưng **không nói cách chuyển** `proto -> JSON`; `protojson` mặc định (`UseProtoNames:false`) cho tên camelCase nhưng: enum ra `FUNCTION`/`INDEX_SCOPE_EXACT` (PQ-32 đòi chữ thường trừ `risk.level`, `overall`), `int64` ra chuỗi, `optional` vắng thì bỏ khoá (TS có `number | null`), `Timestamp` đúng RFC 3339. Giải pháp: encoder do gateway sở hữu, dựa `protoreflect`, một lần cho mọi view:

- tên khoá = `FieldDescriptor.JSONName()`; slice (`repeated`) luôn mảng (rỗng `[]`), `map` luôn object (`{}`);
- `int32/int64/uint*` ra số (vượt 2^53-1 => `CODEINTEL_RESULT_INVALID`); `google.protobuf.Timestamp` ra RFC 3339 UTC;
- enum: tên giá trị bỏ tiền tố `<TÊN_ENUM>_` rồi chữ thường, `*_UNSPECIFIED` hoặc số lạ => `"unknown"` (U4); bảng ghi đè `codeIntelEnumWireOverrides` (ở `channels_codeintel_wire_enums.go`) cho enum HOA (`Risk`) và giá trị có gạch ngang/gạch dưới không suy được; mỗi mục bảng có test đối chiếu mô tả proto khi proto có mặt;
- trường `| null` của UI-API §4 liệt kê trong `codeIntelNullableFields` (khoá `"<FullMessageName>.<field>"`): vắng => `null`; trường khác vắng => bỏ khoá;
- trường `ResultMeta.dev_server_id` và mọi trường trong `codeIntelNeverOnWire` **không** ra (đường dẫn tuyệt đối, id dev server);
- không gọi `protojson`+`json.Unmarshal` rồi `json.Marshal` lại (CR 2.6: không nhân đôi bộ nhớ); ghi trực tiếp vào `bytes.Buffer`.

`encodeEnvelope(meta, view, data, nextPageToken)` dựng `CodeIntelEnvelope` (UI-API 2.2): `repo?`, `worktreeId`, `view`, `sources[]` (`indexedAt`/`commit` rỗng => `null`; `lineBase:1` khi proto > 0), `headCommit|null`, `stale`, `truncated`, `totalCount`, `etag`, `fromCache`, `generatedAt`, `notModified?` (khi đó **không** có `data`), `nextPageToken?`.

Nếu Q1 được chốt theo hướng "service trả JSON sẵn", chỉ thay `channels_codeintel_wire_encoder.go`; runner, envelope và test hợp đồng không đổi.

### 2.6 Lỗi: `codeIntelChannelError`

Theo PQ-02 chặng 4 và UI-API 2.3. Quy trình: (1) `status.FromError` => `st.Message()`; (2) bóc tiền tố `rpc error: code = X desc = ` (lần xuất hiện cuối); (3) nếu khớp `(?s)^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$`: phần người đọc => một dòng, ≤ 200 rune, che đường dẫn tuyệt đối (`/a/b/c...` hoặc `X:\a\b\c...` từ 3 phân đoạn => `<path>`), phần JSON: phải là object hợp lệ ≤ 2 KiB (nếu vượt và có `candidates`, bỏ dần phần tử cuối tới đủ; còn lại bỏ hậu tố); (4) không khớp: ánh xạ theo `status.Code` bảng UI-API 2.3 (`Unavailable|Unimplemented|nil` => `UNAVAILABLE`; `DeadlineExceeded|context.DeadlineExceeded` => `TIMEOUT` kèm hậu tố `{"retryAfterMs":3000,"inProgress":true}`; `NotFound|PermissionDenied` => `NOT_AUTHORIZED`; `InvalidArgument` => `INVALID_PARAMS`; `FailedPrecondition` => `TOOL_FAILED`; `ResourceExhausted` => `RATE_LIMITED` hoặc `RESPONSE_TOO_LARGE` nếu thông điệp nhắc kích thước; `Aborted|AlreadyExists` => `VERSION_CONFLICT`; `Canceled` => `TIMEOUT` không hậu tố; còn lại `INTERNAL: internal error`), thông điệp là **hằng**, không chép lại thông điệp gốc. Không đưa kênh `codeIntel.*` vào `sensitiveArgChannels` (xem Q5).

### 2.7 Giới hạn kích thước và đọc WS

- `handler.go` ngay sau `websocket.Accept`: `conn.SetReadLimit(codeIntelWSReadLimit)` với `const codeIntelWSReadLimit = 320 << 10` (PQ-14, O-2: chạm **mọi** kênh của `/ws`). Khung vượt giới hạn làm thư viện đóng kết nối (hành vi theo tài liệu thư viện, chưa kiểm chứng: mã nguồn module không có trong cache); vì thế các trần theo kênh (16/8/96/256 KiB) phải thấp hơn 320 KiB và được kiểm **trước** khi giải mã.
- `args[0]` theo `spec.MaxArgsBytes`; chuỗi/mảng theo UI-API 2.4 (`reason`/`note` ≤ 500, `findingKey` ≤ 128, `key` ≤ 1024, `pageToken` ≤ 512, `kinds` ≤ 32); `depth`/`limit` ngoài khoảng bị từ chối, không kẹp.
- Phản hồi: `proto.Size(resp) > spec.MaxResponse` => `CODEINTEL_RESPONSE_TOO_LARGE | {"bytes":N,"limit":L}`.
- Hằng thời gian đặt cạnh catalog: `codeIntelReadTimeout = 20 s`, `codeIntelStateTimeout = 8 s`, `codeIntelSummaryTimeout = 24 s`; test `TestCodeIntelTimeouts_ShorterThanInvokeTimeout` theo mẫu `channels_test.go:1212`.

### 2.8 Parity MCP và tài liệu

- `excluded_channels.yaml`: **một** dòng `pattern: "codeIntel.*"`, `category: code-intel-v1`, `reason` >= 20 ký tự, **cùng PR** với `registerCodeIntelChannels` (nếu không `TestChannelInventory` đỏ). Nó phủ cả `codeIntel.quality.*`.
- Test mới `TestCodeIntelChannelInventory` (trong `wscompat`): dựng registry rỗng + `registerCodeIntelChannels`, khẳng định đúng 46 tên của hợp đồng (liệt kê cứng), 45 `ChannelUnary` + 1 `ChannelStream`; đây là lưới chống lệch catalog so với UI-API §3.
- `README.md` của gateway: mục "Code intelligence channels (CR-CV-040)": downstream `code-intel-service`, ba biến môi trường + `CODEINTEL_INTERNAL_CALLER_TOKEN`, 46 kênh, `SetReadLimit`.
- `deploy/dev/docker-compose.yml`: `CODE_INTEL_SERVICE_ADDR: code-intel-service:9090` cạnh `MCP_SERVICE_ADDR` (dòng 71) **sau khi** `BE-CV-SOL-010` thêm service; trước đó để trống.

---

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Catalog 46 kênh là dữ liệu, một runner generic | bốn solution không lặp guard/timeout/trần; test hợp đồng đếm được |
| D2 | Placeholder cho kênh chưa nối | đạt G3 ngay; không có đường nào làm gateway trả `not yet implemented` |
| D3 | Kiểm `client == nil` **trước** mọi kiểm khác | gateway chưa cấu hình luôn cho client tín hiệu `unsupported` |
| D4 | Quét khoá cấp một trước khi giải mã chặt | báo đúng tên trường cấm, không rò giá trị |
| D5 | Encoder `protoreflect` của gateway | hợp đồng thiếu quy tắc ra JSON; tránh viết tay 60 kiểu mirror |
| D6 | `CodeIntelConfig` riêng, dial có điều kiện | `OtherServiceAddrs` bị dial hết |
| D7 | `SetReadLimit(320 KiB)` toàn `/ws` | PQ-14; nếu không có thì `reviewState.save` 256 KiB không bao giờ tới handler |
| D8 | Lỗi ánh xạ theo hằng, không chép thông điệp gốc | D8/G6: không mã nguồn, đường dẫn, tham số |
| D9 | `pathsafety` gói riêng, `tools` giữ hàm bọc | không import vòng; không đổi `resources/uri.go` |
| D10 | Không retry ở gateway cho bất kỳ kênh nào | kênh ghi không idempotent; `CODEINTEL_TIMEOUT` có `inProgress:true` cho client tự thử lại |

## 4. Phụ thuộc và thứ tự

TASK-040-01 (config, dial) và 040-02 (pathsafety) song song; 040-03, 040-04, 040-05 song song được (độc lập); 040-06 (envelope + runner) cần 03, 04, 05; 040-07 (catalog + đăng ký + placeholder) cần 01, 06; 040-08 (read limit, bất biến timeout) độc lập sau 01; 040-09 (parity, README, compose) cuối. Chi tiết [tasks/README](../tasks/README.md).

## 5. Kiểm thử

- Unit: `decodeCodeIntelArgs` (khoá lạ, khoá cấm `tenantId|userId|deviceId|role|devServerId|workspaceRoot|repo|args|command|cypher`, `len(args) > 1`, `args[0]` không phải object, `args` rỗng, `{}`, cỡ vượt, kiểu sai); `codeIntelChannelError` bảng (mỗi `status.Code`, mã `CODEINTEL_*` đi qua, đường dẫn bị che, nhiều dòng, > 200 rune, hậu tố JSON hỏng/quá lớn, `candidates` cắt dần); encoder (enum, `int64`, `Timestamp`, `optional`, nullable, map rỗng, `devServerId` không ra); envelope (`notModified` không `data`); runner (thứ tự guard, `DeviceID`, timeout đặt đúng, `Unimplemented` => `UNAVAILABLE`, trần phản hồi); timeouts; `pathsafety` (vector hiện có + full-width `．．`, `%2e%2e`, `\`, `/abs`).
- Hai dialect gateway (native + session-client): kết quả `json.RawMessage` đi qua `writeDialectResult`; lỗi hiện ở `message` của cả hai.
- Cô lập tenant: fake client ghi lại metadata gRPC; khẳng định `x-orca-tenant-id`/`x-orca-user-id`/`x-orca-role` lấy từ `Identity`, không từ `args` (gửi `tenantId` giả => `INVALID_PARAMS`).
- `go test ./internal/adapter/wscompat/... ./internal/adapter/pathsafety/... ./internal/adapter/mcpserver/... ./cmd/server/... ./internal/config/...`; `buf`/proto không thuộc solution này.
- Chưa chạy bất kỳ test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Giới hạn đọc mặc định của `coder/websocket` v1.8.15** chưa kiểm chứng (không có mã module trong cache); hành vi khi khung vượt giới hạn (đóng kết nối, client thấy rớt thay vì `CODEINTEL_INVALID_PARAMS`) cũng theo tài liệu thư viện.
- `SetReadLimit` cao hơn làm tăng bộ nhớ tối đa mỗi kết nối (một khung 320 KiB); chưa đo; cần O-2 duyệt.
- `MaxCallRecvMsgSize` ở client nhưng **server** `code-intel-service` cũng có `MaxSendMsgSize` mặc định `MaxInt32`; trần 2 MiB của service (PQ-14) là kỷ luật phía service.
- JSON có thể lớn hơn `proto.Size` nhiều lần (tên khoá lặp); chưa có trần JSON trong hợp đồng; đo ở CR-071.
- Encoder phụ thuộc proto chưa tồn tại: bảng `codeIntelNullableFields`, `codeIntelEnumWireOverrides` chỉ kiểm chứng được khi `codeintel_*.proto` có mặt (cổng G0). Tới lúc đó test dùng message proto tổng hợp (`dynamicpb`).
- `internalcaller` token rỗng ở dev: service chặn hết RPC (proto-and-data-map §3); thiếu biến ở gateway = mọi kênh `CODEINTEL_UNAVAILABLE`/`PermissionDenied` mơ hồ; log cảnh báo khi `ServiceAddr` có nhưng token rỗng.
- Phiên SSH/remote: gateway không chạm dev server (D2 của feature); không có nhánh riêng.

## 7. Câu hỏi mở

- **Q1.** Quy tắc `proto -> JSON` (enum, `int64`, `optional`, nullable): encoder ở gateway (đề xuất này) hay service trả JSON dựng sẵn? Cần chủ sở hữu CR-020/021 chốt trước khi viết `codeintel_graph.proto`.
- **Q2.** Tên biến token nội bộ phía gateway: dùng chung `CODEINTEL_INTERNAL_CALLER_TOKEN` với service (đề xuất) hay `CODE_INTEL_INTERNAL_CALLER_TOKEN` theo tiền tố gateway? Hợp đồng §6.2 không liệt kê.
- **Q3 (O-2).** Duyệt `SetReadLimit(320 KiB)` toàn `/ws`.
- **Q4.** Trần JSON đầu ra (xem rủi ro) có cần không.
- **Q5.** Thêm `codeIntel.quality.turn.record` (trường `promptExcerpt`) vào `sensitiveArgChannels`: chi phí thấp, tránh lọt qua mọi đường ghi log tương lai; CR-CV-040 nói không thêm vì "không phải secret".

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `CONTRACT-codeintel-proto-and-data-map.md`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/{registry.go,handler.go,push_bridge.go,register_production.go,channels_push.go,channels_mcp.go,session_dialect.go,envelope.go}`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/grpc/dial.go`, `cmd/server/{main.go,mcp_governance_wiring.go}`, `internal/config/{config.go,config_mcp.go}`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/mcpserver/tools/{parity_test.go,excluded.go,excluded_channels.yaml,sensitive_path_rules.go}`
- `/opt/repos/orca/backend-go/common/internalcaller/internalcaller.go`
- `/opt/repos/orca/specs/backend-go/crs/v6/gateway-and-mcp/solutions/BE-REQ-SOL-016-api-gateway-request-channels.md` (mẫu)
