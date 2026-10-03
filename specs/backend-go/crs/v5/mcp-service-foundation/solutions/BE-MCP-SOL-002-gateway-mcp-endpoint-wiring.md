# BE-MCP-SOL-002: Nối `/mcp` vào `api-gateway` — adapter `mcpserver`, cấu hình, chống tấn công transport, khung kênh `mcp.*`

> **✅ Implemented (unit/integration tests) — see Gaps.** Phụ thuộc [BE-MCP-SOL-001](./BE-MCP-SOL-001-scaffold-mcp-service.md) (client `mcpv1.McpServiceClient`).

**CR:** [CR-MCP-002](../../../../../../docs/crs/v5/mcp-service-foundation/CR-MCP-002-gateway-mcp-endpoint-wiring.md)
**Service:** `api-gateway` (`internal/adapter/mcpserver`, `internal/adapter/wscompat`, `internal/adapter/httpgateway`, `internal/config`, `cmd/server`) · `deploy/dev` nginx · CI
**TDD tham chiếu:** [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §2 (edge), §3, §6 (package layout), §9 (rate limit/security) · **Hợp đồng FE:** [CONTRACT](../../CONTRACT-mcp-ui-api.md) §0 (C1–C4, C8), §3 (endpoint HTTP)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `httpgateway/router.go` (`NewRouter`: auth routes → trace → push → SCM webhook → `/ws` → pairing → `/agent` → `r.Group(authed)` với `authMiddleware` + `rateLimitMiddleware`), `middleware.go`, `config/config.go`, `cmd/server/main.go` (dial lazy qua `gatewaygrpc.Dial(cfg.OtherServiceAddrs[...])`, `otelhttp.NewHandler(router,"api-gateway")`, `http.Server{Addr,Handler}` **không timeout**), `wscompat/registry.go` (`Identity{TenantID,UserID,DeviceID,Role}`, `Dispatch` bọc `dispatchRPCTimeout=60s`, `Register`/`RegisterStream`), `session_dialect.go:94-103` (lỗi WS = `err.Error()`), `usecase/rate_limit.go` (token bucket **per-tenant, per-replica**), `deploy/dev/docker/nginx/orca.conf` (đã có mẫu SSE `location = /api/trace-stream` với `proxy_buffering off`, `proxy_read_timeout 3600s`).

Điểm cần chú ý cho thiết kế:

1. **Lỗi WS mang nguyên chuỗi gRPC.** `err.Error()` của status gRPC là `rpc error: code = NotFound desc = MCP_NOT_FOUND: …` — **không** khớp `^([A-Z0-9_]+): ` mà CONTRACT C4 hứa với FE. ⇒ mọi kênh `mcp.*` phải đi qua `mcpChannelError` (§C) để cắt phần `rpc error…desc =`. (FE-MCP-SOL-001 cũng phân tích chịu lỗi cả hai dạng.)
2. **`/ws` không kiểm Origin** (`wscompat/handler.go:91 InsecureSkipVerify: true`) mà cookie là cách xác thực ⇒ kênh có tác dụng phụ (`mcp.consent.decide`, `mcp.approval.decide`, `mcp.token.create`) chịu rủi ro cross-site WebSocket hijacking. Xử lý ở §D.3 (đã chốt: `WS_ALLOWED_ORIGINS`).
3. **Không có handler `NotFound` tuỳ biến**; `mountStubRoutes` chỉ đăng ký prefix `/v1/...` ⇒ khi không mount, `/mcp` ra 404 mặc định của chi (đúng acceptance `MCP_ENABLED=false ⇒ 404`).
4. `rateLimitMiddleware` là hàm **không export** trong `httpgateway` và lấy tenant từ identity do `authMiddleware` đặt; adapter `mcpserver` nằm package khác ⇒ gọi thẳng `usecase.RateLimiter.Allow(tenantID)` sau khi verify token.

### Quyết định khác/thêm so với CR gốc

- CR-002 chỉ nêu route `/mcp` + `.well-known`; solution này **cũng** dựng khung kênh WS `mcp.*` (đăng ký gating `MCP_DISABLED`/`MCP_NOT_ADMIN`, ánh xạ lỗi, chia file) để SOL-003..014 chỉ việc thêm kênh — tránh 8 PR cùng sửa một `channels_mcp.go`.
- Thêm `ReadHeaderTimeout` cho `publicServer` (toàn cục, vô hại cho WS/SSE) nhưng **không** đặt `WriteTimeout`/`ReadTimeout` (sẽ cắt SSE/WS). Hạn ghi riêng cho phản hồi JSON dùng `http.NewResponseController(w).SetWriteDeadline`.

## 2. Giải pháp

### A. Cấu hình (`internal/config/config.go`, đọc qua `commonconfig.StringEnv` + helper bool/int sẵn có trong file)

| Env | Mặc định | Ghi chú |
|---|---|---|
| `MCP_ENABLED` | `false` | **công tắc tổng cấp tiến trình** (mặc định `false` tới khi qua gate CR-015); tắt ⇒ không mount route, không dial mcp-service; kênh `mcp.*` trả `MCP_DISABLED` (C8) |
| `MCP_TENANT_DEFAULT_ENABLED` | `true` | **Đã chốt (2026-10-01, D6):** giá trị `enabled` ban đầu mà `mcp-service` ghi khi tạo lười (lazy) hàng `tenant_settings` của tenant mới. Chỉ là cấu hình ngoài (env), admin vẫn tắt được từng tenant; **không** ảnh hưởng `MCP_ENABLED` và **không** đổi mặc định rủi ro (xem BE-MCP-SOL-012 §tenant settings, BE-MCP-SOL-015 rollout). Biến này do `mcp-service` đọc (khai báo ở config của mcp-service, BE-001); liệt kê ở đây để compose truyền đủ |
| `MCP_PUBLIC_BASE_URL` | = `PUBLIC_BASE_URL` | gốc để dựng `resourceUrl = <base>/mcp` và metadata (BE-005); phải `https` ở production |
| `MCP_ALLOWED_ORIGINS` | = `WS_ALLOWED_ORIGINS` khi không đặt | CSV origin đúng chuỗi `scheme://host[:port]` (cho `/mcp`); **Đã chốt (D3):** không đặt ⇒ dùng `WS_ALLOWED_ORIGINS`; cả hai rỗng ⇒ mọi request `/mcp` **có** `Origin` bị 403 (endpoint mới, không có yêu cầu tương thích ngược) |
| `WS_ALLOWED_ORIGINS` | rỗng | **Đã chốt (D3):** CSV origin chính xác hoặc host pattern (vd `https://orca.example.com,https://*.example.com`) cho `wscompat.Handler` và `wsbridge.Handler`; không rỗng ⇒ thay `InsecureSkipVerify` bằng package `internal/adapter/originpolicy` (**đã hiện thực**: `originpolicy.go` + test; nối vào `wscompat.Handler`, `wsbridge.Handler` bằng `WithOriginPolicy`, kiểm TRƯỚC `websocket.Accept` và trả `403`; tự kiểm scheme+host vì `OriginPatterns` của thư viện chỉ so host); rỗng ⇒ giữ hành vi cũ (cho phép mọi origin) kèm WARN to khi khởi động — xem §D.3 |
| `MCP_MAX_REQUEST_BYTES` | `1048576` | `http.MaxBytesReader` |
| `MCP_SESSION_IDLE_TTL` | `30m` | gateway chỉ đọc để tính thông điệp lỗi; **reaper nằm ở mcp-service** (BE-004), cùng biến env truyền cho cả hai qua compose |
| `MCP_MAX_SSE_STREAMS_PER_USER` / `_PER_TENANT` | `5` / `200` | giới hạn cứng, chặt tại BE-004 (đếm theo DB); BE-002 chỉ khai báo + giới hạn per-replica làm lưới an toàn (T6) |
| `MCP_READ_HEADER_TIMEOUT` | `10s` | áp cho `publicServer.ReadHeaderTimeout` |
| `MCP_SERVICE_ADDR` | rỗng | thêm khoá `"mcp-service"` vào `OtherServiceAddrs`; rỗng + `MCP_ENABLED=true` ⇒ lỗi khởi động rõ ràng |

`Config.Validate()` mới: `MCP_ENABLED && MCPPublicBaseURL==""` ⇒ lỗi; URL không parse được ⇒ lỗi; mỗi `MCP_ALLOWED_ORIGINS` phải parse được và không có path; `WS_ALLOWED_ORIGINS` chấp nhận pattern host (`*.example.com`) nên chỉ kiểm không rỗng-từng-phần tử và không có path.

### B. Package `internal/adapter/mcpserver/`

Chỉ là điểm lắp ghép (không chứa giao thức — BE-003 — hay logic tool — BE-007):

```go
package mcpserver

// Deps: mọi thứ adapter cần; field nil = tính năng chưa được solution khác cắm vào.
type Deps struct {
    Logger       *slog.Logger
    Config       Config                 // lát cắt của config.Config: ResourceURL, AllowedOrigins, MaxBodyBytes, StreamLimits
    Verifier     TokenVerifier          // BE-005/006; mặc định denyAllVerifier
    RateLimiter  *usecase.RateLimiter   // dùng chung bộ giới hạn theo tenant của gateway
    Transport    http.Handler           // BE-003 cắm vào; nil ⇒ 501 JSON
    ProtectedResourceMetadata http.Handler // BE-005; nil ⇒ không mount
    AuthServerMetadata        http.Handler // BE-005; nil ⇒ không mount
    Recorder     Recorder               // BE-015 (metrics); mặc định noop
}

type TokenVerifier interface {
    // Verify KHÔNG đọc cookie. Lỗi ⇒ 401/403 do middleware dựng (WWW-Authenticate).
    Verify(ctx context.Context, r *http.Request) (Principal, error)
}
type Principal struct { TenantID, UserID, Role, ClientID, TokenID, GrantID string; Scopes []string }

func New(d Deps) *Handler
func (h *Handler) Mount(r chi.Router) // gọi từ NewRouter, NGOÀI r.Group(authed)
```

`Mount`:

```go
func (h *Handler) Mount(r chi.Router) {
    r.Route("/mcp", func(mr chi.Router) {
        mr.Use(h.limitBody, h.guardOrigin, h.rejectCookieOnly, h.authenticate, h.rateLimit, h.limitStreams)
        mr.Method(http.MethodPost, "/", h.transportOrNotImplemented())
        mr.Method(http.MethodGet, "/", h.transportOrNotImplemented())
        mr.Method(http.MethodDelete, "/", h.transportOrNotImplemented())
    })
    if h.d.ProtectedResourceMetadata != nil { r.Get("/.well-known/oauth-protected-resource", h.d.ProtectedResourceMetadata.ServeHTTP) }
    if h.d.AuthServerMetadata != nil        { r.Get("/.well-known/oauth-authorization-server", h.d.AuthServerMetadata.ServeHTTP) }
}
```

Lưu ý định tuyến chi: client có thể gọi `/mcp` hoặc `/mcp/` ⇒ đăng ký thêm `r.Handle("/mcp", …)` cùng handler (test cả hai), không redirect 301 (POST sẽ mất body).

Thứ tự middleware có lý do bảo mật:

| # | Middleware | Hành vi |
|---|---|---|
| 1 | `limitBody` | `http.MaxBytesReader(w, r.Body, MaxBodyBytes)`; vượt ⇒ 413 JSON-RPC error `-32600` |
| 2 | `guardOrigin` | không có `Origin` ⇒ cho qua (CLI/desktop, vẫn cần token); có `Origin` ⇒ phải **bằng đúng** một phần tử allow-list; `Origin: null` hoặc không khớp ⇒ `403` (chống DNS rebinding, spec Streamable HTTP). Không so khớp theo hậu tố tự do; pattern host (`*.example.com`) chỉ được dùng khi đến từ `WS_ALLOWED_ORIGINS`/`MCP_ALLOWED_ORIGINS` đã chốt (D3) và đi qua cùng matcher của `coder/websocket` để hai đường nhất quán |
| 3 | `rejectCookieOnly` | có cookie `orca_session` mà không có `Authorization: Bearer` ⇒ `401` + `WWW-Authenticate`; **luôn** xoá header `Cookie` khỏi `r` trước khi vào handler sâu hơn (xoá khả năng handler vô tình dùng cookie — chống confused deputy/CSRF) |
| 4 | `authenticate` | `Verifier.Verify`; lỗi ⇒ `401` + `WWW-Authenticate: Bearer resource_metadata="<base>/.well-known/oauth-protected-resource"` (+ `error="invalid_token"` khi token sai); thiếu scope ⇒ `403`. Mặc định `denyAllVerifier` ⇒ 401 cho tới khi BE-005/006 cắm vào |
| 5 | `rateLimit` | `RateLimiter.Allow(principal.TenantID)` sau khi biết tenant; hết hạn mức ⇒ `429` + `Retry-After: 1`. Mở rộng khoá `(tenant,user,client,risk)` là T6, thực hiện ở BE-007/012 |
| 6 | `limitStreams` | chỉ cho `GET`: semaphore per-user/per-tenant **per-replica** (`MaxSSEStreams*`), vượt ⇒ `429`; số liệu chính xác theo cluster do BE-004 (bảng `session_streams`) |

Principal được đưa vào `context` (`mcpserver.PrincipalFromContext`) cho BE-003. Không đặt Principal vào `httpgateway.identityContextKey` (tách biệt hoàn toàn với đường cookie).

### C. Khung kênh WS `mcp.*` trong `wscompat` (C1, C3, C4, C8)

Tách file (đúng quy tắc không đặt tên mơ hồ, không `max-lines` disable):

| File | Nội dung | Chủ |
|---|---|---|
| `channels_mcp.go` | `McpChannelDeps{Enabled bool; Client mcpv1.McpServiceClient; ServerInfo ServerInfoProvider}` + `RegisterMcpChannels(r *Registry, d McpChannelDeps)` gọi các `registerMcp*` bên dưới | SOL-002 |
| `channels_mcp_gating.go` | `mcpHandler(d, adminOnly, fn)`: bọc handler (xem dưới), `mcpChannelError(err)` | SOL-002 |
| `channels_mcp_server_info.go` | `mcp.server.info` | SOL-003 |
| `channels_mcp_session.go` | `mcp.session.*`, `mcp.admin.session.list` | SOL-004 |
| `channels_mcp_{consent,token,approval,policy,…}.go` | các nhóm còn lại | SOL-005..014 |

```go
// channels_mcp_gating.go
func mcpHandler(d McpChannelDeps, adminOnly bool, fn ChannelHandler) ChannelHandler {
    return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        if !d.Enabled { return nil, errors.New("MCP_DISABLED: MCP is disabled on this server") } // C8
        if adminOnly && id.Role != "admin" { return nil, errors.New("MCP_NOT_ADMIN: admin role required") } // C3
        out, err := fn(ctx, id, args)
        return out, mcpChannelError(err)
    }
}

// mcpChannelError cắt "rpc error: code = X desc = " để message còn "<CODE>: <msg>" (C4).
// Lỗi không có mã MCP_* ⇒ thông điệp chung, KHÔNG rò chi tiết nội bộ.
func mcpChannelError(err error) error {
    if err == nil { return nil }
    if st, ok := status.FromError(err); ok {
        if mcpCodePattern.MatchString(st.Message()) { return errors.New(st.Message()) }
        if st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded {
            return errors.New("mcp-service temporarily unavailable") // xem "Đề nghị đổi CONTRACT": MCP_UNAVAILABLE
        }
    }
    var le *mcpLocalError; if errors.As(err, &le) { return errors.New(le.Error()) }
    return errors.New("internal error")
}
var mcpCodePattern = regexp.MustCompile(`^MCP_[A-Z0-9_]+: `)
```

`mcpChannelError` chỉ dùng thông điệp của gRPC status và **không bao giờ nối/echo args** của kênh (D1/CONTRACT C11: kênh `mcp.externalServer.setSecret` mang `value` plaintext). Khung kênh `mcp.*` cũng phải có allow-list kênh nhạy cảm để log/trace WS và trace store che `args` (test `TestSensitiveChannelArgsRedacted`; chi tiết BE-MCP-SOL-014 §B).

`RegisterMcpChannels` chỉ đăng ký kênh đã có solution chủ (SOL-003 trở đi); kênh chưa có chủ **không** được đăng ký giả (rơi về `notImplementedHandler` hiện có — FE coi như "chưa hỗ trợ"). Không phát minh mã `MCP_NOT_IMPLEMENTED` ngoài CONTRACT §2.3. Gating `Enabled=false` áp cho mọi kênh đã đăng ký; ngoại lệ `mcp.server.info` luôn trả `{enabled:false}` (C8, SOL-003).

Role thật: `Dispatch` đã `AttachIdentity(... Role: id.Role)` (sửa lỗi CR-DS-006 — `registry.go:187`), nên mcp-service đọc được role từ metadata; gateway kiểm `id.Role` thêm một lần ở đây (fail-closed: `Role==""` ⇒ không phải admin).

### D. Chống tấn công transport

1. **Server timeouts** (`main.go`): `publicServer = &http.Server{Addr, Handler, ReadHeaderTimeout: cfg.MCPReadHeaderTimeout}`. Không `WriteTimeout`. Phản hồi JSON của `/mcp` gọi `rc := http.NewResponseController(w); rc.SetWriteDeadline(time.Now().Add(60*time.Second))` (khớp `dispatchRPCTimeout`); SSE không đặt hạn, có heartbeat (BE-004).
2. **`otelhttp.NewHandler` bọc router**: phải giữ `http.Flusher` cho SSE. Test bắt buộc: `httptest.NewServer(otelhttp.NewHandler(handler,"x"))` + handler gọi `Flush()` và client đọc từng chunk (đã có mẫu SSE ở `trace_routes.go`, nhưng chưa có test qua `otelhttp` — **(chưa xác minh)** ⇒ test này là cổng chặn).
3. **WS Origin** (rủi ro §1.2) — **Đã chốt (D3, 2026-10-01): phê duyệt, đang được hiện thực trong mã.** Áp cho `wscompat.Handler`, `wsbridge.Handler` và endpoint `/mcp` mới. Env `WS_ALLOWED_ORIGINS` (CSV origin chính xác hoặc host pattern, vd `https://orca.example.com,https://*.example.com`): khi **không rỗng** ⇒ truyền `websocket.AcceptOptions{OriginPatterns: …}` thay `InsecureSkipVerify` (`coder/websocket` mặc định cho phép request không có `Origin` và same-origin nên desktop/CLI/mobile không bị ảnh hưởng; trình duyệt từ origin lạ bị từ chối). Khi **rỗng** ⇒ giữ hành vi cũ (`InsecureSkipVerify`) để deployment hiện có không gãy, kèm **WARN to khi khởi động** ("WS_ALLOWED_ORIGINS unset: WebSocket Origin not verified"). `MCP_ALLOWED_ORIGINS` cho `/mcp` mặc định bằng `WS_ALLOWED_ORIGINS` khi không đặt. Không còn cờ phụ thuộc `MCP_ENABLED`: bảo vệ này độc lập với MCP. Test: origin trong danh sách ⇒ upgrade OK; origin lạ ⇒ 403; không `Origin` ⇒ OK; env rỗng ⇒ WARN + hành vi cũ. Lưới an toàn mức kênh (nonce same-origin cho `mcp.consent.decide`/`mcp.approval.decide`/`mcp.token.create`) vẫn nên có cho deployment để trống danh sách — ghi cho BE-005/013.
4. **nginx** (`deploy/dev/docker/nginx/orca.conf`, và bản `deploy/old/**` nếu còn dùng — **(chưa xác minh)**):

```nginx
# SSE + header MCP phải đi nguyên vẹn; không buffer, đọc dài.
location ^~ /mcp {
    proxy_pass $api_gateway;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_buffering off; proxy_cache off; chunked_transfer_encoding on;
    proxy_read_timeout 3600s; proxy_send_timeout 3600s;
    add_header X-Accel-Buffering no;
}
location ^~ /.well-known/oauth- { proxy_pass $api_gateway; proxy_http_version 1.1; proxy_set_header Host $host; }
```

nginx chuyển tiếp header request mặc định (kể cả `Mcp-Session-Id`, `MCP-Protocol-Version`, `Last-Event-ID` — tên có dấu gạch ngang, không dính cơ chế bỏ header chứa `_`) và header response (`Mcp-Session-Id`); **không** được thêm `proxy_hide_header`. Đường `/oauth/*` thêm ở BE-005.
5. **CI**: `backend-go/ci/check-nginx-mcp-routing.sh` (khuôn `check-nginx-admin-api-routing.sh`, nhưng kiểm **tĩnh** để chạy được khi `MCP_ENABLED=false`): `grep -A12 'location ^~ /mcp'` phải chứa `proxy_buffering off` và `proxy_read_timeout` ≥ 600s, không chứa `proxy_hide_header`; thêm bước tuỳ chọn có stack: `curl -N -H 'Origin: https://evil.example' …/mcp` ⇒ 403 và `curl …/mcp` (không token) ⇒ 401 có `WWW-Authenticate` khi `MCP_ENABLED=true`.

### E. Wiring `main.go` & router

- `httpgateway.Deps` thêm `MCP *mcpserver.Handler` (nil hợp lệ — khớp quy ước các field optional khác). `NewRouter`: `if deps.MCP != nil { deps.MCP.Mount(r) }` đặt **trước** `r.Group(authed)` (cùng chỗ với `/ws`).
- `main.go`: nếu `cfg.MCPEnabled`: `mcpConn := gatewaygrpc.Dial(cfg.OtherServiceAddrs["mcp-service"])`; `mcpClient := mcpv1.NewMcpServiceClient(mcpConn)`; `healthSrv.Register("mcp-service", grpcConnHealthCheck(mcpConn))`; `mcpserver.New(Deps{…, Verifier: denyAllVerifier{}})`. Luôn gọi `wscompat.RegisterMcpChannels(wsCompatRegistry, McpChannelDeps{Enabled: cfg.MCPEnabled, Client: mcpClient /* nil khi tắt */})` ngay sau `RegisterRealChannels`.
- `internal/domain/registry.go`: **không** thêm `ServiceRegistry` entry cho `/mcp` (registry này mô tả proxy REST `/v1/...` và stub 501; `/mcp` không thuộc loại đó).

### F. Tài liệu

`services/api-gateway/README.md` thêm hàng: `POST/GET/DELETE /mcp` — **Stub (501) sau BE-002; Real sau BE-003**; `/.well-known/*` — chưa mount (BE-005). Cập nhật theo sự thật từng bước (CR acceptance: không ghi "Real" cho thứ còn stub).

## Hợp đồng với frontend

- Kênh: khung cho toàn bộ `mcp.*` của CONTRACT §2 (đăng ký bởi các solution chủ). FE-MCP-SOL-001 dựa vào ba hành vi do solution này cam kết: (1) `MCP_ENABLED=false` ⇒ mọi kênh đã đăng ký trả `MCP_DISABLED: …` còn `mcp.server.info` trả `{enabled:false}` (SOL-003); (2) kênh admin sai vai trò ⇒ `MCP_NOT_ADMIN: …`; (3) message luôn dạng `<CODE>: <msg>` (đã cắt `rpc error`).
- Mã lỗi dùng: `MCP_DISABLED`, `MCP_NOT_ADMIN`.
- **Đề nghị đổi CONTRACT (không tự sửa):** thêm mã `MCP_UNAVAILABLE` (§2.3) cho `Unavailable/DeadlineExceeded` từ mcp-service; hiện chỉ có thể trả chuỗi không mã `mcp-service temporarily unavailable`, FE xử lý như lỗi chung.
- Điểm cuối HTTP CONTRACT §3 (`/mcp`, `/.well-known/*`) chỉ có khung; hành vi đầy đủ ở BE-003/005.

## Sửa TDD kèm theo

- **T1** — `api-gateway.md` §3 (thêm `/mcp`, `/.well-known/*`, `/oauth/*`), §6 (adapter `mcpserver` là *edge protocol adapter*, không có quy tắc nghiệp vụ); `arch/08` mục "API Gateway responsibilities".
- **T2** — `api-gateway.md` §6 mô tả `wscompat.Registry` là lớp dịch tới gRPC client; quyết định allow/deny/approval luôn do mcp-service trả về.
- **T6** — `api-gateway.md` §9: khoá giới hạn mở rộng `client`/`risk`; giới hạn số stream SSE; ghi rõ sai số per-replica.
- **T8** — danh sách env `MCP_*`.
- Ngoài bảng T: `api-gateway.md` §9 bổ sung chính sách Origin cho `/ws`.

## Kiểm thử

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/mcpserver/... ./internal/adapter/httpgateway/... ./internal/adapter/wscompat/... ./internal/config/...
bash ../../ci/check-nginx-mcp-routing.sh
```

- `router_test.go`: (a) `MCP=nil` ⇒ `GET/POST /mcp` = 404; (b) bật ⇒ route phản hồi 401 (không token) với `WWW-Authenticate` chứa `resource_metadata=`; (c) `/mcp` **không** gọi `CookieValidator` (fake đếm lời gọi = 0) kể cả khi có cookie hợp lệ; (d) `/.well-known/*` truy cập không token khi handler được cắm.
- `mcpserver/origin_test.go` (bảng): không Origin → qua; Origin trong allow-list → qua; Origin khác/`null`/khác cổng/`http` vs `https` → 403.
- `mcpserver/limits_test.go`: body > `MaxBodyBytes` → 413; 6 GET đồng thời cùng user → cái thứ 6 = 429; `goleak.VerifyNone`.
- `channels_mcp_gating_test.go`: tắt ⇒ `MCP_DISABLED`; non-admin gọi kênh admin ⇒ `MCP_NOT_ADMIN`; `Role==""` ⇒ `MCP_NOT_ADMIN`; lỗi `status.Error(NotFound,"MCP_NOT_FOUND: x")` ⇒ message đúng `MCP_NOT_FOUND: x`; lỗi lạ ⇒ `internal error` (không rò chuỗi gốc).
- `otelhttp_flush_test.go`: SSE chạy qua `otelhttp.NewHandler` nhận chunk trước khi handler kết thúc.
- `config_test.go`: bảng `Validate()`.

## Rủi ro & phụ thuộc

- Đổi `/ws` Origin (D.3) có thể làm hỏng client tự dựng gửi `Origin` lạ ⇒ rollout bằng `WS_ALLOWED_ORIGINS` (rỗng = hành vi cũ + WARN, nên không gãy deployment hiện có), thử với web client + mobile + CLI (`cmd/orca-cli`) trước.
- `denyAllVerifier` nghĩa là sau SOL-002 mọi gọi `/mcp` đều 401 — đúng chủ ý cho tới SOL-005/006.
- Per-replica limiter (T6) có thể cho gấp N lần hạn mức khi N replica: chấp nhận, ghi rõ; chặt thật ở BE-004.
- **Impact (`gitnexus`, chưa chạy)** — chạy trước khi sửa: `impact({target:"NewRouter", direction:"upstream"})` (kỳ vọng LOW–MEDIUM: gọi từ `main.go` + `router_test.go`); `impact({target:"Deps", direction:"upstream"})` cho `httpgateway.Deps` (thêm field, kỳ vọng LOW); `impact({target:"RegisterRealChannels", direction:"upstream"})` (chỉ đọc, ta *không* đổi chữ ký — gọi hàm mới riêng); `impact({target:"Handler", direction:"upstream"})` ở `wscompat` trước khi đổi `Accept` (D.3, kỳ vọng **HIGH**: đường vào duy nhất của mọi RPC UI — cần cảnh báo người duyệt).

## Không thuộc phạm vi

Giao thức JSON-RPC/initialize (BE-003), session/SSE thực (BE-004), verifier thật + metadata (BE-005/006), policy/audit, metrics thật (BE-015 — `Recorder` chỉ là điểm cắm).

## Liên quan

[CR-002](../../../../../../docs/crs/v5/mcp-service-foundation/CR-MCP-002-gateway-mcp-endpoint-wiring.md) · [BE-MCP-SOL-001](./BE-MCP-SOL-001-scaffold-mcp-service.md) · `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` · `.../wscompat/registry.go` · `deploy/dev/docker/nginx/orca.conf`
