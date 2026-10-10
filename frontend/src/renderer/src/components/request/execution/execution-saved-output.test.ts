import { describe, expect, it } from 'vitest'
import { hasSavedExecutionOutput } from './execution-saved-output'
import type { ExecutionResult } from '../../../../../shared/request-artifact-types'

const result = (over: Partial<ExecutionResult> = {}): ExecutionResult => ({
  taskId: 't1',
  attempt: 1,
  parseStatus: 'missing',
  filesChanged: [],
  checksRun: [],
  source: 'last_execution_output',
  stdoutTail: 'OK',
  ...over
})

describe('hasSavedExecutionOutput', () => {
  it('is true for a ready result with saved output', () => {
    expect(hasSavedExecutionOutput('ready', result())).toBe(true)
  })

  it('is true for a structured record even without output text', () => {
    expect(
      hasSavedExecutionOutput(
        'ready',
        result({ source: 'execution_record', stdoutTail: undefined })
      )
    ).toBe(true)
  })

  it('is false when the saved output is empty or whitespace', () => {
    expect(hasSavedExecutionOutput('ready', result({ stdoutTail: '  \n' }))).toBe(false)
    expect(hasSavedExecutionOutput('ready', result({ stdoutTail: undefined }))).toBe(false)
  })

  it.each(['idle', 'loading', 'legacy', 'unsupported', 'error'] as const)(
    'is false while the status is %s',
    (status) => {
      expect(hasSavedExecutionOutput(status, result())).toBe(false)
    }
  )

  it('is false without a result', () => {
    expect(hasSavedExecutionOutput('ready', null)).toBe(false)
  })
})
