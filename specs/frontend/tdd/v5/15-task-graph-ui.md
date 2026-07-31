# TDD-FE-15: Task Graph UI

**Document:** TDD-FE-15 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Task UI — DAG visualization, task card, detail panel, AI decompose, agent execution
**Feature:** F37
**ADR:** ADR-010
**HLD Ref:** C3.11b
**Backend TDD:** TDD-18
**Source files (to create):**
- `src/renderer/src/components/task/TaskGraph.tsx`
- `src/renderer/src/components/task/TaskCard.tsx`
- `src/renderer/src/components/task/TaskDetail.tsx`
- `src/renderer/src/components/task/TaskAIDecompose.tsx`
- `src/renderer/src/components/task/TaskPromptEditor.tsx`
- `src/renderer/src/components/task/TaskStatusBadge.tsx`
- `src/renderer/src/hooks/useTask.ts`

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. TaskGraph Component

```typescript
// src/renderer/src/components/task/TaskGraph.tsx
// Dual view: Tree (default) / DAG (toggle)

// Tree View Layout:
// ┌──────────────────────────────────────────────────────────────────────┐
// │ Tasks               [+ New Task] [Tree ▼] [Filter ▼] [🔍 Search]    │
// ├──────────────────────────────────────────────────────────────────────┤
// │ ▼ [EPIC] Auth System                       🔴 Critical  0% [→ ...]  │
// │   ▼ [STORY] Login Flow                     🟠 High     40% [→ ...]  │
// │     ✅ [TASK] Setup DB schema              🟡 Med      done [→ ...]  │
// │     🔄 [TASK] Implement JWT auth           🟠 High     40% [→ ...]  │
// │     ⏳ [TASK] Add SSO endpoints            🟢 Low     todo [→ ...]  │
// │   ▷ [STORY] Registration Flow              🟠 High      0%           │
// │                                                                      │
// └──────────────────────────────────────────────────────────────────────┘
//
// DAG View (toggle) → React Flow with dependency edges

export function TaskGraph({ projectId }: { projectId: string }) {
  const { dagView, filteredTasks, expandedNodes, toggleExpanded, setActiveTask } = useTasks(projectId)
  const [viewMode, setViewMode] = useState<'tree' | 'dag'>('tree')

  return (
    <div className="task-graph">
      <TaskGraphHeader
        projectId={projectId}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
      />

      {viewMode === 'tree' ? (
        <TaskTreeView
          tasks={filteredTasks}
          expandedNodes={expandedNodes}
          onToggle={toggleExpanded}
          onSelect={setActiveTask}
        />
      ) : (
        <TaskDAGView tasks={filteredTasks} onSelect={setActiveTask} />
      )}
    </div>
  )
}
```

---

## 2. TaskTreeView — Recursive

```typescript
// src/renderer/src/components/task/TaskTreeView.tsx

interface TaskTreeViewProps {
  tasks: OrcaTask[]       // flat list — render tree from parentId relationships
  expandedNodes: Set<string>
  onToggle: (id: string) => void
  onSelect: (id: string) => void
  depth?: number
}

export function TaskTreeView({ tasks, expandedNodes, onToggle, onSelect, depth = 0 }: TaskTreeViewProps) {
  const roots = tasks.filter(t => t.parentId === null)

  return (
    <div className="task-tree">
      {roots.map(task => (
        <TaskTreeNode
          key={task.id}
          task={task}
          allTasks={tasks}
          expanded={expandedNodes.has(task.id)}
          onToggle={onToggle}
          onSelect={onSelect}
          depth={depth}
        />
      ))}
    </div>
  )
}

function TaskTreeNode({ task, allTasks, expanded, onToggle, onSelect, depth }: TaskTreeNodeProps) {
  const children = allTasks.filter(t => t.parentId === task.id)
  const hasChildren = children.length > 0

  return (
    <div className="task-tree-node" style={{ paddingLeft: depth * 20 }}>
      <TaskCard
        task={task}
        hasChildren={hasChildren}
        expanded={expanded}
        onExpand={() => onToggle(task.id)}
        onSelect={() => onSelect(task.id)}
      />
      {expanded && hasChildren && (
        <TaskTreeView
          tasks={allTasks}
          expandedNodes={expandedNodes}
          onToggle={onToggle}
          onSelect={onSelect}
          depth={depth + 1}
        />
      )}
    </div>
  )
}
```

---

## 3. TaskCard Component

