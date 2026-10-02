# backend-go Solutions — MCP Authorization (v5)

**CRs:** [docs/crs/v5/mcp-authorization/](../../../../../../docs/crs/v5/mcp-authorization/README.md)
**Hợp đồng:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Quy ước chung & bảng T1..T8:** [crs/v5/README.md](../../README.md)
**TDD tham chiếu:** [`auth-service.md`](../../../../tdd/services/auth-service.md), [`api-gateway.md`](../../../../tdd/services/api-gateway.md), `architecture/05`, `07`, `08` · **Phía frontend:** [specs/frontend/crs/v5/mcp-authorization/solutions](../../../../../frontend/crs/v5/mcp-authorization/solutions/README.md)

## 1. Solutions

| Solution | CR | Service / Area | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-005](./BE-MCP-SOL-005-oauth21-resource-server.md) | CR-MCP-005 | `auth-service` (AS: 9 RPC `OAuth*`, 6 bảng), `mcp-service` (consent/grant), `api-gateway` (`/.well-known/*`, `/oauth/*`, `adapter/mcpauth`, 8 kênh WS), nginx/ingress | Large (8–12 ngày) | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [BE-MCP-SOL-006](./BE-MCP-SOL-006-mcp-tokens-scopes-and-role.md) | CR-MCP-006 | `common/{jwtauth,mcpscope}`, `auth-service` (PAT, `ResolveMcpPrincipal`), `mcp-service` (policy PAT), `api-gateway` (`ValidateMCP`, REST + 3 kênh WS) | Medium (4–6 ngày) | ✅ Implemented (unit/integration tests) — see service README for gaps |

