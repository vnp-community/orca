# frontend Solutions — MCP Authorization (v5)

**CRs:** [docs/crs/v5/mcp-authorization/](../../../../../../docs/crs/v5/mcp-authorization/README.md) (CR-MCP-005 mục D "Consent" và CR-MCP-006 "UI quản lý PAT" đều ghi là việc của frontend)
**Hợp đồng:** [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE v5:** [crs/v5/README.md](../../README.md) · **Phía backend:** [specs/backend-go/crs/v5/mcp-authorization/solutions](../../../../../backend-go/crs/v5/mcp-authorization/solutions/README.md)
**TDD:** [`specs/frontend/tdd/v5`](../../../../tdd/v5/00-index.md), `v4/02-auth-flow`, `v4/06-web-entry`

Feature này **có UI** (không thuộc nhóm "không UI"): 1 trang SPA web-only, 3 tab Settings > MCP cho người dùng, 2 tab chỉ admin.

## 1. Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| [FE-MCP-SOL-003](./FE-MCP-SOL-003-consent-page-connected-apps-and-clients.md) | CR-MCP-005 (UI) | Trang `/oauth/consent` + định tuyến/`return_to` tối thiểu ở `web/` + tab "Connected apps" + tab admin "OAuth clients" và "All grants" | Medium–Large (≈4–5 ngày) | ✅ Implemented — see Gaps |
| [FE-MCP-SOL-004](./FE-MCP-SOL-004-access-tokens-pat-ui.md) | CR-MCP-006 (UI) | Tab "Access tokens": danh sách, tạo, reveal secret một lần, thu hồi, snippet CLI | Medium (≈2–3 ngày) | ✅ Implemented — see Gaps |

Hai solution **không** định nghĩa lại phần nền của **FE-MCP-SOL-001** (`frontend/src/shared/mcp-types.ts`, `window.api.mcp.*`, `store/slices/mcp-slice.ts`, section Settings `mcp`, khung `McpSettingsPane`). Chúng chỉ thêm component tab và yêu cầu FE-001 mount.

## 2. Tính nhất quán BE ↔ FE (kiểm chéo với BE-MCP-SOL-005/006)

| Chủ đề | Giá trị dùng ở cả hai phía |
|---|---|
| Kênh | `mcp.consent.get/decide`, `mcp.grant.list/revoke`, `mcp.admin.client.list/setStatus`, `mcp.admin.grant.list/revoke`, `mcp.token.list/create/revoke`, `mcp.server.info` (đúng CONTRACT §2) |
| Kết quả `consent.decide` | `{ redirectUrl: string }` ⇒ `window.location.assign` (FE chỉ chấp nhận scheme `http:`/`https:`) |
| Điều hướng | `GET /oauth/authorize` ⇒ `302 /login?return_to=<RequestURI tương đối>` hoặc `302 /oauth/consent?request_id=<uuid>`; `return_to` hợp lệ chỉ `/oauth/authorize?…` hoặc `/oauth/consent?request_id=…` |
| Mã lỗi | `MCP_CONSENT_NOT_FOUND` (cả đã quyết/khác user), `MCP_CONSENT_EXPIRED`, `MCP_SCOPE_INVALID`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_TOKEN_TOO_LONG`, `MCP_TOKEN_LIMIT`, `MCP_KILL_SWITCH_ACTIVE`, `MCP_NOT_ADMIN`, `MCP_NOT_FOUND`, `MCP_DISABLED` — BE bọc lỗi gRPC thành `"CODE: msg"` (`mcpChannelError`, BE-005 §G) để regex `^([A-Z0-9_]+): ` của FE khớp |
| Secret PAT | chỉ ở kết quả `mcp.token.create`; FE giữ trong state cục bộ của dialog; BE không log args/result của kênh này |
| Trần hạn PAT | `McpServerInfo.maxTokenDays` = `min(90, tenant.maxTokenDays)` (BE-006) ⇒ FE `expiryOptions(maxTokenDays)` |
| Thu hồi | UI nói "trong khoảng một phút" khớp trần 30s cache gateway + RPC (BE-005 §D, BE-006 §C) |
| Trường thời gian/id | RFC 3339 UTC / UUID dạng chuỗi (C7) |

## 3. Re-verify: hiện trạng frontend so với giả định

| # | Giả định (README v5 / giao việc) | Thực tế (đã đọc mã) | Drift |
|---|---|---|---|
| 1 | Entry web ở `frontend/src/renderer/web/*` | Ở `frontend/src/renderer/src/web/` (`main.tsx`, `main-web-bootstrap.tsx`, `WebConnect.tsx`, `login/LoginPage.tsx`); HTML `frontend/src/renderer/web-index.html` | Có — sửa đường dẫn |
| 2 | `runtime/web/web-preload-api.ts` | `renderer/src/web/web-preload-api.ts` (có `createAdminApi`, `callRuntimeResult`) | Có — README v5 ghi sai đường dẫn |
| 3 | Có định tuyến theo đường dẫn / `/login` là route | **Không có router.** `WebRoot` chọn `LoginPage`/`App` theo `sessionUser`; `/login` chỉ là đích của `installAuthFailedRedirect` và nhờ nginx SPA fallback | Có — cần định tuyến tối thiểu cho `/oauth/consent` (FE-003 Bước 2) |
| 4 | Login hỗ trợ `return_to` | Không (grep = 0 trong `frontend/src` và `backend-go`); local login → `'/'`; SSO callback → `302 /`; cookie `SameSite=Strict` | Có — FE-003 Bước 1–2 (sessionStorage + JS forward); backend không đổi |
| 5 | `docs/STYLEGUIDE.md` | Chuẩn thật: `guides/STYLEGUIDE.md` (xác nhận lại) | Đã biết (README v5) |
| 6 | Khoá i18n `auto.mcp.scope.<id>` | `i18next` `nsSeparator=':'` ⇒ id `orca:read` không tra được; dùng `scopeI18nKey` (`:`→`_`) | Có — diễn giải CONTRACT, không đổi CONTRACT |
| 7 | `ConfirmationDialogProvider` luôn có | Có trong `App`/Settings; **không** có ở shell tối giản của trang consent | Có — consent không dùng `useConfirmationDialog` |
| 8 | Budget lint | `.tsx` ≤ 400, `.ts` ≤ 300 dòng (`.oxlintrc.json`); `main-web-bootstrap.tsx` đã 461 dòng thô ⇒ logic ở file mới | Ràng buộc mới |
| 9 | Hạ tầng route | `deploy/dev/docker/nginx/orca.conf` chỉ proxy danh sách cố định; `/oauth/*`, `/.well-known/*`, `/mcp` hiện rơi vào SPA fallback; `/oauth/consent` phải **ở lại** SPA kèm header chống framing | Có — yêu cầu cho BE-005 §E; FE-003 liệt kê |

## 4. Thứ tự thực thi & phụ thuộc

1. **FE-MCP-SOL-001** (nền: types, `window.api.mcp`, slice, section `mcp`, `McpSettingsPane`) — chặn cả hai.
2. **FE-MCP-SOL-003** Bước 1–3 (`oauth-return-to.ts`, sửa `WebRoot`, trang consent, `mcp-scope-presentation.ts`, `lib/mcp-error-code.ts`) — cần BE-MCP-SOL-005 PR 4 (+ nginx/ingress) để chạy thật; phát triển được trước bằng mock `window.api.mcp`.
3. **FE-MCP-SOL-003** Bước 4–5 (Connected apps, admin tabs) — cần BE-005 PR 4.
4. **FE-MCP-SOL-004** — dùng `mcp-scope-presentation.ts` và `parseMcpError` của FE-003 (nên merge FE-003 Bước 3 trước); cần BE-006 PR 5.

Có thể song song: FE-003 Bước 4–5 với FE-004 sau khi đã có file dùng chung.

## 5. Yêu cầu cho các solution khác / CONTRACT

- **FE-MCP-SOL-001:** (a) mount `McpConnectedAppsTab` (id `apps`), `McpAccessTokensTab` (id `tokens`) cho mọi user và `McpOAuthClientsTab` (id `clients`), `McpAllGrantsTab` (id `grants`) chỉ khi `currentUser?.role === 'admin'`; (b) xuất selector `McpServerInfo` đã nạp và action nạp lại; (c) chốt tên `window.api.mcp.*` — nếu khác quy ước kênh→namespace, FE-003/004 chỉ đổi dòng gọi.
- **BE-MCP-SOL-005 / hạ tầng:** block nginx cho `location = /oauth/consent` với `Cache-Control: no-store`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, và **không** proxy `/oauth/consent` về gateway.
- **CONTRACT:** không cần sửa. Gợi ý làm rõ ở §3 rằng `/oauth/consent` không được proxy tới gateway (đã ghi ở README BE).

## 6. Không thuộc feature này

Hộp thoại phê duyệt tool và deep link Web Push (FE-MCP-SOL-009), policy/kill switch (FE-MCP-SOL-008), phiên MCP đang hoạt động (FE-MCP-SOL-002), đăng ký client MCP ngoài (FE-MCP-SOL-011).
