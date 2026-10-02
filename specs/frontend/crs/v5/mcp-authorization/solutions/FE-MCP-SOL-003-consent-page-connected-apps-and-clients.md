# FE-MCP-SOL-003: Trang consent `/oauth/consent`, "Connected apps" và quản trị OAuth client

> 🔲 Designed — chưa implement. Cần backend [BE-MCP-SOL-005](../../../../../backend-go/crs/v5/mcp-authorization/solutions/BE-MCP-SOL-005-oauth21-resource-server.md) (PR 4) mới chạy end-to-end; hợp đồng: [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md).

## CR Reference

- **CR:** [CR-MCP-005](../../../../../../docs/crs/v5/mcp-authorization/CR-MCP-005-oauth21-resource-server.md) — mục D (Consent) mà CR ghi "ngoài phạm vi backend-go, cần CR frontend riêng".
- **Mức độ:** 🔴 P0 (chặn mọi client MCP chuẩn kết nối).
- **Phạm vi:** (1) trang SPA `/oauth/consent`; (2) tab "Connected apps" (người dùng); (3) hai tab admin: "OAuth clients" và "All grants"; (4) thay đổi định tuyến tối thiểu + xử lý `return_to`. Không gồm shell Settings > MCP, kiểu dùng chung, runtime client, slice (đó là **FE-MCP-SOL-001**).

## Backend dependency

| Kênh (CONTRACT) | Tham số → Kết quả | BE | Khi BE chưa có |
|---|---|---|---|
| `mcp.consent.get` | `requestId` → `McpConsentRequest` | BE-MCP-SOL-005 | trang hiện trạng "unavailable" (lỗi chung), không crash |
| `mcp.consent.decide` | `requestId`, `{decision:'approve'\|'deny', scopes}` → `{redirectUrl}` | BE-MCP-SOL-005 | như trên |
| `mcp.grant.list` / `mcp.grant.revoke` | `—` → `McpGrant[]` / `grantId` → `{ok:true}` | BE-MCP-SOL-005 | tab hiện lỗi + Retry |
| `mcp.admin.client.list` / `.setStatus` | `—` → `McpOAuthClient[]` / `clientId, 'allowed'\|'blocked'` → `McpOAuthClient` | BE-MCP-SOL-005 | như trên |
| `mcp.admin.grant.list` / `.revoke` | `{userId?}` → `McpGrant[]` / `grantId` | BE-MCP-SOL-005 | như trên |
| `mcp.server.info` | → `McpServerInfo` (`enabled`, `resourceUrl`) | BE-MCP-SOL-003 qua FE-MCP-SOL-001 | `enabled:false` ⇒ ẩn toàn bộ |

