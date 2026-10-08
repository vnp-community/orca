# FE-CV-SOL-053-impact-lens-and-symbol-detail: Lens Ảnh hưởng, panel chi tiết symbol, liên kết hai chiều với diff, mã hoá lớp phủ

> 🚧 Triển khai 2026-10-07: 7/8 task DONE (053-01..07; W6: 053-04 đã có dòng index cũ), 1 PARTIAL (053-08 thiếu e2e). Test: impact/*, review-overlay-model, review-diff-navigation, diff-cursor-line-bus, use-diff-line-reveal, i18n coverage đều xanh. Còn mở: kiểm tay `var()` trong SVG xyflow, O-13.

**CR:** [CR-CV-053](../../../../../../docs/crs/v7/review-frontend/CR-CV-053-impact-lens-and-symbol-detail.md)
**Area:** frontend (`components/review-map`, `components/editor`, `store/slices/editor.ts`, `lib`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (§3.1 `impact`, `symbol`, `subgraph`; §4.2 `ImpactGraph`, `SymbolDetail`, `SymbolRef`; §4.3 `ChangeOverlay`, `ViolationRef`; §2.3 `AMBIGUOUS_SYMBOL`, `SYMBOL_NOT_FOUND`, `PATH_NOT_ALLOWED`, `RESPONSE_TOO_LARGE`; U9), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-09, PQ-19(5), PQ-20, PQ-32; O-3, O-13).
**TDD tham chiếu:** [v5/08-editor-and-files §2 Editor Slice, §3 Editor Component, §6 Source Control, §7 Diff Comments](../../../../tdd/v5/08-editor-and-files.md), [v5/15-task-graph-ui §1 TaskGraph](../../../../tdd/v5/15-task-graph-ui.md) (mẫu xyflow), [v5/05-ui-components §6](../../../../tdd/v5/05-ui-components.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `store/slices/editor.ts` (:528 `openDiff`, :536 `openBranchDiff`, :696, :740-741 `pendingEditorReveal`, :2538, :2627), `components/editor/DiffViewer.tsx` (488 dòng; :55, :100-105, :133, :196-245 cuộn tới ghi chú/thay đổi đầu tiên), `diff-viewer-props.ts` (tên), `EditorContent.tsx` (1025 dòng), `check-annotation-open.ts` (`openAnnotationLocation`, `cancelAnnotationRevealFrame`), `lib/emulator-right-split-target.ts`, `components/task/TaskDAGView.tsx`/`DAGPreview.tsx` (hex, không dùng), `shared/cross-platform-path.ts` (tên), `shared/types.ts:3793`.

Xác nhận: `DiffViewer` chưa có API cuộn tới dòng tuỳ ý; `pendingEditorReveal` chỉ cho chế độ sửa; `CombinedDiffViewer` (2146 dòng theo CR) ngoài phạm vi.

**Correction relative to CR-CV-053 (hợp đồng thắng):**

| # | CR-053 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `impact` nhận `target: key` | `target = {key} \| {name, file?, kind?}`, `direction` (upstream mặc định), `depth` 1..3 (2), `includeTests?`; **ngoài khoảng bị từ chối, không kẹp** | Client kiểm khoảng trước khi gửi |
| 2 | Có thể có `from`/`edges` | PQ-19(5): `ImpactGraph` **không có cạnh ở v7** (hạn chế đã biết); `levels:[{depth, symbols:[{symbol, via: string, confidence?, direct}]}]`; `affectedFlows[{flowId,label,stepCount,changedStep?}]`, `affectedClusters[{id|null,label,hits,impact}]`, `testsCovering: SymbolRef[]`, `risk` có `UNKNOWN` | Nhánh "không cạnh" là **đường duy nhất**: cột theo tầng; chỉ vẽ cạnh nút `direct:true` ↔ tâm; dòng giải thích luôn hiển thị; không suy diễn cạnh giữa tầng 2+ |
| 3 | Panel dùng `symbol` + `subgraph depth 1` | `SymbolDetail {symbol, incoming: Record<kind, {uid?,name,filePath}[]>, outgoing, flows[{id,label,stepCount,step}], source|null, sourceOmitted}`; `includeSource` **mặc định `true`**; `symbol` cần quyền `read_source`, ≤ 320 KiB | Một lời gọi `symbol {includeSource:false}` cho gọi bởi/gọi tới/luồng; mã nguồn chỉ lấy khi mở mục (`includeSource:true`); **không** gọi `subgraph` ở MVP |
| 4 | Test phủ từ `impact` riêng | `impact … includeTests:true` ⇒ `testsCovering` | Tải lười khi mở mục "Test phủ" |
| 5 | Mục `incoming` có `uid` | Chỉ `uid?`, `name`, `filePath` (không `key`, không dòng) | Điều hướng giữa symbol dùng `{name, file}`; `uid` chỉ hiển thị khi có; `ambiguous` → hộp chọn |
| 6 | Cờ `violation` theo symbol | `ViolationRef` theo **file** (`file`, `status touched|introduced`) | Cờ `violation` áp theo file chứa symbol; tooltip nêu `rule` |
| 7 | Cờ `untested` từ `uncoveredSymbols` | `ChangedSymbol.tested: 'yes'|'no'|'unknown'` và `uncoveredSymbols` | `untested` khi `tested==='no'`/trong `uncoveredSymbols`; `unknown` hiển thị "Chưa rõ", không nét đứt |
| 8 | Dòng index có thể lệch ±1 | PQ-20: agent chuẩn hoá, dòng 1-based, `lineBase:1` | Không cộng/trừ ở client; vẫn nhắc "theo index tại {commit}" khi index cũ/`OVERLAY` |
| 9 | `SymbolRef.filePath` tương đối gốc repo | `CODEINTEL_PATH_NOT_ALLOWED` nếu `symbol.file` không an toàn | Client cũng chặn `..`, tuyệt đối, `\` trước khi gửi/mở (`resolveAnnotationPathInsideWorktree`) |
| 10 | Lỗi chỉ theo kind CR-050 | `CODEINTEL_NOT_AUTHORIZED` ở `symbol` (thiếu `read_source`) | Ẩn mục "Mã nguồn", phần còn lại dùng được |

## 2. Hợp đồng áp dụng

`impact` (20 s), `symbol` (20 s, `read_source`), `subgraph` (không dùng ở MVP; khai báo ở 050-02), `changeOverlay` dữ liệu lớp phủ; `Env<ImpactGraph>.truncated/totalCount` ⇒ banner; U9 (chuỗi từ backend là văn bản thuần).

## 3. Lệch giữa CR và hợp đồng

Bảng mục 1 (10 dòng). Hệ quả lớn: lens Ảnh hưởng ở v7 là **danh sách cột theo tầng có thêm vẽ**; giá trị của `@xyflow/react` giảm (xem câu hỏi 1).

## 4. Giải pháp

### 4.1 Cây file

```
components/review-map/ImpactLens.tsx ImpactToolbar.tsx ImpactGraphCanvas.tsx ImpactSymbolNode.tsx ImpactColumnsList.tsx
  SymbolDetailPanel.tsx ReviewOverlayLegend.tsx (+ SymbolRelationList, SymbolCoveringTests, SymbolRelatedFlows, SymbolSourcePreview)
  review-overlay-model.ts impact-column-layout.ts symbol-line-index.ts
lib/review-diff-navigation.ts lib/diff-cursor-line-bus.ts
components/editor/use-diff-line-reveal.ts useDiffCursorEmitter.ts   (mới); DiffViewer.tsx, diff-viewer-props.ts, EditorContent.tsx (sửa)
store/slices/editor.ts (thêm pendingDiffReveal); store/slices/review-ui.ts (thêm impactFocusKey)
```

### 4.2 Lens

Tâm = `impactFocusKey` (mặc định = `selectedSymbolKey` nếu thuộc `changedSymbols`); chưa có tâm ⇒ rỗng "Chọn một symbol đã đổi". Hai truy vấn song song theo hướng đang bật (`useCodeIntelQuery('impact', {target:{key}, direction, depth, includeTests:false})`), không tự gọi cho mọi symbol đã đổi. Bố cục `layoutImpactColumns(upstream, downstream, {rowHeight:56, columnWidth:260, columnGap:80, maxPerColumn:60})`: cột `-d` hướng ngược, `0` tâm, `+d` hướng xuôi; trong cột sắp theo thư mục rồi tên; cột > 60 nút ⇒ "+N khác". Canvas: `nodesDraggable=false`, `onlyRenderVisibleElements`, `fitView` (`duration:0` khi reduced-motion), `MiniMap` chỉ > 80 nút; cạnh chỉ giữa tâm và nút `direct`; danh sách thay thế là mặc định khi > 300 nút hoặc người dùng chọn; thanh công cụ: hướng, độ sâu 1-3, Đồ thị|Danh sách, chú giải. `risk` của `ImpactGraph` hiển thị (có `UNKNOWN` ⇒ "Chưa đủ dữ liệu").

### 4.3 Mã hoá lớp phủ

`OVERLAY_ENCODING` là bảng duy nhất (`changed`: viền đặc 2 px + `CircleDot`; `affected`: viền 1 px; `untested`: nét đứt; `violation`: `TriangleAlert`), màu qua `--review-*` (SOL-050-review-tab-wiring). `computeOverlayFlags(symbolKey|file, overlay, impact)`; `overlayClassNames`, `overlaySvgProps`; chú giải dựng từ cùng bảng, chỉ cờ có dữ liệu. `affected` = nút trong kết quả `impact` không thuộc `changedSymbols`.

### 4.4 Panel chi tiết

`symbol {key | name+file, includeSource:false}` ⇒ đầu panel (tên, `kind`, `qualifiedName`, `filePath:startLine-endLine`, `signature`, `isExported`), "Gọi bởi"/"Gọi tới" (gộp theo kind cạnh, ≤ 20 + xem thêm), "Luồng liên quan" (`flows`, nhấp ⇒ `setReviewLens('dataflow')` kèm nhãn; xem SOL-056 câu hỏi id), "Test phủ" (tải lười, `impact includeTests`), "Mã nguồn" (tải khi mở, `includeSource:true`, ≤ 200 dòng, không tô cú pháp, bỏ khỏi cache khi đóng; `sourceOmitted` ⇒ nhãn: `gitignored|binary|sensitive_path|not_requested`). Khe `SymbolDetailActionsSlot` cho SOL-060.

### 4.5 Liên kết diff

`openReviewDiffAtSymbol(worktreeId, ref, scope)`: `resolveAnnotationPathInsideWorktree`; chọn bộ mở theo nơi thay đổi (`openBranchDiff` nếu trong `gitBranchChangesByWorktree`, `openDiff(staged:false)` nếu chỉ chưa commit, `range/hostedReview` ⇒ compare tổng hợp **chưa kiểm chứng** (O-13), không có trong thay đổi ⇒ mở file hiện tại và ghi chú); sau hai `requestAnimationFrame` đặt `pendingDiffReveal {fileId, line, side:'modified', nonce}`. `DiffViewer` nhận `revealLine/revealSide/revealNonce` qua `diff-viewer-props.ts`, `use-diff-line-reveal.ts` gọi `revealLineInCenter` khi Monaco đã gắn và nhường cuộn tự động (:166-245). Diff → đồ thị: `diff-cursor-line-bus.ts` (phát chỉ khi có người nghe, debounce 150 ms, `suppressDiffCursorUntil` 500 ms), `findInnermostSymbolAtLine` chọn khoảng hẹp nhất; làm nổi nút, **không** mở drawer, không giành tiêu điểm. `CombinedDiffViewer` ngoài phạm vi.

## 5. Quyết định thiết kế

Một tâm mỗi lần; không suy diễn cạnh; danh sách thay thế luôn có; mã hoá = màu + dấu hình học, một nguồn; diff một file; `pendingDiffReveal` mô phỏng `pendingEditorReveal`; bus con trỏ chỉ phát khi có người nghe; không thêm thư viện; mã nguồn symbol là văn bản thuần (U9, không giải mã/ghi log).

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng |
|---|---|---|
| `impact`, `symbol`, `changeOverlay` | `BE-CV-SOL-040-codeintel-view-channels`; model `BE-CV-SOL-020-canonical-graph-model`; agent `AG-CV-SOL-002-gitnexus-extraction`, `AG-CV-SOL-005-detect-changes`; `BE-CV-SOL-036-change-overlay-pipeline` | G3 (fake G4 trước) |
| `violations` | `BE-CV-SOL-037-structure-findings-and-dismissals` | đợt 5 (cờ vắng thì không hiển thị) |
| Cạnh `ImpactGraph` | chưa có (PQ-19); đề xuất BE thêm `edges` | v7+ |
| Chỉnh editor/diff | nội bộ frontend; chạy toàn bộ test diff hiện có | — |

## 7. Tiêu chí chấp nhận

- [ ] Chọn symbol đã đổi ⇒ gọi `impact` đúng hướng/độ sâu; `depth` ngoài 1..3 không được gửi.
- [ ] Không cạnh: cột theo tầng, đường nối chỉ tâm↔direct, dòng giải thích; không đường nối giả.
- [ ] Cột > 60 nút có "+N khác"; > 300 nút mặc định danh sách.
- [ ] Mã hoá lớp phủ đúng bảng ở nút, hàng, chú giải; không dùng màu làm dấu duy nhất; `tested:'unknown'` không nét đứt.
- [ ] Panel: một lời gọi `symbol` với `includeSource:false`; mã nguồn chỉ khi mở mục; thiếu quyền ⇒ ẩn mục mã.
- [ ] "Xem diff" cuộn đúng dòng kể cả Monaco mount sau; "Mở trong editor" không thoát worktree.
- [ ] Con trỏ diff làm nổi nút, không mở drawer, không vòng lặp; không listener thì `DiffViewer` không phát.
- [ ] Không hex; `translate()` 5 locale; `fitView` không hoạt ảnh khi reduced-motion; test diff cũ vẫn xanh.

## 8. Kiểm thử

`impact-column-layout.test.ts`, `review-overlay-model.test.ts` (không hex trong class/props), `symbol-line-index.test.ts`, `review-diff-navigation.test.ts`, `use-diff-line-reveal.test.ts`, `diff-cursor-line-bus.test.ts`, `ImpactLens.test.tsx`, `SymbolDetailPanel.test.tsx`, `ReviewOverlayLegend.test.tsx` (mock `@xyflow/react` như `TaskDAGView.test.tsx`), `code-intel-locale-coverage` mở rộng. Kiểm tay `var()` trong SVG xyflow sáng/tối. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

Không cạnh ⇒ lens kém hữu ích (chặn chính); xyflow áp dụng `var()` cho `stroke`/`MiniMap.nodeColor` chưa kiểm; sửa `DiffViewer`/`EditorContent` dễ hồi quy; compare tổng hợp cho `range/hostedReview` chưa kiểm; hai `impact` song song qua SSH chậm; xung đột phím trong nút xyflow.

## 10. Câu hỏi mở

1. Với `ImpactGraph` không cạnh, có nên bỏ `@xyflow/react` ở lens này và chỉ dùng danh sách cột có đường nối CSS? (đề xuất: giữ canvas P1, danh sách là mặc định ở P0).
2. BE có thêm `edges` hoặc `from` cho `ImpactSymbol` (đề xuất)? Hoặc dùng `subgraph` để làm giàu cạnh (thêm 1 truy vấn)?
3. `FlowSummary.id`/`affectedFlows[].flowId` có cùng không gian id với `DataFlow.id` không (xem SOL-056)?
4. `uid` → `key`: BE có thêm `key` vào `incoming/outgoing` entries?
5. Có tự tạo split bên phải khi mở diff? Mặc định không.

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-053-01](../tasks/FE-CV-TASK-053-01-review-overlay-model-and-legend.md) | Mô hình mã hoá lớp phủ và chú giải | P0 |
| [FE-CV-TASK-053-02](../tasks/FE-CV-TASK-053-02-impact-column-layout.md) | Bố cục cột theo tầng | P0 |
| [FE-CV-TASK-053-03](../tasks/FE-CV-TASK-053-03-impact-lens-canvas-list-toolbar.md) | `ImpactLens`, canvas, danh sách, thanh công cụ | P0 |
| [FE-CV-TASK-053-04](../tasks/FE-CV-TASK-053-04-symbol-detail-data-and-panel.md) | Dữ liệu và panel chi tiết symbol | P0 |
| [FE-CV-TASK-053-05](../tasks/FE-CV-TASK-053-05-pending-diff-reveal-in-diff-viewer.md) | `pendingDiffReveal` và cuộn dòng ở `DiffViewer` | P0 |
| [FE-CV-TASK-053-06](../tasks/FE-CV-TASK-053-06-review-diff-navigation.md) | `openReviewDiffAtSymbol` và mở trong editor | P0 |
| [FE-CV-TASK-053-07](../tasks/FE-CV-TASK-053-07-diff-cursor-bus-and-symbol-line-index.md) | Diff → đồ thị: bus con trỏ, chỉ mục dòng | P1 |
| [FE-CV-TASK-053-08](../tasks/FE-CV-TASK-053-08-impact-lens-registration-i18n-e2e.md) | Đăng ký lens, i18n, e2e | P1 |

Thứ tự: 053-01 → 053-02 → 053-03; 053-04; 053-05 → 053-06 → 053-07 → 053-08.

## 12. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-053-impact-lens-and-symbol-detail.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/frontend/src/renderer/src/components/editor/{DiffViewer.tsx,EditorContent.tsx,check-annotation-open.ts}`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/editor.ts`, `/opt/repos/orca/guides/STYLEGUIDE.md`.

## 13. Ghi chú triển khai (2026-10-07)

Xem ghi chú cuối `tasks/FE-CV-TASK-053-08-*.md` (sai lệch so với spec).
