/**
 * ai-summary-report-section.ts — FE-CV-TASK-093-05
 *
 * Labelled Markdown block for the AI summary, passed to the report builder through
 * `extraSections`. It is its own section, never part of the "Quality gate" section,
 * and says plainly that it is AI-inferred.
 *
 * @module components/review-map/ai-summary/ai-summary-report-section
 */

import { escapeMarkdownText } from '../report/review-report-markdown'
import type { ReportTranslate } from '../report/review-report-markdown'
import type { AiReviewSummary } from './ai-summary-wire-parser'

const K = 'auto.components.reviewMap.aiSummary.report'

export function buildAiSummaryReportSection(
  summary: AiReviewSummary,
  t: ReportTranslate
): { id: string; markdown: string } {
  const lines: string[] = [
    `### ${t(`${K}.heading`, 'AI-inferred summary')}`,
    `_${t(`${K}.disclaimer`, 'Can be wrong or incomplete; check it against the diff. Model: {{model}}. Data sent: {{level}}.', {
      model: escapeMarkdownText(summary.model || '—', 80),
      level: summary.level
    })}_`,
    '',
    // Blockquoted so model text can never open a heading or list at the top level.
    `> ${escapeMarkdownText(summary.summary, 4000)}`
  ]
  if (summary.risks.length > 0) {
    lines.push('', `**${t(`${K}.risks`, 'Risks')}**`)
    for (const risk of summary.risks.slice(0, 10)) {
      lines.push(`- ${escapeMarkdownText(risk.text, 300)}`)
    }
  }
  if (summary.readFirst.length > 0) {
    lines.push('', `**${t(`${K}.readFirst`, 'Read first')}**`)
    for (const item of summary.readFirst.slice(0, 10)) {
      lines.push(`- ${escapeMarkdownText(item.file, 120)} — ${escapeMarkdownText(item.why, 200)}`)
    }
  }
  return { id: 'ai-summary', markdown: lines.join('\n') }
}
