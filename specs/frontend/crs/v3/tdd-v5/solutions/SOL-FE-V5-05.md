# SOL-FE-V5-05: Task Graph UI

**TDD Ref:** [TDD-FE-15](../../../tdd/15-task-graph-ui.md)  
**Feature:** F37 | **ADR:** ADR-010 | **HLD:** C3.11b  
**Status:** ✅ DONE — Implemented via TASK-V5-14, TASK-V5-15, TASK-V5-16  
**Dependency:** WorkspaceContext (SOL-FE-V5-02), @xyflow/react (SOL-FE-V5-04)

---

## 1. Files Cần Tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/store/slices/task.ts` | Zustand Slice | tasks, activeTaskId, taskGraph |
| `src/renderer/src/components/task/TaskGraph.tsx` | Component | Tree/DAG dual-view |
| `src/renderer/src/components/task/TaskTreeView.tsx` | Component | Recursive tree list |
| `src/renderer/src/components/task/TaskDAGView.tsx` | Component | React Flow DAG |
| `src/renderer/src/components/task/TaskCard.tsx` | Component | Task row/card |
| `src/renderer/src/components/task/TaskDetail.tsx` | Component | Right-panel detail |
| `src/renderer/src/components/task/TaskAIDecompose.tsx` | Component | AI task decomposition UI |
| `src/renderer/src/components/task/TaskPromptEditor.tsx` | Component | Prompt textarea + model picker |
| `src/renderer/src/components/task/TaskStatusBadge.tsx` | Component | Status icon/color |
| `src/renderer/src/components/task/TaskPriorityBadge.tsx` | Component | Priority indicator |
| `src/renderer/src/components/task/CreateTaskDialog.tsx` | Component | New task dialog |
| `src/renderer/src/hooks/useTask.ts` | Hook | Tasks CRUD + AI ops |
| `src/renderer/src/hooks/useTasks.ts` | Hook | List + filter + tree traversal |

---

## 2. Task Slice

```typescript
// src/renderer/src/store/slices/task.ts

export type OrcaTask = {
  id:           string
  projectId:    string
  parentId:     string | null
  type:         'epic' | 'story' | 'task' | 'bug' | 'chore'
  title:        string
  description?: string
  status:       TaskStatus      // 'todo' | 'in_progress' | 'done' | 'cancelled'
  priority:     TaskPriority    // 'critical' | 'high' | 'medium' | 'low'
  assigneeId?:  string
  dependsOn:    string[]        // task IDs
  agentPrompt?: string
  progress:     number          // 0-100
  createdAt:    number
  updatedAt:    number
}

export type TaskSlice = {
  tasks:          OrcaTask[]
  activeTaskId:   string | null
  taskIsLoading:  boolean

  setTasks(tasks: OrcaTask[]): void
  addTask(task: OrcaTask): void
  updateTask(id: string, patch: Partial<OrcaTask>): void
  removeTask(id: string): void
  setActiveTask(id: string | null): void
}
```

---

## 3. TaskGraph — Dual View Architecture

```typescript
// Tree View: renderTree(tasks, parentId=null)
// → Recursive — uses parentId for hierarchy

function renderTree(tasks: OrcaTask[], parentId: string | null, depth: number): ReactNode {
  const children = tasks.filter(t => t.parentId === parentId)
  if (children.length === 0) return null

  return children.map(task => (
    <TaskCard key={task.id} task={task} depth={depth}>
      {expandedNodes.has(task.id) && renderTree(tasks, task.id, depth + 1)}
    </TaskCard>
  ))
}

// DAG View: TaskDAGView
// → @xyflow/react nodes
// → edges from task.dependsOn (cross-hierarchy deps)
// → node style by status (todo=gray, in_progress=blue, done=green, critical=red border)
```

---

## 4. TaskAIDecompose — AI Decomposition

```typescript
// src/renderer/src/components/task/TaskAIDecompose.tsx
// User clicks "🤖 Decompose with AI" → LLM generates subtasks

export function TaskAIDecompose({ parentTask }: { parentTask: OrcaTask }) {
  const [prompt, setPrompt] = useState('')
  const [isDecomposing, setIsDecomposing] = useState(false)
  const [proposedSubtasks, setProposedSubtasks] = useState<Partial<OrcaTask>[]>([])

  const decompose = async () => {
    setIsDecomposing(true)
    const result = await rpc('task.aiDecompose', {
      parentTaskId: parentTask.id,
      instruction:  prompt || undefined
    }) as { subtasks: Partial<OrcaTask>[] }
    setProposedSubtasks(result.subtasks)
    setIsDecomposing(false)
  }

  const acceptSubtasks = async () => {
    for (const st of proposedSubtasks) {
      await rpc('task.create', { ...st, parentId: parentTask.id, projectId: parentTask.projectId })
    }
    // Refresh task list
    useAppStore.getState().setTasks(
      await rpc('task.list', { projectId: parentTask.projectId }) as OrcaTask[]
    )
    setProposedSubtasks([])
  }

  // UI:
  // - Optional prompt override
  // - "Decompose" button with spinner
  // - Proposed subtasks list (editable before accept)
  // - "Accept All" / "Cancel" buttons
}
```

