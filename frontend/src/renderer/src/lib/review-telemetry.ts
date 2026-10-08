/**
 * review-telemetry.ts — FE-CV-TASK-095-03
 *
 * Typed wrappers and bucketing for the review / quality-gate telemetry events.
 * Components call these, never `track()` directly. Payloads are closed enums and
 * buckets (see shared/review-telemetry-events.ts): no ids, paths, messages or raw counts.
 * The web build has no telemetry bridge, so every wrapper is inert there.
 *
 * @module lib/review-telemetry
 */

import { REVIEW_TOOL_VALUES } from '../../../shared/review-telemetry-events'
import type { EventProps } from '../../../shared/telemetry-events'
import { track } from './telemetry'

// ---------------------------------------------------------------------------
// Buckets
// ---------------------------------------------------------------------------

export type CountBucket = EventProps<'quality_gate_viewed'>['reasons']
export type LargeBucket = EventProps<'review_findings_summary'>['shown']
export type LatencyBucket = EventProps<'review_decision_made'>['latency']
export type DwellBucket = EventProps<'review_lens_viewed'>['dwell']
export type ToolBucket = EventProps<'quality_finding_triaged'>['tool']

function whole(n: number): number {
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0
}

export function bucketCount(n: number): CountBucket {
  const v = whole(n)
  if (v === 0) {return '0'}
  if (v === 1) {return '1'}
  if (v <= 3) {return '2-3'}
  if (v <= 10) {return '4-10'}
  return '11+'
}

export function bucketLarge(n: number): LargeBucket {
  const v = whole(n)
  if (v === 0) {return '0'}
  if (v <= 3) {return '1-3'}
  if (v <= 10) {return '4-10'}
  if (v <= 30) {return '11-30'}
  return '31+'
}

const MINUTE = 60_000

export function bucketLatencyMs(ms: number): LatencyBucket {
  const v = whole(ms)
  if (v < MINUTE) {return '<1m'}
  if (v < 5 * MINUTE) {return '<5m'}
  if (v < 30 * MINUTE) {return '<30m'}
  if (v < 240 * MINUTE) {return '<4h'}
  return '>=4h'
}

export function bucketDwellMs(ms: number): DwellBucket {
  const v = whole(ms)
  if (v < 10_000) {return '<10s'}
  if (v < MINUTE) {return '<1m'}
  if (v < 5 * MINUTE) {return '<5m'}
  return '>=5m'
}

const KNOWN_TOOLS: ReadonlySet<string> = new Set(REVIEW_TOOL_VALUES)

/** Maps a free-form tool name to the closed enum; anything unrecognized is 'other'. */
export function toToolBucket(tool: string | null | undefined): ToolBucket {
  const normalized = (tool ?? '').toLowerCase().replace(/[^a-z0-9]+/g, '_')
  return KNOWN_TOOLS.has(normalized) ? (normalized as ToolBucket) : 'other'
}

// ---------------------------------------------------------------------------
// Wrappers (one per event)
// ---------------------------------------------------------------------------

export function trackReviewOpened(props: EventProps<'review_opened'>): void {
  track('review_opened', props)
}

export function trackReviewLensViewed(a: { lens: EventProps<'review_lens_viewed'>['lens']; dwellMs: number }): void {
  track('review_lens_viewed', { lens: a.lens, dwell: bucketDwellMs(a.dwellMs) })
}

export function trackQualityGateViewed(a: {
  verdict: EventProps<'quality_gate_viewed'>['verdict']
  reasonCount: number
  stale: boolean
  surface: EventProps<'quality_gate_viewed'>['surface']
  source: EventProps<'quality_gate_viewed'>['source']
}): void {
  track('quality_gate_viewed', {
    verdict: a.verdict,
    reasons: bucketCount(a.reasonCount),
    stale: a.stale,
    surface: a.surface,
    source: a.source
  })
}

export function trackReviewDecisionMade(a: {
  decision: EventProps<'review_decision_made'>['decision']
  latencyMs: number
  usedReview: boolean
  gate: EventProps<'review_decision_made'>['gate']
  openFindings: number
}): void {
  track('review_decision_made', {
    decision: a.decision,
    latency: bucketLatencyMs(a.latencyMs),
    used_review: a.usedReview,
    gate: a.gate,
    open_findings: bucketCount(a.openFindings)
  })
}

export function trackReviewFindingsSummary(a: { shown: number; dismissed: number; waived: number; resolved: number }): void {
  track('review_findings_summary', {
    shown: bucketLarge(a.shown),
    dismissed: bucketLarge(a.dismissed),
    waived: bucketCount(a.waived),
    resolved: bucketLarge(a.resolved)
  })
}

export function trackQualityFindingTriaged(a: {
  action: EventProps<'quality_finding_triaged'>['action']
  reason: EventProps<'quality_finding_triaged'>['reason']
  severity: EventProps<'quality_finding_triaged'>['severity']
  tool: string | null | undefined
  blocking: boolean
}): void {
  track('quality_finding_triaged', {
    action: a.action,
    reason: a.reason,
    severity: a.severity,
    tool: toToolBucket(a.tool),
    blocking: a.blocking
  })
}

export function trackReviewReportExported(props: EventProps<'review_report_exported'>): void {
  track('review_report_exported', props)
}

export function trackReviewAiSummary(props: EventProps<'review_ai_summary'>): void {
  track('review_ai_summary', props)
}
