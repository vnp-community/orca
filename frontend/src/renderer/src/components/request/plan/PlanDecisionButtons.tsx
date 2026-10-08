/**
 * PlanDecisionButtons — CR-REQ-021-04
 *
 * Approve / Reject pair shared by the plan, phase and pre_deploy gates. Rejection
 * needs a written reason (RejectReasonDialog, Mod+Enter submits).
 *
 * @module components/request/plan/PlanDecisionButtons
 */

import React, { useState } from 'react'
import { Check, Loader2, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { RejectReasonDialog } from '../solution/RejectReasonDialog'
import type { PlanDecision } from '../../../hooks/usePlanDecision'
import type { Approval } from '../../../../../shared/request-types'

type Props = {
  approval: Approval
  decision: PlanDecision
  approveLabel: string
  rejectLabel: string
  rejectTitle: string
  /** Offer "regenerate from this feedback" in the reject dialog. */
  offerRegenerate?: boolean
  testIdPrefix: string
  /** When set, Approve is locked and this text explains why (CR-REQ-028 decision gate). */
  approveBlockedReason?: string | null
  /** Risk-gate fields forwarded to `approval.approve` (FE-REQ-TASK-036-06). */
  approveExtras?: { viewedImpactDigest?: string; acceptedFindingIds?: string[] }
  /** REQUEST_RISK_APPROVER_NOT_ALLOWED: hide Approve, keep Reject. */
  hideApprove?: boolean
}

export function PlanDecisionButtons({
  approval,
  decision,
  approveLabel,
  rejectLabel,
  rejectTitle,
  offerRegenerate = false,
  testIdPrefix,
  approveBlockedReason = null,
  approveExtras,
  hideApprove = false
}: Props): React.JSX.Element {
  const [dialogOpen, setDialogOpen] = useState(false)
  const approving = decision.isBusy(`approve:${approval.id}`)
  const rejecting = decision.isBusy(`reject:${approval.id}`)
  const locked = approving || rejecting || decision.isDenied(approval.id)

  return (
    <div className="flex items-center gap-2">
      {hideApprove ? null : (
        <Button
          size="sm"
          disabled={locked || approveBlockedReason !== null}
          title={approveBlockedReason ?? undefined}
          onClick={() => void decision.approve(approval, undefined, approveExtras)}
          data-testid={`${testIdPrefix}-approve`}
        >
          {approving ? (
            <Loader2 className="size-3.5 animate-spin" aria-hidden />
          ) : (
            <Check className="size-3.5" aria-hidden />
          )}
          {approveLabel}
        </Button>
      )}
      <Button
        size="sm"
        variant="outline"
        disabled={locked}
        onClick={() => setDialogOpen(true)}
        data-testid={`${testIdPrefix}-reject`}
      >
        <X className="size-3.5" aria-hidden />
        {rejectLabel}
      </Button>
      <RejectReasonDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={rejectTitle}
        submitting={rejecting}
        offerRegenerate={offerRegenerate}
        onSubmit={(comment, options) =>
          decision.reject(approval, comment, { regenerate: options?.regenerate })
        }
      />
    </div>
  )
}
