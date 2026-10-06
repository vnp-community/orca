# CR-CV-061 — Điểm vào Review: hàng agent báo xong, thông báo, Source Control, Cmd+K, tab Review ở right sidebar

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-061 |
| **Tên** | Các điểm vào màn Review: nút Review ở hàng agent đã xong (`DashboardAgentRow`), thông báo, nút/menu trong Source Control, lệnh trong Cmd+K (`WorktreeJumpPalette`), tab Review ở right sidebar hiển thị tóm tắt; nguồn sự kiện "agent xong" |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (loại tab `review`, `ensureReviewTab`, `useCodeIntelSupport`, slice, i18n), CR-CV-051 (khung Review, thanh tóm tắt, `IndexFreshnessChip`), CR-CV-052 (tiến độ đọc, trạng thái review); backend CR-CV-036 (`ChangeOverlay` cho tóm tắt), CR-CV-040 (kênh `codeIntel.changeOverlay`, `codeIntel.findings`) |
| **Mở khoá** | CR-CV-060 (cần sự kiện "agent xong" từ CR này), CR-CV-073 (E2E, rollout) |
| **Tác động** | `components/dashboard/DashboardAgentRow.tsx`, `DashboardAgentRowTrailingControls.tsx`, `components/sidebar/WorktreeCardAgents.tsx` và `worktree-card-compact-agent-row.tsx` (nối nút), `components/right-sidebar/index.tsx`, `activity-bar-buttons.tsx`, `right-sidebar-panel-content.tsx`, `right-sidebar-activity-visibility.ts`, `components/right-sidebar/SourceControl.tsx` (chỉ nối prop), `source-control-header-toolbar.tsx`, `source-control-header-overflow-menu.tsx`, `components/cmd-j/quick-actions.ts`, `quick-action-context.ts`, `components/WorktreeJumpPalette.tsx`, `store/right-sidebar-route.ts`, `shared/types.ts` (`RightSidebarTab`), `i18n/locales/*.json`; mới: `components/review-map/entry/` |

---

## 1. Bối cảnh và vấn đề

Mục tiêu của [10 §2, §8](../../../research/view-code/10-frontend-review-ux.md): từ khi agent báo xong tới thấy tác động chỉ mất **một cú bấm**, và Review có chỗ vào ở những nơi người dùng vốn đã nhìn: hàng agent, Source Control, Cmd+K, right sidebar. Màn Review là **tab trong tab group** (D7); right sidebar chỉ làm lối vào và tóm tắt. CR này nối các điểm vào đó, không vẽ màn Review (CR-CV-051) và không quyết định bố cục tab (CR-CV-050).

Hiện trạng (đã đọc code):

### 1.1 Sự kiện "agent xong" lấy từ đâu

- **Slice `agent-status`** (`store/slices/agent-status.ts`): `agentStatusByPaneKey: Record<paneKey, AgentStatusEntry>` (thời gian thực, trong bộ nhớ). `AgentStatusEntry` (`shared/agent-status-types.ts`) có `state` ∈ `working | blocked | waiting | done`, `stateStartedAt` (khi vào trạng thái hiện tại), `interrupted?`, `agentType?`, `worktreeId?`, `tabId?`, `prompt`, `stateHistory[]` (tối đa 20). Trạng thái do hook của agent báo qua IPC; mã nguồn ghi rõ không suy đoán trạng thái từ tiêu đề terminal. Agent không có tích hợp hook (shell, REPL, agent lạ) **không có entry** (chú thích ở `components/dashboard/useDashboardData.ts`), nghĩa là nút Review ở hàng agent không bao giờ xuất hiện cho chúng.
- `state='done'` **không** bị hạ xuống `idle` theo TTL (chỉ `working|blocked|waiting` mới bị hạ; chú thích ở `useDashboardData.ts` giải thích để giữ tín hiệu hoàn tất cho cơ chế giữ lại). Khi entry biến mất mà trạng thái trước là `done`, nó được giữ ở `retainedAgentsByPaneKey: Record<paneKey, RetainedAgentEntry {entry, worktreeId, tab, agentType, startedAt}>` tới khi người dùng xác nhận (bấm vào worktree). Cả bảng điều khiển (`useDashboardData`) và hover của sidebar dùng chung hai nguồn này.
- `DashboardAgentRow` (kiểu dữ liệu trong `useDashboardData.ts`): `{paneKey, entry, tab, agentType, rowSource: 'live'|'retained'|'subagent', state, activationPaneKey?, startedAt, lineage?}`; `tab.worktreeId` cho biết worktree. Hàng `subagent` không có pane riêng.
- **Thông báo** có đường riêng: `hooks/agent-hook-completion-notifications.ts` + `components/terminal-pane/agent-completion-coordinator.ts` + `use-notification-dispatch.ts` (`dispatchTerminalNotification`, nguồn `agent-task-complete`, `NotificationDispatchRequest` ở `shared/types.ts` mang `worktreeId`, `paneKey`, `agentState`, `agentInterrupted`…), chính sách ở `terminal-pane/agent-task-complete-policy.ts`. Đây là tín hiệu để **hiển thị thông báo** (có thể đến từ hook hoặc từ việc tiến trình thoát, `agentCompletionSource: 'process-exit'`), chứ không phải kho trạng thái; nguồn đáng tin để biết "có agent xong ở worktree W" vẫn là `agentStatusByPaneKey` + `retainedAgentsByPaneKey`.
- **Nhấp thông báo hệ điều hành** (desktop main, `desktop/src/main/ipc/notifications.ts`): trình xử lý `click` gọi `ui:activateWorktree {repoId, worktreeId}` rồi `ui:focusTerminal {tabId, worktreeId, leafId, ackPaneKeyOnSuccess, flashFocusedPane, scrollToBottomIfOutputSinceLastView}`; thông báo **không có nút hành động**. Ở render target web, `window.api.ui.onActivateWorktree` là hàm rỗng (`web/web-preload-api.ts`), tức nhấp thông báo không định tuyến được vào ứng dụng web theo đường này (chưa kiểm chứng đường thông báo web khác).

