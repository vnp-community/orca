/**
 * PlanDriftReviewSheet — FE-REQ-TASK-036-07
 *
 * Expected-vs-actual table plus the two backend-defined outcomes: accept the
 * drift (approve) or return (reject => request goes back to the backlog). There
 * is no "cancel phase" action: no such RPC exists.
 *
 * @module components/request/readiness/PlanDriftReviewSheet
 */

import React, { useEffect, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { DecisionRationaleField } from '../decision/DecisionRationaleField'
import { isRationaleValid } from '../decision/decision-rules'
import { requestErrorMessage } from '../request-error-message'
import { callRequestRpc, type Result } from '../../../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../../../shared/request-rpc-methods'
import { parseImpactDrift } from '../../../../../shared/request-artifact-parsers'
import type { ImpactDrift } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.readiness.'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  phaseId: string
  phaseName?: string
  onAccept: (comment: string) => Promise<Result<unknown>>
  onReturn: (comment: string) => Promise<Result<unknown>>
}

export function PlanDriftReviewSheet({ open, onOpenChange, phaseId, phaseName, onAccept, onReturn }: Props): React.JSX.Element {
  const [drift, setDrift] = useState<ImpactDrift | null>(null)
  const [loadError, setLoadError] = useState(false)
  const [comment, setComment] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open || !phaseId) {return}
    let cancelled = false
    setLoadError(false)
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.IMPACT_DRIFT, { phaseId }).then((res) => {
      if (cancelled) {return}
      if (res.ok) {setDrift(parseImpactDrift((res.value as { drift?: unknown } | null)?.drift ?? res.value))}
      else {setLoadError(res.error.kind !== 'unsupported')}
    })
    return () => { cancelled = true }
  }, [open, phaseId])

  const valid = isRationaleValid(comment, true)
  const act = async (fn: (c: string) => Promise<Result<unknown>>): Promise<void> => {
    if (!valid || busy) {return}
    setBusy(true)
    setError(null)
    const res = await fn(comment.trim())
    setBusy(false)
    if (res.ok) {onOpenChange(false)} else {setError(requestErrorMessage(res.error.kind))}
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="gap-3 p-4 sm:max-w-2xl" data-testid="plan-drift-review-sheet">
        <SheetHeader className="p-0">
          <SheetTitle>{translate(`${T}drift.review`, 'Review and decide')}</SheetTitle>
          <SheetDescription>{phaseName ?? ''}</SheetDescription>
        </SheetHeader>
        {loadError ? <p role="alert" className="text-xs text-destructive">{translate(`${T}drift.loadError`, 'Could not load the drift details')}</p> : null}
        {drift && drift.items.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{translate(`${T}drift.task`, 'Task')}</TableHead>
                <TableHead>{translate(`${T}drift.expected`, 'Expected')}</TableHead>
                <TableHead>{translate(`${T}drift.actual`, 'Actual')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {drift.items.map((it) => (
                <TableRow key={it.taskId}>
                  <TableCell className="font-mono text-xs">{it.taskId}</TableCell>
                  <TableCell className="text-xs">{it.expected}</TableCell>
                  <TableCell className="text-xs">{it.actual}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <p className="text-xs text-muted-foreground">{translate(`${T}drift.noDetails`, 'No detail rows were provided')}</p>
        )}
        <p className="text-xs text-muted-foreground">{translate(`${T}drift.runningNote`, 'Tasks already running continue until they finish.')}</p>
        <DecisionRationaleField value={comment} onChange={setComment} required disabled={busy} />
        {error ? <p role="alert" className="text-xs text-destructive">{error}</p> : null}
        <div className="mt-auto flex flex-wrap gap-2">
          <Button size="sm" disabled={!valid || busy} onClick={() => void act(onAccept)}>{translate(`${T}drift.accept`, 'Accept the drift and continue')}</Button>
          <Button size="sm" variant="outline" disabled={!valid || busy} onClick={() => void act(onReturn)}>{translate(`${T}drift.return`, 'Return')}</Button>
        </div>
        <p className="text-[11px] text-muted-foreground">{translate(`${T}drift.returnNote`, 'Returning sends the request back to the backlog per policy.')}</p>
      </SheetContent>
    </Sheet>
  )
}
