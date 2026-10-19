import { describe, expect, it } from 'vitest'
import { classifyRequestRpcError } from '../../../../shared/request-errors'
import {
  baseRefLockReasonOf,
  baseRefLockReasonText,
  baseRefSuggestionsOf,
  classifyBaseRefWriteError
} from './base-ref-write-error'

const failure = (message: string, code = 'runtime_error') =>
  classifyBaseRefWriteError(classifyRequestRpcError({ code, message }))

describe('trailer parsing', () => {
  it('reads suggestions from the json trailer and bounds them to five strings', () => {
    const message =
      'REQUEST_BASE_REF_NOT_FOUND: ref not found {"suggestions":["main","dev",3,"","a","b","c","d"]}'
    expect(baseRefSuggestionsOf(message)).toEqual(['main', 'dev', 'a', 'b', 'c'])
  })

  it('returns nothing for a missing, malformed or non-object trailer', () => {
    expect(baseRefSuggestionsOf('REQUEST_BASE_REF_NOT_FOUND: ref not found')).toEqual([])
    expect(baseRefSuggestionsOf('REQUEST_BASE_REF_NOT_FOUND: x {"suggestions":[')).toEqual([])
    expect(baseRefSuggestionsOf('REQUEST_BASE_REF_NOT_FOUND: x ["main"]')).toEqual([])
    expect(baseRefSuggestionsOf('REQUEST_BASE_REF_NOT_FOUND: x {"suggestions":"main"}')).toEqual([])
  })

  it('reads the lock reason and treats an unknown one as a lock of unknown cause', () => {
    expect(baseRefLockReasonOf('REQUEST_BASE_REF_LOCKED: locked {"reason":"task"}')).toBe('task')
    expect(baseRefLockReasonOf('REQUEST_BASE_REF_LOCKED: locked {"reason":"quantum"}')).toBe(
      'unknown'
    )
    expect(baseRefLockReasonOf('REQUEST_BASE_REF_LOCKED: locked')).toBeUndefined()
  })
})

describe('lock reason text', () => {
  it('says something different for each reason and has a generic fallback', () => {
    const texts = (
      ['proposal', 'task', 'execution', 'terminal', 'unknown', undefined] as const
    ).map(baseRefLockReasonText)
    expect(new Set(texts).size).toBe(5)
    expect(baseRefLockReasonText('')).toBe(baseRefLockReasonText(undefined))
    expect(baseRefLockReasonText('proposal')).toMatch(/analysis/)
  })
})

describe('classifyBaseRefWriteError', () => {
  it('maps REQUEST_BASE_REF_INVALID to an invalid failure without echoing the backend text', () => {
    const f = failure('REQUEST_BASE_REF_INVALID: contains ..')
    expect(f.kind).toBe('invalid')
    expect(f.message).not.toContain('..')
  })

  it('maps NOT_FOUND with its suggestions', () => {
    const f = failure('REQUEST_BASE_REF_NOT_FOUND: ref not found {"suggestions":["main"]}')
    expect(f).toMatchObject({ kind: 'notFound', suggestions: ['main'] })
  })

  it('maps LOCKED with the reason from the trailer', () => {
    const f = failure('REQUEST_BASE_REF_LOCKED: locked {"reason":"execution"}')
    expect(f).toMatchObject({ kind: 'locked', reason: 'execution' })
    expect(f.message).toBe(baseRefLockReasonText('execution'))
  })

  it('maps a version conflict, a permission error and the agent refusal', () => {
    expect(failure('REQUEST_VERSION_CONFLICT: stale').kind).toBe('conflict')
    expect(failure('REQUEST_FORBIDDEN: no').kind).toBe('forbidden')
    expect(failure('REQUEST_ACTOR_NOT_ALLOWED: agent').kind).toBe('forbidden')
  })

  it('explains a deployment without a ref checker and falls back to the kind copy otherwise', () => {
    const missing = failure('REQUEST_BASE_REF_CHECK_NOT_CONFIGURED: no checker')
    expect(missing.kind).toBe('other')
    expect(missing.message).toMatch(/cannot check branches/)
    expect(failure('SERVICE_UNAVAILABLE: down')).toMatchObject({
      kind: 'other',
      message: 'The request service is unavailable. Try again shortly.'
    })
  })
})
