# Audit toàn diện: Profile/RBAC, Project, Task, Orchestration/Automations (backend + agent)

**Ngày:** 2026-08-13

> Đính chính quan trọng dẫn tới audit này: các kết luận trước đó trong `docs/guides/` (viết
> cùng ngày) dựa trên việc tra `frontend/src/main/` — đây **không phải** server thật. Monorepo
> có 4 package tách biệt (`pnpm-workspace.yaml`): `frontend` (SPA renderer, build
> `frontend/out/web`), `backend` (server thật, build `backend/out/server`, chạy trong container
> `orca-server` production), `agent` (chạy trên chính dev-server), `desktop` (Electron app thật,
> tách biệt khỏi `frontend`). `frontend/src/main/*` là code Electron-main-process **vestigial**
> nằm trong package `frontend`, không thuộc runtime deploy nào.
>
> Audit này dùng 4 subagent đọc trực tiếp `backend/src/` + `agent/src/`, đối chiếu với
> `frontend/src/renderer/` (UI thật) để xác định chính xác: cái gì thật/đang chạy, cái gì mồ
> côi, và bug cụ thể ở đâu. Toàn bộ trích dẫn có file:line, không suy đoán.
>
> Giải pháp cho từng bug/case cụ thể tách sang
> [fix-proposals-per-issue.md](./fix-proposals-per-issue.md). Các điểm cần con người quyết định
> tách sang [decisions-needed.md](./decisions-needed.md). Kế hoạch thực thi tổng hợp xem
> [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md).

## Nguyên tắc kiến trúc mục tiêu (định hướng giải pháp)

Theo yêu cầu: **frontend = hiển thị + tương tác người dùng**, **backend = quản trị + lưu dữ liệu
quản trị**, **agent = thực thi tác vụ + quản trị code trên dev-server**. Mọi giải pháp trong các
file kèm theo audit này bám theo nguyên tắc này — xem
[fix-proposals-per-issue.md](./fix-proposals-per-issue.md).

---

## A. Profile / Department / Team / RBAC

### A1. Backend (`backend/src/main/profile/`) — REAL & LIVE

- **`ProfileService.ts`** (200 dòng): CRUD trên `orca_companies`/`orca_departments`/
  `orca_user_profiles` (migration `0006_company_dept.ts`) + cột `department_id` trên
  `orca_users`. Method: `createCompany`(:22), `getCompanyProfile`/`setCompanyProfile`(:40,:52),
  `createDepartment`(:70), `getDeptProfile`/`setDeptProfile`(:89,:101),
  `getUserProfile`/`setUserProfile`(:119,:134), `getCompanyProfileForUser`/
  `getDeptProfileForUser`(:154,:174, JOIN thật), `setUserDepartment`(:192). Profile lưu dạng
  JSON blob — **không có schema validation khi ghi** (RPC param schema là
  `z.record(z.string(), z.unknown())`).
- **`ProfileResolver.ts`** (321 dòng): merge 3 lớp Company→Dept→User, cache TTL 60s(:29).
  `security` bị company-lock hoàn toàn(:102-110); `agent`/`editor` merge theo field(:170-202);
  `shell` merge phức hợp(:210-256); `mcp.servers` dedupe theo tên, user thắng(:294-319).
- **`profile-rpc-handler.ts`**: `createProfileMethods()` đăng ký `profile.getResolved`(:97),
  `profile.getUserProfile`(:109), `profile.updateUser`(:122, chặn payload `security` với
  `PROFILE_FIELD_LOCKED`), `profile.getCompany`(:155), `profile.updateCompany`(:166),
  `profile.updateDept`(:196), `profile.invalidate`(:226), `profile.setUserDept`(:244),
  `profile.createCompany`(:258), `profile.createDept`(:271). Method sửa company/dept đều gọi
  `requireAdmin()`(:290-297) — kiểm tra role thật.
- **Wiring xác nhận**: `server-bootstrap.ts:407-419` khởi tạo `ProfileService`/`ProfileResolver`,
  gọi `rpcServer.addMethods(createProfileMethods(...))` với `getUserRole` closure thật(:415).

### A2. `Team` — KHÔNG phải entity thật

