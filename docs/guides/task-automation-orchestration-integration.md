# Task, Automation, AI Orchestration — trạng thái thật (backend/agent), và đề xuất liên kết

**Cập nhật:** 2026-08-13 (viết lại toàn bộ sau khi kiểm chứng lại bằng `backend/src/` +
`agent/src/` — xem đính chính ở
[terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md)
và chi tiết đầy đủ ở
[audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md))

> Bản gốc của file này (viết cùng ngày, trước đính chính) kết luận "F37 gần như chưa xây gì" —
> **sai**. Backend Task system hoàn chỉnh, đang chạy thật. Giữ lại nguyên vẹn phần đúng (F14
> Automations đối chiếu doc, và phát hiện quan trọng nhất: pipeline liên kết 3 hệ Task thật sự
> chưa tồn tại) — sửa lại toàn bộ phần sai.

## 1. "Task" là 3 khái niệm khác nhau — xác nhận lại, cả 3 phần backend đều thật

| | Nguồn | Trạng thái |
|---|---|---|
| **Task (Source)** | `TaskPage.tsx` (8278 dòng), `backend/src/shared/task-source-context.ts` | ✅ Thật, đang chạy — work item GitHub/GitLab/Linear/Jira |
| **Task (Plan)** — `OrcaTask` | `backend/src/main/task/*` (5 file, 18 RPC method), `backend/src/shared/task-types.ts` | ✅ **Thật, đang chạy** (đính chính — trước nói sai là "chưa xây") |
| **Task (Execute)** — `TaskRow` | `backend/src/main/runtime/orchestration/*` | ✅ Thật, đang chạy — sub-task tạm thời trong 1 phiên multi-agent |

## 2. Backend Task system — REAL & LIVE, 18 RPC method thật

`backend/src/main/server-bootstrap.ts:487-502` khởi tạo đầy đủ và đăng ký RPC:

- **`TaskDAGValidator.ts`**: `wouldCreateCycle`, `detectCycle`, `getReachable` — SQL thật trên
  `orca_task_edges`.
- **`TaskAIPlanner.ts`**: `decompose()` gọi `relay.call('ai.complete', ...)` qua
  `ProjectServerRouter`, `applyDecomposition()` persist con qua `taskService.create`,
  `generatePromptTemplate()` interpolate `${task.*}`.
- **`TaskGrantService.ts`**: `grantPermission`/`resolvePermission` (đi ngược cây cha, quyền cao
  nhất thắng)/`revokeGrant`/`listGrants` — bảng `orca_task_grants` riêng, **độc lập hoàn toàn
  khỏi `ProjectMember`**.
- **`TaskAgentExecutor.ts`**: `executeTask()` check quyền qua `TaskGrantService`, update status,
  gọi `agentSpawner.spawn(...)` → xuyên tới `agent/` package (xem mục 6).
- **`task-rpc-handler.ts`**: 18 method thật — `task.create/get/update/delete/list/getChildren/
  getAncestors/getSubtree/addEdge/removeEdge/getDependencies/recalculateProgress/addComment/
  grant/resolvePermission/aiDecompose/aiApply/execute`.

## 3. `OrcaTask` — shape thật khác F37 doc, không có field liên kết cross-system

Shape thật (`backend/src/shared/task-types.ts`): `id, projectId?, parentId?, title,
description?, type, status, priority, labels, visibility, reporterId?, assigneeId?,
estimatedHours?, progressPercent, aiContext?, promptTemplate?, dueDate?, createdAt, updatedAt`.

**Khác F37 doc — hoàn toàn không tồn tại**: `dependsOn` (nằm ở bảng riêng `orca_task_edges`),
`subTaskIds` (query theo `parentId`), `ownerId` (dùng `reporterId`/`assigneeId`), `grants`
(bảng `orca_task_grants` riêng), và **quan trọng nhất cho mục 7**: `worktreeId`,
`agentSessionId`, `workflowExecutionId` — **không field nào tồn tại**.

## 4. 🐛 UI (`components/task/*`) — mồ côi, VÀ mọi RPC call đều gọi sai tên method

`App.tsx` render `TaskPage` (GitHub/Linear/Jira, khác hẳn) khi `activeView==='tasks'`. Đường vào
duy nhất của `components/task/*` là qua `WorkspaceLayout.tsx` (mồ côi, xem F38 guide).
`TaskGraphPanel.tsx` **là stub trống thật sự**: `// Task graph panel stub (full implementation
in TASK-V5-07)`.

Bảng RPC call sai tên (dù mount lại vẫn 404 100% nếu không sửa tên):

| File frontend | Gọi | Thật là |
|---|---|---|
| `TaskPromptEditor.tsx` | `task.runAgent` | `task.execute` |
| `TaskDetail.tsx` | `tasks.getDependencies` | `task.getDependencies` |
| `TaskDetail.tsx` | `tasks.runAgent` | `task.execute` |
| `useTask.ts` | `tasks.update` | `task.update` |
| `useTask.ts` | `tasks.delete` | `task.delete` |
| `useTask.ts` | `tasks.aiPlan` | `task.aiDecompose` |
| `useTask.ts` | `tasks.createSubtasks` | `task.aiApply` |

