# FE-REQ-TASK-032-05: `GraphCanvas`, node, nhóm, cạnh, `GraphMini` và `LayoutEngine` bố cục tầng

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.5, 2.6, 2.7
**Priority:** P0
**Area:** frontend / graph (canvas)
**File:** `frontend/src/renderer/src/components/graph/{GraphCanvas,GraphNodeCard,GraphGroupNode,GraphEdgeLine,GraphMini}.tsx`, `graph-layout-engine.ts` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-032-01 (`RiskBadge`, `riskPresentation`), 032-03 (kiểu), 032-04 (hàm thuần); FE-REQ-TASK-032-02 khuyến nghị (dùng `useDocumentColorMode`)
**Status:** [x] DONE (verified 2026-10-08: vitest GraphCanvas (10), GraphNodeCard (10), GraphEdgeLine (2), GraphMini (3), graph-zoom-levels (9), graph-layout-engine (5), graph-layout-engine-contract (11) pass; e2e request-graph.web.e2e.ts a/c vẽ node thật bằng xyflow)

## Context

- `@xyflow/react ^12.11.2` đã có (`dependencies`); `TaskDAGView.tsx` nạp `ReactFlow`, `Background`, `Controls`, `MiniMap` và `@xyflow/react/dist/style.css`; `TaskGraph.tsx` nạp `TaskDAGView` bằng `lazy`. `components/workflow/DAGPreview.tsx` cũng dùng xyflow. Test mock `@xyflow/react` (`TaskDAGView.test.tsx`, `DAGPreview.test.tsx`) vì cần `ResizeObserver` và kích thước thật.
- `buildDAGLayout` trong `TaskDAGView.tsx` (dòng 36 đến 140) là nguồn của bố cục sóng: `HORIZONTAL_GAP = 220`, `VERTICAL_GAP = 90`, chống chu trình bằng tập đã thăm. `waveLayoutEngine` tách logic này (không sửa `TaskDAGView`, tránh xung đột với task 032-02; chỉ sao chép thuật toán và test lại).
- Không hex (SOL-032 mục 2.6); dùng `riskPresentation` (032-01) để lấy lớp token và icon.
- `usePrefersReducedMotion` đã có (`hooks/usePrefersReducedMotion.ts`).
- `GraphMini` được SOL-036 dùng trong `SolutionOptionCard`: tối đa 12 node, chỉ đọc, `aria-hidden` khi có danh sách tương ứng, kèm dòng tóm tắt văn bản.

## Việc cần làm

