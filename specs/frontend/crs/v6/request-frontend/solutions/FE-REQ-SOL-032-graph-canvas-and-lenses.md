# FE-REQ-SOL-032: `GraphCanvas`, bảy lens, token rủi ro và chuyển `TaskDAGView` sang token

> 🚧 **In Progress (4/8 tasks DONE: 032-01..04; PARTIAL: 032-05, 032-06, 032-08; BLOCKED: 032-07).** Triển khai và kiểm 2026-10-07: token `--risk-*`, `RiskBadge`, `TaskDAGView` sang token, `GraphPayload` + parser + `useGraphLens`, các hàm thuần, `GraphCanvas`/`GraphMini`/`GraphPanel`, danh sách, tìm kiếm, `RequestGraphSheet`, i18n 5 locale, 90 test components/graph xanh. Còn: áp mức thu phóng ngữ nghĩa vào hiển thị, nút kết nối dev server, e2e thật, phân tích bundle, đo hiệu năng; `elkjs` chặn bởi duyệt phụ thuộc. Hình dạng `impact.heatmap|drift` và chuỗi sự kiện vẫn "tạm".

**CR:** [CR-REQ-032](../../../../../../docs/crs/v6/request-frontend/CR-REQ-032-graph-canvas-and-lenses.md), [ADDENDUM-2026-10-06](../../../../../../docs/crs/v6/request-frontend/ADDENDUM-2026-10-06.md)
**Area:** frontend (`frontend/src/shared`, `frontend/src/renderer/src/components/graph`, `assets/main.css`, `guides/STYLEGUIDE.md`)
**Hợp đồng backend:** [CONTRACT-request-ui-api.md](../../../../../backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md) mục 7 (quy tắc thêm kênh) và [CR-REQ-030 mục 2.8](../../../../../../docs/crs/v6/impact-risk/CR-REQ-030-impact-assessment-and-risk-scoring.md) (`impact.graph`, `impact.heatmap`). Solution backend của CR-REQ-030 chưa có khi soạn: chỗ dựa vào CR gốc ghi "(tạm)".
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md), [v5/14-workflow-ui](../../../../tdd/v5/14-workflow-ui.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md); [v4/05-runtime-client](../../../../tdd/v4/05-runtime-client.md)
**Đi cùng:** [FE-REQ-SOL-018](./FE-REQ-SOL-018-request-frontend-foundation.md) (`callRequestRpc`, `RequestRpcError`, `request-event-bus`), [FE-REQ-SOL-021](./FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) (`usePlanTree`). Mở khoá [FE-REQ-SOL-036](./FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) (`riskPresentation`, `RiskBadge`, `GraphMini`, `GraphPanel`).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `frontend/src/renderer/src/components/task/TaskDAGView.tsx` (273 dòng), `components/task/TaskGraph.tsx` (299 dòng), `components/task/__tests__/TaskDAGView.test.tsx`, `components/workflow/DAGPreview.tsx`, `assets/main.css` (`@theme inline` dòng 43, `:root` dòng 126, `.dark` dòng 216), `lib/document-theme.ts`, `hooks/usePrefersReducedMotion.ts`, `components/ui/{command,table,toggle-group,sheet,popover,tooltip,badge,skeleton}.tsx`, `components/ShortcutKeyCombo.tsx`, `lib/screen-submit-shortcut.ts`, `frontend/package.json`, `guides/STYLEGUIDE.md`.

Xác nhận đúng như CR-REQ-032:

