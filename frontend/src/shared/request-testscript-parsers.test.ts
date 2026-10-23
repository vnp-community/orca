import { describe, expect, it } from 'vitest'
import {
  parseTestScriptManifest,
  parseTestScriptReport,
  parseTestScripts,
  parseTestScriptSummary
} from './request-testscript-parsers'

describe('request-testscript-parsers', () => {
  it('reads the lane with the job summary the agent reported', () => {
    const got = parseTestScripts({
      requestId: 'r1',
      enabled: true,
      current: {
        id: 'a',
        attempt: 1,
        status: 'needs_review',
        feature: 'TDD-NOTES',
        specVersion: 2,
        branch: 'tsa/REQ-1-TDD-NOTES-v2',
        commitSha: 'c1',
        summary: {
          counts: { files: 2, generated: 3, 'needs-review': 1, failed: 0, skipped: 1 },
          files: [
            { path: 't.py', status: 'needs-review', attempts: 4, cases: ['TC-1'], errors: 2 }
          ],
          report: 'tests/tsa/x/tsa-report.md',
          cost_usd: 0.04
        }
      },
      history: [{ id: 'b', status: 'weird' }]
    })
    expect(got.enabled).toBe(true)
    expect(got.current?.status).toBe('needs_review')
    expect(got.current?.specVersion).toBe(2)
    expect(got.current?.summary.counts).toEqual({
      files: 2,
      generated: 3,
      needsReview: 1,
      failed: 0,
      skipped: 1
    })
    expect(got.current?.summary.files[0].cases).toEqual(['TC-1'])
    expect(got.current?.summary.costUsd).toBe(0.04)
    expect(got.history[0].status).toBe('unknown')
  })

  it('never throws on junk', () => {
    expect(parseTestScripts(null)).toEqual({
      requestId: '',
      enabled: false,
      current: null,
      history: []
    })
    expect(parseTestScriptSummary('x').counts.generated).toBe(0)
    expect(parseTestScriptReport({ text: '# TSA', hasNext: true, next: 5 })).toEqual({
      path: '',
      length: 0,
      text: '# TSA',
      hasNext: true,
      next: 5
    })
  })
})

describe('parseTestScriptManifest', () => {
  it('reads the rows of assertion-manifest.json', () => {
    const m = parseTestScriptManifest(
      JSON.stringify({
        schema: 'tsa-manifest/v1',
        feature: 'PAY',
        specVersion: 1,
        branch: 'tsa/PAY-v1',
        assertions: [
          {
            exp: 'TC-1.E1',
            case: 'TC-1',
            kind: 'errorCode',
            op: '==',
            value: 'FB-01',
            status: 'ok',
            file: 'tests/t.py',
            line: 12,
            function: 'test_x',
            assertion: "x == 'FB-01'"
          },
          { exp: 'TC-1.E2', case: 'TC-1', op: '<=', value: 2, status: 'missing' }
        ]
      })
    )
    expect(m.feature).toBe('PAY')
    expect(m.rows[0]).toMatchObject({ exp: 'TC-1.E1', line: 12, fn: 'test_x', file: 'tests/t.py' })
    expect(m.rows[1]).toMatchObject({
      status: 'missing',
      value: '2',
      file: undefined,
      line: undefined
    })
  })

  it('gives an empty manifest for text that is not JSON', () => {
    expect(parseTestScriptManifest('not json').rows).toEqual([])
  })
})
