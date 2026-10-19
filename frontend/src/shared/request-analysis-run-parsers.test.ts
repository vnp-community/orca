import { describe, expect, it } from 'vitest'
import { latestAnalysisRun, parseAnalysisRun } from './request-analysis-run-parsers'

describe('parseAnalysisRun', () => {
  it('reads the service shape', () => {
    expect(
      parseAnalysisRun({
        id: 'r1',
        kind: 'solution',
        status: 'failed',
        errorCode: 'REQUEST_AI_NOT_CONFIGURED',
        errorMessage: 'log in',
        startedAt: 'a',
        finishedAt: 'b'
      })
    ).toEqual({
      id: 'r1',
      kind: 'solution',
      status: 'failed',
      errorCode: 'REQUEST_AI_NOT_CONFIGURED',
      errorMessage: 'log in',
      startedAt: 'a',
      finishedAt: 'b'
    })
  })

  it('never throws and maps unknown values', () => {
    expect(parseAnalysisRun(null)).toMatchObject({ id: '', kind: 'unknown', status: 'unknown' })
    expect(parseAnalysisRun({ id: 4, status: 'weird', kind: 'x', errorCode: '' })).toMatchObject({
      id: '',
      kind: 'unknown',
      status: 'unknown',
      errorCode: undefined
    })
  })
})

describe('latestAnalysisRun', () => {
  it('is null without runs and picks the newest start otherwise', () => {
    expect(latestAnalysisRun([])).toBeNull()
    const runs = [
      parseAnalysisRun({ id: 'old', status: 'failed', startedAt: '2026-10-10T01:00:00Z' }),
      parseAnalysisRun({ id: 'new', status: 'running', startedAt: '2026-10-10T03:00:00Z' }),
      parseAnalysisRun({ id: 'mid', status: 'succeeded', startedAt: '2026-10-10T02:00:00Z' })
    ]
    expect(latestAnalysisRun(runs)?.id).toBe('new')
  })
})