`TaskPromptEditor.tsx` có comment tự nhận biết lệch tên nhưng chưa sửa. **Bonus**: 2 bản
`OrcaTask`/`task-types.ts` không tương thích dùng lẫn trong cùng cụm mồ côi —
`frontend/src/shared/task-types.ts` khớp backend 100%, nhưng
`frontend/src/renderer/src/types/task-types.ts` khác hoàn toàn (4 status value thay vì 7,
`dependsOn` embed, `agentPrompt` thay vì `promptTemplate`, timestamp `number` thay vì `Date`).

## 5. F14 (Automations) — 🐛 phát hiện mới quan trọng: scheduler KHÔNG CHẠY trên backend

File/type `backend/src/main/automations/*` **giống hệt byte-for-byte** bản `frontend/src/main/
automations/` (đã kiểm tra `diff`). `Automation`/`AutomationRun` model đúng như phát hiện trước:
`prompt`, `agentId`, `sourceContext?: TaskSourceContext`, `rrule` — 1 hành động ngầm định,
không phải pipeline YAML nhiều bước như F14 doc mô tả.

**Nhưng — phát hiện mới**: `AutomationService` **không bao giờ được khởi tạo trong
`backend/src`** (`grep "new AutomationService("` → 0 kết quả toàn bộ `backend/src`). Instance
DUY NHẤT trong repo: `desktop/src/main/index.ts:1810` — code Electron desktop, không phải server.

**Hậu quả thật trên server production**:
- `automation.list/show/create/update/delete` **hoạt động** (đi qua persistence layer đã wire).
- `automation.runNow` **luôn throw** `runtime_unavailable`.
- Scheduler thật (`AutomationService.start()`'s `setInterval(evaluateDueRuns, tickMs)`) **không
  bao giờ chạy** — automation cấu hình `rrule` sẽ không bao giờ tự kích hoạt trên server.

→ F14 vẫn là feature khớp doc nhất về mặt TYPE/SHAPE, nhưng **thực thi lại là phần yếu nhất
trong toàn bộ audit này** — không phải do chưa xây, mà do thiếu đúng 1 dòng khởi tạo trong
`server-bootstrap.ts`.

## 6. AI Orchestration — 2 hệ THẬT, tách biệt hoàn toàn (không phải 1 như đoán trước)

### 6.1 `runtime/orchestration/coordinator.ts` — điều phối lead/worker multi-agent

`TaskRow {id, parent_id, task_title, spec, status, deps, result}` — xác nhận lại đúng như trước.
**Đính chính cách kích hoạt**: KHÔNG phải parse terminal output như đoán trước — mà **điều
khiển qua RPC thật** (`orchestration.*`, đăng ký vào default RPC method table). Agent gọi qua
CLI `orca`/`orca-dev orchestration send|ask|check` chạy như lệnh shell bình thường trong PTY.
Điểm duy nhất đọc terminal output là 1 pre-check (`isTerminalRunningAgent()`), không phải cơ
chế điều phối chính.

### 6.2 `WorkflowOrchestrator` — hệ THỨ 3, phát hiện mới, cũng REAL & LIVE

Hoàn toàn tách biệt khỏi `coordinator.ts` — DAG pipeline nhiều bước (`agent/shell/webhook/
notification/condition`), RPC prefix `workflow.*`, lưu trên `IConnectionPool` (DB chung, khác
SQLite riêng của orchestration). Cả 2 hệ đều thật, đều live, không share code.

**UI duy nhất có thể điều khiển `workflow.*`** (`WorkflowMonitor.tsx`) **là stub mồ côi** — 9
dòng, comment "full implementation in TASK-V5-08". `OrchestrationPane.tsx`/
`OrchestrationSkillPromptDialog.tsx` (có render thật trong Settings) **không liên quan** —
chúng chỉ lo cài đặt CLI `orca` binary, trùng tên gây nhầm lẫn, không gọi `workflow.*`/
`orchestration.*` RPC nào cả.

## 7. Cross-system linkage — xác nhận lại: THẬT SỰ CHƯA CÓ (phần đúng duy nhất từ đề xuất trước)

**(a) `OrcaTask` ↔ `TaskSourceContext`**: 0 field chung, 0 file import cả 2 cùng lúc — trùng tên
tiếng Anh thuần tuý.

**(b) `OrcaTask` ↔ orchestration coordinator's `TaskRow`**: 2 type hoàn toàn khác nhau, 2 bảng
SQLite khác nhau (`orca_tasks` vs `tasks` trong DB riêng), 0 cross-reference 2 chiều.

→ Pipeline 3 tầng Source→Plan→Execute đề xuất trước **vẫn cần xây mới hoàn toàn** — đây là phần
đề xuất còn giữ nguyên giá trị, không có gì đã tồn tại sẵn để tận dụng.

## 8. `agent/` package — vai trò thật, theo từng hệ

- **Task execution — CÓ, trực tiếp**: `ProfileAwareAgentSpawner`/`TaskAgentExecutor` gọi
  `relay.call('agent.exec', ...)`; `TaskAIPlanner` gọi `relay.call('ai.complete', ...)`.
- **Orchestration (lead/worker) — CÓ, nhưng chỉ tầng gõ phím**: `coordinator.ts` gọi
  `sendTerminalAgentPrompt()` → khi worker terminal ở dev-server → `relay.call('pty.write', ...)`.
  Toàn bộ state điều phối (`TaskRow`, message, dispatch context) là SQLite backend-side thuần.
- **Automation runs — KHÔNG kết nối trực tiếp**: `precheck-runner.ts` dùng
  `child_process.spawn`/`ssh2` thẳng, không qua `agent/src/relay`. Khớp với mục 5 — automation
  execution chưa wire trong backend.

## 9. Đề xuất — cập nhật lại theo đúng thực tế đã xác nhận

### 9.1 Việc CẦN SỬA NGAY (bug thật)

1. Sửa tên RPC method trong `components/task/*`/`useTask.ts` — 7 chỗ, đổi từ `tasks.*`/tên sai
   sang đúng `task.*` thật (bảng mục 4).
2. Thay `TaskGraphPanel.tsx`'s stub bằng implementation thật, dùng `@shared/task-types` (không
   phải bản `types/task-types.ts` đang lệch).
