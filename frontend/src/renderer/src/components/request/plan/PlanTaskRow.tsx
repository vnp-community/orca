/**
 * PlanTaskRow — CR-REQ-021-03
 *
 * Read-only row: no status dropdown, no drag, no Run button (Run lives in TaskDetail).
 *
 * @module components/request/plan/PlanTaskRow
 */

import React from 'react'
import { GitCompareArrows } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { TaskStatusBadge } from '../../task/TaskStatusBadge'
import { ExecutionEngineBadge } from '../../task/ExecutionEngineBadge'
import { ReadinessBadge } from '../readiness/ReadinessBadge'
import type { TaskReadinessReport } from '../../../../../shared/request-artifact-types'
import type { OrcaTask } from '../../../../../shared/task-types'

type Props = {
  task: OrcaTask
  onOpen: (task: OrcaTask) => void
  /** Latest readiness report; omitted/null shows no badge. */
  readiness?: TaskReadinessReport | null
  /** Actual change drifted from the plan (impact.drift); chip only, the banner is PlanDriftSection's. */
  drifted?: boolean
}

export function PlanTaskRow({
  task,
  onOpen,
  readiness = null,
  drifted = false
}: Props): React.JSX.Element {
  return (
    <button
      type="button"
      onClick={() => onOpen(task)}
      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      data-testid={`plan-task-row-${task.id}`}
    >
      {task.taskNumber !== undefined && (
        <span className="font-mono text-xs text-muted-foreground">#TG-{task.taskNumber}</span>
      )}
      <span className="min-w-0 flex-1 truncate">{task.title}</span>
      {task.estimatedHours !== undefined && (
        <span className="text-xs text-muted-foreground">
          {translate('auto.components.request.plan.PlanTaskRow.estimate', '{{hours}}h', {
            hours: task.estimatedHours
          })}
        </span>
      )}
      {task.status === 'blocked' && (
        <span className="rounded border border-destructive/40 px-1.5 text-xs text-destructive">
          {translate('auto.components.request.plan.PlanTaskRow.blocked', 'Blocked')}
        </span>
      )}
      {drifted && (
        <span
          className="inline-flex items-center gap-1 rounded border border-risk-medium-border bg-risk-medium-background px-1.5 text-xs text-risk-medium"
          data-testid={`plan-task-drift-${task.id}`}
        >
          <GitCompareArrows className="size-3" aria-hidden />
          {translate('auto.components.request.plan.PlanTaskRow.drifted', 'Drifted')}
        </span>
      )}
      <ReadinessBadge report={readiness} compact />
      <ExecutionEngineBadge task={task} />
      <TaskStatusBadge status={task.status} />
    </button>
  )
}