- `TaskDAGView.tsx` dòng 26 đến 34: `STATUS_COLORS` toàn hex (`#f0fdf4`, `#16a34a`...), dòng 105 `#6b7280`, dòng 131 `#94a3b8`. Trái STYLEGUIDE dòng 24 ("Never hardcode a hex value"). Có khoá `backlog` (sẽ bị FE-REQ-TASK-018-06 gỡ) và `todo`.
- `package.json`: `@xyflow/react ^12.11.2` ở `dependencies`; `cmdk ^1.1.1`, `@tanstack/react-virtual ^3.13.24`, `lucide-react ^0.577.0`, `radix-ui ^1.4.3`, `zustand ^5.0.13`, `mermaid ^11.15.0` ở `devDependencies` (renderer được bundle bởi Vite nên vẫn dùng được). **Không có** `elkjs`, `dagre`, `@dagrejs/dagre`.
- `main.css`: `--status-success` là hex (`#15803d` sáng, `#86efac` tối) cùng `-background`, `-border` bằng `color-mix`; `--destructive` `#e40014` và `#ff6568`; `--ai-action-accent` là `var(--color-violet-*)`. **Không có** `--risk-*`, `--graph-edge-*`, `--review-*`. Dark mode theo lớp `.dark` (không phải media query).
- `components/ui/` **không có** `radio-group`, `switch`, `alert`: chip lens dùng `toggle-group`, công tắc Trước/Sau cũng `toggle-group` (hai mục).
- Chưa có thư mục `components/graph/`, chưa có `shared/graph-types.ts`.

**Correction relative to CR-REQ-032 (hợp đồng backend và code thắng):**

| # | CR-032 ghi | Thực tế / hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `impact.graph {requestId, subjectType, subjectId, lens, base?}` | CR-030 2.8: backend thêm `max_nodes` (mặc định **50**, server cắt, `truncated=true`) | Hook gửi `maxNodes` tường minh (mặc định 500, trần 2.000 theo CR-032 2.8); nếu backend bỏ qua thì UI vẫn chạy với `truncated` và chuyển danh sách |
| 2 | Lens `plan` lấy `risk` từ "CR-REQ-030" chung chung | CR-030 2.8: kênh riêng `impact.heatmap {planTaskId}` (mức theo Phase và Task); backend chỉ dựng 4 lens `architecture|contract|data|impact` bằng `impact.graph` | Lens `plan` = cây từ `usePlanTree` + `impact.heatmap`; lens `execution` = sự kiện task + `impact.drift` |
| 3 | `GraphNode.status` ví dụ `breaking`, `irreversible` | CR-030 ví dụ `modified`, `breaking`, `irreversible`, trạng thái task | `status` là chuỗi tự do theo lens; `graph-status-presentation.ts` ánh xạ giá trị đã biết, lạ thì hiện nhãn thô |
| 4 | Lens `execution` nhận sự kiện "theo kênh nào" (câu hỏi mở 3) | CONTRACT mục 3: `request.event` chỉ kích hoạt tải lại; `phase.started|completed` có sẵn; `task-service` bổ sung sự kiện task cho task có `request_id` (README v6 mục 8 số 9) | Lens `execution` làm mới bằng `useRequestEvents` + tải lại `usePlanTree`; không vá từng node |
| 5 | "Chip lens tắt kèm tooltip lý do" | Không có kênh trả "lens nào có dữ liệu" | Lens bật/tắt suy từ: `request.flowStatus`, có `planTaskId`, `impact.get` trả có đánh giá; chip tắt khi `unsupported` hoặc chưa đánh giá |
| 6 | `elkjs` đề xuất | README v7 O5: **không thêm** `elkjs`/`dagre` ở MVP (kiểm tra `docs/crs/v7/README.md`, có tồn tại) | Mục 3 (quyết định) và task 032-07 (có điều kiện) |

