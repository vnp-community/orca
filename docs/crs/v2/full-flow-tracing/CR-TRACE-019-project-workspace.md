# CR-TRACE-019 — Project Workspace Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-019 |
| **Tên** | Project Workspace — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P3 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/project-workspace.md`, `src/main/workspace/WorkspaceService.ts`, `src/main/workspace/workspace-rpc-handler.ts`, `src/main/project/ProjectServerRouter.ts`, `src/main/dev-server/relay-connection-pool.ts`, `src/main/ssh/fleet-health-monitor.ts`, `src/main/profile/ProfileResolver.ts`, `src/main/runtime/rpc/methods/git-remote.ts`, `src/shared/trace/tracers.ts` |

---

## 1. Vấn đề

Bốn sub-flow BL-PW-01→04 hiện **không có tracer nào**. Đây là flow "vào cửa" mỗi khi user mở 1 project — chậm ở đây ảnh hưởng trực tiếp tới trải nghiệm mở workspace. Các điểm mù cụ thể đã xác nhận qua code:

1. **BL-PW-01 (workspace init)**: `WorkspaceService.initWorkspace()` (`WorkspaceService.ts:82`) chạy `Promise.all` với 3 lời gọi `relay.call()` song song (`git.exec status`, `git.worktree.list`, `fs.readDir`) cộng 1 query DB (`taskService.list`) — mỗi promise có `.catch(() => null)` riêng (offline-tolerant theo comment dòng 80-81), nghĩa là **lỗi bị nuốt âm thầm**. Nếu workspace mở lên thiếu git status hoặc file tree, hiện không có cách biết relay call nào đã fail và vì sao.
2. **BL-PW-02 (file explorer)**: `WorkspaceService.refreshFileTree()` (dòng ~143) gọi `relay.call('fs.readDir', ...)`, cũng `.catch(() => null)` — cùng vấn đề im lặng khi mở rộng thư mục lỗi.
3. **BL-PW-03 (git UI ops)**: `src/main/runtime/rpc/methods/git-remote.ts` expose các RPC method `git.add`, `git.commit`, `git.push`, `git.generateCommitMessage`, `git.pr.create` — toàn bộ đều `relay.call('git.exec', ...)` hoặc `relay.call('ai.complete', ...)` (dòng 383, cho generate commit message) tới Dev Server. Đây là chuỗi thao tác người dùng thực hiện tuần tự (stage → commit → push → PR) — khi 1 bước fail giữa chừng (vd `git.push` bị reject do remote ahead), không có timeline hợp nhất giữa các bước.
4. **BL-PW-04 (workspace integration)**: mô tả `TaskWorkspaceIntegrator.open()` trong flow doc — **grep xác nhận class/hàm này chưa tồn tại trong code** (`openTaskContext` cũng không tìm thấy). Sub-flow này dường như chưa triển khai; CR chỉ định nghĩa tracer placeholder.

## 2. Thành phần & Transport liên quan

| Thành phần (flow doc) | Thực tế trong code | Layer | Transport | CR-TRACE-000 §3.3 row |
|---|---|---|---|---|
| Browser (Explorer/Git/Agent/Tasks panel) | — | UI | WebSocket RPC (`workspace.*`, `git.*` namespace) | Row 1 |
| WorkspaceContextManager | `WorkspaceService.ts:67` (class `WorkspaceService`) — không có class riêng tên `WorkspaceContextManager`; `workspace-rpc-handler.ts` expose `workspace.init` (dòng 23), `workspace.teardown` (36), `workspace.refreshFileTree` (49), `workspace.refreshGitStatus` (63) | Backend | WebSocket RPC | Row 1 |
| ProjectService | `ProjectService.ts` (dùng bởi `ProjectServerRouter`) | Business Logic | in-process | n/a |
| RelayConnectionPool | `relay-connection-pool.ts:25` — `getOrConnect()` dòng 39 | Business Logic | thiết lập kết nối relay (bản thân việc connect là SSH/Agent WS handshake) | Row 2 (khi call), việc `getOrConnect` là tiền đề |
| FleetHealthMonitor | `fleet-health-monitor.ts:18` — `runHealthCheckCycle`-style private polling (dòng 51+), ghi snapshot vào `fleetHealthStore` (import dòng 6, singleton `fleetHealthStore`) | Business Logic | polling nội bộ SSH connection state, không phải request-per-workspace-switch | n/a — cache đọc in-process khi `WorkspaceService` cần |
| ProfileResolver | `ProfileResolver.ts:35` — `resolve()` dòng 44 | Business Logic | in-process (có thể cache/DB) | n/a |
| Dev Server (relay) — git/fs ops | `git-remote.ts` (RPC `git.status`, `git.add`, `git.commit`, `git.push`, `git.pr.create`, ...) đều gọi `relay.call('git.exec', ...)`, đi qua `ProjectServerRouter.getRelayForProject()` | Remote | `relay.call()` | Row 2 |
| Server Database | `orca_projects`, `orca_worktrees`, task/workflow tables | Persistence | in-process | n/a |

> **Ghi chú khác biệt flow doc vs code:** flow doc mô tả BL-PW-02 dùng RPC riêng `fs.readDir`/`fs.readFile`/`fs.search` như method độc lập. Code hiện tại chỉ xác nhận `workspace.refreshFileTree` (wraps `relay.call('fs.readDir', ...)` trong `WorkspaceService.ts`). Không tìm thấy RPC method `fs.readFile` hay `fs.search` cho luồng project-workspace qua relay (có `files.readDir`/`files.search` trong `src/main/runtime/rpc/methods/files.ts`, nhưng đó dùng `runtime.readFileExplorerDir`/`searchRuntimeFiles` — local Orca runtime environment, KHÔNG phải Dev Server relay). Mở/search file qua Dev Server relay trong Project Workspace: **chưa xác định file cụ thể — cần điều tra thêm khi triển khai.**

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  projectWorkspaceInitFlow:    createTracer('projectWorkspace:init'),          // BL-PW-01
  projectWorkspaceExplorerFlow: createTracer('projectWorkspace:explorerBrowse'), // BL-PW-02
  projectWorkspaceGitOpFlow:   createTracer('projectWorkspace:gitOp'),         // BL-PW-03 (1 span / RPC git op)
  projectWorkspaceAgentAttachFlow: createTracer('projectWorkspace:agentAttach'), // BL-PW-04 (placeholder)
}
```

