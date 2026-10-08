/**
 * ReopenRequestDialog — CR-REQ-023-03
 *
 * Reopening sends the Request back to classification (CR-006), keeping the old
 * Solution/Plan/Tasks only as reference, so the copy says so.
 *
 * @module components/request/backlog/ReopenRequestDialog
 */

import React, { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle
} from '@/components/ui/dialog'
import { translate } from '@/i18n/i18n'
import { useRequestActions } from '../../../hooks/useRequestActions'
import { requestErrorMessage } from '../request-error-message'
import type { ReturnedFromStage } from '../../../../../shared/request-types'

const T = 'auto.components.request.backlog.'

export const STAGE_LABELS: Record<ReturnedFromStage, string> = {
  classification: 'Classification', analysis: 'Analysis', plan: 'Plan', phase: 'Phase', task: 'Task', unknown: '-'
}

export function stageLabel(stage: ReturnedFromStage): string {
  return stage === 'unknown' ? '-' : translate(`${T}ReturnedFromStage.${stage}`, STAGE_LABELS[stage])
}

export type ReopenTarget = { id: string; number: number; title: string; returnedFromStage: ReturnedFromStage }

export function ReopenRequestDialog({
  open,
  onOpenChange,
  request,
  onReopened
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  request: ReopenTarget
  onReopened: (requestId: string) => void
}): React.JSX.Element {
  const actions = useRequestActions()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (open) {setError(null)}
  }, [open])

  const confirm = async (): Promise<void> => {
    if (busy) {return}
    setBusy(true)
    setError(null)
    try {
      const result = await actions.reopen(request.id)
      if (result.ok) {
        onOpenChange(false)
        onReopened(request.id)
        return
      }
      const kind = result.error.kind
      if (kind === 'invalid_state' || kind === 'conflict' || kind === 'not_found') {
        // Someone else already handled it: neutral toast, drop the row.
        toast(translate(`${T}BacklogRow.alreadyHandled`, 'This request was already handled.'))
        onOpenChange(false)
        onReopened(request.id)
      } else if (kind === 'forbidden') {
        toast.error(requestErrorMessage('forbidden'))
      } else {
        setError(requestErrorMessage(kind))
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => (busy ? undefined : onOpenChange(next))}>
      <DialogContent data-testid="reopen-request-dialog">
        <DialogHeader>
          <DialogTitle>{translate(`${T}ReopenRequestDialog.title`, 'Reopen this request?')}</DialogTitle>
          <DialogDescription>
            {translate(
              `${T}ReopenRequestDialog.body`,
              'Request #{{number}} was returned from {{stage}} and will be classified again. Its Solution, Plan and Tasks are kept for reference.',
              { number: request.number, stage: stageLabel(request.returnedFromStage) }
            )}
          </DialogDescription>
        </DialogHeader>
        <p className="truncate text-sm font-medium text-foreground">{request.title}</p>
        {error && (
          <p role="alert" className="text-xs text-destructive">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
            {translate(`${T}ReopenRequestDialog.cancel`, 'Not now')}
          </Button>
          <Button autoFocus disabled={busy} onClick={() => void confirm()}>
            {busy && <Loader2 className="size-3.5 animate-spin" aria-hidden />}
            {translate(`${T}ReopenRequestDialog.confirm`, 'Reopen')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
