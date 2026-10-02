# FE-MCP-SOL-001: Nền tảng frontend cho MCP — kiểu dùng chung, `window.api.mcp`, runtime client, slice, section Settings "MCP"

> 🔲 Designed — chưa implement. **Đây là nền mà FE-MCP-SOL-002..012 xây lên**; tên file & symbol ở bảng "Bề mặt xuất" bên dưới là cam kết.

## CR Reference

- **CR:** [CR-MCP-001](../../../../../../docs/crs/v5/mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md), [CR-MCP-002](../../../../../../docs/crs/v5/mcp-service-foundation/CR-MCP-002-gateway-mcp-endpoint-wiring.md) — **chỉ phần FE**: CR-001/002 là hạ tầng backend; phần FE duy nhất là *điều kiện hiển thị* (`MCP_ENABLED=false` ⇒ ẩn toàn bộ UI, CONTRACT C8/§6) và nền tảng gọi kênh `mcp.*`.
- **Mức độ:** 🔴 P0 (chặn mọi FE-MCP-SOL còn lại).
- **TDD tham chiếu:** [`tdd/v5/00-index.md`](../../../../tdd/v5/00-index.md) (nguyên tắc 11/12/19: không sửa `App.tsx`/`main.tsx`/`web-preload-api.ts` ngoài thêm kênh; mọi `on*()` có cleanup), [`tdd/v4/03-admin-spa.md`](../../../../tdd/v4/03-admin-spa.md); `tdd/v5/02` & `05` **lỗi thời** so với mã (README).

## Backend dependency

| Kênh (CONTRACT) | BE solution | FE dùng ở solution này | Khi BE chưa có |
|---|---|---|---|
| `mcp.server.info` | [BE-MCP-SOL-003](../../../../../backend-go/crs/v5/mcp-protocol-server/solutions/BE-MCP-SOL-003-streamable-http-and-lifecycle.md) (+ khung BE-002) | quyết định hiển thị UI | gateway trả `channel "mcp.server.info" is not yet implemented…` ⇒ coi như `enabled:false`, **không** toast |
| `mcp.events.subscribe` (stream) | BE-MCP-SOL-013 | `startMcpEvents()` | lỗi "not implemented" ⇒ ghi log debug, không retry dồn dập (backoff tối đa 5 phút), UI vẫn chạy bằng polling |
| mọi kênh `mcp.*` khác | xem CONTRACT §2 | chỉ định nghĩa kiểu + `call()` generic | — |

Quy ước thật của đường truyền (đã đọc `backend-go/.../wscompat/session_dialect.go`): web client dùng dialect *session-client* — `params` được chuyển thành **đúng một** `args[0]` (object). ⇒ mọi `mcp.*` nhận **một object**; kênh stream ack bằng `null` rồi đẩy mỗi `McpEvent` thành một frame `Streaming:true` có `result` = chính `McpEvent` (không bọc). Lỗi: `{code:"internal", message}` với `message` = `"<MCP_CODE>: …"` (BE-002 đã cắt tiền tố `rpc error`).

## Impact analysis (gitnexus) — **chưa chạy**, lệnh cần chạy trước khi sửa

| Symbol | Lệnh | Rủi ro dự kiến |
|---|---|---|
| `createWebPreloadApi` (`web/web-preload-api.ts:492`) | `impact({target:"createWebPreloadApi", direction:"upstream"})` | MEDIUM — chỉ **thêm 1 property** `mcp` |
| `useIpcEvents` (`hooks/useIpcEvents.ts`) | `impact({target:"useIpcEvents", direction:"upstream"})` | **HIGH** (gọi bởi `App`; nhiều flow) — chỉ thêm 1 dòng `useMcpSync()` cạnh `useDevServersSync()`; báo người duyệt |
| `buildSettingsNavigationMetadata` | `impact({target:"buildSettingsNavigationMetadata", direction:"upstream"})` | MEDIUM (Settings sidebar + Cmd+J + test) — thêm tham số tuỳ chọn `isMcpEnabled` mặc định `false` |
| `SettingsNavTarget` (`lib/settings-navigation-types.ts`) | `impact({target:"SettingsNavTarget", direction:"upstream"})` | MEDIUM — thêm `'mcp'` vào union; chạy `lint:switch-exhaustiveness` vì có thể có `switch` đầy đủ |
| `AppState` (`store/types.ts`) | `impact({target:"AppState", direction:"upstream"})` | HIGH (toàn bộ renderer) — thêm một `& McpSlice` additive |
| `Settings` component | `impact({target:"Settings", direction:"upstream"})` | MEDIUM — thêm một `<SettingsSection>` có điều kiện |

## Bối cảnh (đã xác nhận lại)

