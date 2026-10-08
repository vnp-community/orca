# FE-CV-TASK-088-08: Điều hướng bàn phím dạng lưới và ba hình: `MetricTreemap`, `HotspotHeatmap`, `DependencyMatrix`

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.5, 2.6, 2.7, 2.8
**Priority:** P0
**Area:** frontend / components (hình có ô tương tác)
**File:** `frontend/src/renderer/src/components/quality-charts/useChartKeyboardNavigation.ts`, `MetricTreemap.tsx`, `HotspotHeatmap.tsx`, `DependencyMatrix.tsx` (mới) và `__tests__/useChartKeyboardNavigation.test.tsx`, `MetricTreemap.test.tsx`, `HotspotHeatmap.test.tsx`, `DependencyMatrix.test.tsx`
**Depends on:** FE-CV-TASK-088-04, 088-05, 088-06 (và 088-03 cho glyph)
**Status:** [x] DONE (verified 2026-10-07: grid-navigation-position 4/4, useChartKeyboardNavigation 6/6, MetricTreemap 5/5, HotspotHeatmap 5/5, DependencyMatrix 6/6)

## Context

- CR-088 2.5: một điểm dừng Tab theo mẫu APG "grid" (`role="grid"`, `role="gridcell"`, tiêu điểm lăn `tabindex`): `←→↑↓`, `Home/End`, `Ctrl+Home/End`, `Enter`, `Esc`. Không hàng trăm điểm dừng Tab (đã quyết ở CR-054).
- Mẫu tương tác sẵn có: `WorkspaceSpaceManagerPanel.tsx` — ô là `<button>` định vị tuyệt đối, `aria-label` = "{nhãn}, {dung lượng}" (:917), `focus-visible:ring-2 ring-ring`; học cách đặt nhãn, **không** copy mã.
- STYLEGUIDE: nhãn phím theo nền tảng; chỉ hiển thị chip phím khi phím thật sự cài (ở đây: không hiển thị chip trong 088).
- Không tự gắn phím `j/k` (xung đột đăng ký phím của CR-052/059 ở dock; phím riêng của lưới chỉ khi tiêu điểm nằm trong lưới).
- Giá trị `null` hiển thị "—" (không 0) và không có bậc cường độ.

## Việc cần làm

1. `useChartKeyboardNavigation({rowCount, columnCount, onActivate, onEscape})`: trả `{activeCell, getCellProps(row,col), containerProps}`; `containerProps` đặt `role="grid"`, `aria-rowcount/colcount`, `onKeyDown`; `getCellProps` đặt `role="gridcell"`, `aria-rowindex/colindex` (1-based), `tabIndex` (0 cho ô đang hoạt động, -1 cho còn lại); phím theo APG; chặn cuộn trang của mũi tên khi tiêu điểm trong lưới; `Esc` gọi `onEscape` và trả tiêu điểm về khung. Lưới thưa (treemap): ô được tuyến tính hoá theo thứ tự đọc; nêu rõ trong `aria-description` khung.
2. `MetricTreemap`: dùng `squarify` (088-05) và `bucketIntensity`; ô là `<div role="gridcell">` hoặc `<button>` trong hàng ảo (chọn một cách nhất quán, mặc định `button`); `aria-label` "{label}, {sizeLabel}: {size}, {intensityLabel}: {intensity}"; màu `var(--quality-heat-N)` + viền `--border`; chữ trong ô bậc 1-3 `--foreground`, 4-5 `--background`; ô nhỏ không có chỗ chữ vẫn có nhãn trong bảng; lớp `overlay` (đã đổi...) biểu diễn bằng **hình/nét viền** do nơi gọi truyền (không nhận màu); `maxTiles` mặc định 400: gộp phần dư vào ô "+N khác" (một ô, không tương tác sâu) và hiển thị "Đang hiển thị 400/{N}".
3. `HotspotHeatmap`: `<table>`-giống lưới hoặc `div` với `role="grid"`; hàng ≤ 40, cột ≤ 6; mỗi ô có số + bậc cường độ; `null` "—"; tiêu đề cột có `unit`; chọn hàng bằng `Enter`; `maxRows` cắt có "X/Y".
4. `DependencyMatrix`: sắp bằng `orderByStrongComponents`; SVG: lưới nền là một `<path>`, ô lấp `<rect>` đậm theo `weight`; khối vòng khung đậm + ký hiệu `▲` cho cạnh ngược thứ tự; `description` ghi quy ước đọc (hàng = nguồn, cột = đích); `maxNodes` 60: nút dư gộp "Khác (N)" xếp cuối; `onSelectCell(from,to)`. Ô lấp nằm trong một lưới tương tác chỉ gồm **ô lấp** (điều hướng mũi tên nhảy giữa ô lấp theo hàng/cột; chưa kiểm chứng UX; thay thế dự phòng: lưới đầy đủ N×N nếu thử người dùng không thích).
5. Mọi hình hiển thị trong `ChartFrame`; không `transition`; không toast.

## Kiểm thử

- Hook: mô phỏng phím (`fireEvent.keyDown`) cho mọi phím trong APG; `Home/End` đầu/cuối hàng; `Ctrl+Home/End`; chỉ một phần tử `tabIndex=0`; `Enter` gọi `onActivate`; `Esc` gọi `onEscape`; biên (hàng đầu bấm ↑ không đổi).
- Treemap: số ô DOM ≤ 400 + 1; `aria-label` đúng; vượt trần có "X/Y"; ô có thể chọn bằng bàn phím; `null`/0 size không ném.
- Heatmap: số có trong DOM; `null` là "—"; chọn hàng.
- DSM: khối vòng đánh dấu bằng ký hiệu (không chỉ màu); `aria-rowindex/colindex`; đồ thị rỗng hiển thị trạng thái rỗng có lý do.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts/__tests__/useChartKeyboardNavigation src/renderer/src/components/quality-charts/__tests__/MetricTreemap src/renderer/src/components/quality-charts/__tests__/HotspotHeatmap src/renderer/src/components/quality-charts/__tests__/DependencyMatrix`.

## Tiêu chí hoàn thành

- [ ] Ba hình là một điểm dừng Tab; phím theo APG; test mô phỏng phím xanh.
- [ ] Vượt trần thì gộp + "X/Y", không cắt im lặng.
- [ ] Không hex/lớp màu thô; `focus-visible:ring-2 ring-ring` hiện rõ.

## Rủi ro

- Hành vi mũi tên trong treemap (không phải lưới đều) khó đoán; cần kiểm tay với trình đọc màn hình.
- Chưa kiểm chứng hiệu năng 700 `<rect>` + sự kiện phím; ngân sách ở SOL-088 2.8 chưa đo.

## Ghi chú triển khai (2026-10-07)

Thêm `grid-navigation-position.ts` (hàm thuần, hỗ trợ lưới thưa cho DSM: nhảy giữa ô lấp), `heat-cell-style.ts`. Treemap: tuyến tính hoá một hàng, ô là `<button role=gridcell>`, ô "+N" không tương tác. Chưa kiểm tay với trình đọc màn hình; UX điều hướng DSM thưa chưa thử người dùng.
