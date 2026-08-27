import { ChevronDown, ChevronRight } from 'lucide-react'
import { TaskStatusBadge, TaskPriorityBadge } from './TaskStatusBadge'
import { useAppStore } from '../../store'
import type { OrcaTask } from '../../../../shared/task-types'
import type { ReactNode } from 'react'

type TaskCardProps = {
  task: OrcaTask
  depth: number
  isExpanded: boolean
  onToggle: (id: string) => void
  onSelect: (id: string) => void
  children?: ReactNode
}

export function TaskCard({ task, depth, isExpanded, onToggle, onSelect, children }: TaskCardProps) {
  const hasChildren = useAppStore(s => s.tasks.some(t => t.parentId === task.id))
  
  return (
    <div style={{ paddingLeft: depth * 20 }} data-testid={`task-card-${task.id}`}>
      <div className="flex items-center gap-2 py-1 hover:bg-accent/50 cursor-pointer" onClick={() => onSelect(task.id)}>
        {hasChildren ? (
          <button onClick={e => { e.stopPropagation(); onToggle(task.id) }}>
            {isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          </button>
        ) : (
          <span style={{ width: 12, height: 12, display: 'inline-block' }} />
        )}
        <span className="text-xs font-mono text-muted-foreground uppercase">{task.type}</span>
        <span className="flex-1 text-sm truncate">{task.title}</span>
        <TaskPriorityBadge priority={task.priority} />
        <TaskStatusBadge status={task.status} />
        {task.progressPercent > 0 && (
          <span className="text-xs text-muted-foreground">{task.progressPercent}%</span>
        )}
      </div>
      {children}
    </div>
  )
}
