# FE-MCP-SOL-004: Tab "Access tokens" (PAT cho headless/CI) — tạo, hiển thị secret một lần, thu hồi

> 🔲 Designed — chưa implement. Cần [BE-MCP-SOL-006](../../../../../backend-go/crs/v5/mcp-authorization/solutions/BE-MCP-SOL-006-mcp-tokens-scopes-and-role.md) (PR 5). Hợp đồng: [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md).

## CR Reference

- **CR:** [CR-MCP-006](../../../../../../docs/crs/v5/mcp-authorization/CR-MCP-006-mcp-tokens-scopes-and-role.md) — mục A (PAT) và "Ngoài phạm vi: UI quản lý PAT (frontend)".
- **Mức độ:** 🔴 P0 (đường token cho agent không có trình duyệt).
- **Phạm vi:** chỉ UI tab `tokens` trong Settings > MCP. Phần scope/role/audience hoàn toàn ở backend, FE chỉ hiển thị và ánh xạ lỗi.

## Backend dependency

| Kênh (CONTRACT) | Tham số → Kết quả | BE | Khi BE chưa có |
|---|---|---|---|
| `mcp.token.list` | `—` → `McpToken[]` | BE-MCP-SOL-006 | tab hiện lỗi + Retry; không tạo được |
| `mcp.token.create` | `{name, scopes: McpScopeId[], expiresInDays}` → `{token: McpToken, secret: string}` (**secret chỉ trả lần này**, C6) | BE-MCP-SOL-006 | nút Create vẫn mở dialog, lỗi hiện nội tuyến |
| `mcp.token.revoke` | `tokenId` → `{ok:true}` | BE-MCP-SOL-006 | lỗi hiện toast |
| `mcp.server.info` | → `McpServerInfo` (`resourceUrl`, `maxTokenDays`, `scopesSupported`, `enabled`, `killSwitch`) | BE-MCP-SOL-003 qua **FE-MCP-SOL-001** | `enabled:false` ⇒ tab không mount |