**Mâu thuẫn v7 (đã đọc `docs/crs/v7/README.md`, mục O5):** "Thư viện bố cục/treemap mới (`elkjs`/`dagre`, `d3-*`): Không thêm ở MVP: bố cục tầng đơn giản + treemap SVG tự viết; thêm thư viện cần duyệt riêng". CR-REQ-032 mục 2.6 đề xuất `elkjs`. Cách giải quyết trong solution này: (1) `GraphCanvas` nhận `LayoutEngine` cắm được, mặc định bố cục tầng (wave) nội bộ đủ cho `flow`, `plan`, `execution` và chạy được cho 4 lens còn lại ở mức "xếp theo nhóm rồi theo tầng"; (2) task 032-07 là **tuỳ chọn, chặn bởi quyết định duyệt**; chưa duyệt thì chỉ 032-01 đến 032-06 và 032-08 được làm, không đụng `package.json`; (3) v7 (CR-CV-053, CR-CV-055) có thể dùng cùng `LayoutEngine` mà không bị ép dùng `elkjs`.

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/shared/
  graph-types.ts                   (mới) GraphPayload, GraphNode, GraphEdge, GraphLens, GraphRisk, GraphChange
  graph-wire-parsers.ts            (mới) parseGraphPayload, parseGraphRisk
frontend/src/renderer/src/
  assets/main.css                  (sửa) --risk-*, --graph-edge-* ở :root, .dark, @theme inline
  hooks/useGraphLens.ts            (mới) tải GraphPayload theo lens, cache, huỷ khi unmount
  hooks/useDocumentColorMode.ts    (mới) 'light'|'dark' cho colorMode của xyflow
  components/graph/
    risk-presentation.ts           (mới) riskPresentation(level): {labelKey, icon, borderClass, tokenClass}
    RiskBadge.tsx                  (mới) chữ + icon + token, không chỉ màu
    graph-lens-registry.ts         (mới) GRAPH_LENSES: {id, labelKey, icon, nodeKinds, edgeKinds, legend, source}
    graph-grouping.ts              (mới, thuần) VISIBLE_NODE_LIMIT, pickVisibleNodes, buildGroupNodes
    graph-zoom-levels.ts           (mới, thuần) ZOOM_SERVICE_MAX=0.5, ZOOM_MODULE_MAX=1.2, levelForZoom
    graph-focus-state.ts           (mới, thuần) focusNeighborhood, createFocusReducer
    graph-before-after.ts          (mới, thuần) applyChangeView(payload, 'before'|'after')
    graph-client-lens-adapters.ts  (mới, thuần) flow, plan, execution -> GraphPayload
    graph-layout-engine.ts         (mới) type LayoutEngine, waveLayoutEngine
    GraphPanel.tsx  GraphToolbar.tsx  GraphLensChips.tsx  GraphViewToggle.tsx
    GraphBeforeAfterToggle.tsx  GraphSearchButton.tsx  GraphLegend.tsx
    GraphCanvas.tsx (lazy)  GraphNodeCard.tsx  GraphGroupNode.tsx  GraphEdgeLine.tsx
    GraphListView.tsx  GraphSearchPalette.tsx  GraphNodeSheet.tsx
    GraphEmptyState.tsx  GraphErrorState.tsx  GraphSkeleton.tsx  GraphStatusBanner.tsx
    GraphMini.tsx
    elk-layout-engine.ts, elk-layout.worker.ts   (mới, CHỈ khi duyệt, task 032-07)
  components/task/TaskDAGView.tsx  (sửa) token, icon trạng thái, colorMode
  components/request/RequestDetailHeader.tsx  (sửa, SOL-019) nút "Xem đồ thị"
  components/request/plan/PlanSummaryHeader.tsx (sửa, SOL-021) công tắc Cây | Đồ thị
guides/STYLEGUIDE.md               (sửa) mục token rủi ro và đồ thị
docs/ui/pages/requests.md          (sửa) ghi GraphPanel
```

### 2.2 Hợp đồng dữ liệu và parser

`shared/graph-types.ts` chép đúng CR-REQ-032 mục 2.1 (camelCase). Bổ sung so với CR:

```ts
export type GraphLens = 'flow'|'architecture'|'contract'|'data'|'impact'|'plan'|'execution'
export type GraphRisk = 'low'|'medium'|'high'|'critical'|'unknown'
export type GraphChange = 'added'|'removed'|'unchanged'
export type GraphNode = { id: string; kind: string; label: string; group: string|null
  risk: GraphRisk; status: string|null; meta?: { findingIds?: string[]; taskId?: string } }
