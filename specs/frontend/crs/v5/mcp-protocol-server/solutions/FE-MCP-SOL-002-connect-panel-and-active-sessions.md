# FE-MCP-SOL-002: Tab "Connect" — địa chỉ MCP, snippet cho Claude Code / Claude Desktop / Cursor, bảng phiên đang hoạt động

> 🔲 Designed — chưa implement. Xây trên [FE-MCP-SOL-001](../../mcp-service-foundation/solutions/FE-MCP-SOL-001-mcp-frontend-foundation.md) (kiểu, `mcpClient`, slice, `MCP_TABS`, `useMcpEvent`).

## CR Reference

- **CR:** [CR-MCP-003](../../../../../../docs/crs/v5/mcp-protocol-server/CR-MCP-003-streamable-http-and-lifecycle.md) (kết nối `claude mcp add --transport http orca https://…/mcp`, phiên bản giao thức), [CR-MCP-004](../../../../../../docs/crs/v5/mcp-protocol-server/CR-MCP-004-sessions-sse-resumability.md) (session, đóng session) — **chỉ phần FE**: hiển thị thông tin kết nối + quản lý phiên của chính mình, admin xem phiên mọi user.
- **Mức độ:** 🟠 P1 (không chặn backend; là cửa vào để người dùng biết cách nối agent).
- **TDD:** [`tdd/v5/05-ui-components.md`](../../../../tdd/v5/05-ui-components.md) (lỗi thời — chỉ dùng nguyên tắc chung), [`tdd/v4/03-admin-spa.md`](../../../../tdd/v4/03-admin-spa.md).

## Backend dependency

| Kênh (CONTRACT) | Tham số (1 object) | Kết quả | BE |
|---|---|---|---|
| `mcp.server.info` (đã nạp ở slice FE-001) | — | `McpServerInfo` | [BE-MCP-SOL-003](../../../../../backend-go/crs/v5/mcp-protocol-server/solutions/BE-MCP-SOL-003-streamable-http-and-lifecycle.md) |
| `mcp.session.list` | — | `McpSessionView[]` (chính user) | [BE-MCP-SOL-004](../../../../../backend-go/crs/v5/mcp-protocol-server/solutions/BE-MCP-SOL-004-sessions-sse-resumability.md) |
| `mcp.session.close` | `{ sessionId }` (row id, **không** phải `Mcp-Session-Id`) | `{ok:true}` | BE-MCP-SOL-004 |
| `mcp.admin.session.list` | — | `McpSessionView[]` (mọi user; có `userId`, `userName?`) | BE-MCP-SOL-004 |
| `McpEvent` `session.closed{sessionId}` | luồng `mcp.events.subscribe` | — | BE-MCP-SOL-013 (+004 phát outbox) |

Lỗi dùng: `MCP_DISABLED` (ẩn tab — do pane), `MCP_NOT_ADMIN` (ẩn công tắc "All users"), `MCP_NOT_FOUND` (đóng session không tồn tại/không của mình ⇒ coi như đã đóng, nạp lại). **Khi BE chưa có** `mcp.session.*`: lỗi "not yet implemented" ⇒ bảng hiển thị trạng thái "Sessions aren't available yet" (không toast); phần snippet vẫn dùng được vì chỉ cần `mcp.server.info`.

## Impact analysis (gitnexus) — **chưa chạy**

Toàn bộ file mới; chỉ sửa `mcp-tab-registry.ts` (do FE-001 tạo). Lệnh: `impact({target:"MCP_TABS", direction:"upstream"})` — kỳ vọng LOW (chỉ `McpPane`). Không đụng symbol có sẵn khác.

## Bối cảnh (đã xác nhận lại)

- `components/ui/` có `table`, `tabs`, `badge`, `button`, `input`, `skeleton`, `tooltip`, `dialog`; **không có** `switch`, `alert`, `alert-dialog`. Công tắc "All users" dùng `Checkbox` (`components/ui/checkbox.tsx`) hoặc `ToggleGroup`. Xác nhận hộp thoại bằng `useConfirmationDialog()` (`components/confirmation-dialog.tsx:120`, tuỳ chọn `confirmVariant:'destructive'`).
- Sao chép: dự án dùng trực tiếp `navigator.clipboard.writeText(...)` (ví dụ `workflow/ExecutionMonitor.tsx:38`); báo kết quả bằng `toast` từ `sonner` (`components/ui/sonner.tsx`, dùng ở `SshPane.tsx`).
- Làm tươi định kỳ khi cửa sổ hiện: `installWindowVisibilityInterval({ run, intervalMs })` (`lib/window-visibility-interval.ts`).
- Không có helper thời gian tương đối dùng chung (mỗi file tự có, ví dụ `LinearItemDrawer.tsx:104`) ⇒ tạo `lib/mcp-relative-time.ts` gọn, dùng `Intl.RelativeTimeFormat`.
- Snippet kết nối là nội dung dành cho **client bên thứ ba**: định dạng lệnh/JSON của Claude Code, Claude Desktop, Cursor theo hiểu biết hiện tại, **chưa đối chiếu với tài liệu phiên bản hiện hành của từng client** (không truy cập mạng khi viết) ⇒ checklist ở "Verification" bắt buộc xác nhận trước khi phát hành; builder tách thuần để sửa nhanh.
- `McpServerInfo.resourceUrl` là nguồn duy nhất của địa chỉ (không tự ghép từ `location.origin`).

