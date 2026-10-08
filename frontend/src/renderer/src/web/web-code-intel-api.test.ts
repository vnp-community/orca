import { describe, expect, it, vi } from 'vitest'
import { createCodeIntelApi } from './web-code-intel-api'

type StreamHandlers = { onResponse: (r: { ok: boolean; result?: unknown }) => void; onClose: () => void }

function makeApi(opts: { envelope?: unknown; throwOnCall?: Error; stream?: 'handle' | 'null' } = {}) {
  let handlers: StreamHandlers | null = null
  const unsubscribe = vi.fn()
  const callEnvironmentEnvelope = vi.fn(async () => {
    if (opts.throwOnCall) throw opts.throwOnCall
    return opts.envelope ?? { ok: true, result: { x: 1 } }
  })
  const subscribe = vi.fn((_env: string, _method: string, _params: unknown, h: StreamHandlers) => {
    handlers = h
    return opts.stream === 'null' ? null : Promise.resolve({ unsubscribe })
  })
  const api = createCodeIntelApi({ callEnvironmentEnvelope: callEnvironmentEnvelope as never, subscribe })
  return { api, callEnvironmentEnvelope, subscribe, unsubscribe, handlers: () => handlers! }
}

describe('web createCodeIntelApi.call', () => {
  it('passes ok:false envelopes through with the message untouched', async () => {
    const envelope = { ok: false, error: { code: 'internal', message: 'CODEINTEL_TIMEOUT: slow | {"inProgress":true}' } }
    const { api } = makeApi({ envelope })
    await expect(api.call({ environmentId: 'env-1', method: 'codeIntel.status', params: {} })).resolves.toEqual(envelope)
  })

  it('local target (no runtime) resolves method_not_found, never throws', async () => {
    const { api, callEnvironmentEnvelope } = makeApi()
    const res = await api.call({ environmentId: null, method: 'codeIntel.status', params: {} })
    expect(res.ok).toBe(false)
    expect(res.error?.code).toBe('method_not_found')
    expect(callEnvironmentEnvelope).not.toHaveBeenCalled()
  })

  it('transport errors become an ok:false envelope', async () => {
    const { api } = makeApi({ throwOnCall: new Error('socket closed') })
    const res = await api.call({ environmentId: 'env-1', method: 'codeIntel.status', params: {} })
    expect(res).toMatchObject({ ok: false, error: { code: 'connection_refused', message: 'socket closed' } })
  })
})

describe('web createCodeIntelApi.subscribeEvents', () => {
  const callbacks = () => ({ onEvent: vi.fn(), onClose: vi.fn(), onUnsupported: vi.fn() })

  it('drops the null ack and forwards later frames as parsed events', () => {
    const { api, handlers, subscribe } = makeApi()
    const cb = callbacks()
    api.subscribeEvents('env-1', cb)
    expect(subscribe).toHaveBeenCalledWith('env-1', 'codeIntel.subscribe', {}, expect.any(Object))
    handlers().onResponse({ ok: true, result: null })
    expect(cb.onEvent).not.toHaveBeenCalled()
    handlers().onResponse({ ok: true, result: { event: 'changed', worktreeId: 'w', reason: 'commit' } })
    expect(cb.onEvent).toHaveBeenCalledWith(expect.objectContaining({ event: 'changed' }))
    handlers().onResponse({ ok: false })
    handlers().onResponse({ ok: true, result: { event: 'nope' } })
    expect(cb.onEvent).toHaveBeenCalledTimes(1)
  })

  it('no environment stream -> onUnsupported', () => {
    const { api } = makeApi({ stream: 'null' })
    const cb = callbacks()
    api.subscribeEvents('env-1', cb)
    expect(cb.onUnsupported).toHaveBeenCalledTimes(1)
  })

  it('cancelling before the handle resolves still unsubscribes', async () => {
    const { api, unsubscribe } = makeApi()
    const stop = api.subscribeEvents('env-1', callbacks())
    stop()
    await Promise.resolve()
    await Promise.resolve()
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('cancelling after the handle resolves unsubscribes and silences close', async () => {
    const { api, unsubscribe, handlers } = makeApi()
    const cb = callbacks()
    const stop = api.subscribeEvents('env-1', cb)
    await Promise.resolve()
    await Promise.resolve()
    stop()
    expect(unsubscribe).toHaveBeenCalledTimes(1)
    handlers().onClose()
    expect(cb.onClose).not.toHaveBeenCalled()
  })
})
