/**
 * SolutionOptionCard — CR-REQ-020-03
 *
 * One option as a radio. Selection and "chosen" use icon + text, not colour alone.
 *
 * @module components/request/solution/SolutionOptionCard
 */

import React from 'react'
import { Check, ThumbsDown, ThumbsUp } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import type { SolutionOption } from '../../../../../shared/request-types'

export type SolutionOptionCardProps = {
  option: SolutionOption
  selected: boolean
  /** The option the approved Solution recorded. */
  chosen: boolean
  readOnly: boolean
  tabIndex: number
  onSelect: () => void
  onKeyDown: (e: React.KeyboardEvent<HTMLButtonElement>) => void
  buttonRef?: (el: HTMLButtonElement | null) => void
}

function textField(raw: Record<string, unknown> | undefined, key: string): string | undefined {
  const v = raw?.[key]
  return typeof v === 'string' && v ? v : undefined
}

export function SolutionOptionCard({
  option,
  selected,
  chosen,
  readOnly,
  tabIndex,
  onSelect,
  onKeyDown,
  buttonRef
}: SolutionOptionCardProps): React.JSX.Element {
  const risk = textField(option.raw, 'risk')
  const recommended = option.raw?.recommended === true
  const noData = translate('auto.components.request.SolutionPanel.noData', 'No data')

  return (
    <button
      ref={buttonRef}
      type="button"
      role="radio"
      aria-checked={selected}
      aria-disabled={readOnly || undefined}
      tabIndex={tabIndex}
      data-testid={`solution-option-${option.id}`}
      onClick={() => {
        if (!readOnly) {onSelect()}
      }}
      onKeyDown={onKeyDown}
      className={cn(
        'flex flex-col gap-2 rounded-md border p-3 text-left text-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
        selected ? 'border-primary bg-primary/5' : 'border-border',
        readOnly ? 'cursor-default' : 'cursor-pointer hover:bg-accent/40'
      )}
    >
      <span className="flex flex-wrap items-center gap-2">
        <span className="font-medium">{option.title || noData}</span>
        {recommended && (
          <span className="rounded border border-primary/30 px-1.5 text-xs text-primary">
            {translate('auto.components.request.SolutionOptionCard.recommended', 'Recommended')}
          </span>
        )}
        {chosen && (
          <span className="inline-flex items-center gap-1 text-xs text-[color:var(--status-success)]">
            <Check className="size-3" aria-hidden />
            {translate('auto.components.request.SolutionOptionCard.chosen', 'Chosen')}
          </span>
        )}
      </span>
      <span className="text-muted-foreground">{option.summary || noData}</span>
      {option.pros && option.pros.length > 0 && (
        <ul aria-label={translate('auto.components.request.SolutionOptionCard.pros', 'Pros')} className="space-y-0.5">
          {option.pros.map((p, i) => (
            <li key={i} className="flex gap-1.5">
              <ThumbsUp className="mt-0.5 size-3 shrink-0 text-[color:var(--status-success)]" aria-hidden />
              <span>{p}</span>
            </li>
          ))}
        </ul>
      )}
      {option.cons && option.cons.length > 0 && (
        <ul aria-label={translate('auto.components.request.SolutionOptionCard.cons', 'Cons')} className="space-y-0.5">
          {option.cons.map((c, i) => (
            <li key={i} className="flex gap-1.5">
              <ThumbsDown className="mt-0.5 size-3 shrink-0 text-destructive" aria-hidden />
              <span>{c}</span>
            </li>
          ))}
        </ul>
      )}
      <span className="flex gap-4 text-xs text-muted-foreground">
        <span>
          {translate('auto.components.request.SolutionOptionCard.effort', 'Effort')}: {option.estimatedEffort || noData}
        </span>
        <span>
          {translate('auto.components.request.SolutionOptionCard.risk', 'Risk')}: {risk || noData}
        </span>
      </span>
    </button>
  )
}
