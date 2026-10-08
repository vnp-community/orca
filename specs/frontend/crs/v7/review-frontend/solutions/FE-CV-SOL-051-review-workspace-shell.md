# FE-CV-SOL-051-review-workspace-shell: Khung màn Review (ba cột, thanh tóm tắt, phạm vi, chip index, trạng thái)

> 🚧 **In Progress.** Trạng thái (cập nhật 2026-10-08): 6/7 task DONE; PARTIAL 0. Chi tiết ở dòng `**Status:**` và "Ghi chú hoàn thiện" của từng task.

**CR:** [CR-CV-051](../../../../../../docs/crs/v7/review-frontend/CR-CV-051-review-workspace-shell.md)
**Area:** frontend (`frontend/src/renderer/src/{components/review-map,store/slices,hooks}`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (§2.3 lỗi, §3.1 `status/reindex/reindexStatus/changeOverlay/bindRepo`, §4.1 `IndexStatus`, §4.3 `ChangeOverlay`, `RiskAssessment`, `IndexFreshness`, §5 push), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-08, PQ-09, PQ-16, PQ-30, PQ-31, PQ-32; O-6, O-13).
**TDD tham chiếu:** [v5/05-ui-components §4.3, §6 UI Library](../../../../tdd/v5/05-ui-components.md), [v5/02-state-management §3, §7](../../../../tdd/v5/02-state-management.md), [v5/08-editor-and-files §6 Source Control](../../../../tdd/v5/08-editor-and-files.md), [v5/16-remote-git-ui](../../../../tdd/v5/16-remote-git-ui.md); storage: [browser-storage-catalog §2](../../../../storage/browser-storage-catalog.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `components/ui/` (có `tabs, badge, button, toggle-group, popover, sheet, dialog, command, select, skeleton, progress, scroll-area, tooltip, collapsible, resizable`), `shared/types.ts:3793-3804` (`GitBranchCompareSummary`), `store/slices/editor.ts:696` (`gitBranchCompareSummaryByWorktree`), `store/slices/connectivity-status.ts` (:72, :86), `web/ConnectionStatusBanner.tsx` (tên), `components/dashboard/useNow.ts`, `lib/editable-target.ts`, `components/ShortcutKeyCombo.tsx`, `hooks/useShortcutLabel.ts` (tên). Chưa đọc lại `WorkspaceLayout.tsx:88-160` (theo CR); chưa kiểm chứng mẫu `defaultSize="22"`.

**Correction relative to CR-CV-051 (hợp đồng thắng):**

| # | CR-051 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | Chip index 4 trạng thái từ `IndexStatus[]` theo công cụ | PQ-08: `IndexStatus` đơn, `overall` ∈ `OFFLINE,UNKNOWN,NOT_INSTALLED,BUILDING,MISSING,DEGRADED,OVERLAY,STALE,READY`; `tools[]`, `indexBasis[]`, `activeJob`, `scopeMismatch` | Chip theo `overall` (9 trạng thái, bảng 4.3); popover liệt kê `tools[]`/`indexBasis[]` |
| 2 | Rủi ro `LOW…CRITICAL, UNKNOWN`; `riskReasons[]` | PQ-09: `risk: RiskAssessment {level LOW..CRITICAL, score, incomplete, confidence, reasons[], modelVersion}`; `UNKNOWN` chỉ ở `ImpactGraph.risk`; **không** dùng `riskReasons` top-level; không "điểm đơn" | `incomplete:true` ⇒ nhãn "Chưa đủ dữ liệu"; lý do từ `risk.reasons[].messageKey+params` qua `translate()` |
| 3 | `toChangeOverlayParams` bỏ `head` khi gồm chưa commit | §3.1: params `base?`, `head?`, `mode?: 'worktree'|'committed'`, `detail?`; kết quả `scope {baseRef, baseOid, mergeBase, headOid, mode, includesUncommitted}` | Gửi `mode` rõ ràng (`worktree` khi tick "Gồm chưa commit"); khoá phạm vi lấy từ `overlay.scope` đã phân giải |
| 4 | Chip `violations` từ `violations[]` chưa có hình dạng | §4.3 `ViolationRef {findingKey, rule, severity, file, status touched|introduced}` | Chip đếm `violations.length`; lọc theo `file` |
| 5 | 11 trạng thái | Thêm lỗi `no-binding` (`CODEINTEL_WORKTREE_NOT_FOUND`, `NO_DEV_SERVER`, `DEV_SERVER_NOT_APPROVED`, `DEV_SERVER_MODE_UNSUPPORTED`, `WORKTREE_REF_UNSUPPORTED`), `rate-limited` (`REINDEX_COOLDOWN`), `forbidden` | 13 trạng thái (bảng 4.4); `no-binding` có nút "Thử gắn lại" (`bindRepo`) và hướng dẫn |
| 6 | Reindex đã khoá, hiển thị theo thời lượng | `percent` có thể `null`; cooldown 5 phút sau job thành công; `mode` `incremental|full` | `Progress` không xác định khi `null`; nút khoá có đếm ngược cooldown |
| 7 | Summary bar từ mảng đầy đủ | `detail:'summary'` trả `scope`, `limits.totalCounts`, `risk`, `components`, `indexFreshness` (mảng chi tiết rỗng) | Một lời gọi `full` dùng chung cache; `summary` chỉ là tối ưu mở (câu hỏi 2) |
| 8 | `ambiguous` candidates | `candidates[≤10] {key?, uid, name, kind, filePath, line, score?, impactedCount?, risk?}` | `AmbiguousSymbolDialog` hiển thị các trường này, chọn bằng `key` hoặc `{name,file}` |

Dữ liệu `ChangeOverlay.emptyReason: 'unborn-head'` ⇒ màn rỗng có giải thích. Không dùng `ConnectionStatusBanner` (web-only, `position: fixed`, hex).

## 2. Hợp đồng áp dụng

`status` (IndexStatus phẳng, không phong bì, 8 s), `reindex`, `reindexStatus`, `changeOverlay` (20 s), `bindRepo` (§3.1); lỗi §2.3 (`no-binding`, `index-missing`, `tool-unavailable`, `repo-not-registered`, `rate-limited`, `timeout`, `too-large`, `offline`, `reindex-in-progress`); push `changed`/`reindexProgress` (§5); `TruncatedLimits` (`ChangeOverlay.limits`).

## 3. Lệch giữa CR và hợp đồng

Bảng mục 1. O-13 vẫn mở: `head` vắng = cây làm việc của `detectChanges` theo CR-005 (chưa chạy); phạm vi `range`/`hostedReview` dựa vào `git.branchDiff` chấp nhận compare tổng hợp (chưa kiểm chứng).

## 4. Giải pháp

### 4.1 Cây file (`components/review-map/`, mới)

```
ReviewWorkspace.tsx ReviewHeaderBar.tsx ReviewScopePicker.tsx IndexFreshnessChip.tsx ReindexButton.tsx
ReviewRiskChip.tsx ReviewSummaryBar.tsx ReviewLensTabs.tsx ReviewDetailDrawer.tsx ReviewStateBanners.tsx
ReviewViewStateScreen.tsx ReviewLoadingStage.tsx AmbiguousSymbolDialog.tsx
review-scope-model.ts review-chip-filter.ts review-lens-registry.ts review-view-state.ts review-layout-storage.ts
store/slices/review-ui.ts   hooks/usePerceivedLoadingStage.ts
```

### 4.2 Bố cục

```
┌ Review · feat/x · so với main ─ [Phạm vi ▾] [Index ✓ d819812 · 2 phút] [Rủi ro: TRUNG BÌNH] [↻] ┐
│ 12 file · 38 symbol · 3 luồng · 2 bảng · 1 hợp đồng · 5 chưa test · 0 vi phạm   (mỗi chip = bộ lọc)│
├──────────────┬────────────────────────────────────────────────┬─────────────────────────────────┤
│ Thứ tự đọc   │ Ảnh hưởng │ Kiến trúc │ Luồng │ ERD │ … tab lens  │ Chi tiết nút đang chọn          │
│ (SOL-052)    │          thân lens đang chọn                    │ (SymbolDetailPanel, SOL-053)    │
└──────────────┴────────────────────────────────────────────────┴─────────────────────────────────┘
```

`ResizablePanelGroup orientation="horizontal"`; trái `defaultSize="22" minSize="16" maxSize="35"`, phải `26/20/40`; mở/đóng bằng render có điều kiện. Bề rộng theo `ResizeObserver` trên gốc tab: ≥ 1100 px ba cột; 720-1099 hai cột + drawer thành `Sheet`; < 720 một cột + `ToggleGroup` "Thứ tự đọc | Lens". Phím cục bộ `[`, `]`, `Esc` (bỏ qua `isEditableTarget`), không đăng ký `KEYBINDING_DEFINITIONS`. Hình học lưu `localStorage` khoá `orca.review.layout.v1` bọc `try/catch` (thêm dòng vào `storage/browser-storage-catalog.md` mục 2). `motion-reduce:transition-none`.

### 4.3 Chip index theo `overall`

| `overall` | Chip |
|---|---|
| `READY` | `Check` + `commit rút gọn · thời gian tương đối` |
| `STALE` | `TriangleAlert` + `Index cũ · indexedCommit → HEAD` (nền `muted`) |
| `OVERLAY` | `Layers`-kiểu icon + "Index checkout chính + diff" (giải thích O-6; số dòng có thể lệch) |
| `BUILDING` | spinner (tĩnh khi reduced-motion) + `percent` hoặc không xác định khi `null` |
| `MISSING`, `NOT_INSTALLED` | "Chưa có index" / "Công cụ chưa cài" (thân màn là trạng thái rỗng có hành động) |
| `DEGRADED` | cảnh báo một công cụ lỗi/thiếu; vẫn xem được |
| `OFFLINE`, `UNKNOWN` | "Ngoại tuyến" / "Chưa rõ" (không giả `READY`) |

Bấm chip mở `Popover`: `tools[]` (tên, `version`, `state`, `indexedAt`, `stats`, `pendingChanges`), `indexBasis[]` (`refreshState`, `indexPolicy`), nút "Làm mới index" và menu "Lập lại toàn bộ" (`mode:'full'`, xác nhận ngắn).

### 4.4 Trạng thái (`review-view-state.ts`, thuần)

Ưu tiên (dòng đầu thắng): 1 `unsupported/disabled` (màn, "Đóng tab") · 2 `scope-error` · 3 `no-binding` · 4 `forbidden` · 5 `loading-initial` · 6 `tool-unavailable` · 7 `repo-not-registered/path-not-allowed` · 8 `index-missing` · 9 `index-building` · 10 `offline` (có cache: banner; không: màn) · 11 `error` (`tool-failed/timeout/unknown/too-large`) · 12 `no-changes` (kể cả `emptyReason`) · 13 `ready` + banner (`stale`, `truncated {shown}/{totalCount}`, `scopeMismatch`, "Có dữ liệu mới" từ `codeIntelStaleSignalByWorktree`). Mọi lỗi persistent inline, không toast. Chờ có dữ liệu theo thang `usePerceivedLoadingStage`: `busy` ngay; `dimmed` sau 100 ms (200 ms khi đích từ xa); `spinner` ≥ 1 s; `stages` ≥ 3 s.

### 4.5 Phạm vi

```ts
type ReviewScope =
  | { kind: 'branch'; baseRef: string; includeUncommitted: boolean }
  | { kind: 'range'; baseCommit: string; headCommit: string }
  | { kind: 'hostedReview'; provider: HostedReviewProvider; number: number; headSha: string | null; baseRefName: string | null }
resolveDefaultReviewScope(summary: GitBranchCompareSummary | null)      // O7: branch từ status 'ready'
toChangeOverlayParams(scope): { base?: string; head?: string; mode: 'worktree' | 'committed' }
scopeKey(scope, overlayScope?)    // khoá cache; dùng overlay.scope (mergeBase..headOid) khi đã có
```

Nguồn: `GitBranchCompareSummary` (`status`: `ready|invalid-base|unborn-head|no-merge-base|loading|error`), `repos.searchBaseRefs`, `git.history({limit:50})`, `hostedReview.forBranch` (nhãn "Review đã gửi", không "PR"); mọi lệnh git qua `runtime-git-client`, không thêm lệnh git mới (baseline 2.25).

## 5. Quyết định thiết kế

Bắt đầu từ thay đổi, không từ đồ thị; bề rộng theo tab; mở/đóng cột bằng render có điều kiện; một chip một bộ lọc; lỗi persistent inline; không `ConnectionStatusBanner`; lens đăng ký qua mảng (`REVIEW_LENS_DEFINITIONS`, không tab chết); nhãn không overclaim ("Rủi ro thấp theo index", "Chưa đủ dữ liệu để kết luận").

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng |
|---|---|---|
| `changeOverlay`, `readingOrder` | `BE-CV-SOL-036-change-overlay-pipeline`, `BE-CV-SOL-036-reading-order-and-risk`; agent `AG-CV-SOL-005-detect-changes` | G3 + đợt 3 (fake G4 trước) |
| `status`, `reindex`, `bindRepo` | `BE-CV-SOL-040-codeintel-view-channels`, `BE-CV-SOL-012-index-status-aggregation`, `BE-CV-SOL-012-target-resolution-and-bindings`; agent `AG-CV-SOL-004-reindex-and-index-notifications`, `AG-CV-SOL-080-index-basis-and-reindex-triggers` (OVERLAY, đợt 7) | G3 |
| `overall=OVERLAY`, `indexBasis` | `BE-CV-SOL-080-auto-refresh-index` | đợt 7 (fake trước) |
| Hook, slice, tab | SOL-050-* | cần xong trước |

## 7. Tiêu chí chấp nhận

- [ ] Chỉ lens đã đăng ký có tab; thêm lens không sửa `ReviewLensTabs`.
- [ ] Ba bố cục theo bề rộng; hình học giữ qua mở lại; `localStorage` bị chặn vẫn dùng được.
- [ ] Phạm vi mặc định `branch`; `invalid-base/no-merge-base/unborn-head` hiện màn hướng xử lý.
- [ ] Chip index đúng 9 `overall`; `OVERLAY` có giải thích; `percent:null` không hiển thị "0%".
- [ ] Reindex: khoá ngay; thang thời lượng; cooldown hiển thị; `IN_PROGRESS` gắn job; không toast.
- [ ] Bảy chip luôn hiển thị; một bộ lọc một lúc; chip chuyển lens không đặt bộ lọc.
- [ ] 13 trạng thái đúng thứ tự ưu tiên; `no-binding` có hành động; không toast.
- [ ] Không hex; chuỗi `translate()`; chạy web (Electron theo SOL-050); khoá `reviewUiByWorktree` bị dọn ở cả hai đường xoá worktree.

## 8. Kiểm thử

`review-scope-model.test.ts`, `review-view-state.test.ts` (13 dòng, ưu tiên), `review-chip-filter.test.ts`, `usePerceivedLoadingStage.test.ts` (đồng hồ giả), `IndexFreshnessChip.test.tsx`, `ReviewSummaryBar.test.tsx`, `ReviewViewStateScreen.test.tsx`, `ReviewLensTabs.test.tsx`, `AmbiguousSymbolDialog.test.tsx`, `ReviewWorkspace.test.tsx` (mock `ResizeObserver`), `review-ui.test.ts` + hai test rò rỉ, `code-intel-locale-coverage` mở rộng. Dữ liệu từ `code-intel-fake-backend`. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

API `collapsible` v4 của `react-resizable-panels` chưa dùng trong repo (tránh); ngưỡng 1100/720 px là ước lượng; tập `reindexProgress.stage` chưa biết; "Gồm chưa commit" phụ thuộc `detectChanges` (O-13); `OVERLAY` lệch số dòng; tiêu điểm khi khôi phục tab trong split chưa kiểm.

## 10. Câu hỏi mở

1. `head` vắng và `mode:'worktree'` ngữ nghĩa chính xác (O-13).
2. Có cần gọi `changeOverlay{detail:'summary'}` trước để thanh tóm tắt hiện nhanh hơn trên SSH?
3. Liên kết hướng dẫn cài GitNexus/CodeGraph (chưa có URL xác nhận, không bịa).
4. `no-binding`: hướng dẫn trỏ tới đâu (màn dev server của CR-012)?

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-051-01](../tasks/FE-CV-TASK-051-01-review-ui-slice-and-scope-model.md) | `review-ui` slice và mô hình phạm vi | P0 |
| [FE-CV-TASK-051-02](../tasks/FE-CV-TASK-051-02-view-state-and-perceived-loading.md) | `review-view-state` và `usePerceivedLoadingStage` | P0 |
| [FE-CV-TASK-051-03](../tasks/FE-CV-TASK-051-03-index-freshness-chip-and-reindex-button.md) | Chip index và nút làm mới | P0 |
| [FE-CV-TASK-051-04](../tasks/FE-CV-TASK-051-04-summary-bar-risk-chip-and-chip-filter.md) | Thanh tóm tắt, chip rủi ro, bộ lọc chip | P0 |
| [FE-CV-TASK-051-05](../tasks/FE-CV-TASK-051-05-workspace-layout-lens-registry-drawer.md) | Bố cục, registry lens, drawer, `ReviewWorkspace` | P0 |
| [FE-CV-TASK-051-06](../tasks/FE-CV-TASK-051-06-state-screens-banners-ambiguous-dialog.md) | Màn trạng thái, banner, hộp mơ hồ | P0 |
| [FE-CV-TASK-051-07](../tasks/FE-CV-TASK-051-07-shell-i18n-storage-catalog-and-e2e.md) | i18n, catalog lưu trữ, e2e | P1 |

