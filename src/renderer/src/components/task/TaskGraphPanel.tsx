// TaskGraphPanel.tsx — Task graph panel stub (full implementation in TASK-V5-07)
interface TaskGraphPanelProps {
  projectId: string
}

export function TaskGraphPanel({ projectId }: TaskGraphPanelProps) {
  return (
    <div className="task-graph-panel p-4 text-xs text-muted-foreground" data-testid="task-graph-panel">
      Task Graph for project: {projectId}
    </div>
  )
}
