/**
 * TaskLoadStates — TASK-FE-E2E-007
 *
 * What the task tree says when it has nothing to draw: still loading, genuinely empty,
 * emptied by filters, or failed to load. "No tasks" and "could not load" must never look alike.
 *
 * @module components/task/TaskLoadStates
 */

import React from 'react'
import { AlertCircle } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '../ui/button'
import { requestErrorMessage } from '../request/request-error-message'
import type { TaskLoadError } from '../../hooks/useTasks'

const T = 'auto.components.task.TaskLoadStates.'

type Props = {
  loaded: boolean
  isLoading: boolean
  loadError: TaskLoadError | null
  /** Tasks currently drawn (after filters). */
  visibleCount: number
  /** Tasks of the project before filters. */
  totalCount: number
  filtersActive: boolean
  /** Plan/Phase tasks exist but are hidden by the toggle. */
  planningHidden: boolean
  onRetry: () => void
}

export function TaskLoadStates({
  loaded,
  isLoading,
  loadError,
  visibleCount,
  totalCount,
  filtersActive,
  planningHidden,
  onRetry
}: Props): React.JSX.Element | null {
  if (loadError) {
    const cause =
      loadError.kind === 'forbidden'
        ? translate(`${T}forbidden`, 'You do not have access to the tasks of this project.')
        : requestErrorMessage(loadError.kind)
    return (
      <div
        role="alert"
        data-testid="task-load-error"
        data-error-kind={loadError.kind}
        className="flex flex-col items-start gap-1.5 border-b border-destructive/30 px-3 py-2"
      >
        <p className="flex items-center gap-2 text-sm text-destructive">
          <AlertCircle className="size-4 shrink-0" aria-hidden />
          {totalCount > 0
            ? translate(
                `${T}staleTitle`,
                'Could not refresh the tasks. Showing the last loaded list.'
              )
            : translate(`${T}errorTitle`, 'Could not load the tasks.')}
        </p>
        <p className="text-xs text-muted-foreground">{cause}</p>
        <p className="text-xs text-muted-foreground" data-testid="task-load-error-code">
          {translate(`${T}code`, 'Error code: {{code}}', { code: loadError.code })}
        </p>
        <Button size="xs" variant="outline" onClick={onRetry}>
          {translate(`${T}retry`, 'Retry')}
        </Button>
      </div>
    )
  }
  if (isLoading && totalCount === 0) {
    return (
      <p
        className="px-3 py-4 text-sm text-muted-foreground"
        data-testid="task-loading"
        aria-busy="true"
      >
        {translate(`${T}loading`, 'Loading tasks...')}
      </p>
    )
  }
  if (!loaded || isLoading || visibleCount > 0) {
    return null
  }
  const [testId, text] =
    totalCount === 0
      ? ['task-empty', translate(`${T}empty`, 'No tasks yet.')]
      : filtersActive
        ? [
            'task-empty-filtered',
            translate(`${T}emptyFiltered`, 'No tasks match the current search or filter.')
          ]
        : planningHidden
          ? [
              'task-empty-planning-hidden',
              translate(
                `${T}emptyPlanningHidden`,
                'Only Plan and Phase tasks exist. Turn on "Show Plan/Phase" to see them.'
              )
            ]
          : ['task-empty', translate(`${T}empty`, 'No tasks yet.')]
  return (
    <p className="px-3 py-4 text-sm text-muted-foreground" data-testid={testId}>
      {text}
    </p>
  )
}