- **Không có** section Settings toàn cục cho MCP; chỉ có `components/settings/McpConfigSection.tsx` (cấu hình `.mcp.json` theo repo, gắn ở `RepositoryPane`) — **không đụng** (khác bản chất).
- Thêm section Settings = 3 chỗ, đã đọc: (1) `hooks/useSettingsNavigationMetadata.ts` — mảng `buildSettingsNavigationMetadata` (mục admin dùng `...(isAdmin ? [...] : [])`, `group` thuộc `SETTINGS_NAV_GROUPS` trong `Settings.tsx:115`); (2) `<SettingsSection id=… searchEntries={getSectionSearchEntries(id)}>` trong `components/settings/Settings.tsx` với `isSectionMounted(id)` (lazy-mount); (3) file `*-search.ts` dùng `createLocalizedCatalog` + `translate` + `translateSearchKeyword` (mẫu `privacy-search.ts`). Không có danh sách section cứng nào khác (`settings-load-performance.ts` không liệt kê `admin-org`).
- Tab Admin dùng `Tabs/TabsList/TabsTrigger/TabsContent` của `components/ui/tabs` (`AdminOrgConsole.tsx`); gate `currentUser?.role === 'admin'` (`useSettingsNavigationMetadata.ts:609`).
- Gọi backend: `callRuntimeResult(method, params)` là hàm **private** trong `web-preload-api.ts:3498` (throw `Error(response.error.message)`); stream đăng ký bằng `getClientForEnvironment(env).subscribe(method, params, {onResponse})` (mẫu `createNativeChatApi`, `web-preload-api.ts:1170`). Với web session client, `unsubscribe()` **chỉ xoá callback cục bộ** (`web-session-client.ts:69`) — server vẫn giữ goroutine đến khi đóng socket ⇒ **chỉ đăng ký MỘT lần/app, không đăng ký lại theo mount**.
- `subscribeRuntimeStreamChannel` (runtime-rpc-client) từ chối `target.kind==='local'`, mà web luôn là `local` khi chưa chọn môi trường ⇒ không dùng được; do đó cần `window.api.mcp.subscribeEvents`.
- Frontend hiện chỉ build web (`vite.config.ts` đặt cứng `ORCA_PLATFORM='web'`); preload Electron nằm ở package `desktop/` (ngoài phạm vi, **chưa xác minh** tình trạng). `window.api.mcp` vắng ⇒ coi như không hỗ trợ ⇒ ẩn UI.
- Slice mẫu: `StateCreator<AppState, [], [], XSlice>` đăng ký ở `store/index.ts` (import + spread cuối `create`) và `store/types.ts` (import type + `& XSlice` cuối `AppState`); test cạnh slice với `create<XSlice>()` (mẫu `connectivity-status.test.ts`) hoặc `store-test-helpers.createTestStore`.
- Global sync hook mẫu: `useDevServersSync()` gọi trong `useIpcEvents()` (`hooks/useIpcEvents.ts:866`).
- i18n: `translate('auto.<path>.<id>', 'English')`, không gọi ở top-level (`i18n/no-top-level-translate.test.ts`); khoá thiếu trong `en.json` vẫn render fallback (đã kiểm: `adminOrgTitle` không có trong `en.json`). Script `verify:localization-*` được `package.json` gọi nhưng **không tồn tại** trong `config/scripts/` (**chưa xác minh** — có thể nằm ở nơi khác).

## Giải pháp

### Bước 1 — Kiểu dùng chung

**File:** `frontend/src/shared/mcp-types.ts` (NEW) — sao nguyên văn CONTRACT §1 và thêm sơ đồ kênh. Không thêm field ngoài CONTRACT (C9: chỉ additive).