- Không có bảng `orca_teams` ở bất kỳ migration nào.
- Có bảng `orca_team_members` (từ `0010_tasks.ts:116-129`): `(team_id, user_id, role, added_at)`
  — nhưng `team_id` là chuỗi trơn, không có bảng metadata (tên/mô tả) đứng sau.
  - Người dùng duy nhất: `TaskGrantService.ts:16,224-230` — SELECT read-only cho grant scope
    `'team'`.
  - **Không có `INSERT INTO orca_team_members` ở bất kỳ đâu trong `backend/src`** (grep xác
    nhận 0 kết quả) — không có RPC `team.*` để tạo team/thêm thành viên.
  - **Nhánh grant `'team'` trong `TaskGrantService` là dead code** — bảng luôn rỗng, không bao
    giờ match được user nào.

### A3. 5 bản type Profile/OrcaProfile khác nhau

| # | File | Trạng thái |
|---|---|---|
| 1 | `backend/src/main/profile/OrcaProfile.ts` | Chuẩn/thật |
| 2 | `frontend/src/main/profile/OrcaProfile.ts` | Bản sao y hệt #1, **chết** (0 importer, vestigial Electron-main tree) |
| 3 | `agent/src/main/profile/OrcaProfile.ts` | Bản sao y hệt #1, **chết** (0 importer trong `agent/src`) |
| 4 | `frontend/src/shared/profile-types.ts` | Bản tách biệt, **0 importer ở bất kỳ đâu** |
| 5 | `frontend/src/renderer/src/types/profile-types.ts` | **UI thật dùng bản này** — lệch backend nhiều nhất |

Bảng lệch field chi tiết #5 vs #1: xem
[user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md) mục 4.

### A4. Bug thật — RPC method sai tên (production, chưa ai phát hiện)

- `useProfile.ts:30` gọi `profile.getUser` — **không tồn tại** trên backend (đúng là
  `profile.getUserProfile`). Bị che giấu vì `useProfile.test.ts` mock RPC layer, tự chế response
  cho tên method sai đó — test không bao giờ chạm backend thật.
- `CompanyProfileAdmin.tsx:15`, `DeptProfileAdmin.tsx:17` gọi `profile.listDepts` — **không tồn
  tại ở bất kỳ đâu trong `backend/src`**.

### A5. Bug thật — trang Admin build ra nhưng không serve được

`frontend/vite.config.ts:54-59` build thật entry `admin-index.html` → tồn tại thật trong
`frontend/out/web/admin-index.html`. Nhưng `backend/src/server/http-server.ts:165-168`:
```typescript
if (path.startsWith('/auth') || path.startsWith('/admin')) {
  app(req, res)   // route vào Express TRƯỚC static-file fallback ở :171
  return
}
```
Express chỉ mount `/admin/api/*` (`admin-router.ts:29-49`, đúng, có `requireAdmin`) — không có
route nào serve `/admin`/`/admin-index.html`. Mọi request rơi vào 404 mặc định. **Không có
link/route nào trong app chính dẫn tới `/admin` nữa.** Tầng REST API thật và hoạt động — chỉ
HTML shell không serve được.

Bonus: `DeptProfileAdmin.tsx` không được `AdminApp.tsx` import vào router (chỉ có
`CompanyProfileAdmin` cho `/profile`) — mồ côi ngay trong chính app Admin.

### A6. `agent/` — không tham gia RBAC/profile

Grep `agent/src/relay/*` cho `profile.`/`rbac`/`requireAdmin`/`orca_departments` → 0 kết quả.
Access control cho agent hoàn toàn nằm phía `backend/` trước khi dispatch việc xuống agent.

---

## B. Project / OrcaProject

### B1. Backend — REAL & LIVE, có FK thật từ hệ Task

`ProjectService.ts:70-331` — CRUD đầy đủ: `create`(:82), `get`(:139), `list(userId)`(:163, JOIN
`orca_v5_project_members`), `update`(:192), `delete`(:240), `addMember`(:252), `removeMember`
(:267), `updateMemberRole`(:277), `getMembers`(:287), `getMember`(:303), `assertAccess`(:324,
throw `PROJECT_ACCESS_DENIED`). Bảng: `orca_v5_projects`/`orca_v5_project_members` (migration
`0007_projects.ts:16-63`, v7, đặt tên `orca_v5_*` để tránh đụng bảng `orca_projects` cũ từ
migration 0004 — không liên quan). `0010_tasks.ts:25` thêm `project_id` FK từ `orca_tasks` sang
`orca_v5_projects` — **hệ Task xây thật trên nền OrcaProject**.

