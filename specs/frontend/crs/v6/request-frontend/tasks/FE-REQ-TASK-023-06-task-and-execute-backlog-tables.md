# FE-REQ-TASK-023-06: `TaskBacklogTable` và `ExecuteBacklogTable` (chỉ đọc, nhóm theo Plan/Phase)

**From Solution:** [FE-REQ-SOL-023](../solutions/FE-REQ-SOL-023-backlog-screens.md) mục 2.4, bảng Correction C3 đến C5, C10
**Priority:** P1
**Area:** frontend (components)
**File:** trong `frontend/src/renderer/src/components/request/backlog/` (mới): `TaskBacklogTable.tsx`, `TaskBacklogRow.tsx`, `ExecuteBacklogTable.tsx`, `ExecuteBacklogRow.tsx`, `BacklogGroupHeaderRow.tsx`, `BacklogGateStatusBadge.tsx`, `BacklogEngineBadge.tsx`, `BacklogTaskSheet.tsx` và test cùng tên
**Depends on:** FE-REQ-TASK-023-01, 023-04; FE-REQ-TASK-022-03 (`useRequestSummaries`); FE-REQ-SOL-018 (`TaskStatus` đã gỡ `backlog`, `openRequestPage`)
**Status:** [x] DONE (verified 2026-10-07: BacklogGroupTables.test.tsx 6/6, BacklogGateStatusBadge.test.tsx 6/6; oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- `TaskDetail` (`components/task/TaskDetail.tsx`) không nhận props, đọc `activeTaskId` từ store; mở bằng `setActiveTask(id)` (`store/slices/task.ts:37`). Nó dùng `useWorkspace()`, nên `BacklogTaskSheet` phải render trong cây có provider tương ứng (như `TaskPage`); kiểm bằng test render.
- `components/task/TaskStatusBadge.tsx` có sẵn (SOL-018 gỡ cấu hình `backlog`, fallback `open`). `components/task/ExecutionEngineBadge.tsx` suy luận engine từ `OrcaTask` bằng `useInferredExecutionEngine` và dùng lớp màu thô, nên không dùng cho hàng backlog (chỉ có chuỗi `lastEngine` từ backend).
- Dữ liệu theo nhóm (CR-015): `{requestId, planTaskId?, planTitle?, phaseTaskId?, phaseTitle?, gateStatus, tasks[]}`; hàng task có `blockedByTaskIds` (chỉ phụ thuộc chưa `done`/`cancelled`), `lastEngine`, `lastLinkStatus`, `failedAttempts`, `lastError`, `estimatedHours`. Không có mốc thời gian (C5) và không có `total`.
- Không dùng `useTaskDependencyEdges` (N+1) vì `blockedByTaskIds` đã có.
- Task và Execute chỉ đọc: không nút ghi, không "Chạy lại" (CR-023 mục 2.4).

## Việc cần làm

1. `BacklogGateStatusBadge({status})`: `approved` dùng `status-success`, `pending` dùng `primary`, `rejected` dùng `destructive`, `none`/`unknown` dùng `muted-foreground`; mỗi giá trị có icon lucide và nhãn `GateStatus.<value>` (không dựa riêng vào màu).
2. `BacklogEngineBadge({engine})`: `workflow`, `orchestration`, `direct_agent` (ba engine của CR-TG-008) → nhãn dịch; giá trị lạ hoặc rỗng → `-`. Chỉ dùng `Badge variant="outline"` và token, không màu thô.
3. `BacklogGroupHeaderRow({group, requestTitle})`: dòng gộp cột; Phase hoặc Plan (`phaseTitle ?? planTitle ?? '-'`), `BacklogGateStatusBadge`, tiêu đề Request (qua `useRequestSummaries`, thiếu thì id rút gọn) và liên kết "Mở Plan" → `openRequestPage({section:'requests', requestId, focus:'plan'})`. Khi `gateStatus==='none'` và `phaseTaskId` rỗng hiện `TaskBacklogRow.planNotSplit` (suy luận C10, chỉ ở view task).
4. `TaskBacklogRow({task, resolveTaskLabel, onOpenTask})`: cột Task (id rút gọn hoặc `#số` nếu có trong store, tiêu đề, `TaskStatusBadge`; bấm mở `BacklogTaskSheet`), Estimate (`estimatedHours` hoặc `-`), Phụ thuộc (số, hai tên đầu qua `resolveTaskLabel` đọc `tasks` trong store hoặc id rút gọn, `+N` còn lại trong tooltip; chip `destructive` vì danh sách chỉ có phụ thuộc chưa xong).
5. `ExecuteBacklogRow`: Task như trên, "Bị chặn bởi" (rỗng nếu chỉ `open`), "Lý do lỗi gần nhất" (`lastError` 2 dòng, tooltip đầy đủ; khi `lastLinkStatus==='failed'` mà `lastError` rỗng hiện `ExecuteBacklogTable.noErrorDetail`), "Lần lỗi" (`failedAttempts`), Engine (`BacklogEngineBadge`). Không cột thời gian (ghi chú trong code ngắn: backend chưa trả mốc).
6. `TaskBacklogTable`, `ExecuteBacklogTable`: dựng bảng từ `BacklogGroupData[]`, hàng tiêu đề nhóm rồi các hàng task; `useRowListKeyboardNavigation` trên danh sách task (`Enter` mở `BacklogTaskSheet`); tiêu đề cột dính. Phân trang "Tải thêm" khi `hasMore`.
7. `BacklogTaskSheet({taskId, onClose})`: `Sheet` (`ui/sheet.tsx`); khi mở gọi `setActiveTask(taskId)`, render `<TaskDetail />`; khi đóng gọi `setActiveTask(null)` và khôi phục `activeTaskId` trước đó nếu có.
8. Hành động còn lại: bấm Plan/Request chỉ điều hướng; sau khi đóng Sheet gọi `refetch` của phân đoạn (task có thể đã đổi trạng thái).

## Kiểm thử

- `TaskBacklogRow.test.tsx`: Estimate `-` khi null; phụ thuộc chưa xong dùng chip `destructive`; `+N` khi quá hai; `planNotSplit` chỉ khi `gateStatus==='none'` và không có `phaseTaskId`.
- `ExecuteBacklogRow.test.tsx`: lý do dài cắt 2 dòng và tooltip; `failed` không có `lastError` hiện dòng thay thế; `lastEngine` lạ → `-`; không có nút ghi trạng thái (`queryByRole('button', {name:/run|execute|status/i})` rỗng ngoài nút mở).
- `BacklogGateStatusBadge.test.tsx`: bốn giá trị + `unknown`, chỉ lớp token.
- `TaskBacklogTable.test.tsx` / `ExecuteBacklogTable.test.tsx`: nhóm theo Plan/Phase; `Enter` mở Sheet và `setActiveTask`; mở Plan gọi `openRequestPage` có `focus:'plan'`.
- Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request/backlog`.

## Tiêu chí hoàn thành

- [ ] Task backlog hiện Plan, trạng thái duyệt Plan (`gateStatus`), estimate, phụ thuộc; Execute backlog hiện Phase, bị chặn bởi, lý do lỗi gần nhất, lần lỗi, engine.
- [ ] Không nút ghi trạng thái; không còn cột `backlog` ở Board.
- [ ] Không gọi `task.getDependencies` từ màn này.
- [ ] Hàng mở đúng đích: Request, Task (`TaskDetail` trong Sheet), Plan (tab Plan).
- [ ] Không màu hex hoặc lớp màu thô.

## Rủi ro và lưu ý

- `TaskDetail` trong Sheet chưa kiểm chứng ngoài `TaskPage`/`WorkspaceLayout` (phụ thuộc `useWorkspace`); nếu lỗi, dùng `openTaskPage({...})` thay vì Sheet và ghi vào báo cáo.
- Tên engine backend (`workflow|orchestration|direct_agent`) là giả định theo CR-TG-008; chuỗi lạ hiển thị `-`.
- Task nằm sâu hơn một cấp bị backend bỏ qua (CR-015 Q4); UI không bù được.

## Ghi chú triển khai (2026-10-07)

- Gộp thân hai bảng vào `BacklogGroupTable` (`TaskBacklogTable`/`ExecuteBacklogTable` là vỏ mỏng); test hàng gộp trong `BacklogGroupTables.test.tsx`. `BacklogTaskSheet` được mock `TaskDetail` trong test (chưa thử ngoài `TaskPage`, xem rủi ro SOL-023). Sau khi đóng Sheet gọi `refetch`.
