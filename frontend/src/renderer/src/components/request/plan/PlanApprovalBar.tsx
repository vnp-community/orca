/**
 * PlanApprovalBar — CR-REQ-021-04
 *
 * @module components/request/plan/PlanApprovalBar
 */

import React from 'react'
import { Loader2, RefreshCw } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { planDecisionErrorMessage } from './plan-decision-error-message'
import { PlanDecisionButtons } from './PlanDecisionButtons'
import type { PlanDecision } from '../../../hooks/usePlanDecision'
import type { Approval } from '../../../../../shared/request-types'

const P = 'auto.components.request.plan.PlanApprovalBar.'

type Props = {
  /** Latest plan / task_list approval, or null when none was opened yet. */
  approval: Approval | null
  decision: PlanDecision
  /** Why Approve is locked (no effective Decision yet); null/undefined = not blocked. */
  approveBlockedReason?: string | null
}

export function PlanApprovalBar({ approval, decision, approveBlockedReason = null }: Props): React.JSX.Element | null {
  const failure =
    decision.failure &&
    (decision.failure.scope === approval?.id || decision.failure.scope === 'plan')
      ? decision.failure
      : null
  const regenerating = decision.isBusy('generatePlan')
  const isTaskList = (approval?.subjectType as string | undefined) === 'task_list'

  // Approved plans are read-only: changing one means regenerating, which needs a new approval.
  if (approval?.status === 'approved') {
    return null
  }

  const regenerate = (
    <Button
      size="sm"
      variant="outline"
      disabled={regenerating}
      onClick={() => void decision.regeneratePlan()}
      data-testid="plan-regenerate"
    >
      {regenerating ? (
        <Loader2 className="size-3.5 animate-spin" aria-hidden />
      ) : (
        <RefreshCw className="size-3.5" aria-hidden />
      )}
      {translate(`${P}regenerate`, 'Regenerate plan')}
    </Button>
  )

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
      <div className="flex flex-wrap items-center gap-2">
        {approval?.status === 'pending' && (
          <PlanDecisionButtons
            approval={approval}
            decision={decision}
            testIdPrefix="plan"
            approveBlockedReason={approveBlockedReason}
            approveLabel={
              isTaskList
                ? translate(`${P}approveTaskList`, 'Approve task list')
                : translate(`${P}approve`, 'Approve plan')
            }
            rejectLabel={translate(`${P}reject`, 'Reject')}
            rejectTitle={translate(`${P}rejectTitle`, 'Reject plan')}
            offerRegenerate
          />
        )}
        {(approval?.status === 'pending' || approval?.status === 'rejected') && regenerate}
      </div>
      {decision.isDenied(approval?.id) && !failure && (
        <p role="alert" className="text-xs text-destructive">
          {translate(`${P}notApprover`, 'You are not allowed to approve this.')}
        </p>
      )}
      {failure && (
        <p role="alert" className="text-xs text-destructive" data-testid="plan-decision-error">
          {planDecisionErrorMessage(failure)}
        </p>
      )}
    </div>
  )
}