CONTRACT được phủ: kênh `mcp.consent.get/decide`, `mcp.grant.list/revoke`, `mcp.admin.client.list/setStatus`, `mcp.admin.grant.list/revoke` (005); `mcp.token.list/create/revoke` + REST `/v1/auth/mcp-tokens` (006); endpoint HTTP `/.well-known/*`, `/oauth/{register,authorize,token,revoke}` (005). Mã lỗi dùng: `MCP_CONSENT_NOT_FOUND/EXPIRED`, `MCP_SCOPE_INVALID`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_TOKEN_TOO_LONG`, `MCP_TOKEN_LIMIT`, `MCP_NOT_ADMIN`, `MCP_NOT_FOUND`, `MCP_DISABLED`.

## 2. Re-verify: khẳng định của CR so với mã thật (đã đọc trực tiếp, 2026-10-01)

| # | Khẳng định của CR | Kết quả | Lệch? |
|---|---|---|---|
| 1 | `auth_cli_token_routes.go:13` cố định `aud="orca-cli"`, "never accepted from the request" | Đúng (`const cliTokenAudience`, handler gắn `UserId: identity.UserID`) | Không |
| 2 | `Identity.Role` (`registry.go:29-35`) chỉ điền ở nhánh cookie | **Lệch một phần.** `AuthValidator.Validate` đã `Role: claims.Role` (CR-RBAC-002) và `jwtauth.Claims.Role` tồn tại; nhưng không đường mint nào đặt `Role` (`issue_service_token.go:88`, `complete_device_pairing.go:125`) ⇒ Bearer vẫn `Role==""`. Comment ở `registry.go` lỗi thời | Có — cơ chế khác, hệ quả giống. BE-006 xử lý đúng gốc (đọc role sống) |
| 3 | Verifier hiện tại chưa kiểm `aud` | Đúng: `VerifyWithKey` chỉ `exp/iat/iss`; `Validate` không kiểm `aud` ⇒ chiều "REST/WS từ chối aud MCP" cần mã mới | Không |
| 4 | "`auth-service` đã có GetJWKS, IssueServiceToken, CLI token, SSO" | Đúng (`auth.proto`). Lưu ý `internal/adapter/oauth` là client SSO, không phải AS | Không (cảnh báo trùng tên) |
| 5 | Client lưu ở `mcp-service.oauth_clients` (mục C) vs endpoint ở auth-service (mục B) | CR tự mâu thuẫn; CONTRACT/T3 đặt `oauth_clients` ở auth-service | Có — chọn auth-service (BE-005 quyết định 1) |
| 6 | Thu hồi qua `IsServiceTokenRevoked`-style hoặc deny-list, ≤ 60s | `RevocationClient` = 1 RPC sống/request, không cache; `issued_service_tokens` không có `tenant_id`; `DeactivateUser` không thu hồi token | Có — thay bằng `ResolveMcpPrincipal` + cache 30s |
| 7 | OAuth AS phải đi qua SSO/OIDC hiện có, không thêm đường đăng nhập thứ hai | SSO có thật (`StartSsoLogin/CompleteSsoLogin`), nhưng **không có `return_to`** ở backend lẫn frontend; callback luôn `302 /`; cookie `SameSite=Strict` | Có — cần cơ chế `return_to` ở SPA (BE-005 §H, FE-MCP-SOL-003) |
| 8 | `/mcp` cần `401 WWW-Authenticate` & từ chối cookie | Chưa có route `/mcp`; nginx dev chỉ proxy các đường dẫn liệt kê, còn lại rơi vào SPA fallback ⇒ `/oauth/*`, `/.well-known/*`, `/mcp` hiện trả HTML | Có — thêm block nginx/ingress (BE-005 §E) |
| 9 | auth-service README/TDD §7: không phụ thuộc service khác ở đường login lõi | Đúng; OAuth giữ nguyên: `mcp-service → auth-service` một hướng; `OAuthExchangeToken`/`ResolveMcpPrincipal` chỉ đọc bảng của auth-service | Không |
| 10 | "User thuộc nhiều tenant phải chọn tenant" | `auth.users` unique `(tenant_id,email)`, mỗi user một tenant ⇒ tenant = tenant của phiên | Có — bỏ bước chọn tenant |
| 11 | "Secret scanner CI không báo" | Không thấy cấu hình gitleaks/trufflehog trong `.github/` | (chưa xác minh) — đề xuất tiền tố `omp_` + task ở BE-015 |
| 12 | DCR ẩn danh, admin tenant có thể chặn DCR | DCR xảy ra trước khi biết tenant ⇒ registry toàn deployment + trạng thái theo tenant (`pending` khi tenant `dcrEnabled=false`) | Có — làm rõ ngữ nghĩa (BE-005 quyết định 2) |

## 3. Thứ tự thực thi & phụ thuộc

Phụ thuộc ngoài feature: BE-MCP-SOL-001/002 (scaffold `mcp-service`, `MCP_ENABLED`, proto package), BE-MCP-SOL-003 (mount `/mcp`, khung `channels_mcp.go`, `McpServerInfo`), BE-MCP-SOL-012 (`dcrEnabled`, `maxTokenDays`), BE-MCP-SOL-013 (hợp nhất audit, `McpEvent grant.revoked`).

Hai solution đan xen (CR README: "làm gần nhau; CR-006 cung cấp token + role mà CR-005 phát hành") nên chia PR như sau:

| PR | Nội dung | Thuộc |
|---|---|---|
| 1 | `jwtauth.Claims` thêm field; `common/mcpscope`; `ResolveMcpPrincipal` (phần PAT/OAuth trả `unknown_token` cho tới PR sau); `ValidateMCP`; `RejectAudiences` trong `Validate` | BE-006 §A–D |
| 2 | `/.well-known/*` + `adapter/mcpauth` (401/403, từ chối cookie) + block nginx/ingress | BE-005 §E–F (discovery) |
| 3 | Bảng + RPC OAuth (`Register`, `Validate`, `Exchange`, `Revoke`), `/oauth/{register,token,revoke}`, rate-limit | BE-005 §B–C, E |
| 4 | `mcp-service` consent/grant + `GET /oauth/authorize` + 8 kênh WS + Rego; `ResolveMcpPrincipal` đủ nhánh `mcp_oauth` | BE-005 §D, G |
| 5 | `mcp_tokens` + `IssueMcpToken/List/Revoke` + policy PAT ở mcp-service + REST + 3 kênh WS | BE-006 §E–F |

PR 1 và 2 độc lập với `mcp-service`; PR 4 chặn FE-MCP-SOL-003; PR 5 chặn FE-MCP-SOL-004. Trước PR 3 phải có đủ migration `0011` (OAuth) rồi `0012` (PAT) — đánh số lại nếu migration khác chen vào.

## 4. Quyết định xuyên suốt

- `auth-service` **không** gọi `mcp-service` ở bất kỳ đường nào; đồng bộ trạng thái thu hồi đi `mcp-service → auth-service` (đồng bộ + job reconcile).
- Gateway không chứa nghiệp vụ (T1/T2): chỉ dịch giao thức, gắn `Identity`, bọc lỗi `CODE: msg` (`mcpChannelError`).
- Mọi token/code/verifier/secret bị che khỏi log, trace và audit.

## 5. Đề nghị thay đổi CONTRACT

Không bắt buộc. Gợi ý (không chặn): (a) thêm vào §2.3 mã nội bộ `MCP_CLIENT_NOT_ALLOWED` chỉ nếu muốn FE hiển thị lý do client bị chặn — hiện được trả qua redirect `error=access_denied` nên **không cần**; (b) làm rõ ở §3 rằng `/oauth/consent` phải **không** được proxy tới gateway ở lớp nginx/ingress (hiện chỉ nêu "trang SPA mới").
