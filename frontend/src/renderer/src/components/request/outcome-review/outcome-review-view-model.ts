/**
 * Outcome Review view model — FE-OR-T122, T123
 *
 * Pure functions from the review the server sent to what the tab shows: which screen, which banners, which numbers, and which
 * of them are Unknown. No React and no text: the copy module turns the ids into words, so every combination of state and phase
 * can be tested without a DOM.
 *
 * @module components/request/outcome-review/outcome-review-view-model
 */

import { REVIEW_PHASES, REVIEW_STATES, isKnown } from '../../../../../shared/outcome-review-enums'
import type {
  DesignVerdict,
  OutcomeReview,
  OutcomeReviewRun,
  ReviewEstimate,
  ReviewItem,
  ReviewPhase,
  ReviewSection
} from '../../../../../shared/outcome-review-types'
import type { RequestStatus } from '../../../../../shared/request-types'
import type { OutcomeReviewStatus } from '../../../hooks/useOutcomeReview'

/** Below this share of approved design elements the findings that rest on them are advisory (AM-3). */
export const LOW_REVIEWED_RATIO = 0.5

export type ReviewScreen =
  | 'loading'
  | 'unavailable'
  | 'error'
  /** No review yet and the code does not exist: the invitation to review the design. */
  | 'invite_design'
  /** No review yet and Execution is over: the invitation to review the delivered work. */
  | 'invite_delivered'
  | 'review'

export type ReviewBannerId =
  | 'design_phase'
  | 'execution_finished'
  | 'stale'
  | 'design_not_reviewed'
  | 'kg_unavailable'
  | 'truncated'
  | 'run_failed'
  | 'drawing_failed'

export type ReviewViewInput = {
  status: OutcomeReviewStatus
  review: OutcomeReview | null
  estimate: ReviewEstimate | null
  requestStatus: RequestStatus
  activeRun: OutcomeReviewRun | null
  runs: OutcomeReviewRun[]
}

export function executionFinished(requestStatus: RequestStatus): boolean {
  return requestStatus === 'completed'
}

/** The phase a review made now would have: the code exists only once Execution is over. */
export function phaseOfRequest(requestStatus: RequestStatus): ReviewPhase {
  return executionFinished(requestStatus) ? 'delivered' : 'design'
}

export function phaseOf(review: OutcomeReview): ReviewPhase | 'unknown' {
  return isKnown<ReviewPhase>(REVIEW_PHASES, review.phase) ? review.phase : 'unknown'
}

export function screenOf(input: ReviewViewInput): ReviewScreen {
  switch (input.status) {
    case 'loading':
      return 'loading'
    case 'unsupported':
    case 'disabled':
    case 'not_ready':
      return 'unavailable'
    case 'error':
      return 'error'
    case 'none':
      return executionFinished(input.requestStatus) ? 'invite_delivered' : 'invite_design'
    default:
      return input.review ? 'review' : 'loading'
  }
}

/** The last context run, if it ended badly and no newer one is going: the review keeps its facts and says why the rest is missing. */
export function failedContextRun(
  runs: OutcomeReviewRun[],
  activeRun: OutcomeReviewRun | null
): OutcomeReviewRun | null {
  if (activeRun) {
    return null
  }
  const last = runs.find((r) => r.kind === 'context')
  return last && last.status === 'failed' ? last : null
}

export function bannersOf(input: ReviewViewInput): ReviewBannerId[] {
  const { review, requestStatus } = input
  if (!review) {
    return []
  }
  const model = review.model
  const out: ReviewBannerId[] = []
  const phase = phaseOf(review)
  if (phase === 'design' && !executionFinished(requestStatus)) {
    out.push('design_phase')
  }
  if (phase === 'design' && executionFinished(requestStatus)) {
    out.push('execution_finished')
  }
  if (review.state === 'stale') {
    out.push('stale')
  }
  if (!model.kg.available) {
    out.push('kg_unavailable')
  } else if (
    model.verdict.architecture.designChecked &&
    model.verdict.architecture.reviewedRatio < LOW_REVIEWED_RATIO
  ) {
    out.push('design_not_reviewed')
  }
  if (failedContextRun(input.runs, input.activeRun)) {
    out.push('run_failed')
  }
  if (model.diagrams.some((d) => d.status === 'failed')) {
    out.push('drawing_failed')
  }
  if (model.truncated) {
    out.push('truncated')
  }
  return out
}

