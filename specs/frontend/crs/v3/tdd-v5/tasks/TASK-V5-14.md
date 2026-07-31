# TASK-V5-14: Task Slice + useTasks + useTask

**Order:** 14 | **Prerequisite:** TASK-V5-02 (WorkspaceContext) | **Tests:** 9

---

## Files Cần Tạo

### 1. `src/renderer/src/store/slices/task.ts`

```typescript
import type { OrcaTask, TaskStatus } from '@shared/task-types'

export type TaskSlice = {
  tasks:        OrcaTask[]
  activeTaskId: string | null
  taskLoading:  boolean
  
  setTasks(tasks: OrcaTask[]): void
  addTask(task: OrcaTask): void
  updateTask(id: string, patch: Partial<OrcaTask>): void
  removeTask(id: string): void
  setActiveTask(id: string | null): void
  setTaskLoading(v: boolean): void
}

export function createTaskSlice(set): TaskSlice {
  return {
    tasks: [], activeTaskId: null, taskLoading: false,
    setTasks:       (tasks) => set(s => { s.tasks = tasks }),
    addTask:        (task)  => set(s => { s.tasks.push(task) }),
    updateTask:     (id, patch) => set(s => {
      const idx = s.tasks.findIndex((t: OrcaTask) => t.id === id)
      if (idx !== -1) Object.assign(s.tasks[idx], patch)
    }),
    removeTask:     (id) => set(s => { s.tasks = s.tasks.filter((t: OrcaTask) => t.id !== id) }),
    setActiveTask:  (id) => set(s => { s.activeTaskId = id }),
    setTaskLoading: (v)  => set(s => { s.taskLoading = v }),
  }
}
```

### 2. `src/renderer/src/hooks/useTasks.ts`

```typescript
// Fetch tasks for project, filter, tree traversal helpers

export function useTasks(projectId: string) {
  const { tasks, activeTaskId } = useAppStore(s => ({
    tasks: s.tasks.filter((t: OrcaTask) => t.projectId === projectId),
    activeTaskId: s.activeTaskId,
  }))
  const [expandedNodes, setExpandedNodes] = useState(new Set<string>())
  const [filterStatus,  setFilterStatus]  = useState<TaskStatus | 'all'>('all')
  const [searchQuery,   setSearchQuery]   = useState('')

  const filteredTasks = useMemo(() => {
    let r = tasks
    if (filterStatus !== 'all') r = r.filter(t => t.status === filterStatus)
    if (searchQuery) r = r.filter(t => t.title.toLowerCase().includes(searchQuery.toLowerCase()))
    return r
  }, [tasks, filterStatus, searchQuery])

  const toggleExpanded = (taskId: string) => {
    setExpandedNodes(prev => {
      const s = new Set(prev)
      s.has(taskId) ? s.delete(taskId) : s.add(taskId)
      return s
    })
  }

  useEffect(() => {
    callRuntimeRpc('task.list', { projectId })
      .then(t => useAppStore.getState().setTasks(t as OrcaTask[]))
  }, [projectId])

  return {
    tasks, filteredTasks, activeTaskId, expandedNodes,
    toggleExpanded, filterStatus, setFilterStatus, searchQuery, setSearchQuery,
    setActiveTask: (id: string | null) => useAppStore.getState().setActiveTask(id),
  }
}
```

### 3. `src/renderer/src/hooks/useTask.ts`

```typescript
// Single-task CRUD + AI decompose + agent run

export function useTask(taskId: string) {
  const task = useAppStore(s => s.tasks.find((t: OrcaTask) => t.id === taskId))

  const updateTask = async (patch: Partial<OrcaTask>) => {
    await callRuntimeRpc('task.update', { taskId, ...patch })
    useAppStore.getState().updateTask(taskId, patch)
  }

  const deleteTask = async () => {
    await callRuntimeRpc('task.delete', { taskId })
    useAppStore.getState().removeTask(taskId)
  }

  const aiDecompose = async (instruction?: string) => {
    const result = await callRuntimeRpc('task.aiDecompose', { parentTaskId: taskId, instruction }) as {
      subtasks: Partial<OrcaTask>[]
    }
    return result.subtasks
  }

  const acceptSubtasks = async (subtasks: Partial<OrcaTask>[], projectId: string) => {
    for (const st of subtasks) {
      const created = await callRuntimeRpc('task.create', {
        ...st, parentId: taskId, projectId
      }) as OrcaTask
      useAppStore.getState().addTask(created)
    }
  }

  return { task, updateTask, deleteTask, aiDecompose, acceptSubtasks }
}
```

---

## Files Cần Sửa

`store/index.ts` → register `createTaskSlice`

---

## Tests (9 total)

```
hooks/__tests__/useTasks.test.ts     (6 tests)
  fetches tasks on mount | status filter | search filter
  toggleExpanded | setActiveTask | projectId change re-fetches

hooks/__tests__/useTask.test.ts      (3 tests)
  updateTask calls task.update + store
  aiDecompose calls task.aiDecompose and returns subtasks
  acceptSubtasks creates each subtask via task.create
```

## Acceptance Criteria

- [x] `TaskSlice` registered trong store
- [x] `useTasks.filteredTasks` filters by status + search
- [x] `toggleExpanded` add/remove from set
- [x] `useTask.aiDecompose()` returns subtasks array
- [x] `useTask.acceptSubtasks()` calls `task.create` for each
- [x] 9/9 tests pass