Lỗi: `MCP_TOKEN_TOO_LONG`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_SCOPE_INVALID`, `MCP_TOKEN_LIMIT`, `MCP_KILL_SWITCH_ACTIVE`, `MCP_DISABLED`, `MCP_NOT_FOUND` (CONTRACT §2.3, tách theo `^([A-Z0-9_]+): `).

Tái dùng từ **FE-MCP-SOL-001** (không định nghĩa lại): `frontend/src/shared/mcp-types.ts` (`McpToken`, `McpScopeId`, `McpScopeDescriptor`, `McpServerInfo`), `window.api.mcp.*` (quy ước kênh→namespace: `window.api.mcp.token.list/create/revoke`, `window.api.mcp.server.info`), slice `store/slices/mcp-slice.ts` (chỉ **đọc** `McpServerInfo` đã nạp — tên selector do FE-001 chốt, ở đây gọi tạm `useAppStore((s) => s.mcpServerInfo)`), section Settings id `mcp`. Tái dùng từ **FE-MCP-SOL-003**: `components/settings/mcp/mcp-scope-presentation.ts` (`describeScope`, `isHighRisk`) và `lib/mcp-error-code.ts` (`parseMcpError`).

## Impact analysis (gitnexus) — chưa chạy

Solution này chỉ **thêm file mới** và một điểm mount trong `McpSettingsPane` của FE-001 (file thuộc FE-001). Không sửa symbol hiện có.

| Symbol | Direction | Risk dự kiến | Lệnh |
|---|---|---|---|
| `McpSettingsPane` (FE-001) | upstream | LOW (thêm 1 `TabsContent`) | `gitnexus impact McpSettingsPane --direction upstream` (symbol chưa tồn tại tới khi FE-001 merge) |
| `useConfirmationDialog` | upstream | LOW (chỉ tiêu thụ) | — |

## Bối cảnh (đã xác nhận lại)

- `components/ui/dialog.tsx`: `DialogContent` có prop `showCloseButton` (mặc định `true`) ⇒ có thể ẩn nút X ở bước hiển thị secret. Commit gần đây `da67f0b33` sửa lỗi thiếu `DialogDescription` ở dialog AI Provider ⇒ **mọi `DialogContent` ở đây phải có `DialogDescription`**.
- Không có `alert-dialog`, `switch`, `alert`; có `checkbox`, `select`, `input`, `table`, `badge`, `toggle-group`, `skeleton`, `sonner`. `components/confirmation-dialog.tsx` (`useConfirmationDialog`, `confirmVariant:'destructive'`) dùng được trong Settings vì `App` bọc `ConfirmationDialogProvider`.
- Gọi RPC: `callRuntimeResult` throw `Error(response.error.message)`; trong `web-preload-api.ts`/`platform/adapters/web/rpc-client.ts` đã grep `console.*` — không thấy in payload kết quả (chưa kiểm hết interceptor/trace khác ⇒ có test chứng minh ở phần Verification). Trace store (`addTraceEvent`) chỉ nhận sự kiện từ `initBrowserTrace`; **chưa xác minh** nó không bắt payload RPC — secret vì thế luôn chỉ sống trong state cục bộ của dialog và test kiểm tra điều này.
- `AuthUser.role` ở FE là `'developer' | 'lead' | 'admin'` (backend ánh xạ `user`→`developer`); chỉ `'admin'` được xin `orca:admin` ⇒ UI vô hiệu hoá scope đó cho vai trò khác (server vẫn là chốt chặn: `MCP_SCOPE_NOT_ALLOWED`).
- Chuẩn style: `guides/STYLEGUIDE.md` — form nhiều trường ở `Dialog` (dòng 146–148, 260), `destructive` chỉ cho hành động không hoàn tác (dòng 296), Cancel luôn ghost. `translate()` không ở top-level. Budget lint: `.tsx` ≤ 400 dòng, `.ts` ≤ 300 ⇒ tách file như dưới.
- SSH/Web: URL kết nối hiển thị là `McpServerInfo.resourceUrl` do server trả về — **không** suy từ `window.location`, vì agent chạy trên máy/host khác (SSH, CI) với trình duyệt.

## Giải pháp

### Bước 1 — Dữ liệu cục bộ, không đi qua slice

**File:** `frontend/src/renderer/src/components/settings/mcp/use-mcp-tokens.ts` (NEW)

```ts
export function useMcpTokens() {
  // state: { status: 'loading'|'ready'|'error', tokens: McpToken[], error?: ParsedMcpError }
  // load(): window.api.mcp.token.list(); refetch khi mount và khi cửa sổ lấy lại focus
  // addCreated(token: McpToken): chèn đầu danh sách — CHỈ nhận metadata, hàm không có tham số secret
  // revoke(id): window.api.mcp.token.revoke(id) → cập nhật status 'revoked' tại chỗ (không chờ refetch)
}
```

Quyết định: **danh sách PAT và secret không nằm trong `mcp-slice.ts`** — chỉ `McpServerInfo` (cần cho `maxTokenDays`) đọc từ slice. Lý do: slice được persist/subscribe rộng hơn dự kiến và `McpToken` không cần chia sẻ giữa component; cấu trúc hook cục bộ cũng làm cho "secret không thể lọt vào store" là bất biến theo kiểu (không có đường nào truyền secret vào hook).

### Bước 2 — Form tạo token (thuần, dễ test)

**File:** `frontend/src/renderer/src/components/settings/mcp/mcp-token-form.ts` (NEW, thuần TS)

```ts
export const EXPIRY_PRESET_DAYS = [7, 30, 60, 90] as const
export function expiryOptions(maxTokenDays: number): number[]   // preset ≤ maxTokenDays; luôn gồm maxTokenDays nếu < 90
export function defaultExpiryDays(maxTokenDays: number): number // min(30, maxTokenDays)
export type TokenFormInput = { name: string; scopes: McpScopeId[]; expiresInDays: number }
export type TokenFormErrors = { name?: string; scopes?: string; expiresInDays?: string }
export function validateTokenForm(i: TokenFormInput, maxTokenDays: number): TokenFormErrors
//   name: trim, 1..80 ký tự; scopes: ≥ 1; expiresInDays: số nguyên 1..maxTokenDays
export function isScopeSelectable(id: string, role: 'developer'|'lead'|'admin'|undefined): boolean // 'orca:admin' chỉ cho admin
```

`maxTokenDays` chưa nạp (undefined/≤ 0) ⇒ nút Create **disabled** (không đoán giá trị).

### Bước 3 — Dialog tạo + hiển thị secret một lần

**Files:** `McpCreateTokenDialog.tsx` (NEW, điều phối 2 bước), `McpTokenSecretReveal.tsx` (NEW), `McpTokenCliSnippet.tsx` (NEW) — cùng thư mục `components/settings/mcp/`.

State máy của dialog: `form → creating → reveal(secret) → closed`. **Secret chỉ tồn tại trong `useState` của `McpCreateTokenDialog` ở bước `reveal`**; nó được truyền xuống `McpTokenSecretReveal` qua prop và bị `setSecret(null)` khi "Done" hoặc unmount.

```tsx
// McpCreateTokenDialog.tsx (rút gọn)
async function submit(): Promise<void> {
  if (creating) return                                   // chặn bấm đúp ⇒ không sinh hai PAT
  const errs = validateTokenForm(input, maxTokenDays); if (hasErrors(errs)) return setErrors(errs)
  setCreating(true); setBanner(null)
  try {
    const { token, secret } = await window.api.mcp.token.create({ name: input.name.trim(), scopes: input.scopes, expiresInDays: input.expiresInDays })
    onCreated(token)                                     // metadata → useMcpTokens().addCreated
    setSecret(secret)                                    // duy nhất nơi giữ secret
    setStep('reveal')
  } catch (e) {
    applyCreateError(parseMcpError(e), { setErrors, setBanner, maxTokenDays })
  } finally { setCreating(false) }
}
```

**Bước form** (`Dialog` kích thước trung bình; `DialogTitle` "Create access token"; `DialogDescription` "Tokens let scripts and CI agents call Orca's MCP server without a browser."):

- `Input` **Name** (bắt buộc, `maxLength=80`, placeholder "CI pipeline").
- **Permissions**: `fieldset` các `Checkbox` từ `McpServerInfo.scopesSupported` (nhãn/mô tả qua `describeScope`; `isHighRisk` ⇒ icon + chữ "High risk"). Mặc định chỉ tick `orca:read` (đặc quyền tối thiểu). `orca:admin` `disabled` khi role ≠ admin, kèm mô tả "Only administrators can grant this."
- **Expires in**: `Select` các `expiryOptions(maxTokenDays)` + dòng trợ giúp "Your organization allows up to {maxTokenDays} days." (bắt buộc có hạn — không có "never").
- Nút: `Cancel` = `variant="ghost"`; `Create token` = default, disabled khi đang `creating` hoặc `maxTokenDays` chưa nạp hoặc `killSwitch.active`.

**Bước reveal** (`showCloseButton={false}`; `onEscapeKeyDown`/`onInteractOutside` gọi `preventDefault()` cho tới khi người dùng tick "I have saved it"):

- `DialogTitle` "Copy your token now"; `DialogDescription` "This is the only time Orca shows this token. Store it in a secret manager — it can't be recovered, only revoked."
- Khối `code` `font-mono break-all` hiển thị secret (che mặc định dạng `omp_••••••`, nút `Show`/`Hide`) + nút **Copy** (`navigator.clipboard.writeText`, nhãn đổi "Copied" 2 giây; nếu clipboard bị từ chối ⇒ tự hiện secret và gợi ý chọn-copy tay). Secret không bao giờ nằm trong `title`, `aria-label`, toast, hay URL.
- `Checkbox` "I have saved this token" → mở khoá nút **Done** (`default`, disabled cho tới khi tick). "Done" ⇒ `setSecret(null)`, đóng dialog.
- Bên dưới: `McpTokenCliSnippet` (xem dưới) — snippet **không chứa secret**.
- Cảnh báo ngắn: tên + hạn + scopes của token vừa tạo (từ `token`) để người dùng xác nhận.

**Ánh xạ lỗi (`applyCreateError`):**

| Mã | Hiển thị |
|---|---|
| `MCP_TOKEN_TOO_LONG` | lỗi dưới trường Expires: "Maximum lifetime is {maxTokenDays} days." và gọi nạp lại `McpServerInfo` (action của FE-001) phòng trần đã đổi |
| `MCP_SCOPE_NOT_ALLOWED` | lỗi dưới nhóm Permissions: "Your role can't grant one or more of these permissions." (giữ lựa chọn để người dùng bỏ tick) |
| `MCP_SCOPE_INVALID` | "Select at least one valid permission." |
| `MCP_TOKEN_LIMIT` | banner `role="alert"` trong dialog: "You've reached the maximum number of active tokens. Revoke one, then try again." |
| `MCP_KILL_SWITCH_ACTIVE` / `MCP_DISABLED` | banner: "MCP access is currently disabled for this organization." + vô hiệu nút Create |
| `UNKNOWN` | banner với thông điệp đã bỏ tiền tố mã + nút "Try again" |

### Bước 4 — Tab và bảng

**File:** `McpAccessTokensTab.tsx` (NEW, ~180 dòng)

- Đầu tab: mô tả + `McpTokenCliSnippet` thu gọn (URL `resourceUrl`) + nút **Create token** (`default`).
- Bảng: **Name** · **Permissions** (Badge theo `describeScope`, high-risk có icon) · **Created** · **Expires** (ngày tuyệt đối; còn < 7 ngày ⇒ `Badge variant="outline"` "Expires soon") · **Last used** (`lastUsedAt` hoặc "Never used") · **Status** (`active`/`revoked`/`expired` Badge; hai trạng thái sau làm hàng mờ) · hành động.
- **Revoke** (chỉ hàng `active`): nút `ghost` `sm` → `useConfirmationDialog({ title: 'Revoke "{name}"?', description: 'Anything using this token stops working within about a minute. This can\'t be undone.', confirmLabel: 'Revoke token', confirmVariant: 'destructive' })` → `revoke(id)`; thành công `toast.success("Token revoked")`; `MCP_NOT_FOUND` ⇒ làm mới danh sách.
- Trạng thái: **loading** (3 hàng `Skeleton`) · **empty** ("No access tokens yet." + nút Create) · **error** (`role="alert"` + Retry) · **disabled-by-flag** (`enabled:false` hoặc `killSwitch.active` ⇒ banner và nút Create disabled; danh sách vẫn xem/thu hồi được khi kill switch bật) · **forbidden**: không có (kênh dành cho mọi user đăng nhập; `Role` không ảnh hưởng, chỉ giới hạn scope) · không phân trang (CONTRACT trả mảng đầy đủ).

### Bước 5 — Snippet dùng CLI (không chứa secret)

**File:** `McpTokenCliSnippet.tsx` (NEW). `ToggleGroup` chọn shell, mặc định theo `navigator.userAgent.includes('Windows')` (đa nền tảng — AGENTS.md); `resourceUrl` lấy từ `McpServerInfo`.

```bash
# macOS / Linux
export ORCA_MCP_TOKEN='<paste token here>'
claude mcp add --transport http orca <resourceUrl> --header "Authorization: Bearer $ORCA_MCP_TOKEN"
```
```powershell
# Windows (PowerShell)
$env:ORCA_MCP_TOKEN = '<paste token here>'
claude mcp add --transport http orca <resourceUrl> --header "Authorization: Bearer $env:ORCA_MCP_TOKEN"
```

Mỗi khối có nút Copy (không chứa secret thật). Dòng chú thích: "Any MCP client that supports HTTP transport and a custom Authorization header works the same way." Cú pháp `claude mcp add --transport http` lấy từ tiêu chí chấp nhận của CR-005; các cờ khác của client **chưa xác minh** — nếu client đổi cú pháp, chỉ sửa chuỗi trong file này.

### Bước 6 — i18n

Khoá `auto.mcp.tokens.*` (tiêu đề, cột, nhãn form, lỗi theo bảng trên, nút, snippet caption) trong `i18n/locales/en.json`; gọi `translate('auto.mcp.tokens.…', 'English')` trong hàm/component (không top-level). Số ngày dùng `translate(..., { count })` hoặc nội suy `{{days}}` theo mẫu hiện có.

## Trạng thái UI (tổng hợp)

| Vùng | loading | empty | error | forbidden | disabled |
|---|---|---|---|---|---|
| Danh sách | Skeleton | CTA tạo | alert + Retry | — | banner (flag/kill switch) |
| Dialog form | — | — | lỗi theo trường + banner | — | Create disabled khi chưa có `maxTokenDays`/kill switch |
| Reveal | — | — | clipboard bị chặn ⇒ hiện secret để chép tay | — | Done disabled tới khi tick |

## A11y & i18n & style

`DialogTitle` + `DialogDescription` ở cả 2 bước (tránh lỗi Radix đã gặp); focus vào ô Name khi mở; khi sang `reveal` focus vào tiêu đề (không vào Copy/Done); `Copied`/lỗi qua `aria-live="polite"`; bảng có `<caption class="sr-only">`; rủi ro không chỉ bằng màu; không hard-code màu (token `assets/main.css`); `Cancel` ghost, `destructive` chỉ cho "Revoke token".

## Files cần sửa

| File | Loại |
|---|---|
| `frontend/src/renderer/src/components/settings/mcp/McpAccessTokensTab.tsx` | NEW |
| `…/mcp/McpCreateTokenDialog.tsx`, `McpTokenSecretReveal.tsx`, `McpTokenCliSnippet.tsx` | NEW |
| `…/mcp/use-mcp-tokens.ts`, `mcp-token-form.ts` | NEW |
| tests cạnh từng file (`*.test.ts(x)`) | NEW |
| `frontend/src/renderer/src/i18n/locales/en.json` | MODIFY |
| (FE-MCP-SOL-001) `McpSettingsPane` mount tab id `tokens` | MODIFY |
| (FE-MCP-SOL-003) `lib/mcp-error-code.ts`, `mcp-scope-presentation.ts` | dùng lại |

## Verification

```bash
cd /opt/repos/orca/frontend
npx vitest run --config config/vitest.config.ts src/renderer/src/components/settings/mcp
npx tsc --noEmit -p tsconfig.json
cd /opt/repos/orca && pnpm lint
```

Ca bắt buộc: `validateTokenForm`/`expiryOptions` (bảng: `maxTokenDays` 1, 30, 90; 0 và 91 ngày; tên rỗng/81 ký tự; 0 scope; role ≠ admin chọn `orca:admin`); tạo thành công ⇒ dialog sang `reveal`, `addCreated` nhận đối tượng **không có** `secret`; **bất biến bí mật**: (a) spy `Storage.prototype.setItem` cho `localStorage`/`sessionStorage` ⇒ không lần gọi nào chứa secret, (b) spy `console.*` ⇒ không chứa secret, (c) `useAppStore.getState()` serialize ⇒ không chứa secret, (d) sau "Done"/unmount, DOM không còn secret; Esc và click ngoài không đóng khi chưa tick; Done disabled tới khi tick; bấm Create hai lần chỉ gọi `create` một lần; từng mã lỗi trong bảng ánh xạ ra đúng vị trí; revoke xác nhận ⇒ gọi `revoke` đúng id; kill switch ⇒ Create disabled nhưng Revoke còn dùng được.

## Sửa TDD kèm theo

- `specs/frontend/tdd/v5/02-state-management.md` (đã lỗi thời về số slice): ghi rõ danh sách PAT là state cục bộ và secret không bao giờ vào store.
- `specs/frontend/tdd/v5/05-ui-components.md`: thêm mẫu "hiển thị secret một lần" (Dialog không đóng được cho tới khi xác nhận đã lưu).

## Không làm ở solution này

Slice/runtime client/section `mcp` (FE-MCP-SOL-001); trang consent, Connected apps, quản trị client (FE-MCP-SOL-003); liệt kê PAT của người khác (CONTRACT không có kênh admin cho PAT); chỉnh `maxTokenDays` của tenant (FE-MCP-SOL-008); lưu secret vào keychain/clipboard manager; sửa `web-preload-api.ts` ngoài namespace do FE-001 thêm.
