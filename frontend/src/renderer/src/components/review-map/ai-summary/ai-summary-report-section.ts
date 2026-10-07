/**
 * ai-summary-report-section.ts — FE-CV-TASK-093-05
 *
 * Builds a labeled Markdown section for the AI summary, suitable for
 * inclusion in the review report (090). Section uses a sentinel comment
 * so the report builder can identify and merge it.
 *
 * @module components/review-map/ai-summary/ai-summary-report-section
 */

import type { AiSummaryModel } from './ai-summary-wire-parser'

// ---------------------------------------------------------------------------
// Sentinel markers (aligned with review-report-markdown.ts pattern)
// ---------------------------------------------------------------------------

export const AI_SUMMARY_SECTION_BEGIN = '<!-- orca-ai-summary:begin -->'
export const AI_SUMMARY_SECTION_END = '<!-- orca-ai-summary:end -->'

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

/**
 * Build a labeled Markdown block for the AI summary.
 * Includes sentinel markers for idempotent insertion into larger reports.
 *
 * Language rule: never claim "AI reviewed" — use "AI Summary" label only.
 */
export function buildAiSummaryReportSection(model: AiSummaryModel): string {
  const parts: string[] = []

  parts.push(AI_SUMMARY_SECTION_BEGIN)
  parts.push('')
  parts.push('## AI Summary')
  parts.push('')

  if (model.title) {
    parts.push(`_${model.title}_`)
    parts.push('')
  }

  if (model.summary) {
    parts.push(model.summary)
    parts.push('')
  }

  for (const section of model.sections) {
    parts.push(`### ${section.title}`)
    parts.push('')
    parts.push(section.body)
    parts.push('')
  }

  parts.push(AI_SUMMARY_SECTION_END)

  return parts.join('\n')
}

/**
 * Merge (replace or append) an AI summary section into an existing body.
 * Idempotent: replaces existing section if markers found.
 */
export function mergeAiSummaryIntoBody(
  existingBody: string,
  model: AiSummaryModel
): string {
  const section = buildAiSummaryReportSection(model)
  const beginIdx = existingBody.indexOf(AI_SUMMARY_SECTION_BEGIN)
  const endIdx = existingBody.indexOf(AI_SUMMARY_SECTION_END)

  if (beginIdx !== -1 && endIdx !== -1 && endIdx > beginIdx) {
    const before = existingBody.slice(0, beginIdx)
    const after = existingBody.slice(endIdx + AI_SUMMARY_SECTION_END.length)
    return (before + section + after).trimEnd()
  }

  const trimmed = existingBody.trimEnd()
  return trimmed ? `${trimmed}\n\n${section}` : section
}
