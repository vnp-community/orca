# FE-CV-SOL-054-structure-lens: Lens Cấu trúc (treemap SVG squarified + cây thư mục ảo hoá)

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06; chưa chạy test hay ứng dụng.

**CR:** [CR-CV-054](../../../../../../docs/crs/v7/review-frontend/CR-CV-054-structure-lens.md)
**Area:** frontend (`components/review-map`, `hooks`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (§3.1 `structure`; §4.2 `ModuleNode/ModuleEdge/ModuleGraph`; §4.3 `ChangedFile.area`; §2.4 phân trang, `depth` 1..3 ngoài khoảng bị từ chối; §2.3 `PATH_NOT_ALLOWED`, `RESPONSE_TOO_LARGE`), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-12, PQ-14, PQ-20; §8.2 CR-054 chỉ FE).
**TDD tham chiếu:** [v5/17-file-explorer-ui §3 FileTreeNode, §Addendum Lazy Expand](../../../../tdd/v5/17-file-explorer-ui.md) (mẫu cây tải theo thư mục), [v5/05-ui-components §6](../../../../tdd/v5/05-ui-components.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `components/status-bar/workspace-space-layout.ts` (124 dòng; `buildTreemapLayout(items)` chia đôi cân bằng đệ quy, bounds 0..100, **không** squarified), `components/status-bar/WorkspaceSpaceManagerPanel.tsx:283,770` (`getTreemapFill`, `WorkspaceTreemap`), `package.json` (`@tanstack/react-virtual` ^3.13.24; không `d3-*`/`elkjs`), `components/ui/*`. Mẫu `useVirtualizer`: `editor/CsvViewer.tsx`, `sidebar/WorktreeList.tsx` (theo CR).

**Correction relative to CR-CV-054:**

| # | CR-054 ghi | Hợp đồng / mã thật | Quyết định |
|---|---|---|---|
| 1 | "Không có mã treemap nào" | Có treemap riêng cho analyzer dung lượng (`workspace-space-layout.ts`; README v7 §8 mục 28 đã ghi) | Vẫn viết `layoutSquarifiedTreemap` mới (thuật toán, tham số `rect`/`padding`, kết quả khác); **không** sửa file cũ; task 054-01 so sánh tỷ lệ cạnh, nếu hàm cũ đủ thì dùng lại (câu hỏi 3) |
| 2 | `area` suy từ đường dẫn | `ModuleNode.area?` có trong §4.2; `ChangedFile.area: string` luôn có | Ưu tiên `ModuleNode.area`, rồi `ChangedFile.area`, cuối cùng tiền tố đường dẫn |
| 3 | `structure {path, depth}` chưa chắc | §3.1: `path?` (tương đối), `depth?` 1..3 (mặc định 2), `limit?`, `pageToken?`; **ngoài khoảng bị từ chối** | Gốc `depth:2`, mở thư mục `depth:1`; phân trang bằng `nextPageToken` (hook `useCodeIntelPagedQuery`) thay cho "tải thêm" tự đoán |
| 4 | `symbolCount` thư mục có phải tổng cây con? | Hợp đồng im lặng (`ModuleNode.symbolCount` số nguyên) | Coi là giá trị của chính nút; tổng = cộng các con đã tải, hiển thị "≥" khi chưa tải đủ (câu hỏi 1) |
| 5 | `truncated` theo thư mục | Envelope có `truncated`, `totalCount`, `nextPageToken` cho mỗi lời gọi | Mỗi thư mục lưu `{nextPageToken, totalCount}` riêng; hiển thị "{shown}/{total}" |
| 6 | `loc` tuỳ chọn | `loc?` | Giữ: "Dòng" vô hiệu khi thiếu |
| 7 | Chuẩn hoá đường dẫn Windows | `ModuleNode.id` tương đối gốc repo (đường dẫn POSIX) | So khớp với `ChangedFile.path` sau `normalizeRuntimePathSeparators` |
| 8 | Phản hồi lớn | `RESPONSE_TOO_LARGE {bytes,limit}` ⇒ `too-large` | Gợi ý mở thư mục con nhỏ hơn; không tự tăng `limit` |
| 9 | `color-mix(in srgb, …)` trong `fill` | tương tự CR | Chưa kiểm chứng Chromium Electron; dùng thuộc tính `style` |

## 2. Hợp đồng áp dụng

`structure` (20 s, quyền `read`, `Env<ModuleGraph>`); `ModuleEdge (imports)` **không dùng**; `ChangeOverlay.changedFiles/uncoveredSymbols/violations` cho lớp phủ; `limits.truncated`.

## 3. Lệch giữa CR và hợp đồng

Bảng mục 1 (9 dòng); không xung đột lớn.

## 4. Giải pháp

### 4.1 Cây file

```
components/review-map/StructureLens.tsx StructureToolbar.tsx StructureTreemap.tsx StructureTree.tsx StructureFileDetail.tsx
  structure-treemap-layout.ts structure-area-model.ts structure-changed-index.ts structure-tree-model.ts
hooks/useStructureTree.ts
```

### 4.2 Dữ liệu

`useStructureTree(worktreeId, scopeKeyIndex)`: gốc `structure {path:'', depth:2}`; mở thư mục `structure {path, depth:1}` (một lần, loại trùng theo `path`, ≤ 2 truy vấn đồng thời, huỷ khi gỡ); tổng nút đã tải ≤ 5 000 (vượt: dừng + thông báo). Cache theo commit của index (`headCommit` phong bì) chứ không theo phạm vi review. `path` chỉ chuỗi tương đối, không `..`.

### 4.3 Treemap

`layoutSquarifiedTreemap(items, rect, {padding})` thuần, giảm dần theo `value` với khoá phụ `id`; một mức; ≤ 400 ô (ô < 24×16 px hoặc ngoài top 400 gộp "+N nhỏ"); `ResizeObserver` + `requestAnimationFrame`. Màu `deriveStructureArea` gán `--review-area-1..5` theo thứ tự chữ cái các khu vực đang hiển thị, còn lại `--review-area-6`; luôn có nhãn chữ. Lớp phủ qua `overlaySvgProps` (SOL-053): `changed` (file đổi) viền đậm + dấu, thư mục chấm + số; `untested` nét đứt; `violation` icon; **không** có `affected`. File bị xoá không có ô, hiển thị dòng "{n} file bị xoá không hiển thị". Treemap là `role="img"`; cây là đường truy cập.

### 4.4 Cây (WAI-ARIA Tree)

`role="tree"/treeitem`, `aria-level/expanded/setsize/posinset/selected`; phím `↑↓ ← → Home End Enter`; không `*`; hàng đặc biệt (đang tải, lỗi + Thử lại, `{shown}/{total}` + "Tải thêm" khi có `nextPageToken`); `useVirtualizer` 28 px, `overscan` 12. Bộ lọc chip: nhánh không có file đổi ẩn (cây) / mờ (treemap).

## 5. Quyết định thiết kế

Squarified một mức; cây là truy cập chính; tải theo thư mục; màu theo thứ tự chữ cái (giải thích được); cờ `affected` không áp dụng; không `d3`/`elkjs` (O5/O12).

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng |
|---|---|---|
| `structure` (`path`, `depth`, phân trang) | `BE-CV-SOL-040-codeintel-view-channels`; collector `BE-CV-SOL-021-agent-collector`; agent `AG-CV-SOL-002-gitnexus-extraction`, `AG-CV-SOL-003-codegraph-extraction` | G3 (đợt 4; fake G4 trước) |
| Lớp phủ | `BE-CV-SOL-036-change-overlay-pipeline` | G3 |
| Mã hoá lớp phủ, điều hướng diff | SOL-053 (053-01, 053-06) | trước 054-05 |

## 7. Tiêu chí chấp nhận

- [ ] Squarified: tổng diện tích = khung (sai số < 0,5 %), không chồng, xác định, bỏ giá trị 0.
- [ ] ≤ 400 ô; "+N nhỏ"; bố cục tính lại một lần/khung hình.
- [ ] Màu khu vực xác định, có nhãn chữ; không hex (test quét).
- [ ] Lớp phủ đúng; chú giải chỉ cờ có dữ liệu.
- [ ] Bấm ô thư mục đi vào, breadcrumb và cây đồng bộ; "Xem diff" mở diff của file.
- [ ] Cây 10 000 hàng chỉ vẽ cửa sổ nhìn; ARIA đúng; mở thư mục một lần dù bấm nhanh.
- [ ] ≤ 2 truy vấn đồng thời; 5 000 nút dừng; `{shown}/{total}` đúng; `depth` ngoài 1..3 không được gửi.
- [ ] "Dòng" vô hiệu khi thiếu `loc`; lỗi một thư mục không phá phần đã có.

## 8. Kiểm thử

`structure-treemap-layout.test.ts`, `structure-area-model.test.ts`, `structure-changed-index.test.ts`, `structure-tree-model.test.ts`, `useStructureTree.test.ts`, `StructureTree.test.tsx`, `StructureTreemap.test.tsx`, `StructureLens.test.tsx` (`// @vitest-environment happy-dom`, fake backend 073-02), `code-intel-locale-coverage` mở rộng. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

`symbolCount` thư mục tổng hay không (Q1); `color-mix` trong SVG `fill` chưa kiểm Electron; hiệu năng khi kéo panel chưa đo; `loc` có thể thiếu; đường dẫn Windows/WSL; thứ tự khu vực theo chữ cái đổi màu khi thêm khu vực.

## 10. Câu hỏi mở

1. `symbolCount` của thư mục là tổng cây con? (BE-CV-SOL-021).
2. `ModuleNode.area` do backend điền thật (cách ánh xạ `backend-go/services/<tên>`)?
3. Dùng lại `buildTreemapLayout` nếu đủ tốt? Sunburst không ở MVP.
4. Ô "đã xoá" lấy từ diff (`gitBranchChangesByWorktree`)?

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-054-01](../tasks/FE-CV-TASK-054-01-structure-treemap-layout-squarified.md) | Bố cục squarified | P1 |
| [FE-CV-TASK-054-02](../tasks/FE-CV-TASK-054-02-structure-area-model-and-changed-index.md) | Mô hình khu vực và chỉ mục thay đổi | P1 |
| [FE-CV-TASK-054-03](../tasks/FE-CV-TASK-054-03-structure-tree-model-and-use-structure-tree.md) | Mô hình cây và `useStructureTree` | P1 |
| [FE-CV-TASK-054-04](../tasks/FE-CV-TASK-054-04-structure-tree-component-aria.md) | `StructureTree` (ARIA, ảo hoá) | P1 |
| [FE-CV-TASK-054-05](../tasks/FE-CV-TASK-054-05-structure-treemap-component-and-toolbar.md) | `StructureTreemap` và thanh công cụ | P1 |
| [FE-CV-TASK-054-06](../tasks/FE-CV-TASK-054-06-structure-lens-file-detail-registration-i18n.md) | Chi tiết file, đăng ký lens, i18n | P1 |

Thứ tự: 054-01, 054-02, 054-03 song song → 054-04, 054-05 → 054-06.

## 12. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-054-structure-lens.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/frontend/src/renderer/src/components/status-bar/workspace-space-layout.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/editor/CsvViewer.tsx`, `/opt/repos/orca/guides/STYLEGUIDE.md`.
