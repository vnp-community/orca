/**
 * TaskBacklogRow — CR-REQ-023-06
 *
 * Read-only row of the Task backlog (tasks whose Plan/Phase is not approved yet).
 *
 * @module components/request/backlog/TaskBacklogRow
 */

import React from 'react'
import { TableCell, TableRow } from '@/components/ui/table'
import { cn } from '@/lib/utils'
import { BacklogDependencyChips, BacklogTaskTitleCell } from './BacklogTaskCells'
import type { BacklogTaskRowData } from '../../../../../shared/request-backlog-types'

export type BacklogTaskRowProps = {
  task: BacklogTaskRowData
  active: boolean
  rowProps: Record<string, unknown>
  resolveTaskLabel: (taskId: string) => string
  onOpenTask: (taskId: string) => void
}

export function TaskBacklogRow({ task, active, rowProps, resolveTaskLabel, onOpenTask }: BacklogTaskRowProps): React.JSX.Element {
  return (
    <TableRow {...rowProps} role="row" className={cn(active && 'bg-accent/60')} data-testid="task-backlog-row">
      <TableCell className="max-w-96">
        <BacklogTaskTitleCell task={task} onOpenTask={onOpenTask} />
      </TableCell>
      <TableCell className="tabular-nums">
        {task.estimatedHours === null ? <span className="text-muted-foreground">-</span> : `${task.estimatedHours}h`}
      </TableCell>
      <TableCell>
        <BacklogDependencyChips taskIds={task.blockedByTaskIds} resolveTaskLabel={resolveTaskLabel} />
      </TableCell>
    </TableRow>
  )
}
