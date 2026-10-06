# CR-CV-051 — Khung màn Review: bố cục ba cột, thanh tóm tắt, phạm vi, chip độ tươi index, tab lens, drawer và các trạng thái

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-051 |
| **Tên** | Khung màn Review: bố cục ba cột (`react-resizable-panels`), thanh tóm tắt với chip lọc, bộ chọn phạm vi, chip độ tươi index và làm mới, tab lens, drawer chi tiết, và các trạng thái loading/chưa có index/cũ/công cụ thiếu/ngoại tuyến/bị cắt/mơ hồ |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (kiểu, client, slice, hook, tab `review`, token); backend: CR-CV-012 (trạng thái index), CR-CV-036 (`changeOverlay`, rủi ro), CR-CV-040 (kênh) |
| **Mở khoá** | CR-CV-052, 053, 054, 055, 056, 057, 058, 059, 060, 061 |
| **Tác động** | `frontend/src/renderer/src/components/review-map/` (mới), `store/slices/review-ui.ts` (mới), `hooks/usePerceivedLoadingStage.ts` (mới), `i18n/locales/*.json`, `i18n/code-intel-locale-coverage.test.ts`, `specs/frontend/storage/browser-storage-catalog.md` (thêm một khoá) |

---

## 1. Bối cảnh và vấn đề

Sau khi agent code xong, người dùng cần thấy **thay đổi → ảnh hưởng → có ổn không** trong 1-2 thao tác ([10 §2](../../../research/view-code/10-frontend-review-ux.md)). CR-CV-050 chỉ cung cấp nền; CR này dựng **khung** mà các CR lens (053 đến 059) và danh sách thứ tự đọc (052) cắm vào.

Hiện trạng đã đọc (2026-10-05):

- `react-resizable-panels` 4.12 qua wrapper `components/ui/resizable.tsx` (`ResizablePanelGroup orientation="horizontal"`, `ResizablePanel`, `ResizableHandle`). Chỉ có mẫu dùng `defaultSize="20"`, `minSize="15"`, `maxSize="35"` (chuỗi phần trăm) và **hiện/ẩn bằng render có điều kiện** (`components/workspace/WorkspaceLayout.tsx:88-160`, `rightPanelVisible &&`). API thu gọn bằng lệnh (`collapsible`, ref) của v4 chưa được dùng ở đâu trong repo nên chưa kiểm chứng.
- Primitive sẵn có (`components/ui/`): `tabs`, `badge`, `button`, `toggle-group`, `popover`, `sheet`, `dialog`, `command`, `select`, `skeleton`, `progress`, `scroll-area`, `tooltip`, `collapsible`.
- `GitBranchCompareSummary {baseRef, baseOid, compareRef, headOid, mergeBase, changedFiles, commitsAhead, status: 'ready'|'invalid-base'|'unborn-head'|'no-merge-base'|'loading'|'error'}` và `gitBranchCompareSummaryByWorktree` (`shared/types.ts:3793-3803`, `store/slices/editor.ts:696`) đã cho đúng dữ liệu phạm vi "worktree so với nhánh gốc" (O7). `git.history({worktreePath, limit, baseRef})` trả `GitHistoryItem[]` (`id, subject, displayId, parentIds, timestamp`; `shared/git-history-types.ts`); `repos.searchBaseRefs({repoId, query, limit})` (`api-types.ts:1051`); `hostedReview.forBranch(args)` trả `HostedReviewInfo {provider, number, title, headSha, baseRefName, url}` (`shared/hosted-review.ts:18-39`).
- **`ConnectionStatusBanner` không dùng được** như research 10 §7 gợi ý: nó chỉ có ở web (`web/ConnectionStatusBanner.tsx`), là lớp phủ `position: fixed` góc dưới phải, và hardcode màu hex (`#f59e0b`, `#ef4444`). Tín hiệu ngoại tuyến thật nằm ở `store/slices/connectivity-status.ts` (`connections`, `isConnectivityLikeRpcError`).
- Không có hook nào làm "tiến độ theo thời lượng" của STYLEGUIDE ("Match in-flight feedback to perceived duration"); mỗi nơi tự làm.
- `lib/editable-target.ts` (`isEditableTarget`) để bỏ qua phím tắt khi đang nhập; `components/ShortcutKeyCombo.tsx` và `hooks/useShortcutLabel.ts` cho nhãn phím theo nền tảng.

## 2. Giải pháp đề xuất

### 2.1 Cây component (`components/review-map/`, mới)

