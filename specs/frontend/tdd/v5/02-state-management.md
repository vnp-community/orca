# TDD-FE-02: State Management — Zustand Store

**Document:** TDD-FE-02  
**Domain:** Zustand Store — Slices, Selectors, State Shape  
**Source files:** `src/renderer/src/store/`

---

## 1. Tổng quan

Orca dùng **Zustand** (không Redux, không Context API) cho toàn bộ global state.

**Pattern:** Single store composed từ 40+ slices:

```typescript
// src/renderer/src/store/index.ts
export const useAppStore = create<AppState>()((...a) => ({
  ...createRepoSlice(...a),
  ...createWorktreeSlice(...a),
  ...createTerminalSlice(...a),
  ...createTabsSlice(...a),
  // ... 36 more slices
}))
```

**Không dùng Redux** vì:
- Boilerplate thấp hơn (không cần actions/reducers riêng)
- Middleware đơn giản hơn
- Direct state mutation trong slice (với immer pattern bên trong)

---

## 2. Danh sách tất cả Slices

| Slice | File | Mô tả |
|-------|------|--------|
| `repos` | `slices/repos.ts` (~115K) | Repositories, project groups, SSH repos |
| `worktrees` | `slices/worktrees.ts` (~192K) | Worktree lifecycle, metadata |
| `terminals` | `slices/terminals.ts` (~146K) | Terminal sessions, PTY states |
| `tabs` | `slices/tabs.ts` (~78K) | Tab groups, tab order, active tabs |
| `ui` | `slices/ui.ts` (~103K) | Sidebar, modals, layout, zoom, view state |
| `editor` | `slices/editor.ts` (~176K) | Open files, editor tabs, diff views |
| `github` | `slices/github.ts` (~156K) | GitHub PRs, issues, checks cache |
| `linear` | `slices/linear.ts` (~75K) | Linear issues, teams, projects |
| `jira` | `slices/jira.ts` (~22K) | Jira issues |
| `browser` | `slices/browser.ts` (~75K) | Browser pane state, webview |
| `agent-status` | `slices/agent-status.ts` (~105K) | AI agent status per terminal |
| `ssh` | `slices/ssh.ts` (~7K) | SSH connection states |
| `settings` | `slices/settings.ts` (~8K) | User preferences |
| `keybindings` | `slices/keybindings.ts` (~4K) | Keyboard shortcuts |
| `preflight` | `slices/preflight.ts` (~4K) | Startup checks |
| `sparse-presets` | `slices/sparse-presets.ts` (~8K) | Git sparse checkout |
| `diff-comments` | `slices/diffComments.ts` (~19K) | PR diff inline comments |
| `detected-agents` | `slices/detected-agents.ts` (~13K) | Detected AI agents in terminals |
| `worktree-nav-history` | `slices/worktree-nav-history.ts` (~12K) | Navigation history |
| `runtime-status` | `slices/runtime-status.ts` (~6K) | Runtime connection health |
| `runtime-environment-ssh` | `slices/runtime-environment-ssh.ts` (~9K) | SSH runtime envs |
| `workspace-cleanup` | `slices/workspace-cleanup.ts` (~32K) | Workspace GC state |
| `hosted-review` | `slices/hosted-review.ts` (~17K) | Hosted code review |
| `claude-usage` | `slices/claude-usage.ts` (~6K) | Claude API usage meter |
| `codex-usage` | `slices/codex-usage.ts` (~5K) | Codex usage meter |
| `opencode-usage` | `slices/opencode-usage.ts` (~5K) | OpenCode usage |
| `rate-limits` | `slices/rate-limits.ts` (~5K) | AI rate limiting |
| `stats` | `slices/stats.ts` (~1K) | Usage stats |
| `memory` | `slices/memory.ts` (~2K) | Memory pressure tracking |
| `workspace-space` | `slices/workspace-space.ts` (~5K) | Workspace board layout |
| `pull-request-generation` | `slices/pull-request-generation.ts` (~9K) | AI PR description gen |
| `commit-message-generation` | `slices/commit-message-generation.ts` (~5K) | AI commit msg gen |
| `pane-foreground-agent` | `slices/pane-foreground-agent.ts` (~3K) | Active agent per pane |
| `pinned-tab-close-confirm` | `slices/pinned-tab-close-confirm.ts` (~1K) | Close dialog state |
| `orca-profiles` | `slices/orca-profiles.ts` (~5K) | Profile management |
| `new-issue-draft` | `slices/new-issue-draft.ts` (~2K) | New issue drafts |
| `dictation` | `slices/dictation.ts` (~1K) | Voice input state |
| `workspace-cleanup` | `slices/workspace-cleanup.ts` (~32K) | GC scan state |

