# FE-MCP-SOL-009: Hộp thoại phê duyệt toàn cục + tab "Approvals" (inbox/lịch sử) + deep link

> 🔲 Designed — chưa implement. **Component bảo mật quan trọng nhất của UI MCP**: đây là chốt chặn con người duy nhất giữa agent và hành động `exec/destructive`.

## CR Reference

- **CR:** [CR-MCP-013](../../../../../../docs/crs/v5/mcp-governance-safety/CR-MCP-013-approvals-audit-killswitch.md) §A (phê duyệt ngoài kênh, hiển thị nguyên văn, gắn hash), CONTRACT §4 (deep link `/?section=mcp&tab=approvals&approval=<id>` + Web Push; D5). **Mức độ:** 🔴 P0. **Phạm vi:** chỉ phần FE.

## Backend dependency (CONTRACT)

| Kênh | BE | Ghi chú |
|---|---|---|
| `mcp.events.subscribe` → push `mcp.event` (`approval.requested`, `approval.resolved`, `killswitch.changed`) | BE-MCP-SOL-013 §D | sự kiện là **gợi ý**; nguồn sự thật là `mcp.approval.list` |
| `mcp.approval.list {status?:'pending'\|'all', cursor?, limit?}` → `{approvals, nextCursor?}` | BE-MCP-SOL-013 | chỉ approval của chính user |
| `mcp.approval.decide {approvalId, decision, paramsHash, note?}` → `McpApproval` | BE-MCP-SOL-013 §B | lỗi: `MCP_APPROVAL_EXPIRED`, `MCP_APPROVAL_ALREADY_DECIDED`, `MCP_APPROVAL_HASH_MISMATCH`, `MCP_NOT_FOUND`, `MCP_KILL_SWITCH_ACTIVE` |
| Web Push khi app không focus (notification-service `DeliverPush`) | **CR-NOTIF-002** (`docs/crs/v4/notification/CR-NOTIF-002-deliver-push-usecase.md`) + BE-MCP-SOL-013 §D | **Phụ thuộc thật:** chưa có `DeliverPush` ⇒ push chưa bao giờ được gửi; đường trong-app (hộp thoại) không phụ thuộc |
Khi BE chưa có: `approvalList` ném lỗi ⇒ tab hiện lỗi + Retry; prompt không bao giờ hiện (không có dữ liệu để giả lập). **Không mock approval** ở bất kỳ môi trường nào.

## Phụ thuộc FE-MCP-SOL-001 (tên đã chốt, không định nghĩa lại) và điểm phối hợp

Dùng: `frontend/src/shared/mcp-types.ts` (`McpApproval`, `McpEvent`, `McpRisk`), `window.api.mcp`, slice `mcp-slice.ts` (`mcpServerInfo`, `refreshMcpServerInfo`), section `mcp`, `parseMcpError`, `isMcpSurfaceAvailable()`. **Một luồng `mcp.events.subscribe` duy nhất mỗi tab** (BE giới hạn 5 stream/user): FE-001 sở hữu việc mở stream và phải cung cấp `onMcpEvent(listener): () => void` (fan-out trong renderer, đếm tham chiếu). Nếu FE-001 chưa có khi implement, thêm `lib/mcp-event-stream.ts` (NEW, singleton) và để FE-001 dùng chung — **không** mở stream thứ hai.

## Impact analysis (gitnexus) — chưa chạy, chạy ngay trước khi sửa

| Symbol | Lệnh | Rủi ro dự kiến |
|---|---|---|
| `App` (`App.tsx`) | `impact({target:"App", direction:"upstream"})` | HIGH về fan-in (gốc ứng dụng) nhưng chỉ **thêm** 1 lazy import + 1 khối JSX; báo người dùng trước khi sửa |
| `AppState` / `store/types.ts`, `store/index.ts` | `impact({target:"AppState", direction:"upstream"})` | MEDIUM (kiểu trung tâm); chỉ thêm 1 slice |
Sau khi sửa: `detect_changes({scope:"compare", base_ref:"main"})`.

