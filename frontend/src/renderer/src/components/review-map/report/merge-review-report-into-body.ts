/**
 * merge-review-report-into-body.ts — FE-CV-TASK-090-07
 *
 * Standalone function to insert or replace the review report block in a
 * PR/MR description body. Uses sentinel markers for idempotent replacement.
 *
 * Design:
 * - Not a React hook — pure function
 * - Handles CRLF (\r\n) and LF (\n) line endings
 * - Handles two existing blocks gracefully (uses first begin, last end)
 * - Works for both GitHub PR and GitLab MR bodies
 *
 * @module components/review-map/report/merge-review-report-into-body
 */

import {
  REVIEW_REPORT_BEGIN_MARKER,
  REVIEW_REPORT_END_MARKER,
  buildReviewReportMarkdown,
} from './review-report-markdown'
import type { ReviewReportModel } from './review-report-model-parser'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type MergeResult =
  | { replaced: true; body: string }
  | { replaced: false; body: string }

// ---------------------------------------------------------------------------
// Normalization helpers
// ---------------------------------------------------------------------------

function normalizeCRLF(s: string): string {
  return s.replace(/\r\n/g, '\n')
}

function restoreCRLF(s: string, originalHadCRLF: boolean): string {
  return originalHadCRLF ? s.replace(/\n/g, '\r\n') : s
}

// ---------------------------------------------------------------------------
// Core function
// ---------------------------------------------------------------------------

/**
 * Insert the review report into a PR/MR body.
 * If sentinel markers already exist, replace the existing block.
 * If two blocks exist, use the first begin and the last end (defensive).
 * Idempotent: calling twice with the same model produces the same result.
 *
 * @param body - Existing PR/MR body text (may be empty)
 * @param model - Review report model to serialize
 * @returns { replaced, body } — replaced=true if existing block was updated
 */
export function mergeReviewReportIntoBody(
  body: string,
  model: ReviewReportModel
): MergeResult {
  const hadCRLF = body.includes('\r\n')
  const normalized = normalizeCRLF(body)

  const reportMarkdown = buildReviewReportMarkdown(model)

  const beginIdx = normalized.indexOf(REVIEW_REPORT_BEGIN_MARKER)
  // Use lastIndexOf for end marker to handle two blocks gracefully
  const endIdx = normalized.lastIndexOf(REVIEW_REPORT_END_MARKER)

  if (beginIdx !== -1 && endIdx !== -1 && endIdx > beginIdx) {
    const before = normalized.slice(0, beginIdx)
    const after = normalized.slice(endIdx + REVIEW_REPORT_END_MARKER.length)
    const merged = (before + reportMarkdown + after).trimEnd()
    return {
      replaced: true,
      body: restoreCRLF(merged, hadCRLF),
    }
  }

  // Append at end
  const trimmed = normalized.trimEnd()
  const merged = trimmed ? `${trimmed}\n\n${reportMarkdown}` : reportMarkdown
  return {
    replaced: false,
    body: restoreCRLF(merged, hadCRLF),
  }
}

/**
 * Check if a body already contains a review report block.
 */
export function bodyHasReviewReport(body: string): boolean {
  return body.includes(REVIEW_REPORT_BEGIN_MARKER) && body.includes(REVIEW_REPORT_END_MARKER)
}
