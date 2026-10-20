/**
 * Outcome Review row of the stage timeline — FE-OR-T121
 *
 * The row sits after Execution (and Handover). It is never the current stage: the Request's own status decides that. It is
 * shown only when the project has the review on, so a project without it sees the timeline it always had.
 *
 * @module components/request/request-stage-timeline-review
 */

import type { ReviewRowState } from './outcome-review/outcome-review-view-model'
import type { StageTimeline, Step } from './request-stage-timeline-steps'

export type ReviewTimelineInput = {
  enabled: boolean
  row: ReviewRowState
  /** Items of a delivered review that still wait for a decision. */
  openDecisions: number
}

const REVIEW_LABEL = 'auto.components.request.StageTimeline.step.review'

function reviewStep(input: ReviewTimelineInput): Step {
  const base = { id: 'review' as const, labelKey: REVIEW_LABEL }
  switch (input.row) {
    case 'approved':
      return { ...base, state: 'done', caption: { kind: 'review', row: 'approved' } }
    case 'stale':
      return { ...base, state: 'waiting', caption: { kind: 'review', row: 'stale' } }
    case 'none':
      return { ...base, state: 'pending' }
    case 'delivered':
      return input.openDecisions > 0
        ? {
            ...base,
            state: 'waiting',
            caption: { kind: 'review', row: 'decisions', openDecisions: input.openDecisions }
          }
        : { ...base, state: 'pending', caption: { kind: 'review', row: 'delivered' } }
    case 'design':
      return { ...base, state: 'pending', caption: { kind: 'review', row: 'design' } }
    default:
      return { ...base, state: 'pending', caption: { kind: 'review', row: 'unknown' } }
  }
}

export function withReviewStep(
  timeline: StageTimeline,
  review: ReviewTimelineInput | undefined
): StageTimeline {
  // Why: a flow without Execution (a question, a spike) has no plan to review.
  if (!review?.enabled || !timeline.steps.some((s) => s.id === 'execution')) {
    return timeline
  }
  return { ...timeline, steps: [...timeline.steps, reviewStep(review)] }
}
