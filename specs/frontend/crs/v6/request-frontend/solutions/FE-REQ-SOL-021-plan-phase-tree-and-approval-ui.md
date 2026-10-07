# FE-REQ-SOL-021: Cây Plan → Phase → Task, duyệt Plan/Phase, lọc khỏi Board

> 🔴 **Not Started.** Rà soát 2026-10-07: chỉ có `plan-approval-model.ts` (skeleton cho 021-04), tất cả tasks UI chưa bắt đầu.

**CR:** [CR-REQ-021](../../../../../../docs/crs/v6/request-frontend/CR-REQ-021-plan-phase-tree-and-approval-ui.md)
**Area:** frontend (`components/request/plan/`, `components/task/*`, `hooks/useTasks.ts`)
**Hợp đồng backend:** CR-REQ-016 mục 2.3 (`request.generatePlan`, `request.startPhase`), 2.4 (`approval.*`); `task.list` của `task-service`; CR-REQ-011/012/013 phía backend. CONTRACT chưa có: chỗ tạm ghi "(tạm)".
**TDD tham chiếu:** [v5/15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md)

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `hooks/useTasks.ts` (`setTasks(response.tasks ?? [])` dòng 63; `filteredTasks` dòng 78), `components/task/TaskGraph.tsx` (dùng `filteredTasks` cho cây, Board, DAG; dòng 28, 247-268), `TaskTreeView.tsx` (`renderLevel` chỉ bắt đầu từ `parentId === null`, dòng 10-30), `TaskDetail.tsx` (không nhận props, đọc `activeTaskId` từ store dòng 42-44; `handleRunAgent` dòng 125 gọi `task.execute` dòng 142), `TaskCard.tsx`, `shared/task-types.ts`, `backend-go/.../wscompat/channels_automation_task.go:343`.

**Correction relative to CR-REQ-021:**

| # | CR-021 | Thực tế | Quyết định |
|---|---|---|---|
| 1 | `task.list {projectId, requestId}` | `task.list` chỉ nhận `projectId`, `pageToken`, `pageSize` (đã đọc `channels_automation_task.go:343-355`); cũng không có `getSubtree` | Tạm: `usePlanTree` gọi `task.list {projectId}` (phân trang đến hết) và dựng cây phía client từ `request.planTaskId` xuống theo `parentId`. Khi CR-REQ-011/016 thêm `requestId` hoặc `planTaskId`, đổi một dòng trong `usePlanTree` |
| 2 | Task dưới Phase bị lọc sẽ "nâng lên gốc" trong `TaskTreeView` | `renderLevel` chỉ lấy `parentId===null`: Task con của Phase bị lọc sẽ biến mất, không tự nâng | Phải sửa `TaskTreeView` (hoặc lọc trong `useTasks` bằng cách gán lại `parentId` hiệu dụng) |
| 3 | `request.startPhase {requestId, phaseId}` | CR-016: `request.startPhase {id, phaseTaskId}` | Dùng CR-016 |
| 4 | `approval.approve {approvalId, comment, version}` | `{id, expectedVersion, expectedDigest, comment?}` | Dùng CR-016 (như SOL-020) |
| 5 | "Mở `TaskDetail` trong `Sheet`" | `TaskDetail()` không có props, đọc `activeTaskId` từ store | Đặt `setActiveTask(id)` rồi render `TaskDetail` trong `Sheet`; chưa kiểm chứng `TaskDetail` chịu được bố cục `Sheet` |
| 6 | Ẩn nút Chạy khi Phase chưa duyệt | `TaskDetail` không biết ngữ cảnh Phase | Thêm `executionGateByTaskId` vào request slice; `TaskDetail` đọc và khoá nút; backend vẫn là nguồn chân lý (CR-REQ-013) |

Tiến độ: `OrcaTask.progressPercent` có sẵn (`task-types.ts`), cascade phía backend do CR-REQ-011 (chưa triển khai).

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/shared/task-hierarchy.ts                       (mới) isPlanningTask, isWorkTask, buildPlanSubtree
frontend/src/renderer/src/hooks/usePlanTree.ts              (mới)
frontend/src/renderer/src/hooks/useTasks.ts                 (sửa) lọc plan/phase + showPlanningTasks
frontend/src/renderer/src/components/task/TaskTreeView.tsx  (sửa) Task mồ côi
frontend/src/renderer/src/components/task/TaskGraph.tsx     (sửa) công tắc "Hiện Plan/Phase"
frontend/src/renderer/src/components/task/TaskCard.tsx      (sửa) chip đường dẫn Plan / Phase
frontend/src/renderer/src/components/task/TaskDetail.tsx    (sửa) khoá Chạy theo executionGate
frontend/src/renderer/src/components/request/plan/
  RequestPlanTab.tsx  PlanSummaryHeader.tsx  PlanTree.tsx  PhaseNode.tsx  PlanTaskRow.tsx
  PlanApprovalBar.tsx PhaseApprovalBar.tsx   PlanGateChips.tsx  PlanStates.tsx
  plan-approval-model.ts                                    (mới) hàm thuần: Approval ↔ Plan/Phase