Thứ tự: 051-01 → 051-02 → 051-03, 051-04 → 051-05 → 051-06 → 051-07.

## 12. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-051-review-workspace-shell.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/frontend/src/shared/types.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/ui/resizable.tsx`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/connectivity-status.ts`.

## 13. Ghi chú triển khai (2026-10-07)

**Cách đăng ký một lens (dành cho các agent lens 053-059, 092...)**

1. Tạo component lens nhận `ReviewLensProps` (`worktreeId, environmentId, scope, overlay, selectedSymbolKey, chipFilter, onSelectSymbol, onOpenDiff, requestSymbolChoice`), `export default`, đặt ở thư mục của lens (vd. `components/review-map/impact/ImpactLens.tsx`).
2. Trong `components/review-map/review-lens-registry.ts`, ở mục có `id` của lens trong `REVIEW_LENS_DEFINITIONS` (`impact`, `architecture`, `dataflow`, `erd`, `storage`, `structure`, `contract`) thêm `load: () => import('./impact/ImpactLens')`. Chỉ thêm đúng một dòng; không sửa `ReviewLensTabs`/`ReviewWorkspace`. Lens không thuộc 7 id chuẩn (vd. `requirements`, `quality`) gọi `registerReviewLens({ id, order, labelKey, labelFallback, load, requiresQuality? })` từ module đăng ký của mình (idempotent: cùng `id` thì thay thế).
3. Khoá nhãn `auto.components.reviewMap.lens.<id>.label` đã có đủ 5 locale cho 7 lens chuẩn; lens mới tự thêm khoá của mình.
4. Lens chưa có `load` hiển thị placeholder "{{lens}} is not available yet." qua registry (không có tab chết).
5. Chi tiết symbol trong drawer: `setReviewDrawerRenderer((props) => <SymbolDetailPanel {...props} />)` (053). `onSelectSymbol(key)` mở drawer; Esc/`]` đóng và trả tiêu điểm cho phần tử đã mở.
6. Ký hiệu hai nghĩa của chip: `flows/tables/contracts` chỉ chuyển sang lens `dataflow/erd/contract` (nếu đã hiển thị), không đặt bộ lọc.
7. Lens gặp lỗi `ambiguous` gọi `props.requestSymbolChoice(candidates)` → nhận `{key}` hoặc `{name,file}` (hoặc `null` nếu huỷ) rồi tự gọi lại truy vấn.

