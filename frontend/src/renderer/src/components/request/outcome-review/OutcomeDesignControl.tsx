/**
 * OutcomeDesignControl — FE-OR-T122
 *
 * The head of the tab: two cards, Architecture and Flow. Before the code exists they count the design; after it they count what
 * the code did against the design. The numbers are what an agent reported and Orca checked, and say so. A count that cannot be
 * known (no design to compare with) reads Unknown, never 0.
 *
 * @module components/request/outcome-review/OutcomeDesignControl
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { agentReportedText, designRowText, sectionText, unknownText } from './outcome-review-copy'
import { designCardOf, type DesignCardModel } from './outcome-review-view-model'
import type { OutcomeReview, ReviewSection } from '../../../../../shared/outcome-review-types'

const T = 'auto.components.request.outcomeReview.OutcomeDesignControl.'

function sourceLine(card: DesignCardModel): string {
  if (card.mode === 'design') {
    return card.snapshot
      ? translate(`${T}fromDesign`, 'From the design (snapshot {{snapshot}})', {
          snapshot: card.snapshot
        })
      : translate(`${T}designNotAvailable`, 'Design not available')
  }
  return card.checkedAgainstDesign
    ? translate(`${T}checked`, 'Checked against design (snapshot {{snapshot}})', {
        snapshot: card.snapshot ?? ''
      })
    : translate(`${T}designNotAvailable`, 'Design not available')
}

function Card({
  card,
  onOpen
}: {
  card: DesignCardModel
  onOpen?: (section: ReviewSection) => void
}): React.JSX.Element {
  const title = sectionText(card.section)
  return (
    <section
      className="flex min-w-0 flex-1 flex-col gap-2 rounded-md border border-border p-3"
      aria-label={title}
      data-testid={`outcome-design-card-${card.section}`}
      data-mode={card.mode}
    >
      <div className="flex items-center gap-2">
        <h4 className="text-sm font-medium">{title}</h4>
        {card.agentReported && <Badge variant="outline">{agentReportedText()}</Badge>}
      </div>
      <p
        className="text-xs text-muted-foreground"
        data-testid={`outcome-design-source-${card.section}`}
      >
        {sourceLine(card)}
      </p>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-1 text-sm">
        {card.rows.map((row) => (
          <React.Fragment key={row.id}>
            <dt className="text-muted-foreground">{designRowText(row.id)}</dt>
            <dd
              className="text-right font-medium tabular-nums"
              data-testid={`outcome-design-row-${card.section}-${row.id}`}
            >
              {row.value === null ? unknownText() : row.value}
            </dd>
          </React.Fragment>
        ))}
      </dl>
      {onOpen && (
        <Button
          size="xs"
          variant="ghost"
          className="self-start"
          onClick={() => onOpen(card.section)}
        >
          {translate(`${T}open`, 'Open {{section}} details', { section: title })}
        </Button>
      )}
    </section>
  )
}

type Props = {
  review: OutcomeReview
  /** Jumps to the sub-tab of the section (the system diff, Phase 2). */
  onOpenSection?: (section: ReviewSection) => void
}

export function OutcomeDesignControl({ review, onOpenSection }: Props): React.JSX.Element {
  return (
    <div className="flex flex-col gap-2 sm:flex-row" data-testid="outcome-design-control">
      <Card card={designCardOf(review, 'architecture')} onOpen={onOpenSection} />
      <Card card={designCardOf(review, 'flow')} onOpen={onOpenSection} />
    </div>
  )
}
