import type { ReactNode } from 'react'
import type { OrcaTask } from '../../../../shared/task-types'
import { TaskCard } from './TaskCard'

// Hoisted so the default prop values are stable across renders instead of a new
// Set/function identity being recreated every render (react/no-object-type-as-default-prop).
const EMPTY_SELECTED_IDS = new Set<string>()
const NOOP_TOGGLE_SELECT = (): void => {}

// Recursive tree render from flat list using parentId
function renderLevel(
  tasks: OrcaTask[],
  parentId: string | null,
  depth: number,
  expandedNodes: Set<string>,
  toggleExpanded: (id: string) => void,
  setActiveTask: (id: string | null) => void,
  onCreateSubtask: ((parentId: string) => void) | undefined,
  selectMode: boolean,
  selectedIds: Set<string>,
  onToggleSelect: (id: string) => void
): ReactNode {
  return tasks
    .filter((t) => t.parentId === parentId)
    .map((task) => (
      <TaskCard
        key={task.id}
        task={task}
        depth={depth}
        isExpanded={expandedNodes.has(task.id)}
        onToggle={toggleExpanded}
        onSelect={setActiveTask}
        onCreateSubtask={onCreateSubtask}
        selectMode={selectMode}
        isSelected={selectedIds.has(task.id)}
        onToggleSelect={onToggleSelect}
      >
        {expandedNodes.has(task.id) &&
          renderLevel(
            tasks,
            task.id,
            depth + 1,
            expandedNodes,
            toggleExpanded,
            setActiveTask,
            onCreateSubtask,
            selectMode,
            selectedIds,
            onToggleSelect
          )}
      </TaskCard>
    ))
}

type TaskTreeViewProps = {
  tasks: OrcaTask[]
  expandedNodes: Set<string>
  toggleExpanded: (id: string) => void
  setActiveTask: (id: string | null) => void
  onCreateSubtask?: (parentId: string) => void
  selectMode?: boolean
  selectedIds?: Set<string>
  onToggleSelect?: (id: string) => void
}

export function TaskTreeView({
  tasks,
  expandedNodes,
  toggleExpanded,
  setActiveTask,
  onCreateSubtask,
  selectMode = false,
  selectedIds = EMPTY_SELECTED_IDS,
  onToggleSelect = NOOP_TOGGLE_SELECT
}: TaskTreeViewProps) {
  return (
    <div data-testid="task-tree-view">
      {renderLevel(
        tasks,
        null,
        0,
        expandedNodes,
        toggleExpanded,
        setActiveTask,
        onCreateSubtask,
        selectMode,
        selectedIds,
        onToggleSelect
      )}
    </div>
  )
}