1. `graph-layout-engine.ts`: `export type LayoutPositions = Record<string, { x: number; y: number }>`; `export type LayoutOptions = { direction: 'LR' | 'TB'; groupOf: (nodeId: string) => string | null }`; `export type LayoutEngine = (nodes: GraphNode[], edges: GraphEdge[], options: LayoutOptions) => Promise<LayoutPositions>`. `waveLayoutEngine`: sóng topo theo cạnh `from → to`, chống chu trình (node trong chu trình về sóng 0), trong mỗi sóng sắp theo `group` rồi `id`; cột cách `HORIZONTAL_GAP`, hàng `VERTICAL_GAP`; nhóm cùng `group` xếp liền nhau. Hàm `layoutCacheKey(payloadId, lens, openGroupsKey)` để cache kết quả trong `GraphCanvas` (`useRef<Map>`, tối đa 5).
2. `GraphNodeCard.tsx` (`memo` theo `id`, `risk`, `status`, `change`, `selected`, `dimmed`): hiển thị icon `kind` (lucide theo bảng trong registry), nhãn (cắt `truncate`, đầy đủ trong `title`), `RiskBadge size="sm"` (chữ + icon), chip `status` (ánh xạ nhãn qua `graph-status-presentation.ts`, lạ thì chữ thô), viền theo `riskPresentation` (1 px, 2 px, đôi bằng `outline`, nét đứt cho `unknown`). `change==='added'` có dấu "+" và `removed` có dấu "−" (chữ, không chỉ màu). `tabIndex={0}`, `role="button"`, `aria-label` ghép nhãn, loại, rủi ro, trạng thái; `Enter` gọi `onOpen(id)`; `Arrow` gọi `onNavigate(id, direction)`.
3. `GraphGroupNode.tsx`: hiển thị "+N <loại> không bị ảnh hưởng" (khoá `GroupSummary` với `{count, kinds}`), `maxRisk` bằng `RiskBadge`, nút mở (`Enter`/click) gọi `onToggleGroup(groupId)`; khi mở hiện khung bao các node con (xyflow `parentId`/`extent: 'parent'` nếu dùng node lồng; nếu không, chỉ hiện nhóm đã mở bằng cách đưa thành viên vào `visible`, đơn giản hơn: chọn cách này ở bản đầu).
4. `GraphEdgeLine.tsx` (edge tuỳ biến của xyflow): `change` `added` nét liền + nhãn "+" + `stroke: var(--graph-edge-added)`; `removed` nét đứt (`strokeDasharray`), `opacity` 0,5, nhãn "−", `var(--graph-edge-removed)`; `unchanged` `var(--border)`. Nhãn là chữ trong `EdgeLabelRenderer`, có `aria-label`. `animated` chỉ khi `!prefersReducedMotion` và cạnh `in_progress` (lens execution).
5. `GraphCanvas.tsx` (nạp lười bằng `React.lazy` ở nơi gọi `GraphPanel`): props `{ payload: GraphPayload; layout?: LayoutEngine; selectedId: string | null; onSelect(id|null); onOpenNode(id); openGroups: ReadonlySet<string>; onToggleGroup(id); changeView: 'before'|'after'; focusId: string | null; onFocus(id|null); fitViewSignal?: number; className?: string }`. Dùng `pickVisibleNodes`, `applyChangeView`, `levelForZoom` (cập nhật bằng `onMove`, debounce 100 ms), `focusNeighborhood`. `ReactFlow` với `nodeTypes={{ graphNode, graphGroup }}`, `edgeTypes={{ graphEdge }}`, `colorMode`, `nodesDraggable={false}`, `nodesConnectable={false}`, `elementsSelectable`, `onlyRenderVisibleElements` khi hơn 200 node, `fitView` với `duration: prefersReducedMotion ? 0 : 200`. `Controls` luôn, `MiniMap` chỉ khi hơn 30 node. `Esc` (keydown trong canvas) gọi `onFocus(null)`. Bố cục chạy một lần mỗi `layoutCacheKey` (bất đồng bộ, hiển thị node tại `0,0` rồi `fitView` khi có kết quả; không chặn luồng giao diện).
6. `GraphMini.tsx`: props `{ payload: GraphPayload | null; summaryKey: string; className?: string; interactive?: false }`. Tối đa 12 node (`pickVisibleNodes` với `limit: 12`), không toolbar, không kéo, không `Controls`, bố cục `waveLayoutEngine` đồng bộ (thuật toán thuần, chạy khi render lần đầu bằng `useMemo`); dòng tóm tắt văn bản bắt buộc ("5 service, 2 cạnh thêm, 1 phá vỡ tương thích") tính bởi `summarizeGraph(payload)` (thuần, thêm vào `graph-grouping.ts` hoặc file riêng `graph-summary.ts`), `aria-hidden` cho phần SVG khi `props.hasListEquivalent`. `payload=null` hiện khung trống "Chưa đánh giá" (không vẽ).
7. `graph-status-presentation.ts` (thuần): ánh xạ `status` đã biết (`modified`, `breaking`, `irreversible`, `added`, `removed`, trạng thái task `open|in_progress|review|done|blocked|cancelled`) sang `{labelKey, Icon}`; lạ trả `{labelKey: null, Icon: null}` và UI hiển thị chữ thô.
8. Không dùng `style={{ color/background }}` với hex; kích thước và vị trí là `style` hợp lệ.

## Bảng tham chiếu nhanh

| Component | Props chính | State nội bộ |
|---|---|---|
| `GraphCanvas` | `payload`, `layout?`, `selectedId`, `openGroups`, `changeView`, `focusId`, `fitViewSignal` | mức thu phóng (`useRef`), bố cục (`useRef<Map>`) |
| `GraphNodeCard` | `node`, `selected`, `dimmed`, `onOpen`, `onNavigate` | không |
| `GraphGroupNode` | `group`, `onToggleGroup` | không |
| `GraphEdgeLine` | `edge`, `changeView` | không |
| `GraphMini` | `payload`, `summaryKey`, `hasListEquivalent` | không |

- Khoá i18n: `auto.components.graph.GroupSummary`, `auto.components.graph.Node.ariaLabel`, `auto.components.graph.Edge.added`, `auto.components.graph.Edge.removed`, `auto.components.graph.Mini.summary`, `auto.components.graph.Mini.empty`.
- Phím tắt: `Enter` mở node, `Arrow` nhảy láng giềng, `Esc` thoát tập trung (không dùng `metaKey`/`ctrlKey`).
- Trạng thái: node ngoài hộp nhìn thấy không render (`onlyRenderVisibleElements` khi hơn 200); bố cục đang tính hiển thị node tại gốc rồi `fitView`.
- Kênh WS: không (nhận dữ liệu qua props).

## Trình tự làm gợi ý

1. Viết `graph-layout-engine.test.ts` và `waveLayoutEngine` (sao chép thuật toán từ `buildDAGLayout`, không sửa `TaskDAGView`).
2. Viết `GraphNodeCard`, `GraphEdgeLine`, `GraphGroupNode` kèm test.
3. Viết `GraphCanvas` với mock `@xyflow/react`, rồi `GraphMini`.
4. Kiểm tay 50, 500, 2.000 node bằng dữ liệu giả; ghi số vào PR.
5. Chạy kiểm tĩnh không hex và `pnpm --filter orca-frontend test src/renderer/src/components/graph`.

## Kiểm thử

