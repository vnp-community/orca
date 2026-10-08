/**
 * ExecuteBacklogRow — CR-REQ-023-06
 *
 * Read-only row of the Execute backlog (approved but not started, or failed).
 * No write actions on purpose: re-running goes through TaskDetail so there is
 * a single path that starts an agent.
 *
 * @module components/request/backlog/ExecuteBacklogRow
 */

import React from 'react'
import { TableCell, TableRow } from '@/components/ui/table'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { BacklogEngineBadge } from './BacklogEngineBadge'
import { BacklogDependencyChips, BacklogTaskTitleCell } from './BacklogTaskCells'
import type { BacklogTaskRowProps } from './TaskBacklogRow'

export function ExecuteBacklogRow({ task, active, rowProps, resolveTaskLabel, onOpenTask }: BacklogTaskRowProps): React.JSX.Element {
  // Why no time column: the backend does not return a failure timestamp yet.
  const failedWithoutDetail = task.lastLinkStatus === 'failed' && !task.lastError
  return (
    <TableRow {...rowProps} role="row" className={cn(active && 'bg-accent/60')} data-testid="execute-backlog-row">
      <TableCell className="max-w-96">
        <BacklogTaskTitleCell task={task} onOpenTask={onOpenTask} />
      </TableCell>
      <TableCell>
        <BacklogDependencyChips taskIds={task.blockedByTaskIds} resolveTaskLabel={resolveTaskLabel} />
      </TableCell>
      <TableCell className="max-w-72">
        {task.lastError ? (
          <span className="line-clamp-2 text-sm" title={task.lastError}>{task.lastError}</span>
        ) : failedWithoutDetail ? (
          <span className="text-sm text-muted-foreground">
            {translate('auto.components.request.backlog.ExecuteBacklogTable.noErrorDetail', 'Failed, no error detail recorded')}
          </span>
        ) : (
          <span className="text-muted-foreground">-</span>
        )}
      </TableCell>
      <TableCell className="tabular-nums">{task.failedAttempts}</TableCell>
      <TableCell>
        <BacklogEngineBadge engine={task.lastEngine} />
      </TableCell>
    </TableRow>
  )
}