## Bối cảnh (đã xác nhận lại trong code)

- **Điểm gắn toàn cục có tiền lệ đúng loại:** `App.tsx` dòng 687 `const hasSshCredentialRequest = useAppStore((s) => s.sshCredentialQueue.length > 0)` và dòng ~2816 `{hasSshCredentialRequest ? <Suspense><RecoverableRenderErrorBoundary boundaryId="modal.ssh-passphrase" surface="modal" …><SshPassphraseDialog/></…></Suspense> : null}`; `SshPassphraseDialog` (`components/settings/SshPassphraseDialog.tsx`) đọc `sshCredentialQueue[0]` từ slice `store/slices/ssh.ts`. Lazy import theo khuôn `const SshPassphraseDialog = lazy(() => import(...))` (`App.tsx:362`, `lazy` = `lazyWithRetry`). Khối này nằm **trong** `ConfirmationDialogProvider`/`TooltipProvider`, và `<Toaster/>` ở cuối ⇒ dùng được `useConfirmationDialog`, tooltip, sonner.
- Web cũng render `<App/>` (`web/main-web-bootstrap.tsx` `WebRoot`, `web/pair-code-app-entry.tsx:75`) ⇒ một điểm gắn phủ cả Electron-remote và web.
- Settings **không** định tuyến bằng URL: điều hướng là `openSettingsTarget({pane, repoId, sectionId?})` + `openSettingsPage()` (`store/slices/ui.ts:779-786,1646`; `SettingsNavTarget` ở `lib/settings-navigation-types.ts:8`, FE-001 thêm `'mcp'`). Deep link `/?section=mcp&tab=approvals&approval=<id>` (CONTRACT §4, D5) vì vậy cần một bộ đọc URL → store (Bước 6). Đường dẫn gốc `/` luôn được trả về SPA nên không cần SPA-fallback riêng; parser không phụ thuộc pathname (vẫn chấp nhận `/settings?...` cũ).
- **Service worker (D5, đã chốt):** `web/main-web-bootstrap.tsx:455` đăng ký `/service-worker.js`. Vite `root = src/renderer` nên `publicDir` mặc định là `frontend/src/renderer/public/` ⇒ nguồn SW là **`frontend/src/renderer/public/service-worker.js`** (**đã xác minh tồn tại**, JS thuần, sao chép nguyên văn từ publicDir). Hành vi: `push` ⇒ `showNotification` từ payload JSON `{title, body, deepLink, tag}`; `notificationclick` ⇒ tìm client đang mở của app, nếu có thì `focus()` và `postMessage({type:'orca:navigate', url: deepLink})`, nếu không thì `clients.openWindow(deepLink)`. SW chỉ chấp nhận `deepLink` là đường dẫn tương đối cùng origin (từ chối `//host`, `/\\host`, chuỗi > 2048 ký tự ⇒ rơi về `/`), `skipWaiting` + `clients.claim` để bản sửa có hiệu lực ngay. Payload BE (`pushPayload{id,type,title,body,deepLink}`) đặt `deepLink` ở cấp cao nhất, không trong `data`. TDD v4/10 `/push/...` là lỗi thời so với mã (`/api/vapid-public-key`, `/api/push-subscribe`, `/api/push-unsubscribe`).
- **Phía SPA nhận deep link (Settings không có route URL, điều hướng bằng store):** deep link cố định `/?section=mcp&tab=approvals&approval=<id>` (CONTRACT §4). (i) **Cold start** (`openWindow`): khi boot, SPA parse `location.search` bằng `parseMcpDeepLink` rồi gọi `openSettingsTarget({pane:'mcp', repoId:null})` + `openSettingsPage()`; (ii) **tab đang mở:** lắng nghe `navigator.serviceWorker` `message` `{type:'orca:navigate', url}` và chạy cùng parser (Bước 6). **Hộp thoại trong app (từ `mcp.event` + `approvalList` lúc mount) vẫn là đường chính**; push chỉ phục vụ khi app không focus/đã đóng.
- **Phụ thuộc thật còn lại — CR-NOTIF-002:** `notification-service` **chưa có usecase `DeliverPush`** ([CR-NOTIF-002](../../../../../../docs/crs/v4/notification/CR-NOTIF-002-deliver-push-usecase.md); README service mục "Known gaps": `event.Channels` được gán nhưng không code nào đọc để gửi push, `SignVapidPayload` chưa có caller). Hệ quả: dù SW và deep link đã sẵn sàng, **push không bao giờ thực sự được gửi** cho tới khi CR-NOTIF-002 hoàn tất. Đường out-of-focus của FE-009 vì thế **phụ thuộc CR-NOTIF-002** (cùng BE-MCP-SOL-013 §D); đường trong-app không phụ thuộc.
- **Lỗi hiện có của `useWebPushSubscription` (đã xác minh):** các `fetch('/api/vapid-public-key')`, `fetch('/api/push-subscribe', …)`, `fetch('/api/push-unsubscribe', …)` **không đặt `credentials: 'include'`** (mặc định `same-origin` vẫn gửi cookie khi SPA và API cùng origin sau nginx; nhưng sẽ không gửi nếu khác origin). Ghi nhận để sửa khi triển khai push thật (follow-up nhỏ, ngoài phạm vi FE-009 nếu cùng origin); `catch {}` nuốt lỗi nên `failed` không có chi tiết.
- `mcp.approval.decide` chỉ chủ approval + `paramsHash` khớp (CONTRACT); FE luôn gửi lại `approval.paramsHash` nhận từ server, không tự tính.

