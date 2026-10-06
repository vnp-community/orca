# CR-CV-052 — Thứ tự đọc và tiến độ review: danh sách `readingOrder`, đánh dấu đã xem, nhóm theo component, phím j/k/Enter/Space, lưu qua `reviewState`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-052 |
| **Tên** | Thứ tự đọc và tiến độ review: danh sách `readingOrder` (cột trái), đánh dấu đã xem, nhóm theo component, phím tắt `j`/`k`/`Enter`/`Space`, lưu và khôi phục trạng thái qua `codeIntel.reviewState.get/save` |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (client, slice, kiểu `ReviewState`/`ReadingProgress`), CR-CV-051 (khung, bộ lọc chip, `scopeKey`); backend: CR-CV-036 (`readingOrder`), CR-CV-011 (bảng `review_states`), CR-CV-040 (`reviewState.get/save`) |
| **Mở khoá** | CR-CV-053 (nút "Xem diff" dùng chung), CR-CV-060 (so sánh với lượt trước) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/` (file mới), `store/slices/review-progress.ts` (mới), `store/slices/code-intel.ts` (thêm khoá vào danh sách dọn), `i18n/locales/*.json`, `specs/frontend/storage/feature-persistence-matrix.md` (thêm một dòng) |

---

## 1. Bối cảnh và vấn đề

Cột trái của màn Review là chỗ người dùng bắt đầu đọc: các symbol đã đổi, sắp theo phụ thuộc (callee trước, caller sau; [08 R6](../../../research/view-code/08-views-and-review-models.md)), nhóm theo component, có tiến độ `4/12`, phím `j/k` chuyển mục, `Enter` mở diff, `Space` đánh dấu đã xem ([10 §6.2](../../../research/view-code/10-frontend-review-ux.md)). Trạng thái phải lưu theo `(worktree, commit)` để quay lại không mất.

Hiện trạng đã đọc (2026-10-05):

- **Nơi lưu (O6)**: README v7 chọn backend (`review_states`: `worktree_id`, `base_commit`, `head_commit`, `reading_progress` JSON, `notes` JSON, `status`, `updated_by`, `updated_at`, `version`). Đối chiếu `specs/frontend/storage/*`: **không có IndexedDB, không có middleware `persist` của Zustand** trong renderer; mọi trạng thái bền đi qua `window.api` hoặc `callRuntimeRpc` tới backend (`README.md` mục "Headline findings" 1), `localStorage` chỉ dành cho tuỳ chọn riêng từng thiết bị (`browser-storage-catalog.md`). Phiên tab ở web là `localStorage` cục bộ trình duyệt và bị xoá hàng loạt khi đăng xuất hoặc lỗi xác thực (`dev-server-agent-impact.md` mục 10). Hệ quả: lưu trạng thái đã xem ở `localStorage` sẽ mất khi đăng xuất và không theo người dùng giữa thiết bị, nên đúng với O6.
- **Mẫu ghi có thứ tự**: `store/slices/diff-comments-persist-queue.ts` xếp hàng ghi theo worktree và đọc snapshot mới nhất khi tới lượt. Mẫu này dùng lại cho ghi tiến độ.
- **Dữ liệu `readingOrder`**: README 3.4 liệt kê `readingOrder[]` trong mở rộng của `ChangeOverlay` và có kênh `codeIntel.readingOrder`; **hình dạng phần tử chưa được định nghĩa** ở 05/08 (chỉ: "sắp symbol đã đổi theo phụ thuộc, kèm tóm tắt từng bước"). `SymbolRef` đã có (`key, kind, name, qualifiedName?, filePath, startLine?, endLine?`).
- `isEditableTarget` (`lib/editable-target.ts`) để bỏ qua phím khi đang nhập; `ShortcutKeyCombo` hiển thị chip phím; `ui/checkbox.tsx`, `ui/collapsible.tsx`, `ui/progress.tsx` có sẵn; `@tanstack/react-virtual` đã dùng ở `CsvViewer.tsx`, `WorktreeList.tsx`.
- Ký hiệu trạng thái git (A/M/D) có token `--git-decoration-added|modified|deleted`; theo STYLEGUIDE chỉ dùng cho trạng thái git (đúng trường hợp này), dùng qua `style={{color:'var(--git-decoration-…)'}}` như `SourceControl.tsx:6278`.

## 2. Giải pháp đề xuất

### 2.1 Hợp đồng dữ liệu frontend cần (đề xuất, chờ CR-CV-036)

```ts
type ReadingOrderItem = {
  key: string                    // = symbol.key; khoá của đánh dấu đã xem
  order: number                  // 1..n theo thứ tự đọc
  symbol: SymbolRef
  component: { id: string; name: string } | null   // nhóm; null = "Khác"
  changeKind: 'added' | 'modified' | 'deleted' | 'unknown'
  summary?: string               // tóm tắt một dòng của bước
  reason?: string                // vì sao ở vị trí này (ví dụ "được gọi bởi #3")
}
type ReadingProgress = {
  version: 1
  entries: Record<string /* symbolKey */, { state: 'seen' | 'unseen'; at: number }>
  lastFocusedKey: string | null
}
```

Nguồn: `ChangeOverlay.readingOrder` nếu có (CR-CV-051 đã tải overlay), nếu không thì `useCodeIntelQuery('readingOrder', {scope})`. `ReadingOrderItem` được parse tại `parseReadingOrderItem` (`code-intel-wire-parsers.ts`): enum lạ → `'unknown'`, thiếu `component` → `null`, trùng `key` thì giữ mục đầu.

### 2.2 Cây component (`components/review-map/`, mới)

```
ReadingOrderList                   (props: worktreeId, scope, overlay, chipFilter, onOpenDiff, onSelectSymbol)
├─ ReadingProgressBar              ("4/12", ui/progress, trạng thái lưu)
├─ ReadingOrderFilterNotice        (khi có bộ lọc chip: "Lọc: chưa có test ✕")
├─ [virtual] ReadingOrderGroupHeader  (▾ tên component, 1/3, ô đánh dấu cả nhóm)
└─ [virtual] ReadingOrderRow       (ô tick, số thứ tự, icon kind, tên, A/M/D, tóm tắt, cờ lớp phủ)
ReadingOrderKeyHint                (chân cột: j/k, Enter, Space)
useRovingListKeys                  (hook phím danh sách dùng chung với các lens, vd. DataFlowStepList của CR-CV-056)
```

```
┌ Thứ tự đọc ──────────────── 4/12 ┐
│ ▓▓▓▓░░░░░░░░  33%        Đã lưu ✓ │
│ Lọc: chưa có test ✕               │
├───────────────────────────────────┤
│ ▾ usecase (3)                 1/3 │
│   ☑ 1  ƒ CreateRequest        M   │
│ ▌ ☐ 2  ƒ ValidateInput        A ┄ │   (▌ = đang chọn; ┄ = chưa có test)
│ ▸ adapter/postgres (2)        0/2 │
├───────────────────────────────────┤
│ J K chuyển · Enter mở diff · Space đã xem │
└───────────────────────────────────┘
```

Hàng ảo hoá khi tổng số hàng > 150 (`useVirtualizer`, `estimateSize` 32 px nhóm và 40 px mục, `overscan` 8); ≤ 150 thì vẽ thẳng để giữ trợ năng đơn giản. Toàn bộ danh sách tối đa 2 000 symbol (README 3.2, `detectChanges`).

### 2.3 Mô hình hàng (`reading-order-model.ts`, thuần, có test)

- `buildReadingOrderRows(items, {collapsedGroupIds, filter})`: nhóm theo `component.id` (thứ tự nhóm = `order` nhỏ nhất của nhóm; trong nhóm theo `order`); mục không có component vào nhóm "Khác" cuối cùng; trả mảng phẳng gồm hàng nhóm và hàng mục kèm chỉ số tiến độ `seen/total` của nhóm.
- `filter` là `Set<symbolKey>` từ `reviewChipPredicate` (CR-CV-051): mục không khớp **ẩn** khỏi danh sách (khác với lens, nơi bị làm mờ), nhưng **tiến độ `n/N` luôn tính trên toàn bộ danh sách**, không trên phần đã lọc, để con số không đổi nghĩa khi bấm chip. Dòng thông báo "Đang lọc {k}/{N}" và nút bỏ lọc hiển thị khi có bộ lọc.
- Nhóm thu gọn lưu cục bộ trong state component (không lưu bền).

### 2.4 Trạng thái và lưu (`store/slices/review-progress.ts`, mới)

```
ReviewProgressSlice {
  reviewProgressByWorktree: Record<worktreeId, {
    stateKey: string                     // `${overlay.base}..${overlay.head}` (xem dưới)
    status: 'idle'|'loading'|'ready'|'error'
    serverVersion: number | null
    progress: ReadingProgress
    saveStatus: 'saved'|'dirty'|'saving'|'error'
    lastErrorKind: CodeIntelErrorKind | null
  }>
  loadReviewProgress(worktreeId, base, head)
  setReadingItemSeen(worktreeId, key, seen: boolean) / setReadingGroupSeen(worktreeId, keys, seen)
  setReadingLastFocused(worktreeId, key | null)
  flushReviewProgress(worktreeId): Promise<void>
}
```

- **Khoá trạng thái** `(base, head)` lấy từ `ChangeOverlay.base` và `ChangeOverlay.head` đã được backend phân giải (05 §2.7), **không** tự suy từ phạm vi phía client, để khớp `review_states.base_commit/head_commit`. Khi `head` đổi (agent thêm commit), khoá đổi và trạng thái mới là **trống** (xem 7.2 về mang theo); tiến độ cũ vẫn ở backend.
- `loadReviewProgress`: `codeIntel.reviewState.get {worktreeId, baseCommit, headCommit}`. Không có bản ghi (backend trả rỗng hoặc lỗi không-tìm-thấy, chưa chốt) → tiến độ trống `serverVersion=null`. Lỗi `offline` → `status='error'` nhưng vẫn cho đánh dấu cục bộ, `saveStatus='dirty'`.
- **Ghi**: tối ưu hoá lạc quan (cập nhật cục bộ ngay), gom bằng debounce 800 ms, tuần tự hoá theo worktree (mẫu `diff-comments-persist-queue.ts`, một ghi đang chạy và một ghi chờ; lúc tới lượt đọc snapshot mới nhất). `codeIntel.reviewState.save {worktreeId, baseCommit, headCommit, readingProgress, expectedVersion}`; thành công cập nhật `serverVersion` từ phản hồi. `flushReviewProgress` chạy khi: gỡ tab Review, đổi phạm vi, đổi worktree, `document.visibilityState==='hidden'`.
- **Xung đột** (`kind:'conflict'`, mã chưa chốt, CR-CV-050 7.3): tải lại, hợp nhất bằng `mergeReadingProgress(local, remote)` (`reading-progress-merge.ts`, thuần): với từng khoá chọn bản có `at` lớn hơn (last-writer-wins từng mục; `unseen` có dấu thời gian nên bỏ đánh dấu không bị hồi sinh), `lastFocusedKey` lấy từ bên mới hơn; ghi lại một lần. Hai người cùng review một worktree sẽ không ghi đè nhau.
- **Ngoại tuyến hoặc lỗi ghi**: giữ `saveStatus='error'` và hiển thị "Chưa lưu" kèm "Thử lại" trên `ReadingProgressBar`; tự thử lại khi kết nối `established` (`connectivity-status.ts`) hoặc mỗi 15 s khi cửa sổ hiển thị. **Không** khẳng định "Đã lưu" cho tới khi `save` trả thành công.
- **Quyền**: `forbidden` ở `save` → `saveStatus='error'`, thông báo "Bạn không có quyền lưu tiến độ", đánh dấu vẫn dùng được cục bộ cho phiên.
- Khoá `reviewProgressByWorktree` thêm vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (CR-CV-050); timer và hàng chờ ghi theo worktree (module-level) bị huỷ khi dọn. Có test rò rỉ qua cả hai đường xoá worktree.
- Không nhân bản vào `localStorage`: sẽ có hai nguồn sự thật và bị xoá khi đăng xuất. Thêm một dòng vào `feature-persistence-matrix.md` (slice `review-progress.ts`: bền qua `codeIntel.reviewState.*`).

### 2.5 Phím tắt

Xử lý trên **phần tử danh sách có `role="listbox"`** (tiêu điểm lăn: một `tabIndex=0` cho mục đang chọn, `aria-activedescendant` hoặc roving tabindex), không bắt phím toàn cục, nên không chạm terminal hay editor. Bỏ qua khi `isEditableTarget(event.target)`.

| Phím | Hành vi |
|---|---|
| `j` hoặc `ArrowDown` | Mục tiếp theo (bỏ qua hàng nhóm thu gọn; nhảy qua hàng nhóm khi mở rộng) |
| `k` hoặc `ArrowUp` | Mục trước |
| `Enter` | Gọi `onOpenDiff(symbol)` (CR-CV-053); **không** tự đánh dấu đã xem (không khẳng định đã đọc khi mới mở) |
| `Space` | Bật/tắt đã xem của mục; giữ vị trí (không tự nhảy) |
| `Home`/`End` | Mục đầu/cuối |
| `ArrowLeft`/`ArrowRight` trên hàng nhóm | Thu gọn/mở rộng |

Không có phím phụ thuộc nền tảng (không dùng `metaKey`/`ctrlKey`), nên không cần chọn theo `navigator.userAgent`. Chip phím ở chân cột hiển thị bằng `ShortcutKeyCombo` vì các phím đã được cài đặt thật. Chọn mục gọi `onSelectSymbol(symbol)` (drawer chi tiết cập nhật theo) và `setReadingLastFocused`. Tiêu điểm mặc định khi mở có chủ đích là mục `lastFocusedKey` (nếu còn) hoặc mục chưa xem đầu tiên (CR-CV-051 2.2).

Tương tác chuột: bấm hàng chọn mục, bấm ô tick đánh dấu, bấm đúp hoặc nút `Xem diff` ở hàng đang chọn mở diff. Mọi ô tick có `aria-label` ("Đánh dấu đã xem {tên}"), hàng có `aria-selected`, số tiến độ có `aria-live="polite"` (không đọc lại mỗi lần gõ phím, chỉ khi giá trị đổi sau 500 ms).

### 2.6 Lớp phủ nhất quán trên hàng

Mỗi hàng hiển thị cờ lớp phủ cùng bảng mã hoá ở CR-CV-053 mục 2.5 (đổi, bị ảnh hưởng, chưa test, vi phạm): ở cột này chỉ dùng **chấm/icon nhỏ kèm tooltip**, nguồn từ `ChangeOverlay` (`uncoveredSymbols`, `violations`). Ký hiệu A/M/D theo `changeKind` dùng token `--git-decoration-*`. Không dùng màu duy nhất để phân biệt (luôn có chữ hoặc icon).

### 2.7 Trạng thái và lỗi

| Tình huống | Hiển thị |
|---|---|
| Đang tải `readingOrder` | `Skeleton` 8 hàng (theo thang thời lượng của CR-CV-051) |
| Danh sách rỗng | "Không có symbol đã đổi trong phạm vi này"; nếu `changedFiles` > 0 mà `readingOrder` rỗng thì "Không dựng được thứ tự đọc từ index; thay đổi nằm ở file không có symbol" (không khẳng định nguyên nhân, chỉ nêu điều quan sát được) |
| `truncated` | Dòng "Đang hiển thị {shown}/{total}" cuối danh sách (CR-CV-051 banner) |
| Tải `reviewState` lỗi | Dòng nhỏ trên thanh tiến độ "Không tải được tiến độ đã lưu" + Thử lại; danh sách vẫn dùng được |
| Lưu lỗi | Xem 2.4 |

## 3. Quyết định thiết kế

- **Trạng thái đã xem ở backend** (O6), khoá `(base, head)` do backend phân giải; frontend chỉ giữ bản lạc quan trong slice.
- **Hợp nhất last-writer-wins theo từng mục** với dấu thời gian, để hai người review không ghi đè nhau và bỏ đánh dấu không hồi sinh.
- **`Enter` không đánh dấu đã xem**: "đã xem" là hành động có chủ đích; tránh nói điều chưa chắc.
- **Tiến độ tính trên toàn bộ danh sách**, bộ lọc chỉ ẩn hàng.
- **Phím xử lý trên danh sách, không toàn cục**, và không phụ thuộc nền tảng.
- **Không nhân bản vào `localStorage`**.
- Hàng ảo hoá chỉ khi > 150 để giữ mã đơn giản ở trường hợp thường gặp.

## 4. Tiêu chí chấp nhận

- [ ] Danh sách hiển thị đúng thứ tự `order`, nhóm theo `component`, mục không có component vào nhóm "Khác" cuối; trùng `key` chỉ hiển thị một lần.
- [ ] `j`/`k`/`↑`/`↓` chuyển mục; `Enter` gọi `onOpenDiff` đúng symbol mà không đổi trạng thái đã xem; `Space` bật/tắt đã xem và không nhảy mục; phím bị bỏ qua khi tiêu điểm ở ô nhập.
- [ ] Thanh tiến độ hiển thị `seen/total` trên toàn danh sách và không đổi khi bật bộ lọc chip; bộ lọc ẩn hàng và hiển thị "Đang lọc k/N".
- [ ] Đánh dấu một mục gọi `reviewState.save` sau debounce 800 ms với `expectedVersion`; nhiều thao tác nhanh chỉ tạo tối đa một ghi đang chạy và một ghi chờ; kết quả cuối khớp trạng thái cuối.
- [ ] Mở lại màn Review của cùng `(base, head)` khôi phục đúng các mục đã xem và `lastFocusedKey`; `head` đổi thì tiến độ trống mà không làm mất bản ghi cũ ở backend.
- [ ] Xung đột phiên bản: hai bản cục bộ và từ xa được hợp nhất theo `at` từng mục; một mục bỏ đánh dấu mới hơn không bị bản cũ hơn hồi sinh.
- [ ] Ngoại tuyến hoặc lỗi ghi hiển thị "Chưa lưu", không bao giờ hiển thị "Đã lưu" trước khi `save` thành công; tự thử lại khi kết nối trở lại.
- [ ] `forbidden` khi lưu không chặn đánh dấu cục bộ.
- [ ] Hơn 150 hàng dùng ảo hoá; di chuyển bằng `j`/`k` cuộn mục đang chọn vào khung nhìn.
- [ ] Khoá `reviewProgressByWorktree` và timer bị dọn khi xoá worktree bằng cả hai đường.
- [ ] Không hex, không `metaKey`; chuỗi qua `translate()` đủ 5 locale; hoạt động ở Electron và web.

## 5. Kiểm thử

- `reading-order-model.test.ts`: nhóm, thứ tự nhóm, nhóm "Khác", trùng khoá, lọc, thu gọn, tiến độ nhóm.
- `reading-progress-merge.test.ts`: LWW từng mục, `unseen` mới thắng `seen` cũ, `lastFocusedKey`.
- `store/slices/review-progress.test.ts` (đồng hồ giả, `vi.mock` client như `mcp-slice.test.ts`): debounce, tuần tự hoá, flush khi đổi phạm vi, xung đột → gộp → ghi lại, offline → `dirty` → thử lại, `forbidden`.
- `review-progress-worktree-removal-leak.test.ts` và `review-progress-bulk-purge-leak.test.ts` (mẫu CR-CV-050 mục 5).
- `ReadingOrderList.test.tsx` (`// @vitest-environment happy-dom`): phím `j/k/Enter/Space`, bỏ qua khi tiêu điểm ở `input`, `aria-selected`, thu gọn nhóm bằng phím mũi tên, ảo hoá với 500 mục (kiểm chỉ một phần hàng được vẽ).
- `ReadingProgressBar.test.tsx`: "Chưa lưu" so với "Đã lưu" theo `saveStatus`.
- `i18n/code-intel-locale-coverage.test.ts` mở rộng.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Hình dạng `readingOrder[]` và thuật toán sắp xếp** chưa tồn tại; toàn bộ CR phụ thuộc đề xuất ở 2.1. Chu trình phụ thuộc (đệ quy lẫn nhau) khiến thứ tự "callee trước" không xác định; backend phải giải quyết, frontend chỉ hiển thị `order`.
- **Hợp đồng `reviewState.get/save`**: tên mã lỗi cho xung đột, hành vi khi chưa có bản ghi, giới hạn kích thước `reading_progress` (một review có thể có 2 000 mục; `entries` có thể vài trăm KiB) chưa kiểm chứng.
- `at` do **đồng hồ client** quyết định thứ tự LWW; lệch đồng hồ giữa hai thiết bị có thể đảo thứ tự (chấp nhận được cho một đánh dấu đã xem; có thể thay bằng đồng hồ do server cấp nếu backend hỗ trợ).
- Chưa kiểm chứng `aria-activedescendant` với ảo hoá của `@tanstack/react-virtual` ở trình đọc màn hình.
- `ChangeOverlay.base/head` có được trả đầy đủ không (05 §2.7 có, nhưng README 3.2 chỉ mô tả phần đầu chung) chưa kiểm chứng.

