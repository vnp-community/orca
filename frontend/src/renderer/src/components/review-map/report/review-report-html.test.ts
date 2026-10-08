import { describe, expect, it } from 'vitest'
import { buildReviewReportHtml, escapeHtml } from './review-report-html'
import { reviewReportFixture } from './review-report-model.fixture'
import type { ReportThemeTokens } from './review-report-theme-tokens'

const t = (_key: string, fallback: string, opts?: Record<string, unknown>) =>
  fallback.replace(/\{\{(\w+)\}\}/g, (_m, name: string) => String(opts?.[name] ?? ''))

const tokens: ReportThemeTokens = {
  light: { background: 'Canvas', foreground: 'CanvasText', muted: 'Canvas', mutedForeground: 'GrayText', border: 'GrayText', destructive: 'CanvasText' },
  dark: { background: 'Canvas', foreground: 'CanvasText', muted: 'Canvas', mutedForeground: 'GrayText', border: 'GrayText', destructive: 'CanvasText' }
}

function html(model = reviewReportFixture(), svg: (string | null)[] = [null]) {
  return buildReviewReportHtml(model, { t, locale: 'es', tokens, svgByDiagram: svg })
}

describe('buildReviewReportHtml', () => {
  it('is self-contained: strict CSP, no script, no external URL', () => {
    const out = html()
    expect(out).toContain(`content="default-src 'none'; img-src data:; style-src 'unsafe-inline'"`)
    expect(out).not.toMatch(/<script/i)
    expect(out).not.toMatch(/\bsrc\s*=|\bhref\s*=|url\(|@import/i)
    expect(out).not.toMatch(/https?:\/\//i)
    expect(out).toContain('<html lang="es">')
  })

  it('escapes model text (XSS)', () => {
    const out = html(
      reviewReportFixture({
        subject: { branch: '"><svg onload=alert(1)>', baseRef: '<script>x</script>', headCommit: 'abc' },
        findings: {
          counts: {},
          top: [{ ruleId: '<b>r</b>', severity: 'error', file: '<img src=x onerror=alert(1)>', line: 1, message: '<script>alert(1)</script>' }]
        },
        readingOrder: [{ n: 1, file: '</code><script>', reason: '<iframe>', symbols: [] }],
        diagrams: [{ kind: 'flow', mermaid: 'graph TD', alt: ['<img onerror=1>'], truncated: false }]
      })
    )
    expect(out).not.toMatch(/<script/i)
    expect(out).not.toContain('<img')
    expect(out).not.toContain('<svg onload')
    expect(out).not.toContain('<iframe')
    expect(out).toContain('&lt;script&gt;alert(1)&lt;/script&gt;')
  })

  it('has table captions and header scopes for accessibility', () => {
    const out = html()
    expect(out).toContain('<caption>Top findings</caption>')
    expect(out).toContain('<th scope="col">Severity</th>')
  })

  it('embeds pre-sanitized SVG inside a labelled image role', () => {
    const out = html(reviewReportFixture(), ['<svg><title>x</title></svg>'])
    expect(out).toContain('role="img"')
    expect(out).toContain('<svg><title>x</title></svg>')
    expect(out).toContain('A calls B')
  })

  it('writes light and dark token blocks and contains no color literals', () => {
    const out = html()
    expect(out).toContain('--bg:Canvas;')
    expect(out).toContain('prefers-color-scheme: dark')
    expect(out).not.toMatch(/#[0-9a-fA-F]{3,8}\b|rgb\(|hsl\(/)
  })

  it('neutralizes token values that could break out of the style block', () => {
    const evil = { ...tokens, light: { ...tokens.light, background: 'red;}</style><script>x</script>' } }
    const out = buildReviewReportHtml(reviewReportFixture(), { t, locale: 'en', tokens: evil, svgByDiagram: [] })
    expect(out).not.toMatch(/<script/i)
    expect(out).toContain('--bg:inherit;')
  })

  it('states unknown gates as not enough data', () => {
    const out = html(reviewReportFixture({ gate: { verdict: 'unknown' } }))
    expect(out).toContain('Not enough data to conclude.')
    expect(out).not.toContain('checks that ran passed')
  })

  it('escapeHtml handles all five characters', () => {
    expect(escapeHtml(`<>&"'`)).toBe('&lt;&gt;&amp;&quot;&#39;')
  })
})
