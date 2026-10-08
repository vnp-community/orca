// Tests for request-rpc-client.ts (CR-REQ-018-02)
import { describe, it, expect, vi, beforeEach } from 'vitest'

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

import {
  callRequestRpc,
  callRequestRpcOrThrow,
  subscribeRequestEvents
} from './request-rpc-client'

function rpcError(code: string, message: string): Error {
  return Object.assign(new Error(message), { code })
}

beforeEach(() => {
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
    if (!result.ok) {expect(result.error.kind).toBe('conflict')}
  })

  it('classifies method_not_found as unsupported', async () => {
    callRuntimeRpc.mockRejectedValue(rpcError('method_not_found', 'no such method'))
    const result = await callRequestRpc('request.list')
    if (result.ok) {throw new Error('expected error')}
    expect(result.error.kind).toBe('unsupported')
  })

  it('never throws on a network failure', async () => {
    callRuntimeRpc.mockRejectedValue(new TypeError('Failed to fetch'))
    const result = await callRequestRpc('request.list')
    if (result.ok) {throw new Error('expected error')}
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
    push({ requestId: 'r1', eventType: 'request.status_changed', occurredAt: '2026-01-01T00:00:00Z' })
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