Lỗi dùng: `MCP_CONSENT_NOT_FOUND`, `MCP_CONSENT_EXPIRED`, `MCP_SCOPE_INVALID`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_NOT_ADMIN`, `MCP_NOT_FOUND`, `MCP_DISABLED` (tách theo `^([A-Z0-9_]+): `, CONTRACT C4). Điều hướng backend: `GET /oauth/authorize` → `302 /login?return_to=<RequestURI tương đối>` hoặc `302 /oauth/consent?request_id=<uuid>`.

Tên `window.api.mcp.*`, kiểu trong `frontend/src/shared/mcp-types.ts`, slice `store/slices/mcp-slice.ts`, section Settings `mcp` do **FE-MCP-SOL-001** định nghĩa; solution này **chỉ gọi** theo quy ước kênh→namespace (`mcp.consent.get` ⇒ `window.api.mcp.consent.get`). Nếu FE-001 chốt tên khác, chỉ đổi dòng gọi — kiểu và hành vi giữ nguyên. Danh sách tab sẽ do `McpSettingsPane` của FE-001 mount (tab id: `apps`, `clients`, `grants`; xem README feature).

## Impact analysis (gitnexus) — chưa chạy

| Symbol | Direction | Risk dự kiến | Lệnh chạy trước khi sửa |
|---|---|---|---|
| `WebRoot` (`web/main-web-bootstrap.tsx`) | upstream | MEDIUM (được `bootstrapWebApp` → `main.tsx` dùng; sửa nhánh render sau đăng nhập) | `gitnexus impact WebRoot --direction upstream` |
| `resolveHasEnvironment` | upstream | LOW (không sửa; chỉ đọc) | `gitnexus impact resolveHasEnvironment --direction upstream` |
| `LoginPage` | upstream | LOW (**không sửa**) | — |
| `ConfirmationDialogProvider`/`useConfirmationDialog` | upstream | LOW (chỉ tiêu thụ) | — |

Ghi blast radius vào PR; chạy `detect_changes` trước commit.

## Bối cảnh (đã xác nhận lại bằng mã thật)

- **Đường dẫn khác tài liệu giao việc:** entry web nằm ở `frontend/src/renderer/src/web/` (không phải `renderer/web/`): `main.tsx`, `main-web-bootstrap.tsx`, `WebConnect.tsx`, `login/LoginPage.tsx`, `web-preload-api.ts` (cũng **không** ở `runtime/web/` như README v5 ghi). HTML entry: `frontend/src/renderer/web-index.html` (`window.__orca_platform = 'web'`), build input `web-index` ở `frontend/vite.config.ts`.
- **Không có router theo đường dẫn.** `WebRootBoundary` gọi `fetchCurrentUser()`; `WebRoot` quyết định `LoginPage` / `App` chỉ theo `sessionUser` và environment — mọi URL (kể cả `/login`, `/oauth/consent`) hiện đều ra cùng kết quả. `/login` chỉ là đích của `installAuthFailedRedirect` (`window.location.href = '/login'`) và dựa vào nginx SPA fallback.
- **Không có `return_to` ở đâu** (`grep -rn "return_to\|returnTo" frontend/src` = 0). `LoginPage` gọi `onLoginSuccess` ⇒ `WebRoot` đặt `window.location.href = '/'`; `SsoButton` là `<a href="/auth/sso/{provider}">`; backend callback luôn `302 /`.
- Cookie `orca_session` là `SameSite=Strict` ⇒ lần điều hướng đầu tiên từ ngoài site (client MCP mở trình duyệt, hoặc 302 từ IdP) **không mang cookie**; điều hướng do JS cùng site thì có. Vì vậy SPA phải tự "đẩy tiếp" tới `return_to` sau khi `fetchCurrentUser()` thành công.
- `WebRoot` ở nhánh `sessionUser !== null` gọi `installWebPreloadApi()` (đặt `window.api`) rồi render `<App/>` bọc `ConnectionStatusProvider` + `WorkspaceProvider`. Trang consent cần `window.api.mcp` nhưng **không** được tải `App` (terminal/worktree) — CONTRACT §3.
- i18n: `i18next` mặc định `nsSeparator=':'` ⇒ khoá chứa id scope `orca:read` sẽ bị hiểu là namespace và **không bao giờ tra được bản dịch** (rơi về `defaultValue`). CONTRACT nói "ưu tiên khoá `auto.mcp.scope.<id>`" ⇒ dùng `<id>` đã chuẩn hoá `:`→`_` (`auto.mcp.scope.orca_read.label|description`). Cấm `translate()` ở top-level (`i18n/no-top-level-translate.test.ts`). Lint chạy `verify:localization-catalog` — mọi khoá mới phải có trong `i18n/locales/en.json`.
- UI: có `card`, `button`, `badge`, `checkbox`, `table`, `tabs`, `skeleton`, `dialog`, `sonner`; không có `alert`, `switch`, `alert-dialog` — dùng khối `div role="alert"`. `components/confirmation-dialog.tsx` (`useConfirmationDialog`, `confirmVariant:'destructive'`) cần `ConfirmationDialogProvider` — có trong `App` (Settings) nhưng **không** có ở shell tối giản của trang consent ⇒ trang consent không dùng nó.
- Style: chuẩn thật là `guides/STYLEGUIDE.md`: `destructive` chỉ cho mất dữ liệu/không hoàn tác, "Cancel/Dismiss/Close/Discard không phải destructive" (dòng 296), mặc định ghost; token ở `assets/main.css`. Budget oxlint: `.ts` ≤ 300, `.tsx` ≤ 400 dòng (không blank/comment); `main-web-bootstrap.tsx` đã 461 dòng thô ⇒ logic mới phải ở file riêng, phần sửa tại bootstrap ≤ ~12 dòng.
- `web-preload-api.ts` và `App.tsx`, `main.tsx` bất biến (TDD v5 nguyên tắc 10/11/19) — chỉ FE-001 **thêm** namespace `mcp`; `main-web-bootstrap.tsx` không nằm trong danh sách bất biến.

## Giải pháp

### Bước 1 — Mô-đun `return_to` và nhận diện route consent

**File:** `frontend/src/renderer/src/web/oauth-return-to.ts` (NEW, ~70 dòng, không phụ thuộc React)

```ts
// Why: allow-list tuyệt đối — return_to đến từ query nên không được trở thành open-redirect hay javascript: URL.
const RETURN_TO_ALLOWED = [/^\/oauth\/authorize\?/, /^\/oauth\/consent\?request_id=[A-Za-z0-9-]{8,64}$/]
const STORAGE_KEY = 'orca.oauth.returnTo.v1'      // sessionStorage: sống qua vòng IdP cùng tab, không sang tab khác
const ATTEMPT_KEY = 'orca.oauth.returnTo.attempts'
const MAX_ATTEMPTS = 2                            // chặn vòng lặp /login ⇄ /oauth/authorize khi cookie không khớp