Kết luận: định nghĩa sự kiện (mới) `AgentTurnCompletion = { worktreeId, paneKey, agentType, doneAt: entry.stateStartedAt, interrupted, source: 'live' | 'retained', id: "${paneKey}:${doneAt}" }`, suy ra từ `agentStatusByPaneKey` (state `done`) hợp với `retainedAgentsByPaneKey` (state `done`), bỏ hàng `subagent`. CR-CV-060 tái dùng đúng danh tính này làm `turnId`.

### 1.2 Các nơi sẽ gắn điểm vào (đã đọc)

- **`DashboardAgentRow`** (`components/dashboard/DashboardAgentRow.tsx`, 421 dòng) là component dùng chung; nơi render là `components/sidebar/WorktreeCardAgents.tsx` (hàng trong thẻ worktree ở sidebar; import `DashboardAgentRow` ở dòng 6). Thẻ gọn dùng **component riêng** `CompactAgentRow` trong `components/sidebar/worktree-card-compact-agent-row.tsx` (props `agent`, `now`, `onActivate`, `sendTargetStatus`…, tự dựng markup), nên nút Review phải được thêm ở cả hai nơi. Props gồm `agent`, `onDismiss`, `onActivate(tabId, paneKey)`, `now`, `isUnvisited`, `hideExpand`, `sendTargetStatus`, `onSendTargetClick`… Cụm điều khiển cuối hàng là `DashboardAgentRowTrailingControls.tsx` (khung `h-3.5 w-12`: thời gian tương đối dùng chung ô với nút "X" Dismiss, chevron mở rộng, và nút "Send" tuyệt đối khi ở chế độ chọn đích gửi). `activateAndRevealWorktree` (`lib/worktree-activation.ts`) được `WorktreeCardAgents` dùng để chuyển tới worktree.
- **Source Control**: thanh đầu `SourceControlHeaderToolbar` (`components/right-sidebar/source-control-header-toolbar.tsx`) có lọc tên tệp, nút tạo PR, liên kết review của nhà cung cấp, và `SourceControlHeaderOverflowMenu` (`source-control-header-overflow-menu.tsx`: đổi chế độ cây/danh sách, đổi base ref, làm mới so sánh nhánh, mở rộng ghi chú); hàng ngữ cảnh nhánh `source-control-branch-context-row.tsx` hiển thị thống kê so với base và dùng `SourceControlHeaderIconButton`. `SourceControl.tsx` rất lớn (6 723 dòng khi đọc; `config/max-lines-baseline.txt` có chú thích gọi nó là file "grandfathered"), nên chỉ thêm một hook/prop nhỏ, logic nằm ở file mới.
- **Cmd+K**: `components/WorktreeJumpPalette.tsx` (2 344 dòng) dựng danh mục hành động từ `getCmdJQuickActions()` (`components/cmd-j/quick-actions.ts`: mỗi `CmdJQuickAction {id, kind:'action', title, description, icon, verbKeywords, isAvailable(ctx), run(ctx)}`) với ngữ cảnh `CmdJQuickActionContext` (`quick-action-context.ts`: `activeView`, `activeWorktreeId`, `activeGroupId`, `sshStatus`, `runtimeMode`, các hàm `openNewBrowserTab`, `openCreateWorkspace`…) và lý do không khả dụng `CmdJUnavailableReason` (`'loading'|'no-active-workspace'|'ssh-disconnected'|'no-active-group'`) cùng `getUnavailableQuickActionMessage`. Chú thích trong `quick-actions.ts`: hành động Cmd+J dành cho động từ "tần suất cao, an toàn, ít ngữ cảnh". Có test `quick-action-context.test.ts`.
- **Right sidebar**: `ActiveRightSidebarTab = Exclude<RightSidebarTab,'search'>` ở `shared/types.ts` (hiện `explorer|search|vault|workspaces|pr-checks|source-control|checks|ports`); `store/right-sidebar-route.ts` (`normalizeRightSidebarRoute` liệt kê tab hợp lệ, tab lạ rơi về `explorer`); danh sách mục `ActivityBarItem[]` ở `components/right-sidebar/index.tsx` (mỗi mục `{id, icon, title, shortcut, gitOnly?, folderOnly?, sshOnly?}`); `getVisibleRightSidebarActivityItems` (`right-sidebar-activity-visibility.ts`) lọc theo `isFolder/isFolderWorkspace/isSshRepo`; nội dung panel ở `right-sidebar-panel-content.tsx` (lazy theo `effectiveTab`); `ActivityBarButton` nhận `statusIndicator` (hiện chỉ truyền `checksStatus` cho `checks`, `activity-bar-buttons.tsx` có `STATUS_DOT_COLOR`). Các tab Source Control/Checks/Ports còn được mở bằng phím tắt trong `App.tsx` (`actions.setRightSidebarTab(...)`, ~dòng 1933–1952).

