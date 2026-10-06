# FE-REQ-TASK-032-04: Hàm thuần gom nhóm, thu phóng, tập trung, trước/sau và adapter lens phía client

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.3, 2.5
**Priority:** P0
**Area:** frontend / graph (logic thuần)
**File:** `frontend/src/renderer/src/components/graph/graph-grouping.ts`, `graph-zoom-levels.ts`, `graph-focus-state.ts`, `graph-before-after.ts`, `graph-client-lens-adapters.ts` (mới); test cùng tên (`*.test.ts`)
**Depends on:** FE-REQ-TASK-032-03 (kiểu `GraphPayload`), FE-REQ-TASK-018-01 (`REQUEST_FLOW_REGISTRY`), FE-REQ-TASK-021-01 và 021-02 (`task-hierarchy.ts`, `usePlanTree` cho adapter `plan`/`execution`)
**Status:** [ ] TODO

## Context

- Mọi hàm là **thuần** (không React, không đồng hồ, không `Math.random`): để test trước, tái lập, chạy được trong Worker nếu cần.
- Quy tắc CR-REQ-032 2.4: gom khi hơn 50 node, `VISIBLE_NODE_LIMIT = 50`, thứ tự ưu tiên `change != unchanged`, `risk` cao, láng giềng 1 bước của node chọn; thu phóng: dưới 0,5 chỉ cấp 1 của `group`, 0,5 đến 1,2 cấp module, trên 1,2 node lá; tập trung: node và láng giềng 1 bước giữ nguyên, phần còn lại mờ; trước/sau trên một đồ thị. Các con số **chưa đo**, nên là hằng có tên để chỉnh một chỗ.
- `GraphNode.group` có dạng đường dẫn "service/module" hoặc `null` (CR-032 2.1); `null` coi là nhóm gốc `''`.
- Lens client: `flow` từ `REQUEST_FLOW_REGISTRY` và `request.status`; `plan` từ `usePlanTree` (FE-REQ-SOL-021: `{plan, phases[], tasksByPhase, flatTasks}`) cộng `impact.heatmap`; `execution` từ cây cộng trạng thái task (README v6 mục 8 số 9: sự kiện task cho task có `request_id`) và `impact.drift`.
- Mẫu hằng/thuần: `request-flow-registry.ts` (018-01).

## Việc cần làm

1. `graph-grouping.ts`: `export const VISIBLE_NODE_LIMIT = 50`. `pickVisibleNodes(payload: GraphPayload, opts: { selectedId?: string | null; openGroups: ReadonlySet<string>; limit?: number }): { visible: GraphNode[]; groups: GraphGroup[] }`. Thứ tự chọn: (a) node `change` khác `unchanged` (tính từ cạnh: node có cạnh `added` hoặc `removed` hoặc `status` ∈ {`added`,`removed`,`modified`}), (b) `risk` theo `GRAPH_RISK_ORDER` giảm dần, (c) láng giềng 1 bước của `selectedId`, (d) `id` tăng dần để ổn định. Node thuộc nhóm trong `openGroups` luôn thuộc phần thấy (không tính vào `limit`). Phần còn lại gom theo `group` thành `GraphGroup { id: string; label: string; count: number; byKind: Record<string, number>; memberIds: string[]; maxRisk: GraphRisk }`; nhóm `maxRisk` kế thừa mức cao nhất của thành viên (không trộn `unknown` thành `low`).
2. `buildGroupEdges(payload, visible, groups)`: cạnh từ/đến node bị gom được nối lên `GraphGroup` tương ứng, hợp nhất cạnh trùng (đếm `weight`), cạnh nội bộ một nhóm bỏ.
3. `graph-zoom-levels.ts`: `ZOOM_SERVICE_MAX = 0.5`, `ZOOM_MODULE_MAX = 1.2`, `type GraphZoomLevel = 'service' | 'module' | 'leaf'`, `levelForZoom(zoom: number): GraphZoomLevel` (biên: `0.5` thuộc `module`, `1.2` thuộc `module`, trên 1,2 `leaf`; `NaN` hoặc âm trả `module`), `collapseToLevel(visible, groups, level)` trả node đại diện theo cấp (service = đoạn đầu của `group` trước `/`).
4. `graph-focus-state.ts`: `focusNeighborhood(edges: GraphEdge[], id: string): Set<string>` (chính nó và láng giềng 1 bước, vô hướng); `type FocusAction = {type:'focus'; id} | {type:'clear'}`; `focusReducer(state: {focusId: string | null}, action)`; `isDimmed(nodeId, focusSet | null): boolean`.
5. `graph-before-after.ts`: `type GraphChangeView = 'before' | 'after'`; `applyChangeView(payload, view)`: `'after'` giữ tất cả cạnh, đánh dấu `removed` là mờ (`dim: true`); `'before'` bỏ cạnh `added`, giữ `removed` như cạnh thường (`change` đổi thành `'unchanged'` trong bản trả về, không đổi payload gốc), bỏ node chỉ liên quan tới cạnh `added` nếu `status==='added'`. Trả `GraphPayload` mới (bất biến). Lens không có `change` (không cạnh nào khác `unchanged`): trả nguyên và `hasChangeAxis(payload)` trả `false` để UI ẩn công tắc.
6. `graph-client-lens-adapters.ts`:
   - `buildFlowGraph(request): GraphPayload` từ `REQUEST_FLOW_REGISTRY[request.type].stages` (hoặc danh sách trạng thái tương đương): node `kind='step'`, `status` là `done|current|pending|skipped` theo `request.status` và `REQUEST_STATUS_ORDER`; cạnh `transition`; `risk='unknown'` cho mọi node (lens flow không có rủi ro; riêng `unknown` ở đây nghĩa là không áp dụng, chú thích bằng `meta`); `awaiting_information` là node `step` bổ sung nằm giữa bước hiện tại và bước kế khi `request.status==='awaiting_information'`.
   - `buildPlanGraph(tree, heatmap?)`: node `phase` và `task` (`group` là tên Phase), cạnh `depends_on` từ `dependencyEdges`; `risk` từ `heatmap` theo `taskId` nếu có, không có thì `'unknown'`.
   - `buildExecutionGraph(tree, outcomes?, drift?)`: như plan nhưng `status` là trạng thái task thực, node nằm trong `drift` có `meta.drifted=true` (UI gắn nhãn "lệch").
   - Cả ba trả `GraphPayload` với `totalNodes`, `truncated:false`, `assessedAt/tool` từ heatmap nếu có, `stale:false`.
