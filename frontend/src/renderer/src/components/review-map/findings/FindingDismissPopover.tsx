/**
 * FindingDismissPopover.tsx — FE-CV-TASK-059-05
 *
 * "Ignore" flow: a reason preset is required (stored in `reason`), the note is optional.
 * Mod+Enter confirms (Cmd on macOS, Ctrl elsewhere). The copy says the dismissal applies to the
 * whole repo and does not waive the quality gate.
 *
 * @module components/review-map/findings/FindingDismissPopover
 */

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Textarea } from '@/components/ui/textarea'
import { ShortcutKeyCombo } from '@/components/ShortcutKeyCombo'
import { getScreenSubmitModifierLabel, getScreenSubmitShortcutLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import { FINDING_TEXT_MAX } from '../../../hooks/useFindingDismissal'
import { tf } from './findings-i18n'

export const FINDING_REASON_PRESETS = ['not_applicable', 'accepted_risk', 'false_positive', 'later'] as const
export type FindingReasonPreset = (typeof FINDING_REASON_PRESETS)[number]

export function presetLabel(preset: string): string {
  switch (preset) {
    case 'not_applicable':
      return tf('reason.not_applicable', 'Not applicable')
    case 'accepted_risk':
      return tf('reason.accepted_risk', 'Accepted risk')
    case 'false_positive':
      return tf('reason.false_positive', 'False positive')
    case 'later':
      return tf('reason.later', 'Fix later')
    default:
      // Unknown reason codes are shown as written.
      return preset
  }
}

export function FindingDismissPopover({
  disabled,
  onConfirm,
  children
}: {
  disabled?: boolean
  onConfirm: (input: { reason: string; note?: string }) => void
  children: React.ReactNode
}): React.JSX.Element {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState<string>('')
  const [note, setNote] = useState('')

  const submit = (): void => {
    if (!reason) {
      return
    }
    onConfirm({ reason, ...(note.trim() ? { note: note.trim() } : {}) })
    setOpen(false)
    setReason('')
    setNote('')
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild disabled={disabled}>
        {children}
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-72 space-y-2 p-3 text-xs"
        onKeyDown={(event) => {
          if (isScreenSubmitShortcut(event)) {
            event.preventDefault()
            submit()
          }
        }}
      >
        <fieldset className="space-y-1">
          <legend className="font-medium">{tf('dismiss.reasonLegend', 'Why ignore this finding?')}</legend>
          {FINDING_REASON_PRESETS.map((preset) => (
            <label key={preset} className="flex items-center gap-2">
              <input
                type="radio"
                name="finding-dismiss-reason"
                value={preset}
                checked={reason === preset}
                onChange={() => setReason(preset)}
              />
              {presetLabel(preset)}
            </label>
          ))}
        </fieldset>
        <Textarea
          value={note}
          maxLength={FINDING_TEXT_MAX}
          onChange={(e) => setNote(e.target.value)}
          placeholder={tf('dismiss.notePlaceholder', 'Optional note')}
          aria-label={tf('dismiss.noteLabel', 'Note')}
          className="min-h-16 text-xs"
        />
        <p className="text-[11px] text-muted-foreground">
          {tf(
            'dismiss.scopeNote',
            'Applies to the whole repository. It does not waive the quality gate for error findings.'
          )}
        </p>
        <div className="flex items-center justify-between gap-2">
          <span className="flex items-center gap-1 text-[11px] text-muted-foreground" title={getScreenSubmitShortcutLabel()}>
            <ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']} />
          </span>
          <Button type="button" size="xs" disabled={!reason} onClick={submit}>
            {tf('dismiss.confirm', 'Ignore')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