// ---------------------------------------------------------------------------
// Architecture and Flow cards
// ---------------------------------------------------------------------------

export type DesignRowId =
  | 'containers_changed'
  | 'new_edges'
  | 'removed_edges'
  | 'unplanned_edges'
  | 'new_calls'
  | 'removed_calls'
  | 'reordered'
  | 'unplanned_calls'
  | 'design_items'
  | 'drawings'

/** A count, or null when it cannot be known (nothing to compare with): the card shows Unknown, never 0. */
export type DesignRow = { id: DesignRowId; value: number | null }

export type DesignCardModel = {
  section: Extract<ReviewSection, 'architecture' | 'flow'>
  /** What the card counts: the design itself before the code exists, or the code against the design after. */
  mode: 'design' | 'delivered'
  /** Always true: the numbers come from what an agent reported and Orca checked. */
  agentReported: boolean
  /** The design was available to compare with (delivered mode only). */
  checkedAgainstDesign: boolean
  snapshot?: string
  reviewedRatio: number
  advisory: number
  rows: DesignRow[]
}

function itemsOf(review: OutcomeReview, section: ReviewSection): ReviewItem[] {
  return review.model.items.filter((i) => i.section === section)
}

function deliveredRows(section: 'architecture' | 'flow', v: DesignVerdict): DesignRow[] {
  // Why: counts of what is not in the design mean nothing without a design; they are Unknown, not zero.
  const unknownWithoutDesign = (n: number): number | null => (v.designChecked ? n : null)
  return section === 'architecture'
    ? [
        { id: 'containers_changed', value: v.containersChanged },
        { id: 'new_edges', value: v.newEdges },
        { id: 'removed_edges', value: v.removedEdges },
        { id: 'unplanned_edges', value: unknownWithoutDesign(v.unplannedEdges) }
      ]
    : [
        { id: 'new_calls', value: v.newCalls },
        { id: 'removed_calls', value: v.removedCalls },
        { id: 'reordered', value: v.reordered },
        { id: 'unplanned_calls', value: unknownWithoutDesign(v.unplannedCalls) }
      ]
}

export function designCardOf(
  review: OutcomeReview,
  section: 'architecture' | 'flow'
): DesignCardModel {
  const model = review.model
  const verdict = section === 'architecture' ? model.verdict.architecture : model.verdict.flow
  const delivered = phaseOf(review) === 'delivered'
  const base = {
    section,
    agentReported: true,
    checkedAgainstDesign: delivered && verdict.designChecked,
    snapshot: model.kg.available ? model.kg.snapshot : undefined,
    reviewedRatio: verdict.reviewedRatio,
    advisory: verdict.advisory
  }
  if (delivered) {
    return { ...base, mode: 'delivered', rows: deliveredRows(section, verdict) }
  }
  return {
    ...base,
    mode: 'design',
    rows: [
      { id: 'design_items', value: itemsOf(review, section).length },
      {
        id: 'drawings',
        value: model.diagrams.filter((d) => d.section === section && d.variant === 'design').length
      }
    ]
  }
}

// ---------------------------------------------------------------------------
// Verdict cards (delivered only)
// ---------------------------------------------------------------------------

export type RequirementsCard = {
  delivered: number
  total: number
  partial: number
  notCovered: number
  unknown: number
  /** Every task tied to every criterion is done (Orca does not know which check proved it). Never "met" or "ready". */
  allDelivered: boolean
  /** No acceptance criterion had data: the card says Unknown, never Delivered. */
  noData: boolean
}

export function requirementsCardOf(review: OutcomeReview): RequirementsCard {
  const r = review.model.verdict.requirements
  return {
    delivered: r.delivered,
    total: r.total,
    partial: r.partial,
    notCovered: r.notCovered,
    unknown: r.unknown,
    allDelivered: r.total > 0 && r.delivered === r.total,
    noData: r.total === 0 || r.unknown === r.total
  }
}

export type GateId = 'pass' | 'warn' | 'fail' | 'unknown' | 'absent' | 'other'
export type RiskId = 'low' | 'medium' | 'high' | 'critical' | 'unknown' | 'other'