export function sanitizeReturnTo(raw: string | null | undefined): string | null {
  if (!raw || raw.length > 4096 || !raw.startsWith('/') || raw.startsWith('//')) return null
  if (raw.includes('\\') || /[\u0000-\u001f]/.test(raw)) return null
  return RETURN_TO_ALLOWED.some((re) => re.test(raw)) ? raw : null
}
export function isOAuthConsentPath(loc: Pick<Location, 'pathname'>): boolean {
  return loc.pathname === '/oauth/consent'
}
// Gọi khi CHƯA đăng nhập: nhớ đích để quay lại sau local login lẫn SSO.
export function stashReturnToFromLocation(loc: Pick<Location, 'pathname' | 'search'>): void
// Gọi khi ĐÃ đăng nhập: trả đích cần chuyển tiếp (query ?return_to → stash), có đếm lượt; null nếu hết lượt.
export function takeForwardTarget(loc: Pick<Location, 'pathname' | 'search'>): string | null
export function clearReturnTo(): void
```

Mọi truy cập `sessionStorage` bọc `try/catch` (private window/blocked) — thiếu storage thì chỉ mất khả năng quay lại, không lỗi. Với `stashReturnToFromLocation`: nếu đang ở `/oauth/consent?request_id=…` mà chưa có phiên (hết hạn giữa chừng) thì stash chính URL đó; ngược lại stash `?return_to=` (đã `sanitize`).

### Bước 2 — Sửa tối thiểu `WebRoot`

**File:** `frontend/src/renderer/src/web/main-web-bootstrap.tsx` (MODIFY, +~12 dòng; `LoginPage`, `App.tsx`, `main.tsx`, `web-preload-api.ts` **không đổi**)

```tsx
const McpConsentPage = lazy(() =>
  import('../components/mcp/consent/McpConsentPage').then((m) => ({ default: m.McpConsentPage })))

// trong WebRoot, TRƯỚC mọi `return` có điều kiện (quy tắc hooks):
const forwardTo = useMemo(
  () => (sessionUser !== null && !isOAuthConsentPath(window.location) ? takeForwardTarget(window.location) : null),
  [sessionUser])
useEffect(() => { if (sessionUser === null) stashReturnToFromLocation(window.location) }, [sessionUser])
useEffect(() => { if (forwardTo) window.location.replace(forwardTo) }, [forwardTo])

