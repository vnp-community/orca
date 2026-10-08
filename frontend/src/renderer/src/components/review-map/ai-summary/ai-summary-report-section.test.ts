import { describe, expect, it } from 'vitest'
import { buildAiSummaryReportSection } from './ai-summary-report-section'
import { parseAiSummaryResponse } from './ai-summary-wire-parser'
import { summaryWire } from './ai-summary.fixture'
import { buildReviewReportMarkdown } from '../report/review-report-markdown'
import { reviewReportFixture } from '../report/review-report-model.fixture'

const t = (_key: string, fallback: string, opts?: Record<string, unknown>) =>
  fallback.replace(/\{\{(\w+)\}\}/g, (_m, n: string) => String(opts?.[n] ?? ''))
const summary = (over: Record<string, unknown> = {}) => parseAiSummaryResponse({ summary: summaryWire(over) }).summary!

describe('buildAiSummaryReportSection', () => {
  it('is labelled as inferred with model and level and never claims a review', () => {
    const { markdown } = buildAiSummaryReportSection(summary(), t)
    expect(markdown).toContain('### AI-inferred summary')
    expect(markdown).toContain('Model: claude-x')
    expect(markdown).toContain('Data sent: metadata')
    expect(markdown.toLowerCase()).not.toMatch(/ai reviewed|approved|\bsafe\b|verified/)
  })

  it('neutralizes model text so it cannot open headings, tables, html or links', () => {
    const { markdown } = buildAiSummaryReportSection(
      summary({ summary: '# Heading\n| a | b |\n<script>x</script> [l](http://x)', risks: [{ text: '`code` <b>', refs: [] }] }),
      t
    )
    const body = markdown.split('\n').filter((l) => l.startsWith('>'))
    expect(body).toHaveLength(1)
    expect(markdown).not.toMatch(/(^|[^\\])<script/)
    expect(markdown).not.toMatch(/(^|[^\\])\[l\]/)
    expect(markdown.split('\n').some((l) => l.startsWith('# '))).toBe(false)
  })

  it('lands in the report outside the "Quality gate" section', () => {
    const extra = buildAiSummaryReportSection(summary(), t)
    const { markdown } = buildReviewReportMarkdown(reviewReportFixture(), { t, reviewLabel: 'pull request', extraSections: [extra] })
    const gateStart = markdown.indexOf('### Quality gate')
    const aiStart = markdown.indexOf('### AI-inferred summary')
    expect(aiStart).toBeGreaterThan(gateStart)
    const between = markdown.slice(gateStart, aiStart)
    expect(between).not.toContain('AI-inferred')
    expect(markdown.slice(gateStart, aiStart).split('\n\n').length).toBeGreaterThan(0)
    // The gate section ends before the AI section begins (separate blocks).
    expect(markdown.slice(0, aiStart).endsWith('\n\n')).toBe(true)
  })
})
