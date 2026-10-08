/**
 * RiskOverrideMenu — FE-REQ-TASK-036-06
 *
 * Secondary, audited escape hatch. Only rendered when the backend says the
 * viewer may override; the client never infers permission.
 *
 * @module components/request/impact/RiskOverrideMenu
 */

import React, { useState } from 'react'
import { MoreHorizontal } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { RISK_OVERRIDE_REASON_MIN } from '../../../hooks/useImpactAssessment'
import { requestErrorMessage } from '../request-error-message'
import type { Result } from '../../../runtime/request-rpc-client'

const T = 'auto.components.request.impact.'

type Props = {
  gate: string
  /** From the backend (`viewerCan.override`); false/undefined hides the menu. */
  allowed: boolean | undefined
  onOverride: (input: { gate: string; reason: string }) => Promise<Result<unknown>>
}

export function RiskOverrideMenu({ gate, allowed, onOverride }: Props): React.JSX.Element | null {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState('')
  const [touched, setTouched] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  if (!allowed) {return null}
  const valid = reason.trim().length >= RISK_OVERRIDE_REASON_MIN

  const submit = async (): Promise<void> => {
    setTouched(true)
    if (!valid || busy) {return}
    setBusy(true)
    setError(null)
    const res = await onOverride({ gate, reason })
    setBusy(false)
    if (res.ok) {
      setOpen(false)
      setReason('')
      setTouched(false)
    } else {
      setError(res.error.kind === 'validation' ? translate(`${T}override.reasonRequired`, 'At least {{min}} characters', { min: RISK_OVERRIDE_REASON_MIN }) : requestErrorMessage(res.error.kind))
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size="icon-xs" variant="ghost" aria-label={translate(`${T}override.menu`, 'More gate options')}>
            <MoreHorizontal aria-hidden />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => setOpen(true)}>{translate(`${T}override.title`, 'Bypass the gate')}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Dialog open={open} onOpenChange={(next) => { if (!busy) {setOpen(next)} }}>
        <DialogContent data-testid="risk-override-dialog">
          <DialogHeader>
            <DialogTitle>{translate(`${T}override.title`, 'Bypass the gate')}</DialogTitle>
            <DialogDescription>{translate(`${T}override.description`, 'This is recorded in the audit log with your reason.')}</DialogDescription>
          </DialogHeader>
          <Label htmlFor="risk-override-reason" className="text-xs">{translate(`${T}override.reasonLabel`, 'Reason')}</Label>
          <Textarea
            id="risk-override-reason"
            rows={4}
            value={reason}
            disabled={busy}
            aria-invalid={touched && !valid ? true : undefined}
            onBlur={() => setTouched(true)}
            onChange={(e) => setReason(e.target.value)}
          />
          <p className={touched && !valid ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>
            {reason.trim().length}/{RISK_OVERRIDE_REASON_MIN}
          </p>
          {error ? <p role="alert" className="text-xs text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button variant="ghost" disabled={busy} onClick={() => setOpen(false)}>{translate(`${T}override.cancel`, 'Cancel')}</Button>
            <Button disabled={!valid || busy} onClick={() => void submit()}>{translate(`${T}override.submit`, 'Bypass gate')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
