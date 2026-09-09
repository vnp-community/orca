# CR-TG-007 — Frontend: Task CRUD/Tree/DAG Fixes, Board View, Grant/Share UI, Run-Agent UX

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-007 |
| **Tên** | Đóng toàn bộ gap frontend của F37 — CRUD, DAG thật, **Board/Kanban view mới**, **Grant/Share modal mới**, sửa Run-Agent UX, Activity Feed |
| **Loại** | Feature |
| **Priority** | P0 (Board + Grant UI là 2 gap chính ma trận hoàn thành nêu ra) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-TG-001](./CR-TG-001-orcatask-data-model-widening.md)/[002](./CR-TG-002-ai-decompose-context-and-dependency-edges.md)/[003](./CR-TG-003-task-access-control-team-scope-and-sharing.md) (field/RPC UI cần gọi); [`CR-FLOW-TASK-005`](../../v3/flow-task/CR-FLOW-TASK-005-frontend-unified-task-execution-ui.md) (áp dụng `useTaskActivity`/engine badge, không thiết kế lại); [`CR-FLOW-TASK-003`](../../v3/flow-task/CR-FLOW-TASK-003-unified-task-activity-event-catalog.md) (kênh event cho phần real-time — có thể ship interim polling trước) |
| **Tác động** | `frontend/src/renderer/src/components/task/*` (9 file hiện có + file mới), `hooks/useTasks.ts`, `hooks/useTask.ts` |

---

## 1. Vấn đề

Kiểm chứng trực tiếp `frontend/src/renderer/src/components/task/` (9 file,
650 dòng) xác nhận: `grep -rli "board|kanban"` và
`grep -rli "grantmodal|sharemodal|permissionmodal"` trên toàn thư mục đều
**0 kết quả** — đúng như ma trận hoàn thành đã nêu, không có Board view và
không có Grant/Share modal ở bất kỳ đâu.

Chi tiết từng gap (đối chiếu `specs/frontend/bugs/task-v1/BUG-FE-TASKV1-001..006`):

