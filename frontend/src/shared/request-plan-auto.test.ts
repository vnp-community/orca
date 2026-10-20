import { describe, expect, it } from 'vitest'
import { REQUEST_RPC_METHODS } from './request-rpc-methods'
import { parsePlanRunView, REQUEST_PLAN_AUTO_FEATURE } from './request-plan-auto'

const EMPTY_RUN = {
  runId: '',
  status: '',
  errorCode: '',
  errorMessage: '',
  engine: '',
  autoPlan: null
}

describe('parsePlanRunView', () => {
  it('reads a full answer with the autoPlan block', () => {
    expect(
      parsePlanRunView({
        runId: 'run1',
        status: 'failed',
        errorCode: 'REQUEST_PLAN_AI_INVALID_JSON',
        errorMessage: 'bad json',
        engine: 'native',
        proposal: null,
        autoPlan: {
          enabled: true,
          state: 'pending',
          origin: 'event',
          attempt: 2,
          maxAttempts: 2,
          reasonCode: 'REQUEST_ANALYSIS_BUSY',
          nextAttemptAt: '2026-10-20T10:00:00Z'
        }
      })
    ).toEqual({
      runId: 'run1',
      status: 'failed',
      errorCode: 'REQUEST_PLAN_AI_INVALID_JSON',
      errorMessage: 'bad json',
      engine: 'native',
      autoPlan: {
        enabled: true,
        state: 'pending',
        origin: 'event',
        attempt: 2,
        maxAttempts: 2,
        reasonCode: 'REQUEST_ANALYSIS_BUSY',
        nextAttemptAt: '2026-10-20T10:00:00Z'
      }
    })
  })

  it('gives autoPlan null for a gateway that predates CR-REQ-044', () => {
    expect(parsePlanRunView({ runId: 'r', status: 'running' })).toEqual({
      ...EMPTY_RUN,
      runId: 'r',
      status: 'running'
    })
  })

  it('keeps enabled:false apart from a missing block', () => {
    const view = parsePlanRunView({ autoPlan: { enabled: false } })
    expect(view.autoPlan).toEqual({
      enabled: false,
      state: '',
      origin: '',
      attempt: 0,
      maxAttempts: 0,
      reasonCode: ''
    })
    expect(parsePlanRunView({ autoPlan: {} }).autoPlan).not.toBeNull()
  })

  it('reads an autoPlan whose proto3 zero values were omitted', () => {
    const view = parsePlanRunView({ autoPlan: { enabled: true, state: 'running', attempt: 1 } })
    expect(view.autoPlan).toMatchObject({
      enabled: true,
      state: 'running',
      attempt: 1,
      maxAttempts: 0,
      reasonCode: ''
    })
    expect(view.autoPlan).not.toHaveProperty('nextAttemptAt')
  })

  it('turns unknown enums into empty and bad numbers into 0', () => {
    const view = parsePlanRunView({
      status: 'exploded',
      autoPlan: {
        enabled: 'yes',
        state: 'sleeping',
        origin: 'cron',
        attempt: -3,
        maxAttempts: '2',
        reasonCode: 7,
        nextAttemptAt: ''
      }
    })
    expect(view.status).toBe('')
    expect(view.autoPlan).toEqual({
      enabled: false,
      state: '',
      origin: '',
      attempt: 0,
      maxAttempts: 0,
      reasonCode: ''
    })
    expect(
      parsePlanRunView({ autoPlan: { attempt: Infinity, maxAttempts: Number.NaN } }).autoPlan
    ).toMatchObject({
      attempt: 0,
      maxAttempts: 0
    })
  })

  it('accepts the revision origin of a change request job (CR-REQ-045)', () => {
    expect(
      parsePlanRunView({ autoPlan: { enabled: true, state: 'running', origin: 'revision' } })
        .autoPlan
    ).toMatchObject({ origin: 'revision', state: 'running' })
  })

  it.each([null, undefined, 'text', 42, [], [{ runId: 'x' }], true])(
    'never throws and answers empty for %j',
    (raw) => {
      expect(parsePlanRunView(raw)).toEqual(EMPTY_RUN)
    }
  )

  it.each([null, 'x', 3, [], undefined])('reads autoPlan %j as missing', (autoPlan) => {
    expect(parsePlanRunView({ runId: 'r', autoPlan }).autoPlan).toBeNull()
  })

  it('ignores wrongly typed scalar fields', () => {
    expect(parsePlanRunView({ runId: 5, errorCode: {}, engine: [] })).toEqual(EMPTY_RUN)
  })
})

describe('plan auto wire names', () => {
  it('uses the names the gateway registers', () => {
    expect(REQUEST_PLAN_AUTO_FEATURE).toBe('request.planAuto')
    expect(REQUEST_RPC_METHODS.RETRY_PLAN).toBe('request.retryPlan')
  })
})
