# TDD-07: OrcaRuntimeService — Core Business Logic

**Document:** TDD-07  
**Domain:** OrcaRuntimeService — Runtime, Worktrees, Git, Projects  
**Source files:** `src/main/runtime/orca-runtime.ts` (~989KB, 26682 lines!)

---

## 1. Vị trí trong kiến trúc

`OrcaRuntimeService` là **orchestrator trung tâm** của toàn bộ business logic:

```
OrcaRuntimeRpcServer
  └─ RpcDispatcher
       └─ Method handlers (worktree.ts, git.ts, terminal.ts, ...)
            └─ OrcaRuntimeService  ← ĐÂY
                 ├─ Store (persistence)
                 ├─ DaemonPtyAdapter (PTY)
                 ├─ SshConnectionManager (SSH)
                 ├─ FileWatcher (fs events)
                 └─ Git runner
```

---

## 2. Constructor và Dependencies

```typescript
// src/main/runtime/orca-runtime.ts
class OrcaRuntimeService {
  constructor(
    private store: Store,
    private daemon: DaemonPtyAdapter | DaemonPtyRouter,
    private sshManager: SshConnectionManager,
    private options: RuntimeOptions
  )

  // Lazy-init services:
  private orchestrationDb: OrchestrationDb   // agent orchestration
  private fileWatcher: FileWatcher           // parcel-watcher
  private browser: BrowserPaneService        // headless Chromium
}
```

---

## 3. Project Management

```typescript
// Projects là top-level container
createProject(args: { name: string; repoIds?: string[] }): Project
updateProject(id: string, args: ProjectUpdateArgs): Project
deleteProject(id: string): void

// Project host setup:
// Khai báo cách setup project trên execution host
setupProjectHost(args: ProjectHostSetupCreateArgs): ProjectHostSetupCreateResult
// Ví dụ: clone repo, run setup script

getProjects(): Project[]
getProject(id: string): Project | undefined
```

---

## 4. Repo Management

```typescript
// Repos là git repositories
createRepo(args: {
  path: string
  projectId?: string
  connectionId?: ExecutionHostId   // local hoặc ssh-<id>
  name?: string
}): Repo

updateRepo(id: string, updates: Partial<Repo>): Repo
deleteRepo(id: string): void

// Remote repos (qua SSH):
createRemoteRepo(args: {
  connectionId: string    // ssh target id
  path: string            // path trên remote server
  projectId?: string
}): Repo

getRepos(): Repo[]
getReposForProject(projectId: string): Repo[]
```

---

## 5. Worktree Lifecycle

Worktrees là core feature của Orca — mỗi task chạy trong **git worktree** riêng:

```typescript
// Tạo worktree mới:
async createWorktree(args: {
  repoId: string
  branch?: string          // tạo branch mới hoặc checkout existing
  baseBranch?: string      // từ branch nào fork
  fromPr?: GitHubPrStartPoint  // từ GitHub PR
  worktreeRootOverride?: string  // custom root path
}): Promise<CreateWorktreeResult> {

  // 1. Validate repo
  const repo = this.store.getRepo(args.repoId)

  // 2. Resolve execution host (local vs SSH)
  const host = this.resolveExecutionHost(repo.connectionId)

  // 3. Prepare worktree directory
  const wtPath = await resolveWorktreeCreateBase(repo.path, args)

  // 4. Run git worktree add
  await gitExecFileAsync(['worktree', 'add', wtPath, args.branch ?? '-b', branchName])

  // 5. Run orca.yaml setup script
  await this.runWorktreeSetupScript(wtPath, repo)

  // 6. Persist metadata
  const meta: WorktreeMeta = { id, repoId, path: wtPath, branch: branchName }
  this.store.addWorktree(meta)

  return { worktreeId: id, path: wtPath }
}
```

### Worktree Detection

```typescript
// Detect worktrees từ git (không chỉ persist)
async detectWorktrees(repoId: string): Promise<DetectedWorktreeListResult>
// Chạy: git worktree list --porcelain
// Reconcile với store metadata
```

### Worktree Cleanup

```typescript
// Remove worktree:
async deleteWorktree(args: {
  worktreeId: string
  force?: boolean        // git worktree remove --force
  deleteBranch?: boolean // xóa branch luôn
}): Promise<void> {
  // 1. Kill active PTYs
  await this.killPtysInWorktree(worktreeId)

  // 2. Run git worktree remove
  await gitExecFileAsync(['worktree', 'remove', wtPath, ...(force ? ['--force'] : [])])

  // 3. Remove from store
  this.store.removeWorktree(worktreeId)
}
```

---

## 6. Terminal (PTY) Management

```typescript
// Tạo terminal trong worktree:
async createTerminal(args: {
  worktreeId: string
  command?: string      // custom command (default: shell)
  env?: Record<string, string>
  cols?: number
  rows?: number
}): Promise<string> {    // returns ptyId

  const wt = this.store.getWorktree(args.worktreeId)
  const host = this.resolveExecutionHost(wt.repoId)

  if (host.type === 'local') {
    // Tạo PTY qua daemon
    return this.daemon.createPty({
      cwd: wt.path,
      shell: this.getShellForHost(host),
      env: { ...process.env, ...args.env },
      cols: args.cols ?? 80,
      rows: args.rows ?? 24
    })
  } else {
    // SSH relay PTY
    return this.sshManager.getSession(host.targetId).createPty({ ... })
  }
}
```

---

## 7. Git Operations