## Giải pháp

### Bước 1 — API (thêm vào `McpApi` của FE-001)

```ts
approvalList: (p: { status?: 'pending' | 'all'; cursor?: string; limit?: number }) =>
  callRuntimeResult<{ approvals: McpApproval[]; nextCursor?: string }>('mcp.approval.list', p)
approvalDecide: (p: { approvalId: string; decision: 'approve' | 'deny'; paramsHash: string; note?: string }) =>
  callRuntimeResult<McpApproval>('mcp.approval.decide', p)        // một object ở args[0] (R-2)
```
Kiểu bổ sung: `McpApprovalPage` trong `frontend/src/shared/mcp-governance-types.ts` (file của FE-008; thêm additive).

### Bước 2 — Slice **chỉ trong bộ nhớ**

**File:** `frontend/src/renderer/src/store/slices/mcp-approval-slice.ts` (NEW; đăng ký ở `store/index.ts` + `store/types.ts`, khuôn `ssh.ts`)

```ts
export type McpApprovalSlice = {
  mcpApprovalQueue: McpApproval[]                 // chỉ status 'pending', cũ → mới (createdAt)
  mcpApprovalLocalDeadline: Record<string, number> // id → Date.now()-based deadline (xem countdown)
  mcpApprovalPromptOpen: boolean                   // false sau "Decide later" tới khi có approval MỚI hoặc người dùng mở lại
  mcpUiIntent: McpUiIntent | null                  // McpUiIntent = { tab?: 'approvals'|'policies'|'audit'; approvalId?: string }
  enqueueMcpApproval: (a: McpApproval, receivedAt: number) => void   // dedupe theo id; bỏ nếu không pending
  resolveMcpApproval: (id: string) => void
  replaceMcpApprovals: (list: McpApproval[], fetchedAt: number) => void // đồng bộ lại từ server (nguồn sự thật)
  focusMcpApproval: (id: string) => void           // đưa lên đầu hàng + mở prompt
  setMcpApprovalPromptOpen: (open: boolean) => void
  setMcpUiIntent: (i: McpUiIntent | null) => void
  clearMcpApprovals: () => void                    // gọi khi logout/ MCP bị tắt
}
```
**Bất biến bảo mật:** slice **không** nằm trong bất kỳ danh sách persist nào (kiểm tra `store/slices/*` — Zustand không dùng middleware persist; lưu bền là theo từng slice qua `window.api.ui.set`, **chưa xác minh** hết các đường); không ghi vào `localStorage/sessionStorage/IndexedDB`, URL, `history.state`, telemetry, `console.*`, thông báo hệ thống. Test `mcp-approval-slice.persist.test.ts` khẳng định snapshot UI được persist không chứa khoá `mcp*`.