export type GraphEdge = { from: string; to: string; kind: string; change: GraphChange }
export type GraphPayload = { lens: GraphLens; nodes: GraphNode[]; edges: GraphEdge[]
  totalNodes: number; truncated: boolean; assessedAt: string|null; tool: string|null
  stale: boolean; droppedEdges: number }
```

- `parseGraphPayload(raw, expectedLens)`: không ném lỗi; trường lạ bị bỏ; `risk` thiếu hoặc lạ thành `'unknown'` (**không** `'low'`); `change` thiếu thành `'unchanged'`; `kind` và `status` lạ giữ chuỗi thô; cạnh trỏ node không có bị bỏ và cộng `droppedEdges`; `totalNodes` thiếu thì `nodes.length`; `lens` không khớp `expectedLens` thì giữ `expectedLens`.
- `meta.finding_ids` của backend (snake_case, CR-030 2.8) được parser đổi thành `meta.findingIds`; đây là ngoại lệ có chủ đích với quy tắc C13 của CONTRACT (khoá lạ giữ nguyên) vì `GraphNode.meta` do frontend định nghĩa.
- `droppedEdges` là trường **frontend tự tính**, không có ở backend.

### 2.3 Lens

`graph-lens-registry.ts` là cấu hình thuần; thêm lens không sửa `GraphCanvas`:

| Lens | Nguồn | Tải bằng | Điều kiện bật chip |
|---|---|---|---|
| `flow` | client, `REQUEST_FLOW_REGISTRY` + `request.status` (FE-REQ-SOL-018) | `buildFlowGraph(request)` | luôn bật |
| `architecture` / `contract` / `data` / `impact` | backend | `impact.graph {requestId, subjectType, subjectId, lens, base?, maxNodes}` | đã có đánh giá (`impact.get` không trả `unsupported` hoặc rỗng) |
| `plan` | client + backend | `buildPlanGraph(planTree)` + `impact.heatmap {planTaskId}` | có `planTaskId`; heatmap `unsupported` thì `risk='unknown'` cho mọi node |
| `execution` | client + backend | `buildExecutionGraph(planTree, outcomes)` + `impact.drift {phaseId}` | Request ở `executing` hoặc đã có Phase chạy |

- Lens mặc định: lens backend có node `risk` cao nhất (thứ tự `critical > high > medium > low`, `unknown` không tính); nếu không có đánh giá thì `flow`. Hàm thuần `pickDefaultLens(summary)`.
- `subjectType` hợp lệ: `solution_option | plan | task` (CR-REQ-032 2.11). Với Solution thì `subjectId` là id Solution kèm `optionId`; **tham số chính xác chưa có CONTRACT** (câu hỏi mở 2).

### 2.4 Kênh WS

Hằng ở `shared/request-rpc-methods.ts` (FE-REQ-SOL-018; thêm bởi task 032-03, đổi ở một chỗ):

| Hằng | Kênh | Tham số | Kết quả |
|---|---|---|---|
| `IMPACT_GRAPH` | `impact.graph` | `{requestId, subjectType, subjectId, lens, base?, maxNodes?}` | `GraphPayload` |
| `IMPACT_HEATMAP` | `impact.heatmap` | `{planTaskId}` | mức rủi ro theo Phase và Task (hình dạng chưa chốt, parser chịu thiếu) |
| `IMPACT_DRIFT` | `impact.drift` | `{phaseId}` | dự kiến so với thực tế (dùng nhiều ở SOL-036) |
| `IMPACT_GET` | `impact.get` | `{subjectType, subjectId}` | tóm tắt (SOL-036) |

- Mọi gọi qua `callRequestRpc` (SSH/remote đúng `getActiveRuntimeTarget`). Mã lỗi theo `classifyRequestRpcError`: `unsupported` ẩn nút "Xem đồ thị", `forbidden` hiện "Bạn không có quyền xem đánh giá", `REQUEST_RISK_ASSESSMENT_PENDING` hiện trạng thái đang tải kèm nhãn giai đoạn, `REQUEST_IMPACT_NO_CONNECTION` hiện "Chưa có dev server kết nối" (CR-030 2.10).
- Làm mới: sự kiện `impact.assessed` (CONTRACT mục 7: thêm giá trị `RequestEventType` là additive; chuỗi `eventType` chính xác chưa chốt, xem câu hỏi mở 3) chỉ kích hoạt `refetch` của `useGraphLens`.
- `useGraphLens({request, lens, subject})` giữ cache tối đa 3 payload cho mỗi Request (khoá `lens|subjectId|assessmentDigest`), hoãn hiện skeleton 200 ms, trả `{payload, status: 'idle'|'loading'|'ready'|'error', error, refetch}`.

### 2.5 Hành vi (hàm thuần có test trước)

- **Gom nhóm:** `pickVisibleNodes(payload, {selectedId, openGroups})` chọn tối đa `VISIBLE_NODE_LIMIT = 50` theo thứ tự: `change !== 'unchanged'`, rồi `risk` giảm dần, rồi láng giềng một bước của node đang chọn, rồi theo `id` để ổn định. Phần còn lại gom theo `group` thành `GraphGroupNode` ("+N node không bị ảnh hưởng", đếm theo `kind`). Con số 50 theo nghiên cứu, **chưa đo**.
- **Thu phóng ngữ nghĩa:** `levelForZoom(zoom)` trả `'service'` (dưới 0,5), `'module'` (0,5 đến 1,2), `'leaf'` (trên 1,2). Hằng ở `graph-zoom-levels.ts`. `onMove` của xyflow cập nhật mức bằng `useRef` + debounce để không render lại mỗi khung hình.
- **Tập trung:** `focusNeighborhood(edges, id)` trả tập id láng giềng 1 bước; phần còn lại `opacity-30`, vẫn bấm được; `Esc` thoát.
- **Trước/Sau:** `applyChangeView(payload, 'after')` (mặc định) giữ cạnh `removed` (nét đứt, mờ, nhãn "−"); `'before'` ẩn cạnh và node chỉ có cạnh `added`, vẽ `removed` như cạnh thường. Một đồ thị, không có hai đồ thị cạnh nhau.
- **Danh sách:** `GraphListView` luôn sẵn; mặc định khi `truncated`, khi trình đọc màn hình bật (`window.matchMedia('(prefers-reduced-motion)')` không đủ; dùng cờ `aria` từ cài đặt a11y của app nếu có, **chưa kiểm chứng** app có cờ này) hoặc khi màn hẹp. Ảo hoá bằng `useVirtualizer` khi hơn 100 hàng.
- **Tìm:** `GraphSearchPalette` dùng `components/ui/command.tsx`; phím `/` khi tiêu điểm không ở ô nhập; Enter chọn node, mở nhóm chứa nó, `fitView`, đặt tiêu điểm bàn phím.
- **Bàn phím:** `Tab` qua node theo thứ tự danh sách, `Arrow` nhảy láng giềng, `Enter` mở `GraphNodeSheet`, `Esc` thoát tập trung. Nhãn phím hiển thị bằng `ShortcutKeyCombo`; `/` là phím thường nên không cần `metaKey`/`ctrlKey`; không thêm phím có Mod mới.
- **Hoạt ảnh:** `usePrefersReducedMotion()` true thì `fitView` với `duration: 0` và `animated=false` trên cạnh.

### 2.6 Màu, hình và token

Chữ trước, hình thứ hai, màu thứ ba (CR-032 2.5). `riskPresentation(level)` trả `{labelKey, icon, borderWidth, dashed, tokenVar}`:

| Mức | `labelKey` | Icon (lucide) | Viền | Token |
|---|---|---|---|---|
| `low` | `auto.components.graph.RiskLevel.low` | `CircleCheck` | 1 px | `--risk-low` |
| `medium` | `...medium` | `TriangleAlert` | 1 px + chấm góc | `--risk-medium` |
| `high` | `...high` | `OctagonAlert` | 2 px | `--risk-high` |
| `critical` | `...critical` | `ShieldAlert` | 2 px đôi (`outline` + `border`) | `--risk-critical` |
| `unknown` | `...unknown` | `CircleHelp` | nét đứt | `--muted-foreground` |

Chữ không viết "an toàn". Tooltip: "Rủi ro Thấp, đánh giá lúc `<assessedAt>`, dựa trên `<tool>`" (khoá `RiskTooltip`, tham số nội suy). **Token thiếu và phải thêm** (đã xác nhận bằng grep `main.css`: không có `--risk`, `--graph-edge`):

```css
/* :root */  --risk-low: var(--status-success);
             --risk-medium: var(--color-amber-600);   --risk-high: var(--color-orange-600);
             --risk-critical: var(--destructive);
             --risk-low-background / -border ... = color-mix(in srgb, var(--risk-*) 10% / 25%, transparent)
             --graph-edge-added: var(--status-success);  --graph-edge-removed: var(--destructive);