**Sai lệch so với spec**

- Kiểu `IndexStatus/ChangeOverlay/ReadingStep/ReviewState` dùng bản local `review-wire-types.ts` (theo hợp đồng) vì `shared/code-intel-types.ts` (SOL-050) vẫn mang hình dạng cũ trước hợp đồng (`IndexOverall` 5 giá trị, `ChangeOverlay` = một file); khi 050 chỉnh lại thì thay alias. Mọi truy cập wire đi qua `review-shell-data.ts` (normalizer + `classifyReviewError` theo mã `CODEINTEL_*`) và `review-data-api-default.ts` (tải lười store/client để tránh vòng import).
- Không dùng `useCodeIntelQuery` (cần slice cache + `codeIntelSupportState`); khung dùng `useReviewShellData` riêng (poll 3 s khi đang lập chỉ mục, nghe `changed`/`reindexProgress` từ `code-intel-event-bus`).
- `Lập lại toàn bộ` xác nhận inline trong popover thay vì `useConfirmationDialog` (không cần provider).
- Mở diff mặc định = `openDiff(worktree, path)` (diff cây làm việc), chưa nhảy tới dòng (053-06 sẽ thay bằng `openReviewDiffAtSymbol`).
- Phạm vi mặc định: `gitBranchCompareSummaryByWorktree[wt]` nếu có, nếu chưa thì `worktree.baseRef`; chưa tự gọi `git.branchCompare` (051-01 mục 3). `hostedReview` trong bộ chọn phạm vi chưa được cấp dữ liệu (prop `hostedReview: null`).
- Progress UI: `ui/progress` không đẩy `aria-valuenow`; thanh tiến độ báo `aria-label` + live region.
- Điểm mở: `useCodeIntelSelector` (050-09) trả object mới mỗi lần khi `ready` → zustand v5 coi là snapshot đổi liên tục; khung dùng bản snapshot nguyên thuỷ trong `useReviewWorkspaceModel`. Cần W1-A sửa hook gốc.
- Khoá i18n động (`shell.screen.scope.*`, `shell.risk.*`, `shell.chip.*`, `readingOrder.reason.*`, `lens.*.label`) được kiểm bằng `review-shell-locale-coverage.test.ts`.

### W6 (2026-10-07): gắn thành phần vào khung

Dock đáy `shell/ReviewBottomDock.tsx` + `review-dock-registry.ts` (lens quality đăng ký qua `registerReviewDockPanel`, `requiresQuality`); `shell/use-review-companions.ts` + `ReviewCompanionStrip.tsx` mount recorder lượt, ReviewTurnSwitcher, ReportMenu, AI card; `review-lens-availability.ts` ẩn tab Lưu trữ; `use-review-surface-telemetry.ts`. Test: ReviewWorkspace.companions.test 8/8.