## 4. Instrumentation theo từng sub-flow

### BL-PW-01 — Project Workspace Context

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `workspace.init` | `start` | `projectId`, `userId` | `workspace-rpc-handler.ts:23` |
| Lấy relay cho project (có thể connect mới) | `step('relay-resolve')` | `projectId`, `relayReused: boolean` (nếu `getOrConnect` expose được thông tin pool hit) | `ProjectServerRouter.ts:29` (`getRelayForProject`), `relay-connection-pool.ts:39` |
| 3 relay call + 1 DB query song song — MỖI relay call fail âm thầm hiện tại, cần log riêng vì `.catch(() => null)` | `step('parallel-fetch')` với 4 field con | `gitStatusOk`, `worktreesOk`, `fileTreeOk`, `pendingTasksOk` (đều boolean) | `WorkspaceService.ts:85-100` (`initWorkspace()`) |
| Kết thúc | `ok({ hasGitStatus, worktreeCount, fileTreeNodeCount })` | | `WorkspaceService.ts:113` (return) |

```typescript
// WorkspaceService.ts — initWorkspace()
async initWorkspace(projectId: string, userId: string): Promise<WorkspaceInitResult> {
  const span = Tracers.projectWorkspaceInitFlow.start({ projectId, userId })
  const relay = await this.router.getRelayForProject(projectId, userId).catch((err) => {
    span.step('relay-resolve', { projectId, relayReused: false, error: String(err) })
    return null
  })
  if (relay) span.step('relay-resolve', { projectId, relayReused: true })

  const [gitStatusRaw, worktreeRaw, fileTreeRaw, pendingTasks] = await Promise.all([
    relay ? relay.call('git.exec', { args: ['status', '--porcelain=v2', '--branch'] })
      .catch((err) => { span.step('parallel-fetch', { gitStatusOk: false, error: String(err) }); return null })
      : Promise.resolve(null),
    // ...tương tự cho worktreeRaw, fileTreeRaw...
    this.taskService.list({ projectId, limit: 100 })
      .then(tasks => tasks.filter(t => ['todo', 'in_progress', 'blocked'].includes(t.status)))
      .catch((): OrcaTask[] => []),
  ])

  const worktrees = (worktreeRaw as { worktrees?: GitWorktree[] } | null)?.worktrees ?? []
  const fileTree = Array.isArray(fileTreeRaw) ? fileTreeRaw : []
  span.ok({ hasGitStatus: !!gitStatusRaw, worktreeCount: worktrees.length, fileTreeNodeCount: fileTree.length })
  return { gitStatus: gitStatusRaw?.stdout ? this.parseGitStatus(gitStatusRaw.stdout) : null, worktrees, fileTree, pendingTasks }
}
```

