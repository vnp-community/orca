/**
 * Words of the Outcome Review row in the stage timeline — FE-OR-T121
 *
 * @module components/request/outcome-review/outcome-review-timeline-copy
 */

import { translate } from '@/i18n/i18n'
import type { ReviewRowCaption } from '../request-stage-timeline-steps'

const T = 'auto.components.request.outcomeReview.timeline.'

export function reviewCaptionText(row: ReviewRowCaption, openDecisions: number): string {
  switch (row) {
    case 'design':
      return translate(`${T}design`, 'design review')
    case 'delivered':
      return translate(`${T}delivered`, 'after-code review')
    case 'stale':
      return translate(`${T}stale`, 'out of date')
    case 'approved':
      return translate(`${T}approved`, 'approved')
    case 'decisions':
      return translate(`${T}decisions`, '{{count}} decisions open', { count: openDecisions })
    case 'none':
      return translate(`${T}none`, 'not generated')
    default:
      return translate(`${T}unknown`, 'unknown')
  }
}
