# CR-REQ-021 — Cây Plan → Phase → Task, duyệt Plan và Phase, tiến độ

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-021 |
| **Tên** | Cây Plan, Phase, Task trong chi tiết Request; duyệt Plan và Phase; tiến độ; lọc task `plan`/`phase` khỏi Board mặc định |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-018, 019, 020 (cùng `RequestDetailPane`, `RejectReasonDialog`); backend CR-REQ-011 (task `plan`/`phase`, cascade), CR-REQ-012 (sinh Plan/Phase/Task), CR-REQ-013 (thực thi, phản hồi ngược), CR-REQ-009 (Approval); kênh CR-REQ-016 |
| **Mở khoá** | CR-REQ-022, 023 |
| **Tác động** | `frontend/src/renderer/src/components/request/plan/` (mới), `hooks/useTasks.ts`, `components/task/{TaskGraph,TaskBoardView,TaskTreeView,TaskDAGView}.tsx`, `shared/task-types.ts`, `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Sau khi Solution được duyệt, `change_request` đi tiếp Plan → Phase → Task; `bug`, `refactor` có Plan (Phase khi `size=L`); `task` và `docs` có danh sách task (`task_list`), không Phase (README 3.4). Quyết định D2: Plan và Phase là Task với `type` `plan`, `phase`, nên chúng sẽ xuất hiện trong `task.list` và rơi vào Board, DAG, cây Task hiện có, làm sai thống kê và cho phép kéo-thả một Plan sang `done` (README mục 7: mọi nơi giả định "Task là việc làm được").

Hiện trạng code: `TaskGraph.tsx` lấy `filteredTasks` từ `useTasks` rồi đưa cho `TaskTreeView`, `TaskBoardView`, `TaskDAGView`; không lọc theo `type`. `TaskTreeView` dựng cây theo `parentId`. `OrcaTask` chưa có `requestId`. Chưa có UI duyệt Plan/Phase (README mục 1).

## 2. Giải pháp đề xuất

### 2.1 Cây component trong `RequestPlanTab`

```
RequestPlanTab                       props: request
├─ PlanSummaryHeader                 (tên Plan, trạng thái duyệt, tiến độ tổng, số Phase/Task)
│    └─ PlanApprovalBar              (Duyệt Plan / Từ chối / Sinh lại Plan)
├─ PlanTree                          (Plan → Phase → Task)
│    ├─ PhaseNode x N                (tiêu đề, trạng thái, ProgressBar, PhaseApprovalBar)
│    │    └─ PlanTaskRow x M         (#TG-N, tiêu đề, TaskStatusBadge, engine, estimate, bị chặn bởi)
│    └─ FlatTaskList                 (khi loại là task_list: không có Phase)
├─ PlanGateChips                     (các cổng liên quan: plan, phase, pre_deploy)
├─ PlanEmptyState / PlanErrorState / PlanSkeleton
└─ RejectReasonDialog                (dùng lại từ CR-REQ-020, lý do bắt buộc)
```

Bố cục: tiêu đề Plan ghim ở trên, cây cuộn bên dưới; bấm một Task mở `TaskDetail` hiện có trong `Sheet` (không đổi `activeView`). Nhấn mũi tên (hoặc phím `ArrowRight`/`ArrowLeft` khi hàng có tiêu điểm) mở/thu Phase.

### 2.2 Nguồn dữ liệu

- Plan: `request.plan_task_id` (README 3.5). Cây: hook mới `usePlanTree(requestId)` (`hooks/usePlanTree.ts`) gọi `task.list {projectId, requestId}`, dựng cây theo `parentId` bắt đầu từ `plan_task_id`. Tham số `requestId` cho `task.list` dựa vào cột `request_id` của CR-REQ-011; chưa kiểm chứng `task.list` có lọc theo cột này (mục 7). Không dùng `useTasks` vì nó thay toàn bộ `tasks` trong store.
- Approval: `useApprovals({requestId})` (`approval.list`) lấy các cổng `plan`, `task_list`, `phase`, `pre_deploy`; ghép với nút theo `subject_id` (id task Plan hoặc Phase).
- Sự kiện làm mới: `plan.generated`, `phase.started`, `phase.completed`, `approval.requested`, `approval.decided`, `request.status_changed`; thêm sự kiện task của `task-service` nếu có (xem CR-REQ-013), nếu không thì polling 15 giây khi Request `executing`.
- Frontend `OrcaTask` thêm `requestId?: string`; `TaskType` thêm `plan | phase` (đã do CR-REQ-018). `taskNumber` của Plan/Phase luôn trống (O2): UI không hiển thị `#TG-N` cho hai loại này, chỉ tiêu đề.

### 2.3 Trạng thái và tiến độ

- Trạng thái Plan/Phase là suy ra từ con (O1): UI chỉ đọc, không có dropdown trạng thái, không kéo-thả. `TaskStatusBadge` dùng nguyên.
- Tiến độ: dùng `progressPercent` mà backend cascade (CR-REQ-011), vẽ bằng `ui/progress.tsx`. Không tự tính lại ở client trừ khi trường trống: khi đó dùng `TASK_STATUS_PROGRESS` (đã bỏ `backlog` ở CR-REQ-018) làm dự phòng ghi nhãn "ước tính".
- Hiển thị tóm tắt Phase: `3/5 task xong`, số task `blocked` (chip `destructive`), task đang chạy (`primary`).
- Task thực thi do CR-REQ-013; nút "Chạy" của Task giữ nguyên cơ chế `TaskDetail`, CR này không thêm đường chạy mới. Phase chưa duyệt thì Task con ẩn nút Chạy kèm tooltip "Phase chưa được duyệt" (khớp điều kiện Execute backlog README 3.8).

### 2.4 Duyệt Plan và Phase

| Cổng (`subject_type`) | Hiện ở | Điều kiện hiện nút | RPC |
|---|---|---|---|
| `plan` | `PlanApprovalBar` | Approval `pending`, người xem có quyền | `approval.approve` / `approval.reject` |
| `task_list` | `PlanApprovalBar` (loại `task`, `docs`) | như trên | như trên |
| `phase` | `PhaseApprovalBar` trên từng Phase | Phase có Approval `pending` | như trên |
| `pre_deploy` | `PlanGateChips` + khối hành động riêng | Approval `pending` (`hotfix`, `security`, `ops_request`) | như trên |

- Từ chối bắt buộc nhập lý do (dùng `RejectReasonDialog`, tối thiểu 10 ký tự): Plan bị từ chối hiện lý do và nút "Sinh lại Plan" (`request.generatePlan`, kèm phản hồi nếu backend cho phép). Phase bị từ chối đưa Request về `planning` theo máy trạng thái CR-REQ-003.
- Duyệt Plan xong, với loại có Phase: Phase đầu hiện nút "Bắt đầu Phase" (`request.startPhase {requestId, phaseId}`) sau khi Phase đó được duyệt; Phase kế tiếp chỉ mở khi Phase trước `done` (nếu CR-REQ-013 quy định tuần tự; chưa kiểm chứng).
- Plan đã duyệt không sửa được tại chỗ; muốn đổi thì "Sinh lại Plan" tạo bản mới và bản cũ thành tham chiếu (README 3.3 về giữ phần đã làm).
- `Mod+Enter` gửi hộp từ chối; không có phím tắt cho Duyệt.

### 2.5 Lọc `plan`/`phase` khỏi Board mặc định

- Thêm `shared/task-hierarchy.ts` (mới): `isPlanningTask(task)` (type là `plan` hoặc `phase`) và `isWorkTask(task)`.
- `useTasks.ts`: `filteredTasks` mặc định loại bỏ `isPlanningTask`; thêm state `showPlanningTasks` (mặc định `false`) và công tắc "Hiện Plan/Phase" trong thanh công cụ `TaskGraph`. Vì cả `TaskTreeView`, `TaskBoardView`, `TaskDAGView` nhận `filteredTasks`, lọc một chỗ là đủ.
- Hệ quả cho cây: Task dưới Phase có `parentId` trỏ vào Phase đang bị lọc; `TaskTreeView` khi lọc phải coi Task mồ côi là gốc (nâng lên cấp gốc) và hiện đường dẫn `Plan / Phase` dạng chip nhỏ trên `TaskCard`. Cần chỉnh `renderLevel` và test.
- Bộ chọn "chọn nhiều để chạy hàng loạt" (`selectedIds`) không chọn được Plan/Phase.
- Thống kê tiến độ dự án và các nơi đếm Task rà lại để không đếm Plan/Phase (rà bằng tìm kiếm `tasks.filter`, kết quả chưa có).

### 2.6 Trạng thái rỗng, tải, lỗi

| Tình huống | UI |
|---|---|
| Request chưa tới `planning` | Tab hiện "Chưa lập Plan" kèm bước hiện tại từ dòng thời gian |
| `planning` (đang sinh) | Skeleton + "AI đang lập Plan" |
| Plan sinh lỗi | Lỗi + "Sinh lại Plan" |
| Plan không có Task | "Plan trống" + "Sinh lại" |
| `task.list` thiếu quyền (`forbidden`) | "Bạn không có quyền xem Task của Request này" |
| Kênh `request_id` không được runtime hỗ trợ (`unsupported`) | Tab hiện thông báo "Runtime chưa hỗ trợ xem Plan"; không hiện lỗi đỏ |
| Loại không có Plan (`question`, `spike`) | Không render tab |

### 2.7 i18n

Tiền tố `auto.components.request.plan.`: `RequestPlanTab.empty`, `RequestPlanTab.generating`, `PlanApprovalBar.{approve,reject,regenerate}`, `PhaseApprovalBar.{approve,reject,start,notApproved}`, `PlanSummaryHeader.progress`, `PlanTaskRow.blockedBy`, `PlanGateChips.preDeploy`, `PlanErrorState.forbidden`, `PlanErrorState.unsupported`, `TaskGraph.showPlanning`, đủ 5 locale.

## 3. Quyết định thiết kế

- Cây Plan nằm trong chi tiết Request (không phải Board): người dùng ra quyết định theo Request.
- Lọc ở `useTasks` một điểm; công tắc cho người cần xem.
- Trạng thái Plan/Phase chỉ đọc (O1); tiến độ lấy từ backend.
- `usePlanTree` riêng, không đụng `tasks` của store để khỏi ghi đè dữ liệu Board.
- Từ chối bắt buộc lý do, nhất quán với CR-REQ-020.

## 4. Tiêu chí chấp nhận

- [ ] Request `change_request` đã `planning` hiện cây Plan → Phase → Task đúng thứ tự, tiến độ khớp `progressPercent`.
- [ ] `task` và `docs` hiện danh sách phẳng (`task_list`), không có Phase.
- [ ] Plan, Phase không hiện `#TG-N`; không có cách đổi trạng thái tay.
- [ ] Board, cây Task, DAG mặc định không có task `plan`/`phase`; công tắc "Hiện Plan/Phase" hiển thị chúng; Task con của Phase vẫn thấy (nâng lên gốc) kèm chip đường dẫn.
- [ ] Duyệt Plan, duyệt Phase, duyệt `pre_deploy` gọi `approval.approve` đúng `subject_id`.
- [ ] Từ chối Plan/Phase với lý do rỗng: nút bị khoá, không gửi.
- [ ] Task dưới Phase chưa duyệt không có nút Chạy và có tooltip giải thích.
- [ ] Sau `phase.completed`, cây cập nhật không cần tải lại trang.
- [ ] Runtime không có `requestId` trên `task.list`: tab báo không hỗ trợ, không lỗi đỏ.
- [ ] Mọi chuỗi mới có 5 locale; nhãn phím khớp binding theo nền tảng.

## 5. Kiểm thử

- Unit: `isPlanningTask`; dựng cây từ danh sách phẳng (Task mồ côi, vòng lặp `parentId`); lọc trong `useTasks` với và không có công tắc; tiến độ dự phòng.
- Component: `PlanTree` (mở/thu, tiêu điểm bàn phím), `PhaseApprovalBar`, `PlanApprovalBar` (quyền, từ chối), `TaskTreeView` với Task mồ côi, `TaskGraph` công tắc; cập nhật `TaskBoardView.test.tsx`, `TaskTreeView.test.tsx`, `TaskDAGView.test.tsx`.
- Hook: `usePlanTree` (huỷ khi unmount, sự kiện, polling).
- E2E (cần CR-REQ-011/012/013): từ Solution đã duyệt đến Plan, duyệt Plan, bắt đầu Phase, Task xong, Phase `done`.
- Chưa chạy; kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- `task.list` có lọc `requestId` hay không chưa kiểm chứng; nếu không, phải tải cả dự án rồi lọc client (đắt).
- Các nơi khác dùng `tasks` của store (thống kê, thông báo, `TaskBatchExecution`) có thể đếm Plan/Phase; chưa rà hết.
- Nâng Task mồ côi lên gốc thay đổi hành vi `TaskTreeView` cho cả dự án không dùng Request; phải bọc sau điều kiện "có Plan/Phase trong dữ liệu".
- Quy tắc tuần tự giữa các Phase chưa có trong README.
- `TaskType` frontend vốn lệch backend (`story|subtask|spike`).

## 7. Câu hỏi mở

1. `task.list` nhận `requestId` hoặc `parentId` (lấy cây con theo Plan)? Cần CR-REQ-011/016 xác nhận; hay có `GetSubtree` (nhắc ở CR-TG-007).
2. Có RPC riêng `request.getPlan` trả cây hoàn chỉnh kèm Approval để giảm số lời gọi?
3. Phase chạy tuần tự hay song song; `StartPhase` có bắt buộc khi Phase đã được duyệt?
4. Từ chối Phase đưa Request về `planning` hay chỉ Phase đó về `draft`?
5. Cho sửa tay Task trong Plan chưa duyệt (đổi tiêu đề, xoá) hay chỉ "Sinh lại"?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (D2, O1, O2, 3.4, 3.5, 3.8)
- `/opt/repos/orca/frontend/src/renderer/src/hooks/useTasks.ts`, `hooks/useTask.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/{TaskGraph,TaskGraphPanel,TaskTreeView,TaskBoardView,TaskDAGView,TaskCard,TaskDetail,TaskStatusBadge}.tsx`
- `/opt/repos/orca/frontend/src/shared/task-types.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/ui/{progress,sheet,collapsible}.tsx`
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-007-frontend-task-crud-board-grant-ui.md`
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- Mới: `components/request/plan/*`, `hooks/usePlanTree.ts`, `shared/task-hierarchy.ts`
