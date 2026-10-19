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
