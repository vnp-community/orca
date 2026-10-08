import { describe, expect, it } from 'vitest'
import { parseReviewReportModel } from './review-report-model-parser'
import { reviewReportWire } from './review-report-model.fixture'

describe('parseReviewReportModel', () => {
  it('parses a full contract payload', () => {
    const model = parseReviewReportModel(reviewReportWire())
    expect(model.subject.branch).toBe('feature/login')
    expect(model.gate).toMatchObject({ verdict: 'fail', waivers: { count: 1 } })
    expect(model.gate.reasons[0]).toMatchObject({ check: 'lint', code: 'errors_over_limit' })
    expect(model.findings.top).toHaveLength(1)
    expect(model.diagrams[0]).toMatchObject({ kind: 'components', truncated: false })
    expect(model.risk.level).toBe('MEDIUM')
  })

  it.each([null, undefined, 42, 'x', [], {}])('never throws and yields empty defaults for %j', (raw) => {
    const model = parseReviewReportModel(raw)
    expect(model.gate.verdict).toBe('unknown')
    expect(model.risk.level).toBe('UNKNOWN')
    expect(model.findings.top).toEqual([])
    expect(model.warnings).toContain('no_gate')
  })

  it('maps unknown enums to unknown, never to a pass', () => {
    const model = parseReviewReportModel(
      reviewReportWire({
        risk: { level: 'low' }, // contract is upper-case
        gate: { verdict: 'passed!', reasons: [{ check: 'x', result: 'weird' }] },
        diagrams: [{ kind: 'sankey', mermaid: '', alt: [] }]
      })
    )
    expect(model.risk.level).toBe('UNKNOWN')
    expect(model.gate.verdict).toBe('unknown')
    expect(model.gate.reasons[0].result).toBe('unknown')
    expect(model.diagrams[0].kind).toBe('unknown')
  })

  it('drops wrongly typed values and clamps counts to non-negative integers', () => {
    const model = parseReviewReportModel(
      reviewReportWire({
        summary: { files: -4, added: 'x', removed: 2.9, symbols: Infinity, flows: null, components: [1, 'ok'] },
        warnings: ['index_stale', 7]
      })
    )
    expect(model.summary).toMatchObject({ files: 0, added: 0, removed: 2, symbols: 0, flows: 0, components: ['ok'] })
    expect(model.warnings).toEqual(['index_stale'])
  })
})
