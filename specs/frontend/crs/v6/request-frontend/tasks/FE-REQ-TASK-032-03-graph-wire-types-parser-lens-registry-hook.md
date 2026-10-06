# FE-REQ-TASK-032-03: Kiểu `GraphPayload`, parser, registry lens và hook `useGraphLens`

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.2, 2.3, 2.4
**Priority:** P0
**Area:** frontend / shared / hooks
**File:** `frontend/src/shared/graph-types.ts`, `frontend/src/shared/graph-wire-parsers.ts` (mới); `frontend/src/shared/request-rpc-methods.ts` (sửa, tạo ở FE-REQ-TASK-018-01); `frontend/src/renderer/src/components/graph/graph-lens-registry.ts` (mới); `frontend/src/renderer/src/hooks/useGraphLens.ts` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-018-01 (`request-types.ts`, `request-rpc-methods.ts`), 018-02 (`callRequestRpc`, `classifyRequestRpcError`), 018-03 (mẫu hook, `request-event-bus`)
**Status:** [ ] TODO

## Context

- Hợp đồng dữ liệu: CR-REQ-032 mục 2.1; backend trả đúng dạng đó (CR-REQ-030 2.8, JSON camelCase qua gateway). Backend chỉ dựng 4 lens `architecture|contract|data|impact`; `plan`, `execution`, `flow` do client (SOL-032 mục 1, hàng 2).
- Backend `impact.graph` nhận `{requestId, subjectType, subjectId, lens, base?, maxNodes}` (CR-030 2.8 dùng `max_nodes`, mặc định 50 nếu không gửi). `subjectType` ∈ `solution_option|plan|task`. Hình dạng `subjectId` cho Solution chưa chốt (câu hỏi mở 2 SOL-032).
- Mẫu parser chịu enum lạ: `request-wire-parsers.ts` (FE-REQ-TASK-018-01). Mẫu hook: `useRequest` (018-03), huỷ bằng cờ `cancelled`, làm mới theo `subscribeRequestBus`.
- `callRequestRpc<T>(method, params)` và `RequestRpcError {kind, code, message}` từ 018-02; `kind='unsupported'` nghĩa là ẩn tính năng, không toast.
- `shared/` dùng chung main, preload, renderer: file mới không import React hay DOM.

## Việc cần làm