## 2. Giải pháp đề xuất

### 2.1 Mô-đun dùng chung (`components/review-map/entry/`, mới)

| File | Vai trò |
|---|---|
| `agent-turn-completion.ts` | Hàm thuần/selector: `selectAgentTurnCompletions(state, worktreeId)` trả `AgentTurnCompletion[]` từ `agentStatusByPaneKey` + `retainedAgentsByPaneKey` (loại `subagent`, chỉ `state==='done'`, sắp theo `doneAt` giảm dần); `selectLatestCompletion`; `hasUnreviewedCompletion(completions, reviewedTurnId)` |
| `use-agent-turn-completions.ts` | Hook subscribe store với so sánh nông; dùng ở hàng agent, tab right sidebar, Source Control, và `use-review-turn-recorder` của CR-CV-060 |
| `open-review-entry.ts` | `openReviewFromEntryPoint(worktreeId, source, options?)`, `source ∈ 'agent-row' | 'source-control' | 'cmd-k' | 'right-sidebar' | 'notification'`; thứ tự: kiểm tra cờ/khả dụng → `activateAndRevealWorktree(worktreeId)` → nếu đã có tab `review` của worktree thì kích hoạt, nếu chưa thì tạo/kích hoạt bằng `ensureReviewTab(worktreeId, { placement })` (CR-CV-050 2.8, `lib/ensure-review-tab.ts`; đã tự dedupe và trả `null` khi tính năng không `enabled`) → đặt phạm vi mặc định (O7) và bộ lọc/lens theo `options` (`{ lens?: ReviewLensId; completionId?: string }`) |
| `use-code-intel-entry-availability.ts` | `useReviewEntryAvailability(worktreeId): { visible: boolean; reason?: 'not-enabled' \| 'not-git' \| 'no-active-worktree' }`; `visible` chỉ khi `useCodeIntelSupport().state === 'enabled'` (cờ `code_intel_enabled` O8 do backend quảng bá, CR-CV-050 2.4) và worktree thuộc repo git (không phải folder workspace). `unknown` (đang thăm dò) coi như chưa hiển thị để tránh nút chớp |
| `ReviewEntryButton.tsx` | Nút nhỏ dùng lại (icon `ScanSearch` của lucide, có `Tooltip`, `aria-label`) cho hàng agent và Source Control |
| `use-code-intel-review-summary.ts` | Hook tóm tắt cho tab right sidebar (2.5) |

Không có route mới: tab `review` và `ensureReviewTab` do CR-CV-050 định nghĩa. Nếu cờ tắt, **không điểm vào nào hiển thị** và không gọi RPC `codeIntel.*`. Chưa cần quyết định có/không ghi telemetry/feature-interaction (xem 7).

### 2.2 Hàng agent đã xong (`DashboardAgentRow`)

```
  ● Claude  Fix hover scope                      ⌕ Review   2m  ⌄       ← hàng 'done', worktree chưa xem (isUnvisited)
  ● Codex   Refactor repo layer                          2m  ⌄       ← hàng 'done' đã xem: nút chỉ hiện khi hover
  ◐ Claude  (working…)                                         ⌄       ← không có nút
```