**Lưu ý quan trọng:** hiện tại `.catch(() => null)` nuốt lỗi hoàn toàn (không giữ `err`). CR này yêu cầu sửa các `.catch()` trong `initWorkspace()` để log field `error` vào span TRƯỚC KHI trả `null` — đây là thay đổi hành vi tối thiểu (không đổi control flow, giữ nguyên "offline-tolerant") nhưng cần thiết để tracing có tác dụng.

### BL-PW-02 — Remote File Explorer

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `workspace.refreshFileTree` | `start` | `projectId`, `path` | `workspace-rpc-handler.ts:49` |
| Lấy relay | (gộp — dùng lại `getRelayForProject`, không step riêng nếu đã có sẵn từ init) | | `ProjectServerRouter.ts:29` |
| Gọi `fs.readDir` | `step('agent-call')` | `method: 'fs.readDir'`, `path`, `depth` | `WorkspaceService.ts` (`refreshFileTree`, dòng ~150-155) |
| Kết thúc | `ok({ entryCount })` / `fail(err)` | | |

```typescript
// WorkspaceService.ts — refreshFileTree()
async refreshFileTree(projectId: string, userId: string, path?: string): Promise<FileTreeNode[]> {
  const span = Tracers.projectWorkspaceExplorerFlow.start({ projectId, path: path ?? '.' })
  const relay = await this.router.getRelayForProject(projectId, userId).catch(() => null)
  if (!relay) { span.fail('RELAY_UNAVAILABLE'); return [] }

  span.step('agent-call', { method: 'fs.readDir', path: path ?? '.', depth: 2 })
  const result = await relay.call('fs.readDir', { path: path ?? '.', depth: 2 })
    .catch((err) => { span.fail(err); return null }) as FileTreeNode[] | null

  const entries = Array.isArray(result) ? result : []
  span.ok({ entryCount: entries.length })
  return entries
}
```

**`fs.readFile` và `fs.search`** (mô tả trong flow doc cho "Open File" và "File Search") chưa xác định file cụ thể triển khai qua relay — khi tính năng này được xác nhận có RPC method riêng, áp dụng cùng pattern span (`start` → `step('agent-call')` → `ok`/`fail`) trên cùng tracer `projectWorkspaceExplorerFlow`, phân biệt bằng field `operation: 'readDir' | 'readFile' | 'search'`.

### BL-PW-03 — Remote Git UI Operations

Mỗi RPC git op (`git.add`, `git.commit`, `git.push`, `git.generateCommitMessage`, `git.pr.create` trong `git-remote.ts`) là 1 span độc lập — vì đây là hành động rời rạc do user chủ động bấm (Stage, Commit, Push, AI Generate, Create PR), không phải 1 flow liên tục tự động.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC git op | `start` | `op` (`'add'\|'commit'\|'push'\|'generateCommitMessage'\|'pr.create'`), `worktreePath` | `git-remote.ts:118,146,168,353,394` |
| Gọi relay | `step('agent-call')` | `method: 'git.exec' \| 'ai.complete'` | `git-remote.ts` (mỗi handler, ví dụ dòng 383 cho AI commit message) |
| Kết thúc | `ok({ exitCode })` / `fail(err)` | | |

```typescript
// git-remote.ts — ví dụ cho 'git.push' (dòng 168)
defineMethod({
  name: 'git.push',
  handler: async (params, ctx) => {
    const span = Tracers.projectWorkspaceGitOpFlow.start({ op: 'push', worktreePath: params.worktreePath })
    try {
      const relay = await router.getRelayForProject(ctx.projectId, ctx.userId)
      span.step('agent-call', { method: 'git.exec', args: ['push', 'origin', params.branch] })
      const result = await relay.call('git.exec', { cwd: params.worktreePath, args: ['push', 'origin', params.branch] })
      span.ok({ exitCode: result.exitCode })
      return result
    } catch (err) {
      span.fail(err, { op: 'push' })
      throw err
    }
  }
})
```

**AI Commit Message (`git.generateCommitMessage`, dòng 353, relay `ai.complete` dòng 383)** đáng có field riêng `method: 'ai.complete'` để phân biệt latency AI call khỏi latency git diff — theo đúng nguyên tắc §5 CR-TRACE-000 (network call ra ngoài, có thể chậm độc lập).

### BL-PW-04 — Workspace Integration (Agent+Git+Tasks+Workflows)