---

## 3. Slice Pattern

Mỗi slice theo cùng pattern:

```typescript
// src/renderer/src/store/slices/repos.ts (ví dụ)

// 1. Type definition
type RepoSlice = {
  repos: Record<string, Repo>
  projectGroups: ProjectGroup[]
  repoOrder: string[]
  // ... actions
  addRepo: (repo: Repo) => void
  removeRepo: (repoId: string) => void
  updateRepo: (repoId: string, updates: Partial<Repo>) => void
}

// 2. Slice factory
export const createRepoSlice: StateCreator<AppState, [], [], RepoSlice> = (set, get) => ({
  repos: {},
  projectGroups: [],
  repoOrder: [],

  addRepo: (repo) => set(state => {
    state.repos[repo.id] = repo
    state.repoOrder.push(repo.id)
  }),

  removeRepo: (repoId) => set(state => {
    delete state.repos[repoId]
    state.repoOrder = state.repoOrder.filter(id => id !== repoId)
    // Cascade: remove related worktrees, terminals, tabs
  }),
})
```

---

## 4. AppState type (`store/types.ts`)

```typescript
// src/renderer/src/store/types.ts
type AppState = RepoSlice
  & WorktreeSlice
  & TerminalSlice
  & TabsSlice
  & UISlice
  & EditorSlice
  & GitHubSlice
  & LinearSlice
  & JiraSlice
  & BrowserSlice
  & AgentStatusSlice
  & SshSlice
  & SettingsSlice
  & KeybindingsSlice
  & PreflightSlice
  & SparsePresetsSlice
  & DiffCommentsSlice
  & DetectedAgentsSlice
  & WorktreeNavHistorySlice
  & RuntimeStatusSlice
  & WorkspaceCleanupSlice
  & HostedReviewSlice
  & ClaudeUsageSlice
  & CodexUsageSlice
  & OpenCodeUsageSlice
  & RateLimitSlice
  & StatsSlice
  & MemorySlice
  & WorkspaceSpaceSlice
  & PullRequestGenerationSlice
  & CommitMessageGenerationSlice
  & PaneForegroundAgentSlice
  & PinnedTabCloseConfirmSlice
  & OrcaProfilesSlice
  & NewIssueDraftSlice
  & DictationSlice
  & RuntimeEnvironmentSshSlice
```

---

## 5. Key State Shapes

### 5.1 Repos

```typescript
type RepoSlice = {
  repos: Record<string, Repo>              // keyed by repoId
  projectGroups: ProjectGroup[]
  folderWorkspaces: FolderWorkspace[]
  repoOrder: string[]                       // display order
  executionHostOrder: ExecutionHostId[]     // host display order
  visibleExecutionHostIds: ExecutionHostId[] | null
  setupScriptDismissals: Record<string, boolean>
  projectHostCapabilities: Record<string, ProjectHostCapabilities>
}
```

### 5.2 Worktrees