`ProjectServerRouter.ts:18-84`: `getRelayForProject()`(:33), `getProjectContext()`(:64),
`getProject()`(:81). Blast radius CodeGraph xác nhận caller thật: `ProfileAwareAgentSpawner.ts`,
`TaskAIPlanner.ts`, `StepExecutors.ts`, `WorkflowOrchestrator.ts`, `WorkspaceService.ts`.

`ProfileAwareAgentSpawner.ts:53-156`: `spawn()` resolve project context, merge profile env,
inject `ORCA_PROJECT_ID`/`ORCA_USER_ID`/`ORCA_REPO_PATH`/`ORCA_DEV_SERVER_ID`/
`ORCA_AI_PROVIDER_ID`/`ORCA_AI_MODEL_ID`(:101-117), gọi `relay.call('agent.exec', ...)`(:130).

RPC đăng ký thật (`server-bootstrap.ts:422-432`): `project.list`, `project.get`,
`project.create`, `project.update`, `project.delete`, `project.addMember`, `project.removeMember`,
`project.updateMemberRole`, `project.getMembers`, `project.agentSpawn`.

### B2. 🐛 Bug thật — `project.agentSpawn` luôn lỗi

`server-bootstrap.ts:431` gọi `createProjectMethods(projectService, getUserRole)` — chỉ 2 tham
số, thiếu tham số thứ 3 `agentSpawner` (optional, `project-rpc-handler.ts:87-91`).
`ProfileAwareAgentSpawner` mãi tới `:499` mới được khởi tạo, chỉ wire vào `TaskAgentExecutor`
(:500), không re-register vào project methods. `project-rpc-handler.ts:232`:
`if (!agentSpawner) throw new Error('AGENT_SPAWNER_NOT_AVAILABLE')` — **mọi lời gọi
`project.agentSpawn` luôn throw lỗi này trên server đã deploy.**

### B3. `backend/src/shared/project-types.ts` vs frontend copy

Gần như giống hệt `frontend/src/shared/project-types.ts` — chỉ khác: backend's
`UpdateProjectParams`(:65-73) có thêm `devServerId?: string` (project rebinding, dùng thật ở
`ProjectService.update():197-220`, có TODO thiếu guard `PROJECT_HAS_ACTIVE_WORKFLOWS`) mà bản
frontend không có — frontend/shared bị lỗi thời hơn backend 1 field.

### B4. `OrcaProject` ↔ `Repo`/`Worktree` — xác nhận lại: KHÔNG có quan hệ

`orca_v5_projects`(:23-34) chỉ có `dev_server_id`/`repo_path` (chuỗi thô) — không FK tới
`Repo`/`Worktree` nào. FK thật duy nhất vào bảng này đến từ hệ Task (`0010_tasks.ts:25`).
`agent/src/shared/types.ts:108-121` có `Project` type RIÊNG (repo-grouping UI concept, dùng bởi
`agent/src/main/persistence.ts`) — cấu trúc và ngữ nghĩa khác hẳn `OrcaProject`, xác nhận không
có cầu nối kể cả ở tầng agent.

### B5. 🐛 Bug thật — `WorkspaceContext.switchProject()` gọi 2/4 RPC sai

| Call | Backend thật | Verdict |
|---|---|---|
| `project.get({projectId})` | Khớp, nhưng return-type lệch: backend trả `visibility:'company'`/`createdAt: Date`; type renderer khai `'public'`/`number` | Chạy nhưng dữ liệu sai kiểu |
| `git.status({projectId})` | Schema thật yêu cầu `worktree: string` (`git-params.ts:3-13`), không phải `projectId` — luôn fail validation | **Luôn lỗi**, bị `.catch(() => null)` nuốt |
| `workspace.listFiles({projectId, dirPath})` | **Không tồn tại** — `workspace.*` thật chỉ có `init/teardown/refreshFileTree/refreshGitStatus` | **Luôn "method not found"**, bị nuốt |
| `profile.getResolved({})` | Khớp | OK |

### B6. Cụm UI orphaned — xác nhận lại, mở rộng hơn trước