### Bước 3 — `McpGlobalLayer` (điểm gắn duy nhất, additive vào `App.tsx`)

**File:** `renderer/src/components/mcp/McpGlobalLayer.tsx` (NEW, lazy). Việc: (1) nếu `serverInfo.enabled` ⇒ `approvalList({status:'pending', limit:50})` → `replaceMcpApprovals`; (2) `onMcpEvent`: `approval.requested` ⇒ `enqueueMcpApproval(e.approval, Date.now())` (+ `setMcpApprovalPromptOpen(true)`), `approval.resolved` ⇒ `resolveMcpApproval(e.id)`, `killswitch.changed` ⇒ `refreshMcpServerInfo()`; (3) đồng bộ lại khi tab hiện lại (`visibilitychange`) và mỗi 30s **chỉ khi** `document.visibilityState==='visible'` (phòng mất sự kiện; stream BE đóng khi tràn bộ đệm); (4) đọc deep link (Bước 6); (5) render `<McpApprovalPrompt/>`. Teardown mọi listener/interval (nguyên tắc 12 của TDD v5). Khi `enabled` chuyển `false` hoặc logout ⇒ `clearMcpApprovals()`.

**File:** `renderer/src/App.tsx` (MODIFY — chỉ 3 phần additive, đặt cạnh khối `SshPassphraseDialog`):
```tsx
const McpGlobalLayer = lazy(() => import('./components/mcp/McpGlobalLayer'))              // cạnh dòng 362
// …trong App(): const mcpSurface = isMcpSurfaceAvailable()                                  // cạnh dòng 687
{mcpSurface ? (<Suspense fallback={null}><RecoverableRenderErrorBoundary boundaryId="modal.mcp-approval" surface="modal" resetKey={activeModal} compact><McpGlobalLayer /></RecoverableRenderErrorBoundary></Suspense>) : null}
```
`isMcpSurfaceAvailable()` (FE-001) rẻ, không gọi mạng; khi false thì **không tải chunk** nào của MCP.

### Bước 4 — Hộp thoại phê duyệt

**File:** `renderer/src/components/mcp/McpApprovalPrompt.tsx` (NEW) + `mcp-approval-display.ts` (NEW)