```typescript
// src/main/git/runner.ts
// Git operations thông qua child_process.execFile('git', args)
// Với caching cho git status (avoid redundant calls)

gitExecFileAsync(args: string[], options?: GitExecOptions): Promise<ExecResult>
gitSpawn(args: string[], options?: GitSpawnOptions): ChildProcess

// In runtime:
async getGitStatus(worktreeId: string): Promise<GitStatus>
async gitCommit(worktreeId: string, message: string): Promise<void>
async gitPush(worktreeId: string, args: GitPushTarget): Promise<void>
async gitFetch(worktreeId: string): Promise<void>
async gitDiff(worktreeId: string, opts?: DiffOptions): Promise<string>
async gitLog(worktreeId: string, opts?: LogOptions): Promise<GitLogEntry[]>
async gitBranch(worktreeId: string): Promise<BranchInfo[]>
async createBranch(worktreeId: string, name: string, from?: string): Promise<void>
```

---

## 8. File Watch Service

```typescript
// Parcel-watcher cho filesystem events
// src/main/ipc/filesystem-watcher.ts (+ parcel-watcher-process-supervisor.ts)

// Chạy watcher trong separate process (parcel-watcher-process-entry.ts)
// Vì parcel-watcher native addon — crash isolation

async watchDirectory(path: string, callback: (events: WatchEvent[]) => void): Watcher
stopWatcher(path: string): void

// Events:
type WatchEvent = {
  type: 'create' | 'update' | 'delete'
  path: string
}
```

---

## 9. Agent Status Tracking

```typescript
// src/main/runtime/orca-runtime.ts — agent status management
// Track trạng thái AI agent trong mỗi terminal

updateAgentStatus(args: {
  worktreeId: string
  ptyId: string
  status: AgentStatus    // 'idle' | 'running' | 'waiting' | 'error'
  title?: string
}): void

getAgentStatusForWorktree(worktreeId: string): AgentStatusEntry | null

// Detection từ terminal title/output:
// - Claude: terminal title match pattern
// - Cursor: custom ANSI sequence
// - Codex: title pattern
// - Droid: OSC sequence
```

---

## 10. Execution Host Resolution

```typescript
// src/shared/execution-host.ts + src/main/runtime/
type ExecutionHostId =
  | 'local'
  | `ssh-${string}`           // ssh-<target-id>
  | `ephemeral-vm-${string}`  // on-demand VM

// Resolution:
resolveExecutionHost(hostId: ExecutionHostId): ExecutionHost {
  if (hostId === 'local') return { type: 'local' }
  if (hostId.startsWith('ssh-')) {
    const targetId = hostId.slice(4)
    const target = this.store.getSshTarget(targetId)
    const session = this.sshManager.getSession(targetId)
    return { type: 'ssh', target, session }
  }
  // ephemeral-vm: tương tự SSH
}
```

### Addendum v5.0: `getRepoProviderConnectionKey` — IMPLEMENTED ✅ (2026-08-02/03)

`Repo` giờ có thể gắn remote host qua `connectionId` (SSH target) **hoặc** `devServerId` (Dev Server, xem TDD-13 §11) — 2 field loại trừ nhau. Helper `getRepoProviderConnectionKey(repo)` (`src/shared/execution-host.ts`) trả về bare provider-registry key dùng chung cho cả 2:

```typescript
function getRepoProviderConnectionKey(
  repo: Pick<Repo, 'connectionId' | 'devServerId'>
): string | null {
  return repo.connectionId ?? repo.devServerId ?? null
}
```

Khác với `ExecutionHostId` ở trên (dạng prefix `ssh:<id>` / `devServer:<id>`, dùng cho UI/host-scope selection), key này là raw id truyền thẳng vào `getSshGitProvider`/`getSshFilesystemProvider` — 2 registry giờ transport-agnostic, chứa cả provider SSH-backed lẫn Dev-Server-backed (xem [05-ssh-relay.md Addendum v5.0](./05-ssh-relay.md#addendum-v50-provider-registries-are-transport-agnostic)). Áp dụng tại choke-point `resolveRuntimeGitTarget`/`resolveRuntimeFileTarget` trong `orca-runtime.ts`, fix ~45 call site downstream (orca-runtime-git.ts/orca-runtime-files.ts) miễn phí, cùng ~24 hàm trong `worktree-remote.ts` và 53 call site trong `worktrees.ts`.

---

## 11. Clone và Setup Flow

```typescript
// Clone repo về execution host:
async cloneRepo(args: {
  url: string
  path: string
  connectionId?: ExecutionHostId
  branch?: string
}): Promise<Repo> {

  const host = this.resolveExecutionHost(args.connectionId ?? 'local')

  if (host.type === 'local') {
    await gitExecFileAsync(['clone', args.url, args.path, '--progress'])
  } else {
    // Clone qua SSH relay
    await host.session.gitExec(args.path, ['clone', args.url, '.', '--progress'])
  }

  // Detect git metadata
  const [remoteName, defaultBranch] = await getGitRemoteIdentity(args.path)

  // Persist repo
  const repo = this.store.addRepo({
    id: generateId('repo'),
    name: extractRepoName(args.url),
    path: args.path,
    connectionId: args.connectionId,
    gitRemoteIdentity: { remoteName, defaultBranch }
  })

  return repo
}
```

---

## 12. Orchestration DB

```typescript
// src/main/runtime/orchestration/db.ts
// Quản lý multi-agent orchestration state

class OrchestrationDb {
  // Lưu orchestration tasks, progress, results
  createTask(input: OrchestrationTaskInput): OrchestrationTask
  getTask(id: string): OrchestrationTask | undefined
  updateTask(id: string, patch: Partial<OrchestrationTask>): void
  listTasks(filter: TaskFilter): OrchestrationTask[]
  cancelTask(id: string): void
}
```
