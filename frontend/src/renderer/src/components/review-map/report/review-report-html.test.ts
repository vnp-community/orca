/**
 * review-report-html.test.ts — FE-CV-TASK-090-04
 */

import { describe, it, expect } from 'vitest'
import { buildReviewReportHtml } from './review-report-html'
import type { ReviewReportModel } from './review-report-model-parser'

const BASE_MODEL: ReviewReportModel = {
  title: 'Test Report',
  description: 'A description',
  summary: {
    overallRisk: 'LOW',
    testedBehavior: ['Login flow'],
    untestedBehavior: ['Error recovery'],
    checklistItems: [
      { id: '1', text: 'Code reviewed', checked: true },
      { id: '2', text: 'Tests pass', checked: false },
    ],
  },
  sections: [
    { id: 's1', title: 'Coverage', body: 'Coverage is 85%', severity: 'info', diagram: null },
    {
      id: 's2',
      title: 'Security',
      body: 'No issues',
      severity: 'warn',
      diagram: 'graph TD\n  A --> B',
    },
  ],
  overlay: [],
  headCommit: 'abc123',
  generatedAt: '2026-10-07T10:00:00Z',
  modelVersion: 1,
}

describe('buildReviewReportHtml', () => {
  it('produces valid HTML structure', () => {
    const html = buildReviewReportHtml(BASE_MODEL)
    expect(html).toContain('<!DOCTYPE html>')
    expect(html).toContain('<html lang="en">')
    expect(html).toContain('</html>')
    expect(html).toContain('<title>Test Report</title>')
  })

  it('includes CSP meta tag with no script/external sources', () => {
    const html = buildReviewReportHtml(BASE_MODEL)
    expect(html).toContain("Content-Security-Policy")
    expect(html).toContain("default-src 'none'")
    expect(html).not.toContain('<script')
  })

  it('escapes XSS in title', () => {
    const model: ReviewReportModel = { ...BASE_MODEL, title: '<script>alert(1)</script>' }
    const html = buildReviewReportHtml(model)
    expect(html).not.toContain('<script>')
    expect(html).toContain('&lt;script&gt;')
  })

  it('escapes XSS in section body', () => {
    const model: ReviewReportModel = {
      ...BASE_MODEL,
      sections: [
        { id: 'xss', title: 'XSS Test', body: '<img src=x onerror=alert(1)>', severity: 'info', diagram: null }
      ],
    }
    const html = buildReviewReportHtml(model)
    expect(html).not.toContain('onerror=alert')
    expect(html).toContain('&lt;img')
  })

  it('renders valid mermaid as pre/code block', () => {
    const html = buildReviewReportHtml(BASE_MODEL)
    expect(html).toContain('mermaid-pre')
    expect(html).toContain('graph TD')
  })

  it('skips invalid mermaid diagram', () => {
    const model: ReviewReportModel = {
      ...BASE_MODEL,
      sections: [
        {
          id: 'bad',
          title: 'Bad Diagram',
          body: 'Content',
          severity: 'info',
          diagram: '<script>bad()</script>',
        }
      ],
    }
    const html = buildReviewReportHtml(model)
    expect(html).not.toContain('<script>')
  })

  it('does not contain external URL references', () => {
    const html = buildReviewReportHtml(BASE_MODEL)
    expect(html).not.toMatch(/src\s*=\s*["']https?:/)
    expect(html).not.toMatch(/href\s*=\s*["']https?:/)
  })

  it('includes checklist items', () => {
    const html = buildReviewReportHtml(BASE_MODEL)
    expect(html).toContain('Code reviewed')
    expect(html).toContain('Tests pass')
    expect(html).toContain('checked')
  })

  it('includes overview risk', () => {
    const html = buildReviewReportHtml(BASE_MODEL)
    expect(html).toContain('LOW')
  })

  it('handles empty optional fields', () => {
    const model: ReviewReportModel = {
      ...BASE_MODEL,
      description: '',
      generatedAt: null,
      summary: { ...BASE_MODEL.summary, testedBehavior: [], untestedBehavior: [], checklistItems: [] },
      sections: [],
    }
    expect(() => buildReviewReportHtml(model)).not.toThrow()
  })
})