Bố cục (`Dialog` — "quyết định bắt buộc", STYLEGUIDE bảng chọn bề mặt; `DialogContent` rộng vừa `sm:max-w-xl`):
```
Tiêu đề: "An AI agent is asking permission"         (mô tả: "Review exactly what will run. Nothing runs until you approve.")
Hàng 1:  [Bot] {clientName}  ·  phiên …{sessionId.slice(-6)}                    "Request 1 of 3"
Hàng 2:  {tool.title}  <code>{tool.name}</code>   [RiskBadge]                  "Expires in 4:32"
Khối:    <pre aria-label="Exact tool arguments" tabIndex=0 class="font-mono text-xs whitespace-pre-wrap break-all max-h-64 overflow-auto rounded-md border bg-muted p-3">{text}</pre>
         [Badge "Secrets hidden"] khi argsPreview.redacted
Chân:    [Decide later (ghost)]                         [Deny (ghost)] [Approve (default)]
```
Quy tắc cứng:
1. `argsPreview.text` hiển thị **nguyên văn trong font mono, là text node React** (`{text}`), **không** `dangerouslySetInnerHTML`, không markdown, không linkify, không cắt/diễn giải. Ngoại lệ duy nhất, chỉ để hiển thị: `visualizeControlChars(text)` thay ký tự điều khiển/vô hình/đảo chiều (U+0000–001F trừ `\n\t`, U+007F, U+200B–200F, U+202A–202E, U+2060–2064, U+2066–2069, U+FEFF) bằng `\u{XXXX}` có thể thấy (lớp phòng thủ thứ hai sau lớp escape của server). Marker `[REDACTED]` do server chèn được giữ nguyên.
2. `clientName` (do client tự đặt qua DCR ⇒ không tin cậy): chỉ text node, `truncate` + `title`, không HTML.
3. **Risk badge không chỉ bằng màu**: icon + chữ — `read` (`secondary`, "Read"), `write_reversible` (`outline`, "Changes data"), `exec` (`outline` + icon `Terminal`, "Runs commands"), `destructive`/`admin` (`destructive`, "Destructive"/"Admin"). Màu trạng thái lỗi cho `destructive`/`admin` là mô tả *khả năng nguy hiểm* (state), không phải nút.
4. **Deny là ghost yên lặng** (`variant="ghost"`, không màu, không chip phím); **Approve** `variant="default"`; "Decide later" ghost nhỏ, nhãn trung tính. *Không* dùng `destructive` ở đây (không mất dữ liệu).
5. **Không bao giờ tự duyệt:** không timer/effect/cleanup/keyboard shortcut nào gọi `approvalDecide({decision:'approve'})`; chỉ có **một** call-site, trong `onClick` của nút Approve. Test tĩnh `mcp-approval-no-auto-approve.test.ts` đọc mã nguồn thư mục `components/mcp/` và khẳng định đúng 1 lần xuất hiện `decision: 'approve'`.
6. **Chống Enter/Space vô tình** (người dùng đang gõ trong terminal khi hộp thoại bật lên): `onOpenAutoFocus={(e) => { e.preventDefault(); denyRef.current?.focus() }}` — tiêu điểm vào **Deny** (thất bại an toàn), *lệch có chủ đích* khỏi quy tắc "focus vào hành động chính" của STYLEGUIDE vì ở đây hành động chính là hành động nguy hiểm; không có `<form>` nào; `onKeyDown` ở `DialogContent` chặn `Enter` khi `target` không phải `button`; **Approve phải được click/Tab-tới rồi kích hoạt chủ động**.
7. **Khoá trễ trước khi bấm được Approve** (`mcp-approval-display.ts`): `APPROVE_LOCK_MS = { read: 0, write_reversible: 600, exec: 1500, destructive: 2000, admin: 2000 }`; nhãn "Approve (2s)" `disabled` + `aria-disabled`; đồng hồ khoá chỉ chạy khi `document.visibilityState==='visible' && document.hasFocus()`, **đặt lại** mỗi khi cửa sổ lấy lại focus hoặc approval hiện hành đổi (chống bấm mù ngay khi bật). Click handler tự kiểm: `if (lockActive || !e.isTrusted) return`.
8. **Hàng đợi:** hiển thị `queue[0]` (hoặc approval vừa `focusMcpApproval`), tiêu đề "Request i of N"; sau mỗi quyết định chuyển sang cái kế (khoá trễ chạy lại). "Decide later" ⇒ `setMcpApprovalPromptOpen(false)` + toast sonner "N request(s) waiting for approval" kèm hành động "Review" (mở lại); approval **mới** sẽ mở lại prompt. Nếu ≥3 pending từ cùng client: dòng thông tin "This client has N pending requests" + liên kết sang tab Approvals (không có thao tác hàng loạt — CONTRACT không có).
9. **Đếm ngược** (`hooks/useMcpApprovalDeadline.ts`): `deadline = receivedAt + (Date.parse(expiresAt) − Date.parse(createdAt)) − 2000` với sự kiện live (miễn nhiễm lệch đồng hồ); với mục nạp từ `list`: `Date.parse(expiresAt)` so `Date.now()`. Hiển thị `<time dateTime={expiresAt}>`, cập nhật 1 Hz bằng một interval dùng chung (có cleanup), `aria-live="off"` (không đọc từng giây); mốc 30s/10s đặt `aria-live="polite"` một lần. Về 0 ⇒ nút bị khoá, nhãn "Expired", gọi `approvalList` một lần để đối chiếu server (nếu server còn `pending` vì lệch đồng hồ ⇒ giữ hành động được, hiển thị "Expiring…"); tự dọn khỏi hàng đợi sau 3s khi server xác nhận.
10. **Kill switch tenant đang bật** (`mcpServerInfo.killSwitch.active`): hiện dòng "MCP access is suspended" và khoá Approve (server cũng trả `MCP_KILL_SWITCH_ACTIVE`).

