/**
 * QualityWaivePopover.tsx — FE-CV-TASK-087-14
 *
 * Waive a check finding (reason 1..1000 and an expiry of at most 30 days are both required) or
 * remove an existing waiver. Waiving never changes the gate locally; the hook reloads it.
 * Submit with Mod+Enter (platform convention); Esc closes.
 *
 * @module components/review-map/quality/findings/QualityWaivePopover
 */

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Button } from '../../../ui/button'
import { Input } from '../../../ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '../../../ui/popover'
import { Textarea } from '../../../ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '../../../ui/toggle-group'
import { ShortcutKeyCombo } from '../../../ShortcutKeyCombo'
import { getScreenSubmitModifierLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'
import { WAIVE_REASON_MAX_LENGTH } from '../../../../hooks/useQualityWaive'
import type { UseQualityWaiveResult } from '../../../../hooks/useQualityWaive'
import { qf } from './quality-findings-copy'
import {
  WAIVE_EXPIRY_DAY_CHOICES,
  resolveWaiveExpiry,
  waiveDateInputBounds
} from './quality-waive-expiry-options'
import type { WaiveExpiryChoice } from './quality-waive-expiry-options'

const systemNow = (): Date => new Date()

export type QualityWaivePopoverProps = {
  finding: QualityFinding
  waive: UseQualityWaiveResult
  /** Test seam for the clock. */
  now?: () => Date
}

export function QualityWaivePopover({
  finding,
  waive,
  now = systemNow
}: QualityWaivePopoverProps): React.JSX.Element {
  const busy = waive.busyFingerprint === finding.fingerprint
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState('')
  const [choice, setChoice] = useState<WaiveExpiryChoice | null>(null)
  const [error, setError] = useState<string | null>(null)
  const reasonRef = useRef<HTMLTextAreaElement | null>(null)
  // Why: recomputed each time the popover opens so the 30-day window follows the clock.
  // oxlint-disable-next-line react-hooks/exhaustive-deps
  const bounds = useMemo(() => waiveDateInputBounds(now()), [now, open])

  useEffect(() => {
    if (open) {
      reasonRef.current?.focus()
    } else {
      setError(null)
    }
  }, [open])

  if (finding.waiver) {
    return (
      <Button
        type="button"
        variant="ghost"
        size="xs"
        disabled={busy}
        onClick={async () => {
          const outcome = await waive.revoke(finding)
          setError(outcome.ok ? null : outcome.message)
        }}
        title={error ?? undefined}
      >
        {qf('waiveRevoke')}
      </Button>
    )
  }

  const submit = async (): Promise<void> => {
    if (busy) {
      return
    }
    const trimmed = reason.trim()
    if (trimmed === '' || trimmed.length > WAIVE_REASON_MAX_LENGTH) {
      setError(qf('waiveErrReasonRequired'))
      return
    }
    const expiry = resolveWaiveExpiry(choice, now())
    if (!expiry.ok) {
      setError(expiry.reason === 'past' ? qf('waiveErrExpiryPast') : qf('waiveErrExpiryRequired'))
      return
    }
    const outcome = await waive.waive(finding, { reason: trimmed, expiresAt: expiry.expiresAt })
    if (outcome.ok) {
      setOpen(false)
      setReason('')
      setChoice(null)
    } else if (outcome.message) {
      setError(outcome.message)
    }
  }

  const daysValue = choice?.kind === 'days' ? String(choice.days) : ''
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="xs">
          {qf('waiveOpen')}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="flex w-80 flex-col gap-2"
        onOpenAutoFocus={(event) => event.preventDefault()}
        onKeyDown={(event) => {
          if (isScreenSubmitShortcut(event)) {
            event.preventDefault()
            void submit()
          }
        }}
      >
        <label className="text-xs font-medium" htmlFor={`waive-reason-${finding.fingerprint}`}>
          {qf('waiveReasonLabel')}
        </label>
        <Textarea
          id={`waive-reason-${finding.fingerprint}`}
          ref={reasonRef}
          value={reason}
          maxLength={WAIVE_REASON_MAX_LENGTH}
          placeholder={qf('waiveReasonPlaceholder')}
          onChange={(event) => setReason(event.target.value)}
        />
        <span className="self-end text-xs text-muted-foreground tabular-nums">
          {qf('waiveReasonCount', { count: reason.length, max: WAIVE_REASON_MAX_LENGTH })}
        </span>
        <span className="text-xs font-medium">{qf('waiveExpiryLabel')}</span>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={daysValue}
          onValueChange={(value) => setChoice(value ? { kind: 'days', days: Number(value) } : null)}
        >
          {WAIVE_EXPIRY_DAY_CHOICES.map((days) => (
            <ToggleGroupItem key={days} value={String(days)}>
              {qf('waiveExpiryDays', { days })}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        <Input
          type="date"
          min={bounds.min}
          max={bounds.max}
          aria-label={qf('waiveExpiryDate')}
          value={choice?.kind === 'date' ? choice.date : ''}
          onChange={(event) =>
            setChoice(event.target.value ? { kind: 'date', date: event.target.value } : null)
          }
          className="h-8 text-xs"
        />
        {error ? (
          <p className="text-xs text-destructive" role="alert">
            {error}
          </p>
        ) : null}
        <div className="flex items-center justify-end gap-2">
          <Button type="button" variant="ghost" size="xs" onClick={() => setOpen(false)}>
            {qf('waiveCancel')}
          </Button>
          <Button type="button" size="xs" disabled={busy} onClick={() => void submit()}>
            {busy ? qf('waiveSaving') : qf('waiveSubmit')}
            <ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']} />
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
