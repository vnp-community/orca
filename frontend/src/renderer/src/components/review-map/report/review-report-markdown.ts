/**
 * review-report-markdown.ts — FE-CV-TASK-090-02
 *
 * Builds the PR/MR description block from a ReviewReportModel. The backend returns
 * the model, the frontend writes the words (one builder, localized via `t`).
 *
 * Wording rule: only state what the checks showed. `unknown` reads as "not enough
 * data", and nothing here says a change is safe, approved or ready to merge.
 *
 * @module components/review-map/report/review-report-markdown
 */

import { guardMermaidSource } from './review-report-diagram-guard'
import type { ReviewReportModel } from './review-report-model-parser'

export const REVIEW_REPORT_START_MARKER = '<!-- orca-review:start -->'
export const REVIEW_REPORT_END_MARKER = '<!-- orca-review:end -->'
export const DEFAULT_REPORT_MAX_CHARS = 20_000

export type ReportTranslate = (key: string, fallback: string, opts?: Record<string, unknown>) => string

export type ReviewReportMarkdownOptions = {
  t: ReportTranslate
  /** Localized word for pull/merge request, e.g. "merge request". */
  reviewLabel: string
  maxChars?: number
  includeDiagrams?: boolean
  /** Extra blocks (e.g. the AI summary, SOL-093) placed after the gate section. */
  extraSections?: { id: string; markdown: string }[]
}

const K = 'auto.components.reviewMap.report.md'
const MAX_REASONS = 5
const MAX_FINDINGS = 10
const MAX_READING = 15
const MAX_ALT_LINES = 12
const MAX_CELL_CHARS = 160

/** Make untrusted text safe for a Markdown table cell or list item. */
export function escapeMarkdownText(value: string, maxChars = MAX_CELL_CHARS): string {
  const flat = value.replace(/[\r\n\t]+/g, ' ').trim()
  const cut = flat.length > maxChars ? `${flat.slice(0, maxChars - 1)}…` : flat
  return cut.replace(/[\\`|<>[\]]/g, (c) => `\\${c}`)
}

type Block = { id: string; priority: number; markdown: string }

function codeSpan(value: string, maxChars: number): string {
  return `\`${value.replace(/[`\r\n\t]+/g, ' ').trim().slice(0, maxChars)}\``
}