```typescript
type WorktreeSlice = {
  worktrees: Record<string, WorktreeMeta>   // keyed by worktreeId
  detectedWorktreesByRepo: Record<string, DetectedWorktree[]>
  worktreeCreationState: WorktreeCreationState | null
  managedWorktrees: Record<string, ManagedWorktreeState>
}
```

### 5.3 Terminals

```typescript
type TerminalSlice = {
  terminalsByWorktree: Record<string, TerminalTabInfo[]>
  terminalPtys: Record<string, PtyState>   // keyed by ptyId
  activeTerminalIdByWorktree: Record<string, string | null>
  terminalOrphansByWorktree: Record<string, string[]>
}
```

### 5.4 Tabs

```typescript
type TabsSlice = {
  tabsByWorktree: Record<string, Tab[]>    // all tabs per worktree
  tabGroups: TabGroup[]                    // group layout
  activeTabByGroup: Record<string, string>  // groupId → tabId
  tabGroupLayout: TabGroupLayoutNode | null  // split/column layout
  sessionTabs: SessionTabState | null
  pinnedTabs: string[]                      // pinned tab ids
}
```

### 5.5 UI

```typescript
type UISlice = {
  // Sidebar
  leftSidebarVisible: boolean
  leftSidebarWidth: number
  rightSidebarVisible: boolean
  rightSidebarWidth: number
  workspaceBoardVisible: boolean
  workspaceBoardOpacity: number

  // Active views
  activeWorktreeId: string | null
  activeRepoId: string | null
  activeView: AppView           // 'workspace' | 'tasks' | 'pr' | ...

  // Modals
  modalStack: ModalEntry[]
  confirmationDialog: ConfirmationDialogState | null

  // Zoom
  uiZoom: number                // 0.7 - 2.0

  // Onboarding
  onboardingVisible: boolean
  onboardingStep: number

  // Feature flags / tours
  contextualTourIds: string[]
  featureTipIds: string[]
}
```

### 5.6 Agent Status

```typescript
type AgentStatusSlice = {
  agentStatusByPtyId: Record<string, AgentStatusEntry>
  agentStatusByWorktree: Record<string, AgentStatusEntry[]>
  lastAcknowledgedAgentByWorktree: Record<string, string | null>
  agentValueMoments: AgentValueMoment[]      // for star nag
}

type AgentStatusEntry = {
  ptyId: string
  worktreeId: string
  status: AgentStatus            // 'idle' | 'running' | 'waiting' | 'error'
  title?: string                 // terminal title parsed từ ANSI/OSC
  agentType?: AgentType          // 'claude' | 'codex' | 'cursor' | ...
  lastUpdatedAt: number
  staleAfter?: number
}
```

---

## 6. Selectors (`store/selectors.ts`)

```typescript
// src/renderer/src/store/selectors.ts
// Computed selectors cho derived state:

export function selectReposForHost(
  state: AppState,
  hostId: ExecutionHostId
): Repo[] {
  return Object.values(state.repos).filter(r => r.connectionId === hostId)
}

export function selectWorktreesForRepo(
  state: AppState,
  repoId: string
): WorktreeMeta[] {
  return Object.values(state.worktrees).filter(w => w.repoId === repoId)
}

export function selectActiveTerminalForWorktree(
  state: AppState,
  worktreeId: string
): TerminalTabInfo | null {
  const activeId = state.activeTerminalIdByWorktree[worktreeId]
  return state.terminalsByWorktree[worktreeId]?.find(t => t.id === activeId) ?? null
}
```

---

## 7. Store Cascades — Garbage Collection

```typescript
// store-cascades.test.ts: 171K test file!
// Store cascades: khi xóa entity, cleanup liên quan tự động

// Ví dụ: removeRepo(repoId)
// → cascade:
// 1. Remove worktrees for repo
// 2. Remove terminals in those worktrees
// 3. Remove tabs for those terminals
// 4. Remove agent status entries
// 5. Remove editor open files
// 6. Remove diff comments
// 7. Remove GitHub/Linear/Jira cache for repo

// Leak tests (các file *.leak.test.ts):
// Verify không có memory leak sau cascade
```

