# FE-MCP-SOL-012: Kiểm thử (vitest + Playwright), telemetry UI, tài liệu người dùng, hành vi rollout

> 🔲 Designed — chưa implement. Chạy xuyên suốt cùng các FE-MCP-SOL-001..011; mỗi solution đó tự mang test của nó theo bảng §1, solution này định nghĩa khung chung, e2e, telemetry, tài liệu, rollout.

## CR Reference

- **CR:** [CR-MCP-015](../../../../../../docs/crs/v5/mcp-quality-rollout/CR-MCP-015-conformance-e2e-observability-rollout.md) — **chỉ phần FE**: kiểm thử UI/e2e (consent, token, approval), telemetry UI, tài liệu người dùng (mục D của CR), hành vi UI theo giai đoạn rollout (mục E). Conformance giao thức/metrics/alert là backend ([BE-MCP-SOL-015](../../../../../backend-go/crs/v5/mcp-quality-rollout/solutions/BE-MCP-SOL-015-conformance-e2e-observability-rollout.md)).
- **Mức độ:** 🟠 P1 (gate trước GA).
- **TDD:** [`tdd/v5/00-index.md`](../../../../tdd/v5/00-index.md) (nguyên tắc test/cleanup), `tests/e2e/AGENTS.md` (xác nhận bằng DOM, không bằng store; ưu tiên unit test slice khi logic thuần).

## Backend dependency

| Thứ cần | Từ | Dùng cho |
|---|---|---|
| Mọi kênh `mcp.*`, `McpEvent` | CONTRACT §2 (BE-003..014) | e2e trên backend dev |
| Stack dev có `MCP_ENABLED=true`, `MCP_TENANT_DEFAULT_ENABLED=true` | BE-015 §A (workflow) / `deploy/dev` | e2e token/approval |
| Script seed `backend-go/tests/mcpconformance/cmd/seed-dev` (user thường + admin, 1 PAT, 1 approval treo) | BE-015 "Hợp đồng với frontend" | dựng trạng thái cho Playwright |
| `POST /mcp` + PAT | BE-003/006 | spec approval gọi `tools/call` thật để sinh approval (có thể dùng client Python SDK chính thức của job `backend-go/ci/mcp-conformance/` — BE-015 §A.2, D4 — thay cho client viết tay; API SDK chưa xác minh, kiểm khi cài đặt) |

Khi BE chưa có: toàn bộ **vitest** (mock `mcpClient`) vẫn chạy; Playwright dev-backend tự `test.skip` nếu `ORCA_E2E_MCP_BASE_URL` chưa đặt; chỉ spec consent (mock WS) luôn chạy được.

## Impact analysis (gitnexus) — **chưa chạy**

Chủ yếu file test mới. Sửa có sẵn: `shared/telemetry-events.ts` (`eventSchemas`): `impact({target:"eventSchemas", direction:"upstream"})` — kỳ vọng MEDIUM (validator ở main process + nhiều test schema); `hooks/useSettingsNavigationMetadata.ts` (thêm `badge` — đã nêu ở FE-001). `package.json` gốc (scripts) không phải symbol.

## Bối cảnh (đã xác nhận lại)

- Vitest: `cd frontend && npx vitest run --config config/vitest.config.ts <path>`; `include` `src/**/*.test.ts(x)` và `config/scripts/**/*.test.mjs`; test React dùng `// @vitest-environment happy-dom` + `@testing-library/react`/`user-event` + `cleanup()` thủ công (mẫu `web/__tests__/ConnectionStatusBanner.test.tsx`); slice test dựng `create<XSlice>()` (mẫu `store/slices/connectivity-status.test.ts`).
- Playwright hiện chỉ chạy **Electron**: `tests/playwright.config.ts` + `tests/e2e/global-setup.ts` build `out/main/index.js`; `testMatch: '**/*.spec.ts'`. Không có chạy trình duyệt thường. `@playwright/test ^1.59.1` có trong root và `frontend/` ⇒ có `page.routeWebSocket`. Frontend web: `vite` root `src/renderer`, entry `web-index.html`, `isWebClientLocation()` dựa trên `pathname.endsWith('/web-index.html')` hoặc `window.__ORCA_WEB_CLIENT__`.
- Wire WS của web client (dialect session-client, đọc `envelope.go`/`session_dialect.go`): gửi `{id, authToken:'cookie-auth', method, params}`; nhận `{id, ok:true, result, _meta:{runtimeId:'backend-go'}}` hoặc `{id, ok:false, error:{code:'internal', message}, _meta:{runtimeId:null}}`; luồng: ack `result:null` rồi các frame `{…, streaming:true, result:<McpEvent>}`, kết thúc `result:{type:'end'}`.
- Telemetry: `track(name, props)` (`lib/telemetry.ts`) an toàn khi không có bridge; **web: `telemetryTrack` là no-op**. Sự kiện phải khai báo trong `eventSchemas` (`shared/telemetry-events.ts`, zod `.strict()`, đổi phá vỡ ⇒ tên sự kiện mới).
- Docs: `docs/guides/<chủ đề>/*.md` tiếng Việt (ví dụ `docs/guides/task-automation/jira-task-source-setup.md`).
- Nhãn `badge` của nav section tồn tại (`SettingsNavSection.badge?`, dùng ở mục `dev`).