```
ReviewWorkspace                       (gốc của tab review; props: worktreeId)
├─ ReviewHeaderBar
│    ├─ ReviewScopePicker             (2.3)
│    ├─ IndexFreshnessChip            (2.4)  + ReindexButton
│    └─ ReviewRiskChip                (2.5)
├─ ReviewStateBanners                 (stale, truncated, offline-có-cache; 2.7)
├─ ReviewSummaryBar                   (2.5)
└─ ReviewPanels                       (ResizablePanelGroup ngang; 2.2)
     ├─ [trái]  <ReadingOrderList/>   (CR-CV-052)
     ├─ [giữa]  ReviewLensTabs + thân lens (lazy, 2.6)
     └─ [phải]  ReviewDetailDrawer    (2.6; chứa SymbolDetailPanel của CR-CV-053)
ReviewViewStateScreen                 (thay toàn thân khi chưa sẵn sàng; 2.7)
```

### 2.2 Bố cục

```
┌ Review · feat/x · so với main ─ [Phạm vi ▾] [Index: d819812 · 2 phút trước ✓] [Rủi ro: TRUNG BÌNH] [↻] ┐
├────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ 12 file · 38 symbol · 3 luồng · 2 bảng · 1 hợp đồng · 5 chưa có test · 0 vi phạm       (mỗi chip = bộ lọc)│
├──────────────┬───────────────────────────────────────────────────────────┬─────────────────────────────┤
│ Thứ tự đọc   │ Ảnh hưởng │ Kiến trúc │ Luồng │ ERD │ Lưu trữ │ Cấu trúc │…│ Chi tiết nút đang chọn      │
│ (CR-CV-052)  │                                                           │ (SymbolDetailPanel, 053)    │
│              │              thân lens đang chọn                          │                             │
└──────────────┴───────────────────────────────────────────────────────────┴─────────────────────────────┘
```

- `ResizablePanelGroup orientation="horizontal"`; trái `defaultSize="22" minSize="16" maxSize="35"`, giữa phần còn lại, phải `defaultSize="26" minSize="20" maxSize="40"`. Cột trái và phải mở/đóng bằng **render có điều kiện** (mẫu `WorkspaceLayout.tsx`), không phụ thuộc API thu gọn chưa kiểm chứng của v4.
- **Theo bề rộng của chính tab**, không theo cửa sổ (tab Review có thể nằm trong split cạnh terminal của agent): đo bằng `ResizeObserver` trên gốc. ≥ 1 100 px: ba cột. 720 đến 1 099 px: hai cột, drawer phải thành `Sheet` (`ui/sheet.tsx`, mở khi chọn nút). < 720 px: một cột, thanh `ToggleGroup` hai mục "Thứ tự đọc" và "Lens" chọn nội dung hiển thị, drawer là `Sheet`.
- Phím tắt cục bộ khi tiêu điểm nằm trong tab Review và không phải ô nhập (`isEditableTarget`): `[` mở/đóng cột trái, `]` mở/đóng cột phải/drawer, `Esc` đóng drawer khi tiêu điểm trong drawer. Không thêm hành động vào `KEYBINDING_DEFINITIONS` (`shared/keybindings.ts`) ở CR này (không có bàn phím toàn cục nào để đăng ký). Chỉ hiển thị chip phím (`ShortcutKeyCombo`) cho những phím đã cài đặt, ở chân hai cột tương ứng; hai phím không cần phân biệt Mac/Windows nên không có logic `metaKey`.
- Hình học panel (mở/đóng, kích thước) lưu `localStorage` khoá `orca.review.layout.v1`, bọc `try/catch` (trình duyệt riêng tư có thể chặn), vẫn hiển thị đúng khi không đọc được; thêm một dòng vào `specs/frontend/storage/browser-storage-catalog.md` mục 2 (hình học panel). Trạng thái đã xem **không** nằm ở đây (CR-CV-052, backend).
- Tiêu điểm mặc định khi mở có chủ đích (từ nút Review, CR-CV-061): danh sách Thứ tự đọc. Khi tab được khôi phục lúc khởi động (không do người dùng bấm) thì không giành tiêu điểm: kiểm `document.activeElement` là `body` hoặc nằm trong nhóm tab này.
- `prefers-reduced-motion`: mở/đóng cột và `Sheet` dùng lớp `motion-reduce:transition-none` (mẫu `onboarding/OnboardingFlow.tsx`).

### 2.3 Phạm vi (`ReviewScopePicker`)