---

## 8. Dev Tools

```typescript
// E2E test: đọc trực tiếp từ store
// window.__store là Zustand store
if ((import.meta.env.DEV || e2eConfig.exposeStore) && typeof window !== 'undefined') {
  (window as any).__store = useAppStore
}

// Playwright tests:
// const state = await page.evaluate(() => window.__store.getState())
// expect(state.repos['repo-123'].name).toBe('my-repo')
```

---

## 9. React Patterns cho Store

```typescript
// Pattern 1: Subscribe shallow để tránh re-render thừa
import { useShallow } from 'zustand/react/shallow'

const { repos, worktrees } = useAppStore(
  useShallow(state => ({ repos: state.repos, worktrees: state.worktrees }))
)

// Pattern 2: Selector trực tiếp
const activeWorktree = useAppStore(
  state => state.worktrees[state.activeWorktreeId ?? '']
)

// Pattern 3: Một-shot read (không subscribe)
const currentRepos = useAppStore.getState().repos

// Pattern 4: Action call
useAppStore.getState().addRepo(newRepo)
```

---

## Addendum v3.0: New Slices (onboarding + remote-server CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-23 | **TDD-FE-09/10:** [09-onboarding-devserver.md](./09-onboarding-devserver.md) | [10-fleet-management.md](./10-fleet-management.md)

### New Slices

| Slice | File | CR | Key State |
|-------|------|----|-----------|
| `DevServerSlice` | `slices/dev-servers.ts` | OB-002 | `devServers[]`, `activeDevServerId` |
| Remote Agent (ext) | `slices/onboarding.ts` | OB-003 | `agentDetectionByServer: Record<string, DetectionState>` |
| Remote Preflight (ext) | `slices/preflight.ts` | OB-005 | `remotePreflightByServer: Record<string, RemotePreflightStatus>` |
| Fleet Import (ext) | `slices/ssh.ts` | RS-001 | `fleetImportStatus: FleetImportStatus \| null` |
| Server Grouping (ext) | `slices/ssh.ts` | RS-002 | `sshTargetGroups[]`, `activeGroupFilter` |
| Bulk Provisioning (ext) | `slices/ssh.ts` | RS-003 | `bulkProvisioningProgress` |
| Bootstrap (ext) | `slices/ssh.ts` | RS-004 | `bootstrapStatusByServer: Record<string, BootstrapStatus>` |
| Fleet Health (ext) | `slices/ssh.ts` | RS-005 | `serverHealthMetrics`, `fleetAlerts[]`, `lastFleetHealthCheck` |
| RBAC (ext) | `slices/ssh.ts` | RS-006 | `currentUser`, `accessPolicy` |

### DevServerSlice (NEW)

```typescript
// src/renderer/src/store/slices/dev-servers.ts
type DevServerSlice = {
  devServers: DevServer[]
  activeDevServerId: string | null

  setDevServers: (servers: DevServer[]) => void
  upsertDevServer: (server: DevServer) => void
  removeDevServer: (id: string) => void
  setActiveDevServerId: (id: string | null) => void
  updateDevServerStatus: (id: string, status: DevServer['status'], extra?: Partial<DevServer>) => void
}

// Selectors:
export function useDevServers(): DevServer[]
export function useActiveDevServer(): DevServer | null
export function useConnectedDevServers(): DevServer[]
```

### SshSlice extensions summary

