# FE-CV-SOL-052-reading-order-and-progress: Thứ tự đọc, tiến độ review, phím tắt, lưu qua `reviewState`

> 🚧 **In Progress.** Triển khai và kiểm chứng 2026-10-07: 5/6 task DONE, 052-05 PARTIAL (thiếu cờ lớp phủ `review-overlay-model` của 053-01). Viết ngày 2026-10-06.

**CR:** [CR-CV-052](../../../../../../docs/crs/v7/review-frontend/CR-CV-052-reading-order-and-review-progress.md)
**Area:** frontend (`components/review-map`, `store/slices/review-progress.ts`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (§3.1 `readingOrder`, `reviewState.get/save`; §4.3 `ReadingStep`, `ComponentGroup`, `ReasonCode`; §4.6 `ReadingProgress`, `ReviewState`; §2.3 `VERSION_CONFLICT`), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-22, PQ-31, PQ-14).
**TDD tham chiếu:** [v5/02-state-management §3, §7](../../../../tdd/v5/02-state-management.md), [v5/08-editor-and-files §7 Diff Comments](../../../../tdd/v5/08-editor-and-files.md); storage: [README](../../../../storage/README.md) (không `persist`), [feature-persistence-matrix](../../../../storage/feature-persistence-matrix.md), [dev-server-agent-impact](../../../../storage/dev-server-agent-impact.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `store/slices/diff-comments-persist-queue.ts` (tên; mẫu hàng chờ ghi theo worktree, **chưa đọc nội dung**), `lib/editable-target.ts`, `components/ShortcutKeyCombo.tsx`, `ui/{checkbox,collapsible,progress,skeleton}` (có trong danh sách `components/ui`), `components/editor/CsvViewer.tsx` (mẫu `useVirtualizer`, theo CR), storage README (không IndexedDB/`persist`), `connectivity-status.ts`. Chưa tồn tại: `review-progress.ts`, `reading-order-model.ts`.

**Correction relative to CR-CV-052 (hợp đồng thắng):**

| # | CR-052 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `ReadingOrderItem` mỗi **symbol**, khoá `symbol.key`, `order`, `component{id,name}` | PQ-31: `ReadingStep {stepKey, n, file, symbols[], hunks[], reason: ReasonCode, reasonParams?, dependsOn[], tests[], cycleGroup?, layer}`; `ComponentGroup {componentId, containerId, label, files, symbols, added, removed, riskPoints, stepKeys[]}`; tiến độ khoá **`stepKey`** | Một hàng = một **bước (file)**, mở rộng để thấy `symbols`; nhóm theo `ComponentGroup.stepKeys`; `ReadingOrderItem` dựng ở client từ `ReadingStep`+`ComponentGroup`+`ChangedFile` |
| 2 | `summary`, `reason` chuỗi | `reason` là **mã** (`contract|dependency-of|leaf|cycle|no-edges|test|doc|generated|overflow`) + `reasonParams` | Nhãn qua `translate()` theo mã; mã lạ hiển thị nguyên văn |
| 3 | `changeKind` per symbol | `ChangedFile.status`, `ChangedSymbol.changeKind` | Ký hiệu A/M/D từ `ChangedFile.status` (file) |
| 4 | `Enter` mở diff tại symbol | `hunks[{startLine,endLine}]` có sẵn | `Enter` mở diff tại `hunks[0].startLine` (nếu không có: dòng đầu symbol đầu) |
| 5 | `reviewState.get {worktreeId, baseCommit, headCommit}`; chưa có bản ghi chưa chốt | `reviewState.get` (sel) `baseCommit?`, `headCommit?`; chưa có ⇒ **mặc định `version:0`** (không lỗi); `save` có `notes?`, `turnMarkers?`, `status?`, `expectedVersion`; `turnMarkers` chỉ ở dòng mức worktree (`baseCommit=''`, `headCommit=''`) | Slice giữ **toàn bộ `ReviewState`** đã tải và luôn gửi lại `notes`/`turnMarkers` đang biết (vì ngữ nghĩa "vắng" của `save` chưa chốt, câu hỏi 1); dòng `(base,head)` không mang `turnMarkers` |
| 6 | Khoá `(base,head)` từ `ChangeOverlay.base/head` | Overlay trả `scope {baseRef, baseOid, mergeBase, headOid, mode, includesUncommitted}` | `baseCommit = scope.mergeBase ?? scope.baseOid`, `headCommit = scope.headOid` (đề xuất; hợp đồng **chưa** nói cặp nào là khoá; câu hỏi 2) |
| 7 | `entries` không giới hạn | `readingProgress` ≤ 64 KiB; `args[0]` ≤ 256 KiB | Cắt tỉa: giữ mọi `seen`, bỏ tombstone `unseen` cũ nhất khi vượt 56 KiB; nếu vẫn vượt ⇒ cảnh báo "Tiến độ quá lớn, một phần không lưu" |
| 8 | Xung đột mã chưa chốt | `CODEINTEL_VERSION_CONFLICT {currentVersion?}` ⇒ `kind:'conflict'` | Tải lại, `mergeReadingProgress`, ghi lại một lần |
| 9 | Dùng `ChangeOverlay.readingOrder` hoặc kênh `readingOrder` | Cả hai tồn tại; `detail:'summary'` trả mảng rỗng; `readingOrder {base?, head?}` trả `{steps, components}` | Dùng overlay `full` nếu có; ngược lại kênh `readingOrder` |

## 2. Hợp đồng áp dụng

`readingOrder` (20 s, quyền `read`), `reviewState.get` (8 s), `reviewState.save` (8 s, `review_write`, args ≤ 256 KiB); `ReviewState.status: 'open'|'reviewed'`; giới hạn: `readingProgress` ≤ 64 KiB, `notes` ≤ 256 KiB/500 mục, `turnMarkers` ≤ 5. `version:0` = tạo; lệch ⇒ `VERSION_CONFLICT`; `mergeReadingProgress` last-writer-wins theo `at`. Lỗi `forbidden` ở save ⇒ chỉ đánh dấu cục bộ.

## 3. Lệch giữa CR và hợp đồng

Bảng mục 1 (9 dòng). Cần chốt với BE: dòng 5 và 6.

## 4. Giải pháp

### 4.1 Cây file

```
components/review-map/ReadingOrderList.tsx ReadingOrderRow.tsx ReadingOrderGroupHeader.tsx ReadingProgressBar.tsx
  reading-order-model.ts reading-progress-merge.ts useRovingListKeys.ts reading-reason-labels.ts
store/slices/review-progress.ts
```

### 4.2 Mô hình hàng

```ts
type ReadingOrderItem = { stepKey: string; n: number; file: string; symbols: SymbolRef[]; hunks: {startLine:number;endLine:number}[]
  reason: ReasonCode | string; reasonParams?: Record<string,string>; dependsOn: string[]; tests: SymbolRef[]; cycleGroup?: string
  layer: string; fileStatus: ChangedFile['status'] | 'unknown'; component: { id: string; label: string } | null }
buildReadingOrderRows(items, {collapsedGroupIds, filter: ReadonlySet<string> | null}) // hàng nhóm + hàng bước
```

Nhóm theo `ComponentGroup`; bước không thuộc nhóm nào vào "Khác" cuối; trùng `stepKey` giữ mục đầu; `filter` (từ `reviewChipPredicate`, khoá theo file) **ẩn** hàng nhưng tiến độ `seen/total` luôn tính trên toàn bộ. `cycleGroup` ⇒ nhãn "vòng phụ thuộc"; `dependsOn` ⇒ "đọc sau bước n". `reason:'overflow'` ⇒ dòng "{shown}/{totalCount}".

### 4.3 Tiến độ và lưu

`reviewProgressByWorktree[worktreeId] = {stateKey, status, serverState: ReviewState | null, progress: ReadingProgress, saveStatus: 'saved'|'dirty'|'saving'|'error', lastErrorKind}`. Ghi lạc quan, debounce 800 ms, tuần tự hoá theo worktree (một đang chạy + một chờ, đọc snapshot mới nhất khi tới lượt), `flush` khi gỡ tab, đổi phạm vi/worktree, `visibilityState==='hidden'`. Xung đột: tải `get`, `mergeReadingProgress(local, remote)` (từng khoá chọn `at` lớn hơn; `lastFocusedKey` từ bên mới hơn), ghi lại. Offline/lỗi: `saveStatus='error'` ("Chưa lưu" + Thử lại), tự thử khi kết nối `established` hoặc 15 s khi cửa sổ hiển thị; **không** hiển thị "Đã lưu" trước khi `save` thành công. `forbidden` ⇒ thông báo chỉ-cục-bộ. Không nhân bản vào `localStorage` (bị xoá khi đăng xuất; storage README). Thêm một dòng vào `feature-persistence-matrix.md`.

### 4.4 Phím (trên `role="listbox"`, bỏ qua `isEditableTarget`)

`j/↓`, `k/↑`, `Home/End`, `Enter` (mở diff, **không** đánh dấu đã xem), `Space` (bật/tắt đã xem, giữ vị trí), `←/→` trên nhóm. Không có phím phụ thuộc nền tảng. `useRovingListKeys` dùng lại ở SOL-056 (danh sách bước). Chip phím ở chân cột chỉ cho phím đã cài. Ảo hoá `useVirtualizer` khi > 150 hàng (28 px nhóm / 40 px bước, `overscan` 8).

## 5. Quyết định thiết kế

Trạng thái bền ở backend (O6); hàng = bước (file) theo hợp đồng; hợp nhất LWW từng mục; `Enter` không tự đánh dấu; tiến độ tính trên toàn bộ danh sách; một slice quản **toàn bộ** `ReviewState` để SOL-060 (nhóm B) tái dùng cùng hàng chờ ghi, tránh hai nơi ghi `save` ghi đè nhau.

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng |
|---|---|---|
| `readingOrder`, `components[]` | `BE-CV-SOL-036-reading-order-and-risk` (đợt 3), `BE-CV-SOL-036-change-overlay-pipeline` | G3 (fake trước) |
| `reviewState.get/save` | `BE-CV-SOL-040-codeintel-write-and-stream-channels`; bảng `review_states`: `BE-CV-SOL-011-data-model-and-migrations`, `BE-CV-SOL-011-repositories-and-maintenance` | G3 (đợt 5; fake trước) |
| Cùng writer với ghi chú | `FE-CV-SOL-060-review-notes-and-turn-compare` (nhóm B) | đồng bộ trước khi cài 052-03 |

## 7. Tiêu chí chấp nhận

- [ ] Hàng theo `n`, nhóm theo component, "Khác" cuối; trùng `stepKey` một lần.
- [ ] `j/k`, `Enter` (diff tại hunk đầu, không đổi trạng thái), `Space` bật/tắt, bỏ qua ô nhập.
- [ ] Tiến độ `seen/total` không đổi khi bật bộ lọc; "Đang lọc k/N".
- [ ] Ghi sau debounce 800 ms với `expectedVersion`; tối đa một ghi chạy + một chờ.
- [ ] Mở lại đúng `(base,head)` khôi phục mục đã xem và `lastFocusedKey`; `head` đổi ⇒ trống, bản ghi cũ còn ở backend.
- [ ] `VERSION_CONFLICT` gộp theo `at`; bỏ đánh dấu mới hơn không bị hồi sinh.
- [ ] Offline/lỗi ghi: "Chưa lưu"; không "Đã lưu" giả; `forbidden` không chặn đánh dấu cục bộ.
- [ ] Vượt 64 KiB: cắt tỉa tombstone, cảnh báo nếu vẫn vượt; `reviewProgressByWorktree` và timer bị dọn ở hai đường xoá worktree.

## 8. Kiểm thử

`reading-order-model.test.ts`, `reading-progress-merge.test.ts`, `review-progress.test.ts` (đồng hồ giả; debounce, tuần tự, flush, conflict, offline, forbidden, giới hạn kích thước), hai test rò rỉ, `ReadingOrderList.test.tsx` (phím, aria, ảo hoá 500 hàng), `ReadingProgressBar.test.tsx`, `useRovingListKeys.test.tsx`, `code-intel-locale-coverage` mở rộng. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

`stepKey` định dạng/độ dài chưa biết (ảnh hưởng 64 KiB: ước ~130 B/mục ⇒ ~500 mục); `at` theo đồng hồ client có thể lệch; `aria-activedescendant` với ảo hoá chưa kiểm; chu trình phụ thuộc hiển thị `cycleGroup` do backend quyết; mang tiến độ qua `head` đổi chưa có (để SOL-060).

## 10. Câu hỏi mở

1. `reviewState.save` bỏ `notes`/`turnMarkers`: giữ nguyên hay xoá? (hợp đồng im lặng; client tạm luôn gửi lại).
2. Cặp `(baseCommit, headCommit)` khoá dòng khi `includesUncommitted`: đề xuất `mergeBase`/`headOid`; BE xác nhận.
3. Giới hạn số mục hay nén `entries` phía backend?
4. Đồng hồ do server cấp cho `at`?

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-052-01](../tasks/FE-CV-TASK-052-01-reading-order-row-model.md) | Mô hình hàng thứ tự đọc | P0 |
| [FE-CV-TASK-052-02](../tasks/FE-CV-TASK-052-02-reading-progress-merge-and-size-guard.md) | Hợp nhất tiến độ và chặn kích thước | P0 |
| [FE-CV-TASK-052-03](../tasks/FE-CV-TASK-052-03-review-progress-slice-load-save-queue.md) | Slice tiến độ, tải/ghi tuần tự | P0 |
| [FE-CV-TASK-052-04](../tasks/FE-CV-TASK-052-04-use-roving-list-keys.md) | `useRovingListKeys` | P0 |
| [FE-CV-TASK-052-05](../tasks/FE-CV-TASK-052-05-reading-order-list-components.md) | `ReadingOrderList` và thành phần con | P0 |
| [FE-CV-TASK-052-06](../tasks/FE-CV-TASK-052-06-reading-order-states-i18n-and-matrix.md) | Trạng thái, i18n, ma trận lưu trữ | P1 |

