# FE-REQ-SOL-023: Màn hình Backlog ba phân đoạn (Request, Task, Execute)

> 🔴 **Not Started.** Rà soát 2026-10-07: chưa có `BacklogTab`, `RequestBacklogTable`, `TaskBacklogTable`. `useBacklog.ts` (2.2 KB) tồn tại nhưng chỉ là skeleton từ SOL-018. Ngày soạn 2026-10-06.

**CR:** [CR-REQ-023](../../../../../../docs/crs/v6/request-frontend/CR-REQ-023-backlog-screens.md)
**Backend liên quan:** CR-REQ-015 (`ListBacklog`, `ListExecutionStates`), CR-REQ-006 (`ReopenRequest`, `CancelRequest`, `returned_category`), CR-REQ-013 (`task_run_outcomes`), CR-REQ-016 (kênh WS)
**Service/Area:** `frontend/src/renderer/src` (components, hooks, shared, i18n)
**TDD tham chiếu:** [02-state-management](../../../../tdd/v5/02-state-management.md), [03-runtime-client-layer](../../../../tdd/v5/03-runtime-client-layer.md), [15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md) (TaskBoardView, TaskDetail, badge, test), [05-ui-components](../../../../tdd/v5/05-ui-components.md)
**Phụ thuộc:** FE-REQ-SOL-018 (kiểu, `callRequestRpc`, slice, `RequestPage`, `RequestTypeBadge`, `RequestSourceBadge`, gỡ `backlog` khỏi `TaskStatus`), FE-REQ-SOL-019 (`openRequestPage`, chi tiết Request), FE-REQ-SOL-021 (tab Plan), FE-REQ-SOL-022 task 04 (`useRowListKeyboardNavigation`), task 03 (`useRequestSummaries`, `useMinuteClock`), task 03 (`request-relative-time.ts`)

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06:

- `components/request/` chưa tồn tại; `hooks/useBacklog.ts` chưa tồn tại (CR-018 sẽ tạo). Mọi tên 018 dưới đây ghi "(khớp SOL-018 khi merge)".
- `components/task/TaskDetail.tsx` không nhận props: đọc `activeTaskId` từ store (`useAppStore((s) => s.activeTaskId)`), nạp bằng `useTask(activeTaskId!)`; setter là `setActiveTask(id)` (`store/slices/task.ts:37`). Mở `TaskDetail` trong `Sheet` nghĩa là gọi `setActiveTask(taskId)` rồi render `<TaskDetail />` trong `ui/sheet.tsx`; đóng thì `setActiveTask(null)`.
- `components/task/ExecutionEngineBadge.tsx` nhận `task: OrcaTask` và **suy luận phía client** (`useInferredExecutionEngine`: workflow, orchestration, direct_agent) và dùng lớp màu thô (`text-purple-600`...). Không dùng được cho hàng backlog (chỉ có `lastEngine` dạng chuỗi từ backend, và STYLEGUIDE cấm màu thô).
- `components/task/TaskBoardView.tsx` có `STATUS_ORDER` (dòng 9) với `backlog`; SOL-018 gỡ. Test hiện tại gọi `<TaskBoardView tasks onSelect />`.
- `hooks/useTaskDependencyEdges.ts` là N+1 (`task.getDependencies` từng task, đồng thời 4). Không dùng cho backlog: CR-015 đã trả `blocked_by_task_ids` theo lô.
- Phím tắt: không có sẵn bộ xử lý `j`/`k`; `lib/screen-submit-shortcut.ts` chỉ cho `Mod+Enter`. Màn này không cần phím sửa đổi.
- `components/confirmation-dialog.tsx` có `useConfirmationDialog()` (confirmVariant `destructive`), nhưng CR-023 yêu cầu `CancelRequestDialog` riêng có ô lý do; dùng `ui/dialog.tsx`.
- Chưa có `docs/ui/pages/requests.md` (SOL-018 sẽ thêm); task cuối bổ sung mục Backlog.

### Correction relative to CR-REQ-023 (đối chiếu CR-REQ-015, 016, 006)

