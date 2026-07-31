# TDD-FE-03: Runtime Client Layer

**Document:** TDD-FE-03  
**Domain:** Runtime Client — RPC, State Sync, Web Session  
**Source files:** `src/renderer/src/runtime/`

---

## 1. Tổng quan

`src/runtime/` là **bridge layer** giữa frontend store và Orca backend:

```
Frontend (Zustand store)
  │
  ├─ sync-runtime-graph.ts   ← đẩy snapshot từ backend vào store
  ├─ runtime-rpc-client.ts   ← gọi RPC methods (qua IPC hoặc WebSocket)
  ├─ web-runtime-session.ts  ← session-level ops (create terminal, split, ...)
  ├─ web-session-tabs-sync.ts ← sync tab state với remote runtime
  └─ remote-runtime-terminal-multiplexer.ts ← PTY multiplexing cho web
```

---

## 2. Runtime RPC Client (`runtime-rpc-client.ts`)

```typescript
// src/renderer/src/runtime/runtime-rpc-client.ts

// Target types:
type RuntimeClientTarget =
  | { kind: 'local' }                        // Desktop (Electron IPC)
  | { kind: 'environment'; environmentId: string }  // Web (WebSocket)

// Main API:
async function callRuntimeRpc<TResult>(
  target: RuntimeClientTarget,
  method: string,
  params?: unknown,
  options?: { timeoutMs?: number; ... }
): Promise<TResult>

// Ví dụ:
const result = await callRuntimeRpc(
  { kind: 'local' },
  'worktree.create',
  { repoId: 'repo-123', branch: 'feature/fix-bug' }
)

// Với web target:
const result = await callRuntimeRpc(
  { kind: 'environment', environmentId: 'env-abc' },
  'terminal.subscribe',
  { ptyId: 'pty-456' }
)
```

### Compatibility Check

```typescript
// Khi kết nối tới remote runtime environment, check version compatibility:
async function ensureRuntimeEnvironmentCompatible(
  environmentId: string,
  options?: { reuseRecentCompatibilityFailure?: boolean }
): Promise<void> {
  // Gọi status.get
  // assertRuntimeStatusCompatible() → throw nếu version không match
  // Cache kết quả 60s (RUNTIME_CAPABILITY_STATUS_TTL_MS)
}

// Error types:
class RuntimeRpcCallError extends Error {
  code: string   // 'method_not_found' | 'forbidden' | 'runtime_busy' | ...
}

function isRuntimeScopeForbiddenError(error: unknown): boolean
// → true nếu mobile-scope device cố gọi full-scope method
```

---

## 3. Sync Runtime Graph (`sync-runtime-graph.ts`)

Core của frontend-backend state synchronization. ~56K code.

### Mục đích

Backend (Main Process) có bản "canonical graph" của mọi entities.  
Frontend Zustand store cần được **sync** với graph này sau mỗi mutating operation.

```typescript
// Trigger sync:
scheduleRuntimeGraphSync()
// → Debounce 16ms → callRuntimeRpc('status.get') → applyRuntimeGraph()

// Inject dependencies:
setRuntimeGraphStoreStateGetter(() => useAppStore.getState())
setRuntimeGraphSyncEnabled(true)   // false khi đang init
```

### Sync Flow

```typescript
// src/renderer/src/runtime/sync-runtime-graph.ts
async function performRuntimeGraphSync(): Promise<void> {
  // 1. Gọi status.get (hoặc nội bộ via IPC)
  const graph: RuntimeSyncWindowGraph = await getRuntimeGraph()

  // 2. Deserialize entities từ graph
  const { repos, worktrees, terminals, tabs, sshTargets } = graph

  // 3. Apply vào store (merge, không replace)
  useAppStore.setState(state => {
    // Update repos
    for (const repo of graph.repos) {
      state.repos[repo.id] = repo
    }
    // Prune removed entities
    for (const removedId of graph.removedRepoIds ?? []) {
      delete state.repos[removedId]
    }
    // ... similar for worktrees, terminals, etc.
  })
}
```

### RuntimeSyncWindowGraph type

```typescript
// src/shared/runtime-types.ts
type RuntimeSyncWindowGraph = {
  repos: Repo[]
  worktrees: WorktreeMeta[]
  sshTargets: SshTarget[]
  globalSettings: GlobalSettings
  automations: Automation[]
  projectGroups: ProjectGroup[]
  folderWorkspaces: FolderWorkspace[]
  // Terminal layout (for mobile):
  terminalLayoutSnapshot?: TerminalLayoutSnapshot
  // Active tabs/windows:
  sessionTabs?: RuntimeMobileSessionTabsSnapshot
  // Removed entity IDs:
  removedRepoIds?: string[]
  removedWorktreeIds?: string[]
}
```