Xử lý kết quả `approvalDecide` (`parseMcpError`): thành công ⇒ `resolveMcpApproval(id)` + toast ngắn ("Approved"/"Denied"). `MCP_APPROVAL_EXPIRED` ⇒ toast "This request expired", gỡ khỏi hàng đợi. `MCP_APPROVAL_ALREADY_DECIDED` ⇒ toast thông tin "Already handled (another device)", gỡ. `MCP_APPROVAL_HASH_MISMATCH` ⇒ **không gỡ**: hiển thị lỗi inline "This request changed. Review it again.", gọi `approvalList` để thay bằng bản mới và **đặt lại khoá trễ** (không bao giờ tự gửi lại). `MCP_NOT_FOUND` ⇒ gỡ. `MCP_KILL_SWITCH_ACTIVE` ⇒ inline + banner. Lỗi mạng/khác ⇒ giữ hộp thoại, lỗi inline, cho thử lại (nút bật lại, khoá trễ ngắn 600ms). Gửi: khoá cả hai nút + `aria-busy` ngay khi bấm (SSH-latency; tránh double-submit); `decision:'deny'` gửi `paramsHash` như cũ (BE bỏ qua với deny).

### Bước 5 — Tab "Approvals" (inbox + lịch sử)

**File:** `renderer/src/components/settings/mcp/McpApprovalsTab.tsx` (NEW), `McpApprovalRow.tsx` (NEW)

- Hai chế độ (`Tabs`/`ToggleGroup`): **Pending** (đọc từ slice, đã đồng bộ) và **History** (`approvalList({status:'all', limit:50})`, phân trang con trỏ bằng nút "Load more"; lọc client bỏ mục `pending`). Dữ liệu History giữ trong **state component**, xoá khi unmount.
- Hàng: tool title + `<code>` tên, client, risk badge, trạng thái (`Badge`: approved=`secondary`, denied=`outline`, expired/cancelled=`outline` + chữ), thời gian tạo/quyết định, `decidedVia` (Web/Mobile/Client prompt). Nút "Show arguments" (mặc định **thu gọn** — giảm lộ nội dung khi người khác nhìn màn hình; hiện khi bấm, cùng `<pre>` nguyên văn như trên). Hàng `pending`: nút **Review** ⇒ `focusMcpApproval(id)` (mở prompt, không duyệt tại hàng — luôn qua cùng một đường có khoá trễ).
- Deep link `approvalId`: sau khi tải, `scrollIntoView` + viền `ring`; không thấy trong 3 trang đầu ⇒ dòng "This request is no longer available" (không lỗi). Nếu approval đang `pending` ⇒ đồng thời `focusMcpApproval`.
- Trạng thái UI: loading (`Skeleton`), empty ("No approval requests yet. When an AI agent needs your OK, it will appear here."), error (`role="alert"` + Retry), disabled-by-flag (không mount), kill switch (banner của FE-008 phía trên).

### Bước 6 — Deep link `/?section=mcp&tab=approvals&approval=<id>` (cold start + `orca:navigate`)

