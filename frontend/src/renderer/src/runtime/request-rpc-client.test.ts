// Tests for request-rpc-client.ts (CR-REQ-018-02)
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRuntimeRpc = vi.fn()
const subscribeRuntimeStreamChannel = vi.fn()
let activeTarget: { kind: 'local' } | { kind: 'environment'; environmentId: string } = {
  kind: 'environment',
  environmentId: 'env-1'
}

vi.mock('./runtime-rpc-client', () => ({
  callRuntimeRpc: (...args: unknown[]) => callRuntimeRpc(...args),
  subscribeRuntimeStreamChannel: (...args: unknown[]) => subscribeRuntimeStreamChannel(...args),
  getActiveRuntimeTarget: () => activeTarget
}))
vi.mock('../store', () => ({
  useAppStore: { getState: () => ({ settings: {} }) }
}))

import { callRequestRpc, callRequestRpcOrThrow, subscribeRequestEvents } from './request-rpc-client'
import { cooldownRemainingMs, resetRequestCooldowns } from './request-rate-limit'

function rpcError(code: string, message: string): Error {
  return Object.assign(new Error(message), { code })
}

beforeEach(() => {
  resetRequestCooldowns()
  callRuntimeRpc.mockReset()
  subscribeRuntimeStreamChannel.mockReset()
  activeTarget = { kind: 'environment', environmentId: 'env-1' }
})

describe('callRequestRpc', () => {
  it('returns ok:true with the value on success', async () => {
    callRuntimeRpc.mockResolvedValue({ requests: [] })
    const result = await callRequestRpc('request.list', { pageSize: 1 })
    expect(result).toEqual({ ok: true, value: { requests: [] } })
    expect(callRuntimeRpc).toHaveBeenCalledWith(activeTarget, 'request.list', { pageSize: 1 })
  })

  it('classifies a version conflict code as conflict', async () => {
    callRuntimeRpc.mockRejectedValue(rpcError('runtime_error', 'REQUEST_VERSION_CONFLICT: x'))
    const result = await callRequestRpc('request.cancel', { id: 'r1' })
    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.error.kind).toBe('conflict')
    }
  })

  it('classifies method_not_found as unsupported', async () => {
    callRuntimeRpc.mockRejectedValue(rpcError('method_not_found', 'no such method'))
    const result = await callRequestRpc('request.list')
    if (result.ok) {
      throw new Error('expected error')
    }
    expect(result.error.kind).toBe('unsupported')
  })

  it('never throws on a network failure', async () => {
    callRuntimeRpc.mockRejectedValue(new TypeError('Failed to fetch'))
    const result = await callRequestRpc('request.list')
    if (result.ok) {
      throw new Error('expected error')
    }
    expect(['network', 'unavailable', 'unknown']).toContain(result.error.kind)
  })

  it('does not log params (they can carry request bodies)', async () => {
    const spies = [vi.spyOn(console, 'log'), vi.spyOn(console, 'debug'), vi.spyOn(console, 'error')]
    callRuntimeRpc.mockRejectedValue(rpcError('runtime_error', 'REQUEST_NOT_FOUND: nope'))
    await callRequestRpc('request.create', { body: 'SECRET-BODY' })
    for (const spy of spies) {
      expect(JSON.stringify(spy.mock.calls)).not.toContain('SECRET-BODY')
      spy.mockRestore()
    }
  })

  it('callRequestRpcOrThrow throws the classified error', async () => {
    callRuntimeRpc.mockRejectedValue(rpcError('runtime_error', 'REQUEST_NOT_FOUND: nope'))
    await expect(callRequestRpcOrThrow('request.get', { id: 'x' })).rejects.toMatchObject({
      kind: 'not_found'
    })
  })
})

const THROTTLED = (ms?: number): Error =>
  rpcError(
    'runtime_error',
    `REQUEST_RATE_LIMITED: too many requests${ms === undefined ? '' : ` {"retryAfterMs":${ms}}`}`
  )

