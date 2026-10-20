/**
 * OutcomeModeSwitch — FE-OR-T202
 *
 * Design | After code | Side by side. A mode that cannot be used is dimmed and says when it can: before the code exists,
 * before the after-code review has been made, or without the design. A dimmed mode is not removed, so the switch does not change
 * shape from one review to the next.
 *
 * @module components/request/outcome-review/diff/OutcomeModeSwitch
 */

import React from 'react'
import { cn } from '@/lib/utils'
import { modeReasonText, modeText } from './outcome-diff-copy'
import type { DiffMode, ModeAvailability } from './outcome-diff-view-model'

const MODES: DiffMode[] = ['design', 'after_code', 'side_by_side']

type Props = { mode: DiffMode; availability: ModeAvailability; onChange: (mode: DiffMode) => void }

export function OutcomeModeSwitch({ mode, availability, onChange }: Props): React.JSX.Element {
  return (
    <div
      role="group"
      aria-label={MODES.map(modeText).join(' / ')}
      className="inline-flex flex-wrap items-center gap-1"
      data-testid="outcome-mode-switch"
    >
      {MODES.map((m) => {
        const state = availability[m]
        const selected = mode === m
        return (
          <button
            key={m}
            type="button"
            aria-pressed={selected}
            aria-disabled={!state.enabled}
            title={state.reason ? modeReasonText(state.reason) : undefined}
            onClick={() => state.enabled && onChange(m)}
            className={cn(
              'rounded-md border px-3 py-1 text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring',
              selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border',
              !state.enabled && 'cursor-not-allowed opacity-50'
            )}
            data-mode={m}
          >
            {modeText(m)}
            {!state.enabled && state.reason && (
              <span className="sr-only"> — {modeReasonText(state.reason)}</span>
            )}
          </button>
        )
      })}
    </div>
  )
}