1. `graph-types.ts`: khai `GraphLens`, `GraphRisk`, `GraphChange`, `GraphNode` (thêm `meta?: { findingIds?: string[]; taskId?: string; optionId?: string }`), `GraphEdge`, `GraphPayload` (thêm `droppedEdges: number`) đúng SOL-032 mục 2.2. Hằng `GRAPH_LENSES_BACKEND = ['architecture','contract','data','impact'] as const` và `GRAPH_LENSES_CLIENT = ['flow','plan','execution'] as const`; `GRAPH_RISK_ORDER: Record<GraphRisk, number>` (critical 4, high 3, medium 2, low 1, unknown 0).
2. `graph-wire-parsers.ts`: `parseGraphRisk(raw): GraphRisk` (chuỗi không hợp lệ hoặc thiếu trả `'unknown'`), `parseGraphChange(raw): GraphChange` (thiếu trả `'unchanged'`), `parseGraphPayload(raw: unknown, expectedLens: GraphLens): GraphPayload`. Không ném lỗi với đầu vào bất kỳ (kể cả `null`, mảng, số): trả payload rỗng `{nodes:[], edges:[], totalNodes:0, truncated:false, assessedAt:null, tool:null, stale:false, droppedEdges:0}`. Bỏ node thiếu `id` hoặc `label` (đếm không cần); node trùng `id` giữ bản đầu; cạnh trỏ `from`/`to` không có trong `nodes` bị bỏ và cộng `droppedEdges`. `meta.finding_ids` → `meta.findingIds`. `totalNodes` thiếu hoặc nhỏ hơn `nodes.length` thì bằng `nodes.length`. `truncated` boolean mặc định `false`; `stale` mặc định `false`.
3. `request-rpc-methods.ts`: thêm `IMPACT_GRAPH: 'impact.graph'`, `IMPACT_HEATMAP: 'impact.heatmap'`, `IMPACT_DRIFT: 'impact.drift'`, `IMPACT_GET: 'impact.get'` (hằng bất biến). Không thêm `impact.accept` ở task này (task 036-01).
4. `graph-lens-registry.ts`: `export const GRAPH_LENSES: readonly GraphLensConfig[]` với `GraphLensConfig = { id: GraphLens; labelKey: string; Icon: LucideIcon; nodeKinds: string[]; edgeKinds: string[]; legendKeys: string[]; source: 'client' | 'backend' }`. 7 dòng theo bảng CR-REQ-032 2.2 (icon đề xuất: `GitBranch` flow, `Boxes` architecture, `FileCode` contract, `Database` data, `Radar` impact, `ListTree` plan, `Activity` execution). `getLensConfig(id)`; `pickDefaultLens(summaries: Partial<Record<GraphLens, GraphPayload>>): GraphLens` chọn lens backend có `risk` cao nhất theo `GRAPH_RISK_ORDER` (bỏ `unknown`), hòa thì theo thứ tự registry; không có thì `'flow'`.
5. `useGraphLens.ts`: `useGraphLens({ request, lens, subjectType, subjectId, enabled = true, maxNodes = 500 })` trả `{ payload: GraphPayload | null, status: 'idle'|'loading'|'ready'|'error', error: RequestRpcError | null, refetch: () => void }`. Với lens `GRAPH_LENSES_CLIENT` không gọi `impact.graph` (task 032-04 cung cấp adapter; hook nhận `buildClient?: () => GraphPayload` để cắm). Lens backend: `callRequestRpc(IMPACT_GRAPH, {requestId, subjectType, subjectId, lens, maxNodes})` rồi `parseGraphPayload`. Cache `Map` theo khoá `${requestId}|${lens}|${subjectType}|${subjectId}` tối đa 3 mục cho mỗi `requestId` (xoá cũ nhất). Hiển thị `loading` hoãn 200 ms (cờ `showSkeleton` trả thêm) để tránh nhấp nháy khi SSH nhanh. Huỷ kết quả cũ khi đổi tham số hoặc unmount (cờ `cancelled`, không `setState` sau huỷ).
6. Làm mới: `subscribeRequestBus` (018-02) với `event.requestId === request.id` và `eventType` ∈ {`impact.assessed`, `impact.drift_detected`, `plan.generated`, `phase.started`, `phase.completed`} thì `refetch`. Danh sách chuỗi nằm ở hằng `GRAPH_REFETCH_EVENTS` để đổi một chỗ (chuỗi `impact.*` chưa chốt trong CONTRACT).
7. Lỗi: `classifyRequestRpcError` → `unsupported` thì `status='idle'` và `payload=null` (nơi gọi ẩn nút); `forbidden` thì `status='error'` kèm `error.kind`; `network` giữ `payload` cũ và `status='error'`; mã `REQUEST_RISK_ASSESSMENT_PENDING` thì `status='loading'` và lên lịch `refetch` sau 3 giây tối đa 5 lần (cờ để test được); `REQUEST_IMPACT_NO_CONNECTION` thì `error.code` được UI dùng.

## Bảng tham chiếu nhanh

| Kênh | Tham số | Kết quả | Ghi chú |
|---|---|---|---|
| `impact.graph` | `{requestId, subjectType, subjectId, lens, base?, maxNodes}` | `GraphPayload` | lens `architecture\|contract\|data\|impact` |
| `impact.heatmap` | `{planTaskId}` | rủi ro theo Phase và Task | cho lens `plan` (tạm) |
| `impact.drift` | `{phaseId}` | dự kiến so với thực tế | cho lens `execution` (tạm) |