const GATES: ReadonlySet<string> = new Set(['pass', 'warn', 'fail', 'unknown', 'absent'])
const RISKS: ReadonlySet<string> = new Set(['low', 'medium', 'high', 'critical', 'unknown'])

export function gateOf(review: OutcomeReview): GateId {
  const gate = review.model.verdict.quality.gate
  return GATES.has(gate) ? (gate as GateId) : gate === '' ? 'unknown' : 'other'
}

export function riskOf(review: OutcomeReview): { risk: RiskId; flags: string[] } {
  const { risk, flags } = review.model.verdict.release
  return { risk: RISKS.has(risk) ? (risk as RiskId) : risk === '' ? 'unknown' : 'other', flags }
}

// ---------------------------------------------------------------------------
// Needs your decision (delivered only)
// ---------------------------------------------------------------------------

/** An item that asks for a decision. Advisory items never do: they rest on elements nobody approved (AM-3). */
export function needsDecisionItems(review: OutcomeReview): ReviewItem[] {
  if (phaseOf(review) !== 'delivered') {
    return []
  }
  return review.model.items.filter((i) => i.needsDecision && !i.advisory)
}

export function advisoryItems(review: OutcomeReview): ReviewItem[] {
  return review.model.items.filter((i) => i.advisory)
}

// ---------------------------------------------------------------------------
// Release readiness (delivered only)
// ---------------------------------------------------------------------------

export type ReadinessCell = 'present' | 'none_found' | 'unknown'
export type ReadinessRowId = 'migration' | 'breaking' | 'config'

const READINESS_FLAGS: readonly ReadinessRowId[] = ['migration', 'breaking', 'config']

/**
 * A flag that is raised is "present". A flag that is not raised is "none found" only when the risk was measured; with the risk
 * unknown the cell is Unknown, never a ticked box.
 */
export function releaseReadinessOf(
  review: OutcomeReview
): { id: ReadinessRowId; cell: ReadinessCell }[] {
  const { risk, flags } = riskOf(review)
  return READINESS_FLAGS.map((id) => ({
    id,
    cell: flags.includes(id)
      ? 'present'
      : risk === 'unknown' || risk === 'other'
        ? 'unknown'
        : 'none_found'
  }))
}

// ---------------------------------------------------------------------------
// Timeline row and next action
// ---------------------------------------------------------------------------

export type ReviewRowState = 'none' | 'design' | 'delivered' | 'stale' | 'approved' | 'unknown'

/** The word of the timeline row: where the review is, with stale and approved winning over the phase. */
export function timelineRowStateOf(review: OutcomeReview | null): ReviewRowState {
  if (!review) {
    return 'none'
  }
  if (review.state === 'approved') {
    return 'approved'
  }
  if (review.state === 'stale') {
    return 'stale'
  }
  const phase = phaseOf(review)
  return phase === 'unknown' || !isKnown(REVIEW_STATES, review.state) ? 'unknown' : phase
}

export type ReviewHint = 'review_design' | 'generate_delivered' | 'decide' | 'none'

/**
 * What the next-action bar may suggest about the review. A soft hint: it never blocks anything else. An approved review, and a
 * review the viewer cannot see, suggest nothing.
 */
export function reviewHintOf(input: {
  enabled: boolean
  requestStatus: RequestStatus
  review: OutcomeReview | null
  loaded: boolean
}): ReviewHint {
  if (!input.enabled || !input.loaded) {
    return 'none'
  }
  const { review, requestStatus } = input
  if (review?.state === 'approved') {
    return 'none'
  }
  if (
    review &&
    phaseOf(review) === 'delivered' &&
    needsDecisionItems(review).length > 0 &&
    review.state !== 'stale'
  ) {
    return 'decide'
  }
  if (executionFinished(requestStatus)) {
    return !review || phaseOf(review) !== 'delivered' ? 'generate_delivered' : 'none'
  }
  if (requestStatus === 'awaiting_plan_approval' || requestStatus === 'executing') {
    return review ? 'none' : 'review_design'
  }
  return 'none'
}

/** The sections a regeneration may draw: the ones the review has diagrams for, else the two every review draws. */
export function drawnSectionsOf(review: OutcomeReview): string[] {
  const seen = [...new Set(review.model.diagrams.map((d) => d.section))]
  return seen.length > 0 ? seen : ['architecture', 'flow']
}