### C1 — Không có "New Task" UI; tree lọc client-side; progress không tự cập nhật
`grep "'task.create'"` toàn `frontend/src` = 0 kết quả — đường duy nhất tạo
task là gián tiếp qua AI decompose. `TaskGraph.tsx:8-26`'s toolbar chỉ có
Search + Status filter (3 giá trị `all/todo/in_progress/done` — thiếu 4 status
CR-TG-001 vừa thêm) + toggle Tree/DAG — **không có nút "New Task"**.
`TaskTreeView.tsx`'s `renderLevel()` filter `tasks.filter(t =&gt; t.parentId ===
parentId)` client-side trên toàn mảng đã load — chính lý do cần
`GetSubtree` (CR-TG-001) thay vì tiếp tục load-all-rồi-filter.

### C2 — `TaskDAGView` luôn rỗng (bug tự thú trong code)
```
// TaskDAGView.tsx — buildDAGLayout(), dòng 30-33, 106:
// "OrcaTask has no embedded 'dependsOn' — edges live in a separate table
// (task.getDependencies per task). Always [] until that's wired in
// (out of scope here)."
```
`(task as any).dependsOn ?? []` → luôn `undefined ?? []` → mọi task rơi vào
wave 0, không edge nào render — **bất kể dependency thật trong DB**. Trong
khi `TaskDetail.tsx:42-58` gọi đúng `task.getDependencies` và hiển thị được
("← Blocked by"/"→ Blocks") — chứng minh RPC hoạt động, chỉ chưa được
`TaskDAGView` tiêu thụ. Không UI nào thêm/xoá dependency edge
(`grep "AddEdge|addDependency"` trên `renderer/src` = 0), dù `AddEdge` đã có
ở backend.

### C3 — Không có Grant/Share UI ở bất kỳ đâu
`grep "task.grant|resolvePermission"` trên `frontend/src` = 0 kết quả.
`TaskDetail.tsx` chỉ có 3 tab: Details/Subtasks/AI — không có tab
Access/Sharing. UI hiện tại **không bao giờ ẩn nút** theo quyền — hoàn toàn
dựa vào backend từ chối.

### C4 — "Run with Agent": input người dùng gõ bị bỏ qua hoàn toàn (UX-lie)
`TaskPromptEditor.tsx`'s nút "Run with Agent" disable theo `!prompt.trim()`
(dòng 52) — ngụ ý nội dung gõ có tác dụng. Nhưng `runWithAgent()`
(dòng 15-39) gọi `task.execute` với `{taskId, projectId, worktreePath,
traceId}` — **không có field `prompt`**, tự thú trong comment (dòng 24-25):
*"task.execute has no 'prompt' param — the executor builds the agent prompt
server-side from the task's own promptTemplate."* Người dùng gõ gì cũng
không ảnh hưởng tới agent thật sự chạy. Thêm: không multi-select/"Run
Selected" batch; `task.addComment` tồn tại ở Node legacy nhưng **không có
tương đương ở backend-go's `task.proto`** — không có Comments tab.

### C5 — Không có subscription real-time nào trong Task UI
`grep "subscribeRuntimeEvent("` trên `components/task/` = 0. `useTasks.ts:29-55`
chỉ refetch `task.list` khi `projectId` đổi — không polling, không
subscription, không auto-refresh khi có thay đổi từ nơi khác.

## 2. Giải pháp đề xuất

### 2.1 "New Task" dialog + progress refresh

```tsx
// TaskGraph.tsx toolbar — thêm nút bên cạnh Search/filter/toggle
<Button onClick={() => setShowCreateDialog(true)}>+ New Task</Button>
// TaskCreateDialog (mới) → gọi task.create → onSuccess: refetch subtree
```

Sau mỗi lần status con đổi, gọi lại `task.getSubtree` (CR-TG-001) cho task
cha thay vì tự tính progress ở client.

### 2.2 `TaskDAGView` dùng dependency thật + UI thêm/xoá edge

```tsx
// TaskDAGView.tsx — buildDAGLayout() sửa nguồn dữ liệu
const deps = await Promise.all(tasks.map(t => callRuntimeRpc(target, 'task.getDependencies', { taskId: t.id })))
// hoặc, nếu CR-TG-001 thêm batch endpoint theo project — ưu tiên dùng batch, tránh N+1 RPC
```

Thêm drag-to-connect (hoặc đơn giản hơn: nút "+ Add dependency" chọn từ list)
gọi `task.addEdge` (CR-TG-001's atomic `AddEdge`).

### 2.3 **Board/Kanban view (mới)** — third view mode

```tsx
// TaskGraph.tsx — mở rộng viewMode
const [viewMode, setViewMode] = useState<'tree' | 'dag' | 'board'>('tree')
// + button toolbar 'Board' cạnh Tree/DAG

// TaskBoardView.tsx (MỚI)
// Cột theo Status (7 giá trị từ CR-TG-001: backlog/todo/in_progress/blocked/
// review/done/cancelled), kéo-thả đổi status (gọi task.update), mỗi card
// dùng lại <TaskCard> đã có sẵn (không viết lại rendering card).
export function TaskBoardView({ tasks, onStatusChange }: TaskBoardViewProps) {
  const columns = STATUS_ORDER.map(status => ({
    status, tasks: tasks.filter(t => t.status === status)
  }))
  return (
    <div className="flex gap-3 overflow-x-auto">
      {columns.map(col => (
        <BoardColumn key={col.status} status={col.status} tasks={col.tasks} onDrop={onStatusChange} />
      ))}
    </div>
  )
}
```

Dùng token màu/spacing theo [`docs/STYLEGUIDE.md`](../../../STYLEGUIDE.md) —
không tự định nghĩa màu cột mới, map theo `TaskStatusBadge.tsx`'s bảng màu đã
có sẵn cho từng status.

### 2.4 **Grant/Share modal (mới)** + Access tab + permission badge

```tsx
// TaskGrantModal.tsx (MỚI)
// - Danh sách grant hiện có (gọi task.listGrants — CR-TG-003)
// - Form thêm grant: chọn GranteeKind (User/Team/Company) + PermissionLevel
//   (view/comment/edit/execute/manage) + checkbox "Apply to subtree" + optional expiry
// - Nút Revoke từng dòng (task.revokeGrant)
// - Nút "Generate share link" (task.generateShareLink) — hiển thị link + nút Copy

// TaskDetail.tsx — thêm tab thứ 4
<Tabs>
  <Tab label="Details" />
  <Tab label="Subtasks" />
  <Tab label="AI" />
  <Tab label="Access">
    <TaskGrantModal taskId={task.id} />
  </Tab>