**File:** `renderer/src/components/mcp/mcp-deep-link.ts` (NEW)
```ts
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
export function parseMcpDeepLink(search: string): McpUiIntent | null {
  const q = new URLSearchParams(search)
  if (q.get('section') !== 'mcp') return null                       // KHÔNG phụ thuộc pathname
  const tab = q.get('tab'); const approval = q.get('approval')
  return { tab: tab === 'approvals' || tab === 'policies' || tab === 'audit' ? tab : undefined,
           approvalId: approval && UUID.test(approval) ? approval : undefined }
}
```
`McpGlobalLayer` gọi lúc mount và khi `popstate`: nếu có intent ⇒ `setMcpUiIntent(intent)` → `openSettingsTarget({pane:'mcp', repoId:null})` + `openSettingsPage()` → `history.replaceState(null, '', location.pathname)` bỏ query (không để id lại trong URL/lịch sử). `McpSettingsSection` (FE-001) đọc `mcpUiIntent` để chọn tab ban đầu rồi `setMcpUiIntent(null)`. Tuyệt đối **không** điều hướng tới URL tuỳ ý: chỉ chấp nhận ba giá trị `tab` và UUID. Khi SW (`frontend/src/renderer/public/service-worker.js`) gửi `message {type:'orca:navigate', url}` tới tab đang mở: chỉ lấy `new URL(url, location.origin)`, kiểm `origin` trùng, rồi chạy cùng `parseMcpDeepLink(u.search)`; mọi thứ khác bị bỏ.

## Ghi chú bảo mật tổng hợp
- Không auto-approve; không phím tắt duyệt; một call-site duy nhất (có test tĩnh). Không lưu `argsPreview`/`paramsHash` ngoài bộ nhớ (slice không persist; History ở state component; không `console`, không telemetry của FE-012 chứa trường này). Không render dưới dạng HTML. `clientName`/`tool.title` là text node. Tin `paramsHash` của server (không tự tính). Không thay đổi quyết định theo gợi ý từ nội dung preview (không phân tích ngữ nghĩa). Clear queue khi logout/`enabled=false`.
- Phía BE đã chặn: token MCP không tới được `mcp.approval.decide`; chỉ chủ approval; elicitation không đủ cho `exec/destructive`. UI không thêm kênh nào khác.

## A11y & i18n & style
- `Dialog` có `DialogTitle`/`DialogDescription`; focus trap sẵn của Radix; `aria-describedby` trỏ vào mô tả + khối args; `<pre tabIndex=0>` cuộn bằng bàn phím; nhãn nút nêu đối tượng ("Approve {tool.title}" qua `aria-label`); trạng thái khoá đọc bằng `aria-disabled` + văn bản hiển thị; không dùng âm thanh. Tôn trọng `prefers-reduced-motion` (không animation đếm ngược).
- i18n: `translate('auto.mcp.approval.*', 'English')` trong thân component; dữ liệu do server/agent đưa (tên client, tool title, args) **không** dịch/không nội suy vào chuỗi i18n bằng `dangerouslySetInnerHTML`; dùng placeholder `{{name}}` của i18next với `escapeValue` mặc định.
- Style: token (`bg-muted`, `border`, `font-mono`), không màu mới; `ring` cho hàng được làm nổi từ deep link.

## Files cần sửa
| File | Action |
|---|---|
| `renderer/src/store/slices/mcp-approval-slice.ts`, `store/index.ts`, `store/types.ts` | NEW + MODIFY (đăng ký slice) |
| `renderer/src/components/mcp/{McpGlobalLayer.tsx, McpApprovalPrompt.tsx, mcp-approval-display.ts, mcp-deep-link.ts}` | NEW |
| `renderer/src/hooks/useMcpApprovalDeadline.ts` | NEW |
| `renderer/src/components/settings/mcp/{McpApprovalsTab.tsx, McpApprovalRow.tsx}` | NEW |
| `renderer/src/App.tsx` | MODIFY — 3 phần additive (lazy import, `isMcpSurfaceAvailable()`, khối JSX cạnh `SshPassphraseDialog`) |
| `preload/api-types.ts`, `renderer/src/web/web-preload-api.ts` | MODIFY — `approvalList`, `approvalDecide` |
| `frontend/src/shared/mcp-governance-types.ts` | MODIFY (additive, tạo bởi FE-008) — `McpApprovalPage` |
| `renderer/src/i18n/locales/en.json` | MODIFY — `auto.mcp.approval.*` |
| `frontend/src/renderer/public/service-worker.js` | Đã có (D5): `push` ⇒ notification từ `{title, body, deepLink, tag}`; `notificationclick` ⇒ focus client + `postMessage({type:'orca:navigate', url})` hoặc `openWindow(deepLink)`. FE-009 không sở hữu file này nhưng test hợp đồng với nó |
| `renderer/src/web/main-web-bootstrap.tsx` | MODIFY nhỏ (additive): đăng ký listener `navigator.serviceWorker` `message` → `McpGlobalLayer` (hoặc đặt trong `McpGlobalLayer` như Bước 6) |