## Giải pháp

### Bước 1 — Builder snippet (hàm thuần)

**File:** `frontend/src/renderer/src/lib/mcp-connect-snippets.ts` (NEW)

```ts
import type { McpServerInfo } from '../../../shared/mcp-types'
export const MCP_TOKEN_PLACEHOLDER = '<YOUR_ACCESS_TOKEN>'    // KHÔNG bao giờ chèn token thật vào snippet (C6)
export type McpClientKind = 'claude-code' | 'claude-desktop' | 'cursor'
export type McpSnippet = { kind: McpClientKind; language: 'bash' | 'json'; withOAuth: string; withToken: string; notes: string[] }

export function buildMcpSnippet(kind: McpClientKind, info: Pick<McpServerInfo, 'resourceUrl'>): McpSnippet {
  const url = info.resourceUrl
  switch (kind) {
    case 'claude-code':
      return { kind, language: 'bash',
        withOAuth: `claude mcp add --transport http orca ${url}`,
        withToken: `claude mcp add --transport http orca ${url} --header "Authorization: Bearer ${MCP_TOKEN_PLACEHOLDER}"`,
        notes: [] }
    case 'cursor': // ~/.cursor/mcp.json
      return { kind, language: 'json',
        withOAuth: JSON.stringify({ mcpServers: { orca: { url } } }, null, 2),
        withToken: JSON.stringify({ mcpServers: { orca: { url, headers: { Authorization: `Bearer ${MCP_TOKEN_PLACEHOLDER}` } } } }, null, 2),
        notes: [] }
    case 'claude-desktop': // claude_desktop_config.json qua cầu nối mcp-remote
      return { kind, language: 'json',
        withOAuth: JSON.stringify({ mcpServers: { orca: { command: 'npx', args: ['-y', 'mcp-remote', url] } } }, null, 2),
        withToken: JSON.stringify({ mcpServers: { orca: { command: 'npx', args: ['-y', 'mcp-remote', url, '--header', `Authorization: Bearer ${MCP_TOKEN_PLACEHOLDER}`] } } }, null, 2),
        notes: ['claudeDesktop.connectorsHint'] }  // khoá i18n: "Newer Claude Desktop versions can add a remote server from Settings → Connectors instead."
  }
}
export function isInsecureMcpUrl(url: string): boolean {
  try { const u = new URL(url); return u.protocol !== 'https:' && !['localhost', '127.0.0.1', '[::1]'].includes(u.hostname) } catch { return true }
}
```

Dùng `JSON.stringify` để chuỗi luôn hợp lệ (URL chứa ký tự đặc biệt không phá JSON). Snippet "OAuth" là mặc định (client tự đăng ký DCR + consent — FE-003); "Token" là phương án cho CLI/script (liên kết sang tab `tokens` của FE-004 qua `openMcpTab('tokens')`; nếu tab chưa có trong `MCP_TABS` thì ẩn liên kết).

### Bước 2 — Hook dữ liệu phiên

**File:** `frontend/src/renderer/src/hooks/useMcpSessions.ts` (NEW) — state cục bộ (không thêm slice; dữ liệu chỉ phục vụ tab này).

```ts
export type McpSessionsScope = 'mine' | 'all'
export function useMcpSessions(scope: McpSessionsScope): {
  sessions: McpSessionView[]; status: 'loading' | 'ready' | 'error' | 'unavailable' | 'forbidden'
  error: string | null; reload: () => void; closeSession: (id: string) => Promise<void>
} {
  const resync = useAppStore((s) => s.mcpResyncCounter)
  // load(): mcpClient.call(scope === 'all' ? 'mcp.admin.session.list' : 'mcp.session.list')
  //   McpRpcError.code === 'MCP_NOT_ADMIN' -> 'forbidden' ; isMcpNotImplementedError -> 'unavailable' ; MCP_DISABLED -> 'unavailable'
  // đua: requestSeq ref, bỏ kết quả cũ khi scope đổi nhanh
  // làm tươi: installWindowVisibilityInterval({ run: reload, intervalMs: 15_000 }) ; cleanup khi unmount/scope đổi
  // sự kiện: useMcpEvent('session.closed', (e) => setSessions((xs) => xs.filter((s) => s.id !== e.sessionId)))
  // nối lại luồng (mcpResyncCounter đổi) -> reload()
  // closeSession: optimistic KHÔNG; gọi 'mcp.session.close' {sessionId}; thành công hoặc MCP_NOT_FOUND -> reload()
}
```

