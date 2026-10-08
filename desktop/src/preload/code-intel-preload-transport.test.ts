import { describe, expect, it, vi } from 'vitest'
import { createCodeIntelBridge } from '../../../frontend/src/shared/code-intel-bridge'
import { createCodeIntelPreloadDeps } from './code-intel-preload-transport'
import { subscribeRuntimeEnvironmentFromPreload } from './runtime-environment-subscriptions'

type Listener = (event: unknown, payload: unknown) => void

/** Fake ipcRenderer that drives the real subscription dispatcher. */
function createIpc() {
  const listeners = new Set<Listener>()
  const invoke = vi.fn(async (channel: string, args: { subscriptionId?: string }) => {
    if (channel === 'runtimeEnvironments:subscribe') {
      return { subscriptionId: args.subscriptionId, requestId: 'req-1' }
    }
    return { id: 'r1', ok: true, result: { pong: true }, _meta: { runtimeId: 'rt' } }
  })
  return {
    invoke,
    send: vi.fn(),
    on: (_channel: string, l: Listener) => listeners.add(l),
    removeListener: (_channel: string, l: Listener) => listeners.delete(l),
    emit: (payload: unknown) => listeners.forEach((l) => l(null, payload)),
    listenerCount: () => listeners.size
  }
}

const asIpc = (ipc: ReturnType<typeof createIpc>) =>
  ipc as unknown as Parameters<typeof createCodeIntelPreloadDeps>[0]

const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('createCodeIntelPreloadDeps', () => {
  it('calls the environment with the selector field main expects and returns its envelope', async () => {
    const ipc = createIpc()
    const bridge = createCodeIntelBridge(createCodeIntelPreloadDeps(asIpc(ipc)))
    const result = await bridge.call({
      environmentId: 'env-1',
      method: 'codeIntel.status',
      params: { a: 1 }
    })
    expect(ipc.invoke).toHaveBeenCalledWith('runtimeEnvironments:call', {
      selector: 'env-1',
      method: 'codeIntel.status',
      params: { a: 1 }
    })
    expect(result).toMatchObject({ ok: true, result: { pong: true } })
  })

  it('answers method_not_found locally and turns IPC rejections into envelopes', async () => {
    const ipc = createIpc()
    const bridge = createCodeIntelBridge(createCodeIntelPreloadDeps(asIpc(ipc)))
    expect(
      await bridge.call({ environmentId: null, method: 'codeIntel.status', params: {} })
    ).toMatchObject({
      ok: false,
      error: { code: 'method_not_found' }
    })
    expect(ipc.invoke).not.toHaveBeenCalled()
    ipc.invoke.mockRejectedValueOnce(new Error('gone'))
    expect(await bridge.call({ environmentId: 'env-1', method: 'm', params: {} })).toMatchObject({
      ok: false,
      error: { code: 'connection_refused', message: 'gone' }
    })
  })

  it('maps raw subscription frames to parsed push events and closes cleanly', async () => {
    const ipc = createIpc()
    const bridge = createCodeIntelBridge(
      createCodeIntelPreloadDeps(asIpc(ipc), (i, args, cbs) =>
        subscribeRuntimeEnvironmentFromPreload(i, args, cbs, () => 'sub-1')
      )
    )
    const onEvent = vi.fn()
    const onClose = vi.fn()
    const off = bridge.subscribeEvents('env-1', { onEvent, onClose, onUnsupported: vi.fn() })
    await flush()
    expect(ipc.invoke).toHaveBeenCalledWith(
      'runtimeEnvironments:subscribe',
      expect.objectContaining({
        selector: 'env-1',
        method: 'codeIntel.subscribe',
        subscriptionId: 'sub-1'
      })
    )
    const response = (result: unknown) => ({
      subscriptionId: 'sub-1',
      type: 'response',
      response: { id: 'r', ok: true, result, _meta: { runtimeId: 'rt' } }
    })
    ipc.emit(response(null)) // ack
    ipc.emit(response({ event: 'changed', worktreeId: 'wt', reason: 'commit' }))
    ipc.emit(response({ event: 'unknown.event' }))
    expect(onEvent).toHaveBeenCalledTimes(1)
    expect(onEvent.mock.calls[0][0]).toMatchObject({
      event: 'changed',
      worktreeId: 'wt',
      reason: 'commit'
    })
    ipc.emit({ subscriptionId: 'sub-1', type: 'close' })
    expect(onClose).toHaveBeenCalledTimes(1)
    off()
    expect(ipc.listenerCount()).toBe(0)
  })

  it('reports unsupported when the stream is refused and unsubscribes a handle that lands after cleanup', async () => {
    const ipc = createIpc()
    const onUnsupported = vi.fn()
    const refused = createCodeIntelBridge(
      createCodeIntelPreloadDeps(asIpc(ipc), () => Promise.reject(new Error('no stream')))
    )
    refused.subscribeEvents('env-1', { onEvent: vi.fn(), onClose: vi.fn(), onUnsupported })
    await flush()
    expect(onUnsupported).toHaveBeenCalledTimes(1)

    const unsubscribe = vi.fn()
    const late = createCodeIntelBridge(
      createCodeIntelPreloadDeps(asIpc(ipc), () =>
        Promise.resolve({ unsubscribe, sendBinary: vi.fn() })
      )
    )
    const off = late.subscribeEvents('env-1', {
      onEvent: vi.fn(),
      onClose: vi.fn(),
      onUnsupported: vi.fn()
    })
    off()
    await flush()
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })
})