```ts
// CONTRACT-mcp-ui-api.md §1 — nguồn sự thật; đổi ở đó trước.
export type McpRisk = 'read' | 'write_reversible' | 'exec' | 'destructive' | 'admin'
export type McpScopeId = 'orca:read' | 'orca:write' | 'orca:exec' | 'orca:admin'
export type McpDecision = 'allow' | 'require_approval' | 'deny'
export interface McpScopeDescriptor { id: McpScopeId | string; label: string; description: string; risk: McpRisk }
export interface McpServerInfo { /* đúng §1: enabled, tenantEnabled?, resourceUrl, protocolVersions, authorizationServer,
  scopesSupported, dcrEnabled, maxTokenDays, killSwitch:{active,reason?,at?} */ }
export interface McpGrant { /* §1 */ } export interface McpSessionView { /* §1 */ }
export interface McpConsentRequest { /* §1 */ } export interface McpToken { /* §1 */ }
export interface McpApproval { /* §1 */ } export interface McpToolView { /* §1 */ }
export interface McpToolPolicy { /* §1 */ } export interface McpAuditEntry { /* §1 */ }
export interface McpOAuthClient { /* §1 */ } export interface McpPrompt { /* §1 */ }
export interface McpExternalServer { /* §1 */ }
export type McpEvent = /* §1: approval.requested | approval.resolved | grant.revoked | session.closed | killswitch.changed */

export const MCP_ERROR_CODES = [
  'MCP_DISABLED', 'MCP_NOT_ADMIN', 'MCP_KILL_SWITCH_ACTIVE', 'MCP_CONSENT_NOT_FOUND', 'MCP_CONSENT_EXPIRED',
  'MCP_SCOPE_INVALID', 'MCP_SCOPE_NOT_ALLOWED', 'MCP_TOKEN_TOO_LONG', 'MCP_TOKEN_LIMIT', 'MCP_APPROVAL_EXPIRED',
  'MCP_APPROVAL_ALREADY_DECIDED', 'MCP_APPROVAL_HASH_MISMATCH', 'MCP_POLICY_HARD_DENY', 'MCP_POLICY_VERSION_CONFLICT',
  'MCP_SERVER_SSRF_BLOCKED', 'MCP_SERVER_NOT_APPROVED', 'MCP_NOT_FOUND'
] as const                                            // 17 mã, §2.3
export type McpErrorCode = (typeof MCP_ERROR_CODES)[number]

type McpOk = { ok: true }
export type McpAdminSettings = { enabled: boolean; dcrEnabled: boolean; maxTokenDays: number; approvalTtlSeconds: number;
  killSwitch: McpServerInfo['killSwitch'] }
// Mọi kênh nhận MỘT object (hoặc không tham số). Các field gộp tham số vị trí của CONTRACT §2 vào object.
export interface McpRpcSchema {
  'mcp.server.info': { params: void; result: McpServerInfo }
  'mcp.session.list': { params: void; result: McpSessionView[] }
  'mcp.session.close': { params: { sessionId: string }; result: McpOk }
  'mcp.consent.get': { params: { requestId: string }; result: McpConsentRequest }
  'mcp.consent.decide': { params: { requestId: string; decision: 'approve' | 'deny'; scopes: McpScopeId[] }; result: { redirectUrl: string } }
  'mcp.grant.list': { params: void; result: McpGrant[] }
  'mcp.grant.revoke': { params: { grantId: string }; result: McpOk }
  'mcp.token.list': { params: void; result: McpToken[] }
  'mcp.token.create': { params: { name: string; scopes: McpScopeId[]; expiresInDays: number }; result: { token: McpToken; secret: string } }
  'mcp.token.revoke': { params: { tokenId: string }; result: McpOk }
  'mcp.approval.list': { params: { status?: 'pending' | 'all'; cursor?: string; limit?: number }; result: { approvals: McpApproval[]; nextCursor?: string } }
  'mcp.approval.decide': { params: { approvalId: string; decision: 'approve' | 'deny'; paramsHash: string; note?: string }; result: McpApproval }
  'mcp.admin.settings.get': { params: void; result: McpAdminSettings }
  'mcp.admin.settings.set': { params: Partial<Omit<McpAdminSettings, 'killSwitch'>>; result: McpAdminSettings }
  'mcp.admin.killswitch.set': { params: { scope: 'tenant' | 'client' | 'grant' | 'session'; targetId?: string; active: boolean; reason: string }; result: McpOk }
  'mcp.admin.tool.list': { params: { namespace?: string; risk?: McpRisk }; result: McpToolView[] }
  'mcp.admin.policy.list': { params: void; result: McpToolPolicy[] }
  'mcp.admin.policy.upsert': { params: Omit<McpToolPolicy, 'id' | 'version' | 'updatedAt' | 'updatedBy'> & Partial<Pick<McpToolPolicy, 'id' | 'version'>>; result: McpToolPolicy }
  'mcp.admin.policy.delete': { params: { policyId: string }; result: McpOk }
  'mcp.admin.policy.explain': { params: { tool: string; userId?: string; clientId?: string }; result: { decision: McpDecision; reasons: string[] } }
  'mcp.admin.client.list': { params: void; result: McpOAuthClient[] }
  'mcp.admin.client.setStatus': { params: { clientId: string; status: 'allowed' | 'blocked' }; result: McpOAuthClient }
  'mcp.admin.grant.list': { params: { userId?: string }; result: McpGrant[] }
  'mcp.admin.grant.revoke': { params: { grantId: string }; result: McpOk }
  'mcp.admin.session.list': { params: void; result: McpSessionView[] }
  'mcp.admin.audit.query': { params: { from?: string; to?: string; userId?: string; tool?: string; decision?: McpAuditEntry['decision']; cursor?: string; limit?: number }; result: { entries: McpAuditEntry[]; nextCursor?: string } }
  'mcp.admin.prompt.list': { params: void; result: McpPrompt[] }
  'mcp.admin.prompt.upsert': { params: McpPrompt; result: McpPrompt }
  'mcp.admin.prompt.delete': { params: { promptId: string }; result: McpOk }
  'mcp.externalServer.list': { params: { scope?: McpExternalServer['scope'] }; result: McpExternalServer[] }
  'mcp.externalServer.upsert': { params: Partial<McpExternalServer>; result: McpExternalServer }
  'mcp.externalServer.setSecret': { params: { serverId: string; kind: 'env' | 'header'; name: string; value: string }; result: { hasSecret: true } }
  'mcp.externalServer.probe': { params: { serverId: string }; result: { tools: Array<{ name: string; description: string }>; digest: string } }
  'mcp.externalServer.review': { params: { serverId: string; decision: 'approve' | 'reject'; toolsDigest: string }; result: McpExternalServer }
  'mcp.externalServer.delete': { params: { serverId: string }; result: McpOk }
}
export type McpRpcMethod = keyof McpRpcSchema
// Danh sách runtime của đúng các khoá trên (kiểu bị xoá khi build) — test hợp đồng FE-MCP-SOL-012 đối chiếu với CONTRACT §2.
export const MCP_RPC_METHODS = [/* 35 tên kênh của McpRpcSchema, cùng thứ tự CONTRACT §2 */] as const satisfies readonly McpRpcMethod[]
export const MCP_STREAM_METHODS = ['mcp.events.subscribe'] as const
export type McpRpcParams<M extends McpRpcMethod> = McpRpcSchema[M]['params']
export type McpRpcResult<M extends McpRpcMethod> = McpRpcSchema[M]['result']
```

(Các `/* §1 */` được thay bằng định nghĩa đầy đủ khi cài; test `mcp-types.test.ts` khẳng định `MCP_ERROR_CODES.length === 17` và bằng danh sách CONTRACT.)

### Bước 2 — Lỗi `MCP_*`

**File:** `frontend/src/renderer/src/runtime/runtime-mcp-error.ts` (NEW)

```ts
import { MCP_ERROR_CODES, type McpErrorCode } from '../../../shared/mcp-types'
export class McpRpcError extends Error {
  constructor(readonly code: McpErrorCode | null, readonly detail: string, readonly cause?: unknown) { super(detail) ; this.name = 'McpRpcError' }
}
// Chịu cả hai dạng: "MCP_X: msg" (CONTRACT C4) và "rpc error: code = … desc = MCP_X: msg" (nếu gateway chưa cắt).
const PATTERN = /(?:^|desc = )(MCP_[A-Z0-9_]+): ([\s\S]*)$/
export function parseMcpError(error: unknown): McpRpcError {
  const message = error instanceof Error ? error.message : String(error)
  const m = PATTERN.exec(message)
  const code = m && (MCP_ERROR_CODES as readonly string[]).includes(m[1]) ? (m[1] as McpErrorCode) : null
  return new McpRpcError(code, m ? m[2] : message, error)
}
export const isMcpDisabledError = (e: unknown): boolean => e instanceof McpRpcError && e.code === 'MCP_DISABLED'
export const isMcpNotImplementedError = (e: unknown): boolean => /is not yet implemented/.test((e as Error)?.message ?? '')
```

