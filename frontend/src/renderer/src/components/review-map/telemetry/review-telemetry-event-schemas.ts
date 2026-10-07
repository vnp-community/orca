/**
 * review-telemetry-event-schemas.ts — FE-CV-TASK-095-01
 *
 * Telemetry event schemas for the Review workspace.
 *
 * Privacy rules (CR-095):
 * - NO agent_type, model, or turn IDs in any event
 * - NO raw file paths (use hashed path fingerprints)
 * - NO user-authored text (prompt, comments, PR body)
 *
 * @module components/review-map/telemetry/review-telemetry-event-schemas
 */

// ---------------------------------------------------------------------------
// Event name constants
// ---------------------------------------------------------------------------

export const REVIEW_TELEMETRY_EVENTS = {
  REVIEW_TAB_OPENED: 'review_tab_opened',
  REVIEW_LENS_CHANGED: 'review_lens_changed',
  REVIEW_FINDING_OPENED: 'review_finding_opened',
  REVIEW_READING_PROGRESS_SAVED: 'review_reading_progress_saved',
  REVIEW_QUALITY_GATE_VIEWED: 'review_quality_gate_viewed',
  REVIEW_REPORT_EXPORTED: 'review_report_exported',
  REVIEW_REINDEX_REQUESTED: 'review_reindex_requested',
  REVIEW_AI_SUMMARY_SHOWN: 'review_ai_summary_shown'
} as const

export type ReviewTelemetryEventName = typeof REVIEW_TELEMETRY_EVENTS[keyof typeof REVIEW_TELEMETRY_EVENTS]

// ---------------------------------------------------------------------------
// Event payload types
// ---------------------------------------------------------------------------

/** Common fields (never includes agent_type, model, turnId) */
type ReviewTelemetryCommon = {
  /** Worktree fingerprint hash — NOT the raw worktree ID */
  worktreeFingerprint: string
  /** Lens or area identifier */
  lensId?: string
}

export type ReviewTabOpenedEvent = ReviewTelemetryCommon & {
  trigger: 'click' | 'shortcut' | 'rightSplit' | 'navigate'
}

export type ReviewLensChangedEvent = ReviewTelemetryCommon & {
  lensId: string
  previousLensId: string | null
}

export type ReviewFindingOpenedEvent = ReviewTelemetryCommon & {
  /** Finding category — NOT the finding text or path */
  findingCategory: string
  severity: string
}

export type ReviewReadingProgressSavedEvent = ReviewTelemetryCommon & {
  totalFiles: number
  readFiles: number
  skippedFiles: number
}

export type ReviewQualityGateViewedEvent = ReviewTelemetryCommon & {
  overallRisk: string
}

export type ReviewReportExportedEvent = ReviewTelemetryCommon & {
  format: 'markdown' | 'html'
  sectionCount: number
}

export type ReviewReindexRequestedEvent = ReviewTelemetryCommon & {
  trigger: 'button' | 'auto'
}

export type ReviewAiSummaryShownEvent = ReviewTelemetryCommon & {
  isPartial: boolean
  sectionCount: number
  /** No model/agent_type — only the section count */
}