Re-grep toàn bộ `frontend/src/renderer`: `WorkspaceLayout`/`ProjectSwitcher`/`ProjectSettings`
chỉ có self-reference + file test riêng, `App.tsx` không tham chiếu bất kỳ cái nào. **Mở rộng
mới**: `WorkspaceContext`'s provider **có mount thật** (`main.tsx`, `web/main-web-bootstrap.tsx`),
và `useWorkspace()` được dùng thật bởi cả 1 cụm: `GitPanel.tsx`, `FileViewer.tsx`,
`FileContextMenu.tsx`, `BranchManager.tsx`, `GitHistory.tsx`, `DiffViewer.tsx`,
`FileSearchPanel.tsx`, `PullRequestList.tsx`, `PullRequestForm.tsx`, `TaskPromptEditor.tsx`,
`TaskDetail.tsx`, `CodeReviewPanel`, `commit-message-generator.tsx`, `annotation-panel.tsx` —
**không component nào trong 14 cái này reachable từ `App.tsx`**. Provider sống, toàn bộ cây
consumer chết.

Có `WorkspaceContextV6.tsx` + `WorkspaceContextBridge.ts` cạnh tranh, gate bằng
`__ORCA_WORKSPACE_V6__`, nhưng `main.tsx` import thẳng bản V5, bỏ qua bridge — V6 tự nó cũng
chết.

### B7. `agent/` — chỉ pass-through thụ động

`agent/src/relay/agent-spawner.ts:248,274,284-285` — nhận `projectId` như data thô, echo vào
env `ORCA_PROJECT_ID`. Không có RPC handler nào validate `projectId`, không có gate truy cập
theo project. Toàn bộ access control nằm ở `ProjectService.assertAccess()` phía backend, trước
khi relay call được gửi đi.

### B8. Auth `UNAUTHENTICATED` — bug khác hẳn "chưa mount UI"

Mọi method trong `project-rpc-handler.ts` bắt đầu bằng
`const userId = ctx.userId; if (!userId) throw new Error('UNAUTHENTICATED')`. `ctx.userId`
(`runtime/rpc/core.ts:84-87`) inject qua session token từ WebSocket transport
(`dispatcher.ts:115,143-145,177`). Log production khớp chính xác. **Đây là lỗi transport không
forward `userId` cho phiên web** — độc lập với bug B6 (UI mồ côi); dù B6 được sửa, các RPC vẫn
lỗi cho tới khi gap này được vá.

---

## C. Task / OrcaTask

### C1. Backend — REAL & LIVE, 18 RPC method thật

`server-bootstrap.ts:487-502`: khởi tạo `TaskDAGValidator`(:495), `TaskService`(:496),
`TaskGrantService`(:497), `TaskAIPlanner`(:498), `ProfileAwareAgentSpawner`(:499),
`TaskAgentExecutor`(:500), đăng ký `createTaskMethods(...)`(:501).

- **`TaskDAGValidator.ts:17-129`**: `wouldCreateCycle` (DFS per-edge-type), `detectCycle` (BFS
  toàn bộ edge type), `getReachable` (BFS impact set) — SQL thật trên `orca_task_edges`.
- **`TaskAIPlanner.ts:30-220`**: `decompose()` gọi `relay.call('ai.complete', ...)` qua
  `ProjectServerRouter`, parse JSON; `applyDecomposition()` persist con qua `taskService.create`;
  `generatePromptTemplate()` interpolate `${task.*}`.
- **`TaskGrantService.ts:58-250`**: `grantPermission`/`resolvePermission` (đi ngược cây cha,
  quyền cao nhất thắng)/`revokeGrant`/`listGrants` — bảng `orca_task_grants` thật.
- **`TaskAgentExecutor.ts:32-160`**: `executeTask()` check quyền qua
  `grantService.resolvePermission`, update status `in_progress`→`review`/`blocked`, gọi
  `agentSpawner.spawn(...)`.
- **`task-rpc-handler.ts`**: 18 method thật — `task.create/get/update/delete/list/getChildren/
  getAncestors/getSubtree/addEdge/removeEdge/getDependencies/recalculateProgress/addComment/
  grant/resolvePermission/aiDecompose/aiApply/execute`.

### C2. `OrcaTask` — shape thật khác F37 doc, và không có field liên kết cross-system

Shape thật (`backend/src/shared/task-types.ts:39-62`): `id, projectId?, parentId?, title,
description?, type, status, priority, labels, visibility, reporterId?, assigneeId?,
estimatedHours?, progressPercent, aiContext?, promptTemplate?, dueDate?, createdAt, updatedAt`.

