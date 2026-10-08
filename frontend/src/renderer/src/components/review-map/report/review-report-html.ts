/**
 * review-report-html.ts — FE-CV-TASK-090-04
 *
 * Self-contained, offline HTML export of a ReviewReportModel. No script, no external
 * URL, strict CSP; every model string is HTML-escaped. SVGs arrive pre-sanitized.
 *
 * @module components/review-map/report/review-report-html
 */

import type { ReviewReportModel } from './review-report-model-parser'
import type { ReportColorTokens, ReportThemeTokens } from './review-report-theme-tokens'
import type { ReportTranslate } from './review-report-markdown'

export function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export type ReviewReportHtmlOptions = {
  t: ReportTranslate
  /** UI locale, written to <html lang>. */
  locale: string
  tokens: ReportThemeTokens
  /** Sanitized SVG per model.diagrams[i], or null when it could not be rendered. */
  svgByDiagram: (string | null)[]
}

const K = 'auto.components.reviewMap.report.html'

function cssVars(tokens: ReportColorTokens): string {
  return [
    `--bg:${tokens.background}`,
    `--fg:${tokens.foreground}`,
    `--muted:${tokens.muted}`,
    `--muted-fg:${tokens.mutedForeground}`,
    `--border:${tokens.border}`,
    `--destructive:${tokens.destructive}`
  ]
    .map((v) => `${v};`)
    .join('')
}

// Token values come from computed styles, never from the model; still refuse anything that could close the <style>.
function safeCss(tokens: ReportColorTokens): ReportColorTokens {
  const out = { ...tokens }
  for (const key of Object.keys(out) as (keyof ReportColorTokens)[]) {
    if (/[<>{};]/.test(out[key])) {out[key] = 'inherit'}
  }
  return out
}

