# FE-CV-SOL-061: Điểm vào Review (hàng agent, Source Control, Cmd+K, tab right sidebar)

> **Status:** [x] DONE (verified 2026-10-08)

**CR:** [CR-CV-061](../../../../../../docs/crs/v7/review-frontend/CR-CV-061-review-entry-points.md)
**Area:** frontend (`components/review-map/entry/`, dashboard, sidebar, right-sidebar, cmd-j, store route)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (U7 `CODEINTEL_DISABLED` ⇒ ẩn; §3.1 `changeOverlay` (`detail:'summary'`), `findings`, `reviewState.get`; §4.3 `ChangeOverlay` (`risk`, `limits.totalCounts`, `components`, `indexFreshness`); §6 cờ = `settings.get`; §5 push), [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (**PQ-01, PQ-04, PQ-12, PQ-13, PQ-22, PQ-35**; §7 (đợt 3); §8.2; §9 O-1). **TDD:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md).

## 1. Trạng thái hiện tại (re-verify)

**Đã xác minh trong phiên này (tồn tại):** `components/dashboard/DashboardAgentRow.tsx`, `DashboardAgentRowTrailingControls.tsx`; `components/sidebar/WorktreeCardAgents.tsx`, `worktree-card-compact-agent-row.tsx`; `components/right-sidebar/{index.tsx, activity-bar-buttons.tsx, right-sidebar-panel-content.tsx, right-sidebar-activity-visibility.ts, source-control-header-toolbar.tsx, source-control-header-overflow-menu.tsx, source-control-branch-context-row.tsx}`; `components/cmd-j/{quick-actions.ts, quick-action-context.ts}` (`CmdJUnavailableReason` = `'loading'|'no-active-workspace'|'ssh-disconnected'|'no-active-group'`, dòng 6–10); `components/WorktreeJumpPalette.tsx`; `store/right-sidebar-route.ts`; `shared/types.ts:3337` `RightSidebarTab`, `:3346` `ActiveRightSidebarTab = Exclude<RightSidebarTab,'search'>`; `lib/worktree-activation.ts`; `store/slices/agent-status.ts` (`agentStatusByPaneKey`:101, `retainedAgentsByPaneKey`:115); `shared/agent-status-types.ts` (`stateStartedAt`:112). Không có `components/review-map/entry/`.
**Theo CR-CV-061 (chưa kiểm lại từng dòng):** kích thước `DashboardAgentRow` 421 dòng, `SourceControl.tsx` ~6 700 dòng (baseline max-lines), `web-preload-api` `onActivateWorktree` rỗng, nhấp thông báo OS gọi `ui:activateWorktree`.

### Lệch giữa CR và hợp đồng (hợp đồng thắng)

| # | CR-061 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | "Cờ `code_intel_enabled` do backend quảng bá (capability)" | Nguồn chuẩn là `settings.get` (`effective.codeIntelEnabled`) §6; capability tuỳ chọn | `useReviewEntryAvailability` dựa `useCodeIntelSupport` của FE-CV-SOL-050-store-and-query-hooks (đã đọc settings) |
| 2 | Tóm tắt tab sidebar dùng chung dữ liệu `changeOverlay` đầy đủ với thanh tóm tắt | `changeOverlay.detail:'summary'` chỉ trả `scope`, `limits.totalCounts`, `risk`, `components`, `indexFreshness` (mảng chi tiết rỗng) | Panel dùng `detail:'summary'` nếu cache `full` của khung không có; nếu có thì dùng cache (một nguồn) |
| 3 | Đếm "chưa có test", "bảng", "hợp đồng" | Khoá `totalCounts: Record<string, number>` **chưa được chốt tên** | Đọc theo khoá đề xuất (`files, symbols, flows, tables, contracts, uncovered`); thiếu khoá ⇒ ẩn chip đó (câu hỏi mở 2) |
| 4 | "Phát hiện: 2 cao · 3 vừa" | `Finding.severity ∈ error|warning|info` (PQ-06) | Hiển thị `error`/`warning`/`info`; đếm từ trang đầu `findings` (`limit` nhỏ); có `nextPageToken` ⇒ "N+" |
| 5 | `severity` cao/vừa ở tóm tắt mobile | PQ-36 dùng `error|warning|info` | Không liên quan ở solution này; ghi để nhất quán với 062 |
| 6 | Khoá i18n `auto.components.review.map.entry.` | README nhóm `auto.components.reviewMap.<Thành phần>.<tên>` | Theo README nhóm |
| 7 | Hook `use-agent-turn-completions.ts` (kebab) | README nhóm: hook `useXxx.ts` | `useAgentTurnCompletions.ts`, `useReviewEntryAvailability.ts`, `useCodeIntelReviewSummary.ts` |
| 8 | Tín hiệu agent xong tự dựng | PQ-35: `orca.infra.agent.statusChanged` hoặc `hintAgentTurnFinished` (P1, chưa có kênh v7) | Chỉ dùng `agentStatusByPaneKey` + retained; không gọi hint |

