/**
 * PhaseApprovalBar — CR-REQ-021-04
 *
 * @module components/request/plan/PhaseApprovalBar
 */

import React from 'react'
import { Loader2, Play } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { ApprovalRiskGateSection, useApprovalRiskGate } from '../impact/ApprovalRiskGate'
import { planDecisionErrorMessage } from './plan-decision-error-message'
import { PlanDecisionButtons } from './PlanDecisionButtons'
import type { PlanDecision } from '../../../hooks/usePlanDecision'
import type { Approval } from '../../../../../shared/request-types'
import type { OrcaTask } from '../../../../../shared/task-types'

const P = 'auto.components.request.plan.PhaseApprovalBar.'

type Props = {
  phase: OrcaTask
  approval: Approval | null
  decision: PlanDecision
}

export function PhaseApprovalBar({ phase, approval, decision }: Props): React.JSX.Element | null {
  const failure =
    decision.failure &&
    (decision.failure.scope === approval?.id || decision.failure.scope === phase.id)
      ? decision.failure
      : null
  const started = phase.status === 'in_progress' || phase.status === 'done'
  const approved = approval?.status === 'approved'
  const starting = decision.isBusy(`startPhase:${phase.id}`)
  const showStart = !started && (approved || approval?.status === 'pending')
  const risk = useApprovalRiskGate(approval, failure?.error ?? null)

  if (!approval && !showStart) {
    return null
  }

  return (
    <div className="flex flex-col gap-1.5" data-testid={`phase-approval-bar-${phase.id}`}>
      {approval?.status === 'pending' && (
        <ApprovalRiskGateSection
          api={risk}
          approval={approval}
          gateName="phase"
          disabled={decision.isBusy(`approve:${approval.id}`)}
        />
      )}
      <div className="flex flex-wrap items-center gap-2">
        {approval?.status === 'pending' && (
          <PlanDecisionButtons
            approval={approval}
            decision={decision}
            testIdPrefix={`phase-${phase.id}`}
            approveBlockedReason={risk.blockedReason}
            approveExtras={risk.gate.approveExtras}
            hideApprove={risk.approverNotAllowed}
            approveLabel={translate(`${P}approve`, 'Approve phase')}
            rejectLabel={translate(`${P}reject`, 'Reject')}
            rejectTitle={translate(`${P}rejectTitle`, 'Reject phase')}
          />
        )}
        {showStart && (
          <span
            title={approved ? undefined : translate(`${P}notApproved`, 'Phase is not approved yet')}
          >
            <Button
              size="sm"
              variant="outline"
              disabled={!approved || starting}
              onClick={() => void decision.startPhase(phase.id)}
              data-testid={`phase-${phase.id}-start`}
            >
              {starting ? (
                <Loader2 className="size-3.5 animate-spin" aria-hidden />
              ) : (
                <Play className="size-3.5" aria-hidden />
              )}
              {translate(`${P}start`, 'Start phase')}
            </Button>
          </span>
        )}
      </div>
      {approval?.status === 'rejected' && (
        <p className="text-xs text-muted-foreground" data-testid={`phase-${phase.id}-rejected`}>
          {translate(`${P}rejectedNote`, 'Phase rejected. The request may return to planning.')}
          {approval.comment ? ` ${approval.comment}` : ''}
        </p>
      )}
      {decision.isDenied(approval?.id) && !failure && (
        <p role="alert" className="text-xs text-destructive">
          {translate(`${P}notApprover`, 'You are not allowed to approve this.')}
        </p>
      )}
      {failure && (
        <p
          role="alert"
          className="text-xs text-destructive"
          data-testid={`phase-${phase.id}-error`}
        >
          {planDecisionErrorMessage(failure)}
        </p>
      )}
    </div>
  )
}