`code === null` ⇒ lỗi chung (UI hiển thị `detail` đã rút gọn, không hiển thị chuỗi `rpc error`).

### Bước 3 — `window.api.mcp` (typing + web shim)

**File:** `frontend/src/preload/api-types.ts` (MODIFY) — thêm vào `PreloadApi` (cạnh `admin: {…}` dòng ~2425) và export kiểu:

```ts
import type { McpEvent, McpRpcMethod, McpRpcParams, McpRpcResult } from '../shared/mcp-types'
export type McpBridgeApi = {
  /** Một object params (hoặc không tham số); lỗi = Error("<MCP_CODE>: …") — dùng runtime-mcp-client để parse. */
  call: <M extends McpRpcMethod>(method: M, ...params: McpRpcParams<M> extends void ? [] : [McpRpcParams<M>]) => Promise<McpRpcResult<M>>
  /** Mở luồng `mcp.events.subscribe` MỘT lần; trả hàm teardown. onClose gọi khi socket đóng. */
  subscribeEvents: (onEvent: (event: McpEvent) => void, onClose?: () => void) => () => void
}
// trong PreloadApi:  mcp: McpBridgeApi
```

**File:** `frontend/src/renderer/src/web/web-mcp-api.ts` (NEW) — tách khỏi `web-preload-api.ts` (4478 dòng; không thêm dòng vào file khổng lồ, đúng tinh thần AGENTS.md "không bump max-lines"):

```ts
import type { McpBridgeApi } from '../../../preload/api-types'
import type { McpEvent } from '../../../shared/mcp-types'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'

export type McpTransport = {
  callRuntimeResult: <T>(method: string, params?: unknown, timeoutMs?: number) => Promise<T>
  /** null = chưa có môi trường runtime (giống nativeChat: trả teardown rỗng, không lỗi). */
  openStream: (method: string, params: unknown, handlers: {
    onResponse: (r: RuntimeRpcResponse<unknown>) => void; onClose: () => void
  }) => Promise<{ unsubscribe: () => void }> | null
}

export function createMcpApi(t: McpTransport): McpBridgeApi {
  return {
    call: (method, ...params) => t.callRuntimeResult(method, params[0] ?? {}) as never, // dialect: params -> args[0] (object)
    subscribeEvents: (onEvent, onClose) => {
      let cancelled = false
      let handle: { unsubscribe: () => void } | null = null
      const p = t.openStream('mcp.events.subscribe', {}, {
        onResponse: (r) => { if (cancelled || !r.ok || r.result == null) return; if (isMcpEvent(r.result)) onEvent(r.result) },
        onClose: () => { if (!cancelled) onClose?.() }
      })
      if (!p) return () => {}
      void p.then((h) => { if (cancelled) h.unsubscribe(); else handle = h }).catch(() => { if (!cancelled) onClose?.() })
      return () => { cancelled = true; handle?.unsubscribe() }
    }
  }
}
// ack đầu tiên có result=null bị bỏ qua ở trên; frame sau là McpEvent trần (push_bridge pushEventResult).
function isMcpEvent(v: unknown): v is McpEvent { /* type ∈ 5 loại của CONTRACT §1 */ }
```

**File:** `frontend/src/renderer/src/web/web-preload-api.ts` (MODIFY — chỉ thêm namespace, nguyên tắc 11/19): 1 import + 1 property trong object trả về của `createWebPreloadApi` (cạnh `admin: createAdminApi()`, dòng ~795):

```ts
import { createMcpApi } from './web-mcp-api'
// …
mcp: createMcpApi({
  callRuntimeResult,
  openStream: (method, params, h) => {
    const environment = requireActiveEnvironmentOrNull()
    return environment ? getClientForEnvironment(environment).subscribe(method, params, { onResponse: h.onResponse, onClose: h.onClose }) : null
  }
}),
```

### Bước 4 — Runtime client

**File:** `frontend/src/renderer/src/runtime/runtime-mcp-client.ts` (NEW) — **điểm vào duy nhất** cho mọi FE-MCP-SOL (không ai gọi `window.api.mcp` trực tiếp ⇒ lỗi luôn được parse):

```ts
export const mcpClient = {
  call: async <M extends McpRpcMethod>(method: M, ...params: McpRpcParams<M> extends void ? [] : [McpRpcParams<M>]): Promise<McpRpcResult<M>> => {
    try { return await window.api.mcp.call(method, ...params) } catch (e) { throw parseMcpError(e) }
  },
  subscribeEvents: (onEvent: (e: McpEvent) => void, onClose?: () => void) => window.api.mcp.subscribeEvents(onEvent, onClose),
  isBridgeAvailable: (): boolean => typeof window !== 'undefined' && typeof window.api?.mcp === 'object'
}
```

### Bước 5 — Slice

**File:** `frontend/src/renderer/src/store/slices/mcp-slice.ts` (NEW), đăng ký ở `store/index.ts` (import `createMcpSlice` + `...createMcpSlice(...a)` sau `createConnectivitySlice`) và `store/types.ts` (import type `McpSlice` + `& McpSlice` sau `ConnectivitySlice`) — **MODIFY** hai file.