## Giải pháp

### Bước 1 — Ma trận kiểm thử theo solution (vitest)

| Solution | Tệp test (cạnh mã) | Trọng tâm |
|---|---|---|
| FE-001 | `shared/mcp-types.test.ts`, `runtime/runtime-mcp-error.test.ts`, `runtime-mcp-client.test.ts`, `web/web-mcp-api.test.ts`, `store/slices/mcp-slice.test.ts`, `lib/mcp-deep-link.test.ts`, `hooks/useSettingsNavigationMetadata` (mở rộng test hiện có), `hooks/useMcpSync.test.tsx` | xem FE-001 "Verification"; `useMcpSync`: không đăng ký luồng khi `!authed`/`!enabled`, cleanup khi unmount, **không** đăng ký lại khi re-render |
| FE-002 | `lib/mcp-connect-snippets.test.ts`, `hooks/useMcpSessions.test.ts`, `McpConnectTab.test.tsx` | xem FE-002 |
| FE-003 | `ConsentPage.test.tsx`, `McpConnectedAppsTab.test.tsx` | scope bị bỏ chọn không gửi đi; `redirectHost` hiển thị, không hiện URL đầy đủ; `MCP_CONSENT_EXPIRED`; `window.location.assign` gọi đúng `redirectUrl` (spy) |
| FE-004 | `McpTokensTab.test.tsx`, `CreateTokenDialog.test.tsx` | **secret chỉ hiện một lần**: sau đóng dialog không còn trong DOM/state/store (`JSON.stringify(store.getState())` không chứa secret); `MCP_TOKEN_TOO_LONG` ánh xạ lỗi; trần ngày = `info.maxTokenDays` |
| FE-005..011 | test theo từng solution | quyền (admin vs user), trạng thái UI, mã lỗi CONTRACT |
| Mọi tab | `no-top-level-translate.test.ts` (có sẵn) phải xanh | |

**Khung dùng chung** `frontend/src/renderer/src/lib/mcp-test-fixtures.ts` (NEW; chỉ import trong `*.test.ts(x)`):

```ts
export const makeMcpServerInfo = (o: Partial<McpServerInfo> = {}): McpServerInfo => ({ enabled: true, resourceUrl: 'https://orca.example.com/mcp',
  protocolVersions: ['2025-06-18'], authorizationServer: 'https://orca.example.com', dcrEnabled: true, maxTokenDays: 90,
  killSwitch: { active: false }, scopesSupported: [ /* 4 scope CONTRACT */ ], ...o })
export const makeMcpSession/Token/Grant/Approval/ToolView/AuditEntry = (o = {}) => ({ /* giá trị hợp lệ theo CONTRACT §1 */ ...o })
export function createFakeMcpBridge(handlers: Partial<{ [M in McpRpcMethod]: (p: McpRpcParams<M>) => McpRpcResult<M> | Promise<McpRpcResult<M>> }>): {
  bridge: McpBridgeApi; calls: Array<{ method: string; params: unknown }>; emit: (e: McpEvent) => void; closeStream: () => void
}  // gọi method không có handler => throw Error('channel "x" is not yet implemented in backend-go …') giống gateway
export function installFakeMcpBridge(fake: ReturnType<typeof createFakeMcpBridge>): () => void // gán window.api.mcp ; trả hàm khôi phục
```

