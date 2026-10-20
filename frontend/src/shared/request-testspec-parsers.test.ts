import { describe, expect, it } from 'vitest'
import { parseTestSpecs, parseTestSpecDocument } from './request-testspec-parsers'

describe('parseTestSpecs', () => {
  it('reads a finished run with its counts and gaps', () => {
    const t = parseTestSpecs({
      requestId: 'r1',
      enabled: true,
      current: {
        id: 'ts1',
        attempt: 1,
        status: 'completed',
        contextMode: 'solution',
        summary: {
          feature: 'TDD-NOTES',
          version: 2,
          counts: { TestRequirement: 4, TestCase: 9, TestGap: 1 },
          files: ['cases.md'],
          gaps: [{ code: 'G1', question: 'Max size?', owner: 'BA', blocks: 2 }]
        }
      },
      history: []
    })
    expect(t.enabled).toBe(true)
    expect(t.current?.status).toBe('completed')
    expect(t.current?.summary.counts).toEqual({
      requirements: 4,
      cases: 9,
      gaps: 1
    })
    expect(t.current?.summary.gaps[0].question).toBe('Max size?')
  })

  it('never throws on garbage and keeps an unknown status as unknown', () => {
    expect(parseTestSpecs(null)).toEqual({
      requestId: '',
      enabled: false,
      current: null,
      history: []
    })
    expect(parseTestSpecs({ current: { status: 'weird' } }).current?.status).toBe('unknown')
  })

  it('reads a document page', () => {
    expect(parseTestSpecDocument({ text: 'TC-1', hasNext: true, next: 4 })).toMatchObject({
      text: 'TC-1',
      hasNext: true,
      next: 4
    })
  })
})