if (sessionUser !== null) {
  if (forwardTo) return <div className="min-h-dvh bg-background" />        // không mount App khi đang chuyển tiếp
  /* …giữ nguyên: tạo environment 'session-auth' nếu chưa có + installWebPreloadApi()… */
  if (isOAuthConsentPath(window.location)) {
    return (
      <Suspense fallback={<div className="min-h-dvh bg-background" />}>
        <McpConsentPage />                     {/* không ConnectionStatusProvider/WorkspaceProvider/App */}
      </Suspense>)
  }
  /* …giữ nguyên: <ConnectionStatusProvider>… <App/> */
}
```

Luồng đầy đủ:

1. Client MCP mở `GET /oauth/authorize?…` → (có cookie) `302 /oauth/consent?request_id=…` → SPA, `sessionUser` có → trang consent.
2. (Không cookie hoặc Strict chặn) `302 /login?return_to=%2Foauth%2Fauthorize%3F…` → SPA: nếu `fetchCurrentUser()` đã có phiên ⇒ `takeForwardTarget` trả `return_to` ⇒ `location.replace` (cùng site, có cookie) ⇒ bước 1. Nếu chưa có phiên ⇒ `LoginPage` hiện, `stashReturnToFromLocation` lưu đích; đăng nhập local ⇒ `window.location.href='/'` (giữ nguyên) ⇒ `/` có phiên ⇒ `takeForwardTarget` đọc stash ⇒ chuyển tiếp. SSO: `<a href="/auth/sso/…">` ⇒ IdP ⇒ callback `302 /` ⇒ cùng đường. `sessionStorage` còn nguyên qua vòng IdP vì cùng tab/origin.
3. Hết lượt (`MAX_ATTEMPTS`) hoặc `return_to` không hợp lệ ⇒ rơi về `App` bình thường (không vòng lặp, không chuyển hướng ngoài).

Tuỳ chọn nhỏ (cùng PR): `installAuthFailedRedirect` hiện đặt `window.location.href = '/login'`; nếu `isOAuthConsentPath(location)` thì đặt `'/login?return_to=' + encodeURIComponent(location.pathname + location.search)` để consent hết phiên giữa chừng quay lại được.

**Yêu cầu hạ tầng (không phải FE, ghi để không bị bỏ sót):** nginx/ingress phải proxy `/oauth/(authorize|register|token|revoke)`, `/.well-known/`, `/mcp` tới api-gateway nhưng **để `/oauth/consent` rơi vào SPA fallback**, kèm header `Cache-Control: no-store`, `X-Frame-Options: DENY` (hoặc CSP `frame-ancestors 'none'`), `Referrer-Policy: no-referrer` cho riêng `location = /oauth/consent` (chống clickjacking trang cấp quyền). Hiện `deploy/dev/docker/nginx/orca.conf` chưa có các block này (BE-MCP-SOL-005 §E).

### Bước 3 — Trang consent

**Files (NEW):** `frontend/src/renderer/src/components/mcp/consent/McpConsentPage.tsx` (~200 dòng), `use-mcp-consent.ts`, `frontend/src/renderer/src/components/settings/mcp/mcp-scope-presentation.ts` và `frontend/src/renderer/src/lib/mcp-error-code.ts` (cả hai dùng chung với FE-MCP-SOL-004; nếu FE-MCP-SOL-001 đã cung cấp parser lỗi tương đương thì dùng cái đó và bỏ file này).

```ts
// lib/mcp-error-code.ts — CONTRACT C4: message dạng "<CODE>: <thông điệp>"
export type ParsedMcpError = { code: string; message: string }   // code='UNKNOWN' khi không khớp
export function parseMcpError(err: unknown): ParsedMcpError {
  const raw = err instanceof Error ? err.message : String(err)
  const m = /^([A-Z0-9_]+): (.*)$/s.exec(raw)
  return m ? { code: m[1], message: m[2] } : { code: 'UNKNOWN', message: raw }
}
```

```ts
// mcp-scope-presentation.ts — Why: id scope chứa ':' nên không dùng trực tiếp làm khoá i18n.
export const scopeI18nKey = (id: string): string => id.replace(/[:.]/g, '_')
export function describeScope(s: McpScopeDescriptor) {            // gọi trong render, không ở top-level
  const k = `auto.mcp.scope.${scopeI18nKey(s.id)}`
  return { label: translate(`${k}.label`, s.label), description: translate(`${k}.description`, s.description) }
}
export const isHighRisk = (r: McpRisk): boolean => r === 'exec' || r === 'destructive' || r === 'admin'
```

```ts
// use-mcp-consent.ts
type ConsentState =
  | { kind: 'loading' }
  | { kind: 'ready'; req: McpConsentRequest }
  | { kind: 'submitting'; req: McpConsentRequest }
  | { kind: 'redirecting'; url: string }
  | { kind: 'error'; code: 'MCP_CONSENT_EXPIRED' | 'MCP_CONSENT_NOT_FOUND' | 'MCP_DISABLED' | 'MCP_SCOPE_INVALID' | 'MCP_SCOPE_NOT_ALLOWED' | 'UNKNOWN'; message: string; req?: McpConsentRequest }