`components/review-map/review-scope-model.ts` (mới, thuần, có test):

```ts
type ReviewScope =
  | { kind: 'branch'; baseRef: string; baseOid: string | null; mergeBase: string | null;
      headOid: string | null; includeUncommitted: boolean }
  | { kind: 'range'; baseCommit: string; headCommit: string }
  | { kind: 'hostedReview'; provider: HostedReviewProvider; number: number;
      headSha: string | null; baseRefName: string | null }
```

- `resolveDefaultReviewScope(summary: GitBranchCompareSummary | null)`: `status==='ready'` → `{kind:'branch', baseRef, baseOid, mergeBase, headOid, includeUncommitted:true}` (**mặc định O7**). Các trạng thái khác trả `{error: 'invalid-base'|'no-merge-base'|'unborn-head'|'loading'|'error'}` và `ReviewViewStateScreen` hiện hướng xử lý (chọn nhánh gốc khác bằng `ReviewScopePicker`). Nếu `gitBranchCompareSummaryByWorktree[worktreeId]` chưa có, gọi `git.branchCompare` qua `runtime-git-client` (như Source Control) rồi `setGitBranchCompareResult`.
- `scopeKey(scope)` ổn định, chứa commit đã phân giải (`mergeBase..headOid` hoặc `baseCommit..headCommit` hoặc `hr:<provider>:<number>@<headSha>`); dùng làm phần `scopeKey` của `queryKey` ở CR-CV-050 và làm khoá `(baseCommit, headCommit)` của `reviewState.get/save` ở CR-CV-052.
- `toChangeOverlayParams(scope)`: giảm mọi phạm vi về `{base, head?}` (branch: `base=mergeBase`, `head` bỏ khi `includeUncommitted` để backend dùng cây làm việc; range: hai commit; hostedReview: `base` từ `baseRefName`, `head=headSha`). Ngữ nghĩa "head vắng = cây làm việc" của `codeintel.detectChanges` **chưa kiểm chứng** (README 3.2 ghi `head?`); xem 7.1.

Giao diện: `Popover` chứa `ToggleGroup` ba mục (Nhánh gốc, Khoảng commit, Review đã gửi). Nội dung theo mục:

| Mục | Điều khiển | Nguồn |
|---|---|---|
| Nhánh gốc | `Select` tìm kiếm cho `baseRef` (mặc định `summary.baseRef`, tìm bằng `repos.searchBaseRefs`), ô tick "Gồm thay đổi chưa commit" | `GitBranchCompareSummary` |
| Khoảng commit | Hai `Select` "Từ (không gồm)" và "Đến" liệt kê tối đa 50 commit gần nhất (`displayId`, `subject`) | `git.history({worktreePath, limit:50, baseRef})` |
| Review đã gửi | Hiện khi `hostedReview.forBranch` trả về thông tin (`#số tiêu đề`); gọi một lần khi mở popover và dùng bộ nhớ đệm `hosted-review` slice nếu có | `HostedReviewInfo` |

Tên và nhãn dùng "Review đã gửi" (hosted review), không "PR", để GitLab/Bitbucket dùng chung khái niệm (AGENTS.md "Git Provider Compatibility"). Mọi lệnh git qua `runtime-git-client` (hoạt động cả local, SSH, dev server). Khi gọi git phải theo baseline Git 2.25: CR này chỉ dùng các API đã có, không thêm lệnh git mới.

### 2.4 Chip độ tươi index và làm mới (`IndexFreshnessChip`)

Dữ liệu từ `useCodeIntelIndexStatus(worktreeId)`. Mỗi lens/tool có một `IndexStatus` (`state: 'missing'|'building'|'ready'|'stale'`, `indexedCommit`, `headCommit`, `indexedAt`, `pendingChanges`).

| Trạng thái | Chip |
|---|---|
| `ready`, `indexedCommit === headCommit` | icon `Check` (`text-status-success`) + `d819812 · 2 phút trước` |
| `stale` | icon `TriangleAlert` + `Index cũ · d819812 → HEAD a41c9e0` (nền `muted`, không màu cảnh báo mới) |
| `building` | `Loader2` quay (`motion-reduce` thì tĩnh) + `Đang lập chỉ mục {percent}%` |
| `missing` | `Chưa có index` (thân màn là trạng thái rỗng 2.7) |

Bấm chip mở `Popover` chi tiết từng công cụ (`sources[]`: tên, phiên bản, `indexedAt`, `commit`), thống kê (`stats`), `pendingChanges {added, modified, removed}`, và nút "Làm mới index". Nhãn thời gian dùng `Intl.RelativeTimeFormat` cập nhật mỗi phút bằng `useNow` (`components/dashboard/useNow.ts`).

