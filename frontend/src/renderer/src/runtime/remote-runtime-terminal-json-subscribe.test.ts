import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { RemoteRuntimeMultiplexedTerminalCallbacks } from './remote-runtime-terminal-multiplexer'

describe('subscribeTerminalViaJson', () => {
  const runtimeCall = vi.fn()
  const runtimeSubscribe = vi.fn()
  let responseHandler: ((response: unknown) => void) | null = null
  let errorHandler: ((error: { code: string; message: string }) => void) | null = null
  let closeHandler: (() => void) | null = null
  const unsubscribe = vi.fn()

  function makeCallbacks(): {
    onData: ReturnType<typeof vi.fn>
    onSnapshot: ReturnType<typeof vi.fn>
    onSubscribed: ReturnType<typeof vi.fn>
    onEnd: ReturnType<typeof vi.fn>
    onError: ReturnType<typeof vi.fn>
    onTransportClose: ReturnType<typeof vi.fn>
  } & RemoteRuntimeMultiplexedTerminalCallbacks {
    const onData = vi.fn()
    const onSnapshot = vi.fn()
    const onSubscribed = vi.fn()
    const onEnd = vi.fn()
    const onError = vi.fn()
    const onTransportClose = vi.fn()
    return { onData, onSnapshot, onSubscribed, onEnd, onError, onTransportClose }
  }

  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    responseHandler = null
    errorHandler = null
    closeHandler = null
    runtimeCall.mockResolvedValue({ ok: true, result: { unsubscribed: true } })
    runtimeSubscribe.mockImplementation(
      async (
        _args: unknown,
        callbacks: {
          onResponse: (response: unknown) => void
          onError?: (error: { code: string; message: string }) => void
          onClose?: () => void
        }
      ) => {
        responseHandler = callbacks.onResponse
        errorHandler = callbacks.onError ?? null
        closeHandler = callbacks.onClose ?? null
        return { unsubscribe }
      }
    )
    vi.stubGlobal('window', {
      api: {
        runtimeEnvironments: {
          call: runtimeCall,
          subscribe: runtimeSubscribe
        }
      }
    })
  })

  it('calls terminal.subscribe with the terminal handle, client, and viewport', async () => {
    const { subscribeTerminalViaJson } = await import('./remote-runtime-terminal-json-subscribe')
    await subscribeTerminalViaJson({
      environmentId: 'session-auth',
      terminal: 'terminal-1',
      client: { id: 'desktop:tab-1', type: 'desktop' },
      viewport: { cols: 100, rows: 30 },
      callbacks: makeCallbacks()
    })

    expect(runtimeSubscribe).toHaveBeenCalledWith(
      {
        selector: 'session-auth',
        method: 'terminal.subscribe',
        params: {
          terminal: 'terminal-1',
          client: { id: 'desktop:tab-1', type: 'desktop' },
          viewport: { cols: 100, rows: 30 }
        }
      },
      expect.any(Object)
    )
  })

  it('delivers scrollback as onSnapshot and live data as onData', async () => {
    const { subscribeTerminalViaJson } = await import('./remote-runtime-terminal-json-subscribe')
    const callbacks = makeCallbacks()
    await subscribeTerminalViaJson({
      environmentId: 'session-auth',
      terminal: 'terminal-1',
      client: { id: 'desktop:tab-1', type: 'desktop' },
      callbacks
    })

    responseHandler?.({ ok: true, result: { type: 'scrollback', lines: ['hello', 'world'] } })
    expect(callbacks.onSnapshot).toHaveBeenCalledWith('hello\nworld')
    expect(callbacks.onSubscribed).toHaveBeenCalledTimes(1)

    responseHandler?.({ ok: true, result: { type: 'data', chunk: 'live output' } })
    expect(callbacks.onData).toHaveBeenCalledWith('live output')

    // onSubscribed should only fire once even if another snapshot-shaped event arrives.
    responseHandler?.({ ok: true, result: { type: 'scrollback', lines: ['more'] } })
    expect(callbacks.onSubscribed).toHaveBeenCalledTimes(1)
  })

  it('calls onEnd exactly when the server emits type "end"', async () => {
    const { subscribeTerminalViaJson } = await import('./remote-runtime-terminal-json-subscribe')
    const callbacks = makeCallbacks()
    await subscribeTerminalViaJson({
      environmentId: 'session-auth',
      terminal: 'terminal-1',
      client: { id: 'desktop:tab-1', type: 'desktop' },
      callbacks
    })

    responseHandler?.({ ok: true, result: { type: 'end' } })
    expect(callbacks.onEnd).toHaveBeenCalledTimes(1)

    // Events arriving after 'end' must not be forwarded.
    responseHandler?.({ ok: true, result: { type: 'data', chunk: 'late data' } })
    expect(callbacks.onData).not.toHaveBeenCalled()
  })

  it('forwards RPC-level errors and transport-close notifications', async () => {
    const { subscribeTerminalViaJson } = await import('./remote-runtime-terminal-json-subscribe')
    const callbacks = makeCallbacks()
    await subscribeTerminalViaJson({
      environmentId: 'session-auth',
      terminal: 'terminal-1',
      client: { id: 'desktop:tab-1', type: 'desktop' },
      callbacks
    })

    errorHandler?.({ code: 'boom', message: 'something broke' })
    expect(callbacks.onError).toHaveBeenCalledWith('something broke')

    closeHandler?.()
    expect(callbacks.onTransportClose).toHaveBeenCalledTimes(1)
  })

  it('surfaces a failed RPC response (ok: false) via onError', async () => {
    const { subscribeTerminalViaJson } = await import('./remote-runtime-terminal-json-subscribe')
    const callbacks = makeCallbacks()
    await subscribeTerminalViaJson({
      environmentId: 'session-auth',
      terminal: 'terminal-1',
      client: { id: 'desktop:tab-1', type: 'desktop' },
      callbacks
    })

    responseHandler?.({ ok: false, error: { code: 'bad_request', message: 'terminal not found' } })
    expect(callbacks.onError).toHaveBeenCalledWith(expect.stringContaining('terminal not found'))
  })

  it('close() unsubscribes locally and sends terminal.unsubscribe once', async () => {
    const { subscribeTerminalViaJson } = await import('./remote-runtime-terminal-json-subscribe')
    const stream = await subscribeTerminalViaJson({
      environmentId: 'session-auth',
      terminal: 'terminal-1',
      client: { id: 'desktop:tab-1', type: 'desktop' },
      callbacks: makeCallbacks()
    })

    stream.close()
    expect(unsubscribe).toHaveBeenCalledTimes(1)
    expect(runtimeCall).toHaveBeenCalledWith({
      selector: 'session-auth',
      method: 'terminal.unsubscribe',
      params: { subscriptionId: 'terminal-1:desktop:tab-1', client: { id: 'desktop:tab-1' } }
    })

    // Calling close() again must not send a second unsubscribe.
    stream.close()
    expect(unsubscribe).toHaveBeenCalledTimes(1)
    expect(runtimeCall).toHaveBeenCalledTimes(1)
  })

  it('sendInput/resize/claimViewport always return false so the caller falls back to plain RPCs', async () => {
    const { subscribeTerminalViaJson } = await import('./remote-runtime-terminal-json-subscribe')
    const stream = await subscribeTerminalViaJson({
      environmentId: 'session-auth',
      terminal: 'terminal-1',
      client: { id: 'desktop:tab-1', type: 'desktop' },
      callbacks: makeCallbacks()
    })

    expect(stream.sendInput('hi')).toBe(false)
    expect(stream.resize(80, 24)).toBe(false)
    expect(stream.claimViewport(80, 24)).toBe(false)
    await expect(stream.serializeBuffer()).resolves.toBeNull()
  })
})
