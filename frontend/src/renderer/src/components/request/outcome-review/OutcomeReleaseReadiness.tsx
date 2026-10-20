/**
 * OutcomeReleaseReadiness — FE-OR-T122
 *
 * Migration, breaking change and configuration, from the facts Orca measured. A cell Orca could not measure reads Unknown; it is
 * never a ticked box, because "nothing found" and "not looked at" are different things.
 *
 * @module components/request/outcome-review/OutcomeReleaseReadiness
 */

import React from 'react'
import { Check, CircleHelp, TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { readinessCellText, readinessRowText } from './outcome-review-copy'
import { releaseReadinessOf, type ReadinessCell } from './outcome-review-view-model'
import type { OutcomeReview } from '../../../../../shared/outcome-review-types'

const T = 'auto.components.request.outcomeReview.OutcomeReleaseReadiness.'

const ICON: Record<ReadinessCell, React.ComponentType<{ className?: string }>> = {
  present: TriangleAlert,
  none_found: Check,
  unknown: CircleHelp
}

export function OutcomeReleaseReadiness({ review }: { review: OutcomeReview }): React.JSX.Element {
  const title = translate(`${T}title`, 'Release readiness')
  return (
    <section
      className="flex flex-col gap-2 rounded-md border border-border p-3"
      aria-label={title}
      data-testid="outcome-release-readiness"
    >
      <h4 className="text-sm font-medium">{title}</h4>
      <ul className="flex flex-col gap-1 text-sm">
        {releaseReadinessOf(review).map(({ id, cell }) => {
          const Icon = ICON[cell]
          return (
            <li
              key={id}
              className="flex items-center gap-2"
              data-testid={`outcome-readiness-${id}`}
              data-cell={cell}
            >
              <Icon className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
              <span className="w-44 shrink-0">{readinessRowText(id)}</span>
              <span className="text-muted-foreground">{readinessCellText(cell)}</span>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