**Làm mới (O3, người dùng bấm)**: `ReindexButton` gọi `useCodeIntelReindex(worktreeId).start('incremental')`. Khoá nút **ngay** khi bấm (chống bấm đúp, rubric SSH). Hiển thị theo thời lượng: < 100 ms không đổi; 100 ms đến 1 s chỉ vô hiệu hoá (dùng 200 ms nếu đích là môi trường từ xa); ≥ 1 s spinner trong nút; ≥ 3 s hiện nhãn giai đoạn và `Progress` (`ui/progress.tsx`) với `percent` từ `codeIntel.reindexProgress`. Giai đoạn lạ (`stage` ngoài danh sách nhãn đã biết) hiển thị nguyên `message` của backend. Lỗi `CODEINTEL_REINDEX_IN_PROGRESS` thì gắn vào job đang chạy thay vì báo lỗi. Menu phụ trong popover: "Lập lại toàn bộ" (`mode:'full'`) có hộp xác nhận ngắn vì tốn tài nguyên dev server; các giá trị `mode` chưa kiểm chứng (CR-CV-050 mục 7.7). Khi job kết thúc, chip chuyển `ready` và các lens tải lại nhờ `codeIntelResyncCounter` (CR-CV-050). Thất bại: lỗi persistent inline trong popover và trên chip, không dùng toast.

### 2.5 Thanh tóm tắt và rủi ro (`ReviewSummaryBar`, `ReviewRiskChip`)

Dữ liệu: `ChangeOverlay` (`useCodeIntelQuery('changeOverlay', {scope})`). Mỗi chip luôn hiển thị (kể cả 0, ở dạng mờ và vô hiệu) để chiều rộng ổn định, và có `aria-pressed`:

| Chip | Số lấy từ | Hành vi bấm |
|---|---|---|
| `files` | `changedFiles.length` | Bộ lọc: chỉ symbol thuộc file đã đổi |
| `symbols` | `changedSymbols.length` | Bộ lọc: chỉ symbol đã đổi |
| `flows` | `affectedFlows.length` | Chuyển sang lens Luồng (CR-CV-056) |
| `tables` | `touchedTables.length` | Chuyển sang lens ERD (CR-CV-057) |
| `contracts` | `touchedContracts.length` | Chuyển sang lens Hợp đồng (CR-CV-059) |
| `untested` | `uncoveredSymbols.length` | Bộ lọc: chỉ symbol chưa có test |
| `violations` | `violations.length` | Bộ lọc: chỉ phần có vi phạm lớp |

Bộ lọc chọn **một** chip tại một thời điểm (bấm lại để bỏ). Trạng thái ở `ReviewUiState.chipFilter` (2.8); `reviewChipPredicate(overlay, chip)` (`review-chip-filter.ts`, mới, thuần) trả `Set<symbolKey>` hoặc `Set<filePath>` để Thứ tự đọc (CR-CV-052) và các lens (CR-CV-053 đến 056) làm mờ hoặc ẩn phần không khớp. Chip chuyển lens không đặt bộ lọc, chỉ đổi `lens`. Hình dạng phần tử của `violations[]` chưa có trong 05/08 (xem 7.2).

`ReviewRiskChip` hiển thị `ChangeOverlay.risk` (`LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN`) bằng icon và chữ (không chỉ màu): LOW `text-status-success`, MEDIUM `text-foreground`, HIGH và CRITICAL `text-destructive`, UNKNOWN `text-muted-foreground`. Bấm mở `Popover` "Vì sao mức này": hiển thị `riskReasons[]` nếu backend trả (đề xuất ở 7.3); nếu không, chỉ hiển thị số liệu đã có và câu "Mức do dịch vụ tính; chưa có lý do chi tiết". Nhãn không bao giờ ghi "an toàn" cho LOW, mà "Rủi ro thấp theo index".

### 2.6 Lens, drawer, đăng ký lens

`review-lens-registry.ts` (mới):

```ts
type ReviewLensId = 'impact' | 'architecture' | 'dataflow' | 'erd' | 'storage' | 'structure' | 'contract'
type ReviewLensProps = {
  worktreeId: string; scope: ReviewScope; overlay: ChangeOverlay | null
  selectedSymbolKey: string | null; chipFilter: ReviewChipId | null
  onSelectSymbol: (ref: SymbolRef | null) => void
  onOpenDiff: (ref: SymbolRef) => void           // do CR-CV-053 cung cấp
}
REVIEW_LENS_DEFINITIONS: { id; labelKey; icon /* lucide */; load: () => Promise<{default: ComponentType<ReviewLensProps>}> }[]
```

