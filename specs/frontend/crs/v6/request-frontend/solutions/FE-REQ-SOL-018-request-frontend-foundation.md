# FE-REQ-SOL-018: Nền frontend Request (kiểu, RPC, hook, store, định tuyến; gỡ `backlog`)

> 🚧 **Mostly done.** Verified 2026-10-07: 018-01/02/04/06 DONE, 018-03 and 018-05 PARTIAL (useBacklog test/pagination -> 023-02; e2e request-page.spec.ts missing). Deviation: wire parsers/hooks accept both CR-016-draft and CONTRACT-request-ui-api shapes (`new` status, flat `sourceProvider`, `{request}`, `changes`/`at`, approval `id`, spawnChild `linkReason`/`typeHint`); useRequestSubscription now only listens to the bus, the single stream is owned by useRequestEvents.

**CR:** [CR-REQ-018](../../../../../../docs/crs/v6/request-frontend/CR-REQ-018-request-frontend-foundation.md)
**Area:** frontend (`frontend/src/shared`, `frontend/src/renderer/src`)
**Hợp đồng backend:** [CR-REQ-016](../../../../../../docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md). `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` chưa tồn tại tại thời điểm viết; mọi chỗ dựa tạm vào CR-016 được đánh dấu "(tạm, khớp CONTRACT khi có)".
**TDD tham chiếu:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/03-runtime-client-layer](../../../../tdd/v5/03-runtime-client-layer.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md); [v4/04-state-management](../../../../tdd/v4/04-state-management.md), [v4/05-runtime-client](../../../../tdd/v4/05-runtime-client.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `frontend/src/shared/task-types.ts`, `shared/types.ts:3360` (`TopLevelView`), `renderer/src/store/slices/ui.ts` (`TOP_LEVEL_VIEW_LOOKUP` dòng 507, `previousViewBeforeTasks` 632/1336, `openTaskPage` 1348), `store/index.ts:48,117` (`createTaskSlice`), `runtime/runtime-rpc-client.ts` (`callRuntimeRpc`, `getActiveRuntimeTarget`, `subscribeRuntimeStreamChannel`), `runtime/runtime-rpc-result.ts` (`RuntimeRpcCallError.code`), `hooks/useTaskSource.ts`, `hooks/useTaskActivity.ts`, `lib/mcp-event-bus.ts`, `lib/screen-submit-shortcut.ts`, `components/sidebar/SidebarNav.tsx`, `SidebarTaskNavButton.tsx`, `i18n/task-jira-link-locale-coverage.test.ts`.

Xác nhận đúng như CR-REQ-018: không có router, `activeView` quyết định màn hình; `TaskStatus` còn `backlog` (`task-types.ts:24`, `TASK_STATUS_PROGRESS` dòng 151, `TaskStatusBadge.tsx:9`, `TaskDetail.tsx:31`, `TaskBoardView.tsx:11`, `TaskDAGView.tsx:32`).

**Correction relative to CR-REQ-018 (CR-REQ-016 và mã thật thắng):**

| # | CR-018 ghi | Thực tế / hợp đồng CR-016 | Quyết định trong solution |
|---|---|---|---|
| 1 | `solution.chooseOption`, `backlog.list {view}` | CR-016: `solution.choose`, `backlog.requests`, `backlog.tasks`, `backlog.execute` | Dùng tên CR-016 |
| 2 | Thăm dò hỗ trợ bằng `request.list {limit:1}` | CR-016 có `request.flowStatus` trả `{enabled}` (trả lời câu hỏi mở số 2 của CR-018) | Thăm dò bằng `request.flowStatus`; `method_not_found` hoặc `enabled=false` thì ẩn tính năng |
| 3 | Lệnh ghi gửi `version` | CR-016: `approval.approve/reject` nhận `expectedVersion`, `expectedDigest`; các lệnh `request.*` không có tham số version | Chỉ Approval dùng `expectedVersion`/`expectedDigest` |
| 4 | Mã lỗi thô `version_conflict`, `failed_precondition`... | CR-016 2.8: lỗi dạng `CODE: message` (`REQUEST_VERSION_CONFLICT`, `APPROVAL_NOT_APPROVER`...) | Parse tiền tố `CODE:` từ `RuntimeRpcCallError.message`; `code` thô chỉ dùng cho `method_not_found`/`forbidden` |
| 5 | Luồng sự kiện "nếu có" | CR-016 2.5: `request.subscribe {id?}` đẩy `request.event {requestId, eventType, status, type, occurredAt}` | Dùng `subscribeRuntimeStreamChannel` (chỉ target `environment`; target `local` bị reject, xem `runtime-rpc-client.ts:105`) và polling dự phòng |
| 6 | Hook `useRequest` có `history`, `children` | CR-016 `request.get` không kèm lịch sử; có `request.typeHistory`; không có kênh Request con | `useRequest` gọi `request.get` và `request.typeHistory`; liên kết cha/con đọc từ trường `links` của `request.get` nếu có (tạm, CR-016 câu hỏi mở 1) |
| 7 | `task.list` có `requestId` (CR-021) | `channels_automation_task.go:343` chỉ nhận `projectId`, `pageToken`, `pageSize` | Không thuộc CR này; xem FE-REQ-SOL-021 |

`frontend/package.json` chỉ có `build`, `dev`, `test` (`vitest run --config config/vitest.config.ts`), `test:watch`; không có script typecheck hay lint. Lệnh typecheck: `pnpm --filter orca-frontend exec tsc --noEmit -p tsconfig.json` (chưa kiểm chứng; `package.json` gốc có `tc:web` nhưng `config/tsconfig.tc.web.json` không tồn tại ở gốc repo).

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/shared/
  request-types.ts                 (mới) kiểu miền Request, Solution, Approval, Backlog
  request-flow-registry.ts         (mới) REQUEST_FLOW_REGISTRY, REQUEST_STATUS_ORDER, LOW_CONFIDENCE_THRESHOLD
  request-rpc-methods.ts           (mới) REQUEST_RPC_METHODS
  request-errors.ts                (mới) RequestRpcError, classifyRequestRpcError
  request-wire-parsers.ts          (mới) parseRequest/parseSolution/parseApproval/parseBacklogItem
  task-status-normalization.ts     (mới) normalizeTaskStatus
  task-types.ts                    (sửa) bỏ 'backlog', thêm TaskType 'plan'|'phase', OrcaTask.requestId?
  types.ts                         (sửa) TopLevelView += 'requests'
frontend/src/renderer/src/
  runtime/request-rpc-client.ts    (mới) callRequestRpc, subscribeRequestEvents
  lib/request-event-bus.ts         (mới) subscribe/emit nội bộ (mẫu mcp-event-bus.ts)
  hooks/useRequestFlowSupport.ts, useRequests.ts, useRequest.ts, useRequestActions.ts,
        useSolutions.ts, useApprovals.ts, useBacklog.ts, useRequestEvents.ts   (mới)
  store/slices/request.ts          (mới); store/index.ts, store/types.ts (sửa); store/slices/ui.ts (sửa)
  components/request/RequestPage.tsx, RequestPageHeader.tsx, RequestUnsupportedNotice.tsx,
        RequestStatusBadge.tsx, RequestTypeBadge.tsx, RequestSourceBadge.tsx,
        ApprovalStatusBadge.tsx, request-status-presentation.ts               (mới)
  components/sidebar/SidebarRequestNavButton.tsx (mới), SidebarNav.tsx (sửa)
  components/task/{TaskBoardView,TaskDetail,TaskStatusBadge,TaskDAGView}.tsx (sửa)
  App.tsx                          (sửa) lazy RequestPage, gắn useRequestEvents
docs/ui/pages/requests.md, docs/ui/page-tree.md   (mới/sửa)
```

### 2.2 Bảng kênh (khớp CR-016, tạm cho tới khi có CONTRACT)

`REQUEST_RPC_METHODS` là hằng bất biến; mọi tham số camelCase; frontend KHÔNG gửi `tenantId`/`userId` (gateway lấy từ `Identity`, CR-016 mục 4).

| Hằng | Kênh | Tham số | Kết quả dùng |
|---|---|---|---|
| `request.create` | `request.create` | `projectId,title,body,source?{provider,ref,url,site},hints?,clientRequestId?` | `{request, created}` |
| `request.get` | `request.get` | `id` | `OrcaRequest` (kèm `planTaskId`) |
| `request.typeHistory` | `request.typeHistory` | `id` | `{entries}` |
| `request.list` | `request.list` | `projectId?,status[]?,type[]?,sourceProvider?,sourceSite?,sourceRef?,pageSize,pageToken` | `{requests, nextPageToken}` |
| `request.classify` / `confirmType` / `changeType` / `returnToBacklog` / `reopen` / `cancel` / `spawnChild` / `generatePlan` / `startPhase` | cùng tên | theo CR-016 2.3 | `OrcaRequest` |
| `request.subscribe` | stream | `id?` | push `request.event` |
| `request.flowStatus` | `request.flowStatus` | không | `{enabled}` |
| `solution.list` / `generate` / `choose` | cùng tên | theo CR-016 2.4 | `Solution` |
| `approval.get/list/listPending/approve/reject/cancel` | cùng tên | theo CR-016 2.4 | `Approval` |
| `backlog.requests/tasks/execute` | cùng tên | `projectId?` + phân trang (+ `groupBy?`, `planTaskId?`, `phaseTaskId?`) | `{items, nextPageToken}` |

`request.flowSet` (admin) ngoài phạm vi frontend v6; không khai báo hằng.

### 2.3 Kiểu và parser

`request-types.ts` theo README v6 3.2, 3.3, 3.5 (camelCase). Giá trị enum lạ thành `'unknown'` thay vì ném lỗi:

```ts
export type RequestType = 'change_request'|'bug'|'hotfix'|'task'|'spike'|'question'
  |'refactor'|'security'|'performance'|'docs'|'ops_request'
export type RequestStatus = 'new'|'classifying'|'awaiting_type_confirmation'|'analyzing'
  |'awaiting_analysis_approval'|'planning'|'awaiting_plan_approval'|'executing'
  |'completed'|'request_backlog'|'cancelled'
export type SolutionKind = 'solution'|'diagnosis'|'findings'|'answer'
export type ApprovalSubjectType = 'request_type'|'solution'|'findings'|'answer'|'plan'
  |'task_list'|'phase'|'pre_deploy'
export type SolutionOption = { id: string; title: string; summary: string; pros: string[];
  cons: string[]; effort?: string; risk?: string; recommended?: boolean; raw: unknown }
export type Approval = { id: string; requestId: string; subjectType: ApprovalSubjectType|'unknown';
  subjectId: string; stage?: string; status: ApprovalStatus; version: number;
  subjectDigest: string; dueAt?: string; decidedBy?: string; comment?: string }
```

`SolutionOption` và `Approval.version/subjectDigest` là suy luận từ CR-016 (`expectedVersion`, `expectedDigest`) và README mục 8 số 3 (`subject_digest`); schema `options` chưa ai chốt (câu hỏi mở 1).

`REQUEST_FLOW_REGISTRY` chép đúng bảng 3.4 (11 dòng, kể cả `hotfix`: `analysisKind='diagnosis'`, `gates=['request_type','pre_deploy']`). Hằng `REQUEST_FLOW_ANALYSIS_GATE_NONE` cho hotfix (chẩn đoán không cổng).

### 2.4 Lớp RPC và lỗi

`callRequestRpc<T>(method, params)` bọc `callRuntimeRpc(getActiveRuntimeTarget(useAppStore.getState().settings), method, params)`. `classifyRequestRpcError(err): RequestRpcError {kind, code, message, field?}`:

| `kind` | Nhận biết | UI |
|---|---|---|
| `unsupported` | `RuntimeRpcCallError.code === 'method_not_found'`; `REQUEST_FLOW_DISABLED` | Ẩn tính năng, không toast |
| `unavailable` | `REQUEST_UNAVAILABLE` | Banner "Dịch vụ Request chưa cấu hình" |
| `forbidden` | `code==='forbidden'`, `APPROVAL_NOT_APPROVER` | Toast, nút ghi về chỉ đọc |
| `not_found` | `*_NOT_FOUND` | Màn "không còn tồn tại" |
| `conflict` | `*_VERSION_CONFLICT`, `REQUEST_STATE_STALE` | Tải lại, giữ ngữ cảnh |
| `invalid_state` | `REQUEST_TRANSITION_NOT_ALLOWED`, `APPROVAL_ALREADY_DECIDED` | Toast + tải lại |
| `expired` | `APPROVAL_EXPIRED` | Nhãn "Quá hạn" |
| `validation` | `*_REASON_REQUIRED`, `APPROVAL_COMMENT_REQUIRED`, `REQUEST_TYPE_*`, `REQUEST_SOURCE_*` | Lỗi cạnh trường hoặc toast |
| `rate_limited` | `REQUEST_RATE_LIMITED`, `REQUEST_PENDING_LIMIT`, `REQUEST_CLASSIFICATION_LIMIT` | Toast kèm gợi ý chờ |
| `network` | không có `CODE:`, lỗi truyền tải | Banner "Thử lại" |
| `unknown` | còn lại | Toast chung + mã thô |

Hàm tách mã: `/^([A-Z][A-Z0-9_]+):\s*(.*)$/s` trên `error.message`. Bảng này phải khớp CR-016 2.8; mã "mới" (`REQUEST_FLOW_DISABLED`, `REQUEST_RATE_LIMITED`...) chưa có CR nhận sở hữu, nên mã lạ luôn rơi vào `unknown` an toàn.

### 2.5 Hook, sự kiện, store

- Hook theo CR-018 2.3 với chỉnh sửa ở mục 1 (bảng sửa). `useRequestActions` mỗi hàm trả `Result<T, RequestRpcError>` và không ném. `useApprovals.approve/reject` nhận `{approval, comment?}` và tự gắn `expectedVersion`, `expectedDigest` từ `approval`; `reject` chặn ngay ở client khi `comment.trim().length < 10`.
- `useRequestEvents()` gắn một lần ở `App.tsx`: target `environment` thì gọi `subscribeRuntimeStreamChannel(target, 'request.subscribe', {}, onEvent)` và `emitRequestEvent`; target `local` (bị reject) hoặc stream lỗi thì polling: 15 giây khi `RequestPage` mở, 60 giây cho `approval.listPending {pageSize:1}` khi chỉ cần số chờ duyệt, chỉ khi `document.visibilityState === 'visible'`. `eventType` dùng giá trị README 3.7 (khớp chính xác chuỗi chưa kiểm chứng, CR-016 mục 6).
- `store/slices/request.ts`: `requestFlowSupport`, `requestsById`, `pendingApprovalCount`, `requestPage {section, requestId, backlogView, listFilters}`; action trả object một phần (xem ghi chú `slices/task.ts`). `openRequestPage`/`closeRequestPage` đặt trong `ui.ts` cạnh `openTaskPage` vì chạm `activeView` và `previousViewBeforeRequests`.
- Định tuyến: thêm `'requests'` vào `TopLevelView` và `TOP_LEVEL_VIEW_LOOKUP` (thiếu khoá là lỗi biên dịch). `sanitizeHydratedActiveView` (ui.ts:526): khi hydrate `activeView==='requests'` mà `requestFlowSupport !== 'supported'` thì rơi về `terminal`.
- Sidebar: `SidebarRequestNavButton` cạnh `SidebarTaskNavButton` (`SidebarNav.tsx:71`), chấm số `99+`; ẩn khi `unsupported`.

### 2.6 Gỡ `backlog` khỏi `TaskStatus`

Thứ tự trong cùng PR: bỏ `'backlog'` khỏi `TaskStatus` và `TASK_STATUS_PROGRESS`; bỏ khỏi `STATUS_ORDER` (`TaskBoardView.tsx:11`), `TASK_STATUSES` (`TaskDetail.tsx:31`), `STATUS_CONFIG` (`TaskStatusBadge.tsx:9`), `STATUS_COLORS` (`TaskDAGView.tsx:32`); thêm `normalizeTaskStatus(raw)`: `backlog`, giá trị lạ thành `open`; gọi tại `useTasks` (`setTasks`) và `useTask`. Fallback của `TaskStatusBadge` đổi từ `todo` sang `open`. `todo` giữ nguyên (ngoài phạm vi, README v6 mục 8 số 11). Thêm `TaskType` `'plan'|'phase'`; rà bằng `tsc` các `switch` không đầy đủ và `lint:switch-exhaustiveness` (root `package.json`, chưa kiểm chứng chạy được).

## 3. Quyết định thiết kế

- Một view `requests` ba tab, chia sẻ bộ lọc dự án và dữ liệu; điều hướng sâu qua `openRequestPage({section, requestId})`.
- Phân loại lỗi một nơi; component không đọc mã thô.
- Danh sách Solution, Approval để trong hook, không vào store, để tránh hai nguồn sự thật. Store chỉ giữ `requestsById` và `pendingApprovalCount`.
- Parser chịu enum lạ vì backend và frontend phát hành lệch pha (đã gặp với `backlog`).
- Runtime không có kênh: im lặng ẩn mục (đúng cách `useTaskSource`), không cảnh báo.
- SSH/remote: mọi gọi qua `callRuntimeRpc` với `getActiveRuntimeTarget`; không có đường chạy cục bộ riêng. Chịu độ trễ 50 đến 200 ms (trạng thái tải mọi nơi).

## 4. Phụ thuộc và thứ tự

Cần CR-REQ-016 có kênh (hoặc mock hợp đồng ở mức hook). Mở khoá FE-REQ-SOL-019 đến 023. Thứ tự task: 018-01 → 018-02 và 018-04 (song song) → 018-03 → 018-05; 018-06 độc lập, làm sớm được vì không cần backend.

## 5. Kiểm thử

Vitest (`pnpm --filter orca-frontend test -- <đường dẫn>`): `request-flow-registry.test.ts` (11 loại khớp 3.4), `request-wire-parsers.test.ts`, `request-errors.test.ts` (mọi mã CR-016 2.8), `task-status-normalization.test.ts`, slice `request.test.ts`, hook tests (`useRequests`, `useApprovals`, `useRequestFlowSupport`, `useRequestEvents` kể cả local fallback), component (`RequestStatusBadge`, `RequestSourceBadge` chặn `javascript:`, `SidebarNav.test.tsx` mở rộng), cập nhật `TaskBoardView.test.tsx`, `TaskStatusBadge.test.tsx`, `TaskDAGView.test.tsx`; `request-locale-coverage.test.ts` theo mẫu `task-jira-link-locale-coverage.test.ts`. E2E (`tests/e2e/request-page.spec.ts`, mới, theo `tasks-page.spec.ts`): mở trang qua `window.__store.getState().openRequestPage()`, runtime không có kênh thì nút ẩn. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Hình dạng JSON của `Approval`, `Solution.options`, `request.get` chưa có CONTRACT; parser phải chịu thiếu trường.
- `subscribeRuntimeStreamChannel` chỉ hỗ trợ `environment`: người dùng desktop chạy `local` sẽ chỉ có polling.
- `eventType` thật chưa rõ (`request.status_changed` hay `orca.request.request.status_changed`).
- Gỡ `backlog` có thể còn tham chiếu ở `docs/` và e2e chưa rà.
- Thêm `plan|phase` vào `TaskType` có thể phá `switch` ở nơi khác.

## 7. Câu hỏi mở

1. Schema `SolutionOption` và `Approval` (có `version`, `subjectDigest` trong view không).
2. `request.get` có trả `links`, `planTaskId`, `viewerCan` không.
3. `request.event.eventType` chuỗi chính xác.
4. Có nên khai báo `request.flowSet` cho màn admin ở v6 không.

## 8. Tham chiếu

`/opt/repos/orca/docs/crs/v6/README.md` (mục 8 thắng mục 3), `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/frontend/src/shared/task-types.ts`, `/opt/repos/orca/frontend/src/shared/types.ts`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/ui.ts`, `/opt/repos/orca/frontend/src/renderer/src/runtime/runtime-rpc-client.ts`, `/opt/repos/orca/frontend/src/renderer/src/runtime/runtime-rpc-result.ts`, `/opt/repos/orca/frontend/src/renderer/src/hooks/useTaskSource.ts`, `/opt/repos/orca/frontend/src/renderer/src/lib/mcp-event-bus.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/sidebar/SidebarNav.tsx`, `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/channels_automation_task.go`, `/opt/repos/orca/tests/e2e/tasks-page.spec.ts`.