3. Thêm đúng 1 dòng khởi tạo `AutomationService` vào `backend/src/main/server-bootstrap.ts`
   (theo đúng cách `desktop/src/main/index.ts:1810` đã làm) — mở khoá scheduler thật cho
   production.

### 9.2 Pipeline 3 tầng Source → Plan → Execute — đề xuất giữ nguyên, vẫn cần xây mới

```
Tầng 1 — SOURCE (đã có, đang chạy)          Tầng 2 — PLAN (đã có, đang chạy — ĐÍNH CHÍNH)
  TaskSourceContext                    ①      OrcaTask (backend/src/main/task/*)
  (GitHub/Linear/Jira issue)          ────►    18 RPC method thật, DAG validator thật,
                                                grant system thật (TaskGrantService)
                                                          │
                                                          │  ②  MỚI: "task.execute" đã tồn tại
                                                          │  thật (TaskAgentExecutor) — nhưng nó
                                                          │  KHÔNG seed/ghi ngược vào orchestration
                                                          ▼
                                       Tầng 3 — EXECUTE (đã có, đang chạy)
                                         runtime/orchestration/coordinator.ts's TaskRow
                                                          │
                                                          │  ③  MỚI: kết quả chưa ghi ngược OrcaTask
                                                          ▼
                                                 (quay lại Tầng 2 — CHƯA tồn tại)
```

**Khác biệt so với đề xuất trước**: `task.execute` (tầng 2→3 nối "chạy agent") **đã tồn tại
thật** — nhưng nó gọi thẳng `agentSpawner.spawn()` (như F38's Agent tab), **không** đi qua
`runtime/orchestration/coordinator.ts`. Nghĩa là hiện có **2 con đường "chạy 1 task" độc lập,
không giao nhau**: (a) `task.execute` → 1 agent chạy đơn, không có lead/worker; (b)
`orchestration.run` → nhiều agent phối hợp qua `coordinator.ts`. Việc "①②③" đề xuất trước cần
làm rõ: `OrcaTask` sẽ luôn chạy qua (a), hay có thể chọn chạy qua (b) khi cần phân rã cho nhiều
worker? Đây là 1 quyết định thiết kế thật, xem
[decisions-needed.md](./decisions-needed.md).

### 9.3 `TaskGrantService` đã tồn tại — không cần đề xuất "đừng tự phát minh TaskGrant" nữa

Đề xuất trước ("dùng lại `ProjectMember`, đừng tự xây `TaskGrant`") đã **không được áp dụng
trong quá khứ** — `TaskGrantService` được xây độc lập, đầy đủ, đang chạy thật, có cơ chế kế thừa
theo cây. Hiện có **2 hệ RBAC thật, không chia sẻ code**: `ProjectMember` (project-level) và
`orca_task_grants` (task-level, scope `'user'|'team'|'role'|'everyone'`). Xem đề xuất hợp nhất ở
[user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md) mục 5.3.

### 9.4 Thứ tự triển khai

1. Sửa 3 bug ở mục 9.1 trước — không cần thiết kế mới, chỉ sửa lỗi.
2. Quyết định (a) vs (b) ở mục 9.2 — quyết định thiết kế, không tự triển khai.
3. Nối ①②③ theo hướng đã chốt.
4. UI (Tree/Board/Graph view) **sau cùng** — data flow đã chạy qua RPC/script trước, tránh lặp
   lỗi "tab bấm vào trống" như F38.
