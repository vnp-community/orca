# FE-CV-SOL-050-review-tab-wiring: Loại tab `review`, `ensureReviewTab`, vị trí tab group, token `--review-*`, i18n

> ✅ **Done.** Trạng thái (cập nhật 2026-10-08): 6/6 task DONE. Chi tiết ở dòng `**Status:**` và "Ghi chú hoàn thiện" của từng task.

**CR:** [CR-CV-050](../../../../../../docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md) (phần 2.8, 2.9, 2.10 và quyết định tab ở README feature)
**Area:** frontend (`frontend/src/shared`, `frontend/src/renderer/src/{store,components/tab-group,components/tab-bar,lib,hooks,i18n,assets}`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U7 cờ tắt, §6 cờ, §7 môi trường; tab không có kênh riêng), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (§8.1 quy ước, §10 dòng "18+ chỗ tab `review`", PQ-04 cho `entityId`).
**TDD tham chiếu:** [v5/05-ui-components §4.2 Tab Bar, §4.3 Tab Group Layout](../../../../tdd/v5/05-ui-components.md), [v5/02-state-management §5.4 Tabs](../../../../tdd/v5/02-state-management.md), [v5/01-architecture-overview §6 Styling, §7 i18n](../../../../tdd/v5/01-architecture-overview.md), [v5/06-web-client §10 web-workspace-session](../../../../tdd/v5/06-web-client.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `shared/types.ts` (:797-808), `shared/workspace-session-schema.ts` (:94-106), `store/slices/tabs.ts` (:412, :1905-1925), `store/slices/worktrees.ts` (:301), `store/selectors.ts` (:86), `components/tab-group/TabGroupPanel.tsx` (:130-150, :345-376, :405-425), `lib/ensure-simulator-tab.ts` (:1-60), `lib/emulator-right-split-target.ts` (tên), `assets/main.css` (:43 `@theme inline`, :126 `:root`, :154 `--ai-action-accent: var(--color-violet-500)`, :216 `.dark`, :206 `--annotation-highlight`), `node_modules/tailwindcss/theme.css` (có `--color-amber-*`), `lucide-react@0.577.0` (có `scan-search`, `git-pull-request-arrow`, `circle-dot`, `triangle-alert`, `waypoints`, `loader-circle`, `file-search`), `i18n/{no-top-level-translate.test.ts,task-jira-link-locale-coverage.test.ts,locales/*.json}` (tên).

**Danh sách đã đếm bằng `grep "'simulator'"` (không test), 30 file** (CR-050 nói "≥ 18"; số này thắng):
`components/tab-bar/TabBar.tsx` (16), `floating-terminal/FloatingTerminalPanel.tsx` (10), `lib/ensure-simulator-tab.ts` (8), `tab-group/TabGroupPanel.tsx` (5), `tab-bar/group-tab-order.ts` (5), `hooks/resolve-zoom-target.ts` (4), `hooks/modal-return-focus-action.ts` (4), `components/Terminal.tsx` (4), `tab-group/useTabGroupWorkspaceModel.ts` (4), `lib/simulator-palette-search.ts` (3), `components/WorktreeJumpPalette.tsx` (3), `tab-group/tab-drag-preview-activation.ts` (3), `shared/workspace-session-schema.ts` (2), `shared/types.ts` (2), `store/slices/tabs.ts` (2), `lib/tab-number-shortcuts.ts` (2), `hooks/ipc-tab-switch.ts` (2), `terminal/tab-type-cycle.ts` (2), `shared/keybindings.ts` (1, chỉ từ khoá tìm kiếm), `store/slices/worktrees.ts` (1), `store/selectors.ts` (1), `lib/workspace-tab-palette-search.ts` (1), `lib/simulator-tab-shutdown.ts` (1), `lib/file-type-icons.ts` (1, khớp tên, không liên quan), `hooks/useModalReturnFocus.ts` (1), `tab-group/useTabDragSplit.ts` (1), `tab-bar/tab-create-menu-options.ts` (1, nhãn menu), `settings/mobile-emulator-search.ts` (1), `emulator-pane/use-mobile-emulator-tab-intro-actions.ts` (1), `emulator-pane/EmulatorPaneOverlayLayer.tsx` (1). Tab Review chỉ cần chạm tập con; danh sách ở 4.2 chỉ rõ cái nào **cần** và cái nào **không**. Chưa kiểm chứng đầy đủ: bắt buộc chạy `tsc` và `pnpm lint:switch-exhaustiveness` (root `package.json`, chưa chạy được).

Xác nhận:

- `TabContentType` có 7 giá trị, `WorkspaceVisibleTabType` có 4 (`shared/types.ts:799-808`); hai `z.enum` tương ứng ở `workspace-session-schema.ts:96-104`.
- `isRenderableTab` (`tabs.ts:1909-1920`): terminal/browser/simulator có nhánh riêng; mọi loại còn lại phải có `openFiles` khớp `entityId` (`liveEditorIds`), nếu không bị loại khi hydrate ⇒ tab `review` **phải có nhánh `true`** như `simulator`.
- `TabGroupPanel.tsx:354-358`: mọi tab không phải terminal/browser/simulator được vẽ bằng `EditorPanel` (Suspense, "Loading editor..."); `simulator` được vẽ ở lớp worktree (`EmulatorPaneOverlayLayer`), không trong thân tab. Tab `review` phải **vừa bị loại khỏi nhánh `EditorPanel` vừa có nhánh vẽ riêng**.
- `toVisibleTabType` có hai bản sao: `tabs.ts:412` và `worktrees.ts:301` (cùng logic `browser|terminal|simulator`, còn lại `editor`): cả hai phải thêm `review`. (CR-050 ghi :411 và :300; số đúng là :412 và :301.)
- `ensureSimulatorTab` (mẫu): một tab mỗi worktree, `getSimulatorTabForWorktree`, `placement: 'activeGroup' | 'rightSplit'`, `surfacePane`, `findReusableRightSplitGroupId`; trả `string | null` khi cờ tắt (`mobileEmulatorEnabled === false`).
- Không có token `--review-*` trong `main.css`; không có `--warning`.

**Correction relative to CR-050 (mục 2.8, 2.10):**

| # | CR-050 ghi | Thực tế / hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Chạm "≥ 18" file | 30 file có `'simulator'` (bảng trên) | Danh sách task 050-17/050-18 liệt kê từng file với "cần/không cần" |
| 2 | `ensureReviewTab` trả `null` khi `useCodeIntelSupport` ≠ `enabled` | Cờ nằm ở `settings.get` (UI-API §6), đọc đồng bộ từ `codeIntelSupportByEnvironment` (SOL-050-store-and-query-hooks); `state:'unknown'` chưa biết | `unknown` ⇒ **cho mở tab** (hiển thị trạng thái tải trong tab, hook sẽ gọi `settings.get`); `disabled`/`unsupported` ⇒ `null` |
| 3 | Tab khi cờ tắt "hiển thị thông báo" | UI-API U7: cờ tắt ⇒ ẩn tính năng, không toast; tab đã lưu trong phiên vẫn phải vẽ được | `ReviewTabUnavailableNotice` (inline, có "Đóng tab"), không gọi kênh nào |
| 4 | Token `--review-untested` mô tả amber-600/400 | README v7 §8 mục 28: ở chế độ sáng chỉ ~3,19:1 trên `card` (tính tay), cần test | Task 050-18 thêm kiểm tra tương phản (script thuần, giá trị oklch Tailwind) và ghi kết quả; không đổi màu khi chưa đo |
| 5 | Icon "`GitPullRequestArrow` hoặc `ScanSearch`" | Cả hai tồn tại trong `lucide-react@0.577.0`; `GitPullRequestArrow` gợi ý PR (GitHub) trái AGENTS.md "Git Provider Compatibility" | Dùng `ScanSearch` |
| 6 | Không nêu phím | Chuyển tab số/vòng (`tab-number-shortcuts.ts`, `tab-type-cycle.ts`) | Tab Review tham gia vòng chuyển tab số theo vị trí; **không** thêm hành động vào `KEYBINDING_DEFINITIONS` (README feature) |

## 2. Hợp đồng áp dụng

Tab không có kênh riêng. Áp dụng: U7 (cờ tắt ⇒ ẩn, không toast), §6 (không mở `subscribe` khi tắt), PQ-04 (`Tab.entityId = worktreeId`, dạng `<repoId>::<path>`), §8.1 (đặt tên file theo khái niệm; không `max-lines` disable; UI theo `guides/STYLEGUIDE.md`).

## 3. Lệch giữa CR và hợp đồng

Bảng mục 1. Không có xung đột kênh; điểm quan trọng nhất là số file (30) và nhánh `unknown` của cờ.

## 4. Giải pháp

### 4.1 Loại tab và vòng đời

`'review'` thêm vào `TabContentType` và `WorkspaceVisibleTabType`, theo mẫu `simulator`: không bản ghi nền trong `openFiles`, **một tab mỗi worktree**, `Tab.entityId = worktreeId`. Không dùng chế độ ảo của editor (như `check-details`): `editor.ts` đã quá lớn và `OpenFile` ép trường file không hợp. Tab Review được lưu vỏ (id, vị trí, nhóm) trong phiên, trạng thái bên trong (phạm vi, lens) **không** lưu (SOL-051). Phiên lưu nhưng bản cũ gặp `contentType:'review'` có thể hỏng cả lần phân tích vì `z.enum` (rủi ro 9).

### 4.2 Điểm chạm (đã đọc; đánh dấu CẦN / KHÔNG)

| Vị trí | Việc | Task |
|---|---|---|
| `shared/types.ts:799-808` | thêm `'review'` vào hai union | 050-15 |
| `shared/workspace-session-schema.ts:96-104` | thêm vào hai `z.enum` | 050-15 |
| `store/slices/tabs.ts:412`, `store/slices/worktrees.ts:301` (`toVisibleTabType`) | trả `'review'` | 050-15 |
| `store/slices/tabs.ts:1909-1920` (`isRenderableTab`) | `review` ⇒ `true` | 050-15 |
| `store/selectors.ts:86` | đếm tab Review như `simulator` (CẦN, đọc ngữ cảnh trước) | 050-15 |
| `lib/ensure-review-tab.ts` (mới) | tạo/kích hoạt/trả `tabId` hoặc `null` | 050-16 |
| `components/tab-group/TabGroupPanel.tsx:130-150, :354-358` | loại `review` khỏi nhánh `EditorPanel`; vẽ `ReviewTabHost` (lazy, Suspense) | 050-17 |
| `components/tab-group/useTabGroupWorkspaceModel.ts` (4 chỗ) | mục tab, kích hoạt, `setActiveTabType('review')` | 050-17 |
| `components/tab-group/useTabDragSplit.ts`, `tab-drag-preview-activation.ts` | kiểu `tabType`, kích hoạt khi kéo | 050-17 |
| `components/tab-bar/TabBar.tsx` (16), `group-tab-order.ts` (5), `reconcile-order.ts` (chưa đọc, CR nêu) | mục `review`, icon `ScanSearch`, thứ tự, trạng thái chọn | 050-17 |
| `lib/tab-number-shortcuts.ts`, `components/terminal/tab-type-cycle.ts`, `components/Terminal.tsx` (4), `hooks/ipc-tab-switch.ts` | chuyển tab số/vòng; bỏ qua lớp phủ terminal | 050-18 |
| `hooks/resolve-zoom-target.ts`, `hooks/modal-return-focus-action.ts`, `hooks/useModalReturnFocus.ts` | kiểu `activeTabType`; `review` ⇒ zoom `'ui'`, trả tiêu điểm vào tab | 050-18 |
| `lib/workspace-tab-palette-search.ts`, `components/WorktreeJumpPalette.tsx` | tìm tab Review (nhãn "Review"), kiểu `activeTabType` | 050-18 |
| `components/floating-terminal/FloatingTerminalPanel.tsx` (10) | tab Review **không** nằm trong nhóm tab của terminal nổi; chỉ xác nhận không vỡ (CẦN đọc) | 050-18 |
| `runtime/sync-runtime-graph.ts` (`isEditorSurfaceTab`, CR nêu ~:866, **chưa đọc**) | **KHÔNG** đồng bộ tab Review sang mobile (chỉ `editor`/`diff`) | 050-18 |
| `shared/keybindings.ts`, `lib/file-type-icons.ts`, `settings/mobile-emulator-search.ts`, `emulator-pane/*`, `lib/simulator-*`, `ensure-simulator-tab.ts` | **KHÔNG** (chỉ liên quan simulator) | — |

### 4.3 `ensureReviewTab`

```ts
type EnsureReviewTabOptions = { targetGroupId?: string; placement?: 'activeGroup' | 'rightSplit'; surfacePane?: boolean }
getReviewTabForWorktree(worktreeId): { id: string; groupId: string; contentType: string } | null
ensureReviewTab(worktreeId: string, options?: EnsureReviewTabOptions): string | null
// null khi: không có nhóm đích; hoặc support = 'disabled' | 'unsupported' (không phải 'unknown'); hoặc selector 'unsupported'
```

Hành vi: trùng tab thì `activateTab` + `focusGroup` + `setActiveTabType('review')` (như `ensureSimulatorTab` :43-60); `rightSplit` tái dùng nhóm phải bằng `findReusableRightSplitGroupId`. **Không** đăng ký phím toàn cục; điểm vào do SOL-061 (nhóm B).

### 4.4 `ReviewTabHost` (seam với SOL-051)

`components/review-map/ReviewTabHost.tsx` (mới): `({worktreeId, tabId}) => JSX`; bản 050 render `ReviewTabUnavailableNotice` khi cờ tắt/không hỗ trợ và `ReviewTabLoading` (Skeleton) khi `unknown`; khi `enabled` render thân tạm "Review đang được dựng" cho tới khi SOL-051 đổi sang `ReviewWorkspace` (lazy). Mục đích: wiring tab chạy và kiểm thử được độc lập backend.

### 4.5 Token `--review-*` (sửa `assets/main.css`)

Thêm sau `--annotation-highlight` (:206 và bản `.dark`) và bind trong `@theme inline` (cạnh `--color-status-success`, :101-103), tham chiếu biến Tailwind (tiền lệ `--ai-action-accent: var(--color-violet-500)`, :154; **không hex**):

| Token | Sáng / tối |
|---|---|
| `--review-changed` | `var(--color-blue-600)` / `var(--color-blue-400)` |
| `--review-affected` | `var(--color-teal-600)` / `var(--color-teal-400)` |
| `--review-untested` | `var(--color-amber-600)` / `var(--color-amber-400)` |
| `--review-violation` | `var(--destructive)` |
| `--review-area-1..6` | `sky, emerald, amber, rose, indigo, stone` mức 500 / 400 |

Màu chỉ biểu thị trạng thái; mọi lớp phủ kèm dấu hình học (SOL-053). Độ tương phản chưa đo (xem Correction 4).

### 4.6 i18n

Khoá `auto.components.reviewMap.<Thành phần>.<tên>` và `auto.hooks.codeIntel.<tên>`, đủ `en, es, ja, ko, zh`; test `i18n/code-intel-locale-coverage.test.ts` (mẫu `task-jira-link-locale-coverage.test.ts`): mỗi khoá chuỗi không rỗng ở 5 locale; 4 locale không phải `en` không trùng văn bản `en`. Mảng `KEYS` mở rộng dần bởi mỗi solution. Không gọi `translate()` ở cấp module (`no-top-level-translate.test.ts`). Copy không overclaim (STYLEGUIDE "UI copy must not overclaim").

## 5. Quyết định thiết kế

- Mẫu `simulator` (không bản ghi nền) thay vì ảo hoá trong editor.
- Cờ `unknown` vẫn cho mở tab để không "mất" tab sau khi tải trang trước khi `settings.get` về; `disabled` thì thông báo inline.
- `ScanSearch`, không `GitPullRequestArrow` (provider-neutral).
- Tab Review không đồng bộ sang mobile.
- Token qua `var(--color-*)`, không hex.

## 6. Phụ thuộc chéo khu vực

Không có phụ thuộc backend hay agent: toàn bộ solution làm được với fake backend (G4) và không cần kênh thật (G3). Chỉ cần SOL-050-store-and-query-hooks (cờ `codeIntelSupportByEnvironment`) cho task 050-16. Task 050-18 chạm `runtime/sync-runtime-graph.ts` (đọc kỹ trước). `desktop/`: nếu Electron dùng `desktop/src/renderer` riêng (SOL-050-types-and-runtime-bridge mục 1) thì mọi thay đổi ở solution này **không tự có mặt ở Electron** (ghi vào câu hỏi mở 1).

## 7. Tiêu chí chấp nhận

- [ ] `'review'` có trong hai union và hai `z.enum`; `tsc` và `lint:switch-exhaustiveness` không báo `switch` thiếu.
- [ ] `ensureReviewTab` không tạo trùng; `disabled`/`unsupported` ⇒ `null`, `unknown` ⇒ mở được.
- [ ] Tab Review hydrate lại không bị loại; phiên có tab Review khi cờ tắt: tab hiện thông báo inline, không ném, không toast, không gọi kênh.
- [ ] `TabGroupPanel` không vẽ `EditorPanel` cho tab `review`; `TabBar`, kéo-thả, chia nhóm, chuyển tab số hoạt động với tab Review.
- [ ] Token `--review-*` có ở `:root`, `.dark`, `@theme inline`; không hex mới trong TS/TSX.
- [ ] Khoá i18n đủ 5 locale; `code-intel-locale-coverage.test.ts` xanh; `no-top-level-translate.test.ts` xanh.
- [ ] Không `max-lines` disable mới; tab Review không xuất hiện trong đồ thị đồng bộ mobile.

## 8. Kiểm thử

`lib/ensure-review-tab.test.ts` (mẫu `ensure-simulator-tab.test.ts`, `ensure-simulator-tab-behavior.test.ts`), `store/slices/tabs.review.test.ts`, `shared/workspace-session-schema.test.ts` mở rộng (hydrate `review`), `components/tab-group/TabGroupPanel` và `components/tab-bar/*` test hiện có mở rộng, `components/review-map/ReviewTabHost.test.tsx`, `i18n/code-intel-locale-coverage.test.ts`, `assets/review-tokens.test.ts` (quét `main.css`: có đủ token ở ba nơi, không hex trong giá trị `--review-*`). E2E (khi kênh thật chạy): `tests/e2e/review-tab.spec.ts` (mới) mở/đóng tab, xoá worktree khi tab mở. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

- 30 file chạm; `switch` không đầy đủ có thể lọt nếu bỏ qua `lint:switch-exhaustiveness` (chưa kiểm chứng lệnh chạy được).
- Bản app cũ đọc phiên có `contentType:'review'` có thể hỏng cả phiên (`z.enum`); chưa kiểm chứng `catch` ở cấp trên của schema.
- `reconcile-order.ts`, `sync-runtime-graph.ts`, `FloatingTerminalPanel.tsx` chưa đọc kỹ.
- Độ tương phản `--review-untested` (sáng) ~3,19:1 theo README v7; chưa test.
- `Terminal.tsx` (:2020-2128 theo CR) có lớp phủ liên quan tab type: chưa đọc.

## 10. Câu hỏi mở

1. Electron: `desktop/src/renderer` có là build thật của app đóng gói không? Nếu có, tab Review và toàn bộ Review chỉ chạy ở web cho tới khi đồng bộ renderer.
2. Có cần giới hạn số tab Review đồng thời (nhiều worktree mở)? Đề xuất không.
3. Tab Review đóng bằng nút `x` có cần xác nhận khi có ghi chú chưa gửi (SOL-060, nhóm B)? Để nhóm B quyết định, chỗ cắm là `onBeforeClose` của `ReviewTabHost`.

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-050-15](../tasks/FE-CV-TASK-050-15-tab-type-union-and-session-schema.md) | Union `review`, schema phiên, `toVisibleTabType`, `isRenderableTab`, selectors | P0 |
| [FE-CV-TASK-050-16](../tasks/FE-CV-TASK-050-16-ensure-review-tab.md) | `ensureReviewTab` | P0 |
| [FE-CV-TASK-050-17](../tasks/FE-CV-TASK-050-17-tab-group-panel-and-tab-bar-wiring.md) | `TabGroupPanel`, `TabBar`, kéo-thả, `ReviewTabHost` | P0 |
| [FE-CV-TASK-050-18](../tasks/FE-CV-TASK-050-18-tab-shortcuts-focus-palette-wiring.md) | Chuyển tab, tiêu điểm, palette, zoom, loại khỏi sync mobile | P1 |
| [FE-CV-TASK-050-19](../tasks/FE-CV-TASK-050-19-review-tokens-in-main-css.md) | Token `--review-*` và kiểm tương phản | P0 |
| [FE-CV-TASK-050-20](../tasks/FE-CV-TASK-050-20-i18n-keys-and-locale-coverage-test.md) | i18n và test phủ khoá | P0 |

Thứ tự: 050-15 → 050-16 → 050-17 → 050-18; 050-19 và 050-20 độc lập, làm song song bất kỳ lúc nào.

## 12. Tham chiếu

`/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/frontend/src/shared/types.ts`, `/opt/repos/orca/frontend/src/shared/workspace-session-schema.ts`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/{tabs,worktrees}.ts`, `/opt/repos/orca/frontend/src/renderer/src/store/selectors.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/tab-group/TabGroupPanel.tsx`, `/opt/repos/orca/frontend/src/renderer/src/lib/ensure-simulator-tab.ts`, `/opt/repos/orca/frontend/src/renderer/src/assets/main.css`.