**Test hợp đồng chống lệch** — `shared/mcp-contract-drift.test.ts` (NEW): đọc `../specs/backend-go/crs/v5/CONTRACT-mcp-ui-api.md` nếu tồn tại (`describe.skipIf(!existsSync(...))` vì `frontend/` là bản "isolated copy" có thể tách repo), trích tên kênh bằng regex ``/\| `(mcp\.[A-Za-z.]+)` \|/g`` và mã lỗi từ §2.3, rồi khẳng định: tập kênh (trừ `mcp.events.subscribe`) **bằng** `MCP_RPC_METHODS`; `mcp.events.subscribe` ∈ `MCP_STREAM_METHODS`; tập mã lỗi **bằng** `MCP_ERROR_CODES`. Sửa CONTRACT mà quên FE ⇒ đỏ ngay.

### Bước 2 — Playwright cho web (tách khỏi bộ Electron)

**File:** `tests/playwright.web.config.ts` (NEW)

```ts
import { defineConfig, devices } from '@playwright/test'
export default defineConfig({
  testDir: './e2e/mcp-web',
  testMatch: '**/*.web.e2e.ts',            // KHÔNG khớp **/*.spec.ts của project Electron
  timeout: 60_000, expect: { timeout: 10_000 }, retries: 0, forbidOnly: !!process.env.CI, reporter: 'list',
  use: { baseURL: process.env.ORCA_E2E_WEB_URL ?? 'http://127.0.0.1:5174', trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: process.env.ORCA_E2E_WEB_URL ? undefined : {
    command: 'pnpm --dir frontend exec vite --port 5174', url: 'http://127.0.0.1:5174/web-index.html', reuseExistingServer: !process.env.CI
  }  // lệnh khởi động Vite chưa đối chiếu script `dev` thật — kiểm khi cài (frontend/package.json: "dev": "vite")
})
```

Script root `package.json` (MODIFY): `"test:e2e:mcp-web": "npx playwright test --config tests/playwright.web.config.ts"`. Không dùng `globalSetup` Electron.

**Hỗ trợ** (`tests/e2e/mcp-web/support/`):

| File | Nội dung |
|---|---|
| `mock-orca-ws.ts` | `mockOrcaWs(page, handlers)`: `page.routeWebSocket(/\/ws$/, ws => …)`; với mỗi frame `{id,method,params}` gọi `handlers[method]` ⇒ trả `{id, ok:true, result, _meta:{runtimeId:'backend-go'}}` hoặc `{ok:false, error:{code:'internal', message:'MCP_X: …'}}`; hỗ trợ `streamHandlers['mcp.events.subscribe']` (ack `null` rồi `push(event)` với `streaming:true`, `end()` gửi `{type:'end'}`); trả `calls[]` để assert; kênh không khai báo ⇒ lỗi "is not yet implemented" như gateway thật |
| `orca-session.ts` | `loginViaApi(request, context, {email, password})` (`POST /auth/local`, đặt cookie vào context) và `mockAuth(page, user)` (route `**/auth/me`, `**/auth/config`) cho spec mock |
| `mcp-dev-backend.ts` | đọc `ORCA_E2E_MCP_BASE_URL`, `ORCA_E2E_ADMIN_*`, `ORCA_E2E_USER_*`; `createPatViaApi`, `callMcp(request, pat, method, params)` (POST `/mcp` có `initialize` trước) |

**Spec** (đều khẳng định bằng DOM — `getByRole`/`getByText`/`toBeVisible` — theo `tests/e2e/AGENTS.md`; store chỉ để dựng trạng thái):

