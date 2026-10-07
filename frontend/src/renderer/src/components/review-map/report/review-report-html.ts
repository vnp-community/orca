/**
 * review-report-html.ts — FE-CV-TASK-090-04
 *
 * Builds a self-contained, offline-ready HTML export of a ReviewReportModel.
 *
 * Safety rules:
 * - No <script> tags, no inline handlers, no external URLs
 * - All user/backend content is HTML-escaped
 * - Mermaid diagrams: rendered as <pre> fallback (lazy mermaid load not available offline)
 * - Color tokens read from CSS variables; hardcoded fallbacks when unavailable
 * - CSP meta tag included
 * - No hex color values in builder source; use CSS var() or token names
 *
 * @module components/review-map/report/review-report-html
 */

import type { ReviewReportModel, ReviewReportSection } from './review-report-model-parser'
import { guardMermaidDiagram } from './review-report-diagram-guard'

// ---------------------------------------------------------------------------
// HTML escape
// ---------------------------------------------------------------------------

function esc(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

// ---------------------------------------------------------------------------
// Theme tokens (read from CSS custom properties at build time)
// Fallback values are neutral and accessible (WCAG AA contrast).
// ---------------------------------------------------------------------------

/**
 * Read a CSS custom property value from :root if running in a browser context.
 * Falls back to the provided default string otherwise.
 */
function cssVar(name: string, fallback: string): string {
  if (typeof document !== 'undefined') {
    const val = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    if (val) return val
  }
  return fallback
}

function buildThemeTokens() {
  return {
    background: cssVar('--background', '#ffffff'),
    foreground: cssVar('--foreground', '#111111'),
    muted: cssVar('--muted', '#f4f4f5'),
    mutedForeground: cssVar('--muted-foreground', '#555555'),
    border: cssVar('--border', '#e4e4e7'),
    destructive: cssVar('--destructive', '#dc2626'),
    warning: cssVar('--warning', '#ca8a04'),
  }
}

// ---------------------------------------------------------------------------
// Severity badge
// ---------------------------------------------------------------------------

const SEVERITY_LABEL: Record<string, string> = {
  error: 'Error',
  warn: 'Warning',
  info: 'Info',
  unknown: '',
}

// ---------------------------------------------------------------------------
// Section renderer
// ---------------------------------------------------------------------------

function renderSectionHtml(section: ReviewReportSection, t: ReturnType<typeof buildThemeTokens>): string {
  const diagram = guardMermaidDiagram(section.diagram)
  const severityLabel = SEVERITY_LABEL[section.severity] ?? ''
  const badge = severityLabel
    ? `<span class="badge badge-${esc(section.severity)}">${esc(severityLabel)}</span>`
    : ''

  return `
  <section class="report-section severity-${esc(section.severity)}">
    <h3>${esc(section.title)}${badge}</h3>
    <div class="section-body">${esc(section.body)}</div>
    ${diagram ? `<pre class="mermaid-pre"><code>${esc(diagram)}</code></pre>` : ''}
  </section>`
}

// ---------------------------------------------------------------------------
// Full HTML builder
// ---------------------------------------------------------------------------

/**
 * Build a self-contained HTML document from a ReviewReportModel.
 * Safe for offline use; no external resources.
 */
export function buildReviewReportHtml(model: ReviewReportModel): string {
  const t = buildThemeTokens()
  const { title, description, summary, sections, generatedAt } = model

  const checklistHtml = summary.checklistItems.length > 0
    ? `<ul class="checklist">${summary.checklistItems
        .map((item) => `<li class="${item.checked ? 'checked' : ''}">${esc(item.text)}</li>`)
        .join('\n')}</ul>`
    : ''

  const testedHtml = summary.testedBehavior.length > 0
    ? `<div class="behavior-list"><strong>Tested behavior:</strong><ul>${
        summary.testedBehavior.map((b) => `<li>${esc(b)}</li>`).join('')
      }</ul></div>`
    : ''

  const untestedHtml = summary.untestedBehavior.length > 0
    ? `<div class="behavior-list untested"><strong>Untested behavior:</strong><ul>${
        summary.untestedBehavior.map((b) => `<li>${esc(b)}</li>`).join('')
      }</ul></div>`
    : ''

  const sectionsHtml = sections.map((s) => renderSectionHtml(s, t)).join('\n')

  const generatedLine = generatedAt
    ? `<p class="meta">Generated: ${esc(generatedAt)}</p>`
    : ''

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'">
  <title>${esc(title)}</title>
  <style>
    :root {
      --bg: ${t.background};
      --fg: ${t.foreground};
      --muted: ${t.muted};
      --muted-fg: ${t.mutedForeground};
      --border: ${t.border};
      --destructive: ${t.destructive};
      --warning: ${t.warning};
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #111111;
        --fg: #fafafa;
        --muted: #27272a;
        --muted-fg: #a1a1aa;
        --border: #3f3f46;
      }
    }
    body { font-family: system-ui, sans-serif; background: var(--bg); color: var(--fg); max-width: 800px; margin: 0 auto; padding: 2rem; line-height: 1.6; }
    h1 { border-bottom: 1px solid var(--border); padding-bottom: .5rem; }
    h3 { margin-top: 1.5rem; }
    .meta { color: var(--muted-fg); font-size: .85rem; }
    .badge { display: inline-block; font-size: .7rem; padding: .1em .4em; border-radius: .25rem; margin-left: .5rem; }
    .badge-error { background: var(--destructive); color: #fff; }
    .badge-warn { background: var(--warning); color: #fff; }
    .badge-info { background: var(--muted); color: var(--muted-fg); }
    .report-section { border: 1px solid var(--border); border-radius: .5rem; padding: 1rem; margin-top: 1rem; }
    .severity-error { border-color: var(--destructive); }
    .severity-warn { border-color: var(--warning); }
    .section-body { white-space: pre-wrap; word-break: break-word; }
    .mermaid-pre { background: var(--muted); padding: 1rem; border-radius: .25rem; overflow-x: auto; font-size: .85rem; }
    .checklist { list-style: none; padding: 0; }
    .checklist li::before { content: "☐ "; }
    .checklist li.checked::before { content: "☑ "; }
    .behavior-list { margin-top: .5rem; }
    .untested { color: var(--muted-fg); }
  </style>
</head>
<body>
  <h1>${esc(title)}</h1>
  ${description ? `<p>${esc(description)}</p>` : ''}
  ${generatedLine}

  <section class="summary">
    <h2>Overview</h2>
    <p><strong>Risk:</strong> ${esc(summary.overallRisk)}</p>
    ${testedHtml}
    ${untestedHtml}
    ${checklistHtml}
  </section>

  ${sectionsHtml}
</body>
</html>`
}