```ts
export type McpTabId = 'connect' | 'apps' | 'tokens' | 'approvals' | 'tools' | 'policy' | 'prompts' | 'audit' | 'servers'
export type McpSlice = {
  mcpServerInfo: McpServerInfo | null
  mcpServerInfoStatus: 'idle' | 'loading' | 'ready' | 'error'
  mcpServerInfoError: string | null
  mcpAdminSetupAvailable: boolean                   // admin + info.enabled=false + mcp.admin.settings.get thành công => hiện thẻ "bật MCP" (xem Bước 5, Trạng thái UI)
  mcpEventsState: 'off' | 'connecting' | 'on'
  mcpResyncCounter: number                          // tăng sau mỗi lần nối lại luồng ⇒ tab tự nạp lại dữ liệu
  mcpNavigation: { tab: McpTabId; focusId?: string } | null   // deep link, tiêu thụ một lần bởi McpPane
  refreshMcpServerInfo: () => Promise<void>
  applyMcpEvent: (event: McpEvent) => void          // killswitch.changed -> cập nhật info; còn lại chỉ fan-out qua mcp-event-bus
  startMcpEvents: () => () => void                  // idempotent; trả hàm dừng
  openMcpTab: (tab: McpTabId, focusId?: string) => void     // openSettingsPage() + openSettingsTarget({pane:'mcp',repoId:null}) + set mcpNavigation
  clearMcpNavigation: () => void
  resetMcp: () => void                              // khi đăng xuất
}
// D6: tenant mới mặc định tenantEnabled=true (MCP_TENANT_DEFAULT_ENABLED); chỉ ẩn khi tenant bị tắt tường minh
export const selectMcpEnabled = (s: AppState): boolean => s.mcpServerInfo?.enabled === true && s.mcpServerInfo?.tenantEnabled !== false
export const selectMcpSectionVisible = (s: AppState): boolean => selectMcpEnabled(s) || s.mcpAdminSetupAvailable
export const selectMcpKillSwitchActive = (s: AppState): boolean => s.mcpServerInfo?.killSwitch.active === true
```

Hành vi `refreshMcpServerInfo` (thêm): khi kết quả là `enabled:false` **và** `currentUser.role==='admin'`, gọi thêm `mcpClient.call('mcp.admin.settings.get')` — thành công ⇒ `mcpAdminSetupAvailable=true` (MCP bật ở mức process nhưng tenant đang tắt: theo CONTRACT §1/§6, `server.info.enabled` = chỉ cờ process `MCP_ENABLED` và `tenantEnabled` = cài đặt tenant; FE coi *khả dụng* = `enabled && tenantEnabled !== false`, nên nhánh `enabled:false` ở trên cũng bao gồm `enabled:true && tenantEnabled:false`. **D6:** tenant mới có `tenantEnabled=true` theo mặc định (`MCP_TENANT_DEFAULT_ENABLED`), nên thẻ "bật MCP" chỉ xuất hiện khi admin đã tắt hoặc vận hành đặt mặc định `false`; admin vẫn tắt lại được ở FE-MCP-SOL-008); `MCP_DISABLED`/not-implemented/lỗi khác ⇒ `false`. Nhờ đó admin không bị kẹt "MCP tắt nên không có chỗ bật" (FE-MCP-SOL-008 cung cấp công tắc; FE-012 §Rollout chốt hành vi).

Hành vi `refreshMcpServerInfo`: bridge không có ⇒ `status 'ready'`, `info=null`; gọi `mcpClient.call('mcp.server.info')`; `McpRpcError` với `isMcpDisabledError` hoặc `isMcpNotImplementedError` ⇒ `info = {enabled:false,…rỗng}` (không lỗi); lỗi khác ⇒ giữ `info` cũ, `status:'error'`, `mcpServerInfoError = e.detail`. Chống đua: biến `requestSeq` module-level, chỉ áp dụng kết quả mới nhất.

`startMcpEvents` (module-level `stop`/`retryTimer`, **một** subscription cho cả app):

```ts
const unsub = mcpClient.subscribeEvents((ev) => get().applyMcpEvent(ev), () => scheduleReconnect())
// scheduleReconnect: backoff 1s,2s,4s…≤30s (≤5 phút nếu lỗi not-implemented), chỉ khi selectMcpEnabled && còn authed;
// sau khi nối lại: set({ mcpResyncCounter: n+1 }) và void get().refreshMcpServerInfo()
```

**File:** `frontend/src/renderer/src/lib/mcp-event-bus.ts` (NEW): `subscribeMcpEvents(listener): () => void`, `emitMcpEvent(event)` (Set listener; lỗi listener không làm hỏng listener khác). **File:** `frontend/src/renderer/src/hooks/useMcpEvent.ts` (NEW): `useMcpEvent<T extends McpEvent['type']>(type, handler)` với cleanup `useEffect`.

### Bước 6 — Đồng bộ toàn cục

**File:** `frontend/src/renderer/src/hooks/useMcpSync.ts` (NEW) + **MODIFY** `hooks/useIpcEvents.ts`: thêm `useMcpSync()` ngay dưới `useDevServersSync()` (HIGH impact — một dòng).

```ts
export function useMcpSync(): void {
  const authed = useAppStore((s) => s.currentUser !== null)
  const enabled = useAppStore(selectMcpEnabled)
  useEffect(() => {                                   // 1. nạp server.info; làm mới khi tab quay lại + mỗi 5 phút
    if (!authed || !mcpClient.isBridgeAvailable()) { useAppStore.getState().resetMcp(); return }
    void useAppStore.getState().refreshMcpServerInfo()
    const onVisible = (): void => { if (document.visibilityState === 'visible') void useAppStore.getState().refreshMcpServerInfo() }
    document.addEventListener('visibilitychange', onVisible)
    const timer = window.setInterval(onVisible, 5 * 60_000)
    return () => { document.removeEventListener('visibilitychange', onVisible); window.clearInterval(timer) }
  }, [authed])
  useEffect(() => {                                   // 2. luồng sự kiện: chỉ khi enabled
    if (!authed || !enabled) return
    return useAppStore.getState().startMcpEvents()
  }, [authed, enabled])
}
```