| Tệp | Backend | Kịch bản |
|---|---|---|
| `consent.web.e2e.ts` | **mock WS** (trang consent độc lập, không tải shell — CONTRACT §3) | (1) mở `/oauth/consent?request_id=r1` ⇒ thấy tên client, `redirectHost` (không URL đầy đủ), danh sách scope đã chọn sẵn; (2) bỏ chọn một scope + Approve ⇒ `calls` chứa `mcp.consent.decide {requestId:'r1', decision:'approve', scopes:[…đã giảm…]}` và trình duyệt điều hướng tới `redirectUrl` (route fulfill `https://client.example/cb*`); (3) Deny ⇒ `decision:'deny'`; (4) `MCP_CONSENT_EXPIRED` ⇒ trạng thái lỗi "expired", không nút Approve; (5) `isNewClient && registeredViaDcr` ⇒ cảnh báo "unverified application" hiển thị; (6) chưa đăng nhập ⇒ chuyển `/login` (hành vi do FE-003) |
| `token.web.e2e.ts` | dev backend | admin/user tạo PAT 7 ngày với scope `orca:read` ⇒ secret hiện **một lần** (`getByTestId('mcp-token-secret')`), Copy hoạt động; đóng dialog ⇒ secret biến mất, reload ⇒ chỉ thấy hàng token (prefix/tên, không secret); thu hồi ⇒ `status` đổi; dùng `callMcp` với secret ⇒ 200 trước khi thu hồi, 401 sau; `expiresInDays` > `maxTokenDays` ⇒ thông báo `MCP_TOKEN_TOO_LONG` |
| `approval.web.e2e.ts` | dev backend + seed | user đã đăng nhập mở app; test dùng PAT gọi `tools/call` tool `exec` (policy `require_approval`) ⇒ hộp thoại phê duyệt xuất hiện qua `mcp.event` (không reload) hiển thị `argsPreview.text` nguyên văn + `tool.risk`; Approve ⇒ lời gọi `tools/call` đang treo hoàn tất (assert response); ca Deny ⇒ `isError:true`; ca hết hạn (`approvalTtlSeconds` ngắn trong seed) ⇒ nút bị vô hiệu + `MCP_APPROVAL_EXPIRED`; ca mở **deep link** `/settings?section=mcp&tab=approvals&approval=<id>` ⇒ tab Approvals mở và dòng được focus (cần SPA fallback cho `/settings` — xem rủi ro) |
| `rollout.web.e2e.ts` | dev backend (hai biến thể stack) | `MCP_ENABLED=false`: Settings không có mục MCP, không console error; `enabled` bật: có mục (kèm badge giai đoạn); admin bật kill switch ⇒ banner xuất hiện **trong vài giây không cần reload** ở phiên user khác và `Connect` snippet vẫn copy được; tenant tắt + admin ⇒ thẻ "MCP is turned off for your organization" |

Spec có thể `test.skip(!process.env.ORCA_E2E_MCP_BASE_URL, 'needs MCP dev backend')`. Workflow: thêm job `web-e2e` vào `.github/workflows/backend-go-mcp-conformance.yml` (BE-015) sau khi stack lên: `pnpm --dir frontend build && ORCA_E2E_WEB_URL=… pnpm run test:e2e:mcp-web`; riêng `consent.web.e2e.ts` chạy ở workflow PR của frontend (không cần backend) — workflow frontend hiện có là `pr.yml` (**chưa xác minh** có chạy vitest FE hay không; đọc khi cài).

### Bước 3 — Telemetry UI

**File:** `frontend/src/shared/mcp-telemetry-events.ts` (NEW) — schema `.strict()`, **không** chứa id, tên client, URL, text tự do:

```ts
export const mcpEventSchemas = {
  mcp_settings_opened: z.object({ tab: mcpTabEnum, role: z.enum(['admin', 'user']) }).strict(),
  mcp_connect_snippet_copied: z.object({ client: z.enum(['claude-code', 'claude-desktop', 'cursor']), mode: z.enum(['oauth', 'token']) }).strict(),
  mcp_session_closed: z.object({ outcome: z.enum(['success', 'error']) }).strict(),
  mcp_consent_decided: z.object({ decision: z.enum(['approve', 'deny']), scope_count: z.enum(['1', '2', '3+']), new_client: z.boolean(), via_dcr: z.boolean(), narrowed: z.boolean() }).strict(),
  mcp_token_created: z.object({ lifetime_bucket: z.enum(['<=7d', '<=30d', '<=90d']), scope_count: z.enum(['1', '2', '3+']) }).strict(),
  mcp_token_revoked: z.object({}).strict(),                       // dùng placeholder `_v` như smart_to_recent_switch nếu zod empty-object gây vấn đề (telemetry-events.ts:1220)
  mcp_approval_decided: z.object({ decision: z.enum(['approve', 'deny']), risk: riskEnum, via: z.enum(['dialog', 'inbox', 'deeplink']), latency_bucket: z.enum(['<10s', '<60s', '<10m', '>=10m']) }).strict(),
  mcp_killswitch_toggled: z.object({ scope: z.enum(['tenant', 'client', 'grant', 'session']), active: z.boolean() }).strict()
} as const
```

