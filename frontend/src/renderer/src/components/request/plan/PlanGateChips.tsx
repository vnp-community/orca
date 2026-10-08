/**
 * PlanGateChips — CR-REQ-021-04
 *
 * Gate overview (plan, phases, pre_deploy) plus an action block for pending pre_deploy.
 *
 * @module components/request/plan/PlanGateChips
 */

import React from 'react'
import { ShieldCheck } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { ApprovalStatusBadge } from '../ApprovalStatusBadge'
import { planDecisionErrorMessage } from './plan-decision-error-message'
import { PlanDecisionButtons } from './PlanDecisionButtons'
import type { ApprovalMap } from './plan-approval-model'
import type { PlanDecision } from '../../../hooks/usePlanDecision'

const P = 'auto.components.request.plan.PlanGateChips.'

type Props = {
  approvals: ApprovalMap
  decision: PlanDecision
}

export function PlanGateChips({ approvals, decision }: Props): React.JSX.Element | null {
  const planApproval = approvals.plan ?? approvals.taskList
  const phaseApprovals = Object.values(approvals.byPhaseId).filter((a) => a !== null)
  if (!planApproval && phaseApprovals.length === 0 && approvals.preDeployList.length === 0) {
    return null
  }

  const phasesApproved = phaseApprovals.filter((a) => a?.status === 'approved').length
  const pendingPreDeploy = approvals.preDeployList.filter((a) => a.status === 'pending')
  const failure =
    decision.failure && pendingPreDeploy.some((a) => a.id === decision.failure?.scope)
      ? decision.failure
      : null

  return (
    <div
      className="flex flex-col gap-2 border-t border-border px-4 py-3"
      data-testid="plan-gate-chips"
    >
      <div className="flex flex-wrap items-center gap-2 text-xs">
        {planApproval && (
          <span className="inline-flex items-center gap-1" data-testid="gate-chip-plan">
            <span className="text-muted-foreground">{translate(`${P}plan`, 'Plan')}</span>
            <ApprovalStatusBadge status={planApproval.status} size="xs" />
          </span>
        )}
        {phaseApprovals.length > 0 && (
          <span
            className="inline-flex items-center gap-1 text-muted-foreground"
            data-testid="gate-chip-phase"
          >
            <ShieldCheck className="size-3" aria-hidden />
            {translate(`${P}phases`, 'Phases approved {{approved}}/{{total}}', {
              approved: phasesApproved,
              total: phaseApprovals.length
            })}
          </span>
        )}
        {approvals.preDeployList.map((a) => (
          <span
            key={a.id}
            className="inline-flex items-center gap-1"
            data-testid={`gate-chip-pre-deploy-${a.id}`}
          >
            <span className="text-muted-foreground">
              {translate(`${P}preDeploy`, 'Pre-deploy')}
            </span>
            <ApprovalStatusBadge status={a.status} size="xs" />
          </span>
        ))}
      </div>
      {pendingPreDeploy.map((a) => (
        <div
          key={a.id}
          className="flex flex-col gap-1.5 rounded-md border border-border p-2"
          data-testid={`pre-deploy-actions-${a.id}`}
        >
          <p className="text-xs font-medium">
            {translate(`${P}preDeployPending`, 'Pre-deploy approval needed')}
          </p>
          <PlanDecisionButtons
            approval={a}
            decision={decision}
            testIdPrefix={`pre-deploy-${a.id}`}
            approveLabel={translate(`${P}approvePreDeploy`, 'Approve pre-deploy')}
            rejectLabel={translate(`${P}reject`, 'Reject')}
            rejectTitle={translate(`${P}rejectTitle`, 'Reject pre-deploy')}
          />
        </div>
      ))}
      {failure && (
        <p role="alert" className="text-xs text-destructive">
          {planDecisionErrorMessage(failure)}
        </p>
      )}
    </div>
  )
}