- Props mới (tuỳ chọn) cho `DashboardAgentRow`: `onReview?: (agent: DashboardAgentRowData) => void`. Chỉ truyền khi `useReviewEntryAvailability` cho phép; vắng mặt thì hành vi hiện tại giữ nguyên (các test hiện có không đổi).
- Điều kiện hiển thị nút: `agent.state === 'done'` và `agent.rowSource !== 'subagent'` (hàng `retained` được phép: worktree vẫn còn dù pane đã đóng). Hàng `interrupted` vẫn có nút nhưng tooltip nêu "Agent bị ngắt giữa chừng" (thay đổi dở dang vẫn cần review). Hàng không `done` (đang làm, chờ, bị chặn) **không** có nút: kết quả chưa ổn định.
- Hiển thị: khi `isUnvisited` thì nút luôn hiện (vì đây là hành động tiếp theo hợp lý); ngược lại chỉ hiện khi hover/focus-visible (cùng quy tắc `group-hover/agent-row` của nút Dismiss). Chỗ trống trong `DashboardAgentRowTrailingControls` hẹp (`w-12`), nên nút là biểu tượng (`size-3.5`, `ScanSearch`) kèm chữ ở chế độ rộng; chưa chắc vừa ở thẻ gọn (`CompactAgentRow`): **cần kiểm tra giao diện khi triển khai** và có thể phải đổi ô thành `w-auto`. Không chồng lên nút "Send" ở chế độ chọn đích gửi (khi `sendTargetStatus` đặt thì ẩn nút Review, giống cách thời gian/Dismiss bị ẩn).
- Hành vi: `event.stopPropagation()` và `onMouseDown` dừng nổi bọt (như các nút trong `DashboardAgentRowTrailingControls`) để không kích hoạt hàng/thẻ; gọi `openReviewFromEntryPoint(agent.tab.worktreeId, 'agent-row', { completionId })`. Bàn phím: nút là `button` có thể focus; Enter/Space kích hoạt (và `stopKeyDown` như các nút khác).
- Nối ở `WorktreeCardAgents.tsx` (`handleReviewAgent`) cho cả `DashboardAgentRow` lẫn `CompactAgentRow` (cùng prop `onReview` và cùng `ReviewEntryButton`); `DashboardAgentRow` không tự gọi store để giữ nó là component thuần.
- Ngoài phạm vi hàng agent: bảng điều khiển (dashboard) toàn cục dùng cùng component nên có nút khi truyền `onReview`.

### 2.3 Thông báo

MVP **không** đổi thông báo hệ điều hành (muốn thêm nút hành động phải sửa `desktop/src/main/ipc/notifications.ts`, ngoài thư mục frontend và chỉ macOS/Windows hỗ trợ khác nhau; chưa kiểm chứng):

1. Khi người dùng nhấp thông báo "agent xong", luồng hiện có đưa họ vào đúng worktree và focus terminal; từ đó mọi điểm vào khác (hàng agent, chấm trên tab right sidebar) đã sẵn sàng. Đây là điểm vào "thông báo" ở MVP.
2. Nếu CR-CV-050 muốn hành động trực tiếp, đề xuất sau (câu hỏi mở 1): `NotificationDispatchRequest` thêm trường `reviewable?: boolean` và main thêm nút "Review" (macOS `actions`), click gọi `ui:activateWorktree` kèm `openReview: true`; renderer đọc cờ và gọi `openReviewFromEntryPoint(..., 'notification')`. Ở web (`onActivateWorktree` rỗng) cần đường riêng; chưa kiểm chứng.
3. Không dùng toast mặc định: toast chỉ dành cho xác nhận thoáng qua (STYLEGUIDE) và đã có thông báo hệ điều hành. Nếu muốn nhắc trong ứng dụng, dùng chấm trên tab Review (2.5), không ngắt luồng.

### 2.4 Source Control

```
Source Control                                  [🔍] [⎇ Create PR] [⌕] [⋯]
 ↳ feat/x vs main · 12 files · +340 −58 · ↑2                ← hàng ngữ cảnh nhánh
                                                [⌕ Review changes]   (nút biểu tượng cạnh Refresh)
 Overflow ⋯:  View as tree · Change Base Ref… · Refresh branch compare · ─ · ⌕ Review changes · Notes…
```

- File mới `components/right-sidebar/source-control-review-entry.tsx` xuất `useSourceControlReviewEntry({ worktreeId })` → `{ visible, open, hasUnreviewed }`, dùng `useReviewEntryAvailability` và `useAgentTurnCompletions`. `SourceControl.tsx` chỉ gọi hook và truyền `onReviewChanges`/`reviewChangesVisible` xuống `SourceControlHeaderToolbar` (chỉ thêm vài dòng, tránh phình file lớn).
- `SourceControlHeaderOverflowMenu` thêm mục "Review changes" (icon `ScanSearch`, `DropdownMenuItem`), đặt trước cụm "Notes". `source-control-branch-context-row.tsx` thêm một `SourceControlHeaderIconButton` cùng icon khi có thay đổi so với base (`branchSummary.changedFiles > 0`); khi không có thay đổi, nút bị ẩn (không mở Review rỗng).
- Nút mở Review với phạm vi mặc định O7 (merge-base). Nếu người dùng đang lọc tệp ở Source Control thì **không** truyền bộ lọc sang Review (hai khái niệm khác nhau; tránh bất ngờ).
- Việc Review cần backend nên khi `visible=false` mục menu và nút **không render** (không phải bị khoá), để không gây nhầm lẫn ở người dùng chưa bật tính năng.

