# TDD-FE-12: Project Workspace Shell

**Document:** TDD-FE-12 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Project Workspace — ProjectSwitcher, WorkspaceLayout, panel orchestration
**Feature:** F34, F38
**ADR:** ADR-011
**HLD Ref:** C3.12, C4.10
**Backend TDD:** TDD-15, TDD-19
**Source files (to create):**
- `src/renderer/src/context/WorkspaceContext.tsx`
- `src/renderer/src/components/workspace/WorkspaceLayout.tsx`
- `src/renderer/src/components/project/ProjectSwitcher.tsx`
- `src/renderer/src/components/project/ProjectSettings.tsx`
- `src/renderer/src/components/project/MemberManager.tsx`
- `src/renderer/src/hooks/useWorkspace.ts`

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. WorkspaceContext

```typescript
// src/renderer/src/context/WorkspaceContext.tsx
// (Full implementation documented in TDD-19 backend — this covers UI integration)

// WorkspaceProvider wraps the main App layout:
// <WorkspaceProvider>
//   <ProjectSwitcher />
//   <WorkspaceLayout />
// </WorkspaceProvider>

// useWorkspace() hook exposes:
// - project, devServer, isOffline, isInitializing
// - gitStatus, worktrees, currentWorktree, fileTree
// - resolvedProfile, activeAgentSessionId
// - switchProject(), setCurrentWorktree(), refreshGitStatus(), refreshFileTree()
// - emit(), on()   ← micro event bus
```

---

## 2. ProjectSwitcher Component

```typescript
// src/renderer/src/components/project/ProjectSwitcher.tsx

// Layout:
// ┌─────────────────────────────────────────────────────────────────┐
// │ [🔷 MyApp Backend ▼] [+ New Project] [⚙ Settings]             │
// └─────────────────────────────────────────────────────────────────┘
//
// Dropdown:
// ┌──────────────────────────────────┐
// │ 🔍 Search projects...            │
// │ ──────────────────────────────── │
// │ ● MyApp Backend     linux-srv1   │
// │ ● Frontend Web      linux-srv1   │
// │ ○ API Gateway       linux-srv2   │ ← ○ = different server
// │ ──────────────────────────────── │
// │ + Create New Project             │
// └──────────────────────────────────┘

export function ProjectSwitcher() {
  const { project, switchProject, isInitializing } = useWorkspace()
  const projects = useAppStore(s => s.projects)
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')

  const filtered = projects.filter(p =>
    p.name.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="outline" className="project-switcher-trigger" disabled={isInitializing}>
          {isInitializing ? (
            <Loader2 className="animate-spin" size={16} />
          ) : (
            <div className="project-icon">{project?.name[0]?.toUpperCase()}</div>
          )}
          <span>{project?.name ?? 'Select Project'}</span>
          <ChevronsUpDown size={14} className="ml-auto opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-0" align="start">
        <Command>
          <CommandInput placeholder="Search projects..." value={search} onValueChange={setSearch} />
          <CommandList>
            <CommandEmpty>No projects found</CommandEmpty>
            <CommandGroup>
              {filtered.map(p => (
                <CommandItem
                  key={p.id}
                  value={p.id}
                  onSelect={() => { switchProject(p.id); setOpen(false) }}
                >
                  <Check className={cn('mr-2', p.id === project?.id ? 'opacity-100' : 'opacity-0')} />
                  <span>{p.name}</span>
                  <span className="ml-auto text-xs text-muted-foreground">{p.devServerId}</span>
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
            <CommandItem onSelect={() => openCreateProjectDialog()}>
              <Plus size={14} className="mr-2" /> Create New Project
            </CommandItem>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
```

---

## 3. WorkspaceLayout Component

