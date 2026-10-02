# CR-MCP-005 — OAuth 2.1 resource server cho MCP (metadata, đăng ký client, consent, PKCE)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-005 |
| **Tên** | Orca là OAuth 2.1 *resource server* của MCP; `auth-service` (hoặc IdP) là *authorization server*; hỗ trợ discovery + đăng ký client + consent |
| **Loại** | Feature (bảo mật) |
| **Priority** | 🔴 P0 — chặn mọi client MCP "chuẩn" kết nối an toàn |
| **Effort** | Large (8–12 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `api-gateway` authn, `auth-service` |
| **Phụ thuộc** | [CR-MCP-003](../mcp-protocol-server/CR-MCP-003-streamable-http-and-lifecycle.md), [CR-MCP-001](../mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md) |

---

## Bối cảnh & Vấn đề

Hiện có hai đường xác thực (api-gateway README): cookie `orca_session` (opaque, cho trình duyệt) và Bearer JWT RS256 xác minh qua JWKS của `auth-service` (cho mobile/CLI). Cả hai đều **không** phù hợp trực tiếp cho client MCP:

- Cookie là đặc quyền ambient của trình duyệt ⇒ cho phép dùng cookie ở `/mcp` mở ra CSRF/confused-deputy. **Quyết định: `/mcp` từ chối cookie.**
- Client MCP chuẩn khám phá auth bằng `401` + `WWW-Authenticate: Bearer resource_metadata="…"` rồi chạy luồng OAuth (authorization code + PKCE). Orca chưa có metadata, chưa có đăng ký client, chưa có màn consent.
- `auth-service` đã có `GetJWKS`, `IssueServiceToken`, CLI token, SSO (OIDC) — nhưng chưa có OAuth authorization server cho bên thứ ba.

## Giải pháp đề xuất

### A. Discovery

- `GET /.well-known/oauth-protected-resource` (RFC 9728): `resource` = `MCP_PUBLIC_BASE_URL + /mcp`, `authorization_servers`, `scopes_supported`, `bearer_methods_supported:["header"]`.
- Khi thiếu/sai token: `401` + `WWW-Authenticate: Bearer resource_metadata="<url>", scope="…"`; thiếu scope: `403` + `error="insufficient_scope"`.
- Authorization server metadata (RFC 8414) ở `/.well-known/oauth-authorization-server` (hoặc trỏ tới IdP ngoài nếu tenant dùng IdP riêng).

### B. Authorization server — đặt ở đâu

Đề xuất: endpoint `/authorize`, `/token`, `/register`, `/revoke` do **`auth-service`** phục vụ (nó đã sở hữu user/session/JWKS/ký khoá); `mcp-service` giữ **dữ liệu đặc thù MCP** (client đã đăng ký, grant/consent) qua gRPC. Lý do: không nhân đôi nơi giữ khoá ký và danh tính. **Cần ADR** nếu chọn hướng khác (ví dụ ủy quyền hoàn toàn cho IdP ngoài).

Yêu cầu bắt buộc:
- Authorization code + **PKCE (S256) bắt buộc**; từ chối `plain`, implicit, password grant.
- **Resource Indicators (RFC 8707)**: yêu cầu `resource` ở authorize/token; token cấp ra có `aud` = URL resource MCP. Server MCP **chỉ nhận token có `aud` đúng nó**.
- Redirect URI so khớp chính xác; chỉ `https` hoặc loopback (`http://127.0.0.1:*`/`localhost`) cho client native; cấm wildcard.
- Refresh token xoay vòng (rotation) + phát hiện tái sử dụng ⇒ thu hồi cả chuỗi.
- Access token ngắn hạn (≤ 15 phút khuyến nghị); `state` chống CSRF.

### C. Đăng ký client

- Hỗ trợ **Dynamic Client Registration (RFC 7591)** có kiểm soát (giới hạn tốc độ, chỉ client public + PKCE, tên/redirect_uri được kiểm tra) và, nếu spec bản triển khai có, **Client ID Metadata Documents**. Cho phép admin tenant chặn DCR và chỉ cho client đã duyệt (allow-list).
- Lưu ở `mcp-service.oauth_clients`; secret (nếu có client confidential) qua Vault.

### D. Consent

Màn hình consent trong web UI (cần CR frontend riêng — ghi nhận phụ thuộc, không nằm trong `backend-go`): hiển thị tên client, tenant, **scope bằng ngôn ngữ người dùng** ("Đọc project", "Chạy lệnh trong terminal"…), cho phép từng scope. Ghi `grants`; người dùng xem/thu hồi grant ở Settings. Lần đầu và mỗi lần mở rộng scope đều hỏi lại.

### E. Chống lỗi thiết kế đã biết

- **Cấm token passthrough:** token nhận ở `/mcp` **không** được chuyển tiếp nguyên trạng sang service khác. api-gateway đổi nó thành `Identity` nội bộ rồi gọi gRPC như mọi route khác.
- **Confused deputy:** consent theo từng (client, user, scope); không dùng chung consent giữa client khác nhau.
- Không log token/code; che trong trace.

## Acceptance Criteria

- [ ] Client MCP thật (MCP Inspector, Claude Code `--transport http`) tự khám phá, đăng ký, đăng nhập, gọi `tools/list` mà **không** cấu hình tay token.
- [ ] Token có `aud` sai ⇒ `401`; scope thiếu ⇒ `403 insufficient_scope`; cookie ⇒ `401`.
- [ ] PKCE thiếu/`plain` ⇒ từ chối; redirect URI lệch 1 ký tự ⇒ từ chối.
- [ ] Tái sử dụng refresh token cũ ⇒ cả chuỗi bị thu hồi + ghi audit.
- [ ] Thu hồi grant ở UI ⇒ token hiện có ngừng hiệu lực trong ≤ 60s (qua `IsServiceTokenRevoked`-style check hoặc `jti` deny-list).
- [ ] Test bảo mật: open-redirect, code replay, mix-up, DCR spam.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** phạm vi lớn — tách PR theo (discovery → DCR → authorize/token → consent).
- **Rủi ro:** SSO/OIDC của tenant đã cấu hình ⇒ OAuth AS phải đi qua đăng nhập SSO hiện có, không tạo đường đăng nhập thứ hai.
- **Ngoài phạm vi:** UI consent (frontend), token dài hạn (CR-006).