7. Mọi hàm xuất kèm kiểu; không `any`; hằng là `as const`. Không import từ `@xyflow/react` (tách xa thư viện để test nhanh và dùng lại trong Worker).

## Bảng tham chiếu nhanh

| Hằng | Giá trị | Nơi |
|---|---|---|
| `VISIBLE_NODE_LIMIT` | 50 | `graph-grouping.ts` |
| `ZOOM_SERVICE_MAX` | 0.5 | `graph-zoom-levels.ts` |
| `ZOOM_MODULE_MAX` | 1.2 | `graph-zoom-levels.ts` |
| `GraphChangeView` | `'before' \| 'after'` | `graph-before-after.ts` |

- Kênh WS: không (hàm thuần); dữ liệu đầu vào do `useGraphLens` và `usePlanTree` cấp.
- Khoá i18n: không; nhãn nhóm "+N" nằm ở `GraphGroupNode` (032-05) khoá `GroupSummary`.
- Phím tắt: không.
- Trạng thái: hàm thuần xử lý đầu vào rỗng (0 node) bằng kết quả rỗng, không ném lỗi.
- Quy ước đặt tên: mỗi file một khái niệm; không tạo `graph-utils.ts` hay `graph-helpers.ts`.

## Kiểm thử

- `graph-grouping.test.ts`: 120 node → đúng 50 hiển thị cộng nhóm; node `critical` và node có cạnh `added` luôn hiển thị; láng giềng của node chọn hiển thị; kết quả ổn định (cùng đầu vào cùng thứ tự); nhóm trong `openGroups` mở ra; `maxRisk` của nhóm không đưa `unknown` thành `low`; cạnh nối nhóm hợp nhất đúng `weight`; 0 node và 1 node không lỗi.
- `graph-zoom-levels.test.ts`: biên 0,49, 0,5, 1,2, 1,21, `NaN`, âm.
- `graph-focus-state.test.ts`: láng giềng hai chiều; node cô lập chỉ có chính nó; reducer `clear`.
- `graph-before-after.test.ts`: `after` giữ `removed` mờ; `before` bỏ `added`; không đổi payload gốc (đóng băng bằng `Object.freeze`); lens không có `change` thì `hasChangeAxis=false`.
- `graph-client-lens-adapters.test.ts`: `flow` cho `bug` (hotfix có thể khác) có node đúng thứ tự và đúng một node `current`; `awaiting_information` thêm bước; `plan` không có heatmap thì mọi `risk='unknown'`; `execution` đánh dấu `drifted`.
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/graph/graph-grouping src/renderer/src/components/graph/graph-zoom-levels src/renderer/src/components/graph/graph-focus-state src/renderer/src/components/graph/graph-before-after src/renderer/src/components/graph/graph-client-lens-adapters`.
- Hiệu năng (đo tay, ghi số vào PR): `pickVisibleNodes` với 2.000 node và 6.000 cạnh; mục tiêu đề xuất dưới 50 ms (chưa đo).

## Tiêu chí hoàn thành

- [ ] Đồ thị hơn 50 node chỉ hiện tối đa 50 (cộng nhóm đã mở); node `added`/`removed`/rủi ro cao luôn thuộc phần thấy.
- [ ] Ba mức thu phóng đổi cấp hiển thị đúng biên.
- [ ] Công tắc trước/sau chỉ có ý nghĩa khi có trục `change`.
- [ ] Ba adapter client trả `GraphPayload` hợp lệ qua `parseGraphPayload` (test vòng khứ hồi).
- [ ] Không file nào import React hay `@xyflow/react`.

## Rủi ro và lưu ý

- Quy tắc "láng giềng của node chọn" làm hiển thị thay đổi khi chọn node; để tránh nhảy bố cục, `GraphCanvas` (032-05) chỉ áp dụng tại chỗ (không tái bố cục khi chọn).
- Với repo cỡ nghìn symbol, 50 có thể quá ít hoặc quá nhiều; hằng nằm một chỗ để chỉnh sau khi đo.
- `buildFlowGraph` phụ thuộc hình dạng `REQUEST_FLOW_REGISTRY` của 018-01: đọc lại file đó trước khi viết và dùng đúng tên trường.
- `heatmap` có hình dạng chưa chốt: parser của 036/032-03 phải chịu thiếu; ở đây nhận `Record<taskId, GraphRisk>` đã chuẩn hoá.