### 2.5 Cmd+K (`WorktreeJumpPalette`)

Thêm ba `CmdJQuickAction` vào `getCmdJQuickActions` (đều qua `translate()` trong `createLocalizedCatalog`, như các action hiện có):

| id | Tiêu đề | `verbKeywords` (qua `translate`) | Hành vi |
|---|---|---|---|
| `review-changes` | "Review changes" | review changes, review code, review agent, kiểm tra thay đổi | `ctx.openReviewChanges({ lens: 'impact' })` |
| `open-architecture-map` | "Open architecture map" | architecture map, c4, component diagram | `ctx.openReviewChanges({ lens: 'architecture' })` |
| `open-erd` | "Open ERD" | erd, database diagram, entity relationship | `ctx.openReviewChanges({ lens: 'erd' })` |

- `CmdJQuickActionContext` thêm: `codeIntelEnabled: boolean`, `reviewLensAvailable: (lens: ReviewLensId) => boolean`, `openReviewChanges: (options?: { lens?: ReviewLensId }) => void`. `buildCmdJQuickActionContext` nhận thêm tham số tương ứng; `WorktreeJumpPalette.tsx` tạo `openReviewChangesAction` bằng `useCallback` như `openCreateWorkspaceAction` (gọi `openReviewFromEntryPoint(activeWorktreeId, 'cmd-k', …)` rồi `closeModal`).
- `isAvailable`: dùng `getWorkspaceScopedActionAvailability` (đã bao gồm `loading`, `ssh-disconnected`, `no-active-group`) **và** `codeIntelEnabled` **và** (với `architecture`/`erd`) `reviewLensAvailable(lens)` (lens chưa phát hành/không có capability thì action không hiện). Thêm `CmdJUnavailableReason` `'code-intel-disabled'` và nhánh trong `getUnavailableQuickActionMessage` ("Can't review changes — Code review isn't enabled for this workspace."; cập nhật test `quick-action-context.test.ts`). Hành động bị lọc khỏi danh sách khi không khả dụng (cơ chế sẵn có: `actionResults.filter((action) => action.isAvailable(ctx).available)` ở `WorktreeJumpPalette.tsx`), nên thông báo lý do chỉ hiện khi chạy trực tiếp.
- Lưu ý chính sách "curated" của danh mục Cmd+J (`quick-actions.ts`): ba hành động này bổ sung có chủ ý; hai lens (`architecture`, `erd`) là lối tắt tới màn Review, không phải thao tác thiết lập. Nếu bị cho là phá quy tắc, giữ riêng `review-changes` (câu hỏi mở 4).
- Nhảy tới symbol/bảng/component từ Cmd+K (10 §8) nằm ngoài CR này (cần tìm kiếm trên đồ thị, thuộc CR-CV-053+).
- Lệnh không có phím tắt riêng ở MVP, nên không hiển thị chip phím (chỉ hiện chip khi phím đã được cài thật, theo 10 §8).

### 2.6 Tab Review ở right sidebar (tóm tắt)

```
┌ Review ───────────────────────────── [Mở đầy đủ ⤢] ┐
│ feat/x · so với main                                  │
│ Index d819812 · 2 phút trước ✓   (IndexFreshnessChip) │
│ Rủi ro: MEDIUM · chạm 3 luồng cross-community  [vì sao]│
│ 12 file · 38 symbol · 3 luồng · 2 bảng · 1 hợp đồng    │
│ 5 chưa có test            (mỗi chip mở Review + lọc)   │
│ Phát hiện: 2 cao · 3 vừa                               │
│ Tiến độ đọc 4/12 · Ghi chú: 3 chưa gửi                 │
│ Lượt 3 · Claude xong 14:32 · [Review]                  │
└────────────────────────────────────────────────────────┘
```

Thay đổi để thêm tab `review` (đã đọc từng điểm chạm):

1. `shared/types.ts`: thêm `'review'` vào `RightSidebarTab` (và do đó `ActiveRightSidebarTab`).
2. `store/right-sidebar-route.ts`: thêm `tab === 'review'` vào danh sách hợp lệ của `normalizeRightSidebarRoute` (bản cũ lưu 'review' sẽ về `explorer` khi hạ cấp, hợp lý) và cập nhật `right-sidebar-route.test.ts`.
3. `components/right-sidebar/index.tsx`: thêm `ActivityBarItem { id:'review', icon: ScanSearch, title: translate(...,'Review'), shortcut: '', gitOnly: true }`, nhưng chỉ khi `code_intel_enabled`; mở rộng `getVisibleRightSidebarActivityItems` (`right-sidebar-activity-visibility.ts`) bằng cờ `codeIntelOnly?: boolean` trên `ActivityBarItem` và trường `isCodeIntelEnabled` trong trạng thái, thay vì chèn điều kiện rời trong `index.tsx`; cập nhật `right-sidebar-activity-visibility.test.ts`. Không đặt phím tắt ở MVP (App.tsx đang gán phím cho Source Control/Checks/Ports qua registry phím tắt; thêm cho Review cần mục `keybindings`, chưa làm).
4. `right-sidebar-panel-content.tsx`: lazy `ReviewSummaryPanel` (file mới `components/right-sidebar/ReviewSummaryPanel.tsx`) cho `effectiveTab === 'review'`.
5. `ActivityBarButton`: `index.tsx` truyền `statusIndicator` cho `review` bằng một chấm "có lượt agent xong chưa review" (dùng cơ chế chấm đã có của `checks`; màu từ token, kèm `aria-label`/tooltip nêu nghĩa, không chỉ màu). `activity-bar-overflow.ts` cần xử lý chấm của mục bị đẩy vào menu tràn như đã làm cho `checks` (`TopActivityOverflowMenu` có `checksStatus`): dùng chung phần mở rộng thay vì thêm tham số riêng.

