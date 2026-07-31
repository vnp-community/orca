# TASK-V5-15: TaskGraph + TaskTreeView + TaskCard

**Order:** 15 | **Prerequisite:** TASK-V5-14 | **Tests:** 14

---

## Files Cần Tạo

### 1. `src/renderer/src/components/task/TaskStatusBadge.tsx`

```typescript
const STATUS_CONFIG = {
  todo:        { label: 'Todo',        icon: '⏳', className: 'text-gray-500' },
  in_progress: { label: 'In Progress', icon: '🔄', className: 'text-blue-600' },
  done:        { label: 'Done',        icon: '✅', className: 'text-green-600' },
  cancelled:   { label: 'Cancelled',   icon: '❌', className: 'text-gray-400' },
}
const PRIORITY_CONFIG = {
  critical: { label: 'Critical', className: 'text-red-600',    dot: '🔴' },
  high:     { label: 'High',     className: 'text-orange-500', dot: '🟠' },
  medium:   { label: 'Medium',   className: 'text-yellow-600', dot: '🟡' },
  low:      { label: 'Low',      className: 'text-green-600',  dot: '🟢' },
}
export function TaskStatusBadge({ status }) { ... }
export function TaskPriorityBadge({ priority }) { ... }
```

### 2. `src/renderer/src/components/task/TaskCard.tsx`

```typescript
// Single task row in tree view
// Shows: type badge | title | priority | status | progress bar | expand button

export function TaskCard({ task, depth, isExpanded, onToggle, onSelect, children }) {
  const hasChildren = /* check store for tasks with parentId === task.id */
  return (
    <div style={{ paddingLeft: depth * 20 }} data-testid={`task-card-${task.id}`}>
      <div className="flex items-center gap-2 py-1 hover:bg-accent/50 cursor-pointer" onClick={() => onSelect(task.id)}>
        {hasChildren && (
          <button onClick={e => { e.stopPropagation(); onToggle(task.id) }}>
            {isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          </button>
        )}
        <span className="text-xs font-mono text-muted-foreground uppercase">{task.type}</span>
        <span className="flex-1 text-sm truncate">{task.title}</span>
        <TaskPriorityBadge priority={task.priority} />
        <TaskStatusBadge status={task.status} />
        {task.progress > 0 && <span className="text-xs text-muted-foreground">{task.progress}%</span>}
      </div>
      {children}
    </div>
  )
}
```

### 3. `src/renderer/src/components/task/TaskTreeView.tsx`

```typescript
// Recursive tree render from flat list using parentId
function renderLevel(tasks: OrcaTask[], parentId: string | null, depth: number, ...): ReactNode {
  return tasks
    .filter(t => t.parentId === parentId)
    .map(task => (
      <TaskCard key={task.id} task={task} depth={depth} isExpanded={expandedNodes.has(task.id)} onToggle={toggleExpanded} onSelect={setActiveTask}>
        {expandedNodes.has(task.id) && renderLevel(tasks, task.id, depth + 1, ...)}
      </TaskCard>
    ))
}

export function TaskTreeView({ tasks, expandedNodes, toggleExpanded, setActiveTask }) {
  return <div data-testid="task-tree-view">{renderLevel(tasks, null, 0, expandedNodes, toggleExpanded, setActiveTask)}</div>
}
```

### 4. `src/renderer/src/components/task/TaskGraph.tsx`

```typescript
import { lazy, Suspense, useState } from 'react'
import { useTasks } from '../../hooks/useTasks'
import { TaskTreeView } from './TaskTreeView'
const TaskDAGView = lazy(() => import('./TaskDAGView'))

export function TaskGraph({ projectId }: { projectId: string }) {
  const { filteredTasks, expandedNodes, toggleExpanded, setActiveTask, filterStatus, setFilterStatus, searchQuery, setSearchQuery } = useTasks(projectId)
  const [viewMode, setViewMode] = useState<'tree' | 'dag'>('tree')

  return (
    <div className="task-graph flex flex-col h-full" data-testid="task-graph">
      <div className="flex items-center gap-2 p-2 border-b">
        <Input placeholder="Search tasks..." value={searchQuery} onChange={e => setSearchQuery(e.target.value)} className="flex-1 h-7 text-sm" />
        <select value={filterStatus} onChange={e => setFilterStatus(e.target.value as any)} className="text-sm border rounded px-2 py-1">
          <option value="all">All Status</option>
          <option value="todo">Todo</option>
          <option value="in_progress">In Progress</option>
          <option value="done">Done</option>
        </select>
        <div className="flex border rounded overflow-hidden">
          <button onClick={() => setViewMode('tree')} className={`px-2 py-1 text-xs ${viewMode === 'tree' ? 'bg-primary text-primary-foreground' : ''}`} data-testid="view-tree">Tree</button>
          <button onClick={() => setViewMode('dag')} className={`px-2 py-1 text-xs ${viewMode === 'dag' ? 'bg-primary text-primary-foreground' : ''}`} data-testid="view-dag">DAG</button>
        </div>
      </div>
      <div className="flex-1 overflow-auto">
        {viewMode === 'tree' ? (
          <TaskTreeView tasks={filteredTasks} expandedNodes={expandedNodes} toggleExpanded={toggleExpanded} setActiveTask={setActiveTask} />
        ) : (
          <Suspense fallback={<div className="p-4 text-sm">Loading DAG...</div>}>
            <TaskDAGView tasks={filteredTasks} onSelect={setActiveTask} />
          </Suspense>
        )}
      </div>
    </div>
  )
}
```

---

## Tests (14 total)

```
__tests__/task/TaskGraph.test.tsx      (5 tests)
  renders tree view by default | switches to DAG on click
  filter by status | search filter | click task calls setActiveTask

__tests__/task/TaskTreeView.test.tsx   (5 tests)
  renders root tasks (parentId=null) | expand toggle shows children
  recursive grandchildren render | collapsed hides children
  priority badge shown per task

__tests__/task/TaskCard.test.tsx       (4 tests)
  renders task title + type | status badge correct
  expand button for tasks with children | no expand for leaf tasks
```

## Acceptance Criteria

- [x] `TaskGraph` default: tree view
- [x] DAG view lazy loaded
- [x] `TaskTreeView` renders only `parentId=null` tasks at root
- [x] Expand toggle shows children, collapse hides them
- [x] Filter + search work
- [x] 14/14 tests pass