> **Ghi chú quan trọng:** `TaskWorkspaceIntegrator` và `workspace.openTaskContext` mô tả trong flow doc **không tồn tại trong code hiện tại** (grep xác nhận). Sub-flow BL-PW-04 dường như là composite/tương lai của BL-PW-01 (workspace switch) + BL-TG-04 (agent execute, xem CR-TRACE-018) + BL-PW-03 (git ops) — không có 1 hàm điều phối riêng để gắn span cha. Đây là **placeholder instrumentation spec**.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Mở task workspace context | `start` → `ok`/`fail` | `taskId`, `projectId` | chưa xác định file cụ thể — cần điều tra thêm khi triển khai |
| Switch worktree | `step('worktree-switch')` | `worktreeId` | chưa xác định file cụ thể — cần điều tra thêm khi triển khai (liên quan `git.worktree.add`/`remove` đã có trong `git-remote.ts` dòng 320/337, nhưng không thấy `worktree.switch`) |

Khi tính năng này được implement, span cha (`projectWorkspaceAgentAttachFlow`) nên dùng cùng mô hình `parentTraceId` như CR-TRACE-017 §4 để nhóm các span con: `projectWorkspaceInitFlow` (BL-PW-01 tái sử dụng) + `taskGraph:execute` (CR-TRACE-018) + `projectWorkspaceGitOpFlow` × N (stage/commit/push/PR).

## 5. Lan truyền traceId qua transport của flow này

- **Browser → Orca Server (`workspace.init`, `workspace.refreshFileTree`, `git.*` WS RPC)**: theo CR-TRACE-000 §3.3 hàng 1 — `workspace-rpc-handler.ts` và `git-remote.ts` đọc `params.traceId` (nếu FE gửi) để `resume` span tương ứng, giống pattern đã có ở `devServer.browseDir` (`dev-server.ts:208`).
- **Orca Server → Dev Server (mọi `relay.call('git.exec' | 'fs.readDir' | 'ai.complete', ...)`)**: theo CR-TRACE-000 §3.3 hàng 2 — đính `traceId: span.id` vào params envelope của từng `relay.call()` trong `WorkspaceService.ts` và `git-remote.ts`. Vì `ProjectServerRouter.getRelayForProject()` là điểm dùng chung để lấy `DevServerRelayBridge`, cách đơn giản nhất là mỗi call site tự thêm `traceId` vào object params trước khi gọi `.call()` — không cần sửa `ProjectServerRouter` hay `RelayConnectionPool`.
- **BL-PW-01's parallel fetch**: vì 4 lời gọi (`git.exec`, `git.worktree.list`, `fs.readDir`, `taskService.list`) chạy song song dưới 1 span cha (`projectWorkspaceInitFlow`), cả 3 relay call nên mang **cùng 1** `traceId` (= `span.id` của span cha) — khác với mô hình BL-WF-02/BL-TG (mỗi step 1 span con riêng) vì đây không phải các bước độc lập của 1 pipeline mà là 1 "gather" duy nhất; dùng `span.step()` để log riêng từng nhánh là đủ, không cần span con riêng biệt.

## Acceptance Criteria

- [ ] `Tracers.projectWorkspaceInitFlow` ghi lại rõ nhánh nào trong `Promise.all` (`git.exec`/`git.worktree.list`/`fs.readDir`/`taskService.list`) fail, thay vì nuốt lỗi hoàn toàn qua `.catch(() => null)`
- [ ] `Tracers.projectWorkspaceExplorerFlow` bao phủ `refreshFileTree()`, phân biệt được `RELAY_UNAVAILABLE` khỏi lỗi từ `fs.readDir` chính nó
- [ ] `Tracers.projectWorkspaceGitOpFlow` tạo 1 span riêng cho mỗi loại git op (`add`/`commit`/`push`/`generateCommitMessage`/`pr.create`), field `op` phân biệt rõ loại
- [ ] AI commit message (`git.generateCommitMessage`) có field `method: 'ai.complete'` riêng để tách latency AI khỏi latency git diff
- [ ] `traceId` forward đúng vào mọi `relay.call()` trong `WorkspaceService.ts` và `git-remote.ts` theo CR-TRACE-000 §3.3 hàng 2
- [ ] BL-PW-04 tracer (`projectWorkspaceAgentAttachFlow`) được document nhưng gắn cờ rõ "chưa triển khai" — không tạo call site giả trong code khi merge CR
- [ ] Không thêm `span.step()` cho các DB SELECT/UPDATE đơn dòng (`taskService.list` bên trong `initWorkspace`, `parseGitStatus` in-memory parse) theo nguyên tắc §5 CR-TRACE-000
- [ ] TracePanel hiển thị 4 tracer mới dưới namespace `projectWorkspace:*`, không đụng `devServer:*`/`taskGraph:*`/`workflow:*` sẵn có
