# CR-CLI-002 — Credential không tương tác (headless) cho CLI khi chạy CI/CD

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CLI-002 |
| **Tên** | Thêm cơ chế cấp bearer JWT dài hạn (API token / service-account) ở `auth-service` cho CLI headless — không tự chế cơ chế mới |
| **Loại** | Feature + Security |
| **Priority** | 🟡 P1 (chặn use case CI/CD của F09 §"CI/CD Integration") |
| **Effort** | Medium — cần thiết kế domain mới ở `auth-service`, nhưng verify path phía `api-gateway` **giữ nguyên** |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai — cần review bảo mật (scope/expiry/revocation) trước khi implement, xem "Giải pháp đề xuất" |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F09 ở lớp backend-go" |
| **Tác động HLD** | C2 (F09), liên quan C3 (F23 Multi-User Auth) |
| **Tác động Features** | F09 (Orca CLI), F23 (Multi-User Auth) |
| **Phụ thuộc** | CR-CLI-001 (transport backend-go phải tồn tại để dùng token này); độc lập với CR-RBAC-002/003 nhưng nên đi sau khi role-claim propagation (CR-RBAC-002) ổn định vì token mới cũng cần mang role claim đúng |

---

## Bối cảnh & Vấn đề

F09's use case "CI/CD Integration" và "Automation Script" (`docs/features/F09-orca-cli.md` dòng 89-105) giả định CLI chạy **hoàn toàn không tương tác** — không có ai ngồi login qua trình duyệt trước. CR-CLI-001 đề xuất transport backend-go mới cho CLI dùng bearer JWT xác thực qua `api-gateway`'s `SessionValidator.ValidateToken` (`backend-go/services/api-gateway/internal/adapter/authclient/session_validator.go:51`) — nhưng khảo sát `auth-service` cho thấy: **hiện không có cách nào để lấy 1 bearer JWT hợp lệ mà không đi qua luồng login tương tác**.

Bằng chứng cụ thể (grep trực tiếp, không có kết quả nào cho các khái niệm sau trong `backend-go/services/auth-service`):

- Không có `APIKey`/`PersonalAccessToken`/`ServiceAccount` trong domain (`grep -rli "api.?key\|personal access token\|pat_"` → rỗng).
- `session_validator.go:6` tự ghi chú rõ: giá trị cookie-session là "a raw, high-entropy token — never a JWT), not a bearer token" — tức bearer JWT hiện tại **chỉ sinh ra sau khi có 1 phiên đăng nhập** (OIDC/GitHub theo CR-RBAC context), không có đường "mint token trực tiếp cho máy/script".
- `CR-AUTO-008`'s audit về `HandleExternalTrigger` (`docs/crs/v4/automations/CR-AUTO-008-backend-go-rest-parity-webhook-auth.md`) xác nhận cùng 1 lỗ hổng ở domain khác: automation-service's external trigger **không có auth nào cả** (không phải vì có PAT mà check sai — mà vì hoàn toàn chưa có khái niệm token-cho-máy trong toàn bộ backend-go).

→ Đây là gap **có thật và chặn cứng** CR-CLI-001's mục tiêu CI/CD: nếu không có credential loại này, transport mới ở CR-CLI-001 chỉ dùng được cho user đã đăng nhập tương tác trước đó (vẫn hữu ích cho SSH/remote thủ công, nhưng không phục vụ được kịch bản GitHub Actions của chính F09's spec).

## Giải pháp đề xuất

**Không tự chế cơ chế xác thực mới cho CLI** — theo đúng ràng buộc của nhiệm vụ này và theo tinh thần CR-RBAC-002 (đang chuẩn hoá đúng 1 đường bearer-JWT cho toàn api-gateway). Đề xuất:

1. `auth-service` thêm 1 domain mới, hẹp phạm vi: **CLI/service token** — một JWT dài hạn (vd. 90 ngày, có thể revoke), ký cùng khoá với JWT phiên đăng nhập hiện tại (dùng lại `JWKSClient`/`jwks_client.go` phía verify — **không cần đổi gì ở `api-gateway`**, `SessionValidator.ValidateToken` xác thực được ngay vì cùng cơ chế JWKS).
2. Endpoint mint token: `POST /v1/auth/cli-tokens` (yêu cầu phiên đăng nhập tương tác hiện tại — user tự tạo token 1 lần qua UI hoặc `orca login` chạy trên máy có trình duyệt, sau đó copy token sang máy CI/CD headless qua secret manager của CI). Claim JWT mang role hiện tại của user tại thời điểm mint (tái dùng cơ chế role-claim của CR-RBAC-002, không thêm claim mới).
3. Revoke: liệt kê/thu hồi token qua Admin surface hiện có (sau khi CR-RBAC-001 cutover) hoặc 1 API riêng `DELETE /v1/auth/cli-tokens/{id}` — audit log mọi lần mint/revoke qua cùng `AuditEntry` domain của CR-RBAC-005 (không tạo audit trail riêng).
4. CLI: thêm biến môi trường `ORCA_API_TOKEN` (đọc bởi transport mới ở CR-CLI-001) và lệnh tiện ích `orca login --print-token` (chạy trên máy có phiên đăng nhập, in token để copy vào CI secret) — **không** lưu token vào file cấu hình CLI mặc định (tránh rò rỉ qua dotfile), chỉ qua biến môi trường hoặc flag tường minh.

