/**
 * RunAllStepsConfirmDialog — FE-REQ-TASK-043-02
 *
 * The one place that says what "run all steps" commits the approver to: every step starts with their access and the run only
 * stops for the reasons listed. Closing it (Esc, backdrop, "Not now") sends nothing.
 *
 * @module components/request/plan/RunAllStepsConfirmDialog
 */

import React from 'react'
import { Loader2 } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import type { RunAllDialogModel } from './plan-run-all-model'

const T = 'auto.components.request.plan.RunAllStepsConfirmDialog.'

type Props = RunAllDialogModel & {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** True while the approval is being sent; both buttons lock and the dialog cannot be dismissed. */
  busy: boolean
  onConfirm: () => void
  /** Radix only restores focus to a DialogTrigger; this dialog is opened by state, so the opener does it. */
  onCloseAutoFocus?: (event: Event) => void
}

export function RunAllStepsConfirmDialog({
  open,
  onOpenChange,
  stepCount,
  mentionsPreDeploy,
  busy,
  onConfirm,
  onCloseAutoFocus
}: Props): React.JSX.Element {
  const preDeploy = mentionsPreDeploy
    ? translate(`${T}preDeploy`, ', or it reaches the pre-deploy approval')
    : ''
  return (
    <Dialog open={open} onOpenChange={(next) => (busy ? undefined : onOpenChange(next))}>
      <DialogContent
        showCloseButton={!busy}
        onCloseAutoFocus={onCloseAutoFocus}
        data-testid="run-all-confirm-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            {translate(`${T}title`, 'Run all {{n}} steps without asking again?', {
              n: stepCount
            })}
          </DialogTitle>
          <DialogDescription asChild>
            <div className="flex flex-col gap-2">
              <p>
                {translate(
                  `${T}bodyStart`,
                  'Orca starts each step as soon as the previous one finishes, using your access.'
                )}
              </p>
              <p>
                {translate(
                  `${T}bodyStops`,
                  'It stops only when the agent needs information from you, a task keeps failing (the request then goes back to the backlog), a change drifts from the plan and needs a review{{preDeploy}}. You can stop the run at any time.',
                  { preDeploy }
                )}
              </p>
            </div>
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => onOpenChange(false)}
            data-testid="run-all-cancel"
          >
            {translate(`${T}cancel`, 'Not now')}
          </Button>
          <Button disabled={busy} onClick={onConfirm} data-testid="run-all-confirm">
            {busy && <Loader2 className="size-4 animate-spin" aria-hidden />}
            {translate(`${T}confirm`, 'Run all steps')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
