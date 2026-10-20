import { afterEach, describe, expect, it, vi } from 'vitest'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import {
  MAX_COOLDOWN_MS,
  autoRetryDelayMs,
  cooldownMsFor,
  cooldownRemainingMs,
  isRetrySafe,
  rateLimitClassOf,
  resetRequestCooldowns,
  startRequestCooldown,
  subscribeRequestCooldown
} from './request-rate-limit'

afterEach(resetRequestCooldowns)

describe('classes and retry safety', () => {
  it('puts AI starts in ai, state changes in write, everything else in read', () => {
    expect(rateLimitClassOf('solution.generate')).toBe('ai')
    expect(rateLimitClassOf('request.generatePlan')).toBe('ai')
    expect(rateLimitClassOf('request.retryPlan')).toBe('ai')
    expect(rateLimitClassOf('request.classify')).toBe('ai')
    expect(rateLimitClassOf('request.cancel')).toBe('write')
    expect(rateLimitClassOf('request.setBaseRef')).toBe('write')
    expect(rateLimitClassOf('approval.approve')).toBe('write')
    expect(rateLimitClassOf('request.list')).toBe('read')
    expect(rateLimitClassOf('clarification.list')).toBe('read')
  })

  it('classifies every declared method (no method falls through unnoticed)', () => {
    for (const m of Object.values(REQUEST_RPC_METHODS)) {
      expect(['read', 'write', 'ai']).toContain(rateLimitClassOf(m))
    }
  })

  it('retries reads and version-guarded writes, never the other writes', () => {
    expect(isRetrySafe('request.list')).toBe(true)
    expect(isRetrySafe('clarification.answer')).toBe(true)
    expect(isRetrySafe('clarification.request')).toBe(true)
    expect(rateLimitClassOf('clarification.request')).toBe('write')
    expect(isRetrySafe('approval.approve')).toBe(true)
    expect(isRetrySafe('solution.generate')).toBe(true)
    expect(isRetrySafe('request.setBaseRef')).toBe(true)
    expect(isRetrySafe('request.create')).toBe(false)
    expect(isRetrySafe('request.cancel')).toBe(false)
    expect(isRetrySafe('backlog.execute')).toBe(false)
    expect(isRetrySafe('request.generatePlan')).toBe(false)
  })
})

describe('autoRetryDelayMs', () => {
  const throttle = { backendCode: 'REQUEST_RATE_LIMITED' }
  it('uses retryAfterMs, defaulting to one second', () => {
    expect(autoRetryDelayMs('request.list', { ...throttle, retryAfterMs: 300 })).toBe(300)
    expect(autoRetryDelayMs('request.list', throttle)).toBe(1000)
  })
  it('is null for other codes, unsafe writes and waits over the cap', () => {
    expect(
      autoRetryDelayMs('request.list', { backendCode: 'REQUEST_AI_QUOTA_EXHAUSTED' })
    ).toBeNull()
    expect(autoRetryDelayMs('request.list', {})).toBeNull()
    expect(autoRetryDelayMs('request.create', throttle)).toBeNull()
    expect(autoRetryDelayMs('request.list', { ...throttle, retryAfterMs: 5001 })).toBeNull()
  })
  it('lets the caller override the default', () => {
    expect(autoRetryDelayMs('request.create', throttle, true)).toBe(1000)
    expect(autoRetryDelayMs('request.list', throttle, false)).toBeNull()
  })
})

describe('cooldown store', () => {
  it('counts down per class and notifies subscribers', () => {
    const seen = vi.fn()
    const off = subscribeRequestCooldown(seen)
    startRequestCooldown('write', 1500, 1_000)
    expect(seen).toHaveBeenCalledTimes(1)
    expect(cooldownRemainingMs('write', 1_000)).toBe(1500)
    expect(cooldownRemainingMs('write', 2_000)).toBe(500)
    expect(cooldownRemainingMs('write', 2_600)).toBe(0)
    expect(cooldownRemainingMs('read', 1_000)).toBe(0)
    off()
    startRequestCooldown('ai', 100)
    expect(seen).toHaveBeenCalledTimes(1)
  })

  it('never shortens a longer wait and caps absurd values', () => {
    startRequestCooldown('read', 5_000, 0)
    startRequestCooldown('read', 100, 0)
    expect(cooldownRemainingMs('read', 0)).toBe(5_000)
    startRequestCooldown('ai', 10 * 60_000, 0)
    expect(cooldownRemainingMs('ai', 0)).toBe(MAX_COOLDOWN_MS)
    expect(cooldownMsFor({ retryAfterMs: 10 * 60_000 })).toBe(MAX_COOLDOWN_MS)
    expect(cooldownMsFor({})).toBe(1000)
  })
})
