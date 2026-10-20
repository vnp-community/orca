/**
 * OutcomeReviewHeader — FE-OR-T122
 *
 * Who the review is about and how current it is: the phase, the state, what it compares (`base → head`), the knowledge-graph
 * snapshot the design came from, and when it was made. "Regenerate…" belongs here once the regeneration panel exists.
 *
 * @module components/request/outcome-review/OutcomeReviewHeader
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { phaseText, stateText, titleText } from './outcome-review-copy'
import { phaseOf } from './outcome-review-view-model'
import type { OutcomeReview } from '../../../../../shared/outcome-review-types'

const T = 'auto.components.request.outcomeReview.OutcomeReviewHeader.'

function headOf(review: OutcomeReview): string {
  const heads = review.model.subject.heads
  if (heads.length === 0) {
    return review.model.subject.headCommit?.slice(0, 7) ?? ''
  }
  const first = heads[0].branch || heads[0].commit.slice(0, 7)
  return heads.length > 1 ? `${first} +${heads.length - 1}` : first
}

function whenText(iso: string): string {
  const time = Date.parse(iso)
  return Number.isNaN(time) ? '' : new Date(time).toLocaleString()
}

type Props = {
  review: OutcomeReview
  /** Opens the regeneration panel; absent hides the button. */
  onRegenerate?: () => void
}

export function OutcomeReviewHeader({ review, onRegenerate }: Props): React.JSX.Element {
  const { subject, kg } = review.model
  const head = headOf(review)
  const generated = whenText(review.updatedAt || review.createdAt)
  return (
    <header
      className="flex flex-wrap items-center gap-x-3 gap-y-1"
      data-testid="outcome-review-header"
    >
      <h3 className="text-sm font-semibold">{titleText()}</h3>
      <Badge variant="outline" data-testid="outcome-review-phase">
        {phaseText(phaseOf(review))}
      </Badge>
      <Badge
        variant={review.state === 'approved' ? 'default' : 'secondary'}
        data-testid="outcome-review-state"
      >
        {stateText(review.state)}
      </Badge>
      {subject.baseRef && head && (
        <span className="text-xs text-muted-foreground" data-testid="outcome-review-subject">
          {subject.baseRef} → {head}
        </span>
      )}
      <Badge variant="outline" data-testid="outcome-review-kg">
        {kg.available && kg.snapshot
          ? translate(`${T}kgSnapshot`, 'KG snapshot {{snapshot}}', { snapshot: kg.snapshot })
          : translate(`${T}kgUnavailable`, 'KG unavailable')}
      </Badge>
      {generated && (
        <span className="text-xs text-muted-foreground">
          {translate(`${T}generated`, 'Generated {{when}}', { when: generated })}
        </span>
      )}
      {onRegenerate && (
        <Button
          size="xs"
          variant="outline"
          className="ml-auto"
          onClick={onRegenerate}
          data-testid="outcome-header-regenerate"
        >
          {translate(`${T}regenerate`, 'Regenerate…')}
        </Button>
      )}
    </header>
  )
}
