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
import { usePlanAutoGeneration } from '../../../hooks/usePlanAutoGeneration'
import { usePlanDecision } from '../../../hooks/usePlanDecision'
import { supportsPlanAuto } from '../../../store/slices/request'
import { REQUEST_FLOW_REGISTRY } from '../../../../../shared/request-flow-registry'
import { buildStageTimeline } from '../request-stage-timeline-model'
import { attachApprovals, computeExecutionGates, listPlanWorkTasks } from './plan-approval-model'
import { canGeneratePlan, derivePlanViewState, type PlanDisplayView } from './plan-view-state'
import { planDecisionErrorMessage } from './plan-decision-error-message'
import { firstAiSetupCode } from '../request-ai-setup'
import { PhaseApprovalBar } from './PhaseApprovalBar'
import { PlanApprovalBar } from './PlanApprovalBar'
import { PlanGateChips } from './PlanGateChips'
import { PlanStates } from './PlanStates'
import { PlanSummaryHeader } from './PlanSummaryHeader'
import { PlanTree } from './PlanTree'
import { PlanApprovalCard } from './PlanApprovalCard'
import { PlanOutline } from './PlanOutline'
import { PlanDocumentsPane } from './documents/PlanDocumentsPane'
import { PlanRevisionSection } from './revision/PlanRevisionSection'
import { PlanVersionBar } from './revision/PlanVersionBar'
import { usePlanRevision } from './revision/usePlanRevision'
import { usePlanVersions } from './revision/usePlanVersions'
import { formatRequestRelativeTime } from '@/lib/request-relative-time'
import { PlanDriftSection } from '../readiness/PlanDriftSection'
import { TestSpecsPanel } from '../execution/TestSpecsPanel'
import { useTestSpecs } from '../../../hooks/useTestSpecs'
import { useDecisions } from '../../../hooks/useDecisions'
import { planBlockedByDecision } from '../decision/decision-rules'
import { GraphSkeleton } from '../../graph/GraphStates'
import type { RequestFlowView } from '../../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../../shared/request-types'

const GraphPanel = React.lazy(() =>
  import('../../graph/GraphPanel').then((m) => ({ default: m.GraphPanel }))
)

export type RequestPlanTabProps = {
  request: OrcaRequest
  /** Call after any mutation so the detail pane refetches the request. */
  onChanged: () => void
  /** The flow of the request's type (phases, gates); the built-in table answers when absent. */
  flow?: RequestFlowView | null
  /** Where the approved plan is followed; omitted hides the link. */
  onOpenExecution?: () => void
}