## 2. Giải pháp

### 2.1 Mô-đun dùng chung `components/review-map/entry/` (mới)

| File | Vai trò |
|---|---|
| `agent-turn-completion.ts` | `selectAgentTurnCompletions(state, worktreeId)` từ `agentStatusByPaneKey` (`state==='done'`) + `retainedAgentsByPaneKey` (done), loại hàng `subagent`; `AgentTurnCompletion {worktreeId, paneKey, agentType, doneAt: stateStartedAt, interrupted, source:'live'|'retained', id:"${paneKey}:${doneAt}"}`; `hasUnreviewedCompletion(completions, reviewedTurnId)` |
| `useAgentTurnCompletions.ts` | subscribe store với so sánh nông; dùng bởi hàng agent, tab sidebar, Source Control, `useReviewTurnRecorder` (SOL-060), `FE-CV-SOL-089` |
| `open-review-entry.ts` | `openReviewFromEntryPoint(worktreeId, source, options?)` (`source ∈ 'agent-row'|'source-control'|'cmd-k'|'right-sidebar'|'notification'`; `options {lens?, completionId?, filter?}`): kiểm cờ → `activateAndRevealWorktree` → kích hoạt tab `review` đã có hoặc `ensureReviewTab` (FE-CV-SOL-050-review-tab-wiring; dedupe, trả `null` khi không `enabled`) → đặt phạm vi mặc định (O7) và lens/lọc |
| `useReviewEntryAvailability.ts` | `{visible, reason?: 'not-enabled'|'not-git'|'no-active-worktree'}`; `visible` chỉ khi `effective.codeIntelEnabled` và worktree thuộc repo git; trạng thái `unknown` ⇒ chưa hiển thị (tránh chớp) |
| `ReviewEntryButton.tsx` | nút nhỏ (`ScanSearch`, `Tooltip`, `aria-label`) |
| `useCodeIntelReviewSummary.ts` | tóm tắt cho tab sidebar (2.6) |

Không route mới. Cờ tắt ⇒ **không điểm vào nào render** và không có yêu cầu `codeIntel.*`.

### 2.2 Hàng agent đã xong

`DashboardAgentRow` nhận `onReview?: (agent) => void` (tuỳ chọn; vắng ⇒ hành vi cũ, test hiện có không đổi). Nút chỉ khi `state==='done'` và `rowSource !== 'subagent'` (retained được phép); `interrupted` vẫn có nút, tooltip "Agent bị ngắt giữa chừng". Luôn hiện khi `isUnvisited`, còn lại hiện khi hover/focus-visible (cùng quy tắc `group-hover/agent-row` của nút Dismiss). Ẩn khi `sendTargetStatus` đặt (chế độ chọn đích gửi). Dừng nổi bọt (`stopPropagation`, `onMouseDown`). Ô cuối `DashboardAgentRowTrailingControls` hẹp (`w-12` theo CR): **phải kiểm bằng mắt** ở thẻ gọn; có thể đổi `w-auto`. Nối ở `WorktreeCardAgents.tsx` (`handleReviewAgent`) cho cả `DashboardAgentRow` và `CompactAgentRow`; `DashboardAgentRow` không tự gọi store.