Chỉ lens **đã được đăng ký** mới có tab: CR lens sau thêm một mục vào mảng này cùng với component; không có tab chết. Thứ tự tab cố định theo research 10 §5: Ảnh hưởng (mặc định), Kiến trúc, Luồng, ERD, Lưu trữ, Cấu trúc, Hợp đồng. Mỗi lens tải lười (`React.lazy` + `Suspense`, fallback `Skeleton`) để `@xyflow/react` và Mermaid không vào chunk khởi động (Mermaid đã tải lười ở `MermaidBlock.tsx`). `ReviewLensTabs` dùng `ui/tabs.tsx` (Radix, điều hướng bằng mũi tên); thân lens chỉ gắn khi tab được chọn, giữ trạng thái cuộn/viewport của lens khi đổi tab bằng cache theo `(worktreeId, lens)` do từng lens tự lo.

`ReviewDetailDrawer`: nhận `children`; mặc định là `SymbolDetailPanel` (CR-CV-053) khi có `selectedSymbolKey`, rỗng thì `Chọn một nút để xem chi tiết`. Lens khác (ERD, C4) có thể thay thế nội dung drawer qua context `ReviewDrawerContent` (mới). `Esc` đóng, tiêu điểm trả lại phần tử đã kích hoạt.

### 2.7 Các trạng thái

`review-view-state.ts` (mới, hàm thuần, có test) tính `ReviewViewState` từ `useCodeIntelSupport`, `useCodeIntelIndexStatus`, truy vấn `changeOverlay` và `connections`. Thứ tự ưu tiên, dòng đầu thắng:

| # | Trạng thái | Điều kiện | Hiển thị (toàn thân nếu ghi "màn", còn lại banner inline) |
|---|---|---|---|
| 1 | `unsupported`/`disabled` | `useCodeIntelSupport` | Màn "Tính năng chưa bật cho tổ chức này" (chỉ gặp khi tab được khôi phục), nút "Đóng tab". Không toast |
| 2 | `scope-error` | `resolveDefaultReviewScope` lỗi | Màn: giải thích (`invalid-base`, `no-merge-base`, `unborn-head`) + mở `ReviewScopePicker` |
| 3 | `loading-initial` | chưa có dữ liệu, đang tải | `ReviewLoadingStage` (bảng dưới) |
| 4 | `tool-unavailable` | `CODEINTEL_TOOL_UNAVAILABLE` | Màn lỗi **persistent** inline: "GitNexus/CodeGraph chưa có trên dev server" + cách xử lý, không toast |
| 5 | `repo-not-registered`/`path-not-allowed` | mã tương ứng | Màn lỗi inline có hướng xử lý (không lộ đường dẫn thô) |
| 6 | `index-missing` | `IndexStatus.state==='missing'` hoặc `CODEINTEL_INDEX_MISSING` | Màn rỗng có hành động trực tiếp: nút "Lập chỉ mục" (nếu không `forbidden`) + liên kết hướng dẫn cài công cụ (7.4) |
| 7 | `index-building` | job `running` hoặc `state==='building'` | Màn: `Progress` + giai đoạn (cùng thang thời lượng ở 2.4) |
| 8 | `offline` | `kind==='offline'` | Có cache: banner "Không kết nối được dev server, đang hiển thị dữ liệu lưu lúc {thời gian}" + "Thử lại"; không cache: màn |
| 9 | `error` | `tool-failed`, `timeout`, `unknown`, `too-large` | Màn lỗi inline, nút "Thử lại", chi tiết sao chép được; `too-large` gợi ý thu hẹp phạm vi |
| 10 | `no-changes` | overlay sẵn sàng, `changedFiles.length===0` | Màn rỗng "Phạm vi này không có thay đổi" + mở bộ chọn phạm vi |
| 11 | `ready` | còn lại | Khung đầy đủ, kèm banner theo cờ |

Banner inline khi `ready` (nằm trong `ReviewStateBanners`, nền `muted`, `border-border`, icon, nút hành động; **không dùng toast** vì người dùng cần đọc và hành động):