```typescript
// src/renderer/src/components/task/TaskCard.tsx

// Layout:
// ┌─────────────────────────────────────────────────────────────────────┐
// │ ▼ [TASK] Setup DB schema               🟡 Medium  40%  @user [→] │
// └─────────────────────────────────────────────────────────────────────┘

export function TaskCard({ task, hasChildren, expanded, onExpand, onSelect }: TaskCardProps) {
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') onSelect()
  }

  return (
    <div
      className={cn(
        'task-card flex items-center gap-2 py-1.5 px-2 rounded-md cursor-pointer',
        'hover:bg-accent transition-colors group',
        task.status === 'done' && 'opacity-60'
      )}
      onClick={onSelect}
      onKeyDown={handleKeyDown}
      tabIndex={0}
      role="button"
      aria-label={`Task: ${task.title}`}
    >
      {/* Expand toggle */}
      {hasChildren ? (
        <Button variant="ghost" size="icon" className="h-4 w-4" onClick={e => { e.stopPropagation(); onExpand() }}>
          {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </Button>
      ) : (
        <div className="w-4" />
      )}

      {/* Type badge */}
      <TaskTypeBadge type={task.type} />

      {/* Title */}
      <span className={cn('flex-1 text-sm truncate', task.status === 'done' && 'line-through')}>
        {task.title}
      </span>

      {/* Priority */}
      <PriorityIcon priority={task.priority} />

      {/* Progress bar (for epics/stories) */}
      {['epic', 'story'].includes(task.type) && (
        <Progress value={task.progressPercent} className="w-16 h-1" />
      )}

      {/* Status */}
      <TaskStatusBadge status={task.status} />

      {/* Assignee */}
      {task.assigneeId && <UserAvatar userId={task.assigneeId} size={16} />}

      {/* Actions (on hover) */}
      <div className="hidden group-hover:flex gap-1">
        <Button variant="ghost" size="icon" className="h-5 w-5">
          <MoreHorizontal size={12} />
        </Button>
      </div>
    </div>
  )
}
```

---

## 4. TaskDetail — Right Panel

```typescript
// src/renderer/src/components/task/TaskDetail.tsx

// Layout:
// ┌───────────────────────────────────────────────────────────┐
// │ [TASK] Implement JWT auth     [🔄 In Progress] [Edit]     │
// │ Project: MyApp Backend  │  Priority: 🟠 High              │
// │ Assignee: @binh         │  Reporter: @admin               │
// │ Labels: [auth] [backend]│  Est: 3h / Actual: 1.5h         │
// ├───────────────────────────────────────────────────────────┤
// │ Description                                               │
// │ Implement JWT token generation and validation middleware   │
// │                                                           │
// │ Dependencies:                                             │
// │ ← Blocked by: [Setup DB schema ✅]                       │
// │ → Blocks:     [Add SSO endpoints ⏳]                     │
// │                                                           │
// │ AI Context                                                │
// │ [Use jose library, validate exp + iss claims]             │
// │                                                           │
// │ [🤖 Decompose with AI] [▶ Execute with Agent]             │
// │                                                           │
// ├───────────────────────────────────────────────────────────┤
// │ Comments & Activity                                       │
// │ @agent 2m: Generated 3 files: jwt.ts, middleware.ts...    │
// │ @binh 5m: Added JWT_SECRET to env vars                    │
// └───────────────────────────────────────────────────────────┘

export function TaskDetail({ taskId }: { taskId: string }) {
  const task = useTaskById(taskId)
  const { decomposeWithAI, executeWithAgent } = useTaskActions(taskId)

  if (!task) return null

  return (
    <div className="task-detail overflow-y-auto">
      <TaskDetailHeader task={task} />
      <TaskMetaGrid task={task} />
      <TaskDescription task={task} />
      <TaskDependencies taskId={taskId} />
      <TaskAIContext task={task} />
      <TaskActions task={task} onDecompose={decomposeWithAI} onExecute={executeWithAgent} />
      <TaskComments taskId={taskId} />
    </div>
  )
}
```

---

## 5. TaskAIDecompose Component