```

### 2.2 `usePlanTree(request)`

- Đầu vào: `request {id, projectId, planTaskId, type, size}`. Gọi `task.list {projectId, pageSize:200, pageToken}` lặp tới hết trang (dừng khi `nextPageToken` rỗng, tối đa 20 trang, quá thì cờ `truncated`), `parseTask` + `normalizeTask` (018-06). Không dùng `useTasks` (nó thay toàn `tasks` trong store).
- `buildPlanSubtree(tasks, planTaskId)`: BFS theo `parentId` từ Plan, chống vòng lặp (tập đã thăm), trả `{plan, phases[], tasksByPhase, flatTasks}`; loại `task_list` (không Phase): `flatTasks` con trực tiếp của Plan.
- Approval: `useApprovals({requestId})` rồi `plan-approval-model.ts#attachApprovals(tree, approvals)` ghép theo `subjectId` (id Task Plan hoặc Phase): `plan`, `task_list`, `phase`, `pre_deploy`.
- Làm mới: sự kiện `plan.generated`, `phase.started|completed`, `approval.*`, `request.status_changed` qua `request-event-bus`; khi `executing` mà không có luồng sự kiện thì polling 15 giây (hook dừng khi tab ẩn).
- Task `plan`/`phase` không có `#TG-N` (O2): UI không in `taskNumber` cho hai loại này.

### 2.3 Hiển thị

- `PlanSummaryHeader`: tên Plan, trạng thái duyệt, tiến độ tổng (`Progress` từ `progressPercent` của Plan; nếu trống dự phòng `TASK_STATUS_PROGRESS` nhãn "ước tính"), số Phase/Task.
- `PhaseNode`: tiêu đề, `TaskStatusBadge`, `Progress`, tóm tắt `3/5 task xong`, chip `blocked` (`destructive`) và đang chạy (`primary`); thu/mở bằng `Collapsible` (`ui/collapsible.tsx`); `ArrowRight/ArrowLeft` mở/thu khi hàng có tiêu điểm.
- `PlanTaskRow`: tiêu đề, `#TG-N`, `TaskStatusBadge`, `ExecutionEngineBadge`, ước lượng, "bị chặn bởi"; bấm thì `setActiveTask(id)` và mở `Sheet` chứa `TaskDetail`.
- Trạng thái Plan/Phase chỉ đọc (O1): không dropdown, không kéo-thả.

### 2.4 Duyệt

| Cổng | Hiện ở | Điều kiện | RPC |
|---|---|---|---|
| `plan` / `task_list` | `PlanApprovalBar` | Approval `pending` với `subjectId = plan.id` | `approval.approve/reject` |
| `phase` | `PhaseApprovalBar` | Approval `pending` với `subjectId = phase.id` | như trên |
| `pre_deploy` | `PlanGateChips` + khối hành động | Approval `pending` (hotfix, security, ops_request) | như trên |

- Duyệt gửi `{id, expectedVersion, expectedDigest, comment?}`; từ chối bắt buộc lý do qua `RejectReasonDialog` (SOL-020), "Sinh lại Plan" gọi `request.generatePlan {id}` (không có tham số phản hồi trong CR-016; lý do từ chối chỉ lưu ở Approval). Không phím tắt cho Duyệt; `Mod+Enter` gửi hộp từ chối.
- "Bắt đầu Phase": `request.startPhase {id, phaseTaskId}` khi Phase có Approval `approved` và chưa chạy; tính tuần tự giữa các Phase là câu hỏi mở (CR-REQ-013), UI chỉ dựa lỗi `REQUEST_TRANSITION_NOT_ALLOWED`.
- Plan đã duyệt không sửa tại chỗ; muốn đổi thì "Sinh lại Plan".
- Task dưới Phase chưa `approved`: `executionGateByTaskId[taskId]='phase_not_approved'`; `TaskDetail` khoá "Run agent" kèm tooltip; backend vẫn kiểm.

### 2.5 Lọc Plan/Phase khỏi Board (D2)

- `task-hierarchy.ts`: `isPlanningTask(t)` (`type` là `plan|phase`), `isWorkTask`.
- `useTasks.ts`: `showPlanningTasks` (mặc định `false`) và `filteredTasks` bỏ `isPlanningTask` khi tắt; công tắc "Hiện Plan/Phase" ở toolbar `TaskGraph`.
- `TaskTreeView.renderLevel` hiện chỉ lấy `parentId===null`. Cách làm: trong `useTasks`, khi lọc, tính `effectiveParentId` (đi lên qua các tổ tiên bị lọc tới tổ tiên đầu tiên hiển thị hoặc `null`) và đưa danh sách đã gán lại `parentId` hiệu dụng cho cả ba view (Tree, Board, DAG); `TaskCard` hiện chip `Plan / Phase` từ đường dẫn gốc. Chỉ kích hoạt khi dữ liệu có Task `plan|phase` (dự án không dùng Request giữ nguyên hành vi).
- `selectedIds` (chạy hàng loạt) không chọn được Plan/Phase. Rà các nơi đếm Task trong `tasks.filter` để không đếm Plan/Phase (kết quả rà chưa có).