**MODIFY** `shared/telemetry-events.ts`: spread `...mcpEventSchemas` vào `eventSchemas` (thêm tên sự kiện mới là tương thích — quy tắc đã ghi ở file). **File:** `renderer/src/lib/mcp-telemetry.ts` (NEW) — hàm `trackMcpConsentDecided(...)`… gói `track()` + hàm chia xô (`bucketLifetimeDays`, `bucketLatencyMs`); các component gọi hàm này, không gọi `track` trực tiếp ⇒ dễ test và không rò dữ liệu. Test: `mcp-telemetry-events.test.ts` (mỗi schema chấp nhận mẫu hợp lệ, **từ chối** thêm field lạ như `client_name`, `token`), `mcp-telemetry.test.ts` (bucket biên). **Thực tế web:** vì `telemetryTrack` no-op, nguồn số liệu rollout đáng tin là metric backend (`orca_mcp_*`); schema chuẩn bị sẵn cho bridge sau.

### Bước 4 — Tài liệu người dùng (vi) — `docs/guides/mcp/`

| Tệp (NEW) | Nội dung |
|---|---|
| `README.md` | MCP trong Orca là gì; ai cần (người dùng vs admin); sơ đồ ngắn luồng kết nối; điều kiện (admin đã bật; xem `server.info`) |
| `connect-claude-code.md`, `connect-claude-desktop.md`, `connect-cursor.md` | các bước dùng snippet ở tab Connect (cả OAuth và token), cách xác nhận kết nối (tab Active sessions), xử lý lỗi thường gặp (URL không HTTPS, token hết hạn, bị kill switch) — **đối chiếu tài liệu hiện hành của từng client trước khi xuất bản** |
| `personal-access-tokens.md` | tạo/thu hồi PAT, scope, hạn tối đa (`maxTokenDays`), vì sao secret chỉ hiện một lần, `POST /v1/auth/mcp-tokens` cho script |
| `approving-agent-actions.md` | hộp thoại phê duyệt: đọc `argsPreview` thế nào, mức rủi ro, hết hạn, thông báo đẩy/deep link |
| (BE-015 viết) `admin-security-model.md`, `runbook-mcp-operations.md` | quyền admin, kill switch, vận hành |

Giữ đồng bộ với UI bằng một bảng "chuỗi UI ↔ tài liệu" trong PR (không tự động hoá).

### Bước 5 — Hành vi rollout & trạng thái rỗng (không thêm cờ build)

| Giai đoạn (BE-015 §E) | Điều khiển | Hành vi UI |
|---|---|---|
| Tắt (mặc định, process) | `MCP_ENABLED=false` | không mục MCP, không lỗi; deep link bỏ qua |
| Tenant đã tắt (admin tắt, hoặc vận hành đặt `MCP_TENANT_DEFAULT_ENABLED=false`; **D6:** mặc định là bật) | `server.info.tenantEnabled=false` | user thường: không thấy gì; admin: thẻ "MCP is turned off for your organization" + công tắc (FE-008) — nhờ `mcpAdminSetupAvailable` (FE-001) |
| 0 Nội bộ / 1 Dogfood / 2 Beta | `enabled=true` | mục MCP có `badge` "Beta" (`translate('auto.mcp.nav.badge','Beta')`) — hằng `MCP_UI_STAGE = 'beta'` trong `lib/mcp-labels.ts`; GA = đổi hằng thành `'ga'` (không badge) |
| Pack `exec` chưa mở | server không liệt kê tool exec | tab Tools (admin, FE-005) hiển thị đúng thực tế; không văn bản cứng "exec available" |
| Kill switch | `killSwitch.active` | banner cố định (FE-001) trong mọi tab + cập nhật realtime qua `killswitch.changed` |
| Rollback | tắt `MCP_ENABLED`/tenant | lần `refreshMcpServerInfo` kế (visibility/5 phút) ẩn UI; phiên đang mở gặp `MCP_DISABLED` ⇒ slice đặt `enabled:false`, đóng luồng, không toast |

Trạng thái rỗng chuẩn (chuỗi dùng `auto.mcp.empty.*`): Active sessions "No agents are connected…" (FE-002); Tokens "You haven't created any access tokens."; Connected apps "No applications have access yet."; Approvals "Nothing is waiting for your approval."; Audit/Tools/Policies theo FE tương ứng. Mỗi trạng thái rỗng có **một** hành động gợi ý (liên kết sang tab Connect/Tokens) thay vì để trống.

## Trạng thái UI

Solution không thêm màn hình. Hành vi cần kiểm: (a) badge giai đoạn trên nav; (b) thẻ admin khi tenant tắt; (c) mọi trạng thái rỗng ở trên; (d) không có console error khi tắt/không cài kênh (spec rollout bắt `page.on('console')` + `pageerror`).