- `stale`: "Index cũ: lập chỉ mục tại `{indexedCommit}`, HEAD hiện tại `{headCommit}`. Dữ liệu vẫn hiển thị." + nút "Làm mới index".
- `truncated`: "Đang hiển thị {shown}/{totalCount}. Thu hẹp phạm vi hoặc dùng bộ lọc." + nút mở bộ lọc (không cắt im lặng, [10 §5](../../../research/view-code/10-frontend-review-ux.md)). Mỗi lens báo cờ `truncated` của truy vấn riêng; banner hiển thị theo lens đang chọn.
- `offline` có cache: như dòng 8.

**Mơ hồ (`ambiguous`)**: `CODEINTEL_AMBIGUOUS_SYMBOL` kèm `candidates` mở `AmbiguousSymbolDialog` (`ui/dialog.tsx` + `ui/command.tsx`, có ô tìm kiếm; lấy tiêu điểm vào ô tìm; Enter chọn); chọn xong gọi lại truy vấn với `uid` của ứng viên.

**Ngoại tuyến**: tự thử lại khi `connections[…].status` của kết nối tương ứng chuyển sang `established` (theo `connectivity-status.ts`); thử lại thủ công luôn có. Không dùng `ConnectionStatusBanner` (1).

**`ReviewLoadingStage`** và `hooks/usePerceivedLoadingStage.ts` (mới): `usePerceivedLoadingStage(isPending, {remote})` trả `'idle'|'busy'|'dimmed'|'spinner'|'stages'`. `busy` (khoá điều khiển, `aria-busy`) áp ngay; `dimmed` sau 100 ms (200 ms khi `remote`); `spinner` sau 1 s; `stages` sau 3 s (nhãn giai đoạn "Đang truy vấn index…", "Đang tính ảnh hưởng…" do CR gọi truyền vào). Thang theo STYLEGUIDE; giữ chỗ cố định cho vùng spinner để không nhảy bố cục (đặt `width`, không `min-width`). `remote` lấy từ `getRuntimeEnvironmentIdForWorktree(...) !== null`.

### 2.8 Trạng thái giao diện (`store/slices/review-ui.ts`, mới)

```
ReviewUiSlice {
  reviewUiByWorktree: Record<worktreeId, ReviewUiState>
  ReviewUiState = { scope: ReviewScope | null; lens: ReviewLensId; chipFilter: ReviewChipId | null
                    selectedSymbolKey: string | null; scopePickerOpen: boolean }
  setReviewScope(worktreeId, scope) / setReviewLens / setReviewChipFilter / setReviewSelectedSymbol
  resetReviewUi(worktreeId)
}
```

Đổi `scope` xoá `chipFilter` và `selectedSymbolKey`. Các CR lens thêm trường vào `ReviewUiState` (không đổi slice nào khác): CR-CV-053 thêm `impactFocusKey`, CR-CV-055 thêm `c4ContainerId` và `c4Drafts`, CR-CV-056 thêm `dataFlowId`; mọi trường vẫn nằm dưới khoá `reviewUiByWorktree` nên được dọn cùng một lúc. Khoá `reviewUiByWorktree` được thêm vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (CR-CV-050) và có test rò rỉ qua cả hai đường xoá worktree. Slice đăng ký ở `store/index.ts`, `store/types.ts`, `store-test-helpers.ts`. Không lưu vào phiên (xem CR-CV-050 2.8).

### 2.9 Cấu trúc file

```
components/review-map/ReviewWorkspace.tsx ReviewHeaderBar.tsx ReviewScopePicker.tsx IndexFreshnessChip.tsx
  ReindexButton.tsx ReviewRiskChip.tsx ReviewSummaryBar.tsx ReviewLensTabs.tsx ReviewDetailDrawer.tsx
  ReviewStateBanners.tsx ReviewViewStateScreen.tsx ReviewLoadingStage.tsx AmbiguousSymbolDialog.tsx
  review-scope-model.ts review-chip-filter.ts review-lens-registry.ts review-view-state.ts review-layout-storage.ts
store/slices/review-ui.ts      hooks/usePerceivedLoadingStage.ts
```

## 3. Quyết định thiết kế