| # | CR-REQ-023 viết | Thực tế theo CR backend | Xử lý |
|---|---|---|---|
| C1 | Một kênh `backlog.list {view,...}` (cũng ở CR-018) | CR-016 mục 2.4 chốt ba kênh: `backlog.requests`, `backlog.tasks`, `backlog.execute` (README mục 8 #12: tên kênh do CR-016 chốt) | Dùng ba kênh. `BACKLOG_RPC_BY_VIEW` map `request→backlog.requests`, `task→backlog.tasks`, `execute→backlog.execute` |
| C2 | Bộ lọc `type`, `q`; số mục `total` | Tham số server: `projectId?`, `groupBy?` (`reason`, chỉ `backlog.requests`), `planTaskId?` (tasks), `phaseTaskId?` (execute), `pageSize`, `pageToken`. Không `total`, không `q`, không `type` | `q` và `type` lọc ở client trên trang đã tải; nhãn số mục chỉ hiện cho phân đoạn đã tải, kèm `+` khi còn `nextPageToken` |
| C3 | Wire phẳng `TaskBacklogItem`, `ExecuteBacklogItem` | CR-015 trả `groups[]` (`BacklogGroup`: `requestId, planTaskId, planTitle, phaseTaskId, phaseTitle, gateStatus, tasks[]`) và `BacklogTaskRow` (`taskId,title,status,estimatedHours,assigneeId,blockedByTaskIds,lastEngine,lastLinkStatus,failedAttempts,lastError`) | Parser trả cấu trúc nhóm; bảng hiển thị hàng tiêu đề nhóm (Plan/Phase) rồi các hàng task. Ghi đè hai kiểu phẳng của SOL-018 bằng kiểu mới (task 01) |
| C4 | "Phụ thuộc" lấy từ `useTaskDependencyEdges` | `blockedByTaskIds` đã có theo lô (chỉ phụ thuộc chưa `done`/`cancelled`) | Bỏ N+1; tên task phân giải từ `tasks` trong store nếu có, không thì hiện id rút gọn |
| C5 | Cột "Thời điểm lỗi/cập nhật", "Số lần thử" = số `execution_link` | `BacklogTaskRow` không có mốc thời gian; `failedAttempts` chỉ đếm link `failed` | Bỏ cột thời gian cho tới khi backend thêm (parser đã đọc `lastStartedAt?` nếu có); đổi nhãn thành "Lần lỗi" |
| C6 | Mở lại "quay về bước `returned_from_stage`", có ghi chú tuỳ chọn | CR-006 mục 2.4: mở lại đưa Request về `classifying` (phân loại lại), giữ Solution/Plan/Task cũ làm tham chiếu; kênh `request.reopen {id}` không có `note` hay `version` | Nội dung hộp thoại nói Request sẽ được phân loại lại; bỏ ô ghi chú (thêm khi 016 thêm `note`) |
| C7 | Hủy `request.cancel` lý do tuỳ chọn | 016: `request.cancel {id, reason}`; chưa rõ `reason` bắt buộc | Gửi `reason` nếu có; nếu server trả `REQUEST_REASON_REQUIRED` thì chuyển ô thành bắt buộc (lỗi cạnh ô) |
| C8 | Cột lý do chỉ có `return_reason` | CR-015/006 thêm `returnedCategory` ∈ `missing_info, infeasible, blocked_dependency, rejected, other`, `parentRequestIds`, `returnedBy`, `returnedAt` | Thêm cột "Nhóm lý do" (badge) và bộ lọc nhóm ở client; `groupBy=reason` để dành cho phiên bản sau |
| C9 | Làm mới theo `request.returned`, `phase.started`... | Chỉ có `request.subscribe` → `request.event {requestId,eventType,status,type,occurredAt}` | `eventType` thuộc tập `request.returned`, `request.status_changed`, `plan.generated`, `phase.started`, `phase.completed`, `approval.decided` thì làm mới phân đoạn đang mở (debounce) |
| C10 | "Chưa chia Phase" như một trạng thái | CR-015 chỉ trả `gateStatus ∈ approved|pending|rejected|none` | `none` kèm `phaseTaskId` rỗng hiển thị "Chưa chia Phase" (suy luận, ghi rủi ro) |

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/
├─ shared/request-backlog-types.ts                    (mới) kiểu wire đã parse + parser
└─ renderer/src/
   ├─ hooks/useBacklog.ts                             (SOL-018 tạo; task 02 chốt lại chữ ký)
   ├─ components/request/backlog/                     (mới)
   │   ├─ BacklogTab.tsx                  lắp ghép, trạng thái phân đoạn, sự kiện
   │   ├─ BacklogSegmentControl.tsx       ui/toggle-group, số mục, phím 1/2/3
   │   ├─ BacklogToolbar.tsx              tìm kiếm, loại Request, nhóm lý do, Làm mới
   │   ├─ BacklogStates.tsx               Skeleton, Empty, Filtered, Error
   │   ├─ RequestBacklogTable.tsx, RequestBacklogRow.tsx
   │   ├─ TaskBacklogTable.tsx, TaskBacklogRow.tsx
   │   ├─ ExecuteBacklogTable.tsx, ExecuteBacklogRow.tsx
   │   ├─ BacklogGroupHeaderRow.tsx       tiêu đề Plan/Phase + gateStatus
   │   ├─ BacklogGateStatusBadge.tsx      approved|pending|rejected|none
   │   ├─ BacklogEngineBadge.tsx          nhãn engine từ chuỗi, chỉ dùng token
   │   ├─ BacklogTaskSheet.tsx            Sheet bọc TaskDetail
   │   ├─ ReopenRequestDialog.tsx, CancelRequestDialog.tsx
   │   └─ backlog-view-columns.ts         cột theo phân đoạn, bộ lọc client
   └─ i18n/request-backlog-locale-coverage.test.ts    (mới)
```

Sửa: `components/task/TaskBoardView.tsx` (+ test) thêm dòng gợi ý; `components/request/RequestPage.tsx` gắn `BacklogTab` (SOL-018). Tên file đều theo khái niệm.

### 2.2 Kiểu và parser (`shared/request-backlog-types.ts`)

```ts
export type BacklogView = 'request' | 'task' | 'execute'
export type GateStatus = 'approved' | 'pending' | 'rejected' | 'none' | 'unknown'
export type ReturnedCategory = 'missing_info'|'infeasible'|'blocked_dependency'|'rejected'|'other'|'unknown'
export type RequestBacklogRowData = { requestId; number; title; type; sourceProvider; sourceRef; sourceUrl?;
  returnedFromStage; returnedCategory; returnReason; returnedBy?; returnedAt?; parentRequestIds: string[] }
export type BacklogTaskRowData = { taskId; title; status; estimatedHours: number|null; assigneeId?;
  blockedByTaskIds: string[]; lastEngine?; lastLinkStatus?; failedAttempts: number; lastError?; lastStartedAt?: string }
export type BacklogGroupData = { requestId; planTaskId?; planTitle?; phaseTaskId?; phaseTitle?; gateStatus: GateStatus; tasks: BacklogTaskRowData[] }
export type BacklogPage<T> = { items: T[]; nextPageToken: string | null }
export function parseRequestBacklogPage(raw: unknown): BacklogPage<RequestBacklogRowData>
export function parseTaskBacklogPage(raw: unknown): BacklogPage<BacklogGroupData>   // dùng cho cả task và execute
```

Parser chịu thiếu trường và enum lạ (rơi về `'unknown'`), đọc camelCase (`requestRows`, `groups`, `nextPageToken`) và tạm chấp nhận snake_case để không vỡ nếu gateway lệch (CR-016 yêu cầu camelCase). `sourceUrl` chỉ giữ khi giao thức `http:`/`https:`.

### 2.3 Dữ liệu và kênh WS

| View | Kênh | Tham số | Phản hồi |
|---|---|---|---|
| request | `backlog.requests` | `{projectId?, pageSize: 50, pageToken?}` | `{requestRows[], nextPageToken}` |
| task | `backlog.tasks` | `{projectId?, planTaskId?, pageSize: 20, pageToken?}` | `{groups[], nextPageToken}` |
| execute | `backlog.execute` | `{projectId?, phaseTaskId?, pageSize: 20, pageToken?}` | `{groups[], nextPageToken}` |
| Mở lại | `request.reopen` | `{id}` | `OrcaRequest` mới (hoặc `{status}`) |
| Hủy | `request.cancel` | `{id, reason?}` | như trên |
| Tóm tắt Request (view task/execute) | `request.get` | `{id}` | tiêu đề, `number` |
| Sự kiện | `request.subscribe` | `{}` | `request.event` |

`pageSize` của task/execute tính theo Request (CR-015 mục 2.5), nên một trang có thể ít nhóm hơn; tự gọi tiếp khi trang rỗng nhưng còn `nextPageToken` (tối đa 3 lần liền). `useBacklog(view, filters)` giữ trạng thái riêng cho từng view (`items`, `nextPageToken`, `isLoading`, `error`, `loadedOnce`), chỉ tải view đang mở, giữ cache khi chuyển view, và lỗi một view không làm hỏng view khác. Mã `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` (view task/execute, không trả từng phần) hiển thị như lỗi mạng của riêng view đó. Mã `REQUEST_BACKLOG_BAD_PAGE_TOKEN` thì tải lại từ trang đầu.

### 2.4 Cột

- **Request**: Request (`#number`, title, bấm mở), Nguồn (`RequestSourceBadge`), Loại (`RequestTypeBadge`), Giai đoạn bị trả (`ReturnedFromStage.<stage>`), Nhóm lý do (`ReturnedCategory.<c>`), Lý do (2 dòng, tooltip đủ), Người trả (`system` hoặc rỗng hiển thị "AI/Hệ thống"), Thời điểm (tương đối, tooltip ngày giờ), Hành động (Mở lại, Hủy).
- **Task**: tiêu đề nhóm (Plan, `BacklogGateStatusBadge`, "Chưa chia Phase" theo C10, liên kết Request/tab Plan), rồi mỗi task: `#số` hoặc id rút gọn + tiêu đề + `TaskStatusBadge` (bấm mở `TaskDetail` trong Sheet), Estimate (`estimatedHours` hoặc `-`), Phụ thuộc (số + 2 tên đầu, chip `destructive` vì danh sách chỉ có phụ thuộc chưa xong).
- **Execute**: tiêu đề nhóm (Phase hoặc Plan, `gateStatus`), mỗi task: tiêu đề + `TaskStatusBadge`, Bị chặn bởi (như trên, rỗng nếu chỉ `open`), Lý do lỗi gần nhất (`lastError` 2 dòng, tooltip), Lần lỗi (`failedAttempts`), Engine (`BacklogEngineBadge`).

Task backlog và Execute backlog chỉ đọc. Không có nút ghi trạng thái, không có "Chạy lại" (CR-023 mục 2.4); chạy lại dùng nút trong `TaskDetail`.

### 2.5 Phím tắt

`useRowListKeyboardNavigation` (SOL-022 task 04) cho `j`/`k`/`Enter`. Thêm: phím `1`, `2`, `3` đổi phân đoạn khi tiêu điểm nằm trong thanh phân đoạn hoặc danh sách (bỏ qua trong `input`/`textarea`/`select`/`[contenteditable]`, bỏ qua khi có `ctrlKey/metaKey/altKey/shiftKey` hoặc `isComposing`). Không dùng phím sửa đổi nên không có nhánh Mac/Windows; chip phím trong tooltip phân đoạn dùng `ShortcutKeyCombo keys={['1']}`.

### 2.6 Trạng thái rỗng, tải, lỗi

Skeleton 8 hàng đúng số cột của phân đoạn; rỗng riêng cho ba phân đoạn; rỗng khi lọc + "Xoá bộ lọc"; `network` banner + "Thử lại" giữ dữ liệu cũ mờ; `forbidden` thông điệp riêng; `unsupported` (thiếu `backlog.requests`): SOL-018 ẩn tab, không toast. Chỉ một view lỗi thì chỉ view đó báo lỗi.

### 2.7 Hành động ghi và điều hướng

- Mở lại: `ReopenRequestDialog` hiện `returnedFromStage` để người dùng biết nó trở về từ đâu, và nói rõ Request sẽ được phân loại lại (C6). Sau thành công: bỏ hàng khỏi view, toast có nút "Xem Request" gọi `openRequestPage({section:'requests', requestId})`.
- Hủy: `CancelRequestDialog` `variant=destructive`, ô lý do tuỳ chọn (C7).
- Lỗi: `REQUEST_REOPEN_NOT_ALLOWED`/`REQUEST_TRANSITION_NOT_ALLOWED`/`REQUEST_STATE_STALE` nghĩa là người khác đã xử lý: toast trung tính, bỏ hàng, tải lại. `forbidden` (`REQUEST_APPROVAL_*` không áp dụng; `permission denied`): toast lỗi, giữ hàng. Hành động ghi dùng `useRequestActions` của SOL-018 (`reopen`, `cancel`, trả `Result`).
- Mở Task: `setActiveTask(taskId)` và Sheet; mở Plan: `openRequestPage({section:'requests', requestId, focus:'plan'})`.

### 2.8 Liên hệ cột Board `backlog`

`TaskBoardView` hiện dòng gợi ý `auto.components.task.TaskBoardView.backlogMoved` ("Task chưa chạy xem ở Requests > Backlog") khi `requestFlowSupport === 'supported'`, kèm nút nhỏ mở `openRequestPage({section:'backlog'})`. Không tạo status mới.

### 2.9 i18n

Khoá đọc theo tên dưới `auto.components.request.backlog.`, 5 locale: `BacklogSegmentControl.{request,task,execute}`, `BacklogToolbar.{search,type,category,refresh}`, `RequestBacklogTable.col.*`, `TaskBacklogTable.col.*`, `ExecuteBacklogTable.col.*`, `ReturnedFromStage.{classification,analysis,plan,phase,task}`, `ReturnedCategory.{missing_info,infeasible,blocked_dependency,rejected,other}`, `GateStatus.{approved,pending,rejected,none}`, `ReopenRequestDialog.{title,body,confirm,viewRequest}`, `CancelRequestDialog.{title,body,reason,confirm}`, `BacklogEmptyState.{request,task,execute,filtered}`, `BacklogErrorState.{network,forbidden,retry}`, `TaskBacklogRow.planNotSplit`, `BacklogRow.reopened`, `BacklogRow.alreadyHandled`; ngoài ra `auto.components.task.TaskBoardView.backlogMoved`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Ba kênh của CR-016, không `backlog.list` | README mục 8 #12; tránh gọi kênh không tồn tại |
| D2 | Không tính view ở client | README 3.8 duy nhất ở backend (CR-023 mục 3) |
| D3 | Không `useTaskDependencyEdges` | CR-015 trả sẵn `blockedByTaskIds`; tránh N+1 |
| D4 | Bảng có hàng tiêu đề nhóm thay vì bảng phẳng | Wire là `groups[]`; gom theo Plan/Phase đúng phân trang theo Request |
| D5 | Chỉ tải phân đoạn đang mở | Tiêu chí "chuyển phân đoạn không gọi lại phân đoạn khác"; đổi lại nhãn số mục chỉ có sau khi đã mở |
| D6 | Engine badge mới chỉ dùng token | `ExecutionEngineBadge` suy luận client và dùng màu thô |
| D7 | Task/Execute chỉ đọc | Không mở đường chạy agent thứ hai (CR-023 2.4) |

## 4. Phụ thuộc và thứ tự

1. SOL-018: kiểu cơ sở, `callRequestRpc`, `useRequestActions`, `useRequestEvents`/`subscribeRequestEvents`, `requestFlowSupport`, `RequestSourceBadge` (đã chặn `javascript:`), `RequestTypeBadge`, gỡ `backlog` khỏi `TaskStatus`. **Yêu cầu bổ sung cho SOL-018**: bỏ hai kiểu phẳng `TaskBacklogItem`, `ExecuteBacklogItem` và `BacklogView` trùng; dùng `shared/request-backlog-types.ts`; `useBacklog` chữ ký ở 2.3.
2. SOL-022 task 03, 04 (hook dùng chung).
3. Backend: CR-REQ-015 (kênh `backlog.*` qua CR-016), CR-REQ-006 (`reopen`, `cancel`). Làm trước bằng fake client.
4. Thứ tự task: 023-01 → 023-02 → (023-03, 023-04) → 023-05 → 023-06 → 023-07. Chi tiết ở `tasks/PARTIAL-INDEX-022-023.md`.

## 5. Kiểm thử

Lệnh (chưa chạy): `pnpm --filter orca-frontend test <đường dẫn>`.

- Unit: parser ba loại (thiếu trường, enum lạ, snake_case, `sourceUrl` không phải http), `backlog-view-columns` (cột theo phân đoạn, lọc client), nhãn `ReturnedFromStage`.
- Hook: `useBacklog` (phân trang, tải tiếp khi trang rỗng, polling, sự kiện, lỗi riêng từng view, cache khi chuyển view).
- Component: ba bảng (rỗng, lỗi, dữ liệu), `ReopenRequestDialog`, `CancelRequestDialog`, `BacklogSegmentControl` (phím `1/2/3`, bỏ qua ô nhập), `TaskBacklogRow`, `ExecuteBacklogRow` (cắt gọn lý do dài).
- Cập nhật: `components/task/__tests__/TaskBoardView.test.tsx` (không còn cột `backlog`, có dòng gợi ý khi `supported`).
- i18n: `request-backlog-locale-coverage.test.ts`.
- E2E (cần CR-REQ-006/013/015/016): trả Request về backlog rồi mở lại; task chưa có Plan duyệt vào Task backlog rồi rời đi sau khi duyệt; task chạy lỗi vào Execute backlog.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên trường camelCase của `backlog.*` chưa được CR-016 liệt kê (chỉ CR-015 mô tả proto). Parser chịu cả hai kiểu, nhưng cần xác nhận.
- C10 là suy luận: Plan "chưa chia Phase" và Plan "chưa duyệt" cùng `gateStatus`, phân biệt bằng `phaseTaskId` rỗng; có thể sai với loại không có Phase (CR-023 mục 6).
- Không có `total`: nhãn số chỉ gần đúng, và chỉ cho phân đoạn đã tải.
- Execute backlog lớn: chưa có lọc theo Phase ở UI (kênh có `phaseTaskId`, để phiên bản sau).
- `ListTasks`/`ListExecutionStates` không kiểm grant người gọi (CR-015 mục 6): rủi ro lộ task phải xử lý ở backend; frontend không bù được.
- `Sheet` bọc `TaskDetail` chưa kiểm chứng ngoài `TaskPage` (`TaskDetail` phụ thuộc `useWorkspace`).
- Lệnh typecheck/lint root trỏ cấu hình không có trong repo (chưa kiểm chứng).

## 7. Câu hỏi mở

1. Wire camelCase chính xác của `backlog.*` và có thêm `total`, `lastStartedAt` không?
2. `request.reopen` có nhận `note`/`expectedVersion` không (CR-006 proto có, CR-016 không)?
3. `request.cancel` có bắt buộc `reason` không?
4. Có cần "Chạy lại" ngay tại Execute backlog (CR-023 Q3)? Solution này không làm.
5. Task backlog cho loại không có Phase (CR-015 Q1): UI phân biệt thế nào "chưa duyệt Plan" và "chưa chia Phase"?
6. Task `review` (khi `REQUEST_AUTO_COMPLETE_TASKS` tắt) không thuộc view nào (CR-015 Q3): người dùng sẽ thấy "mất" task; cần câu trả lời sản phẩm.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/request-frontend/CR-REQ-023-backlog-screens.md`, `CR-REQ-018-request-frontend-foundation.md`
- `/opt/repos/orca/docs/crs/v6/backlog-views/CR-REQ-015-backlog-read-views.md`
- `/opt/repos/orca/docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md`
- `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/{TaskBoardView,TaskDetail,ExecutionEngineBadge,TaskStatusBadge}.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/task.ts` (`setActiveTask`)
- `/opt/repos/orca/frontend/src/renderer/src/hooks/useTaskDependencyEdges.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/ui/{table,toggle-group,sheet,dialog,skeleton,tooltip}.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/ShortcutKeyCombo.tsx`
- `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/docs/ui/pages/tasks.md`
