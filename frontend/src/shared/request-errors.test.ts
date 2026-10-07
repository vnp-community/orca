/**
 * Tests for request-errors.ts
 */

import { describe, it, expect } from 'vitest'
import { splitErrorCode, classifyRequestRpcError } from './request-errors'

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
