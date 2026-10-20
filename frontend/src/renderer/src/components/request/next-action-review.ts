/**
 * Outcome Review hints for the next-action bar — FE-OR-T121
 *
 * Soft: the bar's own priority is untouched. A failure or something that waits for the viewer is never covered by a hint; a
 * hint fills the quiet moments (the request is done, or waiting for others) and, while a review is open for decisions, says so.
 *
 * @module components/request/next-action-review
 */

import { translate } from '@/i18n/i18n'
import type { NextAction } from './next-action-rules'
import type { ReviewHint } from './outcome-review/outcome-review-view-model'

const T = 'auto.components.request.outcomeReview.hint.'

export function withReviewHint(
  base: NextAction,
  hint: ReviewHint,
  openDecisions: number
): NextAction {
  if (hint === 'none' || base.kind === 'failed' || base.kind === 'needs_you') {
    return base
  }
  if (hint === 'decide') {
    return {
      kind: 'needs_you',
      title: translate(`${T}decide`, '{{count}} review decisions are waiting for you', {
        count: openDecisions
      }),
      tab: 'outcome-review',
      primary: { label: translate(`${T}openReview`, 'Open the review'), command: 'open_tab' }
    }
  }
  const label =
    hint === 'review_design'
      ? translate(`${T}reviewDesign`, 'Review the design')
      : translate(`${T}generateDelivered`, 'Generate after-code review')
  if (base.primary) {
    return base
  }
  return { ...base, primary: { label, command: 'open_tab' }, primaryTab: 'outcome-review' }
}
