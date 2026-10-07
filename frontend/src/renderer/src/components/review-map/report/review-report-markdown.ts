/**
 * review-report-markdown.ts — FE-CV-TASK-090-02
 *
 * Builds Markdown text for PR/MR descriptions from a ReviewReportModel.
 *
 * @module components/review-map/report/review-report-markdown
 */

import type { ReviewReportModel, ReviewReportSection } from './review-report-model-parser'
import { guardMermaidDiagram } from './review-report-diagram-guard'

// ---------------------------------------------------------------------------
// Sentinel comment (used for merge/replace in PR body)
// ---------------------------------------------------------------------------

export const REVIEW_REPORT_BEGIN_MARKER = '<!-- orca-review-report:begin -->'
export const REVIEW_REPORT_END_MARKER = '<!-- orca-review-report:end -->'

// ---------------------------------------------------------------------------
// Markdown builder
// ---------------------------------------------------------------------------

/**
 * Build a Markdown string for a review report section.
 * Diagram is only included if it passes the guard.
 */
function renderSection(section: ReviewReportSection): string {
  const lines: string[] = []
  lines.push(`### ${section.title}`)
  lines.push('')
  lines.push(section.body.trim())
  lines.push('')

  const guardedDiagram = guardMermaidDiagram(section.diagram)
  if (guardedDiagram) {
    lines.push('```mermaid')
    lines.push(guardedDiagram)
    lines.push('```')
    lines.push('')
  }

  return lines.join('\n')
}

function renderChecklist(items: ReviewReportModel['summary']['checklistItems']): string {
  if (!items.length) return ''
  return items.map((item) => `- [${item.checked ? 'x' : ' '}] ${item.text}`).join('\n')
}

/**
 * Build the full Markdown report wrapped in sentinel markers.
 * Existing markers in a body are replaced by mergeReviewReportIntoBody.
 */
export function buildReviewReportMarkdown(model: ReviewReportModel): string {
  const { title, description, summary, sections } = model

  const parts: string[] = []

  parts.push(REVIEW_REPORT_BEGIN_MARKER)
  parts.push('')
  parts.push(`## ${title}`)
  parts.push('')

  if (description.trim()) {
    parts.push(description.trim())
    parts.push('')
  }

  // Summary section
  parts.push('### Overview')
  parts.push('')
  parts.push(`**Risk:** ${summary.overallRisk}`)
  parts.push('')

  if (summary.testedBehavior.length > 0) {
    parts.push('**Tested behavior:**')
    parts.push(summary.testedBehavior.map((s) => `- ${s}`).join('\n'))
    parts.push('')
  }

  if (summary.untestedBehavior.length > 0) {
    parts.push('**Untested behavior:**')
    parts.push(summary.untestedBehavior.map((s) => `- ${s}`).join('\n'))
    parts.push('')
  }

  const checklist = renderChecklist(summary.checklistItems)
  if (checklist) {
    parts.push('**Review checklist:**')
    parts.push(checklist)
    parts.push('')
  }

  // Content sections
  for (const section of sections) {
    parts.push(renderSection(section))
  }

  parts.push(REVIEW_REPORT_END_MARKER)

  return parts.join('\n')
}

/**
 * Merge a review report into an existing PR body.
 * Replaces content between sentinel markers if they exist,
 * or appends at the end if not.
 */
export function mergeReviewReportIntoBody(
  existingBody: string,
  reportMarkdown: string
): string {
  const beginIdx = existingBody.indexOf(REVIEW_REPORT_BEGIN_MARKER)
  const endIdx = existingBody.indexOf(REVIEW_REPORT_END_MARKER)

  if (beginIdx !== -1 && endIdx !== -1 && endIdx > beginIdx) {
    // Replace existing report block
    const before = existingBody.slice(0, beginIdx)
    const after = existingBody.slice(endIdx + REVIEW_REPORT_END_MARKER.length)
    return (before + reportMarkdown + after).trimEnd()
  }

  // Append at end
  const trimmed = existingBody.trimEnd()
  return trimmed ? `${trimmed}\n\n${reportMarkdown}` : reportMarkdown
}
