# BE-MCP-SOL-003: Streamable HTTP transport, vòng đời `initialize`, kênh `mcp.server.info`

> **🔲 Designed — chưa implement.** Phụ thuộc [BE-MCP-SOL-002](../../mcp-service-foundation/solutions/BE-MCP-SOL-002-gateway-mcp-endpoint-wiring.md) (khung `/mcp`, `TokenVerifier`, `McpChannelDeps`). Session bền + SSE ở [BE-MCP-SOL-004](./BE-MCP-SOL-004-sessions-sse-resumability.md).

**CR:** [CR-MCP-003](../../../../../../docs/crs/v5/mcp-protocol-server/CR-MCP-003-streamable-http-and-lifecycle.md)
**Service:** `api-gateway` (`internal/adapter/mcpserver`, `internal/adapter/wscompat/channels_mcp_server_info.go`); gọi `mcp-service.GetServerInfo` (BE-001)
**TDD tham chiếu:** [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §2–§3, §6 · [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) · **Hợp đồng FE:** [CONTRACT](../../CONTRACT-mcp-ui-api.md) §1 `McpServerInfo`, §2.1 `mcp.server.info`, §3 `/mcp`

---

## 1. Trạng thái hiện tại (re-verify — xem [README](./README.md))

- Không có JSON-RPC/MCP nào ở `backend-go` (`grep -ri modelcontextprotocol backend-go` → 0 trong `go.mod`). `agent/src/relay/agent-rpc-dispatch-misc.ts` chỉ có `tools/list`/`tools/call` trên kênh relay riêng, không có handshake.
- Seam do BE-002 để lại: `mcpserver.Deps.Transport http.Handler` (nil ⇒ 501), `Principal` trong `context`, `Recorder`.
- `wscompat` trả lỗi bằng chuỗi (`err.Error()`); `mcp.server.info` là kênh duy nhất của CONTRACT mà FE gọi **trước** khi biết MCP bật hay không ⇒ phải luôn đăng ký và không bao giờ trả lỗi khi tắt (C8).

### Quyết định khác/thêm so với CR gốc

1. **Seam `ProtocolEngine`; SDK đã chốt, spike chỉ kiểm hook và ghi khoảng trống.** **Đã chốt (D4, 2026-10-01):** dùng **MCP Go SDK chính thức** (`github.com/modelcontextprotocol/go-sdk`) cho server; client kiểm thử/tham chiếu trong CI là **MCP Python SDK chính thức** (BE-015 §A.2). Không còn câu hỏi "SDK hay tự viết"; API cụ thể của go-sdk vẫn **chưa xác minh** (không có trong `go.mod`) nên spike (§B) xác minh các hook Streamable HTTP/session-store và ghi vào ADR (`docs/adrs/v2/ADR-021-mcp-go-sdk-and-transport.md` — số tạm) những chỗ SDK **thiếu** để vá bằng lớp mỏng của ta. Các hook cần có: SDK dùng được cho *transport* nếu (a) cho tiêm `SessionStore` ngoài (BE-004 cần session ở DB + cross-replica), (b) cho tiêm `EventStore`/sinh `id:` SSE tuỳ chỉnh và đọc `Last-Event-ID`, (c) cho kiểm danh tính theo request trước khi vào handler. Nếu một hook thiếu: vẫn dùng SDK cho kiểu/mã hoá JSON-RPC và phần còn lại, **chỉ** vá phần thiếu (trường hợp xấu nhất: tự viết lớp transport ≈600 dòng phía sau cùng `ProtocolEngine`) — thiết kế dưới đây không đổi, và quyết định SDK không bị mở lại.
2. **Session lưu qua cổng `SessionStore`**: BE-003 cài `memorySessionStore` (một replica, dùng cho test và dev); BE-004 thay bằng `grpcSessionStore`. Nhờ vậy BE-003 ship độc lập và client tham chiếu (Python SDK/Inspector) chạy được ở 1 replica.
3. **Quảng bá capability theo handler thực có**, không theo CR (CR liệt kê `tools/resources/prompts` trước khi CR-007/010/011 tồn tại ⇒ khai báo sai làm client gọi method chưa có). `CapabilityBuilder.Register("tools", …)` do pack tương ứng gọi; BE-003 đăng ký `tools` với catalog rỗng (`listChanged:false`) và `logging`; BE-007 bật `listChanged:true`; BE-010/011 thêm `resources`/`prompts`.
4. **Thiếu header `MCP-Protocol-Version` ⇒ dùng phiên bản đã thương lượng của session** (spec cho phép "cách khác để xác định phiên bản"), không bao giờ suy ra từ một hằng; giá trị lạ/không hỗ trợ ⇒ `400`. *(Chi tiết spec 2025-06-18 ghi theo trí nhớ — **chưa đối chiếu lại** văn bản spec; conformance BE-015 là nơi khoá hành vi.)*

## 2. Giải pháp

### A. Cấu trúc `internal/adapter/mcpserver/`

```
transport_http.go        # ServeHTTP: Accept/Content-Type, POST/GET/DELETE, mã HTTP
jsonrpc_codec.go         # decode 1 message, giới hạn độ sâu/kích thước, phân loại request|notification|response
lifecycle.go             # state machine initialize -> initialized -> ready ; ping ; cancelled
capabilities.go          # CapabilityBuilder, serverInfo, instructions
protocol_versions.go     # DUY NHẤT nơi chứa chuỗi phiên bản spec (BE-015 khoá bằng test)
session_store.go         # port SessionStore + memorySessionStore
inflight_registry.go     # map[sessionID]map[requestID]cancel (huỷ cục bộ replica)
pagination_cursor.go     # cursor opaque ký HMAC
logging_level.go         # logging/setLevel + notifications/message có che secret
tool_error_mapping.go    # apperrors/gRPC status -> thông điệp ngắn cho isError:true
```

`protocol_versions.go`:

```go
// Nơi duy nhất liệt kê phiên bản spec MCP hỗ trợ — nâng spec = đổi file này + chạy conformance (BE-015).
var SupportedProtocolVersions = []string{"2025-06-18"} // mới nhất cuối; D5: tối thiểu 2025-06-18
func LatestProtocolVersion() string { return SupportedProtocolVersions[len(SupportedProtocolVersions)-1] }
func Negotiate(requested string) string { // trả đúng bản client xin nếu hỗ trợ, ngược lại bản mới nhất của server
    for _, v := range SupportedProtocolVersions { if v == requested { return v } }
    return LatestProtocolVersion()
}
```

### B. Transport (`transport_http.go`)

| Method | Hành vi BE-003 |
|---|---|
| `POST` | `Content-Type` phải là `application/json` (415 nếu không); `Accept` phải liệt kê **cả** `application/json` và `text/event-stream` (406 nếu thiếu — theo spec). Body = **đúng một** message JSON-RPC (mảng ⇒ lỗi `-32600`, batch đã bỏ ở 2025-06-18). Request ⇒ trả `application/json` (BE-003) hoặc `text/event-stream` (BE-004, khi handler phát progress); notification/response ⇒ `202 Accepted` không body |
| `GET` | `405 Method Not Allowed` + `Allow: POST` cho tới BE-004 (spec cho phép) |
| `DELETE` | `405` cho tới BE-004 |
| mọi method | `MCP-Protocol-Version` kiểm sau `initialize` (quy tắc §1.4); `Mcp-Session-Id` thiếu sau `initialize` ⇒ `400`; session không tồn tại/hết hạn/khác danh tính ⇒ `404` (không phân biệt, chống dò) |

Cổng engine (cắm SDK hoặc tự viết):

```go
type ProtocolEngine interface {
    // Handle xử lý đúng một message đã decode, trong ngữ cảnh session; trả response (nếu là request).
    Handle(ctx context.Context, sess Session, msg Message) (Response, error)
}
type Session struct{ ID, TenantID, UserID, ClientName, ProtocolVersion string; Ready bool; LogLevel string }
```

`ServeHTTP` luôn lấy `Principal` từ context (BE-002), **không** lấy từ header/session. Với `initialize` chưa có session: tạo session gắn `(TenantID, UserID, ClientID, TokenID)` của principal; với request sau: `store.Get(sessionID)` rồi so `sess.UserID == principal.UserID && sess.TenantID == principal.TenantID`, lệch ⇒ `404` + `Recorder.IdentityMismatch()`. Thông số HTTP: JSON response đặt `SetWriteDeadline(60s)`; mọi lỗi giao thức = body JSON-RPC error (HTTP 200) trừ các mã HTTP nêu trên.

Spike SDK (0.5 ngày, trước khi code): dựng một handler `go-sdk` tối thiểu kèm `SessionStore`/`EventStore` giả để kiểm 3 hook (a)(b)(c) ở trên; ghi **khoảng trống** vào ADR (không phải để chọn lại SDK). Hook thiếu ⇒ vá bằng lớp mỏng sau `ProtocolEngine`, không đổi API nội bộ.

### C. Vòng đời

```go
// lifecycle.go — state: New -> Initializing -> Ready -> Closed
switch {
case m.Method == "initialize":     // chỉ hợp lệ khi chưa có session
    ver := Negotiate(p.ProtocolVersion)
    sess := store.Create(ctx, principal, p.ClientInfo, ver, p.Capabilities) // BE-004: lưu bền
    w.Header().Set("Mcp-Session-Id", sess.ID)
    return Result{ProtocolVersion: ver, Capabilities: caps.Build(), ServerInfo: {Name:"orca", Title:"Orca", Version: buildVersion}, Instructions: instructions}
case m.Method == "notifications/initialized": store.MarkReady(sess.ID) // 202
case m.Method == "ping":           return Result{} // luôn trả {} ở mọi trạng thái
case !sess.Ready:                  return Error{Code:-32600, Message:"server not initialized"} // mọi method khác
case m.Method == "notifications/cancelled": inflight.Cancel(sess.ID, p.RequestID)
case m.Method == "logging/setLevel": store.SetLogLevel(sess.ID, p.Level) // phải thuộc RFC 5424: debug..emergency
default: engine dispatch (tools/*, resources/*, prompts/* do BE-007/010/011 đăng ký)
}
```

- `serverInfo.version` lấy từ biến `version` do `-ldflags` đặt ở `main.go` (đã có mẫu ở `usage-service`); `instructions` là hằng ngắn trong `capabilities.go` (không chứa dữ liệu tenant; viết cho LLM đọc, ≤ 600 ký tự).
- `inflight_registry`: `Register(sessID, reqID, cancel)` khi vào handler, `defer Deregister`. Trùng `id` đang chạy trong cùng session ⇒ `-32600` (“duplicate request id”) — là ca “id trùng” của fuzz. Huỷ xuyên replica ở BE-004.
- Logging: `logging/setLevel` lưu mức theo session; `LogSink.Send(level, logger, data)` chỉ đẩy nếu `level >= sess.LogLevel` và **luôn** qua `redact()` (cùng bộ che dùng cho audit, BE-013) — không bao giờ chứa token/secret.
- Phân trang: `pagination_cursor.go` mã hoá `{k:"tools", o:<offset|khoá>, s:<sessionID>, e:<unix>}` bằng base64url + HMAC-SHA256; hết hạn 10 phút; cursor lạ/giả/sai session ⇒ `-32602`. Khoá HMAC: tệp do Vault Agent render (`MCP_CURSOR_KEY_FILE`, khuôn `DATABASE_CREDENTIALS_FILE`), rơi về `MCP_CURSOR_KEY` cho dev; hỗ trợ **hai khoá** (hiện tại + trước đó) để xoay khoá không làm hỏng cursor đang bay — mọi replica phải cùng khoá. (Cần thêm đường dẫn secret vào policy Vault `deploy/dev/orca-policy.hcl` — **chưa xác minh** cấu trúc file.)

### D. Mô hình lỗi

| Loại | Biểu diễn |
|---|---|
| JSON hỏng / vượt độ sâu | `-32700` (parse) hoặc `-32600` (invalid request); độ sâu tối đa 32, kích thước theo `MCP_MAX_REQUEST_BYTES` |
| Method không có | `-32601` |
| Tham số sai (kể cả cursor) | `-32602` |
| Lỗi nội bộ không lường | `-32603` với thông điệp cố định `internal error` |
| Lỗi nghiệp vụ của tool | **kết quả** `{isError:true, content:[{type:"text", text:"<ngắn, đọc được>"}]}`, không phải JSON-RPC error (BE-007 gọi `tool_error_mapping.go`) |

`tool_error_mapping.go`: `status.Convert(err).Message()` nếu khớp `^[A-Z0-9_]+: ` thì dùng phần sau dấu `: ` + mã; nếu không ⇒ `"The operation failed."`. Tuyệt đối không đưa `err.Error()` thô, stack, tên service khác, hay `Err` bọc bên trong (đã bị `apperrors.ToGRPCStatus` loại — chỉ `Code: Message` đi ra).

### E. Kênh `mcp.server.info` (`channels_mcp_server_info.go`)

Đăng ký **không** qua `mcpHandler` gating (phải trả được khi tắt):

```go
r.Register("mcp.server.info", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
    if !d.Enabled { return McpServerInfo{Enabled: false}, nil }          // C8: không lỗi, chỉ enabled:false
    resp, err := d.Client.GetServerInfo(ctx, &mcpv1.GetServerInfoRequest{}) // identity/tenant qua metadata (Dispatch đã Attach)
    if err != nil { return nil, mcpChannelError(err) }
    return McpServerInfo{
        Enabled:             true,                         // = cờ process MCP_ENABLED (CONTRACT §1), KHÔNG trộn cài đặt tenant
        TenantEnabled:       &resp.Enabled,                // mcp.tenant_settings.enabled; tenant mới mặc định MCP_TENANT_DEFAULT_ENABLED=true (D6)
        ResourceURL:         d.Config.ResourceURL,         // <MCP_PUBLIC_BASE_URL>/mcp
        ProtocolVersions:    mcpserver.SupportedProtocolVersions,
        AuthorizationServer: d.Config.IssuerURL,           // MCP_ISSUER_URL, mặc định PUBLIC_BASE_URL (auth-service là AS, T3)
        ScopesSupported:     fromProto(resp.Scopes),
        DCREnabled:          resp.DcrEnabled,
        MaxTokenDays:        int(resp.MaxTokenDays),
        KillSwitch:          killSwitchView(resp.KillSwitch),
    }, nil
})
```

JSON (struct tag camelCase, đúng CONTRACT §1; `omitempty` cho `killSwitch.reason/at`):

```go
type McpServerInfo struct {
    Enabled bool `json:"enabled"`; TenantEnabled *bool `json:"tenantEnabled,omitempty"`; ResourceURL string `json:"resourceUrl"`
    ProtocolVersions []string `json:"protocolVersions"`; AuthorizationServer string `json:"authorizationServer"`
    ScopesSupported []McpScopeDescriptor `json:"scopesSupported"`; DCREnabled bool `json:"dcrEnabled"`
    MaxTokenDays int `json:"maxTokenDays"`; KillSwitch McpKillSwitch `json:"killSwitch"`
}
```

Khi `Enabled:false` vẫn trả các slice rỗng (`[]`, không `null` — `normalizeNilSlices` xử lý struct một cấp nhưng test khẳng định) và `resourceUrl` rỗng được phép; FE dựa `enabled` (process) và `tenantEnabled` (CONTRACT §1/§6): hiện UI khi `enabled && tenantEnabled !== false`; admin thấy công tắc khi `enabled && tenantEnabled === false`.

## Hợp đồng với frontend

- Kênh: `mcp.server.info` (CONTRACT §2.1) — mọi user đăng nhập, không admin. Hình dạng trả về = `McpServerInfo` §1 (byte-for-byte tên field). FE-MCP-SOL-001/002 dựa vào: `enabled=false` (hoặc `tenantEnabled=false` với user thường) ⇒ ẩn UI, không toast lỗi; `resourceUrl` dùng cho snippet kết nối; `protocolVersions` hiển thị; `killSwitch.active` hiện banner.
- Lỗi: tắt ⇒ không lỗi; mcp-service lỗi ⇒ chuỗi lỗi chung (xem đề nghị `MCP_UNAVAILABLE` ở BE-002).
- HTTP §3: `POST /mcp` hiện thực đủ; `GET`/`DELETE` trả 405 tới BE-004.

## Sửa TDD kèm theo

**T1** (adapter giao thức viết tay trong gateway), **T2** (`tools/call` → `Registry.Dispatch` do BE-007 nối; mặc định quyết định allow/deny ở mcp-service), **T8** (`MCP_ISSUER_URL`, `MCP_CURSOR_KEY_FILE`, metric hook `orca_mcp_*`). Thêm ADR `docs/adrs/v2/ADR-021-mcp-go-sdk-and-transport.md`.

## Kiểm thử

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/mcpserver/... ./internal/adapter/wscompat/... -run 'Lifecycle|Transport|ServerInfo|Cursor' -v
go test ./internal/adapter/mcpserver/ -run xxx -fuzz FuzzJSONRPCDecode -fuzztime 60s
```

| Test | Nội dung |
|---|---|
| `lifecycle_test.go` (bảng) | `initialize` phiên bản lạ ⇒ trả `LatestProtocolVersion()`; `tools/list` trước `initialized` ⇒ `-32600`; sau đó OK; `ping` mọi trạng thái; `logging/setLevel` sai mức ⇒ `-32602` |
| `transport_test.go` | Accept thiếu `text/event-stream` ⇒ 406; Content-Type sai ⇒ 415; mảng batch ⇒ `-32600`; notification ⇒ 202 rỗng; header version lạ ⇒ 400; session lạ ⇒ 404; session của user A + token user B ⇒ 404 + counter |
| `pagination_cursor_test.go` | ký/giải ký; sửa 1 byte ⇒ `-32602`; cursor của session khác ⇒ `-32602`; hết hạn; xoay khoá hai-khoá |
| `FuzzJSONRPCDecode` | body ngẫu nhiên/lồng sâu/id trùng không panic, không cấp phát vô hạn |
| `tool_error_mapping_test.go` | không bao giờ chứa `rpc error`, stack hay tên service |
| `channels_mcp_server_info_test.go` | tắt ⇒ `{enabled:false}` + slice rỗng (không `null`); bật ⇒ field đủ, JSON key đúng CONTRACT (so khớp golden JSON); `Client` lỗi `Unavailable` ⇒ lỗi chung |
| Thủ công | MCP Inspector (Streamable HTTP) kết nối một replica với token dev: thấy `serverInfo`, capabilities đúng, `tools/list` có `cursor` |

## Rủi ro & phụ thuộc

- Spec đổi: mọi chuỗi phiên bản chỉ ở `protocol_versions.go`; BE-015 có kiểm tra CI chặn chuỗi `20xx-xx-xx` rải rác.
- SDK (đã chốt) thiếu hook (a)–(c) ⇒ +3–4 ngày vá/tự viết lớp transport; CR ước lượng Medium (5–7 ngày) chưa tính kịch bản này, Effort có thể lên Large nếu tự viết.
- Khoá HMAC phân phối sai giữa replica ⇒ mọi cursor chéo replica hỏng: readiness check của gateway phải fail nếu không đọc được khoá khi `MCP_ENABLED`.
- **Impact (`gitnexus`, chưa chạy)** — trước khi sửa: `impact({target:"Handler", direction:"upstream"})` cho package `mcpserver` (mã mới do BE-002 tạo, kỳ vọng LOW); `impact({target:"Dispatch", direction:"upstream"})` chỉ khi BE-007 gọi (kỳ vọng HIGH vì mọi kênh UI đi qua — **cảnh báo** người duyệt BE-007).

## Không thuộc phạm vi

Auth (BE-005/006), session bền/SSE/resume/DELETE (BE-004), nội dung `tools/*` (BE-007), `resources/*` (BE-010), `prompts/*` (BE-011), `completions` (không làm), policy (BE-012).

## Liên quan

[CR-003](../../../../../../docs/crs/v5/mcp-protocol-server/CR-MCP-003-streamable-http-and-lifecycle.md) · [BE-MCP-SOL-002](../../mcp-service-foundation/solutions/BE-MCP-SOL-002-gateway-mcp-endpoint-wiring.md) · [BE-MCP-SOL-004](./BE-MCP-SOL-004-sessions-sse-resumability.md) · [FE-MCP-SOL-002](../../../../../frontend/crs/v5/mcp-protocol-server/solutions/FE-MCP-SOL-002-connect-panel-and-active-sessions.md)
