/**
 * RequestReasonDialog — CR-REQ-019-03
 *
 * Confirmation dialog with a free-text reason, used by cancel / return to
 * backlog / change type. Submit is blocked while the reason is required but
 * blank, and while a submit is in flight (no double sends).
 *
 * @module components/request/RequestReasonDialog
 */

import React, { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { translate } from '@/i18n/i18n'
import { getScreenSubmitShortcutLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  reasonLabel: string
  submitLabel: string
  reasonRequired: boolean
  destructive?: boolean
  /** Extra controls rendered above the reason (e.g. a stage select). */
  children?: React.ReactNode
  /** Resolves true when the action succeeded; the dialog then closes. */
  onSubmit: (reason: string) => Promise<boolean>
}

export function RequestReasonDialog({
  open,
  onOpenChange,
  title,
  description,
  reasonLabel,
  submitLabel,
  reasonRequired,
  destructive,
  children,
  onSubmit
}: Props): React.JSX.Element {
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const trimmed = reason.trim()
  const blocked = submitting || (reasonRequired && trimmed.length === 0)

  const submit = async (): Promise<void> => {
    if (blocked) {return}
    setSubmitting(true)
    try {
      if (await onSubmit(trimmed)) {
        setReason('')
        onOpenChange(false)
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !submitting && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        {children}
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="request-reason-input">{reasonLabel}</Label>
          <Textarea
            id="request-reason-input"
            value={reason}
            rows={4}
            onChange={(e) => setReason(e.target.value)}
            onKeyDown={(e) => {
              if (isScreenSubmitShortcut(e)) {
                e.preventDefault()
                void submit()
              }
            }}
          />
        </div>
        <DialogFooter>
          <Button variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>
            {translate('auto.components.request.RequestReasonDialog.cancel', 'Cancel')}
          </Button>
          <Button
            variant={destructive ? 'destructive' : 'default'}
            disabled={blocked}
            onClick={() => void submit()}
            title={getScreenSubmitShortcutLabel()}
          >
            {submitLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
