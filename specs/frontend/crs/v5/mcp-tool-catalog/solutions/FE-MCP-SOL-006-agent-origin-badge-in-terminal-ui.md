# FE-MCP-SOL-006: Nhãn "Tạo bởi agent" + nút dừng cho terminal/agent do MCP tạo

> ✅ Implemented (unit/integration tests) — see Gaps. Dùng kiểu/`window.api.mcp`/`mcp-slice` của FE-MCP-SOL-001 chỉ để **đọc** (xem "Backend dependency"); không định nghĩa lại. Phần "tạo `origin`" là BE-MCP-SOL-009.

## CR Reference

- **CR:** [CR-MCP-009](../../../../../../docs/crs/v5/mcp-tool-catalog/CR-MCP-009-long-running-and-streaming-tools.md) — mục "Ngoài phạm vi: UI hiển thị phiên terminal do agent tạo (frontend)" · **Mức độ:** 🟠 P1 (an toàn: người dùng phải nhìn thấy tiến trình do agent bật trên máy mình)
- **Phạm vi (chỉ FE):** (1) hiển thị nhãn nguồn gốc khi kết quả `terminal.*`/`agent.*` có `origin`; (2) điều khiển **dừng**; (3) **không đổi hành vi** khi `origin` vắng (mọi terminal do UI tạo).

## Backend dependency

| Dữ liệu / kênh | Nguồn | Ghi chú |
|---|---|---|
| `origin?: { type:'mcp'; clientName; mcpSessionId; userId }` trên kết quả `terminal.list`, `terminal.create` (`terminal.handle…`), `agent.start/resume/switchAccount` | CONTRACT §5 · BE-MCP-SOL-009 §F (`omitempty`) | vắng = do UI tạo; client cũ bỏ qua field lạ (C9) |
| Dừng: `terminal.stop {terminal: ptyId}` · `terminal.close {terminal: ptyId}` | kênh **có sẵn** (không phải kênh MCP mới); BE-009 §Hợp đồng | cùng quyền với UI; không cần quyền admin |
| `McpEvent` `session.closed {sessionId}` | CONTRACT §1 (`mcp.events.subscribe`, FE-001 giữ subscription) | dùng để làm mới danh sách |
| `McpServerInfo.enabled` | `mcp.server.info` (FE-001, slice) | `false` ⇒ không poll, không hiển thị |

Khi BE-009 chưa có: `origin` không bao giờ xuất hiện ⇒ mọi tính năng im lặng (không lỗi, không poll thừa nhờ gate `enabled`).

## Impact analysis (gitnexus) — chưa chạy

| Symbol | Lệnh cần chạy trước khi sửa | Rủi ro dự kiến |
|---|---|---|
| `SortableTab` (`components/tab-bar/SortableTab.tsx`) | `gitnexus impact({target:"SortableTab", direction:"upstream"})` | MEDIUM (component tab dùng khắp tab bar; chỉ thêm 1 phần tử có điều kiện) |
| `RuntimeTerminalSummary` (`shared/runtime-types.ts`) | `gitnexus impact({target:"RuntimeTerminalSummary", direction:"upstream"})` | LOW–MEDIUM (thêm field optional) |
| `createAppStore`/`AppState` (`store/index.ts`, `store/types.ts`) | `gitnexus impact({target:"AppState", direction:"upstream"})` | MEDIUM (type trung tâm; chỉ thêm 1 slice) |
| `SortableTabContextMenu` | `gitnexus impact({target:"SortableTabContextMenu", direction:"upstream"})` | LOW |

## Bối cảnh (đã xác nhận lại bằng mã thật, 2026-10-01)

