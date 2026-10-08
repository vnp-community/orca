/**
 * RejectReasonDialog — CR-REQ-020-04
 *
 * Generic "reason required" dialog. Independent of Solution so Plan/Phase
 * rejection (SOL-021/022) can reuse it. API is stable: do not rename props.
 *
 * @module components/request/solution/RejectReasonDialog
 */

import React, { useEffect, useId, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { ShortcutKeyCombo } from '@/components/ShortcutKeyCombo'
import { getScreenSubmitModifierLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import { requestErrorMessage } from '../request-error-message'
import { REJECT_REASON_MIN_LENGTH, validateRejectReason } from './solution-view-model'
import type { RequestRpcError } from '../../../../../shared/request-errors'

export type RejectReasonSubmitResult = { ok: boolean; error?: Pick<RequestRpcError, 'kind'> & Partial<RequestRpcError> }

export type RejectReasonDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  title?: string
  /** Resolves ok:true to close the dialog; ok:false keeps it open and shows the error. */
  onSubmit: (comment: string, options?: { regenerate: boolean }) => Promise<RejectReasonSubmitResult>
  submitting?: boolean
  /** Show the "regenerate from this feedback" checkbox (default on when shown). */
  offerRegenerate?: boolean
}

export function RejectReasonDialog({
  open,
  onOpenChange,
  title,
  onSubmit,
  submitting = false,
  offerRegenerate = false
}: RejectReasonDialogProps): React.JSX.Element {
  const [comment, setComment] = useState('')
  const [touched, setTouched] = useState(false)
  const [regenerate, setRegenerate] = useState(true)
  const [busy, setBusy] = useState(false)
  const [errorKind, setErrorKind] = useState<string | null>(null)
  const inFlight = useRef(false)
  const fieldId = useId()

  useEffect(() => {
    if (open) {
      setComment('')
      setTouched(false)
      setRegenerate(true)
      setErrorKind(null)
    }
  }, [open])

  const validity = validateRejectReason(comment)
  const locked = submitting || busy
  const showRequired = touched && !validity.ok

  const submit = async (): Promise<void> => {
    setTouched(true)
    // Why: a rejection without a reason must never reach the backend.
    if (!validateRejectReason(comment).ok || inFlight.current || submitting) {return}
    inFlight.current = true
    setBusy(true)
    setErrorKind(null)
    try {
      const result = await onSubmit(comment.trim(), { regenerate: offerRegenerate && regenerate })
      if (result.ok) {onOpenChange(false)}
      else {setErrorKind(result.error?.kind ?? 'unknown')}
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => (locked ? undefined : onOpenChange(next))}>
      <DialogContent data-testid="reject-reason-dialog">
        <DialogHeader>
          <DialogTitle>
            {title ?? translate('auto.components.request.RejectReasonDialog.title', 'Reject')}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'auto.components.request.RejectReasonDialog.description',
              'Explain why. The reason is recorded and shown to the team.'
            )}
          </DialogDescription>
        </DialogHeader>

        <Textarea
          id={fieldId}
          autoFocus
          value={comment}
          disabled={locked}
          rows={5}
          aria-invalid={showRequired}
          aria-describedby={`${fieldId}-hint`}
          placeholder={translate(
            'auto.components.request.RejectReasonDialog.placeholder',
            'Why is this being rejected?'
          )}
          onChange={(e) => setComment(e.target.value)}
          onBlur={() => setTouched(true)}
          onKeyDown={(e) => {
            if (isScreenSubmitShortcut(e)) {
              e.preventDefault()
              void submit()
            }
          }}
        />
        <p
          id={`${fieldId}-hint`}
          role={showRequired ? 'alert' : undefined}
          className={showRequired ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}
        >
          {showRequired
            ? translate('auto.components.request.RejectReasonDialog.required', 'A reason is required (at least {{min}} characters).', {
                min: REJECT_REASON_MIN_LENGTH
              })
            : `${validity.length} / ${REJECT_REASON_MIN_LENGTH}+`}
        </p>

        {offerRegenerate && (
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={regenerate}
              disabled={locked}
              onCheckedChange={(v) => setRegenerate(v === true)}
            />
            {translate(
              'auto.components.request.RejectReasonDialog.regenerate',
              'Regenerate based on this feedback'
            )}
          </label>
        )}

        {errorKind && (
          <p role="alert" className="text-sm text-destructive">
            {requestErrorMessage(errorKind)}
          </p>
        )}

        <DialogFooter className="items-center">
          <span className="mr-auto inline-flex items-center gap-1 text-xs text-muted-foreground">
            <ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']} />
          </span>
          <Button variant="outline" disabled={locked} onClick={() => onOpenChange(false)}>
            {translate('auto.components.request.RejectReasonDialog.cancel', 'Cancel')}
          </Button>
          <Button variant="destructive" disabled={locked || !validity.ok} onClick={() => void submit()}>
            {locked && <Loader2 className="size-4 animate-spin" aria-hidden />}
            {translate('auto.components.request.RejectReasonDialog.submit', 'Reject')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