export function buildReviewReportMarkdown(
  model: ReviewReportModel,
  opts: ReviewReportMarkdownOptions
): { markdown: string; truncated: boolean } {
  const { t, reviewLabel } = opts
  const maxChars = opts.maxChars ?? DEFAULT_REPORT_MAX_CHARS
  const blocks: Block[] = []
  const add = (id: string, priority: number, lines: (string | null)[]): void => {
    const text = lines.filter((l): l is string => l !== null).join('\n').trim()
    if (text) {blocks.push({ id, priority, markdown: text })}
  }

  // Priority: lower number = kept longest when trimming to fit.
  add('title', 0, [
    `## ${t(`${K}.title`, 'Orca review for this {{review}}', { review: reviewLabel })}`,
    model.subject.branch || model.subject.headCommit
      ? t(`${K}.subject`, 'Branch {{branch}} against {{base}}, head {{head}}.', {
          branch: codeSpan(model.subject.branch, 80),
          base: codeSpan(model.subject.baseRef, 80),
          head: codeSpan(model.subject.headCommit.slice(0, 7), 7)
        })
      : null
  ])

  const verdictText: Record<string, string> = {
    pass: t(`${K}.gate.pass`, 'The required checks that ran passed.'),
    warn: t(`${K}.gate.warn`, 'The quality gate has warnings.'),
    fail: t(`${K}.gate.fail`, 'The quality gate failed.'),
    unknown: t(`${K}.gate.unknown`, 'Not enough data to conclude.')
  }
  add('gate', 0, [
    `### ${t(`${K}.gate.heading`, 'Quality gate')}`,
    verdictText[model.gate.verdict],
    ...model.gate.reasons
      .slice(0, MAX_REASONS)
      .map((r) => `- ${escapeMarkdownText(r.check || r.code || '')}: ${escapeMarkdownText(verdictText[r.result] ?? '')}`),
    model.gate.reasons.length > MAX_REASONS
      ? t(`${K}.gate.moreReasons`, '…and {{count}} more.', { count: model.gate.reasons.length - MAX_REASONS })
      : null,
    // Why: the count is informational; waiver reasons and who granted them stay out of a public description.
    model.gate.waivers.count > 0
      ? t(`${K}.gate.waivers`, '{{count}} active waiver(s).', { count: model.gate.waivers.count })
      : null
  ])

  for (const extra of opts.extraSections ?? []) {
    add(`extra:${extra.id}`, 1, [extra.markdown])
  }

  add('risk', 1, [
    `### ${t(`${K}.risk.heading`, 'Risk')}`,
    t(`${K}.risk.level.${model.risk.level}`, riskFallback(model.risk.level)),
    ...model.risk.reasons.slice(0, MAX_REASONS).map((r) => `- ${escapeMarkdownText(r.code)}`)
  ])

  add('summary', 1, [
    `### ${t(`${K}.summary.heading`, 'Changes')}`,
    t(`${K}.summary.counts`, '{{files}} file(s), +{{added}} / -{{removed}}, {{symbols}} symbol(s), {{flows}} flow(s).', {
      files: model.summary.files,
      added: model.summary.added,
      removed: model.summary.removed,
      symbols: model.summary.symbols,
      flows: model.summary.flows
    }),
    model.summary.components.length > 0
      ? t(`${K}.summary.components`, 'Components: {{list}}.', {
          list: model.summary.components.slice(0, 12).map((c) => escapeMarkdownText(c, 60)).join(', ')
        })
      : null
  ])

  const { counts, top } = model.findings
  add('findings', 2, [
    `### ${t(`${K}.findings.heading`, 'Findings')}`,
    t(`${K}.findings.counts`, '{{error}} error(s), {{warning}} warning(s), {{info}} info.', counts),
    top.length > 0
      ? [
          `| ${t(`${K}.findings.colSeverity`, 'Severity')} | ${t(`${K}.findings.colRule`, 'Rule')} | ${t(`${K}.findings.colLocation`, 'Location')} | ${t(`${K}.findings.colMessage`, 'Message')} |`,
          '| --- | --- | --- | --- |',
          ...top.slice(0, MAX_FINDINGS).map(
            (f) =>
              `| ${escapeMarkdownText(f.severity, 20)} | ${escapeMarkdownText(f.ruleId, 60)} | ${escapeMarkdownText(
                f.line > 0 ? `${f.file}:${f.line}` : f.file,
                80
              )} | ${escapeMarkdownText(f.message)} |`
          )
        ].join('\n')
      : null,
    model.limits.truncated.findings ? t(`${K}.findings.truncated`, 'Only the top findings are listed.') : null
  ])

  const breaking = t(`${K}.contracts.breaking`, 'breaking')
  const contractLines = [
    ...model.contracts.protoRpc.map((c) => `- RPC ${escapeMarkdownText(c.name, 80)}: ${escapeMarkdownText(c.change, 40)}${c.breaking ? ` (${breaking})` : ''}`),
    ...model.contracts.wsChannels.map((c) => `- ${t(`${K}.contracts.channel`, 'Channel')} ${escapeMarkdownText(c.name, 80)}: ${escapeMarkdownText(c.change, 40)}${c.breaking ? ` (${breaking})` : ''}`),
    ...model.contracts.tables.map((c) => `- ${t(`${K}.contracts.table`, 'Table')} ${escapeMarkdownText(c.table, 60)} (${escapeMarkdownText(c.service, 40)}): ${escapeMarkdownText(c.op, 20)}${c.breaking ? ` (${breaking})` : ''}`)
  ]
  if (contractLines.length > 0) {
    add('contracts', 3, [`### ${t(`${K}.contracts.heading`, 'Contracts and data')}`, ...contractLines.slice(0, 25)])
  }

  if (model.readingOrder.length > 0) {
    add('reading', 4, [
      `### ${t(`${K}.reading.heading`, 'Suggested reading order')}`,
      ...model.readingOrder
        .slice(0, MAX_READING)
        .map((s, i) => `${i + 1}. ${codeSpan(s.file, 100)} — ${escapeMarkdownText(s.reason, 100)}`)
    ])
  }

  if ((opts.includeDiagrams ?? true) && model.diagrams.length > 0) {
    const parts: string[] = [`### ${t(`${K}.diagrams.heading`, 'Diagrams')}`]
    for (const diagram of model.diagrams) {
      const guarded = diagram.truncated ? null : guardMermaidSource(diagram.mermaid)
      if (guarded?.ok) {
        parts.push('```mermaid', guarded.src, '```')
      }
      if (diagram.alt.length > 0) {
        parts.push(
          '<details>',
          `<summary>${t(`${K}.diagrams.alt`, 'Text description')}</summary>`,
          '',
          ...diagram.alt.slice(0, MAX_ALT_LINES).map((line) => `- ${escapeMarkdownText(line)}`),
          '',
          '</details>'
        )
      }
    }
    if (parts.length > 1) {
      add('diagrams', 5, parts)
    }
  }

  const warningLines = model.warnings
    .slice(0, 6)
    .map((w) => `- ${t(`${K}.warning.${w}`, escapeMarkdownText(w, 80))}`)
  add('warnings', 1, warningLines.length > 0 ? [`### ${t(`${K}.warnings.heading`, 'Notes')}`, ...warningLines] : [])

  // Footer is never trimmed: it records what this report is (and is not).
  const footer = t(
    `${K}.footer`,
    'Generated by Orca from profile {{profile}} (model {{digest}}). This is a record of checks that ran, not an approval.',
    {
      profile: escapeMarkdownText(model.reproducibility.profileRef || '—', 60),
      digest: escapeMarkdownText(model.reproducibility.modelDigest.slice(0, 12) || '—', 12)
    }
  )
  const truncationNote = t(`${K}.truncated`, 'Shortened; see the full report in Orca.')

  const assemble = (kept: Block[], truncated: boolean): string =>
    [
      REVIEW_REPORT_START_MARKER,
      kept.map((b) => b.markdown).join('\n\n'),
      truncated ? `_${truncationNote}_` : null,
      `<sub>${footer}</sub>`,
      REVIEW_REPORT_END_MARKER
    ]
      .filter((p): p is string => p !== null)
      .join('\n\n')

  let kept = [...blocks]
  let markdown = assemble(kept, false)
  let truncated = false
  // Drop whole blocks, lowest priority first, so no table or code fence is cut in half.
  while (markdown.length > maxChars && kept.some((b) => b.priority > 0)) {
    const worst = Math.max(...kept.map((b) => b.priority))
    const index = kept.map((b) => b.priority).lastIndexOf(worst)
    kept = kept.filter((_, i) => i !== index)
    truncated = true
    markdown = assemble(kept, true)
  }
  return { markdown, truncated }
}

function riskFallback(level: string): string {
  switch (level) {
    case 'LOW':
      return 'Risk level: low.'
    case 'MEDIUM':
      return 'Risk level: medium.'
    case 'HIGH':
      return 'Risk level: high.'
    case 'CRITICAL':
      return 'Risk level: critical.'
    default:
      return 'Risk level: not enough data to conclude.'
  }
}
