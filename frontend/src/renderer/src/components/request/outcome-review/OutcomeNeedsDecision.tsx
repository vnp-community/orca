/**
 * OutcomeNeedsDecision — FE-OR-T122
 *
 * The items of a delivered review that ask for a decision. Before the code exists there is nothing to decide, and the block says
 * so. The decision controls (Phase 3) arrive through `renderActions`, so this list reads the same with or without them.
 *
 * @module components/request/outcome-review/OutcomeNeedsDecision
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { agentReportedText, sectionText, severityText, unverifiedText } from './outcome-review-copy'
import { decisionsLaterText } from './outcome-review-state-copy'
import type { ReviewItem } from '../../../../../shared/outcome-review-types'

const T = 'auto.components.request.outcomeReview.OutcomeNeedsDecision.'

type Props = {
  items: ReviewItem[]
  /** True before the code exists: no decisions can be made yet. */
  later: boolean
  renderActions?: (item: ReviewItem) => React.ReactNode
}

export function OutcomeNeedsDecision({ items, later, renderActions }: Props): React.JSX.Element {
  const title = translate(`${T}title`, 'Needs your decision')
  if (later) {
    return (
      <section
        className="rounded-md border border-dashed border-border p-3 text-sm text-muted-foreground"
        aria-label={title}
        data-testid="outcome-needs-decision-later"
      >
        {decisionsLaterText()}
      </section>
    )
  }
  return (
    <section
      className="flex flex-col gap-2 rounded-md border border-border p-3"
      aria-label={title}
      data-testid="outcome-needs-decision"
    >
      <h4 className="text-sm font-medium">
        {title}
        <span className="ml-2 text-muted-foreground tabular-nums">({items.length})</span>
      </h4>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {translate(`${T}none`, 'Nothing asks for a decision.')}
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {items.map((item) => (
            <li
              key={item.key}
              className="flex flex-col gap-1 rounded-md border border-border/60 p-2"
              data-testid="outcome-decision-item"
              data-item-key={item.key}
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium">{item.title}</span>
                <Badge variant="secondary">{sectionText(item.section)}</Badge>
                <Badge variant="outline">{severityText(item.severity)}</Badge>
                {item.origin === 'agent' && <Badge variant="outline">{agentReportedText()}</Badge>}
                {item.unverified && <Badge variant="outline">{unverifiedText()}</Badge>}
              </div>
              {item.detail && <p className="text-xs text-muted-foreground">{item.detail}</p>}
              {renderActions?.(item)}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
