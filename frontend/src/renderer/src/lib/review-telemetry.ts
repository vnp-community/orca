/**
 * review-telemetry.ts — FE-CV-TASK-095-03
 *
 * Typed wrappers around track() for Review workspace events.
 * Bucket helpers keep payloads coarse (no per-user, no raw paths).
 *
 * Privacy rules (CR-095):
 * - NO agent_type, model, or turn IDs in any event
 * - NO raw file paths — use hashed fingerprints only
 * - NO user-authored text (prompt, comments, PR body)
 * - Web build: track() is a no-op, so all wrappers are inert there
 *
 * @module lib/review-telemetry
 */

import { track } from './telemetry'
import type {
  ReviewTabOpenedEvent,
  ReviewLensChangedEvent,
  ReviewFindingOpenedEvent,
  ReviewReadingProgressSavedEvent,
  ReviewQualityGateViewedEvent,
  ReviewReportExportedEvent,
  ReviewReindexRequestedEvent,
  ReviewAiSummaryShownEvent,
} from '../components/review-map/telemetry/review-telemetry-event-schemas'
import { REVIEW_TELEMETRY_EVENTS } from '../components/review-map/telemetry/review-telemetry-event-schemas'

// ---------------------------------------------------------------------------
// Bucket helpers
// ---------------------------------------------------------------------------

/** Files-read progress bucket */
export type ReviewCountBucket = '0' | '1-5' | '6-20' | '21-50' | '50+'

export function bucketCount(n: number): ReviewCountBucket {
  if (n === 0) return '0'
  if (n <= 5) return '1-5'
  if (n <= 20) return '6-20'
  if (n <= 50) return '21-50'
  return '50+'
}

/** Large-count bucket (e.g. findings, sections) */
export type ReviewLargeBucket = '0' | '1' | '2-5' | '6-20' | '20+'

export function bucketLarge(n: number): ReviewLargeBucket {
  if (n === 0) return '0'
  if (n === 1) return '1'
  if (n <= 5) return '2-5'
  if (n <= 20) return '6-20'
  return '20+'
}

/** Latency bucket in milliseconds */
export type ReviewLatencyBucket = '<1s' | '<5s' | '<15s' | '<60s' | '>=60s'

export function bucketLatencyMs(ms: number): ReviewLatencyBucket {
  if (ms < 1_000) return '<1s'
  if (ms < 5_000) return '<5s'
  if (ms < 15_000) return '<15s'
  if (ms < 60_000) return '<60s'
  return '>=60s'
}

/** Dwell time bucket in milliseconds */
export type ReviewDwellBucket = '<5s' | '<30s' | '<2m' | '>=2m'

export function bucketDwellMs(ms: number): ReviewDwellBucket {
  if (ms < 5_000) return '<5s'
  if (ms < 30_000) return '<30s'
  if (ms < 120_000) return '<2m'
  return '>=2m'
}

/** Tool-name bucket — maps known tool names; unknown → 'other' */
export type ReviewToolBucket =
  | 'commit'
  | 'pr'
  | 'reindex'
  | 'report'
  | 'ai_summary'
  | 'gate'
  | 'lens'
  | 'finding'
  | 'other'

const TOOL_BUCKET_MAP: Record<string, ReviewToolBucket> = {
  commit: 'commit',
  pull_request: 'pr',
  create_pr: 'pr',
  reindex: 'reindex',
  review_report: 'report',
  ai_summary: 'ai_summary',
  quality_gate: 'gate',
  lens_switch: 'lens',
  open_finding: 'finding',
}

export function toToolBucket(toolName: string): ReviewToolBucket {
  return TOOL_BUCKET_MAP[toolName] ?? 'other'
}

// ---------------------------------------------------------------------------
// Track wrappers — one per event schema
// ---------------------------------------------------------------------------

export function trackReviewTabOpened(a: ReviewTabOpenedEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_TAB_OPENED, {
    worktree_fingerprint: a.worktreeFingerprint,
    trigger: a.trigger,
    lens_id: a.lensId ?? null,
  })
}

export function trackReviewLensChanged(a: ReviewLensChangedEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_LENS_CHANGED, {
    worktree_fingerprint: a.worktreeFingerprint,
    lens_id: a.lensId,
    previous_lens_id: a.previousLensId,
  })
}

export function trackReviewFindingOpened(a: ReviewFindingOpenedEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_FINDING_OPENED, {
    worktree_fingerprint: a.worktreeFingerprint,
    finding_category: a.findingCategory,
    severity: a.severity,
    lens_id: a.lensId ?? null,
  })
}

export function trackReviewReadingProgressSaved(a: ReviewReadingProgressSavedEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_READING_PROGRESS_SAVED, {
    worktree_fingerprint: a.worktreeFingerprint,
    total_files: bucketCount(a.totalFiles),
    read_files: bucketCount(a.readFiles),
    skipped_files: bucketCount(a.skippedFiles),
  })
}

export function trackReviewQualityGateViewed(a: ReviewQualityGateViewedEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_QUALITY_GATE_VIEWED, {
    worktree_fingerprint: a.worktreeFingerprint,
    overall_risk: a.overallRisk,
    lens_id: a.lensId ?? null,
  })
}

export function trackReviewReportExported(a: ReviewReportExportedEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_REPORT_EXPORTED, {
    worktree_fingerprint: a.worktreeFingerprint,
    format: a.format,
    section_count: bucketLarge(a.sectionCount),
    lens_id: a.lensId ?? null,
  })
}

export function trackReviewReindexRequested(a: ReviewReindexRequestedEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_REINDEX_REQUESTED, {
    worktree_fingerprint: a.worktreeFingerprint,
    trigger: a.trigger,
    lens_id: a.lensId ?? null,
  })
}

export function trackReviewAiSummaryShown(a: ReviewAiSummaryShownEvent): void {
  track(REVIEW_TELEMETRY_EVENTS.REVIEW_AI_SUMMARY_SHOWN, {
    worktree_fingerprint: a.worktreeFingerprint,
    is_partial: a.isPartial,
    section_count: bucketLarge(a.sectionCount),
    lens_id: a.lensId ?? null,
  })
}