</Tabs>

// Header — permission badge, gọi 1 lần khi mount
const perm = await callRuntimeRpc(target, 'task.resolvePermission', { taskId, action: 'manage' })
// ẩn nút Run/Edit/Delete nếu perm < ngưỡng tương ứng — KHÔNG chỉ dựa vào backend reject nữa
```

### 2.5 Sửa Run-Agent UX-lie

Ngắn hạn (không chờ backend đổi `ExecuteRequest`): xoá textarea gây hiểu lầm,
hiển thị `task.promptTemplate` **read-only** với nút "Edit prompt template"
điều hướng tới flow `GenerateAgentPrompt` (CR-TG-002) thay vì 1 ô nhập tạm
thời không có tác dụng. Nếu team quyết định override-prompt-per-run là cần
thiết, phối hợp với CR-TG-005 thêm field `prompt` tuỳ chọn vào
`ExecuteRequest` trước khi khôi phục textarea.

Thêm multi-select trên `TaskTreeView`/`TaskBoardView` + nút "Run Selected"
gọi `task.execute` tuần tự/song song theo batch.

### 2.6 `useTaskActivity` — áp dụng thiết kế có sẵn của CR-FLOW-TASK-005

```tsx
// hooks/useTaskActivity.ts — implement đúng theo CR-FLOW-TASK-005 §2, không thiết kế lại
useEffect(() => {
  const unsub = subscribeRuntimeEvent(target, `task.activity:${taskId}`, (frame) => dispatch({ type: 'activity', frame }))
  return unsub
}, [taskId])
```

Nếu `CR-FLOW-TASK-003`'s kênh chưa sẵn sàng khi CR này triển khai, fallback
polling `task.get` theo interval (mẫu `useWorkflowExecution.ts`'s 4s poll) —
không để Activity Feed trống hoàn toàn.

## 3. Rủi ro / Không thuộc phạm vi

- Không tự thiết kế lại `useTaskActivity`/engine-badge — dùng nguyên thiết kế
  `CR-FLOW-TASK-005` đã có, chỉ implement.
- Comments tab bị block bởi việc `backend-go`'s `task.proto` chưa có
  `AddComment`/`ListComments` — **CR-TG-001 đã bổ sung RPC này**, CR-TG-007
  chỉ cần build UI sau khi CR-TG-001 merge, không tự thêm RPC ở đây.
- Không thuộc phạm vi: dashboard Task Execute (Engine 2) thật thay
  `OrchestrationPage.tsx`'s storyboard — đó là 1 CR follow-up riêng (gated
  bởi CR-TG-004), tránh trộn 2 phạm vi UI khác nhau (F37 Task Graph vs Task
  Execute dispatch dashboard) vào 1 CR.
- Board view's kéo-thả đổi status qua backend permission check
  (`PermissionEdit` tối thiểu) — không tự cho phép đổi UI-only rồi rollback
  im lặng khi backend từ chối; cần optimistic-update + revert-on-error rõ
  ràng.

## Acceptance Criteria

- [ ] "New Task" dialog tạo task thật qua `task.create`, xuất hiện ngay trong
      Tree/Board không cần refresh trang.
- [ ] `TaskDAGView` render đúng edge thật từ `task.getDependencies` (test:
      task A depends_on B → cạnh A→B xuất hiện, không còn luôn rỗng).
- [ ] Có thể thêm/xoá dependency edge trực tiếp từ DAG view.
- [ ] **Board view mới** hiển thị đủ 7 cột status, kéo-thả đổi status gọi
      đúng `task.update`, có optimistic-update + revert khi backend từ chối.
- [ ] **Grant/Share modal mới** liệt kê/thêm/revoke grant, tạo được share
      link, permission badge ẩn đúng nút theo quyền thực tế của user hiện tại.
- [ ] Run-Agent UI không còn ô nhập prompt "giả" (không có tác dụng thật) —
      hoặc override-prompt hoạt động thật nếu backend đã hỗ trợ.
- [ ] `useTaskActivity` nhận được frame test cho ít nhất 1 trong 3 engine
      (giả lập ở integration test).
- [ ] Toàn bộ UI mới tuân theo [`docs/STYLEGUIDE.md`](../../../STYLEGUIDE.md) —
      không dùng màu/spacing tự chế ngoài token đã có.