```typescript
// src/renderer/src/components/task/TaskAIDecompose.tsx

export function TaskAIDecompose({ taskId, onClose }: { taskId: string; onClose: () => void }) {
  const [isDecomposing, setIsDecomposing] = useState(false)
  const [suggestions, setSuggestions] = useState<SubtaskSuggestion[] | null>(null)
  const [applying, setApplying] = useState(false)

  const decompose = async () => {
    setIsDecomposing(true)
    try {
      const result = await rpc.call('task.decomposeWithAI', { taskId }) as SubtaskSuggestion[]
      setSuggestions(result)
    } finally {
      setIsDecomposing(false)
    }
  }

  const applyAll = async () => {
    if (!suggestions) return
    setApplying(true)
    await rpc.call('task.applyDecomposition', { taskId, suggestions })
    // Refresh tasks in store
    const updatedTasks = await rpc.call('task.getSubtree', { taskId }) as OrcaTask[]
    useAppStore.getState().setTasks(/* projectId */, updatedTasks)
    onClose()
  }

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>AI Task Decomposition</DialogTitle>
          <DialogDescription>
            AI will analyze the task and suggest subtasks with dependencies.
          </DialogDescription>
        </DialogHeader>

        {!suggestions ? (
          <Button onClick={decompose} disabled={isDecomposing}>
            {isDecomposing ? <><Loader2 className="animate-spin" /> Analyzing...</> : '🤖 Decompose with AI'}
          </Button>
        ) : (
          <div className="suggestions-list space-y-2">
            {suggestions.map((s, i) => (
              <div key={i} className="suggestion-item border rounded p-3">
                <div className="flex items-center gap-2">
                  <TaskTypeBadge type={s.type} />
                  <span className="font-medium text-sm">{s.title}</span>
                  {s.estimatedHours && (
                    <span className="ml-auto text-xs text-muted-foreground">{s.estimatedHours}h</span>
                  )}
                </div>
                {s.description && <p className="text-xs text-muted-foreground mt-1">{s.description}</p>}
                {s.dependsOn && s.dependsOn.length > 0 && (
                  <p className="text-xs text-blue-600 mt-1">
                    Depends on: {s.dependsOn.map(d => `Step ${d + 1}`).join(', ')}
                  </p>
                )}
              </div>
            ))}
            <DialogFooter>
              <Button variant="outline" onClick={() => setSuggestions(null)}>Regenerate</Button>
              <Button onClick={applyAll} disabled={applying}>
                {applying ? <Loader2 className="animate-spin" /> : 'Apply All'}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

---

## 6. TaskStatusBadge + PriorityIcon

```typescript
const STATUS_CONFIG = {
  backlog:     { label: 'Backlog',     icon: <Circle size={12} />,         color: 'text-gray-500' },
  todo:        { label: 'Todo',        icon: <CircleDot size={12} />,      color: 'text-blue-500' },
  in_progress: { label: 'In Progress', icon: <Loader2 size={12} />,        color: 'text-yellow-600' },
  blocked:     { label: 'Blocked',     icon: <OctagonX size={12} />,       color: 'text-red-600' },
  review:      { label: 'Review',      icon: <Eye size={12} />,            color: 'text-purple-600' },
  done:        { label: 'Done',        icon: <CheckCircle2 size={12} />,   color: 'text-green-600' },
  cancelled:   { label: 'Cancelled',   icon: <XCircle size={12} />,        color: 'text-gray-400' },
}

export function TaskStatusBadge({ status }: { status: OrcaTask['status'] }) {
  const { label, icon, color } = STATUS_CONFIG[status]
  return (
    <span className={cn('flex items-center gap-1 text-xs font-medium', color)}>
      {icon} {label}
    </span>
  )
}

const PRIORITY_CONFIG = {
  critical: { icon: '🔴', label: 'Critical' },
  high:     { icon: '🟠', label: 'High' },
  medium:   { icon: '🟡', label: 'Medium' },
  low:      { icon: '🟢', label: 'Low' },
}

export function PriorityIcon({ priority }: { priority: OrcaTask['priority'] }) {
  const { icon, label } = PRIORITY_CONFIG[priority]
  return <span title={label} className="text-sm">{icon}</span>
}
```

---

## 7. Test Coverage

```
src/renderer/src/components/task/__tests__/
├── TaskCard.test.tsx
│   ├── renders task title and type badge
│   ├── shows expand toggle when hasChildren
│   ├── done status → line-through class
│   ├── keyboard: Enter triggers onSelect
│   └── hover → actions visible
├── TaskTreeView.test.tsx
│   ├── renders root tasks (parentId=null)
│   ├── expand/collapse children on toggle
│   ├── nested 3-level tree correctly indented
│   └── selection calls setActiveTask
├── TaskDetail.test.tsx
│   ├── renders task metadata
│   ├── shows dependencies list
│   └── Decompose button opens TaskAIDecompose dialog
├── TaskAIDecompose.test.tsx
│   ├── clicking Decompose calls rpc.call('task.decomposeWithAI')
│   ├── shows suggestions with title and estimated hours
│   ├── Apply All calls rpc.call('task.applyDecomposition')
│   └── Regenerate resets suggestions
├── TaskStatusBadge.test.tsx
│   ├── in_progress → yellow Loader2 icon
│   ├── done → green CheckCircle2
│   └── blocked → red OctagonX
└── hooks/__tests__/useTask.test.ts
    ├── fetches tasks on mount
    ├── filterStatus filters correctly
    ├── filterAssignee filters correctly
    └── setActiveTask updates store
