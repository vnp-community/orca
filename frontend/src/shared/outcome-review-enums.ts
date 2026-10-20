/**
 * Closed sets of the Outcome Review enums — FE-OR-T111
 *
 * A value outside its set is not an error: the screen shows "Unknown" and keeps the raw string.
 *
 * @module shared/outcome-review-enums
 */

import type {
  ComponentSource,
  DiagramStatus,
  DiagramVariant,
  EvidenceKind,
  KgReviewState,
  Open,
  ReviewChange,
  ReviewDecisionValue,
  ReviewOrigin,
  ReviewPhase,
  ReviewRunKind,
  ReviewRunStatus,
  ReviewSection,
  ReviewSeverity,
  ReviewState
} from './outcome-review-types'

export const REVIEW_PHASES: ReadonlySet<string> = new Set<ReviewPhase>(['design', 'delivered'])
export const REVIEW_STATES: ReadonlySet<string> = new Set<ReviewState>([
  'draft',
  'ready',
  'stale',
  'approved',
  'superseded'
])
export const REVIEW_SECTIONS: ReadonlySet<string> = new Set<ReviewSection>([
  'architecture',
  'flow',
  'behavior',
  'data',
  'contracts',
  'requirements',
  'config'
])
export const REVIEW_CHANGES: ReadonlySet<string> = new Set<ReviewChange>([
  'added',
  'modified',
  'removed',
  'unchanged',
  'missing'
])
export const REVIEW_ORIGINS: ReadonlySet<string> = new Set<ReviewOrigin>(['fact', 'agent'])
export const REVIEW_SEVERITIES: ReadonlySet<string> = new Set<ReviewSeverity>([
  'info',
  'warning',
  'breaking',
  'deviation',
  'regression'
])
export const COMPONENT_SOURCES: ReadonlySet<string> = new Set<ComponentSource>([
  'kg',
  'sad-text',
  'code'
])
export const KG_REVIEW_STATES: ReadonlySet<string> = new Set<KgReviewState>([
  'approved',
  'needs_review',
  'unknown'
])
export const EVIDENCE_KINDS: ReadonlySet<string> = new Set<EvidenceKind>([
  'kg',
  'source',
  'file',
  'rpc',
  'channel',
  'migration',
  'check',
  'task'
])
export const DIAGRAM_VARIANTS: ReadonlySet<string> = new Set<DiagramVariant>(['design', 'delta'])
export const DIAGRAM_STATUSES: ReadonlySet<string> = new Set<DiagramStatus>([
  'cached',
  'missing',
  'drawing',
  'failed',
  'disabled'
])
export const RUN_KINDS: ReadonlySet<string> = new Set<ReviewRunKind>(['context', 'draw'])
export const RUN_STATUSES: ReadonlySet<string> = new Set<ReviewRunStatus>([
  'queued',
  'running',
  'succeeded',
  'failed',
  'cancelled'
])
export const DECISION_VALUES: ReadonlySet<string> = new Set<ReviewDecisionValue>([
  'accept_exception',
  'send_back',
  'dismiss'
])

/** True for a value of the closed set; false for an empty or unfamiliar string, which the screen shows as "Unknown". */
export function isKnown<T extends string>(
  set: ReadonlySet<string>,
  value: Open<T> | undefined
): value is T {
  return typeof value === 'string' && set.has(value)
}
