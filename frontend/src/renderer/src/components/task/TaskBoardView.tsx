import { useState } from 'react'
import type { OrcaTask, TaskStatus } from '../../../../shared/task-types'
import { normalizeTaskStatus } from '../../../../shared/task-status-normalization'
import { TaskStatusBadge } from './TaskStatusBadge'
import { TaskCard } from './TaskCard'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { openRequestPage } from '../request/request-page-navigation'

const STATUS_ORDER: TaskStatus[] = [
  'open',
  'todo',
  'in_progress',
  'blocked',
  'review',
  'done',
  'cancelled'
]

export function TaskBoardView({
  tasks,
  onSelect,
  selectMode = false,
  selectedIds,
  onToggleSelect
}: {
  tasks: OrcaTask[]
  onSelect: (id: string) => void
  // BL-TG-06: same selectMode/selectedIds/onToggleSelect shape
  // TaskTreeView/TaskGraph (DAG) already accept — Board was the one view
  // missing it, confirmed by reading this file before BL-TG-06 (only
  // `onSelect` existed here).
  selectMode?: boolean
  selectedIds?: Set<string>
  onToggleSelect?: (id: string) => void
}) {
  // Optimistic move + rollback kept local to this view — useTask.ts's updateTask()
  // has no revert path when the backend rejects (e.g. permission denied), so a
  // failed drag would otherwise leave the card in the wrong column until F5.
  const requestFlowSupported = useAppStore((s) => s.requestFlowSupport) === 'supported'
  const [statusOverride, setStatusOverride] = useState<Record<string, TaskStatus>>({})

  const effectiveStatus = (task: OrcaTask): TaskStatus =>
    statusOverride[task.id] ?? normalizeTaskStatus(task.status)

  const handleDrop = async (taskId: string, newStatus: TaskStatus) => {
    const task = tasks.find((t) => t.id === taskId)
    if (!task || effectiveStatus(task) === newStatus) {
      return
    }
    setStatusOverride((prev) => ({ ...prev, [taskId]: newStatus }))
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      await callRuntimeRpc(target, 'task.update', { taskId, patch: { status: newStatus } })
      useAppStore.getState().updateTask(taskId, { status: newStatus })
      setStatusOverride((prev) => {
        const { [taskId]: _drop, ...rest } = prev
        return rest
      })
    } catch (err) {
      // Rollback: drop the override so the card reflects the store's real (unchanged) status.
      setStatusOverride((prev) => {
        const { [taskId]: _drop, ...rest } = prev
        return rest
      })
      const message = err instanceof Error ? err.message : String(err)
      toast.error(`Cannot move "${task.title}" to ${newStatus}: ${message}`)
    }
  }

  return (
    <div className="flex h-full flex-col">
      {requestFlowSupported && (
        // Why: tasks with no approved Plan have no board status any more; point at where they live.
        <div
          className="flex items-center gap-1 px-3 pt-2 text-xs text-muted-foreground"
          data-testid="board-backlog-hint"
        >
          {translate(
            'auto.components.task.TaskBoardView.backlogMoved',
            'Tasks not started yet live under Requests > Backlog'
          )}
          <Button
            variant="link"
            size="sm"
            className="h-auto p-0 text-xs"
            onClick={() => openRequestPage({ section: 'backlog' })}
          >
            {translate('auto.components.task.TaskBoardView.openBacklog', 'Open backlog')}
          </Button>
        </div>
      )}
      <div
        className="task-board-view flex gap-3 overflow-x-auto p-2 h-full"
        data-testid="task-board-view"
      >
        {STATUS_ORDER.map((status) => {
          const columnTasks = tasks.filter((t) => effectiveStatus(t) === status)
          return (
            <div
              key={status}
              className="flex-shrink-0 w-64 flex flex-col"
              data-testid={`board-column-${status}`}
            >
              <div className="text-xs font-medium px-2 py-1 flex items-center gap-1">
                <TaskStatusBadge status={status} /> ({columnTasks.length})
              </div>
              <div
                className="flex-1 space-y-2 overflow-y-auto min-h-[100px]"
                onDrop={(e) => {
                  e.preventDefault()
                  void handleDrop(e.dataTransfer.getData('taskId'), status)
                }}
                onDragOver={(e) => e.preventDefault()}
                data-testid={`board-dropzone-${status}`}
              >
                {columnTasks.map((task) => (
                  <div
                    key={task.id}
                    draggable
                    onDragStart={(e) => e.dataTransfer.setData('taskId', task.id)}
                    data-testid={`board-card-${task.id}`}
                  >
                    <TaskCard
                      task={task}
                      depth={0}
                      isExpanded={false}
                      onToggle={() => {}}
                      onSelect={onSelect}
                      selectMode={selectMode}
                      isSelected={selectedIds?.has(task.id)}
                      onToggleSelect={onToggleSelect}
                    />
                  </div>
                ))}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
