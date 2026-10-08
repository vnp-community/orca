import { ChevronDown, ChevronRight, Plus } from 'lucide-react'
import { TaskStatusBadge, TaskPriorityBadge } from './TaskStatusBadge'
import { useAppStore } from '../../store'
import { computeClientProgress } from '../../hooks/useTasks'
import type { TaskWithPlanPath } from '../../../../shared/task-hierarchy'
import type { ReactNode } from 'react'

type TaskCardProps = {
  task: TaskWithPlanPath
  depth: number
  isExpanded: boolean
  onToggle: (id: string) => void
  onSelect: (id: string) => void
  children?: ReactNode
  onCreateSubtask?: (parentId: string) => void
  selectMode?: boolean
  isSelected?: boolean
  onToggleSelect?: (id: string) => void
}

export function TaskCard({
  task,
  depth,
  isExpanded,
  onToggle,
  onSelect,
  children,
  onCreateSubtask,
  selectMode,
  isSelected,
  onToggleSelect
}: TaskCardProps) {
  // Read once and derive both `hasChildren` and the client-side progress estimate from
  // the same stable `allTasks` reference — see useTasks.ts's own doc comment on why a
  // `.some()`/`.filter()` inside the selector itself would break React's snapshot check.
  const allTasks = useAppStore((s) => s.tasks)
  const hasChildren = allTasks.some((t) => t.parentId === task.id)
  const clientProgress = hasChildren ? computeClientProgress(allTasks, task.id) : null
  const displayProgress = clientProgress ?? task.progressPercent

  return (
    <div style={{ paddingLeft: depth * 20 }} data-testid={`task-card-${task.id}`}>
      <div
        className="group flex items-center gap-2 py-1 hover:bg-accent/50 cursor-pointer"
        onClick={() => onSelect(task.id)}
      >
        {selectMode && (
          <input
            type="checkbox"
            checked={isSelected}
            onClick={(e) => e.stopPropagation()}
            onChange={() => onToggleSelect?.(task.id)}
            data-testid={`task-select-${task.id}`}
          />
        )}
        {hasChildren ? (
          <button
            onClick={(e) => {
              e.stopPropagation()
              onToggle(task.id)
            }}
          >
            {isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          </button>
        ) : (
          <span style={{ width: 12, height: 12, display: 'inline-block' }} />
        )}
        <span className="text-xs font-mono text-muted-foreground uppercase">{task.type}</span>
        <span className="flex-1 text-sm truncate">{task.title}</span>
        {task.planPath && task.planPath.length > 0 && (
          <span
            className="max-w-40 truncate rounded border px-1.5 text-[10px] text-muted-foreground"
            title={task.planPath.join(' / ')}
            data-testid={`task-plan-path-${task.id}`}
          >
            {task.planPath.join(' / ')}
          </span>
        )}
        <TaskPriorityBadge priority={task.priority} />
        <TaskStatusBadge status={task.status} />
        {(hasChildren || task.progressPercent > 0) && (
          <span
            className="text-xs text-muted-foreground"
            title={
              hasChildren
                ? 'Client-side estimate from subtask done ratio — see BUG-TASKV1-001'
                : undefined
            }
          >
            {displayProgress}%
          </span>
        )}
        {onCreateSubtask && (
          <button
            className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-foreground"
            onClick={(e) => {
              e.stopPropagation()
              onCreateSubtask(task.id)
            }}
            title="Add subtask"
            data-testid={`task-add-subtask-${task.id}`}
          >
            <Plus size={12} />
          </button>
        )}
      </div>
      {children}
    </div>
  )
}