---

## 5. TaskPromptEditor Component

```typescript
// src/renderer/src/components/task/TaskPromptEditor.tsx
// For assigning an AI prompt to a task (agent will execute)

// Fields:
// - Model selector (filtered by resolvedProfile.security.approvedModels)
// - Prompt textarea (markdown supported)
// - "Run with Agent" button
// - AgentSessionStatus (shows running/complete status)

// On "Run with Agent":
// rpc('task.runAgent', { taskId, prompt, model }) → { agentSessionId }
// Subscribe to IPC events for streaming output
```

---

## 6. useTasks Hook

```typescript
// src/renderer/src/hooks/useTasks.ts

export function useTasks(projectId: string) {
  const { tasks, activeTaskId } = useAppStore(s => ({
    tasks:       s.tasks.filter(t => t.projectId === projectId),
    activeTaskId: s.activeTaskId,
  }))

  const [expandedNodes, setExpandedNodes] = useState(new Set<string>())
  const [filterStatus, setFilterStatus] = useState<TaskStatus | 'all'>('all')
  const [searchQuery, setSearchQuery] = useState('')

  const filteredTasks = useMemo(() => {
    let result = tasks
    if (filterStatus !== 'all') result = result.filter(t => t.status === filterStatus)
    if (searchQuery) result = result.filter(t =>
      t.title.toLowerCase().includes(searchQuery.toLowerCase())
    )
    return result
  }, [tasks, filterStatus, searchQuery])

  // DAG view: all tasks (for dep edges)
  const dagView = tasks

  const toggleExpanded = (taskId: string) => {
    setExpandedNodes(prev => {
      const s = new Set(prev)
      s.has(taskId) ? s.delete(taskId) : s.add(taskId)
      return s
    })
  }

  const setActiveTask = useCallback((id: string | null) => {
    useAppStore.getState().setActiveTask(id)
  }, [])

  // Fetch on mount + projectId change
  useEffect(() => {
    rpc('task.list', { projectId }).then(tasks => {
      useAppStore.getState().setTasks(tasks as OrcaTask[])
    })
  }, [projectId])

  return { tasks, filteredTasks, dagView, expandedNodes, toggleExpanded,
           setActiveTask, filterStatus, setFilterStatus, searchQuery, setSearchQuery }
}
```

---

## 7. Files Cần Sửa (Additive)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/index.ts` | Register `createTaskSlice` |
| `src/renderer/src/components/workspace/WorkspaceLayout.tsx` | Mount `<TaskGraphPanel />` trong tasks tab |

---

## 8. RPC Methods

| Method | Params | Return |
|--------|--------|--------|
| `task.list` | `{ projectId, parentId?, status? }` | `OrcaTask[]` |
| `task.get` | `{ taskId }` | `OrcaTask` |
| `task.create` | `{ projectId, parentId?, title, type, ... }` | `OrcaTask` |
| `task.update` | `{ taskId, ...patch }` | `OrcaTask` |
| `task.delete` | `{ taskId }` | `void` |
| `task.aiDecompose` | `{ parentTaskId, instruction? }` | `{ subtasks }` |
| `task.runAgent` | `{ taskId, prompt, model? }` | `{ agentSessionId }` |

---

## 9. Test Plan

```
src/renderer/src/components/task/__tests__/
├── TaskGraph.test.tsx             (5 tests)
│   ├── renders tree view by default
│   ├── switching to DAG view shows React Flow
│   ├── click task → sets activeTask
│   ├── filter by status: only matching tasks shown
│   └── search: only matching tasks shown
├── TaskTreeView.test.tsx          (5 tests)
│   ├── renders root tasks (parentId=null)
│   ├── expands children on toggle
│   ├── recursive rendering: grandchildren
│   ├── collapsed node hides children
│   └── priority badge shown per task
├── TaskCard.test.tsx              (4 tests)
│   ├── renders task title + type
│   ├── status badge correct color
│   ├── expand toggle shown for tasks with children
│   └── clicking opens TaskDetail
├── TaskAIDecompose.test.tsx       (5 tests)
│   ├── "Decompose" calls task.aiDecompose RPC
│   ├── shows proposed subtasks list
│   ├── "Accept All" creates subtasks via task.create
│   ├── "Cancel" clears proposed subtasks
│   └── shows spinner during decomposing
└── hooks/__tests__/useTasks.test.ts  (6 tests)
    ├── fetches tasks on mount
    ├── filteredTasks: status filter
    ├── filteredTasks: search filter
    ├── toggleExpanded adds/removes from set
    ├── setActiveTask updates store
    └── projectId change re-fetches tasks
```

**Target:** ≥ 25 tests
