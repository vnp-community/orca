import { ReactNode } from 'react'
import type { OrcaTask } from '@shared/task-types'
import { TaskCard } from './TaskCard'

// Recursive tree render from flat list using parentId
function renderLevel(
  tasks: OrcaTask[],
  parentId: string | null,
  depth: number,
  expandedNodes: Set<string>,
  toggleExpanded: (id: string) => void,
  setActiveTask: (id: string | null) => void
): ReactNode {
  return tasks
    .filter(t => t.parentId === parentId)
    .map(task => (
      <TaskCard
        key={task.id}
        task={task}
        depth={depth}
        isExpanded={expandedNodes.has(task.id)}
        onToggle={toggleExpanded}
        onSelect={setActiveTask}
      >
        {expandedNodes.has(task.id) && renderLevel(tasks, task.id, depth + 1, expandedNodes, toggleExpanded, setActiveTask)}
      </TaskCard>
    ))
}

interface TaskTreeViewProps {
  tasks: OrcaTask[]
  expandedNodes: Set<string>
  toggleExpanded: (id: string) => void
  setActiveTask: (id: string | null) => void
}

export function TaskTreeView({ tasks, expandedNodes, toggleExpanded, setActiveTask }: TaskTreeViewProps) {
  return <div data-testid="task-tree-view">{renderLevel(tasks, null, 0, expandedNodes, toggleExpanded, setActiveTask)}</div>
}
