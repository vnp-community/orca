# CR-MCP-002 — Nối endpoint `/mcp` vào `api-gateway`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-002 |
| **Tên** | Thêm adapter `internal/adapter/mcpserver`, route `/mcp` + `/.well-known/*`, cấu hình và biện pháp chống tấn công transport |
| **Loại** | Feature (wiring) |
| **Priority** | 🔴 P0 |
| **Effort** | Small (2–3 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `api-gateway/internal/adapter/httpgateway/router.go`, `wscompat` |
| **Phụ thuộc** | [CR-MCP-001](./CR-MCP-001-scaffold-mcp-service.md) |

---

## Bối cảnh & Vấn đề

- `router.go` mount `GET /ws` (wscompat) và nhóm route REST `authed`; **không có** path nào cho MCP.
- api-gateway là *"the system's one external listener"* — MCP phải đi qua đây (quyết định D1) để thừa hưởng rate limit theo tenant (`usecase/rate_limit.go`) và các middleware chung.
- `wsbridge` đang đặt `InsecureSkipVerify: true` cho WS upgrade (README: *"No CORS/origin allow-list"*). Endpoint MCP chạy trên HTTP và **không được** lặp lại lỗ hổng đó: spec Streamable HTTP yêu cầu server **xác thực header `Origin`** để chống DNS rebinding.

## Giải pháp đề xuất

### A. Package `internal/adapter/mcpserver/`

Chứa: constructor `NewHandler(deps)`, khai báo dependency (`wscompat.Registry`, `mcpv1.McpServiceClient`, `AuthValidator`), và `Mount(r chi.Router)`. Không chứa logic giao thức (CR-003) hay logic tool (CR-007) — chỉ là điểm lắp ghép, để diff nhỏ và dễ review.

### B. Route

| Path | Mục đích | Auth |
|------|----------|------|
| `POST/GET/DELETE /mcp` | Streamable HTTP endpoint (CR-003) | Bearer token MCP (CR-005/006) — **không** chấp nhận cookie `orca_session` (chống CSRF/confused deputy; xem CR-005) |
| `GET /.well-known/oauth-protected-resource` | Metadata RFC 9728 (CR-005) | Công khai |
| `GET /.well-known/oauth-authorization-server` | Metadata AS (CR-005) | Công khai |

Mount **ngoài** nhóm `authed` hiện tại vì MCP có middleware auth riêng (trả `401` kèm `WWW-Authenticate` theo spec, khác với `authMiddleware` hiện tại).

### C. Cấu hình (`internal/config`)

`MCP_ENABLED` (công tắc tổng cấp tiến trình, mặc định `false` — tắt trong production tới khi qua gate CR-015), `MCP_TENANT_DEFAULT_ENABLED` (**đã chốt, D6:** mặc định `true`; `mcp-service` đọc khi tạo lười hàng settings của tenant mới — tenant mới bật MCP theo mặc định, admin vẫn tắt được; không thay đổi các mặc định an toàn ở CR-012/013), `MCP_PUBLIC_BASE_URL` (dùng cho metadata + resource indicator), `MCP_ALLOWED_ORIGINS` (mặc định = `WS_ALLOWED_ORIGINS` khi không đặt), `MCP_MAX_REQUEST_BYTES`, `MCP_SESSION_IDLE_TTL`.

### D. Chống tấn công transport

- Kiểm `Origin`: nếu có, phải thuộc allow-list; không có `Origin` (client CLI/desktop) vẫn được phép nhưng bắt buộc có token.
- **Đã chốt & được phê duyệt (2026-10-01, D3), đang hiện thực trong mã:** env `WS_ALLOWED_ORIGINS` (CSV origin chính xác hoặc host pattern, vd `https://orca.example.com,https://*.example.com`) áp cho `wscompat.Handler`, `wsbridge.Handler` và `/mcp`. Không rỗng ⇒ thay `InsecureSkipVerify` bằng package `internal/adapter/originpolicy` (**đã hiện thực**: `originpolicy.go` + test; nối vào `wscompat.Handler`, `wsbridge.Handler` bằng `WithOriginPolicy`, kiểm TRƯỚC `websocket.Accept` và trả `403`; tự kiểm scheme+host vì `OriginPatterns` của thư viện chỉ so host); rỗng ⇒ giữ hành vi cũ (cho phép mọi origin) kèm WARN to khi khởi động để deployment hiện có không gãy. `MCP_ALLOWED_ORIGINS` mặc định bằng `WS_ALLOWED_ORIGINS` khi không đặt. Chi tiết: BE-MCP-SOL-002 §D.3.
- Giới hạn kích thước body, timeout đọc header, giới hạn số kết nối SSE đồng thời mỗi tenant/user.
- Rate limit hiện tại tính theo **request**; kết nối SSE sống lâu cần giới hạn riêng theo số stream mở (không để một agent chiếm hết).
- Nginx/proxy: kiểm `deploy` + `ci/check-nginx-admin-api-routing.sh` — phải cho phép SSE (tắt buffering, `proxy_read_timeout` đủ dài) và **không** làm rơi header `Mcp-Session-Id`/`MCP-Protocol-Version`/`Last-Event-ID`. Thêm kiểm tra tương tự cho `/mcp`.

## Acceptance Criteria

- [ ] `MCP_ENABLED=false` ⇒ `/mcp` trả 404; `true` ⇒ route hoạt động.
- [ ] Request có `Origin` ngoài allow-list ⇒ `403`.
- [ ] Test router: `/mcp` không đi qua cookie auth; `/.well-known/*` truy cập không cần token.
- [ ] Script CI kiểm cấu hình nginx cho `/mcp` (SSE, header) chạy xanh.
- [ ] Bảng tại `api-gateway/README.md` cập nhật hàng mới (Real/Stub) đúng sự thật.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** CORS cho client chạy trong trình duyệt (MCP web client) — mặc định không cho phép; mở theo allow-list có chủ đích.
- **Ngoài phạm vi:** giao thức (CR-003), token (CR-005/006).