### Bước 7 — Section Settings "MCP" (3 chỗ bắt buộc)

**(a)** `frontend/src/renderer/src/hooks/useSettingsNavigationMetadata.ts` (MODIFY): thêm tham số `isMcpEnabled = false` (giá trị truyền vào là `selectMcpSectionVisible`, tức bật **hoặc** admin-setup) vào `buildSettingsNavigationMetadata` + hook (`useAppStore(selectMcpSectionVisible)`, thêm vào deps `useMemo`), và chèn **trước** mục `...repos.map(`:

```ts
...(isMcpEnabled ? [{
  id: 'mcp',
  title: translate('auto.mcp.nav.title', 'MCP'),
  description: translate('auto.mcp.nav.description', 'Let AI agents such as Claude Code, Claude Desktop, and Cursor work in Orca on your behalf.'),
  icon: Plug,                                  // lucide-react, thêm vào import
  searchEntries: getMcpPaneSearchEntries(isAdmin),
  group: 'capabilities'
}] : []),
```

(Nhóm `capabilities` = "AI Capabilities"; không thêm nhóm mới để khỏi đụng `SETTINGS_NAV_GROUPS`.) Cũng thêm `'mcp'` vào union `SettingsNavTarget` (`lib/settings-navigation-types.ts`).

**(b)** `frontend/src/renderer/src/components/settings/Settings.tsx` (MODIFY): cạnh `DevToolsPane` (dòng ~112) thêm lazy; trong JSX, sau khối `admin-org` (dòng ~1612):

```tsx
const McpPane = lazy(() => import('./mcp/McpPane').then((m) => ({ default: m.McpPane })))
// …
{isMcpEnabled ? (
  <SettingsSection id="mcp" title={translate('auto.mcp.nav.title', 'MCP')}
    description={translate('auto.mcp.nav.description', 'Let AI agents … on your behalf.')}
    searchEntries={getSectionSearchEntries('mcp')}>
    {isSectionMounted('mcp') ? (<Suspense fallback={<McpPaneSkeleton />}><McpPane /></Suspense>) : null}
  </SettingsSection>
) : null}
```

`const isMcpEnabled = useAppStore(selectMcpSectionVisible)` cạnh `isAdmin`. Dùng chuỗi dịch giống (a) để sidebar/Cmd+J/section không lệch.

**(c)** `frontend/src/renderer/src/components/settings/mcp-search.ts` (NEW): `export const getMcpPaneSearchEntries = (isAdmin: boolean) => …` bằng `createLocalizedCatalog` — mục: "MCP server" (từ khoá `mcp`, `model context protocol`, `agent`, `claude`, `cursor`), "Connect an agent", "Active sessions", "Access tokens", "Connected apps", "Approvals", và (nếu admin) "Tool catalog", "Policies", "Prompts", "Audit log", "External MCP servers". Chỉ khai báo mục thuộc tab mà người dùng thấy.

### Bước 8 — Khung `McpPane` & đăng ký tab

**File:** `frontend/src/renderer/src/components/settings/mcp/McpPane.tsx` (NEW) — đọc `mcpServerInfo`; hiển thị theo bảng "Trạng thái UI"; dùng `Tabs` của `components/ui/tabs`. **File:** `…/mcp/mcp-tab-registry.ts` (NEW):

```ts
export type McpTabDefinition = {
  id: McpTabId
  titleKey: string; titleDefault: string
  adminOnly: boolean
  visible?: (info: McpServerInfo) => boolean          // vd 'servers' chỉ khi …
  load: () => Promise<{ default: ComponentType }>     // lazy: mỗi tab một chunk
}
export const MCP_TABS: readonly McpTabDefinition[] = [
  { id: 'connect', adminOnly: false, load: () => import('./McpConnectTab') /* FE-002 */ },
  // FE-003 'apps', FE-004 'tokens', FE-009 'approvals', FE-005 'tools', FE-008 'policy',
  // FE-007 'prompts', FE-010 'audit', FE-011 'servers' — mỗi solution thêm đúng MỘT dòng ở đây.
]
```

`McpPane` đọc `mcpNavigation` một lần (`useEffect`) → chọn tab → `clearMcpNavigation()`. Tab admin chỉ render khi `currentUser.role==='admin'`; tab `value` không hợp lệ ⇒ rơi về `connect`. Tab chưa có solution ⇒ không xuất hiện trong danh sách (không tab rỗng).

**File:** `…/mcp/McpPaneSkeleton.tsx` (NEW) — khung tải đơn giản (token `bg-muted`, `animate-pulse` đã dùng trong repo).

### Bước 9 — Deep link `/?section=mcp&tab=…&approval=…`

CONTRACT §4 (D5) dùng URL `/?section=mcp&tab=approvals&approval=<id>` cho Web Push. App **không** định tuyến theo URL cho Settings (điều hướng bằng store: `openSettingsPage`/`openSettingsTarget`) và không có parser nào cho `section=`. Thêm:

**File:** `frontend/src/renderer/src/lib/mcp-deep-link.ts` (NEW): `parseMcpDeepLink(loc: Pick<Location,'pathname'|'search'>): { tab: McpTabId; focusId?: string } | null` (chấp nhận `section=mcp` với mọi `pathname`, gồm `/` hiện hành và `/settings` cũ; `tab` lạ ⇒ `connect`; `approval`/`token`/`app` ⇒ `focusId`). Gọi trong `useMcpSync()` (hiệu ứng thứ 3) **sau** khi `enabled===true` lần đầu: `openMcpTab(tab, focusId)` rồi `history.replaceState(null,'', loc.pathname==='/settings' ? '/' : loc.pathname)` để không lặp; cũng xử lý message `{type:'orca:navigate', url}` từ service worker (FE-009 Bước 6). Nếu `enabled===false` ⇒ bỏ qua (không tiết lộ UI). *Phụ thuộc:* không cần SPA-fallback riêng vì deep link dùng đường dẫn gốc `/`; push thật còn phụ thuộc CR-NOTIF-002 (FE-009).

### Bước 10 — i18n

Khoá `auto.mcp.*` (fallback tiếng Anh trong `translate`; thêm vào `locales/en.json` dưới `auto.mcp`): `nav.title`, `nav.description`, `pane.loading`, `pane.error`, `pane.retry`, `pane.unsupported`, `killswitch.banner`, `common.copy`, `common.copied`, `common.close`, `common.cancel`, `tabs.<tabId>`. Chuỗi do server (`McpScopeDescriptor.label/description`) ưu tiên khoá `auto.mcp.scope.<id>` rồi rơi về chuỗi server (CONTRACT §1) — helper `mcpScopeLabel(scope)` đặt ở `lib/mcp-labels.ts` (NEW) dùng chung. Không gọi `translate()` ở top-level (đặt trong hàm/component).

## Bề mặt xuất (các solution khác được phép dựa vào)

| Symbol | File | Dùng bởi |
|---|---|---|
| Kiểu `Mcp*`, `McpEvent`, `MCP_ERROR_CODES`, `McpErrorCode`, `McpRpcSchema`, `McpRpcMethod/Params/Result`, `MCP_RPC_METHODS`, `MCP_STREAM_METHODS`, `McpAdminSettings` | `frontend/src/shared/mcp-types.ts` | tất cả |
| `McpRpcError`, `parseMcpError`, `isMcpDisabledError`, `isMcpNotImplementedError` | `renderer/src/runtime/runtime-mcp-error.ts` | tất cả |
| `mcpClient.call / subscribeEvents / isBridgeAvailable` | `renderer/src/runtime/runtime-mcp-client.ts` | tất cả (thay cho `window.api.mcp`) |
| `window.api.mcp` (`McpBridgeApi`) | `preload/api-types.ts`, `web/web-mcp-api.ts` | chỉ `runtime-mcp-client.ts` |
| `McpSlice`, `McpTabId`, `selectMcpEnabled`, `selectMcpSectionVisible`, `selectMcpKillSwitchActive`; state `mcpServerInfo`, `mcpAdminSetupAvailable`, `mcpResyncCounter`, `mcpNavigation`; actions `refreshMcpServerInfo`, `openMcpTab`, `clearMcpNavigation` | `store/slices/mcp-slice.ts` | 002, 003, 004, 005, 008, 009, 010 |
| `subscribeMcpEvents`, `emitMcpEvent`, `useMcpEvent(type, handler)` | `lib/mcp-event-bus.ts`, `hooks/useMcpEvent.ts` | 002 (session.closed), 003 (grant.revoked), 008 (killswitch), 009 (approval.*) |
| `MCP_TABS`, `McpTabDefinition` | `components/settings/mcp/mcp-tab-registry.ts` | mỗi solution thêm một dòng |
| `McpPane`, `McpPaneSkeleton` | `components/settings/mcp/` | Settings |
| `parseMcpDeepLink` | `lib/mcp-deep-link.ts` | 009 |
| `mcpScopeLabel`, `mcpRiskLabel` | `lib/mcp-labels.ts` | 002, 003, 004, 005, 008 |
| `useMcpSync` | `hooks/useMcpSync.ts` | `useIpcEvents` |

## Trạng thái UI

| Trạng thái | Biểu hiện |
|---|---|
| Chưa đăng nhập / bridge vắng / `enabled:false` / `tenantEnabled:false` (user thường) / `MCP_DISABLED` / kênh chưa cài (user thường) | **Không** có mục MCP trong sidebar/Cmd+J; không toast, không lỗi console; deep link bị bỏ qua |
| `enabled:false` hoặc `tenantEnabled:false` (tenant đã bị admin tắt / vận hành đặt `MCP_TENANT_DEFAULT_ENABLED=false`), nhưng là admin và `mcpAdminSetupAvailable` | Mục MCP hiện cho admin; `McpPane` chỉ render thẻ "MCP is turned off for your organization" kèm điều khiển bật do FE-MCP-SOL-008 cung cấp (`McpAdminEnableCard`, import lazy trong `McpPane`); mọi tab khác ẩn |
| Đang tải `server.info` lần đầu (mở Settings ngay khi app khởi động) | `McpPaneSkeleton` (chỉ khi section được mount) |
| `enabled:true` | tab theo vai trò; `killSwitch.active` ⇒ banner cảnh báo cố định phía trên tab (tông `border-destructive`/`text-destructive` từ token), nội dung `killswitch.banner` + `reason` |
| Lỗi tải `server.info` (mạng) khi đã từng ready | giữ UI cũ + dải nhỏ "Couldn't refresh MCP status" với nút Retry (`variant="ghost"`) |
| Mất luồng sự kiện | chấm trạng thái ẩn; tab tự nạp lại khi `mcpResyncCounter` đổi; không banner (tránh ồn) |
| Forbidden (tab admin, user thường) | tab không render; deep link tới tab admin ⇒ rơi về `connect` |

## A11y & i18n & style