1. **Kiểu terminal phía FE**: `TerminalTab` (`shared/types.ts:846`: `id, ptyId: string|null, worktreeId, title, …, launchAgent?, connectionId?`) là tab **cục bộ** của client; PTY thật gắn qua store `ptyIdsByTabId: Record<tabId, ptyId[]>` (`store/slices/terminals.ts:443`). `RuntimeTerminalSummary` (`shared/runtime-types.ts:370`: `handle, ptyId, worktreeId, tabId, leafId, connected, …`) là hình `terminal.list` mà code FE hiện kỳ vọng (`lib/agent-hibernation-coordinator.ts`, `lib/active-agent-note-target.ts` dùng `callRuntimeRpc(target,'terminal.list',…)` rồi đọc `.terminals`).
2. **Lệch hình thật với backend-go**: `terminal.list` của backend-go trả **mảng trần** `[{ptyId, connectionId, cwd, createdAt, lastActiveAt}]` (`channels_terminal.go`), không phải `{terminals,totalCount,truncated}`; `terminal.create` ack là `{terminal:{ptyId,…,handle}}`. ⇒ client **không được giả định một hình**: phải chuẩn hoá cả hai (Bước 2).
3. **Id PTY ở remote** là `remote:<env>@@<handle>` (`runtime/runtime-terminal-stream.ts` `toRemoteRuntimePtyId/parseRemoteRuntimePtyId`) với `handle == ptyId` của backend (terminal.create đặt `Handle = ptyId`). Khớp `origin` phải dùng `handle`.
4. **PTY do MCP tạo không tự có `TerminalTab`.** Cơ chế khiến PTY ngoài-UI hiện thành tab (mirror `session.tabs` của host trong client web — `components/Terminal.tsx:1046`, `web-preload-api.ts:3957`, kênh `session.tabs.listAll/subscribeAll`) **(chưa xác minh)** có phủ PTY do tool tạo hay không. ⇒ thiết kế hai lớp: (a) nhãn trên tab **khi tab có PTY khớp**; (b) danh sách độc lập "Agent-created terminals" luôn hoạt động và có nút Stop, để người dùng không bao giờ mất khả năng dừng dù không có tab.
5. Không có slice cho terminal-từ-runtime riêng; `store/slices/terminals.ts` rất lớn và dùng chung (nhiều test leak) ⇒ **không sửa** slice này.
6. `SortableTab` nhận `tab: TerminalTab` và đã dùng `useAppStore` (`SortableTab.tsx` import `useAppStore`, `useTabAgent`); menu: `SortableTabContextMenu.tsx`. Primitive: `ui/badge` (variant thật `default|secondary|dot|destructive|outline|ghost|link`), `ui/tooltip`, `useConfirmationDialog()` (`components/confirmation-dialog.tsx:120`; không có `alert-dialog`).
7. `ManageSessionsSection` (`components/settings/`) quản lý PTY của **daemon Electron cục bộ** (`PtyManagementSession`), không phải backend-go ⇒ **không** là nơi gắn nhãn này.

## Giải pháp

### Bước 1 — Kiểu (additive)
**File:** `frontend/src/shared/mcp-types.ts` (FE-001; **chỉ thêm** nếu FE-001 chưa khai báo)
```ts
export interface McpOrigin { type: 'mcp'; clientName: string; mcpSessionId: string; userId: string }  // CONTRACT §5
```
**File:** `frontend/src/shared/runtime-types.ts` (MODIFY): `RuntimeTerminalSummary` thêm `origin?: McpOrigin`. Không thêm vào `TerminalTab` (dữ liệu runtime, không persist: tránh ghi `origin` vào session lưu, và tránh migration).

### Bước 2 — Chuẩn hoá & client lấy nguồn gốc
**File:** `frontend/src/renderer/src/lib/mcp-terminal-origin.ts` (NEW, hàm thuần + 1 hàm gọi RPC)
```ts
export type OriginByHandle = Record<string, McpOrigin>   // key = handle (= ptyId backend)

// Chấp nhận cả mảng trần (backend-go hiện tại) lẫn { terminals: [...] } (hình runtime). Bỏ qua mục không có origin hợp lệ.
export function normalizeTerminalListOrigins(raw: unknown): OriginByHandle

// Khoá so khớp cho ptyId của store: remote:<env>@@<handle> -> handle
export function ptyIdToOriginKey(ptyId: string): string   // dùng parseRemoteRuntimePtyId

export async function fetchMcpTerminalOrigins(target: RuntimeClientTarget): Promise<OriginByHandle>
// = normalizeTerminalListOrigins(await callRuntimeRpc(target, 'terminal.list', {}, { timeoutMs: 8000 }))
```
Hợp lệ `origin` ⇔ `type==='mcp'` và `clientName`, `mcpSessionId`, `userId` là string. `clientName` coi là **không tin cậy** (do client MCP tự khai khi đăng ký) ⇒ chỉ render text thuần, cắt 40 ký tự + `title` đầy đủ, loại ký tự điều khiển/bidi.

### Bước 3 — Slice nhỏ riêng
**File:** `frontend/src/renderer/src/store/slices/mcp-terminal-origin.ts` (NEW) — đăng ký ở `store/index.ts` và `store/types.ts` (hai dòng; mẫu `createAIProviderSlice` ở `index.ts:46,112`, `types.ts:44,107`)
```ts
export type McpTerminalOriginSlice = {
  mcpOriginByHandle: OriginByHandle
  mcpOriginsRefreshedAt: number | null
  setMcpTerminalOrigins: (next: OriginByHandle) => void   // so sánh nông; không set nếu bằng nhau (tránh re-render)
  clearMcpTerminalOrigins: () => void
}
```
Không persist (không nằm trong workspace session). Reset khi `serverInfo.enabled` chuyển `false`.