export function buildReviewReportHtml(model: ReviewReportModel, opts: ReviewReportHtmlOptions): string {
  const { t } = opts
  const e = escapeHtml
  const gateText: Record<string, string> = {
    pass: t(`${K}.gate.pass`, 'The required checks that ran passed.'),
    warn: t(`${K}.gate.warn`, 'The quality gate has warnings.'),
    fail: t(`${K}.gate.fail`, 'The quality gate failed.'),
    unknown: t(`${K}.gate.unknown`, 'Not enough data to conclude.')
  }
  const title = t(`${K}.title`, 'Orca review report')

  const findingRows = model.findings.top
    .map(
      (f) =>
        `<tr><td>${e(f.severity)}</td><td>${e(f.ruleId)}</td><td>${e(f.line > 0 ? `${f.file}:${f.line}` : f.file)}</td><td>${e(f.message)}</td></tr>`
    )
    .join('')

  const diagrams = model.diagrams
    .map((d, i) => {
      const svg = opts.svgByDiagram[i] ?? null
      const alt = d.alt.length
        ? `<ul>${d.alt.map((line) => `<li>${e(line)}</li>`).join('')}</ul>`
        : ''
      return `<figure>${svg ? `<div role="img" aria-label="${e(d.alt.join('; ') || d.kind)}">${svg}</div>` : ''}<figcaption>${e(d.kind)}</figcaption>${alt}</figure>`
    })
    .join('')

  const reading = model.readingOrder
    .map((s) => `<li><code>${e(s.file)}</code> — ${e(s.reason)}</li>`)
    .join('')

  const list = (items: string[]): string => (items.length ? `<ul>${items.map((x) => `<li>${e(x)}</li>`).join('')}</ul>` : '')
  const contractItems = [
    ...model.contracts.protoRpc.map((c) => `RPC ${c.name}: ${c.change}${c.breaking ? ` (${t(`${K}.breaking`, 'breaking')})` : ''}`),
    ...model.contracts.wsChannels.map((c) => `${c.name}: ${c.change}${c.breaking ? ` (${t(`${K}.breaking`, 'breaking')})` : ''}`),
    ...model.contracts.tables.map((c) => `${c.table} (${c.service}): ${c.op}${c.breaking ? ` (${t(`${K}.breaking`, 'breaking')})` : ''}`)
  ]

  const light = safeCss(opts.tokens.light)
  const dark = safeCss(opts.tokens.dark)

  return `<!doctype html>
<html lang="${e(opts.locale)}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'">
<title>${e(title)}</title>
<style>
:root{${cssVars(light)}color-scheme:light dark}
@media (prefers-color-scheme: dark){:root{${cssVars(dark)}}}
body{font-family:system-ui,sans-serif;background:var(--bg);color:var(--fg);max-width:52rem;margin:0 auto;padding:1.5rem;line-height:1.55}
h1,h2{border-bottom:1px solid var(--border);padding-bottom:.25rem}
table{border-collapse:collapse;width:100%}
caption{text-align:left;color:var(--muted-fg);padding:.25rem 0}
th,td{border:1px solid var(--border);padding:.25rem .5rem;text-align:left;vertical-align:top;word-break:break-word}
th{background:var(--muted)}
code{background:var(--muted);padding:0 .25rem;border-radius:.25rem}
.note{color:var(--muted-fg);font-size:.875rem}
.fail{color:var(--destructive)}
figure{margin:1rem 0;overflow-x:auto}
</style>
</head>
<body>
<h1>${e(title)}</h1>
<p class="note">${e(model.subject.branch)} → ${e(model.subject.baseRef)} · ${e(model.subject.headCommit.slice(0, 7))}</p>
<h2>${e(t(`${K}.gate.heading`, 'Quality gate'))}</h2>
<p class="${model.gate.verdict === 'fail' ? 'fail' : ''}">${e(gateText[model.gate.verdict])}</p>
${list(model.gate.reasons.map((r) => `${r.check || r.code || ''}: ${gateText[r.result] ?? ''}`))}
<h2>${e(t(`${K}.summary.heading`, 'Changes'))}</h2>
<p>${e(t(`${K}.summary.counts`, '{{files}} file(s), +{{added}} / -{{removed}}, {{symbols}} symbol(s), {{flows}} flow(s).', model.summary))}</p>
<h2>${e(t(`${K}.findings.heading`, 'Findings'))}</h2>
<p>${e(t(`${K}.findings.counts`, '{{error}} error(s), {{warning}} warning(s), {{info}} info.', model.findings.counts))}</p>
${findingRows ? `<table><caption>${e(t(`${K}.findings.caption`, 'Top findings'))}</caption><thead><tr><th scope="col">${e(t(`${K}.findings.colSeverity`, 'Severity'))}</th><th scope="col">${e(t(`${K}.findings.colRule`, 'Rule'))}</th><th scope="col">${e(t(`${K}.findings.colLocation`, 'Location'))}</th><th scope="col">${e(t(`${K}.findings.colMessage`, 'Message'))}</th></tr></thead><tbody>${findingRows}</tbody></table>` : ''}
${contractItems.length ? `<h2>${e(t(`${K}.contracts.heading`, 'Contracts and data'))}</h2>${list(contractItems)}` : ''}
${reading ? `<h2>${e(t(`${K}.reading.heading`, 'Suggested reading order'))}</h2><ol>${reading}</ol>` : ''}
${diagrams ? `<h2>${e(t(`${K}.diagrams.heading`, 'Diagrams'))}</h2>${diagrams}` : ''}
${model.warnings.length ? `<h2>${e(t(`${K}.warnings.heading`, 'Notes'))}</h2>${list(model.warnings.map((w) => t(`${K}.warning.${w}`, w)))}` : ''}
<p class="note">${e(t(`${K}.footer`, 'Generated by Orca from profile {{profile}} (model {{digest}}). This is a record of checks that ran, not an approval.', { profile: model.reproducibility.profileRef || '—', digest: model.reproducibility.modelDigest.slice(0, 12) || '—' }))}</p>
</body>
</html>
`
}