`ReviewSummaryPanel` dùng `use-code-intel-review-summary.ts`: đọc cùng mục slice `code-intel` mà thanh tóm tắt của CR-CV-051 dùng (`changeOverlay` + `findings` đếm + `reading progress`), **không** tạo yêu cầu riêng song song; chỉ yêu cầu khi tab hiển thị (`rightSidebarOpen && effectiveTab === 'review'`), làm mới theo push `codeIntel.changed` và khi có `AgentTurnCompletion` mới, không polling. Chip "chưa có test" v.v. gọi `openReviewFromEntryPoint(..., 'right-sidebar', { filter })`. Trạng thái rỗng/lỗi/chưa có index dùng lại thành phần chuẩn của CR-CV-051 (rỗng có hành động trực tiếp, lỗi persistent inline, chip index/stale); đang tải theo ngưỡng 100 ms/1 s/3 s, trì hoãn hiển thị ~200 ms qua SSH, nút khoá ngay.

### 2.7 i18n, token, hai render target, phím tắt

- Chuỗi qua `translate()`, khoá tiền tố `auto.components.review.map.entry.` (ví dụ `ReviewEntryButton.label`, `ReviewEntryButton.interruptedTooltip`, `ReviewSummaryPanel.openFull`, `QuickActions.reviewChanges`, `QuickActions.openErd`), đủ 5 locale `en/es/ja/ko/zh`; chuỗi trong `quick-actions.ts` đặt trong `createLocalizedCatalog` (không gọi `translate()` ở top-level; có test `i18n/no-top-level-translate.test.ts`).
- Chỉ dùng `useCodeIntelSupport`, `useCodeIntelQuery` và lib/store sẵn có nên chạy ở Electron và web (Electron local là `unsupported` tới khi preload desktop có `codeIntel`, CR-CV-050 2.2, nên điểm vào tự ẩn). Không điểm vào nào phụ thuộc IPC riêng của Electron, ngoại trừ nút hành động thông báo (2.3, chưa làm).
- Màu: chỉ biến CSS; chấm "chưa review" dùng token accent/primary (CR-CV-050 thêm token nếu thiếu), kèm nhãn chữ.
- Phím: không thêm phím tắt mới ở MVP; mọi nút có thể focus bằng Tab, Enter/Space kích hoạt; nếu sau này đặt phím thì dùng `metaKey` trên Mac và `ctrlKey` nơi khác và nhãn `⌘`/`Ctrl+` theo nền tảng.
- SSH/độ trễ: mở Review là điều hướng cục bộ, không chờ backend; dữ liệu tải trong màn Review theo CR-CV-051.

## 3. Quyết định thiết kế

- **Một hàm mở duy nhất (`openReviewFromEntryPoint`)** cho mọi điểm vào: dedupe tab, đặt phạm vi mặc định và bộ lọc ở một chỗ.
- **Sự kiện "agent xong" lấy từ `agent-status` + retained, không từ thông báo**: thông báo có thể bị tắt/chặn/bỏ qua theo cài đặt và không có ở web; trạng thái hook là nguồn có thẩm quyền.
- **Nút ở hàng agent chỉ cho `done`**: tránh review kết quả đang thay đổi.
- **Không render điểm vào khi cờ tắt** (không khoá): tránh gợi ý tính năng chưa có.
- **Không toast mặc định** và **không đổi thông báo hệ điều hành ở MVP**: giảm nhiễu và tránh đụng desktop main.
- **Tóm tắt right sidebar dùng chung dữ liệu với thanh tóm tắt Review**: một nguồn, không thêm tải lên backend/agent (mỗi lần gọi CLI ≈ 1,8 s, README mục 1).
- **`SourceControl.tsx` chỉ thêm một hook và prop**, logic ở file mới (tôn trọng baseline max-lines, không thêm `max-lines` disable).

## 4. Tiêu chí chấp nhận

