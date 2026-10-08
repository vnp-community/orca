/**
 * RequestPlanTab — CR-REQ-021-05 (mounted by RequestDetailPane, CR-REQ-019-03)
 *
 * Read-only Plan → Phase → Task tree with approval gates. Plan/Phase status is derived
 * from their children (O1), so nothing here edits tasks.
 *
 * @module components/request/plan/RequestPlanTab
 */

import React, { Suspense, useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { useAppStore } from '../../../store'
import { usePlanTree } from '../../../hooks/usePlanTree'
import { usePlanDecision } from '../../../hooks/usePlanDecision'
import { REQUEST_FLOW_REGISTRY } from '../../../../../shared/request-flow-registry'
import { buildStageTimeline } from '../request-stage-timeline-model'
import { attachApprovals, computeExecutionGates } from './plan-approval-model'
import { canGeneratePlan, derivePlanViewState } from './plan-view-state'
import { planDecisionErrorMessage } from './plan-decision-error-message'
import { PhaseApprovalBar } from './PhaseApprovalBar'
import { PlanApprovalBar } from './PlanApprovalBar'
import { PlanGateChips } from './PlanGateChips'
import { PlanStates } from './PlanStates'
import { PlanSummaryHeader } from './PlanSummaryHeader'
import { PlanTree } from './PlanTree'
import { PlanDriftSection } from '../readiness/PlanDriftSection'
import { useDecisions } from '../../../hooks/useDecisions'
import { planBlockedByDecision } from '../decision/decision-rules'
import { GraphSkeleton } from '../../graph/GraphStates'
import type { OrcaRequest } from '../../../../../shared/request-types'

const GraphPanel = React.lazy(() => import('../../graph/GraphPanel').then((m) => ({ default: m.GraphPanel })))

export type RequestPlanTabProps = {
  request: OrcaRequest
  /** Call after any mutation so the detail pane refetches the request. */
  onChanged: () => void
}

export function RequestPlanTab({
  request,
  onChanged
}: RequestPlanTabProps): React.JSX.Element | null {
  const flowSupport = useAppStore((s) => s.requestFlowSupport)
  const setTaskExecutionGates = useAppStore((s) => s.setTaskExecutionGates)
  const { tree, approvals, isLoading, error, truncated, refetch } = usePlanTree(request)
  // Why: view choice lives in component state only (no localStorage) per CR-REQ-032.
  const [planView, setPlanView] = useState<'tree' | 'graph'>('tree')

  const onSettled = useCallback(() => {
    refetch()
    onChanged()
  }, [refetch, onChanged])
  const decision = usePlanDecision(request, onSettled)
  const requestDecisions = useDecisions(request.id)

  const approvalMap = useMemo(
    () => (tree ? attachApprovals(tree, approvals) : null),
    [tree, approvals]
  )
  const flow = request.type === 'unknown' ? null : REQUEST_FLOW_REGISTRY[request.type]
  const planApprovalRequired = flow?.gates.planApproval ?? false

  // Gates only exist while this tab is mounted; the backend re-checks on execute anyway.
  useEffect(() => {
    if (!tree?.plan || !approvalMap) {
      setTaskExecutionGates?.(request.id, {})
      return
    }
    setTaskExecutionGates?.(
      request.id,
      computeExecutionGates(tree, approvalMap, planApprovalRequired)
    )
  }, [tree, approvalMap, planApprovalRequired, request.id, setTaskExecutionGates])
  useEffect(
    () => () => setTaskExecutionGates?.(request.id, {}),
    [request.id, setTaskExecutionGates]
  )

  if (flow?.plan === 'none') {
    return null
  }

  const state = derivePlanViewState({
    request,
    tree,
    isLoading,
    error,
    flowSupported: flowSupport !== 'unsupported'
  })

  if (state !== 'ready' || !tree || !approvalMap) {
    const timeline = buildStageTimeline({
      type: request.type,
      size: request.size,
      status: request.status,
      returnedFromStage: request.returnedFromStage
    })
    const current = timeline.steps.find((s) => s.id === timeline.current)
    const generateFailure = decision.failure?.scope === 'plan' ? decision.failure : null
    return (
      <div className="h-full overflow-auto" data-testid="request-tab-plan">
        <PlanStates
          state={state === 'ready' ? 'error' : state}
          currentStepLabel={current ? translate(current.labelKey, current.id) : undefined}
          canGenerate={canGeneratePlan(request.status)}
          generating={decision.isBusy('generatePlan')}
          generateError={generateFailure ? planDecisionErrorMessage(generateFailure) : null}
          onGenerate={() => void decision.regeneratePlan()}
          onRetry={refetch}
          loadErrorKind={error}
        />
      </div>
    )
  }

  const planApproval = approvalMap.plan ?? approvalMap.taskList
  return (
    <div className="flex h-full flex-col overflow-auto" data-testid="request-tab-plan">
      <PlanSummaryHeader tree={tree} approval={planApproval} view={planView} onViewChange={setPlanView} />
      {truncated && (
        <div
          role="status"
          className="flex items-center gap-2 border-b border-border px-4 py-2 text-xs text-muted-foreground"
          data-testid="plan-truncated"
        >
          <AlertTriangle className="size-3.5" aria-hidden />
          {translate(
            'auto.components.request.plan.RequestPlanTab.truncated',
            'The task list was cut off; some tasks may be missing.'
          )}
          <Button size="sm" variant="ghost" onClick={refetch}>
            {translate('auto.components.request.plan.RequestPlanTab.reload', 'Reload')}
          </Button>
        </div>
      )}
      <PlanDriftSection request={request} approvals={approvals} phases={tree.phases} onChanged={onSettled} />
      <PlanApprovalBar
        approval={planApproval}
        decision={decision}
        approveBlockedReason={
          planBlockedByDecision(requestDecisions.decisions)
            ? translate('auto.components.request.decision.planBlocked', 'No decision recorded for the chosen option yet')
            : null
        }
      />
      {planView === 'graph' ? (
        <Suspense fallback={<GraphSkeleton />}>
          <GraphPanel
            request={request}
            subject={{ type: 'plan', id: request.planTaskId ?? request.id }}
            lensInitial="plan"
            impactAssessed={undefined}
          />
        </Suspense>
      ) : (
        <PlanTree
          tree={tree}
          approvals={approvalMap}
          renderPhaseActions={(phase, approval) => (
            <PhaseApprovalBar phase={phase} approval={approval} decision={decision} />
          )}
        />
      )}
      <PlanGateChips approvals={approvalMap} decision={decision} />
    </div>
  )
}