### Cần security review trước khi triển khai

- Thời hạn token, cơ chế revoke, và việc token có nên scope theo project/repo (thay vì mang toàn bộ quyền của user) là quyết định bảo mật/sản phẩm — CR này đề xuất bản tối giản (1 token = toàn quyền của user, thời hạn cố định, revoke thủ công), **cần security/product xác nhận trước khi merge**, tương tự cách CR-RBAC-007 (SAML) cần business xác nhận trước khi triển khai.
- Việc token dài hạn tăng bề mặt tấn công nếu rò rỉ (đặc biệt nếu vô tình commit vào CI config) — nên khuyến nghị default thời hạn ngắn (vd. 30 ngày) + cảnh báo rõ trong tài liệu `orca login --print-token`.

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/services/auth-service/internal/domain/*.go` (mới) | Domain `CliToken`/tương đương: id, userId, issuedAt, expiresAt, revokedAt |
| `backend-go/services/auth-service/internal/usecase/*.go` (mới) | `MintCliToken`, `RevokeCliToken`, `ListCliTokens` |
| `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go` | Thêm route `POST /v1/auth/cli-tokens`, `GET /v1/auth/cli-tokens`, `DELETE /v1/auth/cli-tokens/{id}` |
| `backend-go/services/api-gateway/internal/adapter/authclient/session_validator.go` | Không đổi — xác nhận bằng test rằng `ValidateToken` chấp nhận JWT mới (cùng JWKS) |
| `desktop/src/cli/runtime/backend-go-transport.ts` (từ CR-CLI-001) | Đọc `ORCA_API_TOKEN` làm `authToken` khi có, ưu tiên hơn phiên đăng nhập cục bộ |
| `desktop/src/cli/handlers/*.ts` (mới lệnh) | `orca login --print-token` |
| `docs/features/F09-orca-cli.md` | Cập nhật use case CI/CD với hướng dẫn `ORCA_API_TOKEN` thật thay vì giả định |

## Không thuộc phạm vi CR này

- Scope token theo project/repo cụ thể (fine-grained) — v1 dùng toàn quyền user, xem "Cần security review".
- UI quản lý token (list/revoke) trong Admin surface — chờ CR-RBAC-001 cutover xong, dùng chung UI đó thay vì xây riêng.
- SSO/SAML machine-to-machine (client-credentials OAuth flow) — nếu enterprise cần sau này, đây là CR riêng, không mở rộng phạm vi ở đây.

## Tiêu chí chấp nhận

- [ ] `orca login --print-token` in ra 1 JWT hợp lệ, xác thực được qua `api-gateway`'s `SessionValidator.ValidateToken` không cần đổi code verify.
- [ ] Token hết hạn đúng thời điểm cấu hình; request sau khi hết hạn trả `401` rõ ràng, CLI hiển thị thông báo "token expired, run `orca login --print-token` again" (không phải lỗi chung chung).
- [ ] Revoke token có hiệu lực ngay (request tiếp theo bị từ chối) — test tự động.
- [ ] Audit log ghi nhận mint/revoke qua `AuditEntry` domain hiện có.
- [ ] Test end-to-end: script CI (GitHub Actions) chạy `orca worktree create --json` chỉ với `ORCA_SERVER_URL` + `ORCA_API_TOKEN` trong secrets, không có bước đăng nhập tương tác nào.

## Impact analysis (gitnexus)

Chưa chạy `impact()`/`context()` cho phần này — toàn bộ symbol đề xuất (`CliToken`, `MintCliToken`, route `/v1/auth/cli-tokens`) là **code mới, chưa tồn tại** trong codebase nên không có gì để phân tích blast-radius. Sẽ chạy `impact()` lên `SessionValidator.ValidateToken` và `AuthServiceClient` (upstream) ngay trước khi implement, theo đúng yêu cầu bắt buộc của CLAUDE.md, để xác nhận thêm route/domain mới không phá vỡ caller hiện tại của luồng bearer-JWT (đặc biệt sau khi CR-RBAC-002 propagate role claim qua cùng đường).

## Liên quan

- CR-CLI-001 (transport phụ thuộc token này)
- CR-RBAC-002 (role-claim propagation qua bearer JWT — dùng chung path)
- CR-RBAC-005 (audit log outcome/coverage — dùng chung `AuditEntry`)
- CR-AUTO-008 (`HandleExternalTrigger` — cùng loại gap "chưa có token-cho-máy" ở domain khác, tham khảo để không lặp lại thiết kế rời rạc)
- `docs/features/F09-orca-cli.md` (use case CI/CD)
