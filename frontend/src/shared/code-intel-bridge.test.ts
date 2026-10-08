import { describe, expect, it, vi } from 'vitest'
import { createCodeIntelBridge } from './code-intel-bridge'
import type { CodeIntelRawSubscribeCallbacks } from './code-intel-bridge'

function makeBridge() {
  let captured: CodeIntelRawSubscribeCallbacks | null = null
  const unsubscribe = vi.fn()
  const callLocal = vi.fn().mockResolvedValue({ ok: true, result: 'local' })
  const callEnvironment = vi.fn().mockResolvedValue({ ok: true, result: 'env' })
  const subscribeEnvironment = vi.fn((_env, _method, _params, cb: CodeIntelRawSubscribeCallbacks) => {
    captured = cb
    return unsubscribe
  })
  const bridge = createCodeIntelBridge({ callLocal, callEnvironment, subscribeEnvironment })
  return { bridge, callLocal, callEnvironment, subscribeEnvironment, unsubscribe, cb: () => captured! }
}

function handlers() {
  return { onEvent: vi.fn(), onClose: vi.fn(), onUnsupported: vi.fn() }
}

describe('createCodeIntelBridge.call', () => {
  it('routes environmentId === null to the local path', async () => {
    const { bridge, callLocal, callEnvironment } = makeBridge()
    await expect(bridge.call({ environmentId: null, method: 'codeIntel.status', params: { a: 1 } })).resolves.toEqual({
      ok: true,
      result: 'local'
    })
    expect(callLocal).toHaveBeenCalledWith('codeIntel.status', { a: 1 })
    expect(callEnvironment).not.toHaveBeenCalled()
  })

  it('routes an environment id to callEnvironment and defaults params to {}', async () => {
    const { bridge, callEnvironment } = makeBridge()
    await bridge.call({ environmentId: 'env-1', method: 'codeIntel.status', params: undefined })
    expect(callEnvironment).toHaveBeenCalledWith('env-1', 'codeIntel.status', {})
  })

  it('exposes no generic send function', () => {
    const { bridge } = makeBridge()
    expect(Object.keys(bridge).sort()).toEqual(['call', 'subscribeEvents'])
  })
})

describe('createCodeIntelBridge.subscribeEvents', () => {
  it('local target: onUnsupported once, no transport subscription', () => {
    const { bridge, subscribeEnvironment } = makeBridge()
    const h = handlers()
    const stop = bridge.subscribeEvents(null, h)
    expect(h.onUnsupported).toHaveBeenCalledTimes(1)
    expect(subscribeEnvironment).not.toHaveBeenCalled()
    expect(() => stop()).not.toThrow()
  })

  it('subscribes to codeIntel.subscribe and forwards known events', () => {
    const { bridge, subscribeEnvironment, cb } = makeBridge()
    const h = handlers()
    bridge.subscribeEvents('env-1', h)
    expect(subscribeEnvironment).toHaveBeenCalledWith('env-1', 'codeIntel.subscribe', {}, expect.any(Object))
    cb().onEvent({ event: 'changed', worktreeId: 'w', reason: 'commit', resync: false })
    expect(h.onEvent).toHaveBeenCalledWith(expect.objectContaining({ event: 'changed', reason: 'commit' }))
  })

  it('normalizes dotted wire events', () => {
    const { bridge, cb } = makeBridge()
    const h = handlers()
    bridge.subscribeEvents('env-1', h)
    cb().onEvent({ event: 'quality.progress', worktreeId: 'w', runId: 'r', percent: null })
    expect(h.onEvent).toHaveBeenCalledWith(expect.objectContaining({ event: 'qualityProgress', percent: null }))
  })

  it('drops the null ack, unknown events and non-objects', () => {
    const { bridge, cb } = makeBridge()
    const h = handlers()
    bridge.subscribeEvents('env-1', h)
    cb().onEvent(null)
    cb().onEvent({ event: 'surprise' })
    cb().onEvent('x')
    expect(h.onEvent).not.toHaveBeenCalled()
  })

  it('cancelling stops forwarding and unsubscribes (safe before ack)', () => {
    const { bridge, cb, unsubscribe } = makeBridge()
    const h = handlers()
    const stop = bridge.subscribeEvents('env-1', h)
    stop()
    expect(unsubscribe).toHaveBeenCalledTimes(1)
    cb().onEvent({ event: 'changed', worktreeId: 'w' })
    cb().onClose()
    expect(h.onEvent).not.toHaveBeenCalled()
    expect(h.onClose).not.toHaveBeenCalled()
  })

  it('forwards onClose and onUnsupported while active', () => {
    const { bridge, cb } = makeBridge()
    const h = handlers()
    bridge.subscribeEvents('env-1', h)
    cb().onClose()
    cb().onUnsupported()
    expect(h.onClose).toHaveBeenCalledTimes(1)
    expect(h.onUnsupported).toHaveBeenCalledTimes(1)
  })
})