```typescript
// src/renderer/src/store/slices/ssh.ts — EXTENDED:
type SshSlice = {
  // existing...
  sshConnectionStates: Record<string, SshConnectionState>

  // RS-001
  fleetImportStatus: FleetImportStatus | null
  // RS-002
  sshTargetGroups: SshTargetGroup[]
  activeGroupFilter: string | null
  // RS-003
  bulkProvisioningProgress: BulkProvisioningProgress | null
  // RS-004
  bootstrapStatusByServer: Record<string, BootstrapStatus>
  // RS-005
  serverHealthMetrics: Record<string, ServerHealthMetrics>
  fleetAlerts: FleetAlert[]
  lastFleetHealthCheck: number | null
  // RS-006
  currentUser: OrcaUser | null
  accessPolicy: OrcaAccessPolicy | null
}
```

---

## Addendum — login CRs (v4.0) ✅

### AuthSlice (NEW — CR-LOGIN-001)

```typescript
// src/renderer/src/store/slices/auth.ts
import { StateCreator } from 'zustand'
import { AuthUser, AuthState } from '../../auth/auth-types'
import { fetchCurrentUser } from '../../auth/auth-api-client'

export type AuthSlice = {
  auth: AuthState  // 'unknown' | 'unauthenticated' | { status: 'authenticated'; user: AuthUser } | 'error'
  setAuth: (state: AuthState) => void
  clearAuth: () => void
  checkSession: () => Promise<void>  // GET /auth/me → update auth state
}

export const createAuthSlice: StateCreator<AuthSlice> = (set) => ({
  auth: { status: 'unknown' },
  setAuth: (state) => set({ auth: state }),
  clearAuth: () => set({ auth: { status: 'unauthenticated' } }),
  checkSession: async () => {
    const user = await fetchCurrentUser()
    set({ auth: user ? { status: 'authenticated', user } : { status: 'unauthenticated' } })
  }
})
```

### Selectors — hooks/useAuthSession.ts

```typescript
export function useAuthUser(): AuthUser | null
export function useIsAuthenticated(): boolean
export function useAuthStatus(): AuthState['status']
```

### SshSlice — extensions (CR-LOGIN-003)

```typescript
// src/renderer/src/store/slices/ssh.ts — EXTENDED v4.0
export type ProvisioningStatus =
  | { phase: 'idle' }
  | { phase: 'checking' }
  | { phase: 'provisioning'; step: string; progress: number }
  | { phase: 'done'; linuxUsername: string }
  | { phase: 'error'; message: string }

export type SshUserAccount = {
  linuxUsername: string
  provisioned: boolean
  provisioningStatus: ProvisioningStatus
}

// Added to SshSlice:
sshUserAccounts: Map<string, SshUserAccount>
setSshUserAccount: (serverId: string, account: SshUserAccount) => void
updateProvisioningStatus: (serverId: string, status: ProvisioningStatus) => void
```

### Slice registry update (store/index.ts)

```typescript
// ADDED: createAuthSlice
export const useAppStore = create<AppState>()((...a) => ({
  ...createAuthSlice(...a),   // ← NEW v4.0
  ...createSshSlice(...a),    // ← EXTENDED v4.0
  // ... rest unchanged
}))
```

---

## v5.0 — State Management Extensions

### Profile Slice

```typescript
// src/renderer/src/store/slices/profile.ts

interface ProfileState {
  resolvedProfile: ResolvedProfile | null
  companyProfile: OrcaProfile | null
  deptProfile: OrcaProfile | null
  userProfile: OrcaProfile | null
  isLoading: boolean
  lastFetchedAt: number | null  // timestamp
}

interface ProfileActions {
  setResolved: (profile: ResolvedProfile) => void
  setCompany: (profile: OrcaProfile) => void
  setDept: (profile: OrcaProfile) => void
  setUser: (profile: OrcaProfile) => void
  updateUserField: (path: string, value: unknown) => void   // deep-set, reject locked
  reset: () => void
}

// Selectors
const useResolvedProfile = () => useAppStore(s => s.resolvedProfile)
const useProfileSource = (fieldPath: string) =>
  useAppStore(s => s.resolvedProfile?._sources[fieldPath] ?? 'user')
```

### Project Slice