- Sự kiện làm mới: `impact.assessed`, `impact.drift_detected`, `plan.generated`, `phase.started`, `phase.completed` (qua `subscribeRequestBus`).
- Trạng thái hook: `idle` (lens client hoặc `unsupported`), `loading` (skeleton hoãn 200 ms), `ready`, `error` (`forbidden`, `network`, `no_dev_server`).
- Khoá i18n: không (task không có chuỗi hiển thị).
- Phím tắt: không.
- Cờ môi trường: `maxNodes` mặc định 500, trần 2.000; backend mặc định 50 nếu bỏ qua (đã nêu ở rủi ro).

## Trình tự làm gợi ý

1. Viết `graph-wire-parsers.test.ts` (đầu vào lạ trước), rồi `graph-types.ts` và parser.
2. Thêm hằng kênh vào `request-rpc-methods.ts`.
3. Viết `graph-lens-registry.ts` và test `pickDefaultLens`.
4. Viết `useGraphLens.test.tsx` với mock `callRequestRpc`, rồi hook.
5. Chạy `pnpm --filter orca-frontend test src/shared src/renderer/src/hooks/useGraphLens`; kiểm `shared/` không import từ `renderer/`.

## Kiểm thử

- `graph-wire-parsers.test.ts`: payload hợp lệ; `kind` và `status` lạ giữ chuỗi; `risk` thiếu → `unknown` và **không** `low`; cạnh mồ côi bị bỏ và `droppedEdges` đúng; node trùng `id`; `meta.finding_ids` → `findingIds`; đầu vào `null`/`[]`/số trả payload rỗng không ném; `totalNodes` thiếu; `truncated` string `"true"` coi là `false` (chỉ boolean).
- `graph-lens-registry.test.ts`: 7 lens duy nhất, mỗi lens có `labelKey`, `Icon`; `pickDefaultLens` (critical thắng; chỉ `unknown` thì `flow`; hòa theo thứ tự).
- `useGraphLens.test.tsx` (mock `callRequestRpc`, mẫu test hook 018-03): gọi đúng kênh và tham số (có `maxNodes`); lens client không gọi RPC; cache tối đa 3; `refetch` khi sự kiện `impact.assessed` đúng `requestId`, bỏ qua `requestId` khác; huỷ khi unmount; `unsupported` → `idle`; `network` giữ payload cũ; lặp lại khi `REQUEST_RISK_ASSESSMENT_PENDING` (fake timers).
- Chạy: `pnpm --filter orca-frontend test src/shared/graph-wire-parsers src/renderer/src/components/graph/graph-lens-registry src/renderer/src/hooks/useGraphLens`.

## Tiêu chí hoàn thành

- [ ] Parser không bao giờ ném lỗi và không bao giờ biến `risk` thiếu thành `low`.
- [ ] `GRAPH_LENSES` có đủ 7 lens; `pickDefaultLens` đúng.
- [ ] `useGraphLens` gọi `impact.graph` với `maxNodes` và camelCase; không gửi `tenantId`/`userId`.
- [ ] `unsupported` ẩn êm, không toast.
- [ ] Hằng kênh nằm ở `request-rpc-methods.ts`, không chuỗi kênh rải rác.

## Rủi ro và lưu ý

- Tên và hình dạng `impact.graph`, `impact.heatmap`, `impact.drift` là theo CR-REQ-030 2.8, chưa có trong CONTRACT: giữ thay đổi trong `request-rpc-methods.ts` và parser. Khi CONTRACT cập nhật, đối chiếu một lượt.
- `maxNodes` có thể bị backend bỏ qua (mặc định 50); UI vẫn đúng nhờ `truncated`.
- Cache theo `assessmentDigest` chưa có trong khoá (payload không mang digest); sự kiện `impact.assessed` là cách duy nhất làm mới, nên không thiếu sự kiện (CONTRACT mục 3: không bảo đảm giao đủ): nơi mở `GraphPanel` luôn `refetch` khi mở.
- File `shared/` không được import từ `renderer/`.
