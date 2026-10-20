/**
 * Tests for request-errors.ts
 */

import { describe, it, expect } from 'vitest'
import {
  splitErrorCode,
  classifyRequestRpcError,
  canonicalApprovalSolutionCode,
  parseBlockingQuestionIds,
  parseRetryAfterMs,
  parseUnconfirmedAssumptionIds,
  requestErrorKindForCode,
  type RequestRpcErrorKind
} from './request-errors'

describe('splitErrorCode', () => {
  it('extracts uppercase code prefix', () => {
    const { code, rest } = splitErrorCode('REQUEST_VERSION_CONFLICT: stale state')
    expect(code).toBe('REQUEST_VERSION_CONFLICT')
    expect(rest).toBe('stale state')
  })

  it('handles message without code prefix', () => {
    const { code, rest } = splitErrorCode('some error message')
    expect(code).toBe('')
    expect(rest).toBe('some error message')
  })

  it('handles multiline rest (s flag)', () => {
    const { code, rest } = splitErrorCode('REQUEST_REASON_REQUIRED: reason\nis required')
    expect(code).toBe('REQUEST_REASON_REQUIRED')
    expect(rest).toContain('reason')
  })
})

describe('classifyRequestRpcError', () => {
  // Helper to build a duck-typed RuntimeRpcCallError
  function rpcError(code: string, message = '') {
    return { code, message: message || `${code}: detail` }
  }

  it.each([
    [rpcError('any', 'REQUEST_FORBIDDEN: no'), 'forbidden'],
    [rpcError('any', 'APPROVAL_FORBIDDEN: no'), 'forbidden'],
    [rpcError('any', 'REQUEST_NOT_FOUND: missing'), 'not_found'],
    [rpcError('any', 'SOLUTION_NOT_FOUND: missing'), 'not_found'],
    [rpcError('any', 'REQUEST_VERSION_CONFLICT: x'), 'conflict'],
    [rpcError('any', 'APPROVAL_VERSION_CONFLICT: x'), 'conflict'],
    [rpcError('any', 'REQUEST_STATE_STALE: x'), 'conflict'],
    [rpcError('any', 'REQUEST_TRANSITION_NOT_ALLOWED: x'), 'invalid_state'],
    [rpcError('any', 'REQUEST_TYPE_CHANGE_NOT_ALLOWED: x'), 'invalid_state'],
    [rpcError('any', 'REQUEST_REASON_REQUIRED: x'), 'validation'],
    [rpcError('any', 'REQUEST_TYPE_REQUIRED: x'), 'validation'],
    [rpcError('any', 'APPROVAL_COMMENT_REQUIRED: x'), 'validation'],
    [rpcError('any', 'REQUEST_PENDING_LIMIT: x'), 'validation'],
    [rpcError('any', 'REQUEST_CLASSIFICATION_LIMIT: x'), 'rate_limited'],
    [rpcError('any', 'SERVICE_UNAVAILABLE: x'), 'unavailable'],
    [rpcError('any', 'GATEWAY_TIMEOUT: x'), 'unavailable']
  ])('maps error %o to kind %s', (error, expectedKind) => {
    const result = classifyRequestRpcError(error)
    expect(result.kind).toBe(expectedKind)
  })

  it('maps method_not_found code to unsupported', () => {
    const result = classifyRequestRpcError(rpcError('method_not_found', 'not supported'))
    expect(result.kind).toBe('unsupported')
  })

  it('maps CAPABILITY_UNSUPPORTED message to unsupported', () => {
    const result = classifyRequestRpcError(rpcError('any', 'CAPABILITY_UNSUPPORTED: x'))
    expect(result.kind).toBe('unsupported')
  })

  it('maps network Error to network kind', () => {
    const result = classifyRequestRpcError(new Error('network timeout'))
    expect(result.kind).toBe('network')
  })

  it('maps generic Error to unknown kind', () => {
    const result = classifyRequestRpcError(new Error('some random error'))
    expect(result.kind).toBe('unknown')
  })

  it('maps unknown primitive to unknown kind', () => {
    const result = classifyRequestRpcError(42)
    expect(result.kind).toBe('unknown')
  })

  it('maps null to unknown kind', () => {
    const result = classifyRequestRpcError(null)
    expect(result.kind).toBe('unknown')
  })

  it('never throws', () => {
    const weirdValues = [undefined, null, '', 0, false, [], {}]
    for (const v of weirdValues) {
      expect(() => classifyRequestRpcError(v)).not.toThrow()
    }
  })
})