```typescript
// src/renderer/src/store/slices/project.ts

interface ProjectState {
  projects: OrcaProject[]
  activeProjectId: string | null
  members: Record<string, ProjectMember[]>  // projectId → members
  isLoadingProjects: boolean
}

interface ProjectActions {
  setProjects: (projects: OrcaProject[]) => void
  setActiveProject: (id: string | null) => void
  addProject: (project: OrcaProject) => void
  updateProject: (id: string, patch: Partial<OrcaProject>) => void
  removeProject: (id: string) => void
  setMembers: (projectId: string, members: ProjectMember[]) => void
}

// Selectors
const useActiveProject = () => useAppStore(s =>
  s.projects.find(p => p.id === s.activeProjectId) ?? null
)
const useProjectById = (id: string) => useAppStore(s =>
  s.projects.find(p => p.id === id) ?? null
)
```

### AI Provider Slice

```typescript
// src/renderer/src/store/slices/ai-provider.ts

interface AIProviderState {
  accounts: AIProviderAccount[]
  usageByAccount: Record<string, { tokens: number; requests: number; cost: number }>
  isLoadingAccounts: boolean
}

interface AIProviderActions {
  setAccounts: (accounts: AIProviderAccount[]) => void
  updateAccountStatus: (accountId: string, status: AIProviderStatus) => void
  setUsage: (accountId: string, usage: { tokens: number; requests: number; cost: number }) => void
  removeAccount: (accountId: string) => void
}

// Selectors
const useActiveAccounts = () => useAppStore(s =>
  s.accounts.filter(a => a.status === 'active')
)
const useAccountsByServer = (devServerId: string) => useAppStore(s =>
  s.accounts.filter(a => a.devServerId === devServerId)
)
```

### Workflow Slice

```typescript
// src/renderer/src/store/slices/workflow.ts

interface WorkflowState {
  templates: WorkflowTemplate[]
  executions: WorkflowExecution[]
  stepStatuses: Record<string, Record<string, StepStatus>>  // execId → stepId → status
  streamingOutput: Record<string, string[]>                  // execId → output lines
  isLoadingTemplates: boolean
}

interface WorkflowActions {
  setTemplates: (templates: WorkflowTemplate[]) => void
  addExecution: (execution: WorkflowExecution) => void
  updateExecutionStatus: (execId: string, status: WorkflowStatus) => void
  updateStepStatus: (execId: string, stepId: string, status: StepStatus) => void
  appendStreamingOutput: (execId: string, line: string) => void
  cancelExecution: (execId: string) => void
}
```

### Task Slice

```typescript
// src/renderer/src/store/slices/task.ts

interface TaskState {
  tasksByProject: Record<string, OrcaTask[]>  // projectId → tasks (flat list)
  activeTaskId: string | null
  expandedNodes: Set<string>                   // task IDs expanded in tree view
  selectedTaskIds: Set<string>                 // multi-select
  dagView: boolean                             // toggle tree vs DAG view
  filterStatus: string[]                       // filter by status
  filterAssignee: string | null
}

interface TaskActions {
  setTasks: (projectId: string, tasks: OrcaTask[]) => void
  addTask: (projectId: string, task: OrcaTask) => void
  updateTask: (taskId: string, patch: Partial<OrcaTask>) => void
  removeTask: (taskId: string) => void
  setActiveTask: (id: string | null) => void
  toggleExpanded: (taskId: string) => void
  toggleDagView: () => void
  setFilter: (filter: Partial<{ status: string[]; assignee: string | null }>) => void
}

// Selectors
const useProjectTasks = (projectId: string) => useAppStore(s =>
  s.tasksByProject[projectId] ?? []
)
const useTaskById = (taskId: string) => useAppStore(s => {
  for (const tasks of Object.values(s.tasksByProject)) {
    const found = tasks.find(t => t.id === taskId)
    if (found) return found
  }
  return null
})
const useFilteredTasks = (projectId: string) => useAppStore(s => {
  const all = s.tasksByProject[projectId] ?? []
  const { filterStatus, filterAssignee } = s
  return all.filter(t => {
    if (filterStatus.length > 0 && !filterStatus.includes(t.status)) return false
    if (filterAssignee && t.assigneeId !== filterAssignee) return false
    return true
  })
})
```