### 2.6 Trạng thái rỗng, tải, lỗi

| Tình huống | UI |
|---|---|
| Request chưa tới `planning` | "Chưa lập Plan" + bước hiện tại |
| `planning`, chưa có Plan | Skeleton + "AI đang lập Plan" |
| Sinh lỗi | Lỗi + "Sinh lại Plan" |
| Plan không có Task | "Plan trống" + "Sinh lại" |
| `task.list` `forbidden` | "Bạn không có quyền xem Task của Request này" |
| `planTaskId` rỗng hoặc Task Plan không có trong danh sách | "Chưa tìm thấy Plan" (không lỗi đỏ) |
| Quá 20 trang | Cảnh báo "Danh sách bị cắt" |
| Loại không có Plan (`question`, `spike`) | Không render tab |

### 2.7 i18n

Tiền tố `auto.components.request.plan.`: như CR-021 2.7 cộng `PlanStates.*`, `PlanTaskRow.*`, `PhaseNode.summary`, `TaskGraph.showPlanning`, `TaskDetail.phaseNotApproved`; đủ 5 locale.

## 3. Quyết định thiết kế

- Cây Plan nằm trong chi tiết Request (người dùng quyết định theo Request), không phải Board.
- Lọc ở `useTasks` một điểm; gán `parentId` hiệu dụng ở cùng điểm.
- `usePlanTree` riêng để không ghi đè `tasks` của store.
- Dựng cây ở client là tạm vì `task.list` chưa có lọc; thay bằng `requestId`/`planTaskId` khi có.
- Trạng thái chỉ đọc; tiến độ từ backend.

## 4. Phụ thuộc và thứ tự

SOL-018 (đặc biệt 018-06), SOL-019, SOL-020 (`RejectReasonDialog`). Backend CR-REQ-011/012/013. Mở khoá SOL-022, 023 (liên kết sang Plan). Thứ tự: 021-01 → 021-02 → 021-03 → 021-04 → 021-05 → 021-06. 021-01 có thể làm sớm (không cần backend).

## 5. Kiểm thử

Vitest: `isPlanningTask`; `buildPlanSubtree` (vòng lặp, mồ côi, `task_list`); lọc và `parentId` hiệu dụng; `attachApprovals`; `usePlanTree` (huỷ, phân trang, polling); `PlanTree`, `PhaseApprovalBar`, `PlanApprovalBar`; `TaskTreeView` với Task mồ côi; `TaskDetail` khoá Chạy; cập nhật `TaskBoardView.test.tsx`, `TaskTreeView.test.tsx`, `TaskDAGView.test.tsx`, `TaskGraph.test.tsx`. E2E `tests/e2e/request-plan-tree.spec.ts` (mới; cần backend 011/012/013; chưa chạy). Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/request/plan src/renderer/src/components/task`.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tải cả dự án để dựng cây là đắt; chỉ chấp nhận tạm.
- Thay đổi `useTasks` và `TaskTreeView` ảnh hưởng mọi dự án: bọc sau điều kiện có Task `plan|phase`.
- Các nơi dùng `tasks` của store (thống kê, thông báo, `TaskBatchExecution`) có thể đếm Plan/Phase; chưa rà hết.
- `TaskDetail` trong `Sheet` chưa kiểm chứng.
- Quy tắc tuần tự Phase chưa có.

## 7. Câu hỏi mở

1. `task.list` thêm `requestId`/`planTaskId`, hay có `GetSubtree`, hay `request.getPlan` trả cây kèm Approval?
2. Phase chạy tuần tự hay song song; `StartPhase` bắt buộc không?
3. Từ chối Phase đưa Request về `planning` hay chỉ Phase về `draft`?
4. Cho sửa tay Task trong Plan chưa duyệt?
5. `generatePlan` có nhận phản hồi từ lý do từ chối?

## 8. Tham chiếu

`/opt/repos/orca/docs/crs/v6/README.md` (D2, O1, O2, 3.8, mục 8), `/opt/repos/orca/docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md`, `/opt/repos/orca/frontend/src/renderer/src/hooks/useTasks.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskGraph.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskTreeView.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskDetail.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskCard.tsx`, `/opt/repos/orca/frontend/src/shared/task-types.ts`, `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/channels_automation_task.go`, `/opt/repos/orca/guides/STYLEGUIDE.md`.