describe('code table (SOL-FE-E2E-003, CONTRACT section 5)', () => {
  // [code, kind]: every code the Request UI can receive and must not show as `unknown`.
  const TABLE: [string, RequestRpcErrorKind][] = [
    ['REQUEST_RATE_LIMITED', 'rate_limited'],
    ['REQUEST_FLOW_DISABLED', 'feature_off'],
    ['REQUEST_PROJECT_NOT_FOUND', 'not_found'],
    ['REQUEST_AI_NOT_CONFIGURED', 'no_ai'],
    ['REQUEST_AI_CREDENTIAL_UNAVAILABLE', 'no_ai'],
    ['REQUEST_AI_SESSION_EXPIRED', 'no_ai'],
    ['REQUEST_AI_NO_PROVIDER', 'no_ai'],
    ['REQUEST_AI_QUOTA_EXHAUSTED', 'rate_limited'],
    ['REQUEST_AI_AGENT_UNAVAILABLE', 'no_dev_server'],
    ['REQUEST_AI_BUSY', 'unavailable'],
    ['REQUEST_AI_COMPLETE_TIMEOUT', 'unavailable'],
    ['REQUEST_SOLUTION_NO_CONNECTION', 'no_dev_server'],
    ['REQUEST_ANALYSIS_NO_CONNECTION', 'no_dev_server'],
    ['REQUEST_ANALYSIS_BUSY', 'unavailable'],
    ['PROJECT_DEV_SERVER_NOT_FOUND', 'no_dev_server'],
    ['PROJECT_DEV_SERVER_UNREACHABLE', 'no_dev_server'],
    ['PROJECT_DEV_SERVER_CONNECTION_FAILED', 'no_dev_server'],
    ['PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED', 'no_dev_server'],
    ['PROJECT_DEV_SERVER_LOOKUP_FAILED', 'unavailable'],
    ['REQUEST_UNAVAILABLE', 'unavailable'],
    ['TASK_NOT_FOUND', 'not_found'],
    ['TASK_FORBIDDEN', 'forbidden'],
    ['REQUEST_NOT_IMPLEMENTED', 'unsupported'],
    ['INVALID_ARGUMENT', 'validation'],
    // The gateway sends the prefixed spelling for approvals and solutions.
    ['REQUEST_APPROVAL_NOT_FOUND', 'not_found'],
    ['REQUEST_SOLUTION_NOT_FOUND', 'not_found'],
    ['REQUEST_APPROVAL_VERSION_CONFLICT', 'conflict'],
    ['REQUEST_SOLUTION_VERSION_CONFLICT', 'conflict'],
    ['REQUEST_APPROVAL_DIGEST_MISMATCH', 'conflict'],
    ['REQUEST_APPROVAL_STAGE_MISMATCH', 'conflict'],
    ['REQUEST_APPROVAL_ALREADY_DECIDED', 'invalid_state'],
    ['REQUEST_APPROVAL_EXPIRED', 'expired'],
    ['REQUEST_APPROVAL_FORBIDDEN', 'forbidden'],
    ['REQUEST_APPROVAL_NOT_APPROVER', 'forbidden'],
    ['REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN', 'forbidden'],
    ['REQUEST_APPROVAL_AGENT_FORBIDDEN', 'forbidden'],
    ['REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER', 'invalid_state'],
    ['REQUEST_APPROVAL_COMMENT_REQUIRED', 'validation'],
    ['REQUEST_SOLUTION_WRONG_STATE', 'invalid_state'],
    ['REQUEST_PLAN_SOLUTION_NOT_APPROVED', 'invalid_state'],
    ['REQUEST_PLAN_AI_UNAVAILABLE', 'unavailable'],
    ['REQUEST_PLAN_REVISION_IN_PROGRESS', 'conflict'],
    ['REQUEST_PLAN_REVISION_NOT_APPLICABLE', 'invalid_state'],
    ['REQUEST_PLAN_REVISION_NO_CHANGE', 'invalid_state'],
    ['REQUEST_PLAN_REVISION_TEXT_PRUNED', 'invalid_state'],
    ['REQUEST_PLAN_REVISION_NOT_FOUND', 'not_found'],
    ['REQUEST_PLAN_BUNDLE_NOT_FOUND', 'not_found'],
    ['REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND', 'not_found'],
    ['REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE', 'unavailable'],
    // Existing short spellings stay mapped.
    ['APPROVAL_NOT_FOUND', 'not_found'],
    ['APPROVAL_VERSION_CONFLICT', 'conflict'],
    ['APPROVAL_EXPIRED', 'expired']
  ]

  it.each(TABLE)('%s is kind %s, never unknown', (code, kind) => {
    expect(requestErrorKindForCode(code)).toBe(kind)
    const err = classifyRequestRpcError({ code: 'runtime_error', message: `${code}: detail` })
    expect(err.kind).toBe(kind)
    expect(err.backendCode).toBe(code)
  })

  it('still reports an unmapped code as unknown but keeps the code', () => {
    const err = classifyRequestRpcError({
      code: 'runtime_error',
      message: 'REQUEST_SOMETHING_NEW: x'
    })
    expect(err.kind).toBe('unknown')
    expect(err.backendCode).toBe('REQUEST_SOMETHING_NEW')
  })

  it('has no backendCode for an uncoded message', () => {
    const err = classifyRequestRpcError({ code: 'runtime_error', message: 'plain text' })
    expect(err.backendCode).toBeUndefined()
  })
})