/* .dark */  --risk-medium: var(--color-amber-400);   --risk-high: var(--color-orange-400);
```

Phải bind từng token trong `@theme inline` (mẫu `--color-status-success`) để dùng `text-risk-high`, `bg-risk-high-background`, `border-risk-high-border`. `guides/STYLEGUIDE.md` cần thêm: bảng token mới, quy tắc "rủi ro luôn kèm chữ và icon", và ghi `--risk-*` là ngoại lệ có chủ đích cho bảng "color is reserved for state" (dòng 7). **Chưa kiểm chứng:** biến `--color-amber-*`, `--color-orange-*` có sẵn trong theme Tailwind của app (chỉ thấy `--color-violet-*` được dùng, `main.css` dòng 154); độ tương phản trên hai chủ đề chưa đo.

Phối hợp v7: CR-CV-050 mục 2.10 đề xuất `--review-changed|affected|untested|violation`. Hai nhóm khác nghĩa; quy ước: `--review-untested` và `--risk-medium` cùng sắc hổ phách (đều tham chiếu `--color-amber-*`); không gộp tên. Xem câu hỏi mở 5.

### 2.7 Bố cục

`graph-layout-engine.ts`: `type LayoutEngine = (nodes, edges, options: {direction, groupOf}) => Promise<Record<string, {x:number; y:number}>>`. `waveLayoutEngine` tách logic `buildDAGLayout` của `TaskDAGView` (sóng topo, `HORIZONTAL_GAP=220`, `VERTICAL_GAP=90`), thêm bước xếp cột theo `group`. Kết quả cache theo `(digest, lens, openGroupsKey)`. `elkjs` chỉ qua task 032-07, nạp lười, trong Web Worker; không có thì mọi lens vẫn chạy bằng wave. Tiêu chí chọn giữa `elkjs` và `dagre` theo bảng CR-032 2.6; đề xuất giữ nguyên (ưu tiên `elkjs`, lùi `dagre` nếu EPL-2.0 hoặc kích thước không qua), đều **chưa đo**.

### 2.8 Chuyển `TaskDAGView` sang token (độc lập, làm trước)

Ánh xạ trạng thái sang lớp Tailwind (không hex, không `style` màu): `done` dùng `bg-status-success-background border-status-success-border` + icon `CircleCheck`; `in_progress` dùng `border-primary` + icon `Loader`; `blocked` dùng `border-destructive` + `Ban`; `review` dùng `border-ai-action-accent` + `Eye` (chọn `--chart-3` nếu bị từ chối khi duyệt); `todo`, `open` dùng `border-border bg-muted`; `cancelled` dùng `text-muted-foreground` + `CircleSlash`. Cạnh `var(--border)`; chữ phụ `text-muted-foreground`. Thêm `colorMode` theo `useDocumentColorMode()`. Giữ `data-testid`, `onConnect`, `addDependency`. Node vẫn dùng `data.label` là React node (không `style` nền).

### 2.9 Tích hợp màn hình

- `RequestDetailHeader` (SOL-019): nút "Xem đồ thị" mở `GraphPanel` trong `Sheet` (rộng `sm:max-w-4xl`); ẩn khi lens backend và client đều không khả dụng.
- `PlanSummaryHeader` (SOL-021): công tắc `Cây | Đồ thị` (`ToggleGroup`), lens `plan`; `TaskDAGView` giữ chế độ `dag` của trang Tasks.
- `SolutionOptionCard` (SOL-020/036): `GraphMini` (tối đa 12 node, chỉ đọc, `aria-hidden` khi có `GraphListView` tương ứng, luôn kèm dòng tóm tắt văn bản).
- Lens `flow` có thể thay hoặc bổ sung `RequestStageTimeline`; giữ bản hiện tại, chỉ thêm liên kết.

## 3. Quyết định thiết kế

- Một canvas, lens là cấu hình; ba lens dựng ở client để chạy được khi backend chưa có CR-030.
- `risk='unknown'` là giá trị riêng, không bao giờ vẽ như `low`; `stale=true` hạ mọi nhãn thành "chưa đánh giá được đầy đủ".
- Danh sách là chế độ ngang hàng, không phải phụ lục; là mặc định khi `truncated`.
- Bố cục qua `LayoutEngine`; **không thêm `elkjs` khi chưa được duyệt** (README v7 O5). Tách task 032-07 để không chặn phần còn lại.
- Làm `TaskDAGView` sang token trước vì độc lập, nhỏ, và là bằng chứng token hoạt động trong SVG của xyflow (điểm chưa kiểm chứng).
- Không sửa điểm rủi ro ở UI; chỉ hiển thị. `GraphNodeSheet` có văn bản thuần, không HTML.
- Không tự làm `GraphCanvas` điều khiển `task.addEdge`: `connectable` chỉ ở `TaskDAGView` (câu hỏi mở 4).

## 4. Phụ thuộc và thứ tự

Cần FE-REQ-SOL-018 (RPC client, bus), FE-REQ-SOL-021 (`usePlanTree`) cho lens `plan`. Backend CR-030 chỉ cần cho 4 lens backend; có thể mock hợp đồng ở mức hook.

```
032-01 (token, RiskBadge) ──┬─▶ 032-02 (TaskDAGView, cần token)
                            └─▶ 032-05 ─▶ 032-06 ─▶ 032-08