## 7. Câu hỏi mở

1. Hình dạng chính xác của `ReadingOrderItem` (đặc biệt `component`, `summary`, `reason`) và cách xử lý vòng phụ thuộc: CR-CV-036.
2. Khi `head` đổi, có **mang theo** các mục đã xem cho symbol không đổi không (cần băm nội dung hoặc `contentHash` mỗi symbol)? Đề xuất để CR-CV-060 ("so sánh với lượt trước") quyết định; CR này bắt đầu trống.
3. Lỗi và dạng trả khi chưa có `review_states` (rỗng hay mã không-tìm-thấy).
4. Có cần một đồng hồ do server cấp cho `at` không?
5. Có nên cho tự đánh dấu đã xem khi người dùng ở lại một diff đủ lâu (tuỳ chọn)? Đề xuất không ở MVP.
6. Giới hạn kích thước `reading_progress` phía backend để không từ chối lưu một review rất lớn.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O6, 3.4, 3.5, 3.7), `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§6.2, §6.8, §8, §10 mục 2), `08-views-and-review-models.md` (R6), `05-graph-schemas.md` (§2.7, §3)
- `/opt/repos/orca/specs/frontend/storage/{README,browser-storage-catalog,feature-persistence-matrix,dev-server-agent-impact}.md`
- `/opt/repos/orca/guides/STYLEGUIDE.md` (Keyboard shortcut chips, Screen UX review rubric: default focus, keyboard navigation)
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/diff-comments-persist-queue.ts`, `store/slices/connectivity-status.ts`, `store/slices/code-intel.ts` (CR-CV-050)
- `/opt/repos/orca/frontend/src/renderer/src/lib/editable-target.ts`, `components/ShortcutKeyCombo.tsx`, `components/ui/{checkbox,collapsible,progress,skeleton}.tsx`, `components/editor/CsvViewer.tsx` (mẫu `useVirtualizer`), `components/right-sidebar/SourceControl.tsx` (:6278, mẫu token git)
- Mới: `components/review-map/{ReadingOrderList,ReadingOrderRow,ReadingOrderGroupHeader,ReadingProgressBar}.tsx`, `components/review-map/useRovingListKeys.ts`, `reading-order-model.ts`, `reading-progress-merge.ts`, `store/slices/review-progress.ts`