- Theo `guides/STYLEGUIDE.md` (xác nhận ở README FE): chỉ dùng token trong `assets/main.css`, không hard-code màu; `Tabs` của `components/ui/tabs`; nút Cancel là `ghost`; `destructive` chỉ cho thao tác mất dữ liệu.
- `Plug` icon có `aria-hidden`; banner kill switch `role="status"` (không `alert` vì primitive `alert` không tồn tại trong `components/ui/`); tab có nhãn dịch được; focus quản lý bởi Radix Tabs.
- Mọi `translate` nằm trong hàm/component (test `no-top-level-translate`); chuỗi mới có fallback tiếng Anh.

## Files cần sửa

| File | Action |
|---|---|
| `frontend/src/shared/mcp-types.ts` | NEW |
| `frontend/src/preload/api-types.ts` | MODIFY — `McpBridgeApi` + `mcp:` trong `PreloadApi` |
| `frontend/src/renderer/src/web/web-mcp-api.ts` | NEW |
| `frontend/src/renderer/src/web/web-preload-api.ts` | MODIFY — 1 import + property `mcp` (không sửa gì khác) |
| `frontend/src/renderer/src/runtime/runtime-mcp-error.ts`, `runtime-mcp-client.ts` | NEW |
| `frontend/src/renderer/src/store/slices/mcp-slice.ts` | NEW |
| `frontend/src/renderer/src/store/index.ts`, `store/types.ts` | MODIFY — đăng ký slice |
| `frontend/src/renderer/src/lib/{mcp-event-bus,mcp-deep-link,mcp-labels}.ts` | NEW |
| `frontend/src/renderer/src/hooks/{useMcpSync,useMcpEvent}.ts` | NEW |
| `frontend/src/renderer/src/hooks/useIpcEvents.ts` | MODIFY — thêm `useMcpSync()` |
| `frontend/src/renderer/src/hooks/useSettingsNavigationMetadata.ts` | MODIFY — mục `mcp`, tham số `isMcpEnabled` |
| `frontend/src/renderer/src/lib/settings-navigation-types.ts` | MODIFY — `'mcp'` vào `SettingsNavTarget` |
| `frontend/src/renderer/src/components/settings/Settings.tsx` | MODIFY — lazy `McpPane` + `<SettingsSection id="mcp">` |
| `frontend/src/renderer/src/components/settings/mcp-search.ts` | NEW |
| `frontend/src/renderer/src/components/settings/mcp/{McpPane.tsx,McpPaneSkeleton.tsx,mcp-tab-registry.ts}` | NEW |
| `frontend/src/renderer/src/i18n/locales/en.json` | MODIFY — nhánh `auto.mcp` |

## Verification

```bash
cd /opt/repos/orca/frontend
npx tsc --noEmit -p tsconfig.json
npx vitest run --config config/vitest.config.ts \
  src/shared/mcp-types.test.ts \
  src/renderer/src/runtime/runtime-mcp-error.test.ts src/renderer/src/runtime/runtime-mcp-client.test.ts \
  src/renderer/src/web/web-mcp-api.test.ts \
  src/renderer/src/store/slices/mcp-slice.test.ts \
  src/renderer/src/lib/mcp-deep-link.test.ts \
  src/renderer/src/hooks/useSettingsNavigationMetadata.test.ts \
  src/renderer/src/i18n/no-top-level-translate.test.ts
cd /opt/repos/orca && pnpm run lint:switch-exhaustiveness && npx oxlint
```

Test chính: (1) `mcp-types.test.ts` — 17 mã khớp CONTRACT; kiểu `McpRpcSchema` biên dịch với fixture mỗi kiểu. (2) `runtime-mcp-error.test.ts` — bảng: `"MCP_NOT_FOUND: x"`, `"rpc error: code = NotFound desc = MCP_NOT_FOUND: x"`, chuỗi lạ ⇒ `code:null`. (3) `web-mcp-api.test.ts` — frame ack `null` bị bỏ qua; `McpEvent` trần được chuyển; `unsubscribe` trước khi `openStream` resolve thì gọi `h.unsubscribe()`; không môi trường ⇒ teardown rỗng. (4) `mcp-slice.test.ts` — `MCP_DISABLED`/not-implemented ⇒ `enabled:false` không lỗi; chỉ áp kết quả mới nhất (đua); `killswitch.changed` cập nhật info; `startMcpEvents` idempotent; backoff có `vi.useFakeTimers`; `resetMcp` dừng luồng. (5) metadata test — `isMcpEnabled=false` ⇒ không có `id:'mcp'`; `true` ⇒ có, nhóm `capabilities`; mục `searchEntries` admin chỉ khi `isAdmin`. (6) test `Settings` render `McpPane` lazy chỉ khi `enabled` (happy-dom, mock `mcpClient`). Thủ công: tắt `MCP_ENABLED` ⇒ Settings không có mục MCP; bật ⇒ có và mở được.

## Sửa TDD kèm theo

- `tdd/v5/02-state-management.md` (≈36 slice, lỗi thời): thêm `McpSlice`; ghi số slice thật khi cập nhật.
- `tdd/v5/03-runtime-client-layer.md`: bổ sung `runtime-mcp-client` + giới hạn của `subscribeRuntimeStreamChannel` với target `local` (web).
- `tdd/v5/07-hooks-and-ipc.md`: `useMcpSync` trong `useIpcEvents`.
- Tài liệu API: thêm `mcp.*` vào danh mục RPC frontend (nếu có `specs/frontend/api/rpc-catalog.md` — **chưa xác minh** vị trí hiện tại).

## Không làm ở solution này

UI từng tab (FE-002..011), trang `/oauth/consent` (FE-003), hộp thoại phê duyệt toàn cục (FE-009), telemetry/Playwright (FE-012), thay đổi preload Electron ở `desktop/`, sửa `McpConfigSection` (cấu hình `.mcp.json` theo repo).