**Khác doc F37 — hoàn toàn không có**: `dependsOn` (nằm ở bảng riêng `orca_task_edges`, không
embed), `subTaskIds` (query theo `parentId`, không lưu mảng), `ownerId` (dùng
`reporterId`/`assigneeId`), `grants` (bảng riêng `orca_task_grants`), và **quan trọng nhất**:
`worktreeId`, `agentSessionId`, `workflowExecutionId` — **không field nào trong 3 cái này tồn
tại trên `OrcaTask`.**

### C3. `TaskGrantService` — hệ grant độc lập, KHÔNG dùng `ProjectMember`

Grep xác nhận 0 tham chiếu tới `ProjectMember`/project RBAC. Model riêng hoàn chỉnh: bảng
`orca_task_grants` (id, task_id, scope, scope_id, permission, apply_tree, granted_by,
expires_at, `0010_tasks.ts:79`), `scope` là `'user'|'team'|'role'|'everyone'`, độc lập hoàn
toàn khỏi bảng project-membership. `resolvePermission` đi ngược `parentId` (qua
`getAncestorIds`), gom grant có `applyTree=1`, lấy quyền cao nhất.

### C4. UI (`components/task/*`) — orphaned, VÀ mọi RPC call đều sai tên

Reachability: `App.tsx` render `TaskPage` (GitHub/Linear/Jira, KHÁC hẳn) khi `activeView===
'tasks'`. `WorkspaceLayout.tsx` lazy-load `TaskGraphPanel` nhưng bản thân `WorkspaceLayout`
không được import ở đâu ngoài test riêng. `TaskGraphPanel.tsx:1-12` **là stub trống thật sự**:
`// Task graph panel stub (full implementation in TASK-V5-07)`.

Bảng RPC call sai tên (tất cả sẽ 404 nếu mount lại mà không sửa):

| File frontend | Gọi | Thật là |
|---|---|---|
| `TaskPromptEditor.tsx:23` | `task.runAgent` | `task.execute` |
| `TaskDetail.tsx:32` | `tasks.getDependencies` | `task.getDependencies` |
| `TaskDetail.tsx:45` | `tasks.runAgent` | `task.execute` |
| `useTask.ts:11` | `tasks.update` | `task.update` |
| `useTask.ts:17` | `tasks.delete` | `task.delete` |
| `useTask.ts:32` | `tasks.aiPlan` | `task.aiDecompose` |
| `useTask.ts:46` | `tasks.createSubtasks` | `task.aiApply` |