- `graph-layout-engine.test.ts`: chuỗi 3 node → ba sóng; kim cương; chu trình không treo (hết giờ 1 s); node cô lập sóng 0; cùng đầu vào cùng kết quả (xác định).
- `GraphNodeCard.test.tsx`: 5 mức rủi ro đều có chữ và icon; `removed` có dấu "−"; `Enter` gọi `onOpen`; `Arrow` gọi `onNavigate`; `dimmed` giảm `opacity` (lớp); không có `#` trong `style`.
- `GraphEdgeLine.test.tsx`: ba loại `change` có nhãn "+", "−", hoặc không nhãn; `animated` tắt khi `prefers-reduced-motion` (mock `matchMedia`).
- `GraphCanvas.test.tsx` (mock `@xyflow/react` theo `TaskDAGView.test.tsx`, expose `nodes`, `edges`, `onNodeClick`, `onMove`): 120 node → mock nhận tối đa 50 cộng nhóm; đổi `changeView` đổi tập cạnh; `Esc` gọi `onFocus(null)`; `MiniMap` chỉ khi hơn 30 node; mở nhóm đưa thành viên vào danh sách.
- `GraphMini.test.tsx`: tối đa 12 node; có dòng tóm tắt; `aria-hidden` khi có danh sách tương ứng; `payload=null` hiện "Chưa đánh giá".
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/graph/graph-layout-engine src/renderer/src/components/graph/GraphNodeCard src/renderer/src/components/graph/GraphEdgeLine src/renderer/src/components/graph/GraphCanvas src/renderer/src/components/graph/GraphMini`.
- Kiểm tay (ghi số): 50, 500, 2.000 node (dữ liệu giả); thời gian `waveLayoutEngine`; FPS khi kéo bản đồ; kiểm `var()` trong SVG xyflow ở sáng và tối.

## Tiêu chí hoàn thành

- [ ] Hơn 50 node hiện tối đa 50 cộng nhóm "+N" mở được.
- [ ] Mọi mức rủi ro có chữ và icon; thang xám vẫn phân biệt 5 mức.
- [ ] Cạnh `added`/`removed` có nhãn "+" "−" ngoài màu và nét.
- [ ] `GraphMini` tối đa 12 node, chỉ đọc, kèm tóm tắt văn bản.
- [ ] `GraphCanvas` được nạp lười (không nằm trong chunk chính; kiểm bằng phân tích bundle ở 032-08).
- [ ] Không hex; `prefers-reduced-motion` tắt hoạt ảnh.

## Rủi ro và lưu ý

- Xyflow trong Electron cần container có kích thước; `GraphCanvas` luôn đặt trong phần tử `h-full min-h-[320px]`; test dùng mock.
- Tái bố cục mỗi lần mở nhóm làm các node nhảy chỗ; dùng cache theo `openGroupsKey` và giữ `fitView` chỉ khi đổi lens hoặc tìm kiếm.
- Node lồng (`parentId`) khó hơn dự kiến; bản đầu dùng nhóm phẳng.
- `GraphEdgeLine` với `EdgeLabelRenderer` có thể cần `pointer-events: none`; kiểm tay.
- Cổng 2.000 node là đề xuất chưa đo; nếu vẽ chậm, hạ `VISIBLE_NODE_LIMIT` hoặc bật `onlyRenderVisibleElements` sớm hơn.

## Ghi chú triển khai (2026-10-07)

`levelForZoom` chạy (ref + debounce 100 ms, `data-zoom-level`) nhưng `collapseToLevel` chưa được dùng để vẽ lại nút theo cấp service/module; chưa đo hiệu năng 50/500/2000 node và bundle. `GraphMini` vẽ SVG thuần (không xyflow) và dùng `computeWaveLayout` đồng bộ. Nhóm đóng/mở bằng cách đưa thành viên vào `visible` (không lồng node).

## Ghi chú triển khai (2026-10-08)

- Thu phóng ngữ nghĩa đã áp vào hiển thị: `levelForZoom` → `zoomPresentation` (`service`: node `minimal` 120 px chỉ nhãn + viền rủi ro, cạnh không dấu "+/−", độ mờ 0,5; `module`: `compact`; `leaf`: `full` có chip trạng thái). Rủi ro không bao giờ bị bỏ (viền + `aria-label`). `collapseToLevel` (gộp node đại diện) vẫn là hàm thuần chưa dùng để vẽ: gộp node khi thu nhỏ sẽ đổi tập node của xyflow mỗi lần cuộn chuột, chọn cách giảm chi tiết thay vì gộp.
- Lỗi thật phát hiện khi chạy e2e: xyflow (controlled) giữ node `visibility: hidden` nếu kích thước đo được không được đưa lại; mỗi lần dựng lại node (đổi mức thu phóng) canvas trắng. Sửa: `onNodesChange` lưu `dimensions` vào `measured` và truyền lại; `fitView` chỉ chạy khi mọi node đã đo (tránh zoom tối đa trên khung rỗng).
- Đo `waveLayoutEngine` (test hợp đồng, máy dev): 50 node 0,3 ms, 500 node 1,3 ms, 2.000 node 2,9 ms. FPS khi kéo bản đồ chưa đo tay.
- `GraphCanvas` nằm ngoài chunk chính: xem phân tích bundle ở 032-08.