Thứ tự: 052-01, 052-02, 052-04 song song → 052-03 → 052-05 → 052-06.

## 12. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-052-reading-order-and-review-progress.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/specs/frontend/storage/README.md`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/diff-comments-persist-queue.ts`, `/opt/repos/orca/guides/STYLEGUIDE.md`.

## 13. Ghi chú triển khai (2026-10-07)

- Mô hình/UI: `reading-order-model.ts` (`buildReadingOrderItems/Rows`), `reading-progress-merge.ts`, `reading-reason-labels.ts`, `useRovingListKeys.ts`, `reading-order/{ReadingOrderList,ReadingOrderRow,ReadingOrderGroupHeader,ReadingProgressBar}.tsx`; slice `store/slices/review-progress.ts` (+ `review-progress-entry.ts` cho kiểu, tách để dưới 300 dòng).
- API cho agent 060 (ghi chú): `patchReviewState(worktreeId, patch)` đi cùng hàng chờ ghi, luôn gửi lại `notes`/`turnMarkers` đang biết; `readingProgress` do slice sở hữu (patch bị bỏ qua trường này). Seam test: `setReviewProgressApi(fake)`.
- Cặp khoá `(baseCommit, headCommit)` = `overlay.scope.mergeBase ?? baseOid` / `headOid` (câu hỏi mở 2 vẫn chờ BE).
- Hàng đợi: debounce 800 ms, một lần ghi chạy + vòng lặp ghi lại phần sửa trong lúc chờ, `VERSION_CONFLICT` tối đa 3 vòng (tải lại → `mergeReadingProgress` → ghi), lỗi khác ⇒ `saveStatus:'error'` + thử lại 15 s; `forbidden` ⇒ `localOnly`, không thử lại. Thử lại khi kết nối `established` và flush khi `visibilityState==='hidden'`/gỡ tab nằm ở `useReviewWorkspaceModel`.
- Space trên hàng nhóm đánh dấu cả nhóm; Enter chỉ mở diff (không đánh dấu). Ảo hoá > 150 hàng với `initialRect` cố định; `aria-activedescendant` + ảo hoá chưa kiểm với trình đọc màn hình.
- Chưa có: cờ lớp phủ trên hàng (đợi `review-overlay-model` 053-01); chưa có tin cậy về `stepKey` dài (cắt tỉa 56 KiB đã có test).
