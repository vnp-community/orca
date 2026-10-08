/**
 * SolutionVersionSwitcher — CR-REQ-020-02
 *
 * Lists solution versions (newest first); superseded ones are labelled "Older version".
 *
 * @module components/request/solution/SolutionVersionSwitcher
 */

import React from 'react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import type { Solution } from '../../../../../shared/request-types'

export function SolutionVersionSwitcher({
  solutions,
  currentId,
  onSelect
}: {
  solutions: Solution[]
  currentId: string
  onSelect: (id: string) => void
}): React.JSX.Element | null {
  if (solutions.length < 2) {return null}
  return (
    <div
      role="group"
      aria-label={translate('auto.components.request.SolutionVersionSwitcher.label', 'Versions')}
      className="flex flex-wrap items-center gap-1"
    >
      {solutions.map((s, i) => {
        const selected = s.id === currentId
        const base = translate('auto.components.request.SolutionVersionSwitcher.version', 'v{{n}}', {
          n: solutions.length - i
        })
        const old = s.status === 'superseded'
        return (
          <button
            key={s.id}
            type="button"
            aria-pressed={selected}
            onClick={() => onSelect(s.id)}
            className={cn(
              'rounded border px-2 py-0.5 text-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
              selected ? 'border-primary bg-primary/10 text-foreground' : 'border-border text-muted-foreground',
              old && 'opacity-80'
            )}
          >
            {base}
            {old && (
              <span className="ml-1">
                {translate('auto.components.request.SolutionVersionSwitcher.old', 'Older version')}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