### Bước 4 — Hook làm mới (có cleanup)
**File:** `frontend/src/renderer/src/hooks/useMcpTerminalOrigins.ts` (NEW) — mount **một lần** (trong `TabBar` hoặc nơi hiển thị danh sách; không ở `App.tsx` — bất biến không sửa `App.tsx`)
- Chỉ chạy khi `serverInfo.enabled` (selector `mcp-slice`, khớp FE-001) **và** có kết nối runtime (`getActiveRuntimeTarget(settings)`).
- Làm mới: lúc mount; mỗi 20 s khi `document.visibilityState==='visible'` **và** (tồn tại ít nhất một phiên MCP đang hoạt động theo `McpSessionView` của slice, hoặc đã từng thấy origin trong 5 phút gần đây) — tránh poll vô ích với người dùng không dùng MCP; ngay khi nhận `McpEvent.session.closed`; khi cửa sổ lấy lại focus.
- Mọi timer/listener/subscription trả hàm teardown (bất biến "mọi `on*()` phải có cleanup"); chống race bằng `requestSeq`; lỗi mạng ⇒ giữ dữ liệu cũ, không toast.

### Bước 5 — Nhãn trên tab
**File:** `frontend/src/renderer/src/components/tab-bar/McpOriginBadge.tsx` (NEW)
```tsx
export function McpOriginBadge(props: { origin: McpOrigin; compact?: boolean }): React.JSX.Element
// <Badge variant="outline"> icon Bot (lucide) + compact ? null : text
// text: translate('auto.mcp.origin.createdByAgent', 'Created by agent {name}') — tiếng Việt khi locale vi: "Tạo bởi agent <clientName>"
// Tooltip: "Started by MCP client "<clientName>". You can stop it from the tab menu."
// aria-label đầy đủ; không dựa vào màu
```
**File:** `.../tab-bar/SortableTab.tsx` (MODIFY, thêm): 
```tsx
const origin = useAppStore((s) => selectMcpOriginForTab(s, tab.id))   // selector bên dưới
{origin ? <McpOriginBadge origin={origin} compact /> : null}           // cạnh icon agent, trước tiêu đề
```
**File:** `.../store/slices/mcp-terminal-origin.ts` export selector `selectMcpOriginForTab(state, tabId)`: `ptyIdsByTabId[tabId]` → `ptyIdToOriginKey` → tra `mcpOriginByHandle`; trả `McpOrigin | null`, ổn định tham chiếu (trả cùng object từ map). **Khi `origin` vắng ⇒ `null` ⇒ không render gì, không đổi layout/DOM của tab hiện tại** (test snapshot DOM trước/sau). Không đụng logic kéo-thả, đóng tab, hay `useTabAgent`.

### Bước 6 — Điều khiển dừng
1. **Menu tab**: `SortableTabContextMenu.tsx` (MODIFY) thêm mục "Stop agent-created process" chỉ khi `origin` có; xác nhận bằng `useConfirmationDialog()` ("This process was started by <clientName> through MCP. Stop it?"; nút xác nhận thường — dừng tiến trình không mất dữ liệu của Orca; nút Cancel ghost).
2. **Danh sách độc lập** `frontend/src/renderer/src/components/settings/mcp/McpAgentTerminalsList.tsx` (NEW): bảng nhỏ gồm các mục trong `mcpOriginByHandle` (Handle rút gọn · Client · Session · Started (nếu có) · **Stop**). Mount làm một khối dưới danh sách phiên MCP của FE-MCP-SOL-002 (tab "Connection/Sessions" trong `McpSettingsPane`) — nơi FE-002 quyết định chỗ gắn; nếu FE-002 chưa có chỗ, mount ở tab `sessions` của FE-001. Đây là đường dừng **không phụ thuộc có tab**.
3. **Hành động** `stopMcpOriginTerminal(handle)` (`lib/mcp-terminal-origin.ts`): `callRuntimeRpc(target,'terminal.stop',{terminal: handle})` (ngắt tiến trình); nếu vẫn còn sau 3 s (làm mới danh sách) hiển thị tuỳ chọn "Force close" → `terminal.close {terminal: handle}`. PTY của agent cũng là PTY: dừng theo `handle` (= ptyId) là đủ; FE **không** gọi `agent.stop` vì `origin` và `terminal.list` không mang `sessionId` (đánh đổi: bản ghi `agent_sessions` được infra-fleet cập nhật khi PTY thoát — (chưa xác minh)). Lỗi → `toast.error(message)` (sonner); thành công → làm mới ngay + `toast.success`. Không có kênh `mcp.*` nào được dùng để dừng (CONTRACT không có).
4. Quyền: mọi user dừng được PTY **của chính họ** (server kiểm); nếu server từ chối ⇒ hiển thị message server.