- [ ] Khi `useCodeIntelSupport` không phải `enabled` (cờ `code_intel_enabled` tắt, hoặc Electron local chưa có preload `codeIntel`): không có nút Review ở hàng agent, Source Control, Cmd+K, tab right sidebar; không phát sinh yêu cầu `codeIntel.*`.
- [ ] Hàng agent `done` (live và retained, không phải subagent) có nút Review; hàng `working/blocked/waiting/idle` không có; nút luôn hiện khi worktree chưa xem và hiện khi hover/focus ở trường hợp còn lại; bấm không kích hoạt hàng/thẻ.
- [ ] Bấm nút Review mở (hoặc kích hoạt tab đã có) tab `review` của đúng worktree trong một thao tác, lens Ảnh hưởng, phạm vi mặc định; không tạo tab trùng.
- [ ] `DashboardAgentRow` không đổi hành vi khi không truyền `onReview` (các test hiện có vẫn xanh); nút Review bị ẩn khi `sendTargetStatus` đặt.
- [ ] Source Control: mục "Review changes" trong menu tràn và nút biểu tượng ở hàng ngữ cảnh nhánh (chỉ khi có thay đổi so với base) mở Review đúng phạm vi O7; không phình `SourceControl.tsx` quá vài dòng.
- [ ] Cmd+K liệt kê "Review changes", "Open architecture map", "Open ERD" khi khả dụng, ẩn khi cờ tắt/lens không có/SSH ngắt; chạy được bằng bàn phím; thông báo lý do đúng cho `code-intel-disabled`.
- [ ] Tab Review ở right sidebar chỉ hiện khi cờ bật và worktree git; `normalizeRightSidebarRoute` chấp nhận `review`, các tab cũ không đổi; panel hiển thị tóm tắt gọn (chip index, rủi ro, số file/symbol/luồng/bảng/hợp đồng/chưa test, phát hiện, tiến độ, lượt agent xong gần nhất) và nút "Mở đầy đủ".
- [ ] Chấm trên tab Review hiện khi có lượt `done` chưa review, có nhãn chữ/tooltip; biến mất sau khi mở Review cho lượt đó.
- [ ] Nhấp thông báo hệ điều hành "agent xong" vẫn hoạt động như cũ (không hồi quy) và dẫn tới worktree nơi các điểm vào khác sẵn sàng.
- [ ] Rỗng/lỗi/đang tải của panel tóm tắt đúng chuẩn khung Review; không có yêu cầu trùng với thanh tóm tắt; không polling.
- [ ] Chuỗi qua `translate()` đủ 5 locale; không hex cứng; hoạt động ở Electron và web; không dùng `components/code-review/*`; không thêm thư viện; không thêm `max-lines` disable.

## 5. Kiểm thử

Vitest + Testing Library theo mẫu repo:

- Hàm thuần: `agent-turn-completion` (live `done`, retained `done`, bỏ `subagent`, thứ tự theo `doneAt`, `interrupted`, danh tính `paneKey:doneAt`), `normalizeRightSidebarRoute('review')` và tab lạ (mở rộng `right-sidebar-route.test.ts`), `getVisibleRightSidebarActivityItems` với `codeIntelOnly` (mở rộng `right-sidebar-activity-visibility.test.ts`), `isAvailable` của ba action Cmd+K (mở rộng `cmd-j/quick-action-context.test.ts`).
- Component: `DashboardAgentRow` với `onReview` (mẫu `renderToStaticMarkup` như `DashboardAgentRow.test.tsx`: có nút cho `done`, không cho `working`, không cho `subagent`, ẩn khi `sendTargetStatus`, nhãn interrupted), test tương tác nút dừng nổi bọt (`WorktreeCardAgents.activation.test.tsx` hiện có là mẫu), `ReviewSummaryPanel` (rỗng, lỗi, chip → mở kèm bộ lọc), `ActivityBarButton` chấm review.
- Hook: `openReviewFromEntryPoint` dedupe tab, gọi `activateAndRevealWorktree` trước `ensureReviewTab`, bỏ qua khi cờ tắt; `use-code-intel-review-summary` chỉ gọi khi tab hiển thị, không polling.
- Hồi quy: các test hiện có của `WorktreeCardAgents*`, `right-sidebar-*`, `cmd-j/*`, `source-control-*` không đổi.
- E2E (cần backend CR-CV-036/040 và agent có hook): agent chạy xong → hàng agent hiện Review → một cú bấm vào lens Ảnh hưởng. **Chưa chạy**, đây là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chỗ trống ở cuối hàng agent** (`w-12`) có thể không đủ cho nút thứ ba; chưa kiểm tra bằng mắt ở thẻ gọn/thẻ thường; có thể cần đổi bố cục ô cuối.
- **Agent không có hook** không có hàng agent nên không có nút Review ở đó; các điểm vào khác bù lại. Tỷ lệ agent có hook chưa thống kê.
- **`done` do hook có thể báo sớm hoặc lặp** (một số agent báo `done` nhiều lần); khử trùng lặp theo `paneKey:doneAt`; độ tin cậy từng loại agent chưa kiểm chứng (liên quan chính sách `agent-task-complete-policy.ts` đã tồn tại để xử lý các trường hợp này, chưa đọc kỹ).
- **Thông báo web**: `onActivateWorktree` rỗng ở `web-preload-api`; đường nhấp thông báo trên web chưa kiểm chứng; nút hành động thông báo chưa làm.
- **Phụ thuộc `ensureReviewTab`/`WorkspaceVisibleTabType`** của CR-CV-050: nếu tab `review` chưa có thì không điểm vào nào hoạt động; thứ tự triển khai CR-050 → 051 → 061 (README mục 5).
- **Tab right sidebar thêm vào `RightSidebarTab`** lan sang các nơi dùng union này (`activity-bar-overflow.ts`, `right-sidebar-effective-tab.ts`, lưu `rightSidebarTabByWorktree` trong phiên); cần chạy `gitnexus_impact` trên `normalizeRightSidebarRoute`, `ActivityBarItem`, `DashboardAgentRow` và `quick-actions` theo quy tắc repo trước khi sửa, và quét các `switch` đầy đủ theo union.
- **Chi phí tóm tắt**: `changeOverlay` gọi agent qua CLI (~1,8 s mỗi lần, README mục 1) nên tóm tắt tab phụ thuộc cache backend (CR-CV-022); chưa đo.
- Cmd+J vốn "curated"; thêm ba action có thể bị phản đối (xem 2.5).