describe('retryAfterMs', () => {
  it('reads the gateway trailer', () => {
    expect(parseRetryAfterMs('REQUEST_RATE_LIMITED: too many requests {"retryAfterMs":1500}')).toBe(
      1500
    )
    const err = classifyRequestRpcError({
      code: 'runtime_error',
      message: 'REQUEST_RATE_LIMITED: too many requests {"retryAfterMs":250}'
    })
    expect(err).toMatchObject({
      kind: 'rate_limited',
      backendCode: 'REQUEST_RATE_LIMITED',
      retryAfterMs: 250
    })
  })

  it('is absent without a trailer, with a non-numeric value, or when the JSON is not at the end', () => {
    expect(parseRetryAfterMs('REQUEST_RATE_LIMITED: too many requests')).toBeUndefined()
    expect(parseRetryAfterMs('x {"retryAfterMs":"soon"}')).toBeUndefined()
    expect(parseRetryAfterMs('x {"retryAfterMs":9} and more text')).toBeUndefined()
  })

  it('accepts the codeintel shape with extra fields', () => {
    expect(parseRetryAfterMs('TIMEOUT {"retryAfterMs":3000,"inProgress":true}')).toBe(3000)
  })
})

describe('canonicalApprovalSolutionCode', () => {
  it('strips REQUEST_ only from APPROVAL_ and SOLUTION_ codes', () => {
    expect(canonicalApprovalSolutionCode('REQUEST_APPROVAL_EXPIRED')).toBe('APPROVAL_EXPIRED')
    expect(canonicalApprovalSolutionCode('request_solution_version_conflict')).toBe(
      'SOLUTION_VERSION_CONFLICT'
    )
    expect(canonicalApprovalSolutionCode('APPROVAL_EXPIRED')).toBe('APPROVAL_EXPIRED')
    expect(canonicalApprovalSolutionCode('REQUEST_RATE_LIMITED')).toBe('REQUEST_RATE_LIMITED')
  })
})

describe('blocking open questions', () => {
  it('maps REQUEST_SOLUTION_BLOCKING_QUESTIONS to its own kind, not invalid_state', () => {
    expect(requestErrorKindForCode('REQUEST_SOLUTION_BLOCKING_QUESTIONS')).toBe(
      'blocking_questions'
    )
    const err = classifyRequestRpcError({
      code: 'runtime_error',
      message: 'REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1'
    })
    expect(err.kind).toBe('blocking_questions')
    expect(err.backendCode).toBe('REQUEST_SOLUTION_BLOCKING_QUESTIONS')
  })

  it('reads the question ids from the message', () => {
    expect(
      parseBlockingQuestionIds(
        'REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1'
      )
    ).toEqual(['Q-1'])
    expect(
      parseBlockingQuestionIds(
        'REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1, Q-2 ,Q-10'
      )
    ).toEqual(['Q-1', 'Q-2', 'Q-10'])
  })

  it('returns nothing for a message without a readable list', () => {
    expect(parseBlockingQuestionIds('REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocked')).toEqual([])
    expect(parseBlockingQuestionIds('')).toEqual([])
    expect(parseBlockingQuestionIds('x: unanswered: <script>alert(1)</script>, Q-3')).toEqual([
      'Q-3'
    ])
  })
})

describe('unconfirmed plan assumptions', () => {
  it('maps REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS to its own kind', () => {
    expect(requestErrorKindForCode('REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS')).toBe(
      'unconfirmed_assumptions'
    )
    const message =
      'REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS: assumptions needing confirmation are unanswered: A-1, A-2'
    expect(classifyRequestRpcError({ code: 'runtime_error', message }).kind).toBe(
      'unconfirmed_assumptions'
    )
    expect(parseUnconfirmedAssumptionIds(message)).toEqual(['A-1', 'A-2'])
    expect(parseUnconfirmedAssumptionIds('REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS: x')).toEqual([])
  })
})
