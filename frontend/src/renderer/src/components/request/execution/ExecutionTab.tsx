/**
 * ExecutionTab — FE-REQ-TASK-037-05
 *
 * Follows a run after the plan is approved: summary strip, what needs the viewer, the steps and tasks, and the engine's
 * ending (Handover for OpenSpec). Before the run starts it says what it waits for; a runtime without the summary channel
 * never shows this tab (the pane hides it).
 *
 * @module components/request/execution/ExecutionTab
 */

import React, { useCallback, useMemo } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlanTree } from '../../../hooks/usePlanTree'
import { usePlanDecision } from '../../../hooks/usePlanDecision'
import { requestErrorMessage } from '../request-error-message'
import { PlanDriftSection } from '../readiness/PlanDriftSection'
import { ExecutionFailureCard } from './ExecutionFailureCard'
import { ExecutionNeedsYou } from './ExecutionNeedsYou'
import { ExecutionStepList } from './ExecutionStepList'
import { ExecutionSummaryStrip } from './ExecutionSummaryStrip'
import { ExecutionTaskSheet, useExecutionTaskSheet } from './ExecutionTaskSheet'
import { HandoverRow } from './HandoverRow'
import { TestSpecsPanel } from './TestSpecsPanel'
import { useTestSpecs } from '../../../hooks/useTestSpecs'
import { buildExecutionSteps } from './execution-step-model'
import { runModeLabelOf } from './execution-run-mode-model'
import { listPlanWorkTasks } from '../plan/plan-approval-model'
import type { RequestFlowView } from '../../../../../shared/request-execution-types'
import type { UseExecutionSummaryResult } from '../../../hooks/useExecutionSummary'
import type { OrcaRequest, RequestStatus } from '../../../../../shared/request-types'

const T = 'auto.components.request.execution.ExecutionTab.'

/** Statuses before any task can run. */
const NOT_STARTED: ReadonlySet<RequestStatus> = new Set([
  'submitted',
  'classifying',
  'awaiting_type_confirmation',
  'analyzing',
  'awaiting_analysis_approval',
  'planning',
  'awaiting_plan_approval'
])

export type ExecutionTabProps = {
  request: OrcaRequest
  /** The flow of the request's type; its phases decide whether "asks before each step" is worth saying. */
  flow?: RequestFlowView | null
  execution: UseExecutionSummaryResult
  busy: boolean
  /** Call after any mutation so the pane refetches the request. */
  onChanged: () => void
  onOpenPlan?: () => void
  onReopen: () => void
  /** Opens the pane's cancel confirmation. */
  onStop: () => void
  onAnswerClarification: () => void
}

export function ExecutionTab({
  request,
  flow = null,
  execution,
  busy,
  onChanged,
  onOpenPlan,
  onReopen,
  onStop,
  onAnswerClarification
}: ExecutionTabProps): React.JSX.Element {
  const { summary, isLoading, error } = execution
  const { tree, approvals, refetch } = usePlanTree(request)
  const onSettled = useCallback(() => {
    refetch()
    execution.refetch()
    onChanged()
  }, [refetch, execution, onChanged])
  const decision = usePlanDecision(request, onSettled)
  const sheet = useExecutionTaskSheet()
  const testSpecs = useTestSpecs(request.id)

  const steps = useMemo(
    () => (tree ? buildExecutionSteps(tree, summary?.currentPhaseId) : []),
    [tree, summary?.currentPhaseId]
  )
  const openTaskById = useCallback(
    (taskId: string) => {
      const task = tree ? listPlanWorkTasks(tree).find((t) => t.id === taskId) : undefined
      if (task) {
        sheet.openTask(task)
      }
    },
    [tree, sheet]
  )

  if (!summary) {
    if (error) {
      return (
        <div
          role="alert"
          className="m-4 flex items-center gap-2 text-sm"
          data-testid="request-tab-execution"
        >
          <span>{requestErrorMessage(error)}</span>
          <Button size="xs" variant="outline" onClick={execution.refetch}>
            {translate('auto.components.request.error.retry', 'Retry')}
          </Button>
        </div>
      )
    }
    return (
      <div
        className="flex flex-col gap-2 p-4"
        aria-busy={isLoading}
        data-testid="request-tab-execution"
      >
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }

  const engine = summary.engine ?? request.engine ?? 'native'
  const stopped = request.status === 'request_backlog'
  const failedOut = summary.failed.find((f) => f.attempts >= f.maxAttempts) ?? summary.failed[0]
  const notStarted = NOT_STARTED.has(request.status) && summary.needsYou.length === 0
  const hasRun = !NOT_STARTED.has(request.status) || summary.progress.tasksDone > 0

  return (
    <div className="flex flex-col gap-3 p-4" data-testid="request-tab-execution">
      {notStarted && !hasRun && (
        <div
          className="flex flex-col items-start gap-2 text-sm"
          data-testid="execution-not-started"
        >
          <p className="text-muted-foreground">
            {translate(`${T}notStarted`, 'Not started. Waiting for plan approval.')}
          </p>
          {onOpenPlan && (
            <Button size="xs" variant="outline" onClick={onOpenPlan}>
              {translate(`${T}openPlan`, 'Open Plan')}
            </Button>
          )}
        </div>
      )}
      {(hasRun || summary.needsYou.length > 0) && (
        <>
          {hasRun && (
            <ExecutionSummaryStrip
              status={request.status}
              summary={summary}
              onStop={request.status === 'executing' ? onStop : undefined}
              runMode={runModeLabelOf(summary.executionMode, flow?.hasPhases)}
            />
          )}
          {tree && (
            <PlanDriftSection
              request={request}
              approvals={approvals}
              phases={tree.phases}
              onChanged={onSettled}
            />
          )}
          <ExecutionNeedsYou
            items={summary.needsYou.filter((n) => !(stopped && n.kind === 'failed_task'))}
            approvals={approvals}
            decision={decision}
            onAnswerClarification={onAnswerClarification}
            onOpenTask={openTaskById}
          />
          {stopped && failedOut && (
            <ExecutionFailureCard
              failed={failedOut}
              progress={summary.progress}
              busy={busy}
              onReopen={onReopen}
              onViewLog={openTaskById}
              onCancel={onStop}
              request={request}
              onChanged={onChanged}
            />
          )}
          {hasRun && (
            <>
              <ExecutionStepList steps={steps} summary={summary} onOpenTask={sheet.openTask} />
              <HandoverRow engine={engine} handover={summary.handover} />
            </>
          )}
        </>
      )}
      <TestSpecsPanel requestId={request.id} testSpecs={testSpecs} />
      <ExecutionTaskSheet open={sheet.open} onOpenChange={sheet.onOpenChange} />
    </div>
  )
}