export function RequestPlanTab({
  request,
  onChanged,
  flow: flowView = null,
  onOpenExecution
}: RequestPlanTabProps): React.JSX.Element | null {
  const flowSupport = useAppStore((s) => s.requestFlowSupport)
  const setTaskExecutionGates = useAppStore((s) => s.setTaskExecutionGates)
  const { tree, approvals, isLoading, error, truncated, refetch } = usePlanTree(request)
  // Why: view choice lives in component state only (no localStorage) per CR-REQ-032.
  const [planView, setPlanView] = useState<PlanDisplayView>('tree')

  const onSettled = useCallback(() => {
    refetch()
    onChanged()
  }, [refetch, onChanged])
  const decision = usePlanDecision(request, onSettled)
  // Why: a refreshed tab has lost its local busy state, but the server run goes on and a second click must not look possible.
  const auto = usePlanAutoGeneration(request, onSettled, decision.regeneratePlan)
  const requestDecisions = useDecisions(request.id)
  const versions = usePlanVersions(request)
  // Test specs start with Plan generation, so they are shown here beside the Plan, not only once the Request executes.
  const testSpecs = useTestSpecs(request.id)

  const approvalMap = useMemo(
    () => (tree ? attachApprovals(tree, approvals) : null),
    [tree, approvals]
  )
  // Why: the backend's flow (request.flow) wins; the built-in table only answers until it arrives or on an older runtime.
  const flow = request.type === 'unknown' ? null : REQUEST_FLOW_REGISTRY[request.type]
  const planApprovalRequired = flowView
    ? flowView.startGate === 'plan' || flowView.startGate === 'task_list'
    : (flow?.gates.planApproval ?? false)
  const hasNoPlan = flowView ? flowView.planKind === 'none' : flow?.plan === 'none'
  const flowPlanKind = flowView?.planKind ?? flow?.plan
  const serverCommit = useAppStore(supportsPlanAuto)
  const revision = usePlanRevision({
    request,
    flowPlanKind,
    serverCommit,
    runState: auto,
    currentRevision: versions.list.current?.revision,
    viewingCurrent: !versions.viewingOlder,
    onSettled
  })
  // Why: a hotfix plan is one task made from the diagnosis; only an authored plan has documents or a proposal to read.
  const documentsView =
    flowPlanKind === 'single_task'
      ? undefined
      : request.engine === 'openspec'
        ? 'documents'
        : 'proposal'
  const selectVersion = (n: number): void => {
    versions.select(n)
    if (documentsView && n !== versions.list.current?.revision) {
      setPlanView('documents')
    }
  }

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

  if (hasNoPlan) {
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
        <div className="px-4 pt-3 empty:hidden">
          <TestSpecsPanel requestId={request.id} testSpecs={testSpecs} />
        </div>
        <PlanStates
          state={state === 'ready' ? 'error' : state}
          currentStepLabel={current ? translate(current.labelKey, current.id) : undefined}
          canGenerate={canGeneratePlan(request.status)}
          generating={decision.isBusy('generatePlan') || auto.busy}
          generateError={generateFailure ? planDecisionErrorMessage(generateFailure) : null}
          generateAiCode={firstAiSetupCode(generateFailure?.error.backendCode)}
          generateErrorCode={generateFailure?.error.backendCode}
          onGenerate={() => void decision.regeneratePlan()}
          onRetry={refetch}
          autoDisplay={auto.display}
          onRetryRun={() => void auto.retry.retry()}
          retryingRun={auto.retry.retrying}
          retryRunError={auto.retry.error}
          loadErrorKind={error}
        />
      </div>
    )
  }

  const planApproval = approvalMap.plan ?? approvalMap.taskList
  const decisionBlock = planBlockedByDecision(requestDecisions.decisions)
    ? translate(
        'auto.components.request.decision.planBlocked',
        'No decision recorded for the chosen option yet'
      )
    : null
  // Why: a plan that runs on its own has one approval; step_by_step keeps the per-step tree untouched (empty counts as step_by_step).
  const continuous = request.executionMode === 'continuous'
  const approvedReadonly = continuous && planApproval?.status === 'approved'
  const work = listPlanWorkTasks(tree)
  return (
    <div className="flex h-full flex-col overflow-auto" data-testid="request-tab-plan">
      <PlanSummaryHeader
        tree={tree}
        approval={planApproval}
        view={planView}
        onViewChange={setPlanView}
        documentsView={documentsView}
      />
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
      <PlanDriftSection
        request={request}
        approvals={approvals}
        phases={tree.phases}
        onChanged={onSettled}
      />
      {revision.busy ? null : continuous && planApproval?.status === 'pending' ? (
        <PlanApprovalCard
          approval={planApproval}
          decision={decision}
          flow={flowView}
          stepCount={flowView?.hasPhases ? tree.phases.length : 0}
          taskCount={work.length}
          approveBlockedReason={decisionBlock}
        />
      ) : (
        <PlanApprovalBar
          approval={planApproval}
          decision={decision}
          approveBlockedReason={decisionBlock}
          runAll={{ request, flow: flowView, stepCount: tree.phases.length }}
        />
      )}
      <PlanRevisionSection revision={revision} display={auto.display} engine={request.engine} />
      <PlanVersionBar versions={versions} view={planView} onSelect={selectVersion} />
      {approvedReadonly && (
        <div
          className="flex items-center gap-2 border-b border-border px-4 py-2 text-xs text-muted-foreground"
          data-testid="plan-approved-strip"
        >
          <span className="min-w-0 flex-1">
            {translate(
              'auto.components.request.plan.RequestPlanTab.approvedStrip',
              'Plan approved {{when}}. The agent runs it on its own.',
              {
                when: formatRequestRelativeTime(planApproval?.approvedAt ?? planApproval?.updatedAt)
              }
            )}
          </span>
          {onOpenExecution && (
            <Button
              size="xs"
              variant="outline"
              onClick={onOpenExecution}
              data-testid="plan-follow-in-execution"
            >
              {translate(
                'auto.components.request.plan.RequestPlanTab.followInExecution',
                'Follow in Execution'
              )}
            </Button>
          )}
        </div>
      )}
      {planView === 'documents' && documentsView ? (
        <PlanDocumentsPane request={request} versions={versions} />
      ) : planView === 'graph' ? (
        <Suspense fallback={<GraphSkeleton />}>
          <GraphPanel
            request={request}
            subject={{ type: 'plan', id: request.planTaskId ?? request.id }}
            lensInitial="plan"
            impactAssessed={undefined}
          />
        </Suspense>
      ) : continuous ? (
        <PlanOutline tree={tree} hasPhases={flowView?.hasPhases ?? false} />
      ) : (
        <PlanTree
          tree={tree}
          approvals={approvalMap}
          renderPhaseActions={(phase, approval) => (
            <PhaseApprovalBar phase={phase} approval={approval} decision={decision} />
          )}
        />
      )}
      {!continuous && <PlanGateChips approvals={approvalMap} decision={decision} />}
    </div>
  )
}
