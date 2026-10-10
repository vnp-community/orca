import { describe, expect, it } from 'vitest'
import { requestErrorMessage, requestErrorMessageForError } from './request-error-message'
import type { RequestRpcErrorKind } from '../../../../shared/request-errors'

const KINDS: RequestRpcErrorKind[] = [
  'forbidden',
  'not_found',
  'conflict',
  'invalid_state',
  'blocking_questions',
  'unconfirmed_assumptions',
  'validation',
  'unsupported',
  'rate_limited',
  'unavailable',
  'expired',
  'pending',
  'no_dev_server',
  'no_ai',
  'feature_off',
  'network',
  'unknown'
]

describe('requestErrorMessage', () => {
  it('has distinct copy for every kind except unknown fallbacks', () => {
    const unknown = requestErrorMessage('unknown')
    for (const kind of KINDS.filter((k) => k !== 'unknown')) {
      expect(requestErrorMessage(kind), kind).not.toBe(unknown)
    }
  })

  it('falls back to the unknown copy for a null or foreign kind', () => {
    expect(requestErrorMessage(null)).toBe(requestErrorMessage('unknown'))
    expect(requestErrorMessage('something_else')).toBe(requestErrorMessage('unknown'))
  })
})

describe('requestErrorMessageForError', () => {
  it('uses code-specific copy so a throttle does not read as a generic failure', () => {
    const msg = requestErrorMessageForError({
      kind: 'rate_limited',
      backendCode: 'REQUEST_RATE_LIMITED'
    })
    expect(msg).not.toBe(requestErrorMessage('rate_limited'))
    expect(msg).toBe('You are going too fast. Wait a moment and try again.')
  })

  it('tells quota exhaustion apart from a short throttle', () => {
    const quota = requestErrorMessageForError({
      kind: 'rate_limited',
      backendCode: 'REQUEST_AI_QUOTA_EXHAUSTED'
    })
    expect(quota).toContain('quota')
  })

  it('names a missing project instead of the generic not-found copy', () => {
    expect(
      requestErrorMessageForError({ kind: 'not_found', backendCode: 'REQUEST_PROJECT_NOT_FOUND' })
    ).toBe('This project no longer exists.')
  })

  it('falls back to the kind copy when the code has no specific text', () => {
    expect(
      requestErrorMessageForError({ kind: 'forbidden', backendCode: 'REQUEST_FORBIDDEN' })
    ).toBe(requestErrorMessage('forbidden'))
    expect(requestErrorMessageForError({ kind: 'conflict' })).toBe(requestErrorMessage('conflict'))
  })
})