- **Bắt đầu từ thay đổi, không từ đồ thị**: thanh tóm tắt và Thứ tự đọc luôn thấy trước; lens là công cụ trả lời.
- **Bề rộng theo tab, không theo cửa sổ**: tab Review nằm cạnh terminal trong split.
- **Mở/đóng cột bằng render có điều kiện** vì chỉ mẫu này đã kiểm chứng trong repo.
- **Một chip một bộ lọc** (không AND/OR nhiều chip): ngữ nghĩa dễ hiểu; chip `flows/tables/contracts` chuyển lens thay vì lọc.
- **Trạng thái lỗi là persistent inline, không toast** (STYLEGUIDE): người dùng cần đọc, thử lại, sao chép.
- **Không dùng `ConnectionStatusBanner`**; dùng `connectivity-status`.
- **Phạm vi gọi là "Review đã gửi"**, không "PR", cho GitLab và các nhà cung cấp khác.
- **Lens đăng ký qua mảng**, không tab chết khi CR lens chưa có.
- Nhãn không overclaim: không "an toàn", không "đã kiểm tra" khi chưa có kết quả.

## 4. Tiêu chí chấp nhận

- [ ] Mở tab Review (qua `ensureReviewTab`) với chỉ lens Ảnh hưởng đã đăng ký: thấy đúng một tab lens; đăng ký thêm lens thì xuất hiện tab, không sửa `ReviewLensTabs`.
- [ ] Ba bố cục theo bề rộng (≥ 1 100, 720-1 099, < 720 px) hoạt động; kéo thay đổi cột giữ nguyên bề rộng sau khi mở lại tab (đọc `orca.review.layout.v1`); `localStorage` bị chặn thì vẫn dùng được.
- [ ] Phạm vi mặc định là `branch` từ `GitBranchCompareSummary` khi `status==='ready'`; `invalid-base`/`no-merge-base`/`unborn-head` hiện màn hướng xử lý; chọn khoảng commit và review đã gửi đổi `scopeKey` và tải lại.
- [ ] Chip index hiển thị `indexedCommit` rút gọn, thời gian tương đối, và đúng 4 trạng thái; `stale` hiển thị cả hai commit.
- [ ] Nút làm mới bị khoá ngay khi bấm; nhìn thấy: không đổi < 100 ms, vô hiệu hoá 100 ms-1 s, spinner ≥ 1 s, nhãn giai đoạn và `Progress` ≥ 3 s; đích từ xa trì hoãn phần hiển thị 200 ms. Kết thúc job thì lens tải lại.
- [ ] Bảy chip hiển thị kể cả bằng 0; bấm `untested` lọc Thứ tự đọc và làm mờ lens; bấm `tables` chuyển sang lens ERD (nếu đã đăng ký) mà không đặt bộ lọc.
- [ ] Mỗi trạng thái ở bảng 2.7 (dòng 1 đến 11) có thân hoặc banner đúng, không toast; `tool-unavailable` và `index-missing` có hành động trực tiếp.
- [ ] `truncated` hiển thị "{shown}/{total}" và gợi ý lọc; không bao giờ cắt im lặng.
- [ ] `ambiguous` mở hộp chọn có tìm kiếm; chọn xong truy vấn lại thành công.
- [ ] Ngoại tuyến có cache giữ dữ liệu và đánh dấu cũ; tự thử lại khi kết nối `established`.
- [ ] Chỉ phím `[`, `]`, `Esc` cục bộ; không hoạt động khi tiêu điểm ở ô nhập; không có nhãn phím cho phím chưa cài đặt.
- [ ] `prefers-reduced-motion` tắt chuyển động mở cột/`Sheet` và spinner dùng biểu diễn tĩnh.
- [ ] Không có hex hay lớp màu Tailwind thô trong file mới; chỉ token và primitive; mọi chuỗi qua `translate()` đủ 5 locale.
- [ ] Hoạt động ở cả Electron và web (cùng mã, chỉ khác bridge ở CR-CV-050).
- [ ] Khoá `reviewUiByWorktree` bị xoá khi xoá worktree bằng cả hai đường; không `max-lines` disable mới.

## 5. Kiểm thử