```typescript
// src/renderer/src/components/workspace/WorkspaceLayout.tsx

// Layout: 3-panel horizontal split (resizable)
// ┌──────────────────────────────────────────────────────────────────────┐
// │ [Explorer] [Git] [Tasks] [Workflows]           ← Tab bar            │
// ├────────────────┬─────────────────────────────┬───────────────────────┤
// │                │                             │                       │
// │  LEFT PANEL    │    CENTER PANEL             │  RIGHT PANEL          │
// │  (20% min)     │    (flex grow)              │  (30% collapsible)    │
// │                │                             │                       │
// │  FileExplorer  │    Active tab content:      │  AgentPanel /         │
// │  (always)      │    - GitPanel (git tab)     │  TaskDetail /         │
// │                │    - TaskGraph (task tab)   │  WorkflowMonitor      │
// │                │    - WorkflowBuilder        │                       │
// │                │                             │                       │
// ├────────────────┴─────────────────────────────┴───────────────────────┤
// │  TerminalPanel (collapsible, bottom)                                  │
// └──────────────────────────────────────────────────────────────────────┘

type WorkspaceTab = 'git' | 'tasks' | 'workflows' | 'agent'

export function WorkspaceLayout() {
  const { project, isOffline, isInitializing } = useWorkspace()
  const [activeTab, setActiveTab] = useState<WorkspaceTab>('git')
  const [rightPanelVisible, setRightPanelVisible] = useState(true)
  const [terminalVisible, setTerminalVisible] = useState(false)

  if (!project) return <NoProjectSelected />
  if (isInitializing) return <WorkspaceSkeletonLoader />

  return (
    <div className="workspace-layout">
      {isOffline && <OfflineBanner message="Dev server unreachable — read-only mode" />}

      <WorkspaceTabBar activeTab={activeTab} onTabChange={setActiveTab} />

      <ResizablePanelGroup direction="horizontal" className="workspace-panels">
        {/* Left: always-visible file explorer */}
        <ResizablePanel defaultSize={20} minSize={15} maxSize={35}>
          <FileExplorerPanel />
        </ResizablePanel>

        <ResizableHandle />

        {/* Center: tab content */}
        <ResizablePanel defaultSize={rightPanelVisible ? 50 : 80}>
          {activeTab === 'git' && <GitPanel />}
          {activeTab === 'tasks' && <TaskGraphPanel />}
          {activeTab === 'workflows' && <WorkflowMonitorPanel />}
          {activeTab === 'agent' && <AgentPanel />}
        </ResizablePanel>

        {/* Right: agent/task detail (collapsible) */}
        {rightPanelVisible && (
          <>
            <ResizableHandle />
            <ResizablePanel defaultSize={30} minSize={20}>
              <RightSidebar activeTab={activeTab} />
            </ResizablePanel>
          </>
        )}
      </ResizablePanelGroup>

      {/* Bottom: terminal (collapsible) */}
      {terminalVisible && (
        <ResizablePanel defaultSize={25} className="workspace-terminal">
          <TerminalPanel />
        </ResizablePanel>
      )}

      <WorkspaceStatusBar
        isOffline={isOffline}
        onToggleTerminal={() => setTerminalVisible(v => !v)}
        onToggleRightPanel={() => setRightPanelVisible(v => !v)}
      />
    </div>
  )
}
```

---

## 4. ProjectSettings Dialog

```typescript
// src/renderer/src/components/project/ProjectSettings.tsx

// Tabs:
// [General] [Members] [Profile Override]
//
// General: name, description, repoPath, defaultBranch, visibility
// Members: add/remove/role-change (owner/member/viewer)
// Profile Override: ProfileEditor scope="project" scopeId={project.id}

export function ProjectSettings({ projectId }: { projectId: string }) {
  const project = useProjectById(projectId)
  const [activeTab, setActiveTab] = useState('general')

  return (
    <Dialog>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Project Settings — {project?.name}</DialogTitle>
        </DialogHeader>
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList>
            <TabsTrigger value="general">General</TabsTrigger>
            <TabsTrigger value="members">Members</TabsTrigger>
          </TabsList>
          <TabsContent value="general">
            <ProjectGeneralForm project={project} />
          </TabsContent>
          <TabsContent value="members">
            <MemberManager projectId={projectId} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
```

---

## 5. OfflineBanner + WorkspaceSkeletonLoader

```typescript
// OfflineBanner: shown at top when devServer unreachable
export function OfflineBanner({ message }: { message: string }) {
  return (
    <div className="offline-banner bg-yellow-50 border-b border-yellow-200 px-4 py-2 flex items-center gap-2">
      <WifiOff size={14} className="text-yellow-600" />
      <span className="text-sm text-yellow-800">{message}</span>
      <Button size="sm" variant="outline" onClick={retry} className="ml-auto">
        Retry Connection
      </Button>
    </div>
  )
}

// WorkspaceSkeletonLoader: shown during project switch
export function WorkspaceSkeletonLoader() {
  return (
    <div className="workspace-skeleton p-4 space-y-3">
      <Skeleton className="h-6 w-40" />   {/* tab bar */}
      <div className="flex gap-3">
        <Skeleton className="h-96 w-1/4" />  {/* explorer */}
        <Skeleton className="h-96 flex-1" /> {/* main panel */}
        <Skeleton className="h-96 w-1/3" /> {/* right panel */}
      </div>
    </div>
  )
}
```

---

## 6. Test Coverage

```
src/renderer/src/components/project/__tests__/
├── ProjectSwitcher.test.tsx
│   ├── renders current project name
│   ├── opens dropdown with project list
│   ├── calls switchProject on item select
│   ├── shows loading spinner during initialization
│   └── filters by search text
├── WorkspaceLayout.test.tsx
│   ├── renders NoProjectSelected when no project
│   ├── renders WorkspaceSkeletonLoader when initializing
│   ├── renders OfflineBanner when isOffline
│   ├── shows GitPanel when git tab active
│   ├── shows TaskGraphPanel when tasks tab active
│   └── toggles terminal panel visibility
src/renderer/src/context/__tests__/
├── WorkspaceContext.test.tsx
│   ├── switchProject loads data and sets state
│   ├── switchProject: DEV_SERVER_UNREACHABLE → isOffline=true
│   ├── refreshGitStatus updates gitStatus
│   ├── emit + on: handler receives event
│   ├── on returns cleanup function that removes handler
│   └── agent.complete event triggers refreshGitStatus listener
```

**Target:** ≥ 25 tests

