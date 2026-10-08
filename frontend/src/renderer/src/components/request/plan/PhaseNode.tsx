/**
 * PhaseNode — CR-REQ-021-03
 *
 * @module components/request/plan/PhaseNode
 */

import React, { useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Progress } from '@/components/ui/progress'
import { TaskStatusBadge } from '../../task/TaskStatusBadge'
import { ApprovalStatusBadge } from '../ApprovalStatusBadge'
import { computePhaseStats, resolveProgress } from './plan-approval-model'
import { PlanTaskRow } from './PlanTaskRow'
import { useTaskReadiness } from '../../../hooks/useTaskReadiness'
import { PhaseReadinessSummary } from '../readiness/PhaseReadinessSummary'
import type { Approval } from '../../../../../shared/request-types'
import type { OrcaTask } from '../../../../../shared/task-types'

type Props = {
  phase: OrcaTask
  tasks: OrcaTask[]
  approval: Approval | null
  onOpenTask: (task: OrcaTask) => void
  /** Slot for the phase approval / start actions (PhaseApprovalBar). */
  actions?: React.ReactNode
}

export function PhaseNode({
  phase,
  tasks,
  approval,
  onOpenTask,
  actions
}: Props): React.JSX.Element {
  const [open, setOpen] = useState(true)
  const stats = computePhaseStats(tasks)
  const taskIds = React.useMemo(() => tasks.map((t) => t.id), [tasks])
  const readiness = useTaskReadiness({ phaseId: phase.id, phaseTaskIds: taskIds })
  const progress = resolveProgress(phase, tasks)

  const onKeyDown = (e: React.KeyboardEvent): void => {
    if (e.key === 'ArrowRight' && !open) {
      e.preventDefault()
      setOpen(true)
    } else if (e.key === 'ArrowLeft' && open) {
      e.preventDefault()
      setOpen(false)
    }
  }

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className="rounded-md border border-border"
      data-testid={`phase-node-${phase.id}`}
    >
      <div className="flex flex-col gap-1.5 px-3 py-2">
        <div className="flex items-center gap-2">
          <CollapsibleTrigger
            onKeyDown={onKeyDown}
            className="flex min-w-0 flex-1 items-center gap-1.5 rounded text-left text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            data-testid={`phase-toggle-${phase.id}`}
          >
            {open ? (
              <ChevronDown className="size-3.5 shrink-0" aria-hidden />
            ) : (
              <ChevronRight className="size-3.5 shrink-0" aria-hidden />
            )}
            <span className="truncate">{phase.title}</span>
          </CollapsibleTrigger>
          {approval && <ApprovalStatusBadge status={approval.status} size="xs" />}
          <TaskStatusBadge status={phase.status} />
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Progress value={progress.value} className="h-1 w-24" aria-label={phase.title} />
          <span data-testid={`phase-summary-${phase.id}`}>
            {translate(
              'auto.components.request.plan.PhaseNode.summary',
              '{{done}}/{{total}} tasks done',
              {
                done: stats.done,
                total: stats.total
              }
            )}
            {progress.estimated &&
              ` (${translate('auto.components.request.plan.PlanSummaryHeader.estimated', 'estimated')})`}
          </span>
          {stats.blocked > 0 && (
            <span className="rounded border border-destructive/40 px-1.5 text-destructive">
              {translate('auto.components.request.plan.PhaseNode.blocked', '{{count}} blocked', {
                count: stats.blocked
              })}
            </span>
          )}
          {stats.running > 0 && (
            <span className="rounded border border-primary/40 px-1.5 text-primary">
              {translate('auto.components.request.plan.PhaseNode.running', '{{count}} running', {
                count: stats.running
              })}
            </span>
          )}
          <PhaseReadinessSummary summary={readiness.phaseSummary} />
        </div>
        {actions}
      </div>
      <CollapsibleContent className="border-t border-border px-1 py-1">
        {tasks.length === 0 ? (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">
            {translate('auto.components.request.plan.PhaseNode.empty', 'No tasks in this phase.')}
          </p>
        ) : (
          tasks.map((t) => <PlanTaskRow key={t.id} task={t} onOpen={onOpenTask} readiness={readiness.byTaskId[t.id] ?? null} />)
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}
