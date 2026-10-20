/**
 * PlanApprovalBar — CR-REQ-021-04
 *
 * @module components/request/plan/PlanApprovalBar
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '../../../store'
import { supportsApproveRunAll } from '../../../store/slices/request'
import { ApprovalRiskGateSection, useApprovalRiskGate } from '../impact/ApprovalRiskGate'
import { planDecisionErrorMessage } from './plan-decision-error-message'
import { PlanDecisionButtons } from './PlanDecisionButtons'
import { PlanUnconfirmedAssumptionsNotice } from './PlanUnconfirmedAssumptionsNotice'
import { canOfferRunAll, runAllDialogModel } from './plan-run-all-model'
import type { PlanDecision } from '../../../hooks/usePlanDecision'
import type { RequestFlowView } from '../../../../../shared/request-execution-types'
import type { Approval, OrcaRequest } from '../../../../../shared/request-types'

const P = 'auto.components.request.plan.PlanApprovalBar.'

type Props = {
  /** Latest plan / task_list approval, or null when none was opened yet. */
  approval: Approval | null
  decision: PlanDecision
  /** Why Approve is locked (no effective Decision yet); null/undefined = not blocked. */
  approveBlockedReason?: string | null
  /** The continuous plan card words the decision as starting the run. */
  approveLabel?: string
  rejectLabel?: string
  /** The continuous plan card commits to running everything: Approve states `continuous` itself. */
  approveExecutionMode?: 'continuous'
  /** What "Approve and run all steps" needs; omitted where the choice makes no sense (the card, the Inbox). */
  runAll?: {
    request: Pick<OrcaRequest, 'executionMode'>
    flow: RequestFlowView | null
    stepCount: number
  }
}

export function PlanApprovalBar({
  approval,
  decision,
  approveBlockedReason = null,
  approveLabel,
  rejectLabel,
  approveExecutionMode,
  runAll
}: Props): React.JSX.Element | null {
  const failure =
    decision.failure &&
    (decision.failure.scope === approval?.id || decision.failure.scope === 'plan')
      ? decision.failure
      : null
  // Why: the wire parser folds task_list into plan; the original kind survives in rawSubjectType.
  const isTaskList =
    approval?.rawSubjectType === 'task_list' ||
    (approval?.subjectType as string | undefined) === 'task_list'
  const risk = useApprovalRiskGate(approval, failure?.error ?? null)
  const runAllSupported = useAppStore(supportsApproveRunAll)
  // Why: no step count means no honest "Run all N steps" question, so the choice stays hidden.
  const offerRunAll =
    runAll !== undefined &&
    runAll.stepCount > 0 &&
    canOfferRunAll({
      request: runAll.request,
      approval,
      flow: runAll.flow,
      supported: runAllSupported,
      approverHidden: risk.approverNotAllowed,
      alreadyPinned: decision.runAllPinned
    })

  // Approved plans are read-only: changing one means regenerating, which needs a new approval.
  if (approval?.status === 'approved') {
    return null
  }

  return (
    <div
      className="flex flex-col gap-2 border-b border-border px-4 py-3"
      data-testid="plan-approval-bar"
    >
      {approval?.status === 'rejected' && (
        <div data-testid="plan-rejected-notice" className="text-sm">
          <p className="font-medium text-destructive">
            {translate(`${P}rejected`, 'This plan was rejected.')}
          </p>
          {approval.comment && (
            <p className="mt-0.5 whitespace-pre-wrap text-muted-foreground">{approval.comment}</p>
          )}
        </div>
      )}
      {approval?.status === 'pending' && (
        <ApprovalRiskGateSection
          api={risk}
          approval={approval}
          gateName="plan"
          disabled={decision.isBusy(`approve:${approval.id}`)}
        />
      )}
      <div className="flex flex-wrap items-center gap-2">
        {approval?.status === 'pending' && (
          <PlanDecisionButtons
            approval={approval}
            decision={decision}
            testIdPrefix="plan"
            approveBlockedReason={approveBlockedReason ?? risk.blockedReason}
            approveExtras={risk.gate.approveExtras}
            hideApprove={risk.approverNotAllowed}
            approveExecutionMode={approveExecutionMode}
            runAll={
              offerRunAll
                ? runAllDialogModel({ stepCount: runAll.stepCount, flow: runAll.flow })
                : undefined
            }
            approveLabel={
              approveLabel ??
              (isTaskList
                ? translate(`${P}approveTaskList`, 'Approve task list')
                : translate(`${P}approve`, 'Approve plan'))
            }
            rejectLabel={rejectLabel ?? translate(`${P}reject`, 'Reject')}
            rejectTitle={translate(`${P}rejectTitle`, 'Reject plan')}
          />
        )}
      </div>
      {decision.isDenied(approval?.id) && !failure && (
        <p role="alert" className="text-xs text-destructive">
          {translate(`${P}notApprover`, 'You are not allowed to approve this.')}
        </p>
      )}
      {failure?.errorClass === 'unconfirmedAssumptions' && (
        <PlanUnconfirmedAssumptionsNotice message={failure.error.message ?? ''} />
      )}
      {failure && failure.errorClass !== 'unconfirmedAssumptions' && (
        <p role="alert" className="text-xs text-destructive" data-testid="plan-decision-error">
          {planDecisionErrorMessage(failure)}
        </p>
      )}
    </div>
  )
}
