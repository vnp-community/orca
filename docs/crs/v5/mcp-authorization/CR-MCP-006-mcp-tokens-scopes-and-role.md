# CR-MCP-006 — Token MCP dài hạn, scope, và khôi phục `Role` cho nhánh Bearer

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-006 |
| **Tên** | Audience `orca-mcp`, mô hình scope, PAT cho headless/CI, và điền `Identity.Role` khi xác thực bằng Bearer |
| **Loại** | Feature + sửa lỗ hổng chức năng |
| **Priority** | 🔴 P0 — nếu không, mọi tool có kiểm quyền admin sẽ fail-closed, còn tool thường chạy với quyền không rõ ràng |
| **Effort** | Medium (4–6 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `wscompat/registry.go`, `auth_cli_token_routes.go` |
| **Phụ thuộc** | [CR-MCP-005](./CR-MCP-005-oauth21-resource-server.md) (cùng nhau) |

---

## Bối cảnh & Vấn đề

1. **Sai audience.** `auth_cli_token_routes.go:13` cố định `aud = "orca-cli"` và nói rõ *"never accepted from the request"*. Token CLI không thể (và không nên) dùng làm token MCP.
2. **Thiếu `Role` ở nhánh Bearer.** `Identity.Role` trong `registry.go:29-35` chỉ được điền ở nhánh cookie/session; chú thích ghi *"never by the bearer-JWT path… empty means unknown, never trust it"*. Hệ quả cho MCP (chỉ dùng Bearer): mọi channel gọi `requireAdmin` sẽ trả `INFRA_NOT_ADMIN` — đúng với fail-closed, nhưng một **admin thật** dùng agent cũng bị chặn, còn các kiểm tra khác dựa trên Role có thể hành xử không nhất quán.
3. **Chưa có scope.** JWT hiện tại biểu thị "user này", không biểu thị "user này cho phép client này làm *những gì*". Agent không nên mặc nhiên có toàn quyền của người dùng.
4. **Headless/CI** (agent chạy nền, không có trình duyệt) cần token dài hạn, thu hồi được, hiển thị trong UI.

## Giải pháp đề xuất

### A. Audience & loại token

| Loại | `aud` | Vòng đời | Nguồn |
|------|-------|---------|-------|
| OAuth access token (CR-005) | URL resource MCP | ≤ 15 phút | luồng OAuth |
| Refresh token | — | xoay vòng | luồng OAuth |
| **MCP PAT** | URL resource MCP | tối đa 90 ngày, bắt buộc có hạn | `POST /v1/auth/mcp-tokens` (mẫu theo `cli-tokens`) |

PAT: chỉ hiển thị **một lần** khi tạo; lưu hash; có `jti` để thu hồi; liệt kê/thu hồi ở `GET/DELETE /v1/auth/mcp-tokens`; ghi audit mỗi lần tạo/dùng lần đầu/thu hồi. Hạn chế `user_id` và `aud` ở phía server (không nhận từ body — cùng quy ước `createUserRequestBody`).

### B. Mô hình scope

Scope thô theo domain + mức: `orca:read`, `orca:write`, `orca:exec` (chạy lệnh/terminal/agent), `orca:admin`, thêm scope mịn theo namespace khi cần (`orca:git:write`, `orca:terminal:exec`…). Mỗi tool khai báo scope tối thiểu (CR-007). Quy tắc: **scope hiệu lực = giao(scope token, quyền của user, policy tenant CR-012)**. Token không bao giờ nâng quyền vượt user.

### C. Điền `Role` cho nhánh Bearer

Trong `AuthValidator` (api-gateway `usecase/validate_identity.go`): sau khi xác minh JWT, tra `Role` hiện hành của user bằng một lời gọi `auth-service` (có cache ngắn ≤ 30s, vô hiệu khi đổi role/vô hiệu hoá user) thay vì tin claim cũ hoặc để trống. Áp riêng cho nhánh MCP trước; nếu an toàn thì mở rộng cho Bearer chung (**cần đánh giá tác động** — `gitnexus_impact` trên `AuthValidator.Validate`/`Identity` trước khi sửa, và ghi rõ blast radius vào PR).
User bị `deactivate` hoặc `force-revoke` ⇒ token MCP vô hiệu ngay (kiểm `jti`/trạng thái user mỗi request, hoặc qua cache có sự kiện huỷ).

### D. Gắn với tenant

Token MCP thuộc đúng một tenant; user thuộc nhiều tenant phải chọn tenant lúc consent/tạo PAT. Không cho client đổi tenant bằng tham số tool.

## Acceptance Criteria

- [ ] Token `aud=orca-cli` bị `/mcp` từ chối; token MCP bị các route REST/WS khác từ chối.
- [ ] Admin dùng MCP gọi được tool admin (đã được policy cho phép); user thường gọi ⇒ bị từ chối với thông điệp rõ.
- [ ] PAT: tạo → dùng → thu hồi; sau thu hồi ≤ 60s thì `401`; hết hạn ⇒ `401`.
- [ ] Hạ quyền user (admin→user) có hiệu lực trong ≤ TTL cache.
- [ ] Scope `orca:read` không gọi được tool `orca:write` (test bảng scope × tool).
- [ ] Không log PAT/JWT; secret scanner của CI không báo.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** thay đổi ngữ nghĩa `Role` ảnh hưởng nhánh Bearer của mobile/CLI ⇒ giới hạn phạm vi sang nhánh MCP trước.
- **Ngoài phạm vi:** UI quản lý PAT (frontend), nội dung policy (CR-012).