## Verification
```bash
cd frontend && npx vitest run src/renderer/src/components/mcp/ src/renderer/src/store/slices/mcp-approval-slice.test.ts \
  src/renderer/src/store/slices/mcp-approval-slice.persist.test.ts src/renderer/src/components/settings/mcp/McpApprovalsTab.test.tsx
cd frontend && npx vitest run src/renderer/src/i18n/no-top-level-translate.test.ts
cd frontend && npx tsc --noEmit -p tsconfig.json
```
Ca bắt buộc: (1) preview là text — payload chứa `<img onerror=…>`/`<script>` render thành chữ, không tạo node (`screen.queryByRole('img')` null); ký tự RLO hiện `\u{202E}`; (2) focus ban đầu ở Deny; Enter ngay khi mở **không** gọi `approvalDecide`; Approve disabled đúng `APPROVE_LOCK_MS` theo risk, đặt lại khi blur→focus (fake timers); (3) chỉ click chủ động gửi `approve` kèm đúng `paramsHash` của server; click giả (`isTrusted=false`) bị bỏ qua; (4) mỗi mã lỗi `EXPIRED/ALREADY_DECIDED/HASH_MISMATCH/NOT_FOUND/KILL_SWITCH` có hành vi đúng (HASH_MISMATCH không gỡ & không tự gửi lại); (5) hàng đợi 3 approval xử lý tuần tự, "Decide later" đóng nhưng approval mới mở lại; (6) deep link: URL hợp lệ ⇒ mở tab Approvals + làm nổi hàng; UUID sai/`tab` lạ/`section` khác ⇒ bỏ qua; query bị xoá khỏi URL; (7) slice không nằm trong persisted snapshot; (8) test tĩnh đúng 1 call-site `approve`; (9) countdown không gọi API mỗi giây, dọn interval khi unmount. Thủ công (cần BE-013): agent gọi `terminal_send` ⇒ prompt hiện trong vài giây ở tab đang mở; đóng tab, mở lại ⇒ prompt xuất hiện nhờ `approvalList`; duyệt trên thiết bị A thì thiết bị B nhận `approval.resolved` và đóng.

## Sửa TDD kèm theo
`specs/frontend/tdd/v4/10-web-push-ui.md §2`: sửa đường dẫn `/push/...` ⇒ `/api/vapid-public-key|push-subscribe|push-unsubscribe`; mô tả SW phải đọc `payload.deepLink` (khoá cấp cao nhất như BE gửi), `postMessage({type:'orca:navigate'})` tới client đang mở hoặc `openWindow(deepLink)`; ghi nguồn tại `frontend/src/renderer/public/service-worker.js`; ghi phụ thuộc CR-NOTIF-002 (`DeliverPush`); ghi `useWebPushSubscription` thiếu `credentials:'include'`. `specs/frontend/tdd/v5/02-state-management.md` (đã lỗi thời): bổ sung slice `mcp-approval-slice` và nguyên tắc "state nhạy cảm không persist".

## Không làm ở solution này
Quyết định hàng loạt/"nhớ lựa chọn"/"cho phép trong phiên" (CONTRACT không có); duyệt ngay tại hàng danh sách (luôn qua prompt có khoá trễ); thông báo hệ điều hành tự dựng ngoài Web Push; triển khai `DeliverPush` ở notification-service (CR-NOTIF-002, phía BE) và viết service worker (việc hiện thực riêng, FE-009 chỉ định nghĩa hợp đồng `orca:navigate`); hiển thị approval của người khác (server chỉ trả của chính user).