`TaskPromptEditor.tsx` có comment tự nhận biết lệch tên ("task.runAgent (số ít) khác với
tasks.runAgent mà TaskDetail dùng") nhưng chưa sửa.

**Bonus**: 2 bản `OrcaTask`/`task-types.ts` không tương thích dùng lẫn trong cùng cụm mồ côi —
`frontend/src/shared/task-types.ts` khớp backend 100% (diff xác nhận), nhưng
`frontend/src/renderer/src/types/task-types.ts` là bản khác hoàn toàn (4 status value thay vì
7, `dependsOn` embed, `agentPrompt` thay vì `promptTemplate`, `progress` thay vì
`progressPercent`, timestamp `number` thay vì `Date`). `TaskGraph.tsx`/`TaskCard.tsx`/
`TaskStatusBadge.tsx`/`TaskTreeView.tsx` import bản đúng; `TaskDetail.tsx`/`TaskPromptEditor.tsx`/
`TaskAIDecompose.tsx`/`TaskDAGView.tsx` import bản sai — 2 nhóm component cùng cụm không tương
thích type với nhau.

### C5. Cross-system linkage — xác nhận THẬT SỰ CHƯA CÓ

`(a) OrcaTask ↔ TaskSourceContext`: `TaskSourceContext` (`backend/src/shared/
task-source-context.ts:46-55`) là context định tuyến host cho Automations
(provider/projectId/hostId/repoId) — 0 field chung với `OrcaTask`, không file nào import cả 2
cùng lúc. Trùng tên tiếng Anh "task" thuần tuý, 2 hệ không liên quan.

`(b) OrcaTask ↔ orchestration coordinator's TaskRow`: 2 `TaskRow` hoàn toàn khác nhau — 1 là
row-mapper nội bộ của `TaskService` cho bảng `orca_tasks`; 1 là type riêng của
`runtime/orchestration/types.ts:38` với `TaskStatus` khác hẳn (`pending/ready/dispatched/
completed/failed/blocked`), đọc bảng SQLite riêng (`tasks`, không phải `orca_tasks`). Grep xác
nhận: `orchestration/*.ts` không import `TaskService`; `task/*.ts` không import
`runtime/orchestration/`.

→ Pipeline 3 tầng Source→Plan→Execute đề xuất trước **thật sự cần xây mới hoàn toàn**, không có
phần nào đã tồn tại sẵn.

### C6. `agent/` — tham gia thật vào Task execution

`ProfileAwareAgentSpawner.ts:130` và `TaskAgentExecutor.ts:107` gọi `relay.call('agent.exec',
...)`; `TaskAIPlanner.ts:62` gọi `relay.call('ai.complete', ...)` — cả 2 landing tại
`agent/src/relay/agent-rpc-dispatch.ts`.

---

## D. Orchestration / Automations / `agent/`

### D1. `runtime/orchestration/coordinator.ts` — REAL & LIVE, điều khiển qua RPC (không phải parse terminal)

`types.ts:38-50`: `TaskRow {id, parent_id, created_by_terminal_handle, task_title, display_name,
spec, status, deps, result, created_at, completed_at}`. `MessageType` (`:1-9`):
`status/dispatch/worker_done/merge_ready/escalation/handoff/decision_gate/heartbeat`. `db.ts:
77-171`: `OrchestrationDb` — SQLite **riêng biệt** (better-sqlite3), bảng
`messages/tasks/dispatch_contexts/decision_gates/coordinator_runs`. `coordinator.ts:94-586`:
poll loop `tick()` dispatch task sẵn sàng cho terminal rảnh.

**Cách kích hoạt**: qua RPC (`orchestration.*`, `rpc/methods/orchestration.ts:205-698` +
`orchestration-gates.ts:38-154`), đăng ký vào default method table
(`rpc/methods/index.ts:11,55`). `orchestration-gates.ts:55-78` là nơi `new Coordinator(...)`
thật sự được tạo, kích hoạt bởi RPC `orchestration.run`. Agent gọi các method này qua CLI
`orca`/`orca-dev orchestration send|ask|check` chạy như lệnh shell bình thường trong PTY —
**không phải scrape terminal output**. Điểm duy nhất có đọc terminal output là
`runtime.isTerminalRunningAgent()` (`orchestration.ts:517`) — 1 pre-check trước khi dispatch,
không phải cơ chế điều phối chính.

### D2. `WorkflowOrchestrator` — hệ THỨ 3, tách biệt hoàn toàn, cũng REAL & LIVE

| | `WorkflowOrchestrator` | `runtime/orchestration/coordinator.ts` |
|---|---|---|
| Khái niệm | DAG pipeline nhiều bước | Điều phối lead/worker multi-agent theo terminal |
| RPC prefix | `workflow.*` | `orchestration.*` |
| Step type | `agent/shell/webhook/notification/condition` | Dispatch cả task cho phiên terminal |
| Lưu trữ | `IConnectionPool` (DB chung app) | SQLite riêng (`orchestration.db`) |
| Wiring | `server-bootstrap.ts:466-485` | Nằm trong default RPC method table, sống ngay khi RPC server start |

Cả 2 đều thật, đều live trên production, không share code. `WorkflowOrchestrator.ts:19-27` có
comment về 1 bug lịch sử ("chưa từng gọi `StepExecutors.execute()` thật") — xác nhận đây là code
đang được bảo trì tích cực, không phải bỏ hoang.

### D3. 🐛 Bug thật lớn — `AutomationService` KHÔNG BAO GIỜ được khởi tạo trong `backend/src`

File list/type (`backend/src/main/automations/*`, `backend/src/shared/automations-types.ts`)
**giống hệt byte-for-byte** bản `frontend/src/main/automations/` — `Automation`/`AutomationRun`
model đúng như trước: `prompt`, `agentId`, `sourceContext?: TaskSourceContext`, `rrule` — 1
action ngầm định, không phải pipeline YAML nhiều bước.

**Nhưng**: `grep -rn "new AutomationService(" backend/src` → **0 kết quả**.
`grep -rn "setAutomationService(" backend/src` → chỉ có định nghĩa setter
(`orca-runtime.ts:589`), **0 nơi gọi nó**. Instantiation DUY NHẤT trong cả repo:
`desktop/src/main/index.ts:1810` — code Electron desktop, không phải backend server.

**Hậu quả thật trên server production**:
- `automation.list/show/create/update/delete` **hoạt động** — đi thẳng qua
  `orca-runtime-automation.ts:49-190` tới store/persistence (đã wire).
- `automation.runNow` (`orca-runtime-automation.ts:193-199`) **luôn throw** `runtime_unavailable`
  vì `this.host.getAutomationService()` (`orca-runtime.ts:557`) trả về `null`.
- Scheduler thật (`AutomationService.start()`'s `setInterval(evaluateDueRuns, tickMs)`,
  `service.ts:73-80`) — **không bao giờ chạy trên backend**. **Automation cấu hình `rrule` sẽ
  không bao giờ tự kích hoạt trên server production.**

### D4. `agent/` — vai trò thật theo từng hệ

- **Task execution — CÓ, trực tiếp**: `ProfileAwareAgentSpawner`/`TaskAgentExecutor` gọi
  `relay.call('agent.exec', ...)`; `TaskAIPlanner` gọi `relay.call('ai.complete', ...)` — cả 2
  landing ở `agent-rpc-dispatch.ts`.
- **Orchestration (lead/worker) — CÓ, nhưng gián tiếp**: `coordinator.ts:506` gọi
  `runtime.sendTerminalAgentPrompt()` → ... → khi worker's terminal ở dev-server,
  `dev-server-pty-provider.ts:120-220` gọi `relay.call('pty.write', ...)` — chỉ tầng "gõ chữ
  vào terminal" chạm `agent/`; toàn bộ state điều phối (`TaskRow`, message, dispatch context) là
  SQLite backend-side thuần, không có `agent/` tham gia.
- **Project-scoped operations — CÓ, trực tiếp**: `dev-server-filesystem-provider.ts`/
  `dev-server-git-provider.ts` gọi `fs.*`/`git.*` RPC.
- **Automation runs — KHÔNG kết nối trực tiếp**: `precheck-runner.ts` dùng
  `child_process.spawn`/`ssh2` thẳng, không qua `agent/src/relay`. `headless-dispatch.ts`'s
  `HeadlessAutomationDispatcher` **không có implementation nào wire trong `backend/`** (khớp D3).

**Capability thật của `agent/`** (`agent-rpc-dispatch.ts:267-1012`): PTY execution
(`pty.create/attach/write/resize/destroy/...`, qua `pty-daemon-server.ts` riêng để terminal
sống sót khi agent restart), spawn agent CLI tương tác (`agent.spawn/kill/sendInput`, node-pty
cho `claude/codex/gemini/opencode/ollama`), `agent.exec` (chạy 1 lần, non-interactive), Git ops
(`git.exec/worktree.*/history/...`), File ops (`fs.readDir/readFile/writeFile/grep/glob/...`),
`ai.complete`, credentials, GitHub/GitLab hosted-repo API, MCP tool registry, và 1 hệ cron
Hermes/OpenClaw riêng (`externalAutomations.*`) không liên quan `automations/` của backend.

### D5. UI orchestration/workflow — 1 mồ côi, 2 cái tên gây nhầm lẫn

| Component | Verdict |
|---|---|
| `WorkflowMonitor.tsx` (`components/workflow/`) | **Mồ côi** — file stub 9 dòng ("full implementation in TASK-V5-08"), 0 RPC call. Host duy nhất (`WorkspaceLayout.tsx`) cũng mồ côi. |
| `OrchestrationPane.tsx` (`components/settings/`) | **Thật, có render** (`App.tsx→Settings.tsx→OrchestrationPane.tsx`) — nhưng chỉ để cài đặt CLI `orca` binary (gọi IPC `cli.getInstallStatus`), **không gọi `workflow.*`/`orchestration.*` RPC nào cả**. Trùng tên gây nhầm lẫn với `WorkflowOrchestrator`/`coordinator.ts`, không liên quan. |
| `OrchestrationSkillPromptDialog.tsx` | **Thật, có render** — cùng lý do, cùng lưu ý trùng tên. |

**Kết luận**: hệ duy nhất có khả năng điều khiển/theo dõi `workflow.*` RPC thật
(`WorkflowMonitor`) là code chết. `WorkflowOrchestrator` thật và live trên backend nhưng hiện
**không có UI consumer nào sống**.