### 2.3 Thông báo

MVP **không** đổi thông báo OS (nút hành động cần sửa `desktop/src/main/ipc/notifications.ts`, ngoài `frontend/`, cần chủ sở hữu desktop duyệt). Nhấp thông báo vẫn vào worktree (luồng cũ) nơi các điểm vào khác sẵn sàng; chấm trên tab Review là nhắc trong ứng dụng. Không toast mặc định.

### 2.4 Source Control

`source-control-review-entry.tsx` (mới): `useSourceControlReviewEntry({worktreeId}) → {visible, open, hasUnreviewed}`. `SourceControl.tsx` chỉ gọi hook và truyền `onReviewChanges`/`reviewChangesVisible` xuống `SourceControlHeaderToolbar` (vài dòng; baseline max-lines, **không** thêm disable). Mục "Review changes" ở `source-control-header-overflow-menu.tsx` (trước cụm Notes) và `SourceControlHeaderIconButton` ở `source-control-branch-context-row.tsx` chỉ khi `changedFiles > 0` so với base. Mở với phạm vi O7; bộ lọc tên tệp của Source Control **không** truyền sang Review. `visible=false` ⇒ không render (không khoá).

### 2.5 Cmd+K

Ba `CmdJQuickAction` trong `createLocalizedCatalog` (không `translate()` cấp module): `review-changes` (lens `impact`), `open-architecture-map` (`architecture`), `open-erd` (`erd`). `CmdJQuickActionContext` thêm `codeIntelEnabled`, `reviewLensAvailable(lens)`, `openReviewChanges({lens?})`; `buildCmdJQuickActionContext` nhận thêm; `WorktreeJumpPalette.tsx` tạo action bằng `useCallback` như `openCreateWorkspaceAction`. `isAvailable` = availability workspace sẵn có **và** `codeIntelEnabled` **và** `reviewLensAvailable(lens)` (lens chưa phát hành ẩn). Thêm `CmdJUnavailableReason` `'code-intel-disabled'` + nhánh `getUnavailableQuickActionMessage` ("Can't … — Code review isn't enabled for this workspace.") và cập nhật `quick-action-context.test.ts`. Danh mục Cmd+J vốn "curated": ba action là bổ sung có chủ ý (nếu bị phản đối giữ riêng `review-changes`, câu hỏi mở 3). Không phím tắt riêng ở MVP.

### 2.6 Tab Review ở right sidebar

Thêm `'review'` vào `RightSidebarTab` (`shared/types.ts`), `normalizeRightSidebarRoute` (+ `right-sidebar-route.test.ts`), mục `ActivityBarItem {id:'review', icon: ScanSearch, gitOnly:true, codeIntelOnly:true}` và cờ `codeIntelOnly` trong `getVisibleRightSidebarActivityItems` (+ test), lazy `ReviewSummaryPanel` ở `right-sidebar-panel-content.tsx`, `statusIndicator` chấm "có lượt xong chưa review" (token accent, `aria-label`/tooltip; `activity-bar-overflow.ts` xử lý chấm khi mục bị đẩy vào menu tràn như `checks`). Không phím tắt ở MVP.