032-03 (kiểu, parser, hook) ─▶ 032-04 (hàm thuần) ─▶ 032-05
032-07 (elkjs, CHỈ khi duyệt) cắm vào 032-05 qua LayoutEngine
```

032-01 và 032-03 chạy song song. 036 cần 032-01 (`RiskBadge`, `riskPresentation`), 032-05 (`GraphMini`), 032-06 (`GraphPanel`).

## 5. Kiểm thử

Vitest (`pnpm --filter orca-frontend test <đường dẫn>`; `frontend/package.json` chỉ có `build`, `dev`, `test`, `test:watch`). Unit: `graph-wire-parsers.test.ts`, `graph-grouping.test.ts` (ưu tiên, cắt 50, nhóm "+N", ổn định), `graph-zoom-levels.test.ts`, `graph-focus-state.test.ts`, `graph-before-after.test.ts`, `graph-client-lens-adapters.test.ts`, `risk-presentation.test.ts` (5 mức đủ chữ, icon, token; không có hex), `graph-lens-registry.test.ts` (mọi lens có `labelKey`; `pickDefaultLens`). Component: `GraphPanel` (đổi lens, công tắc, danh sách đồng bộ), `GraphListView` (ảo hoá, `useVirtualizer` mock), `GraphSearchPalette`, `GraphNodeCard` (không chỉ màu), `GraphGroupNode`, `GraphMini`; mock `@xyflow/react` theo `TaskDAGView.test.tsx`. Cập nhật `TaskDAGView.test.tsx`: không còn `style` hex. Kiểm tĩnh: `rg '#[0-9a-fA-F]{3,6}' frontend/src/renderer/src/components/graph frontend/src/renderer/src/components/task/TaskDAGView.tsx` rỗng (đã chạy được, cần thực thi sau triển khai). i18n: `graph-locale-coverage.test.ts` theo mẫu `task-jira-link-locale-coverage.test.ts`. Đo tay (ghi số): 50, 500, 2.000 node; thời gian bố cục wave (và `elkjs` nếu duyệt); kích thước bundle trước và sau. E2E: `tests/e2e/request-graph.spec.ts` (mới; mẫu `tasks-page.spec.ts`). Tất cả **chưa chạy**.

## 6. Rủi ro và điểm chưa kiểm chứng

- xyflow với `var(--…)` trong SVG và `colorMode` chưa kiểm chứng; nếu `border-color` qua class Tailwind không áp dụng cho node mặc định thì dùng node tuỳ biến (`nodeTypes`).
- Hiệu năng xyflow với hàng trăm node; `onlyRenderVisibleElements` bật khi hơn 200 (chưa đo).
- `--color-amber-*`, `--color-orange-*` có hay không trong theme; độ tương phản hai chủ đề.
- Backend cắt `max_nodes=50` mặc định: nếu không nhận `maxNodes`, mọi đồ thị lớn `truncated` và UI luôn vào danh sách (vẫn đúng, kém giá trị).
- Dữ liệu `impact` thiếu cạnh xuyên gRPC, WS, outbox làm đồ thị "sạch" sai (CR-030 mục 6); đó là lý do có `stale`, `unknown` và dòng chú giải.
- Ngưỡng 50, 2.000, 0,5, 1,2 là đề xuất.
- Hai bộ token `--review-*` (v7) và `--risk-*` có thể trôi lệch.
- `elkjs`: giấy phép EPL-2.0, kích thước, Worker trong Electron/Vite chưa kiểm chứng.

## 7. Câu hỏi mở

1. Duyệt `elkjs` (hay `dagre`, hay không thư viện) và ai duyệt giấy phép EPL-2.0? Mâu thuẫn README v7 O5.
2. `impact.graph` với Solution: `subjectId` là `solutionId` kèm `optionId`, hay `optionId` toàn cục? Hình dạng `impact.heatmap` và `impact.drift` trong CONTRACT.
3. Chuỗi `eventType` thật cho `impact.assessed`, `impact.drift_detected` (additive trong CONTRACT mục 7).
4. `TaskDAGView` có chuyển hẳn sang `GraphCanvas` (bước 2) không, vì `onConnect` ghi qua `task.addEdge`?
5. Hợp nhất `--review-*` (v7) và `--risk-*` hay giữ hai nhóm?
6. Cờ "trình đọc màn hình bật" có sẵn trong app không (để chọn danh sách mặc định)?

## 8. Tham chiếu

`/opt/repos/orca/frontend/src/renderer/src/components/task/TaskDAGView.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskGraph.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/task/__tests__/TaskDAGView.test.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/workflow/DAGPreview.tsx`, `/opt/repos/orca/frontend/src/renderer/src/assets/main.css`, `/opt/repos/orca/frontend/src/renderer/src/lib/document-theme.ts`, `/opt/repos/orca/frontend/src/renderer/src/hooks/usePrefersReducedMotion.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/ui/command.tsx`, `/opt/repos/orca/frontend/package.json`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/docs/crs/v7/README.md` (O5), `/opt/repos/orca/docs/crs/v6/impact-risk/CR-REQ-030-impact-assessment-and-risk-scoring.md`, `/opt/repos/orca/docs/research/receive-request/frontend-visualization-and-ux.md`, `/opt/repos/orca/docs/research/receive-request/impact-assessment-and-risk-scoring.md`, `/opt/repos/orca/specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`.
