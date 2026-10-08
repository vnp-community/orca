import { describe, expect, it } from 'vitest'
import {
  buildReviewReportMarkdown,
  escapeMarkdownText,
  REVIEW_REPORT_END_MARKER,
  REVIEW_REPORT_START_MARKER
} from './review-report-markdown'
import { reviewReportFixture } from './review-report-model.fixture'

// Identity translator: tests assert on the English fallbacks.
const t = (_key: string, fallback: string, opts?: Record<string, unknown>) =>
  fallback.replace(/\{\{(\w+)\}\}/g, (_m, name: string) => String(opts?.[name] ?? ''))

const opts = { t, reviewLabel: 'pull request' }

function count(text: string, needle: string): number {
  return text.split(needle).length - 1
}

describe('buildReviewReportMarkdown', () => {
  it('wraps the report in exactly one marker pair and includes the key sections', () => {
    const { markdown, truncated } = buildReviewReportMarkdown(reviewReportFixture(), opts)
    expect(truncated).toBe(false)
    expect(count(markdown, REVIEW_REPORT_START_MARKER)).toBe(1)
    expect(count(markdown, REVIEW_REPORT_END_MARKER)).toBe(1)
    expect(markdown).toContain('Orca review for this pull request')
    expect(markdown).toContain('The quality gate failed.')
    expect(markdown).toContain('| Severity | Rule | Location | Message |')
    expect(markdown).toContain('```mermaid')
    expect(markdown).toContain('A calls B')
    expect(markdown).toContain('1 active waiver(s).')
  })

  it('uses the provider word it is given (merge request)', () => {
    const { markdown } = buildReviewReportMarkdown(reviewReportFixture(), { t, reviewLabel: 'merge request' })
    expect(markdown).toContain('Orca review for this merge request')
  })

  it('reports unknown as not enough data and never claims safety', () => {
    const { markdown } = buildReviewReportMarkdown(reviewReportFixture({ gate: { verdict: 'unknown' } }), opts)
    expect(markdown).toContain('Not enough data to conclude.')
    expect(markdown.toLowerCase()).not.toMatch(/safe to merge|approved|ready to merge|all clear/)
    expect(markdown).toContain('not an approval')
  })

  it('does not print waiver reasons or people', () => {
    const wire = reviewReportFixture({
      gate: {
        verdict: 'warn',
        reasons: [],
        waivers: { count: 2, earliestExpiry: '2026-11-01T00:00:00Z', reason: 'SECRET-REASON', createdBy: 'alice' }
      }
    })
    const { markdown } = buildReviewReportMarkdown(wire, opts)
    expect(markdown).not.toContain('SECRET-REASON')
    expect(markdown).not.toContain('alice')
  })

  it('escapes table and inline characters from model text', () => {
    const model = reviewReportFixture({
      findings: {
        counts: { error: 1 },
        top: [{ ruleId: 'r|1', severity: 'error', file: 'a`b.ts', line: 1, message: '<img onerror=x> [link](http://x) | pipe\nnewline' }]
      }
    })
    const { markdown } = buildReviewReportMarkdown(model, opts)
    const row = markdown.split('\n').find((l) => l.includes('r\\|1')) ?? ''
    expect(row).toBeTruthy()
    expect(row).not.toMatch(/(^|[^\\])<img/)
    expect(row).toContain('\\<img')
    expect(row).toContain('\\[link\\]')
    expect(row.split('\n')).toHaveLength(1)
    expect(escapeMarkdownText('a|b')).toBe('a\\|b')
  })

  it('drops unsafe diagrams but keeps their alt text', () => {
    const model = reviewReportFixture({
      diagrams: [{ kind: 'flow', mermaid: '%%{init: {}}%%\ngraph TD\nA-->B', alt: ['A calls B'], truncated: false }]
    })
    const { markdown } = buildReviewReportMarkdown(model, opts)
    expect(markdown).not.toContain('```mermaid')
    expect(markdown).not.toContain('%%{')
    expect(markdown).toContain('A calls B')
  })

  it('omits diagrams entirely when includeDiagrams is false', () => {
    const { markdown } = buildReviewReportMarkdown(reviewReportFixture(), { ...opts, includeDiagrams: false })
    expect(markdown).not.toContain('Diagrams')
  })

  it('places extra sections (AI summary) in the block', () => {
    const { markdown } = buildReviewReportMarkdown(reviewReportFixture(), {
      ...opts,
      extraSections: [{ id: 'ai', markdown: '### AI-inferred summary\ntext' }]
    })
    expect(markdown).toContain('### AI-inferred summary')
    expect(markdown.indexOf('AI-inferred')).toBeGreaterThan(markdown.indexOf(REVIEW_REPORT_START_MARKER))
  })

  describe('budget', () => {
    const big = reviewReportFixture({
      findings: {
        counts: { error: 50 },
        top: Array.from({ length: 50 }, (_, i) => ({
          ruleId: `rule-${i}`, severity: 'error', file: `src/file-${i}.ts`, line: i + 1, message: 'm'.repeat(300)
        }))
      },
      readingOrder: Array.from({ length: 50 }, (_, i) => ({ n: i, file: `src/f${i}.ts`, reason: 'r'.repeat(200), symbols: [] })),
      diagrams: [{ kind: 'components', mermaid: `flowchart TD\n${'A-->B\n'.repeat(500)}`, alt: Array.from({ length: 40 }, () => 'alt line'), truncated: false }]
    })

    it.each([2500, 4000, 8000, 20000])('stays within %d chars and keeps both markers', (maxChars) => {
      const { markdown } = buildReviewReportMarkdown(big, { ...opts, maxChars })
      expect(markdown.length).toBeLessThanOrEqual(maxChars)
      expect(markdown.startsWith(REVIEW_REPORT_START_MARKER)).toBe(true)
      expect(markdown.endsWith(REVIEW_REPORT_END_MARKER)).toBe(true)
    })

    it('flags truncation, keeps gate and footer, and never splits a table or a fence', () => {
      const { markdown, truncated } = buildReviewReportMarkdown(big, { ...opts, maxChars: 3000 })
      expect(truncated).toBe(true)
      expect(markdown).toContain('Shortened; see the full report in Orca.')
      expect(markdown).toContain('The quality gate failed.')
      expect(markdown).toContain('not an approval')
      expect(count(markdown, '```') % 2).toBe(0)
      const tableLines = markdown.split('\n').filter((l) => l.startsWith('|'))
      for (const line of tableLines) {expect(line.endsWith('|')).toBe(true)}
    })

    it('drops diagrams before findings', () => {
      const full = buildReviewReportMarkdown(big, opts).markdown
      expect(full).toContain('```mermaid')
      const { markdown } = buildReviewReportMarkdown(big, { ...opts, maxChars: full.length - 50 })
      expect(markdown).not.toContain('```mermaid')
      expect(markdown).toContain('| Severity |')
    })
  })
})