### Registered Terminal Tabs

Mỗi terminal tab phải đăng ký với sync graph để cung cấp PTY state:

```typescript
// Terminal component tự đăng ký khi mount:
registerRuntimeGraphTerminalTab({
  tabId,
  worktreeId,
  getManager: () => paneManagerRef.current,        // layout manager
  getContainer: () => containerRef.current,         // DOM container
  getPtyIdForPane: (paneId) => ptyMap.get(paneId)  // pane → pty mapping
})

// Unregister khi unmount:
unregisterRuntimeGraphTerminalTab(tabId)
```

---

## 4. Web Runtime Session (`web-runtime-session.ts`)

Handles session-level operations khi dùng **web client** (browser):

```typescript
// Check nếu web session active:
function isWebRuntimeSessionActive(activeRuntimeEnvironmentId: string | null): boolean

// Tạo terminal trong web session:
async function createWebRuntimeSessionTerminal(args: {
  worktreeId: string
  environmentId?: string | null
  afterTabId?: string
  command?: string
  cwd?: string
  agent?: TuiAgent
  activate?: boolean
}): Promise<boolean> {
  // → window.api.runtimeEnvironments.call({
  //     method: 'session.tabs.createTerminal',
  //     params: { worktree, ... }
  //   })
}

// Split terminal:
async function splitWebRuntimeSessionTerminal(args: { ... }): Promise<boolean>

// Close terminal:
async function closeWebRuntimeSessionTerminal(tabId: string): Promise<boolean>

// Select worktree (switch active):
function selectWebRuntimeSessionWorktree(worktreeId: string): void
```

---

## 5. Web Session Tabs Sync (`web-session-tabs-sync.ts`)

~97KB — File lớn nhất trong runtime/ layer.  
Sync **session tabs** giữa web client và remote runtime:

```typescript
// src/renderer/src/runtime/web-session-tabs-sync.ts
export function useWebSessionTabsSync(): void {
  // Hook chạy trong App.tsx
  // Subscribe tới session tab updates từ runtime
  // Apply vào Zustand tabs slice

  useEffect(() => {
    const unsubscribe = subscribeToSessionTabUpdates(tabsSnapshot => {
      useAppStore.getState().applySessionTabsSnapshot(tabsSnapshot)
    })
    return unsubscribe
  }, [])
}
```

---

## 6. Remote Runtime Terminal Multiplexer (`remote-runtime-terminal-multiplexer.ts`)

~30KB — Xử lý multiplexing PTY data từ remote runtime:

```typescript
// src/renderer/src/runtime/remote-runtime-terminal-multiplexer.ts
class RemoteRuntimeTerminalMultiplexer {
  // Mỗi khi web client cần terminal data từ remote:
  // 1. Subscribe tới ptyId trên WebSocket
  // 2. Route incoming TerminalStreamFrames tới đúng xterm.js instance
  // 3. Handle resync sau disconnect

  subscribe(ptyId: string, onData: (data: Uint8Array) => void): Subscription
  unsubscribe(ptyId: string): void

  // Backpressure: nếu xterm.js không đọc kịp
  // → Buffer frames trong memory
  // → Signal server giảm tốc
}
```

---

## 7. Runtime File Client (`runtime-file-client.ts`)

~36KB — File operations qua runtime layer:

```typescript
// src/renderer/src/runtime/runtime-file-client.ts
// Abstraction layer: local (Electron IPC) vs remote (RPC)

async function runtimeReadFile(
  target: RuntimeClientTarget,
  path: string
): Promise<Uint8Array>

async function runtimeWriteFile(
  target: RuntimeClientTarget,
  path: string,
  data: Uint8Array
): Promise<void>

async function runtimeListDir(
  target: RuntimeClientTarget,
  path: string,
  opts?: ListOptions
): Promise<DirEntry[]>

async function runtimeSearchFiles(
  target: RuntimeClientTarget,
  args: SearchArgs
): Promise<SearchResult[]>
```

---

## 8. Runtime Git Client (`runtime-git-client.ts`)

~27KB — Git operations:

```typescript
// src/renderer/src/runtime/runtime-git-client.ts

async function runtimeGitStatus(target, repoPath): Promise<GitStatus>
async function runtimeGitLog(target, repoPath, opts): Promise<GitLogEntry[]>
async function runtimeGitDiff(target, repoPath, opts): Promise<string>
async function runtimeGitCommit(target, repoPath, message): Promise<void>
async function runtimeGitPush(target, repoPath, args): Promise<void>
async function runtimeGitBranch(target, repoPath): Promise<BranchInfo[]>
async function runtimeGitMerge(target, repoPath, branch): Promise<MergeResult>
```

---

## 9. Runtime Status (`runtime-status.ts`)

~6KB — Track connection health:

```typescript
// Zustand slice: runtimeStatus
type RuntimeStatusSlice = {
  runtimeStatus: 'connected' | 'connecting' | 'disconnected' | 'error'
  runtimeError: string | null
  runtimeVersion: string | null
  runtimePlatform: string | null
}

// Cập nhật sau mỗi RPC response envelope:
// meta.serverVersion, meta.serverPlatform
// Nếu không nhận response 30s → 'disconnected'
```

---

## 10. Mobile Session Sync

Khi Orca Mobile App kết nối, runtime layer cũng handle sync cho mobile:

```typescript
// src/renderer/src/runtime/sync-runtime-graph.ts
// Phần mobile session:

type RuntimeMobileSessionTabsSnapshot = {
  tabGroups: RuntimeMobileSessionTabGroup[]
  activeTabIdByGroup: Record<string, string>
}

// Snapshot được push xuống mobile app mỗi khi tabs thay đổi:
function buildMobileSessionSnapshot(store: AppState): RuntimeMobileSessionTabsSnapshot {
  // Convert internal tab state → mobile-compatible format
  // Mobile chỉ hiển thị tabs, không render terminal
}
```

---

## 11. Runtime Client Events (`runtime-client-events.ts`)

```typescript
// src/renderer/src/runtime/runtime-client-events.ts
// Push events từ runtime tới frontend (không phải request-response)

// Ví dụ events:
type RuntimeClientEvent =
  | { type: 'worktree.created'; worktreeId: string }
  | { type: 'worktree.deleted'; worktreeId: string }
  | { type: 'agent.statusChanged'; ptyId: string; status: AgentStatus }
  | { type: 'ssh.connectionChanged'; targetId: string; state: SshConnectionState }
  | { type: 'automation.runCompleted'; runId: string; success: boolean }
  | { type: 'notification.received'; notification: NotificationEntry }

// Handler: trong useIpcEvents() (Desktop) hoặc WebSocket event listener (Web)
```

---

## restructure_v1 Addendum: IRpcClient Interface Layer

Từ restructure_v1, có thêm một interface layer trong `src/platform/`:

```
src/platform/
├── rpc-client-interface.ts        IRpcClient — shared interface
└── adapters/web/
    └── rpc-client.ts              WebSocketRpcClient — concrete impl
```

### Vai trò trong kiến trúc

```
ConnectionStatusProvider
  └─ uses IRpcClient (từ platform/rpc-client-interface.ts)
       └─ impl: WebSocketRpcClient (platform/adapters/web/rpc-client.ts)
            └─ JSON-RPC over WebSocket (plain, no E2EE)

web-preload-api.ts (~135KB)
  └─ uses WebRuntimeClient (web/web-runtime-client.ts)
       └─ full E2EE protocol qua WebSocket
       
callRuntimeRpc() (renderer/src/runtime/runtime-rpc-client.ts)
  └─ Electron IPC (Desktop) | WebRuntimeClient (Web)
```

### IRpcClient vs WebRuntimeClient

`WebSocketRpcClient` (implements `IRpcClient`) là transport **đơn giản, không E2EE**, chuyên dùng cho:
1. Bootstrap connection test (trước khi pairing)
2. `ConnectionStatusProvider` polling `isConnected()`

`WebRuntimeClient` là transport **full E2EE**, dùng sau khi pairing thành công để chạy tất cả RPC calls từ `web-preload-api.ts`.

### Test pattern cho IRpcClient mocks

```typescript
// Trong tests, mock IRpcClient:
function createMockClient(connected = true): IRpcClient {
  return {
    isConnected: vi.fn().mockReturnValue(connected),
    on: vi.fn().mockReturnValue(() => {}),
    off: vi.fn(),
    disconnect: vi.fn(),
    connect: vi.fn().mockResolvedValue(undefined),
    invoke: vi.fn(),
    send: vi.fn(),
    once: vi.fn()
  }
}
```
