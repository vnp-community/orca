import { describe, expect, it } from 'vitest'
import {
  APPROVAL_RUN_ALL_FEATURE,
  isRunAllAlreadyPinned,
  isRunAllProgrammingError,
  parseApproveResult,
  parseEngineSettings,
  RUN_ALL_EXECUTION_MODE
} from './request-run-all'

describe('run-all constants', () => {
  it('uses the names the gateway contract fixes', () => {
    expect(APPROVAL_RUN_ALL_FEATURE).toBe('approval.runAll')
    expect(RUN_ALL_EXECUTION_MODE).toBe('continuous')
  })
})

describe('parseApproveResult', () => {
  it('reads the status and the pinned mode', () => {
    expect(parseApproveResult({ requestStatus: 'executing', executionMode: 'continuous' })).toEqual(
      {
        requestStatus: 'executing',
        executionMode: 'continuous'
      }
    )
    expect(parseApproveResult({ executionMode: 'step_by_step' })).toEqual({
      executionMode: 'step_by_step'
    })
  })

  it.each([undefined, null, 'continuous', 7, [], {}])('never throws on %j', (raw) => {
    expect(parseApproveResult(raw)).toEqual({})
  })

  it('drops wrong types and unknown modes', () => {
    expect(parseApproveResult({ requestStatus: 3, executionMode: 'bogus' })).toEqual({})
    expect(parseApproveResult({ executionMode: '' })).toEqual({})
    expect(parseApproveResult({ executionMode: 1 })).toEqual({})
  })
})

describe('run-all error codes', () => {
  it('finds ALREADY_PINNED in the message, the backendCode or the transport code', () => {
    const message = 'REQUEST_EXECUTION_MODE_ALREADY_PINNED: pinned'
    expect(isRunAllAlreadyPinned({ code: 'runtime_error', message })).toBe(true)
    expect(
      isRunAllAlreadyPinned({
        code: 'runtime_error',
        message: 'x',
        backendCode: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED'
      })
    ).toBe(true)
    expect(
      isRunAllAlreadyPinned({ code: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED', message: '' })
    ).toBe(true)
    expect(isRunAllAlreadyPinned({ code: 'runtime_error', message: 'other' })).toBe(false)
    expect(isRunAllAlreadyPinned(null)).toBe(false)
  })

  it('flags the two programming errors only', () => {
    for (const code of [
      'REQUEST_EXECUTION_MODE_NOT_APPLICABLE',
      'REQUEST_EXECUTION_MODE_INVALID'
    ]) {
      expect(isRunAllProgrammingError({ code: 'runtime_error', message: `${code}: no` })).toBe(true)
    }
    expect(
      isRunAllProgrammingError({
        code: 'runtime_error',
        message: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED: x'
      })
    ).toBe(false)
  })
})

describe('parseEngineSettings', () => {
  const full = {
    projectId: 'p1',
    engine: 'openspec',
    openspecMinVersion: '1.2.3',
    solutionAgent: true,
    executionMode: 'continuous',
    version: 4
  }

  it('reads a full answer verbatim', () => {
    expect(parseEngineSettings({ ...full, preflight: { ran: true } })).toEqual(full)
  })

  it('treats omitted zero values as their defaults', () => {
    expect(parseEngineSettings({ projectId: 'p1', engine: 'native' })).toEqual({
      projectId: 'p1',
      engine: 'native',
      openspecMinVersion: '',
      solutionAgent: false,
      executionMode: 'step_by_step',
      version: 0
    })
    expect(parseEngineSettings({ ...full, executionMode: '' })?.executionMode).toBe('step_by_step')
  })

  it.each([
    undefined,
    null,
    'x',
    {},
    { ...full, projectId: '' },
    { ...full, engine: 1 },
    { ...full, engine: undefined },
    { ...full, openspecMinVersion: 3 },
    { ...full, solutionAgent: 'yes' },
    { ...full, version: '4' },
    { ...full, version: Number.NaN },
    { ...full, executionMode: 'bogus' }
  ])('returns null for %j', (raw) => {
    expect(parseEngineSettings(raw)).toBeNull()
  })
})
