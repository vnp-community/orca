/**
 * HighRiskDecisionConfirmDialog — FE-REQ-TASK-036-04
 *
 * Second confirmation for `Decision.riskLevel === 'high'`: retype the option
 * title (paste allowed). Not destructive-styled: nothing is lost, it is a
 * deliberate-attention step. Cancel is silent (no key chip).
 *
 * @module components/request/decision/HighRiskDecisionConfirmDialog
 */

import React, { useEffect, useId, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { requestErrorMessage } from '../request-error-message'
import { matchesConfirmation } from './decision-rules'
import type { Result } from '../../../runtime/request-rpc-client'
import type { Decision } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.decision.'

type Props = {
  open: boolean
  decision: Decision
  optionTitle: string
  reasons: string[]
  onConfirm: (text: string) => Promise<Result<unknown>>
  onCancel: () => void
}

export function HighRiskDecisionConfirmDialog({ open, decision, optionTitle, reasons, onConfirm, onCancel }: Props): React.JSX.Element {
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inFlight = useRef(false)
  const id = useId()
  const matches = matchesConfirmation(text, optionTitle)

  useEffect(() => {
    if (open) {
      setText('')
      setError(null)
    }
  }, [open, decision.id])

  const submit = async (): Promise<void> => {
    if (!matches || inFlight.current) {return}
    inFlight.current = true
    setBusy(true)
    setError(null)
    try {
      // Why: send the user's text verbatim; the server applies the same normalization.
      const res = await onConfirm(text)
      if (!res.ok) {
        setError(
          res.error.kind === 'validation'
            ? translate(`${T}confirmMismatch`, 'The text does not match the option name')
            : requestErrorMessage(res.error.kind)
        )
      }
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next && !busy) {onCancel()} }}>
      <DialogContent data-testid="high-risk-decision-dialog">
        <DialogHeader>
          <DialogTitle>{translate(`${T}confirmTitle`, 'Confirm a high-risk choice')}</DialogTitle>
          <DialogDescription>
            {translate(`${T}confirmBody`, 'This choice was flagged as high risk. Review the reasons, then type the option name to confirm.')}
          </DialogDescription>
        </DialogHeader>
        {reasons.length > 0 ? (
          <ul className="list-disc space-y-0.5 pl-5 text-sm">
            {reasons.map((r, i) => <li key={i}>{r}</li>)}
          </ul>
        ) : null}
        <div className="flex flex-col gap-1">
          <Label htmlFor={id} className="text-xs">
            {translate(`${T}confirmInputLabel`, 'Type "{{title}}" to confirm', { title: optionTitle })}
          </Label>
          <Input
            id={id}
            autoFocus
            value={text}
            disabled={busy}
            aria-invalid={error ? true : undefined}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
                e.preventDefault()
                void submit()
              }
            }}
          />
          {error ? <p role="alert" className="text-xs text-destructive">{error}</p> : null}
        </div>
        <DialogFooter>
          <Button variant="ghost" disabled={busy} onClick={onCancel}>
            {translate(`${T}cancel`, 'Cancel')}
          </Button>
          <Button disabled={!matches || busy} onClick={() => void submit()}>
            {busy ? <Loader2 className="size-4 animate-spin" aria-hidden /> : null}
            {translate(`${T}confirmSubmit`, 'Confirm choice')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