### WorkspaceContext — Event Bus (client-side)

```typescript
// Zustand không handle cross-panel events — WorkspaceContext dùng micro-emitter:

// In WorkspaceContext.tsx:
const eventHandlers = useRef(new Map<string, Set<(e: WorkspaceEvent) => void>>())

const emit = useCallback((event: WorkspaceEvent) => {
  eventHandlers.current.get(event.type)?.forEach(h => h(event))
}, [])

const on = useCallback((type: WorkspaceEvent['type'], handler: (e: WorkspaceEvent) => void) => {
  if (!eventHandlers.current.has(type)) eventHandlers.current.set(type, new Set())
  eventHandlers.current.get(type)!.add(handler)
  return () => eventHandlers.current.get(type)?.delete(handler)  // unsubscribe
}, [])

// Usage in panels:
// ExplorerPanel: on('agent.complete', refreshFileTree)
// GitPanel: on('agent.complete', refreshGitStatus); on('git.commit', refreshGitStatus)
// TasksPanel: on('git.commit', ({ message }) => checkTaskRefs(message))
```

---

## Addendum — Dev Server as a first-class Execution Host (2026-08-03) ✅ IMPLEMENTED

> Shared types, không phải store-owned — nhưng `repos` slice và các selector filter theo host phụ thuộc trực tiếp vào đây.

`src/shared/execution-host.ts`:

```typescript
export type ExecutionHostKind = 'local' | 'ssh' | 'runtime' | 'devServer'
export type ExecutionHostId =
  | 'local'
  | `ssh:${string}`
  | `runtime:${string}`
  | `devServer:${string}`   // NEW

export function toDevServerExecutionHostId(devServerId: string): `devServer:${string}`

// Precedence khi resolve host của một repo:
export function getRepoExecutionHostId(repo: Repo): ExecutionHostId {
  // executionHostId (explicit) → connectionId (SSH) → devServerId → 'local'
}

// Bare, unprefixed connection key — dùng ở những chỗ cần "opaque connection id
// của bất kỳ transport nào đang backing repo này" thay vì typed/prefixed id
// (vd: connection-owner-resolution.ts — xem TDD-FE-04):
export function getRepoProviderConnectionKey(
  repo: Pick<Repo, 'connectionId' | 'devServerId'>
): string | null {
  return repo.connectionId ?? repo.devServerId ?? null
}
```

`buildExecutionHostRegistry()` (`src/shared/execution-host-registry.ts`) nhận thêm `devServers?: readonly DevServerSummary[]` và có `addDevServerHost()` map `DevServerStatus` ('connected'/'connecting'/'disconnected'/'error') → `ExecutionHostHealth`, cùng cách SSH connection state đã map từ trước.

**Gotcha khi thêm một `ExecutionHostKind` mới:** ngoài `execution-host.ts`, còn 2 chỗ union hẹp dễ quên phải widen theo, nếu không sẽ compile error kiểu `Type 'devServer:${string}' is not assignable...`:
- `WorkspaceHostScope` (`src/shared/types.ts`)
- `Repo['executionHostId']`'s type

Trước session này, Dev Servers (`Repo.devServerId`) hoàn toàn tách biệt khỏi `ExecutionHostKind` — sidebar, Available Hosts panel, và mọi routing file/git/terminal chỉ biết `connectionId`/`executionHostId` (SSH). Giờ Dev Server là một host kind thật, đi qua cùng pipeline như local/ssh/runtime — xem TDD-FE-09 §11 và TDD-FE-04 §Connection Resolution.