```

**Target:** ≥ 30 tests

---

## Addendum: HLD Cross-References (v5.0 — 2026-07-30)

> **Nguồn:** [HLD C3.11b](../../../docs/hld/v1/C3-components.md), [HLD C4.9](../../../docs/hld/v1/C4-code.md), [web-server-architecture.md §10.5](../../../docs/hld/web-server-architecture.md)

### OrcaTask — Full Type (từ HLD C4.9)

```typescript
interface OrcaTask {
  id: string              // 'TG-xxx'
  projectId: string
  parentId?: string       // null nếu là root epic
  title: string
  description?: string    // Markdown
  type: 'epic' | 'story' | 'task' | 'bug' | 'spike'
  status: 'todo' | 'in_progress' | 'review' | 'done' | 'blocked' | 'cancelled'
  priority: 'critical' | 'high' | 'medium' | 'low'
  assigneeId?: string
  reporterId: string
  estimatedHours?: number
  actualHours?: number
  dueDate?: Date
  promptTemplate?: string  // Agent prompt khi runAgent
  aiContext?: string       // Extra context cho AI
  agentSessionId?: string  // Linked PTY session
  worktreeId?: string      // Linked worktree
  createdAt: Date
  updatedAt: Date
  completedAt?: Date
}
```

### Task DAG — Dependency Model (từ HLD C4.9)

```typescript
// orca_task_edges:
interface TaskEdge {
  fromTaskId: string    // phụ thuộc
  toTaskId: string      // phụ thuộc vào
  type: 'blocks' | 'is-blocked-by' | 'relates-to'
}

// Auto-blocking: nếu dependency chưa 'done' → task blocked
// Cycle detection: BFS trước mỗi addEdge()
// Critical path: longest path (weighted by estimatedHours)
```

### Task Grant Resolution (từ HLD C4.9)

```typescript
// Chỉ user có grant mới thấy task trong danh sách
// Grant levels:
type TaskGrantLevel = 'view' | 'comment' | 'edit' | 'execute' | 'manage'

// Priority: owner > admin > user > team > company
// apply_tree: grant propagates đến tất cả subtasks (BFS)
// Expiry: grant.expires_at — frontend check khi render

// Frontend behavior:
// - Không có grant → task KHÔNG hiện trong list (backend filter)
// - execute grant → AgentPanel "Run Agent" button enabled
// - manage grant → edit/delete/grant buttons visible
```

### tasks.runAgent() — Full Flow (từ HLD C4.9)

```
User click "Run Agent" trong AgentPanel/TaskDetail
    │
    ├── RPC: tasks.runAgent(taskId, worktreeId?)
    │
    ├── Backend TaskGrantService.hasTaskAccess(userId, taskId, 'execute')
    ├── Backend FleetHealthMonitor.check(devServerId) → healthy
    ├── Backend AIProviderResolver.resolve() → accountId
    ├── Backend ProfileResolver.resolve(userId) → profile
    ├── Backend TaskAgentExecutor.buildPreamble(task, project, user, deps)
    │         → "# Task Context\nTask: <title>\nDependencies: ..."
    ├── Backend ProfileAwareAgentSpawner.spawn({
    │         cwd: worktreePath,
    │         env: { ORCA_TASK_ID, ...profile.shell.envVars },
    │         initFile: preamble + task.promptTemplate
    │   })
    │
    └── → PTY session created → events: pty:data → AgentPanel stream
        → Backend UPDATE orca_tasks SET agent_session_id, status='in_progress'
        → Event push: 'task.status.updated' { taskId, status: 'in_progress' }
        → TaskStatusBadge: render 🔄 in_progress
```

### AI Task Decompose — Response Shape (từ HLD)

```typescript
// tasks.aiPlan(taskId) response:
interface AITaskPlan {
  suggestedSubtasks: Array<{
    title: string
    description: string
    type: 'task' | 'spike'
    estimatedHours: number
    dependsOn: string[]    // indexes into suggestedSubtasks
  }>
  dependencyGraph: Array<[number, number]>  // [from, to] indices
  criticalPath: number[]                     // task indices
  totalEstimate: number
  rationale: string
}

// Frontend TaskAIDecompose component:
// 1. Call tasks.aiPlan(taskId) → hiển thị plan trong modal
// 2. User review: check/uncheck subtasks, edit titles
// 3. Click "Approve & Create" → RPC: tasks.createSubtasks(taskId, approved[])
```

### TaskStatusBadge — Visual Mapping

| Status | Icon | Color | Mô tả |
|--------|------|-------|-------|
| `todo` | ⏳ | Grey | Chưa bắt đầu |
| `in_progress` | 🔄 | Blue | Đang làm |
| `review` | 👁️ | Purple | Cần review |
| `done` | ✅ | Green | Hoàn thành |
| `blocked` | 🚫 | Red | Blocked bởi dependency |
| `cancelled` | ✕ | Grey (dim) | Đã huỷ |
