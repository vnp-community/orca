# FE-CV-TASK-088-05: Hàm thuần: treemap squarified và thứ tự ma trận phụ thuộc (DSM)

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.5, 2.8
**Priority:** P0
**Area:** frontend / pure algorithms
**File:** `frontend/src/renderer/src/components/quality-charts/treemap-squarified-layout.ts`, `dependency-matrix-ordering.ts` (mới) và `__tests__/treemap-squarified-layout.test.ts`, `dependency-matrix-ordering.test.ts`
**Depends on:** không (làm được ngay)
**Status:** [x] DONE (verified 2026-10-07: treemap-squarified-layout 6/6, dependency-matrix-ordering 7/7)

## Context

- Treemap hiện có (`components/status-bar/workspace-space-layout.ts`, `buildTreemapLayout`:117, chia đôi cân bằng) cho tỉ lệ cạnh kém; CR-088 chọn squarified và đặt file ở `quality-charts/` để FE-CV-SOL-054 nhập lại. Không refactor file hiện có.
- DSM: nút ≤ 60, ô lấp ≤ 700; thứ tự theo tô-pô của đồ thị thành phần liên thông mạnh, nhóm vòng thành khối (CR-088 2.7).
- Nguồn dữ liệu thật cho DSM ở 087 là `ModuleGraph` (`codeIntel.structure`): `nodes[].id`, `edges[] {from,to,kind:'imports'|'contains',count}` (hợp đồng §4.2). Hàm ở đây chỉ nhận `{from, to}` chung.

## Việc cần làm

1. `squarify(items, bounds)` (thuật toán Bruls-Huizing-van Wijk): sắp giảm dần theo `size`; bỏ `size <= 0` (trả không có ô, hoặc ô diện tích 0 ở cuối có cờ — chọn: **bỏ** và trả danh sách `omitted` qua kiểu trả về `{ rects, omitted: string[] }`); khung `width/height` bằng 0 trả rỗng; tất định (ổn định theo `id` khi `size` bằng nhau).
2. `orderByStrongComponents(nodes, edges)`: Tarjan (lặp, không đệ quy sâu để tránh tràn ngăn xếp ở 60-150 nút là không cần, nhưng vẫn dùng lặp); trả `{ order, blocks }` trong đó `blocks` là các SCC có > 1 nút hoặc có tự vòng; thứ tự tô-pô của đồ thị ngưng tụ, trong khối giữ thứ tự ổn định theo `nodes` đầu vào; cạnh tới nút không có trong `nodes` bị bỏ; tự vòng được đánh dấu khối.
3. Tính chất đo được (dùng trong test): diện tích tổng ô = diện tích khung ± 0,5%; không chồng ô (dung sai 1e-6); mọi ô nằm trong khung.
4. Ngân sách: squarified 400 ô ≤ 16 ms; DSM 150 nút ≤ 50 ms (test thời gian ngưỡng rộng ×5 để tránh nhiễu CI).

## Kiểm thử

- Squarified: một ô, hai ô, 400 ô ngẫu nhiên (hạt giống cố định), kích thước bằng nhau, một ô áp đảo, `size` âm/NaN, khung rất mỏng; tỉ lệ cạnh trung bình tốt hơn `buildTreemapLayout` trên cùng bộ dữ liệu (so sánh trong test, không bắt buộc ngưỡng cứng; ghi số vào PR).
- DSM: DAG (đúng tô-pô), một vòng 3 nút, hai vòng tách rời, tự vòng, đồ thị rỗng, đồ thị đầy đủ 60 nút (không ném, đúng một khối).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts/__tests__/treemap src/renderer/src/components/quality-charts/__tests__/dependency-matrix-ordering`.

## Tiêu chí hoàn thành

- [ ] Các tính chất ở Việc 3 được test; số đo thời gian được ghi vào PR.
- [ ] Không phụ thuộc DOM/React; không dependency mới.

## Rủi ro

- Thứ tự tô-pô có thể kém đọc với đồ thị gần đầy (SOL-088 6.4); cần thử trên dữ liệu thật ở 087-19.
- Squarified với nhiều ô rất nhỏ tạo ô dưới 1 px: `MetricTreemap` (088-08) chịu trách nhiệm gộp "+N".

## Ghi chú triển khai (2026-10-07)

`squarify` trả `{rects, omitted}`. Test so tỉ lệ cạnh trung bình với `buildTreemapLayout` (squarified tốt hơn hoặc bằng); không sửa `workspace-space-layout.ts`. Thời gian đo: squarify(400) ~1 ms, DSM(150) ~1 ms.