describe('callRequestRpc rate limiting', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('retries a read once after retryAfterMs and returns the second answer', async () => {
    callRuntimeRpc.mockRejectedValueOnce(THROTTLED(1500)).mockResolvedValueOnce({ requests: [] })
    const pending = callRequestRpc('request.list', { pageSize: 1 })
    await vi.advanceTimersByTimeAsync(1499)
    expect(callRuntimeRpc).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(2)
    expect(await pending).toEqual({ ok: true, value: { requests: [] } })
    expect(callRuntimeRpc).toHaveBeenCalledTimes(2)
    expect(cooldownRemainingMs('read')).toBe(0)
  })

  it('waits 1 second when the server gave no retryAfterMs', async () => {
    callRuntimeRpc.mockRejectedValueOnce(THROTTLED()).mockResolvedValueOnce({})
    const pending = callRequestRpc('request.get', { id: 'r1' })
    await vi.advanceTimersByTimeAsync(999)
    expect(callRuntimeRpc).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(2)
    expect((await pending).ok).toBe(true)
  })

  it('retries only once, then reports the throttle and starts the class cooldown', async () => {
    callRuntimeRpc.mockRejectedValue(THROTTLED(2000))
    const pending = callRequestRpc('approval.listPending')
    await vi.advanceTimersByTimeAsync(2100)
    const result = await pending
    expect(callRuntimeRpc).toHaveBeenCalledTimes(2)
    if (result.ok) {
      throw new Error('expected error')
    }
    expect(result.error).toMatchObject({
      kind: 'rate_limited',
      backendCode: 'REQUEST_RATE_LIMITED',
      retryAfterMs: 2000
    })
    expect(cooldownRemainingMs('read')).toBeGreaterThanOrEqual(1900)
    expect(cooldownRemainingMs('write')).toBe(0)
  })

  it('never repeats a write that is not safe to repeat; it locks the write class instead', async () => {
    callRuntimeRpc.mockRejectedValue(THROTTLED(1000))
    const result = await callRequestRpc('request.cancel', { id: 'r1' })
    expect(callRuntimeRpc).toHaveBeenCalledTimes(1)
    expect(result.ok).toBe(false)
    expect(cooldownRemainingMs('write')).toBe(1000)
  })

  it('retries version-guarded writes and an explicitly idempotent call', async () => {
    callRuntimeRpc.mockRejectedValueOnce(THROTTLED(100)).mockResolvedValueOnce({ ok: 1 })
    const answer = callRequestRpc('clarification.answer', { clarificationId: 'c1' })
    await vi.advanceTimersByTimeAsync(150)
    expect((await answer).ok).toBe(true)

    callRuntimeRpc.mockReset()
    callRuntimeRpc.mockRejectedValueOnce(THROTTLED(100)).mockResolvedValueOnce({ planTaskId: 'p' })
    const commit = callRequestRpc(
      'request.generatePlan',
      { id: 'r1', mode: 'commit' },
      { idempotent: true }
    )
    await vi.advanceTimersByTimeAsync(150)
    expect((await commit).ok).toBe(true)
    expect(callRuntimeRpc).toHaveBeenCalledTimes(2)
  })

  it('does not auto-retry AI starts, and locks the ai class', async () => {
    callRuntimeRpc.mockRejectedValue(THROTTLED())
    await callRequestRpc('solution.generate', { requestId: 'r1' }, { idempotent: false })
    await callRequestRpc('request.generatePlan', { id: 'r1' })
    expect(callRuntimeRpc).toHaveBeenCalledTimes(2)
    expect(cooldownRemainingMs('ai')).toBe(1000)
  })

  it('does not wait out a long retryAfterMs: the user sees the cooldown instead', async () => {
    callRuntimeRpc.mockRejectedValue(THROTTLED(20_000))
    const result = await callRequestRpc('request.list')
    expect(callRuntimeRpc).toHaveBeenCalledTimes(1)
    expect(result.ok).toBe(false)
    expect(cooldownRemainingMs('read')).toBe(20_000)
  })

  it('does not retry or lock for other limits (AI quota, classification cap) or other errors', async () => {
    callRuntimeRpc.mockRejectedValue(
      rpcError('runtime_error', 'REQUEST_AI_QUOTA_EXHAUSTED: used up')
    )
    const quota = await callRequestRpc('request.list')
    callRuntimeRpc.mockRejectedValue(rpcError('runtime_error', 'REQUEST_CLASSIFICATION_LIMIT: 5'))
    const limit = await callRequestRpc('request.list')
    callRuntimeRpc.mockRejectedValue(rpcError('runtime_error', 'REQUEST_NOT_FOUND: x'))
    await callRequestRpc('request.list')
    expect(callRuntimeRpc).toHaveBeenCalledTimes(3)
    expect(quota.ok || limit.ok).toBe(false)
    expect(cooldownRemainingMs('read')).toBe(0)
  })
})

describe('subscribeRequestEvents', () => {
  it('falls back once on the local target', () => {
    activeTarget = { kind: 'local' }
    const onFallback = vi.fn()
    const off = subscribeRequestEvents({ onEvent: vi.fn(), onFallback })
    off()
    expect(onFallback).toHaveBeenCalledTimes(1)
    expect(subscribeRuntimeStreamChannel).not.toHaveBeenCalled()
  })

  it('forwards parsed events on an environment target', async () => {
    let push: (e: unknown) => void = () => {}
    subscribeRuntimeStreamChannel.mockImplementation(
      async (_t: unknown, _m: string, _p: unknown, cb: (e: unknown) => void) => {
        push = cb
        return { ack: {}, unsubscribe: vi.fn() }
      }
    )
    const onEvent = vi.fn()
    subscribeRequestEvents({ requestId: 'r1', onEvent, onFallback: vi.fn() })
    await Promise.resolve()
    push({
      requestId: 'r1',
      eventType: 'request.status_changed',
      occurredAt: '2026-01-01T00:00:00Z'
    })
    expect(onEvent).toHaveBeenCalledTimes(1)
    expect(onEvent.mock.calls[0][0].requestId).toBe('r1')
    expect(subscribeRuntimeStreamChannel.mock.calls[0][2]).toEqual({ id: 'r1' })
  })

  it('unsubscribes after ack when cancelled before ack', async () => {
    const unsubscribe = vi.fn()
    subscribeRuntimeStreamChannel.mockResolvedValue({ ack: {}, unsubscribe })
    const off = subscribeRequestEvents({ onEvent: vi.fn(), onFallback: vi.fn() })
    off()
    await new Promise((r) => setTimeout(r, 0))
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('falls back when the subscription is unsupported', async () => {
    subscribeRuntimeStreamChannel.mockRejectedValue(rpcError('method_not_found', 'no such method'))
    const onFallback = vi.fn()
    subscribeRequestEvents({ onEvent: vi.fn(), onFallback })
    await new Promise((r) => setTimeout(r, 0))
    expect(onFallback).toHaveBeenCalledTimes(1)
  })
})