## 7. Câu hỏi mở

1. Có thêm nút hành động "Review" vào thông báo hệ điều hành (sửa `desktop/src/main/ipc/notifications.ts`) không, và trên web làm sao?
2. Có ghi telemetry/feature-interaction cho lượt mở Review theo điểm vào không? Cần thêm khoá vào danh sách hiện có (xem `feature-interaction-writer-boundaries.test.ts`) và schema `shared/telemetry-events.ts`.
3. Phím tắt cho tab Review (cần mục trong `keybindings`) có cần ở MVP không?
4. Giữ cả ba action Cmd+K hay chỉ `review-changes`?
5. Hàng agent `interrupted` có nút Review hay chỉ tooltip? (đề xuất: có nút.)
6. Nếu worktree không có binding dev server (CR-CV-012), điểm vào vẫn hiển thị (và Review báo thiếu binding) hay ẩn? Đề xuất: hiển thị khi cờ bật, để người dùng thấy lý do.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (D7, O7, O8, mục 5 thứ tự thực thi)
- `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§3 điểm vào, §8)
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- `/opt/repos/orca/frontend/src/renderer/src/components/dashboard/DashboardAgentRow.tsx`, `DashboardAgentRowTrailingControls.tsx`, `DashboardAgentRow.test.tsx`, `useDashboardData.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/sidebar/WorktreeCardAgents.tsx`, `worktree-card-compact-agent-row.tsx`, `WorktreeCardAgents.activation.test.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/agent-status.ts`, `shared/agent-status-types.ts`
- `/opt/repos/orca/frontend/src/renderer/src/hooks/agent-hook-completion-notifications.ts`, `components/terminal-pane/use-notification-dispatch.ts`, `agent-completion-coordinator.ts`, `agent-task-complete-policy.ts`
- `/opt/repos/orca/desktop/src/main/ipc/notifications.ts` (xử lý click thông báo)
- `/opt/repos/orca/frontend/src/renderer/src/web/web-preload-api.ts` (`onActivateWorktree` rỗng)
- `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`, `source-control-header-toolbar.tsx`, `source-control-header-overflow-menu.tsx`, `source-control-branch-context-row.tsx`, `source-control-header-icon-button.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/WorktreeJumpPalette.tsx`, `components/cmd-j/quick-actions.ts`, `quick-action-context.ts`, `quick-action-context.test.ts`
- `/opt/repos/orca/frontend/src/renderer/src/store/right-sidebar-route.ts`, `right-sidebar-route.test.ts`, `components/right-sidebar/index.tsx`, `activity-bar-buttons.tsx`, `right-sidebar-panel-content.tsx`, `right-sidebar-activity-visibility.ts`, `right-sidebar-effective-tab.ts`
- `/opt/repos/orca/frontend/src/shared/types.ts` (`RightSidebarTab`, `ActiveRightSidebarTab`, `WorkspaceVisibleTabType`, `NotificationDispatchRequest`)
- `/opt/repos/orca/frontend/src/renderer/src/lib/worktree-activation.ts` (`activateAndRevealWorktree`)
- Mới: `components/review-map/entry/{agent-turn-completion,use-agent-turn-completions,open-review-entry,use-code-intel-entry-availability,use-code-intel-review-summary}.ts(x)`, `ReviewEntryButton.tsx`; `components/right-sidebar/ReviewSummaryPanel.tsx`, `source-control-review-entry.tsx`