### Bước 7 — Nơi khác (tuỳ chọn, thấp)
Sidebar/worktree card có chỉ báo agent (`AgentStateDot.tsx`): **không** thêm nhãn mới ở v1 (giảm nhiễu); chỉ tab và danh sách.

## Trạng thái UI

| Trạng thái | Hiển thị |
|---|---|
| `origin` vắng | Không gì thay đổi (DOM tab y hệt hiện tại) |
| `origin` có, tab có PTY khớp | Badge `outline` + icon `Bot` (compact), tooltip, mục menu Stop |
| `origin` có, **không** có tab | Chỉ xuất hiện ở `McpAgentTerminalsList` |
| loading (lần nạp đầu) | Khối danh sách hiển thị `Skeleton` 2 hàng; tab không hiển thị gì |
| empty | "No terminals started by agents." |
| error | Giữ dữ liệu cũ; trong danh sách: "Couldn't refresh. Retry" (nút ghost) |
| stopping | Nút Stop disabled + spinner; sau thành công mục biến mất khi refetch |
| forbidden | Server từ chối stop ⇒ toast message server |
| disabled-by-flag | `enabled=false` ⇒ hook không chạy, badge/list không render |

## A11y & i18n & style

Badge có `aria-label` ("Created by agent X"), không chỉ màu; tooltip hiện khi focus bàn phím; nút Stop có tên truy cập gồm tên client. Khoá `auto.mcp.origin.*` + fallback tiếng Anh trong `translate()` (gọi trong component, không top-level); thêm `en.json`; bản vi: "Tạo bởi agent {name}", "Dừng tiến trình do agent tạo". Màu theo token (`outline`), không hard-code. `clientName` render bằng text node.

## Files cần sửa

| File | Loại |
|---|---|
| `shared/mcp-types.ts` (`McpOrigin` nếu thiếu) | MODIFY (FE-001) |
| `shared/runtime-types.ts` (`RuntimeTerminalSummary.origin?`) | MODIFY |
| `renderer/src/lib/mcp-terminal-origin.ts` (+ `.test.ts`) | NEW |
| `renderer/src/store/slices/mcp-terminal-origin.ts` (+ `.test.ts`), `store/index.ts`, `store/types.ts` | NEW / MODIFY (2 dòng) |
| `renderer/src/hooks/useMcpTerminalOrigins.ts` | NEW |
| `renderer/src/components/tab-bar/McpOriginBadge.tsx` | NEW |
| `renderer/src/components/tab-bar/SortableTab.tsx`, `SortableTabContextMenu.tsx` | MODIFY |
| `renderer/src/components/settings/mcp/McpAgentTerminalsList.tsx` | NEW |
| `renderer/src/i18n/locales/en.json` | MODIFY |

## Verification

```bash
cd frontend
npx vitest run --config config/vitest.config.ts src/renderer/src/lib/mcp-terminal-origin.test.ts          # mảng trần, {terminals}, origin sai kiểu, remote:env@@handle
npx vitest run --config config/vitest.config.ts src/renderer/src/store/slices/mcp-terminal-origin.test.ts   # set nông, selector ổn định
npx vitest run --config config/vitest.config.ts src/renderer/src/components/tab-bar                         # snapshot tab không origin KHÔNG đổi; có origin có badge
npx vitest run --config config/vitest.config.ts src/renderer/src/i18n/no-top-level-translate.test.ts
npx tsc --noEmit -p tsconfig.json
```
Test thêm: hook dọn timer khi unmount (fake timers); `enabled=false` ⇒ không gọi RPC; `clientName` chứa `<script>`/RLO hiển thị thuần; stop → gọi `terminal.stop` rồi (nếu còn) `terminal.close`.

## Sửa TDD kèm theo
`specs/frontend/tdd/v5/02-store` (số slice đã lỗi thời; ghi thêm `mcp-terminal-origin`); ghi rõ hình `terminal.list` thực tế của backend-go khác `RuntimeTerminalListResult`.

## Không làm ở solution này
Tạo/điều khiển terminal MCP từ UI; xem output terminal agent trong UI (dùng cơ chế tab sẵn có nếu có); sửa `terminals.ts`; adopt PTY MCP thành tab (cần xác minh cơ chế session-tabs); hiển thị nhãn ở sidebar.
