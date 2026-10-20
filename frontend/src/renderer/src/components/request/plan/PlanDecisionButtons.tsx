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
import { PlanRunAllButton } from './PlanRunAllButton'
import type { RunAllDialogModel } from './plan-run-all-model'
import type { PlanDecision } from '../../../hooks/usePlanDecision'
import type { Approval } from '../../../../../shared/request-types'

type Props = {
  approval: Approval
  decision: PlanDecision
  approveLabel: string
  rejectLabel: string
  rejectTitle: string
  testIdPrefix: string
  /** When set, Approve is locked and this text explains why (CR-REQ-028 decision gate). */
  approveBlockedReason?: string | null
  /** Risk-gate fields forwarded to `approval.approve` (FE-REQ-TASK-036-06). */
  approveExtras?: { viewedImpactDigest?: string; acceptedFindingIds?: string[] }
  /** REQUEST_RISK_APPROVER_NOT_ALLOWED: hide Approve, keep Reject. */
  hideApprove?: boolean
  /** CR-REQ-043: the main Approve states the mode explicitly instead of leaving it to the project setting. */
  approveExecutionMode?: 'continuous'
  /** Adds "Approve and run all steps" between Approve and Reject; the caller already decided it may be offered. */
  runAll?: RunAllDialogModel
}

export function PlanDecisionButtons({
  approval,
  decision,
  approveLabel,
  rejectLabel,
  rejectTitle,
  testIdPrefix,
  approveBlockedReason = null,
  approveExtras,
  hideApprove = false,
  approveExecutionMode,
  runAll
}: Props): React.JSX.Element {
  const [dialogOpen, setDialogOpen] = useState(false)
  const approving = decision.isBusy(`approve:${approval.id}`)
  const rejecting = decision.isBusy(`reject:${approval.id}`)
  const locked = approving || rejecting || decision.isDenied(approval.id) || decision.coolingDown

  return (
    <div className="flex flex-wrap items-center gap-2">
      {hideApprove ? null : (
        <Button
          size="sm"
          disabled={locked || approveBlockedReason !== null}
          title={approveBlockedReason ?? undefined}
          onClick={() =>
            void decision.approve(
              approval,
              undefined,
              approveExtras,
              approveExecutionMode ? { executionMode: approveExecutionMode } : undefined
            )
          }
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
      {runAll && !hideApprove && (
        <PlanRunAllButton
          {...runAll}
          locked={locked}
          blockedReason={approveBlockedReason}
          onConfirm={() =>
            decision.approve(approval, undefined, approveExtras, { executionMode: 'continuous' })
          }
        />
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
        onSubmit={(comment) => decision.reject(approval, comment)}
      />
    </div>
  )
}