- `review-scope-model.test.ts`: mặc định từ từng `status`; `scopeKey` ổn định; `toChangeOverlayParams` cho ba loại; `includeUncommitted`.
- `review-view-state.test.ts`: bảng 11 dòng, thứ tự ưu tiên (ví dụ `tool-unavailable` thắng `index-missing`; ngoại tuyến có cache khác không cache).
- `review-chip-filter.test.ts`: từng chip; chip chuyển lens không trả bộ lọc.
- `usePerceivedLoadingStage.test.ts` (đồng hồ giả): ngưỡng 100 ms/1 s/3 s, từ xa 200 ms, `busy` áp ngay, huỷ khi `isPending` về false trước ngưỡng.
- `IndexFreshnessChip.test.tsx`, `ReviewSummaryBar.test.tsx`, `ReviewViewStateScreen.test.tsx`, `ReviewLensTabs.test.tsx`, `AmbiguousSymbolDialog.test.tsx` (`// @vitest-environment happy-dom`, Testing Library): hiển thị theo trạng thái, `aria-pressed`, tiêu điểm vào ô tìm, `Esc` đóng drawer.
- `ReviewWorkspace.test.tsx`: giả `ResizeObserver` để kiểm ba bố cục; `react-resizable-panels` thật hoặc mock như `TaskDAGView.test.tsx` mock `@xyflow/react` (hộp panel cần kích thước thật).
- `store/slices/review-ui.test.ts` và `review-ui-worktree-removal-leak.test.ts` (mẫu ở CR-CV-050 mục 5).
- `i18n/code-intel-locale-coverage.test.ts` mở rộng khoá của CR này.
- Dùng `test-support/code-intel-fake-backend.ts` (CR-CV-050) để dựng `changeOverlay`, `IndexStatus`, lỗi `CODEINTEL_*`.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- API v4 của `react-resizable-panels` (`collapsible`, ref imperative) chưa được dùng trong repo; CR cố tình tránh. Hành vi `defaultSize="22"` (chuỗi phần trăm) chỉ suy từ `WorkspaceLayout.tsx`.
- Ngưỡng bề rộng 1 100/720 px là ước lượng, chưa thử trên tab trong split thật.
- Nhãn giai đoạn của `reindexProgress.stage` chưa biết tập giá trị; mới chỉ có `percent` và `message` (README 3.2).
- `ChangeOverlay.violations[]`, `riskReasons[]`, `readingOrder[]` chưa có hình dạng trong 05/08; chip và popover rủi ro phụ thuộc.
- Chưa chạy giao diện; chưa kiểm chứng tiêu điểm khi tab khôi phục ở nhóm tab chia đôi.
- "Gồm thay đổi chưa commit" giả định index/`detectChanges` hiểu cây làm việc; nếu index chỉ phản ánh commit thì dữ liệu cho phần chưa commit có thể lệch (hiển thị qua `pendingChanges`).

## 7. Câu hỏi mở

1. `codeintel.detectChanges` hiểu `head` vắng là cây làm việc (gồm chưa commit) hay chỉ HEAD? Quyết định `includeUncommitted` và nhãn ô tick.
2. Hình dạng `violations[]` (cần `from`, `to`, `rule`, `evidence`) và `readingOrder[]`: CR-CV-036 chốt.
3. `riskReasons[]` có được trả không (research 10 §6.1 yêu cầu nhãn rủi ro kèm lý do bấm xem được)?
4. Liên kết hướng dẫn cài GitNexus/CodeGraph ở trạng thái `index-missing`: trỏ tới đâu (chưa có URL đã xác nhận, không bịa)?
5. Phạm vi "Review đã gửi": backend nhận số hiệu hay chỉ `base/head`? Frontend tạm gửi `base=baseRefName`, `head=headSha`.
6. Có cần phím tắt toàn cục "Mở Review" trong `KEYBINDING_DEFINITIONS` không? Thuộc CR-CV-061.
7. Giá trị `mode` của reindex và có cần quyền riêng (ẩn nút khi `forbidden`) hay không.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O3, O7, 3.2, 3.3), `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§4, §6, §7, §8)
- `/opt/repos/orca/guides/STYLEGUIDE.md` (UX rules 1; Screen UX review rubric)
- `/opt/repos/orca/frontend/src/renderer/src/components/ui/resizable.tsx`, `components/workspace/WorkspaceLayout.tsx` (:88-160)
- `/opt/repos/orca/frontend/src/shared/types.ts` (:3793-3823), `shared/git-history-types.ts`, `shared/hosted-review.ts` (:18-39)
- `/opt/repos/orca/frontend/src/preload/api-types.ts` (:1051 `searchBaseRefs`, :1766 `hostedReview`, :2857 `git.history`, :2873 `git.branchCompare`)
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/{editor,connectivity-status}.ts`, `web/ConnectionStatusBanner.tsx`, `lib/editable-target.ts`, `components/ShortcutKeyCombo.tsx`, `components/dashboard/useNow.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/__tests__/TaskDAGView.test.tsx` (mẫu mock xyflow)
- `/opt/repos/orca/specs/frontend/storage/browser-storage-catalog.md`
- Mới: `frontend/src/renderer/src/components/review-map/*`, `store/slices/review-ui.ts`, `hooks/usePerceivedLoadingStage.ts`