```
┌ Review ─────────────────────────────── [Mở đầy đủ ⤢] ┐
│ feat/x · so với main                                    │
│ Index d819812 · 2 phút trước (IndexFreshnessChip)       │
│ Rủi ro: MEDIUM [vì sao]  (risk.reasons)                 │
│ 12 file · 38 symbol · 3 luồng · 2 bảng · 1 hợp đồng     │
│ Phát hiện: 2 lỗi · 3 cảnh báo                           │
│ Tiến độ đọc 4/12 · Ghi chú: 3 chưa gửi                  │
│ Lượt 3 · Claude xong 14:32 · [Review]                   │
└─────────────────────────────────────────────────────────┘
```
`useCodeIntelReviewSummary`: chỉ yêu cầu khi `rightSidebarOpen && effectiveTab==='review'`; ưu tiên cache của khung; nếu không thì `changeOverlay {detail:'summary'}` + `findings {limit:50}` + `reviewState.get`; làm mới theo push `changed`/`codeIntelResyncCounter` và khi có `AgentTurnCompletion` mới, **không polling**; chip mở Review kèm bộ lọc. Rủi ro hiển thị `risk.level` + `risk.reasons` (không điểm số đơn; `incomplete` ⇒ nhãn "chưa đủ dữ liệu"). Trạng thái rỗng/lỗi/chưa có index dùng thành phần chuẩn của SOL-051; tải theo ngưỡng 100 ms/1 s/3 s, trì hoãn ~200 ms qua SSH.

### 2.7 i18n, token, target

Khoá `auto.components.reviewMap.Entry*|ReviewSummaryPanel*|QuickActions*` đủ en/es/ja/ko/zh; chỉ token/primitive; Electron và web cùng mã (Electron local còn `unsupported` ⇒ điểm vào tự ẩn). Phím: Tab/Enter/Space; không phím mới.

## 3. Quyết định thiết kế

- **Một hàm mở duy nhất** `openReviewFromEntryPoint`.
- **Sự kiện xong từ `agent-status` + retained**, không từ thông báo (có thể tắt/chặn, không có ở web).
- **Nút chỉ cho `done`**: không review kết quả đang đổi.
- **Cờ tắt ⇒ không render** (không khoá).
- **`SourceControl.tsx` chỉ thêm một hook + prop**.
- **Tóm tắt dùng `detail:'summary'`** để rẻ (mỗi lần gọi CLI ≈ 1,8 s theo README v7), một nguồn với khung khi có cache.

## 4. Phụ thuộc chéo khu vực

| Cần | Solution đối ứng | Ghi chú |
|---|---|---|
| `changeOverlay` (`summary`), `findings`, `reviewState.get`, `settings.get` | `BE-CV-SOL-040-codeintel-view-channels`, `BE-CV-SOL-040-codeintel-write-and-stream-channels`; `BE-CV-SOL-036-change-overlay-pipeline`; `BE-CV-SOL-073-settings-flag-and-rollout` | Nền G3; trước đó fake backend G4 |
| Tab `review`, `ensureReviewTab`, `useCodeIntelSupport`, slice | `FE-CV-SOL-050-review-tab-wiring`, `-store-and-query-hooks` | |
| Khung, `IndexFreshnessChip`, phạm vi | `FE-CV-SOL-051-review-workspace-shell`; tiến độ `FE-CV-SOL-052-…` | |
| Sự kiện xong dùng lại | `FE-CV-SOL-060-…` (recorder), `FE-CV-SOL-089-agent-turn-recorder` | |
| Lens kiểm `reviewLensAvailable` | `FE-CV-SOL-053..059` | |
| Telemetry mở Review | `FE-CV-SOL-095-review-telemetry` | Ngoài phạm vi (câu hỏi mở 4) |
| Nút hành động thông báo (tuỳ chọn, sau) | — (ngoài `frontend/`, `desktop/src/main/ipc/notifications.ts`) | Cần chủ sở hữu desktop duyệt |
| Agent | `AG-*`: — | |

Thứ tự: 050 → 051 → **061** (đợt 3) → 062.

## 5. Tiêu chí chấp nhận