export function useMcpConsent(requestId: string | null) {
  // requestId thiếu/không hợp lệ (regex uuid) ⇒ error NOT_FOUND ngay, không gọi backend.
  // useEffect: window.api.mcp.consent.get(requestId) → ready. Huỷ bằng cờ `cancelled` khi unmount.
  // decide(decision, scopes): chỉ chạy ở 'ready'; ngăn bấm đúp (chuyển 'submitting' đồng bộ).
  //   thành công → { redirectUrl } → kiểm new URL(url).protocol ∈ {http:, https:} → setState(redirecting) → window.location.assign(url)
  //   MCP_SCOPE_INVALID / MCP_SCOPE_NOT_ALLOWED → quay lại 'ready' kèm thông báo nội tuyến (cho chọn lại)
}
```

Hành vi UI (bố cục: `Card` căn giữa `max-w-md`, `px-4` cho điện thoại, tiêu đề `Orca` nhỏ phía trên):

- Tiêu đề: `Authorize {clientName}` (dịch: `auto.mcp.consent.title`). Dòng phụ: `{tenant.name}`; `Will redirect to {redirectHost}` (chỉ host, `font-mono`); `clientUri` (nếu có) hiện **host** dạng text, không là link tự động mở.
- Cảnh báo ngữ cảnh: `isNewClient` ⇒ `Badge variant="outline"` "New app"; `registeredViaDcr` ⇒ khối `role="note"` "This app registered itself automatically. Only continue if you started this connection." (token `text-muted-foreground`, viền `border-border`).
- Danh sách scope (`fieldset` + `legend` "This app is asking to:"): mỗi hàng = `Checkbox` + nhãn + mô tả + dấu hiệu rủi ro **bằng icon + chữ** (không chỉ màu): `read` không dấu; `write_reversible` "Can change data"; `exec`/`destructive`/`admin` icon `ShieldAlert` `text-destructive` + `Badge variant="destructive"` "High risk" và mô tả in đậm hơn. Scope đã cấp trước (`alreadyGranted`) tick sẵn + nhãn "Already allowed". **Scope rủi ro cao không bao giờ tick sẵn** (người dùng phải chủ động) — an toàn hơn mặc định "tick hết"; scope `read`/`write_reversible` xin mới thì tick sẵn.
- Nút (STYLEGUIDE dòng 126–128, 296): `Deny` = `variant="ghost"` (**không** destructive: từ chối không làm mất dữ liệu), `Allow access` = `variant="default"` bên phải. `Allow` `disabled` khi không tick scope nào hoặc đang `submitting`; `Deny` luôn khả dụng trừ lúc `submitting`.
- Gọi: `decide('approve', [...selected])` hoặc `decide('deny', [])`. Thành công ⇒ trạng thái `redirecting`: "Returning to {clientName}… You can close this tab once the app confirms." + liên kết dự phòng "Continue" (`<a href={url}>`) nếu trình duyệt chặn điều hướng.
- Focus ban đầu: đặt vào tiêu đề (`tabIndex={-1}`), **không** autofocus "Allow" — đây là màn quyết định bảo mật nên Enter ngẫu nhiên không được cấp quyền (chủ ý lệch khỏi gợi ý "focus hành động chính" ở STYLEGUIDE dòng 255, ghi lý do trong comment).
- Không `console.log`/telemetry `redirectUrl`, `requestId` hay scope; không lưu gì vào slice/localStorage.

**Trạng thái UI:**

| Trạng thái | Hiển thị |
|---|---|
| loading | `Skeleton` cho tiêu đề + 3 hàng scope; nút ẩn |
| ready | như trên |
| submitting | nút disabled, nhãn "Working…"; checkbox disabled |
| `MCP_CONSENT_EXPIRED` | khối `role="alert"`: "This request has expired. Go back to the app and start the connection again." (không nút thử lại — request đã hết hạn) |
| `MCP_CONSENT_NOT_FOUND` | "This request is no longer valid or was already answered." |
| `MCP_DISABLED` / `enabled:false` | "MCP access is turned off for this organization." |
| `MCP_SCOPE_INVALID`/`NOT_ALLOWED` | thông báo nội tuyến trên nút; giữ lựa chọn, cho sửa |
| lỗi mạng/khác | "Something went wrong" + `Retry` (gọi lại `consent.get`) |
| forbidden | không có (mọi user đăng nhập đều xem được request **của chính họ**; request người khác ⇒ NOT_FOUND) |

### Bước 4 — Tab "Connected apps" (mọi người dùng)

**File:** `frontend/src/renderer/src/components/settings/mcp/McpConnectedAppsTab.tsx` (NEW) + `use-mcp-grants.ts` (NEW, state cục bộ — danh sách không cần cache toàn cục).

- Bảng (`components/ui/table`): **App** (tên + host `clientUri`), **Permissions** (Badge theo `describeScope`, high-risk có icon), **Connected** (`createdAt` → `toLocaleString`), **Last used** (`lastUsedAt` hoặc "Never"), hành động. Hàng `status==='revoked'` mờ + `Badge` "Revoked", không có nút. Sắp xếp: active trước, rồi `createdAt` giảm dần.
- "Revoke" = nút `ghost` kích thước `sm` mở `useConfirmationDialog` (`confirmVariant:'destructive'`, vì app mất quyền và phải xin lại): tiêu đề `Revoke access for {clientName}?`, mô tả "The app will be signed out within about a minute. You can reconnect it later." → `window.api.mcp.grant.revoke(id)` → refetch + `toast.success`.
- Trạng thái: loading (3 hàng Skeleton) · empty ("No apps connected yet." + hướng dẫn `McpServerInfo.resourceUrl` trong `font-mono` có nút Copy — dùng `navigator.clipboard.writeText` như `ExecutionMonitor`) · error (`role="alert"` + Retry) · `MCP_DISABLED` (không render, vì FE-001 đã ẩn) · refetch khi tab mount và khi cửa sổ lấy lại focus.

### Bước 5 — Tab admin "OAuth clients" và "All grants"

**Files (NEW):** `McpOAuthClientsTab.tsx`, `McpAllGrantsTab.tsx` cùng thư mục. FE-001 chỉ mount khi `useAppStore((s) => s.currentUser)?.role === 'admin'` (cùng cổng `AdminOrgConsole`); server vẫn là chốt chặn.

- **OAuth clients:** cột Name · Client ID (`font-mono`, rút gọn + Copy) · Redirect hosts (`redirectUris` → host; đủ URL trong `title`) · Registered via (`dcr`/`admin` Badge) · Status (`allowed`/`blocked`/`pending`) · Active grants · Last used · hành động. `pending` xếp trước và có banner `N apps waiting for approval`. Hành động: `allowed` ⇒ "Block" (confirm destructive: "Blocking {name} signs out its {activeGrants} connections. You can allow it again later."); `blocked`/`pending` ⇒ "Allow" (nút `secondary`, không cần confirm). Gọi `setStatus(clientId, 'allowed'|'blocked')`, thay hàng bằng `McpOAuthClient` trả về. Mỗi dòng có trạng thái `busy` riêng.
- **All grants:** props tuỳ chọn `userId` (để deep-link từ tab Users sau này) → `window.api.mcp.admin.grant.list({ userId })`; lọc phía client theo tên người dùng/app; cột User (`userName`) · App · Permissions · Connected · Last used · Revoke (confirm nêu tên user và app) → `admin.grant.revoke`.
- `MCP_NOT_ADMIN` (vai trò đổi giữa chừng) ⇒ khối "Administrator access required." thay bảng — không toast; các lỗi khác như tab người dùng.

### Bước 6 — i18n và catalog

Khoá mới trong `frontend/src/renderer/src/i18n/locales/en.json` dưới `auto.mcp.consent.*`, `auto.mcp.apps.*`, `auto.mcp.clients.*`, `auto.mcp.grants.*`, `auto.mcp.risk.*` (+ `auto.mcp.scope.<id_chuẩn_hoá>.{label,description}` cho 4 scope hiện có, nội dung bằng chuỗi server). Mọi `translate('auto.…', 'English')` gọi trong hàm/component. Các locale `es/ja/ko/zh` rơi về tiếng Anh (`fallbackLng`); chạy `verify:localization-*` ở root để biết có bắt buộc điền (chưa xác minh phạm vi quét của script với `frontend/`).

## A11y & i18n & style

`main` landmark + `h1`; `fieldset/legend` cho nhóm scope; checkbox có `<label htmlFor>` bao nhãn+mô tả (`aria-describedby`); rủi ro không chỉ bằng màu; `role="alert"`/`aria-live="polite"` cho lỗi/redirecting; thao tác bàn phím đủ (Tab → Deny → Allow; Esc không làm gì trên trang đầy đủ); chạm ≥ 44px trên di động, không cuộn ngang ở 320px. Token từ `assets/main.css`, không hex; bóng/elevation theo STYLEGUIDE (Card mặc định).

## Files cần sửa

| File | Loại |
|---|---|
| `frontend/src/renderer/src/web/oauth-return-to.ts` (+ `oauth-return-to.test.ts`) | NEW |
| `frontend/src/renderer/src/web/main-web-bootstrap.tsx` | MODIFY (+~12 dòng) |
| `frontend/src/renderer/src/components/mcp/consent/McpConsentPage.tsx`, `use-mcp-consent.ts` (+ tests) | NEW |
| `frontend/src/renderer/src/lib/mcp-error-code.ts` (+ test) | NEW |
| `frontend/src/renderer/src/components/settings/mcp/{mcp-scope-presentation.ts,McpConnectedAppsTab.tsx,McpOAuthClientsTab.tsx,McpAllGrantsTab.tsx,use-mcp-grants.ts}` (+ tests) | NEW |
| `frontend/src/renderer/src/i18n/locales/en.json` | MODIFY |
| `frontend/src/renderer/src/web/main-web-bootstrap.test.ts` | MODIFY (thêm ca forward/consent) |
| (FE-MCP-SOL-001) `McpSettingsPane` mount 3 tab | MODIFY |

## Verification

```bash
cd /opt/repos/orca/frontend
npx vitest run --config config/vitest.config.ts src/renderer/src/web/oauth-return-to.test.ts \
  src/renderer/src/web/main-web-bootstrap.test.ts src/renderer/src/components/mcp/consent \
  src/renderer/src/components/settings/mcp
