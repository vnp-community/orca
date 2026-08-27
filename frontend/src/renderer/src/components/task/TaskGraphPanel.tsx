// TaskGraphPanel.tsx — workspace center-panel wrapper around TaskGraph (TASK-V5-07)
// TaskGraph already composes search/filter/tree-vs-dag toggle + TaskTreeView via useTasks
// (task.list RPC), so this panel just gives it the panel-level container WorkspaceLayout expects.
import { TaskGraph } from './TaskGraph'

type TaskGraphPanelProps = {
  projectId: string
}

export function TaskGraphPanel({ projectId }: TaskGraphPanelProps) {
  return (
    <div className="task-graph-panel h-full" data-testid="task-graph-panel">
      <TaskGraph projectId={projectId} />
    </div>
  )
}