- [ ] Cờ tắt/`unsupported`: không nút Review ở hàng agent, Source Control, Cmd+K, tab sidebar; không yêu cầu `codeIntel.*`.
- [ ] Hàng `done` (live/retained, không subagent) có nút; `working|blocked|waiting|idle` không; bấm không kích hoạt hàng/thẻ; ẩn khi `sendTargetStatus`.
- [ ] Một cú bấm mở/kích hoạt tab `review` đúng worktree, lens Ảnh hưởng, phạm vi O7, không tạo tab trùng.
- [ ] `DashboardAgentRow` không đổi hành vi khi không truyền `onReview`.
- [ ] Source Control: mục menu tràn + nút ở hàng nhánh (chỉ khi có thay đổi so với base); không phình `SourceControl.tsx` quá vài dòng.
- [ ] Cmd+K: ba action khả dụng/ẩn đúng; lý do `code-intel-disabled`.
- [ ] Tab right sidebar: chỉ khi cờ bật + git; `normalizeRightSidebarRoute` chấp nhận `review`, tab cũ không đổi; panel hiển thị tóm tắt, nút "Mở đầy đủ"; chấm "chưa review" có nhãn chữ.
- [ ] Nhấp thông báo OS không hồi quy; không polling; không yêu cầu trùng với khung.
- [ ] `translate()` đủ 5 locale; không hex; không `components/code-review/*`; không thêm thư viện; không `max-lines` disable.

## 6. Kiểm thử (Vitest + Testing Library)

Hàm thuần: `agent-turn-completion` (live, retained, subagent, thứ tự, `interrupted`, id); `normalizeRightSidebarRoute('review')`; `getVisibleRightSidebarActivityItems` với `codeIntelOnly`; `isAvailable` ba action. Component: `DashboardAgentRow` (mẫu `renderToStaticMarkup` như `DashboardAgentRow.test.tsx`), tương tác dừng nổi bọt (mẫu `WorktreeCardAgents.activation.test.tsx`), `ReviewSummaryPanel`, `ActivityBarButton` chấm. Hook: `openReviewFromEntryPoint` (dedupe, thứ tự activate→ensure, cờ tắt), `useCodeIntelReviewSummary` (chỉ khi hiển thị). Hồi quy: `WorktreeCardAgents*`, `right-sidebar-*`, `cmd-j/*`, `source-control-*`. **Trước khi sửa** `normalizeRightSidebarRoute`, `ActivityBarItem`, `DashboardAgentRow`, `quick-actions`: chạy `gitnexus_impact`, quét `switch` theo union (`pnpm lint:switch-exhaustiveness`, chưa kiểm chứng chạy được). E2E (`tests/e2e/code-intel-web/review-summary.web.e2e.ts`) trên fake backend — **chưa chạy**.

## 7. Rủi ro và điểm chưa kiểm chứng

- Chỗ trống cuối hàng agent có thể không đủ.
- Agent không có hook không có entry ⇒ không nút (các điểm vào khác bù).
- `done` có thể báo lặp: khử trùng theo `paneKey:doneAt`; độ tin cậy từng loại agent chưa kiểm (chính sách `agent-task-complete-policy.ts` chưa đọc).
- Thêm `'review'` vào union lan sang `activity-bar-overflow.ts`, `right-sidebar-effective-tab.ts`, lưu phiên.
- Khoá `totalCounts` chưa chốt (O: hợp đồng thiếu); chi phí `summary` chưa đo.
- Cmd+J "curated".

## 8. Câu hỏi mở

1. Thêm nút "Review" vào thông báo OS (ngoài `frontend/`)? Trên web làm sao?
2. Chốt tên khoá `ChangeOverlay.limits.totalCounts` (files/symbols/flows/tables/contracts/uncovered).
3. Giữ cả ba action Cmd+K hay chỉ `review-changes`?
4. Có ghi telemetry mở Review theo điểm vào (FE-CV-SOL-095)?
5. Worktree không có binding dev server: hiện điểm vào (Review báo thiếu binding) hay ẩn? Đề xuất hiện khi cờ bật.

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-061-review-entry-points.md`, `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§3, §8), `/opt/repos/orca/guides/STYLEGUIDE.md`, các file frontend nêu ở mục 1, `/opt/repos/orca/desktop/src/main/ipc/notifications.ts` (chỉ tham chiếu).