## A11y & i18n & style

Test a11y tối thiểu bằng `getByRole` (dialog có tên, bảng có `columnheader`, nút có nhãn) — nếu dự án đã có axe cho Playwright thì dùng (**chưa xác minh**). Chuỗi docs không dịch sang locale khác. Badge dùng token có sẵn của `SettingsNavSection.badge`.

## Files cần sửa

| File | Action |
|---|---|
| `tests/playwright.web.config.ts` | NEW |
| `tests/e2e/mcp-web/{consent,token,approval,rollout}.web.e2e.ts` | NEW |
| `tests/e2e/mcp-web/support/{mock-orca-ws,orca-session,mcp-dev-backend}.ts` | NEW |
| `package.json` (gốc) | MODIFY — script `test:e2e:mcp-web` |
| `frontend/src/renderer/src/lib/mcp-test-fixtures.ts` | NEW |
| `frontend/src/shared/mcp-contract-drift.test.ts` | NEW |
| `frontend/src/shared/mcp-telemetry-events.ts` (+ `.test.ts`) | NEW |
| `frontend/src/shared/telemetry-events.ts` | MODIFY — spread `mcpEventSchemas` |
| `frontend/src/renderer/src/lib/mcp-telemetry.ts` (+ `.test.ts`) | NEW |
| `frontend/src/renderer/src/lib/mcp-labels.ts` | MODIFY (FE-001) — `MCP_UI_STAGE` |
| `frontend/src/renderer/src/hooks/useSettingsNavigationMetadata.ts` | MODIFY — `badge` cho mục `mcp` theo `MCP_UI_STAGE` |
| `docs/guides/mcp/{README,connect-claude-code,connect-claude-desktop,connect-cursor,personal-access-tokens,approving-agent-actions}.md` | NEW |
| `.github/workflows/backend-go-mcp-conformance.yml` (BE-015) | MODIFY — job `web-e2e` |

## Verification

```bash
cd /opt/repos/orca/frontend
npx tsc --noEmit -p tsconfig.json
npx vitest run --config config/vitest.config.ts src/shared/mcp-contract-drift.test.ts src/shared/mcp-telemetry-events.test.ts \
  src/renderer/src/lib/mcp-telemetry.test.ts src/renderer/src/store/slices/mcp-slice.test.ts
npx vitest run --config config/vitest.config.ts -t mcp            # toàn bộ test MCP
cd /opt/repos/orca
npx playwright test --config tests/playwright.web.config.ts tests/e2e/mcp-web/consent.web.e2e.ts       # không cần backend
ORCA_E2E_MCP_BASE_URL=http://localhost:8081 ORCA_E2E_WEB_URL=http://localhost:8081 pnpm run test:e2e:mcp-web   # cần stack BE-015
pnpm run test:e2e -- --list | grep -c mcp-web                      # phải = 0: Electron project không nhặt spec web
```

## Sửa TDD kèm theo

`tdd/v5/00-index.md`: bổ sung mục "Kiểm thử web (Playwright web config tách khỏi Electron)". Ghi chú vào `tests/e2e/AGENTS.md`: thư mục `mcp-web/` dùng config riêng, hậu tố `.web.e2e.ts`.

## Rủi ro & phụ thuộc

- `/oauth/consent` cần được máy chủ trả về SPA (fallback nginx/Vite) — **chưa xác minh** cấu hình hiện tại; nếu không, spec consent phải chạy dưới `web-index.html` hoặc BE-002 thêm `location` fallback (đề nghị cho FE-003). Deep link phê duyệt dùng `/?section=mcp&tab=approvals&approval=<id>` (đường dẫn gốc, D5) nên không cần fallback `/settings`.
- `page.routeWebSocket` mô phỏng dialect session-client: nếu BE đổi envelope, spec mock gãy — `mock-orca-ws.ts` là điểm sửa duy nhất; có test hợp đồng cho envelope trong `web/web-mcp-api.test.ts`.
- Telemetry web là no-op ⇒ không đo được funnel UI cho tới khi có bridge.
- Spec approval phụ thuộc đồng hồ (TTL) — dùng `approvalTtlSeconds` nhỏ từ seed, không `waitForTimeout` cố định.

## Không làm ở solution này

Conformance giao thức, metrics, alert, workflow chính (BE-015); đánh giá chất lượng quyết định LLM; thêm cờ build FE; sửa `tests/playwright.config.ts` (Electron); viết UI từng tab.
