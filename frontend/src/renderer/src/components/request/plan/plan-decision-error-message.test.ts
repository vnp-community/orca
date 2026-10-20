import { describe, expect, it } from 'vitest'
import { planDecisionErrorMessage } from './plan-decision-error-message'
import type { PlanDecisionFailure } from '../../../hooks/usePlanDecision'

describe('planDecisionErrorMessage', () => {
  it('explains that approving a plan needs permission to run its execution', () => {
    const failure = {
      scope: 'ap1',
      errorClass: 'forbidden',
      error: {
        kind: 'forbidden',
        code: 'REQUEST_EXECUTION_ACTOR_NOT_ALLOWED',
        message: 'REQUEST_EXECUTION_ACTOR_NOT_ALLOWED: approver cannot run it'
      }
    } as unknown as PlanDecisionFailure
    expect(planDecisionErrorMessage(failure)).toContain('allowed to run their execution')
  })
})

function failureWith(code: string, kind = 'unknown'): PlanDecisionFailure {
  return {
    scope: 'ap1',
    errorClass: 'other',
    error: { kind, code: 'runtime_error', message: `${code}: refused`, backendCode: code }
  } as unknown as PlanDecisionFailure
}

describe('planDecisionErrorMessage run-all codes', () => {
  it('tells the user a pinned request must be approved normally', () => {
    expect(planDecisionErrorMessage(failureWith('REQUEST_EXECUTION_MODE_ALREADY_PINNED'))).toBe(
      'This request already runs step by step because it was started that way before. Approve it normally.'
    )
  })

  it.each(['REQUEST_EXECUTION_MODE_NOT_APPLICABLE', 'REQUEST_EXECUTION_MODE_INVALID'])(
    'gives one generic message for %s',
    (code) => {
      expect(planDecisionErrorMessage(failureWith(code))).toBe(
        "Couldn't start the run this way. Approve it normally."
      )
    }
  )
})