npx tsc --noEmit -p tsconfig.json
cd /opt/repos/orca && pnpm lint   # max-lines, localization catalog, no-top-level-translate
```

Ca kiểm thử bắt buộc: `sanitizeReturnTo` (bảng: `//evil.com`, `/\evil`, `javascript:`, `https://…`, `/oauth/authorize?x`, độ dài, ký tự điều khiển); `takeForwardTarget` hết lượt; `WebRoot`: chưa đăng nhập ⇒ `LoginPage` + stash, đã đăng nhập + `?return_to` hợp lệ ⇒ `location.replace` và **không** mount `App`, đường dẫn `/oauth/consent` ⇒ chỉ `McpConsentPage`; consent: bảng trạng thái ở trên, high-risk không tick sẵn, `Allow` disabled khi 0 scope, `Deny` gọi `decide('deny', [])`, bấm đúp chỉ gọi một lần, `redirectUrl` scheme lạ bị từ chối, `assign` được gọi với URL từ server; tab: empty/error/revoke-confirm/`MCP_NOT_ADMIN`. Playwright (`tests/`): chưa xác minh có harness cho web SPA — nếu có, một kịch bản authorize→login→consent→redirect với backend giả.

## Sửa TDD kèm theo

- `specs/frontend/tdd/v5/06-web-client.md` (và `v4/06-web-entry.md`): ghi **không có router**, route đặc biệt duy nhất là `/oauth/consent`, cơ chế `return_to`/`sessionStorage`, và đường dẫn đúng `renderer/src/web/web-preload-api.ts`.
- `specs/frontend/tdd/v4/02-auth-flow.md`: thêm luồng `return_to` (sau local login/SSO) và ràng buộc `SameSite=Strict`.
- `specs/frontend/tdd/v5/05-ui-components.md`: ghi quy ước "Deny/Cancel = ghost" cho màn quyết định bảo mật và quy tắc khoá i18n với id có `:`.

## Không làm ở solution này

Shell Settings > MCP, kiểu chung, runtime client, slice, đăng ký section (FE-MCP-SOL-001); PAT (FE-MCP-SOL-004); hộp thoại phê duyệt tool (FE-MCP-SOL-009); đăng nhập MCP trong Electron (consent luôn mở bằng trình duyệt → SPA web); bất kỳ thay đổi `LoginPage`/`SsoButton`/`App.tsx`/`main.tsx`/`web-preload-api.ts` ngoài namespace do FE-001 thêm.