---

## Addendum: HLD Cross-References (v5.0 — 2026-07-30)

> **Nguồn:** [HLD C3.12](../../../docs/hld/v1/C3-components.md), [HLD C4.10](../../../docs/hld/v1/C4-code.md), [web-server-architecture.md §10.1–10.2](../../../docs/hld/web-server-architecture.md)

### WorkspaceContext — Full Interface (từ HLD C4.10)

```typescript
interface WorkspaceContextValue {
  // Project state
  project: OrcaProject | null
  devServer: SshHost | null
  isConnected: boolean
  isOffline: boolean

  // Connection — proxied via RPC, managed by backend
  relay: DevServerRelayBridge | null

  // Worktree state
  currentWorktree: Worktree | null
  availableWorktrees: Worktree[]
  setCurrentWorktree: (wt: Worktree) => void

  // Git state
  gitStatus: GitStatus | null
  refreshGitStatus: () => Promise<void>

  // Profile (deep-merged 3 tiers, resolved by backend)
  resolvedProfile: ResolvedProfile | null

  // Agent
  activeAgentSessionId: string | null
  setActiveAgentSession: (id: string | null) => void

  // Cross-panel event bus
  emit: (event: WorkspaceEvent) => void
  on: (event: string, handler: Function) => () => void  // returns unsubscribe

  // Navigation
  switchProject: (projectId: string) => Promise<void>
}

type WorkspaceEvent =
  | { type: 'agent.complete'; filesChanged: number }
  | { type: 'git.commit'; hash: string; message: string }
  | { type: 'git.push'; branch: string }
  | { type: 'worktree.switched'; path: string; branch: string }
  | { type: 'workflow.step.complete'; stepId: string; executionId: string }
```

### switchProject() — Exact Data Flow (từ HLD C4.10)

```
switchProject('proj-abc')
    │
    ├── 1. RPC: projects.get('proj-abc')
    │         → { devServerId, repoPath, name, memberRole }
    │
    ├── 2. Backend: FleetHealthMonitor.getCached(devServerId)
    │         → 'healthy' → proceed
    │         → 'unreachable' → isOffline=true, show offline banner
    │
    ├── 3. Backend: DevServerRelayBridge.connect(devServerId)
    │         (relay established — relay type: relay-ssh | relay-websocket | direct-ws)
    │
    ├── 4. Promise.all([
    │         RPC: git.status({ cwd: repoPath }),
    │         RPC: git.worktree.list({ repoPath }),
    │         RPC: fs.readDir({ path: repoPath, depth: 2 }),
    │         RPC: profile.getEffective(),
    │         RPC: workflows.getActiveExecutions('proj-abc'),
    │   ])
    │
    ├── 5. Dispatch WorkspaceContext state update:
    │         { project, devServer, relay, resolvedProfile,
    │           gitStatus, availableWorktrees, fileTree,
    │           isConnected: true, isOffline: false }
    │
    ├── 6. Start git status poll interval (every 5s)
    │
    └── 7. UI renders:
              ExplorerPanel ← fileTree ready
              GitPanel ← gitStatus ready
              AgentPanel ← resolvedProfile ready
              WorkflowBuilder ← active executions ready
```

### ServerStatusBar — Status Model (từ HLD C3.12)

```typescript
// src/renderer/src/components/workspace/ServerStatusBar.tsx
type ServerStatus = 'healthy' | 'degraded' | 'unhealthy' | 'unreachable'

// Hiển thị:
// 🟢 healthy:     SSH ✔, relay ✔, CPU<80%, RAM<85%
// 🟡 degraded:    relay ✔ nhưng CPU>80% hoặc RAM>85%
// 🟠 unhealthy:   SSH ✔ nhưng relay ✗
// 🔴 unreachable: SSH connect timeout/fail

// Poll: FleetHealthMonitor backend poll 60s → push event 'fleet.health.updated'
// Frontend subscribed qua: on('fleet.health.updated', refreshStatus)
```

### WorkspaceLayout — Sidebar Tab Structure (từ HLD C3.12)

```
WorkspaceLayout
├── Left Sidebar (tab bar)
│   ├── 📁 Explorer tab → ExplorerPanel
│   ├── 🔀 Git tab     → GitPanel
│   ├── 🤖 Agent tab   → AgentPanel
│   ├── ⚡ Workflows tab → WorkflowBuilder
│   ├── 📋 Tasks tab    → TaskGraph
│   └── 📟 Terminal tab → (open terminal panel)
│
├── Main content area (selected tab content)
│   └── FileViewer tabs (Monaco read-only)
│
└── Bottom panel (resizable)
    └── WorkspaceTerminal (PTY sessions)

// Note: tabs lazily mount — ExplorerPanel không unmount khi switch tab
// (preserves expanded state, scroll position)
```

### Project RBAC (từ HLD RBAC)

```typescript
// project member roles:
type ProjectRole = 'developer' | 'lead' | 'admin'

// Quyền theo role:
// developer: view project, read files, spawn agent, create tasks
// lead:      + manage members, update project, create workflows
// admin:     + delete project, change dev server binding
```