### Bước 3 — Thời gian tương đối

**File:** `frontend/src/renderer/src/lib/mcp-relative-time.ts` (NEW) — `formatMcpRelativeTime(iso: string, now = Date.now()): string` bằng `Intl.RelativeTimeFormat(undefined, {numeric:'auto'})`; ISO không hợp lệ ⇒ `'—'`. Phần tử `<time dateTime={iso} title={new Date(iso).toLocaleString()}>`.

### Bước 4 — Component

**File:** `frontend/src/renderer/src/components/settings/mcp/McpConnectTab.tsx` (NEW, default export cho `lazy`)

Bố cục (một cột, `space-y-6`):

1. **Hàng trạng thái** — nhãn "Server address" + `Input readOnly` chứa `info.resourceUrl` + nút `Copy` (ghost, `aria-label` dịch được; sau khi copy đổi nhãn "Copied" 1.5s + `toast.success`); cảnh báo nhỏ khi `isInsecureMcpUrl` ("This address isn't HTTPS. Most agents refuse insecure remote MCP servers."); `Badge` liệt kê `info.protocolVersions`; dòng chú thích "Authorization server: {info.authorizationServer}" (văn bản phụ, ẩn khi rỗng).
2. **Banner kill switch** đã do `McpPane` (FE-001) hiển thị; tab chỉ thêm dòng: khi `killSwitch.active` ⇒ nút copy snippet vẫn bật nhưng thêm chú thích "Agents are currently blocked by an administrator." (không vô hiệu hoá — người dùng vẫn cần cấu hình).
3. **Connect an agent** — `Tabs` con (`claude-code` | `claude-desktop` | `cursor`); mỗi tab: chuyển đổi `ToggleGroup` "Sign in with browser (OAuth)" / "Use an access token", khối `<pre><code>` (token `bg-muted`, `font-mono`, cuộn ngang `overflow-x-auto`), nút Copy; ghi chú theo `notes`; liên kết "Create an access token" (nếu tab `tokens` đã đăng ký).
4. **Active sessions** — tiêu đề + (admin) `Checkbox` "Show all users" (mặc định tắt; chỉ render khi `currentUser.role==='admin'`; nếu gọi `all` nhận `MCP_NOT_ADMIN` ⇒ ẩn công tắc, quay `mine`). `Table`:

| Cột | Nguồn |
|---|---|
| Client | `clientName` (rỗng ⇒ "Unknown client") |
| User (chỉ khi `all`) | `userName ?? userId` rút gọn 8 ký tự |
| Connected | `createdAt` tương đối |
| Last active | `lastSeenAt` tương đối |
| Streams | `activeStreams` |
| Tool calls | `toolCalls` |
| Protocol | `protocolVersion` |
| (hành động) | `Close` — `Button variant="ghost" size="sm"` |

`Close`: `useConfirmationDialog()` với tiêu đề "Close this session?", mô tả "The agent will be disconnected and any tool call it is running will be cancelled. It can reconnect by signing in again." (khớp hành vi BE-004: DELETE huỷ tool đang chạy), `confirmVariant:'default'` (không phải mất dữ liệu — hành động hoàn tác được bằng kết nối lại; không dùng `destructive`). Trong chế độ `all`, hàng của người khác: nút `disabled` + `Tooltip` "Only the session owner can close it. To block an agent, use the kill switch." (BE-004: `mcp.session.close` chỉ cho chủ session). Đang đóng: nút `disabled` + "Closing…".

### Bước 5 — Đăng ký tab

**File:** `frontend/src/renderer/src/components/settings/mcp/mcp-tab-registry.ts` (MODIFY — FE-001) thêm đúng dòng:

```ts
{ id: 'connect', titleKey: 'auto.mcp.tabs.connect', titleDefault: 'Connect', adminOnly: false,
  load: () => import('./McpConnectTab').then((m) => ({ default: m.McpConnectTab })) }
```

(Tab `connect` là mặc định khi không có `mcpNavigation`.)

## Trạng thái UI

| Trạng thái | Hiển thị |
|---|---|
| `enabled:false` | tab/section không tồn tại (FE-001) |
| Loading phiên | 3 hàng `Skeleton` trong bảng; snippet hiển thị ngay (không phụ thuộc phiên) |
| Empty | "No agents are connected. Add Orca to your agent using the instructions above." |
| Error mạng | dòng lỗi ngắn (`detail`) + nút "Retry" (ghost); không toast lặp mỗi lần polling — chỉ toast khi người dùng chủ động đóng thất bại |
| `unavailable` (kênh chưa cài / `MCP_DISABLED` giữa chừng) | "Session list isn't available on this server yet." |
| `forbidden` (admin toggle) | công tắc biến mất, quay về "mine" |
| Disabled-by-killswitch | snippet vẫn dùng được; chú thích "Agents are currently blocked…" |
| Insecure URL | cảnh báo dưới ô địa chỉ |

