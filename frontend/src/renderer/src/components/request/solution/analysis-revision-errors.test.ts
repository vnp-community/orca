import { describe, expect, it } from 'vitest'
import type { RequestRpcError } from '../../../../../shared/request-errors'
import { analysisRevisionMessage, classifyAnalysisRevisionError } from './analysis-revision-errors'

const error = (over: Partial<RequestRpcError>): RequestRpcError => ({
  kind: 'unknown',
  code: 'runtime_error',
  message: 'x',
  ...over
})

describe('classifyAnalysisRevisionError', () => {
  it.each([
    [{ message: 'REQUEST_SOLUTION_FEEDBACK_TOO_LONG: too long', kind: 'validation' }, 'tooLong'],
    [
      {
        backendCode: 'REQUEST_SOLUTION_WRONG_STATE',
        code: 'REQUEST_SOLUTION_WRONG_STATE',
        kind: 'invalid_state'
      },
      'wrongState'
    ],
    [{ kind: 'conflict' }, 'wrongState'],
    [{ message: 'REQUEST_ANALYSIS_NO_CONNECTION: none', kind: 'no_dev_server' }, 'noConnection'],
    [{ message: 'REQUEST_SOLUTION_NO_CONNECTION: none', kind: 'no_dev_server' }, 'noConnection'],
    [{ kind: 'forbidden' }, 'forbidden'],
    [{ kind: 'rate_limited' }, 'rateLimited'],
    [{ kind: 'network' }, 'network'],
    [{ kind: 'unavailable' }, 'other'],
    [{ kind: 'unknown' }, 'other']
  ] as [Partial<RequestRpcError>, string][])('maps %j to %s', (over, kind) => {
    const result = classifyAnalysisRevisionError(error(over))
    expect(result.kind).toBe(kind)
    expect(result.message.trim()).not.toBe('')
  })

  it('names the limit in the too-long sentence', () => {
    expect(analysisRevisionMessage('tooLong')).toContain('2000')
  })

  it('has a fixed sentence for the kinds the panel handles itself and none for the generic ones', () => {
    for (const kind of ['empty', 'tooLong', 'forbidden', 'wrongState', 'noConnection'] as const) {
      expect(analysisRevisionMessage(kind)).toBeTruthy()
    }
    for (const kind of ['rateLimited', 'network', 'other'] as const) {
      expect(analysisRevisionMessage(kind)).toBeNull()
    }
  })

  it('does not echo the server message of a generic failure', () => {
    const result = classifyAnalysisRevisionError(
      error({ kind: 'unknown', message: 'secret feedback text' })
    )
    expect(result.message).not.toContain('secret')
  })
})
