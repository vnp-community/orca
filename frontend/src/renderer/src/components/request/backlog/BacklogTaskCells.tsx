/**
 * BacklogTaskCells — CR-REQ-023-06
 *
 * The title+status cell and the "waiting on" chips shared by the Task and
 * Execute rows.
 *
 * @module components/request/backlog/BacklogTaskCells
 */

import React from 'react'
import { Badge } from '@/components/ui/badge'
import { normalizeTaskStatus } from '../../../../../shared/task-status-normalization'
import { TaskStatusBadge } from '../../task/TaskStatusBadge'
import type { BacklogTaskRowData } from '../../../../../shared/request-backlog-types'

export function BacklogTaskTitleCell({
  task,
  onOpenTask
}: {
  task: BacklogTaskRowData
  onOpenTask: (taskId: string) => void
}): React.JSX.Element {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <TaskStatusBadge status={normalizeTaskStatus(task.status)} />
      <button
        type="button"
        className="truncate text-left font-medium text-foreground hover:underline"
        onClick={() => onOpenTask(task.taskId)}
      >
        {task.title || task.taskId.slice(0, 8)}
      </button>
    </div>
  )
}

const VISIBLE_DEPENDENCIES = 2

/** The backend only lists unfinished dependencies, so every chip is a live blocker. */
export function BacklogDependencyChips({
  taskIds,
  resolveTaskLabel
}: {
  taskIds: string[]
  resolveTaskLabel: (taskId: string) => string
}): React.JSX.Element {
  if (taskIds.length === 0) {return <span className="text-muted-foreground">-</span>}
  const shown = taskIds.slice(0, VISIBLE_DEPENDENCIES)
  const rest = taskIds.slice(VISIBLE_DEPENDENCIES)
  return (
    <div className="flex flex-wrap items-center gap-1">
      <span className="text-xs tabular-nums text-muted-foreground">{taskIds.length}</span>
      {shown.map((id) => (
        <Badge key={id} variant="destructive" className="max-w-40 truncate" data-testid="dependency-chip">
          {resolveTaskLabel(id)}
        </Badge>
      ))}
      {rest.length > 0 && (
        <span className="text-xs text-muted-foreground" title={rest.map(resolveTaskLabel).join(', ')}>
          +{rest.length}
        </span>
      )}
    </div>
  )
}