## A11y & i18n & style

- Ô địa chỉ `readOnly` có `<Label>`; nút Copy `aria-label`; thông báo "Copied" qua `aria-live="polite"` (không dùng `alert`). Bảng dùng `Table*` của `components/ui/table`, `scope="col"` ở `TableHead`; hàng không phụ thuộc màu để truyền đạt trạng thái.
- Khoá i18n `auto.mcp.connect.*` (`serverAddress`, `copy`, `copied`, `insecureUrl`, `connectAgent`, `oauthMode`, `tokenMode`, `createToken`, `claudeDesktop.connectorsHint`), `auto.mcp.sessions.*` (`title`, `showAll`, `empty`, `col.*`, `close`, `closing`, `closeConfirmTitle`, `closeConfirmBody`, `ownerOnly`, `unavailable`). **Snippet và tên client là nội dung kỹ thuật, không dịch.** `translate()` chỉ trong hàm/component.
- Style: `guides/STYLEGUIDE.md` (xác nhận ở README FE) — token màu, không hard-code; Cancel là `ghost`; không `destructive` cho đóng session.

## Files cần sửa

| File | Action |
|---|---|
| `frontend/src/renderer/src/lib/mcp-connect-snippets.ts` | NEW |
| `frontend/src/renderer/src/lib/mcp-relative-time.ts` | NEW |
| `frontend/src/renderer/src/hooks/useMcpSessions.ts` | NEW |
| `frontend/src/renderer/src/components/settings/mcp/McpConnectTab.tsx` | NEW |
| `frontend/src/renderer/src/components/settings/mcp/McpSessionsTable.tsx` | NEW (tách để dưới ngưỡng max-lines) |
| `frontend/src/renderer/src/components/settings/mcp/mcp-tab-registry.ts` | MODIFY — 1 dòng `connect` |
| `frontend/src/renderer/src/i18n/locales/en.json` | MODIFY — `auto.mcp.connect`, `auto.mcp.sessions` |

## Verification

```bash
cd /opt/repos/orca/frontend
npx tsc --noEmit -p tsconfig.json
npx vitest run --config config/vitest.config.ts \
  src/renderer/src/lib/mcp-connect-snippets.test.ts src/renderer/src/lib/mcp-relative-time.test.ts \
  src/renderer/src/hooks/useMcpSessions.test.ts \
  src/renderer/src/components/settings/mcp/McpConnectTab.test.tsx
```

- `mcp-connect-snippets.test.ts`: JSON snippet `JSON.parse` được; URL có ký tự lạ vẫn hợp lệ; placeholder luôn là `<YOUR_ACCESS_TOKEN>`; `isInsecureMcpUrl('http://localhost:8081/mcp')=false`, `'http://orca.example.com/mcp'=true`, chuỗi rác `=true`.
- `useMcpSessions.test.ts` (mock `mcpClient`, `vi.useFakeTimers`): `MCP_NOT_ADMIN` ⇒ `forbidden`; not-implemented ⇒ `unavailable`; sự kiện `session.closed` loại hàng; đổi scope nhanh bỏ kết quả cũ; unmount dừng interval (không rò timer).
- `McpConnectTab.test.tsx` (happy-dom): hiển thị `resourceUrl` đúng; Copy gọi `navigator.clipboard.writeText` với snippet đúng chế độ; user thường không thấy công tắc "Show all users"; admin thấy; hàng của người khác có nút Close `disabled`; xác nhận đóng gọi `mcp.session.close` với `{sessionId}` (object), rồi reload.
- Thủ công bắt buộc trước phát hành: đối chiếu từng snippet với tài liệu hiện hành của Claude Code / Claude Desktop / Cursor và chạy thật một lần (ghi kết quả vào PR).

## Sửa TDD kèm theo

`tdd/v5/05-ui-components.md`: thêm mục "Settings > MCP > Connect" (lưu ý tài liệu này đã lỗi thời so với mã — chỉ bổ sung tham chiếu). Hướng dẫn người dùng tương ứng ở `docs/guides/mcp/connect-*.md` (FE-MCP-SOL-012).

## Không làm ở solution này

Tạo/thu hồi token (FE-004), danh sách ứng dụng đã cấp quyền (FE-003), chặn agent bằng kill switch (FE-008), trang consent `/oauth/consent` (FE-003), hiển thị nhật ký tool call (FE-010).
