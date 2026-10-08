/**
 * merge-review-report-into-body.ts — FE-CV-TASK-090-07
 *
 * Idempotent insert of the review report into a PR/MR description: replaces the
 * marked block, or appends one. Text the user wrote outside the markers is untouched.
 *
 * @module components/review-map/report/merge-review-report-into-body
 */

import { REVIEW_REPORT_END_MARKER, REVIEW_REPORT_START_MARKER } from './review-report-markdown'

export function bodyHasReviewReport(body: string): boolean {
  const start = body.indexOf(REVIEW_REPORT_START_MARKER)
  return start !== -1 && body.includes(REVIEW_REPORT_END_MARKER, start)
}

/** `markdown` must already be wrapped in the start/end markers (the builder does that). */
export function mergeReviewReportIntoBody(body: string, markdown: string): string {
  const usesCrlf = body.includes('\r\n')
  const text = body.replace(/\r\n/g, '\n')
  const start = text.indexOf(REVIEW_REPORT_START_MARKER)
  // Last end marker: if a user pasted two blocks, both are replaced by one.
  const end = text.lastIndexOf(REVIEW_REPORT_END_MARKER)

  let merged: string
  if (start !== -1 && end > start) {
    merged = text.slice(0, start) + markdown + text.slice(end + REVIEW_REPORT_END_MARKER.length)
  } else {
    const base = text.trimEnd()
    merged = base ? `${base}\n\n${markdown}` : markdown
  }
  return usesCrlf ? merged.replace(/\n/g, '\r\n') : merged
}
