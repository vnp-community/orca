/**
 * PlanRunAllButton — FE-REQ-TASK-043-02
 *
 * Secondary "Approve and run all steps": opens the confirmation, and only its confirm sends the approval. The decision itself
 * (arguments, risk fields, busy lock) belongs to the parent's `onConfirm`, so there is still one approval path.
 *
 * @module components/request/plan/PlanRunAllButton
 */

import React, { useRef, useState } from 'react'
import { FastForward } from 'lucide-react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { RunAllStepsConfirmDialog } from './RunAllStepsConfirmDialog'
import type { RunAllDialogModel } from './plan-run-all-model'
import type { PlanDecisionOutcome } from '../../../hooks/usePlanDecision'

const T = 'auto.components.request.plan.PlanRunAllButton.'

type Props = RunAllDialogModel & {
  /** Same lock as the main Approve button (busy, denied, cooling down). */
  locked: boolean
  /** Why Approve is blocked right now; shown as the tooltip, like on the main button. */
  blockedReason?: string | null
  onConfirm: () => Promise<PlanDecisionOutcome>
}

export function PlanRunAllButton({
  stepCount,
  mentionsPreDeploy,
  locked,
  blockedReason = null,
  onConfirm
}: Props): React.JSX.Element {
  const [open, setOpen] = useState(false)
  const [confirming, setConfirming] = useState(false)
  // Why: a ref closes the double-click window that state (one render behind) leaves open.
  const sending = useRef(false)
  const openerRef = useRef<HTMLButtonElement>(null)

  const confirm = async (): Promise<void> => {
    if (sending.current) {
      return
    }
    sending.current = true
    setConfirming(true)
    try {
      const outcome = await onConfirm()
      // Why: on failure the bar shows the reason inline; the dialog has nothing more to add.
      setOpen(false)
      if (outcome.ok && outcome.runAllApplied === false) {
        toast.warning(
          translate(
            `${T}notApplied`,
            "This server didn't run the plan on its own. Approve each step as usual."
          )
        )
      }
    } finally {
      sending.current = false
      setConfirming(false)
    }
  }

  return (
    <>
      <Button
        ref={openerRef}
        size="sm"
        variant="outline"
        disabled={locked || blockedReason !== null}
        title={blockedReason ?? undefined}
        onClick={() => setOpen(true)}
        data-testid="plan-approve-run-all"
      >
        <FastForward className="size-3.5" aria-hidden />
        {translate(`${T}label`, 'Approve and run all steps')}
      </Button>
      <RunAllStepsConfirmDialog
        open={open}
        onOpenChange={setOpen}
        stepCount={stepCount}
        mentionsPreDeploy={mentionsPreDeploy}
        busy={confirming}
        onConfirm={() => void confirm()}
        onCloseAutoFocus={(event) => {
          event.preventDefault()
          openerRef.current?.focus()
        }}
      />
    </>
  )
}
