// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { startDevServerStatusStream } from './useDevServerStatusStream'

const updateMock = vi.hoisted(() => vi.fn())
const storeState = vi.hoisted(() => ({
  settings: {} as unknown,
  devServers: [{ id: 'ds-1', status: 'connected' }] as { id: string; status: string }[],
  updateDevServerStatus: updateMock
}))
vi.mock('../store', () => ({ useAppStore: Object.assign(vi.fn(), { getState: () => storeState }) }))

const subscribeMock = vi.hoisted(() => vi.fn())
const callMock = vi.hoisted(() => vi.fn())
const targetMock = vi.hoisted(() => vi.fn())
vi.mock('../runtime/runtime-rpc-client', () => ({
  getActiveRuntimeTarget: targetMock,
  subscribeRuntimeStreamChannel: subscribeMock,
  callRuntimeRpc: callMock
}))

type Sub = {
  onEvent: (e: unknown) => void
  options: { onClose?: () => void }
  unsubscribe: ReturnType<typeof vi.fn>
}
let subs: Sub[] = []

const flush = async (): Promise<void> => {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve()
  }
}

beforeEach(() => {
  vi.useFakeTimers()
  subs = []
  updateMock.mockReset()
  callMock.mockReset().mockResolvedValue([])
  targetMock.mockReset().mockReturnValue({ kind: 'environment', environmentId: 'env-1' })
  subscribeMock.mockReset().mockImplementation(async (_t, _m, _p, onEvent, options) => {
    const sub = { onEvent, options, unsubscribe: vi.fn() }
    subs.push(sub)
    return { ack: null, unsubscribe: sub.unsubscribe }
  })
  vi.stubGlobal('api', {})
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('startDevServerStatusStream', () => {
  it('subscribes to devServer.subscribeStatus and writes every pushed snapshot to the store', async () => {
    const stop = startDevServerStatusStream()
    await flush()
    expect(subscribeMock).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'devServer.subscribeStatus',
      {},
      expect.any(Function),
      expect.objectContaining({ onClose: expect.any(Function) })
    )
    subs[0]!.onEvent({ devServerId: 'ds-1', connected: true, activeExecutions: 2, busy: true })
    expect(updateMock).toHaveBeenLastCalledWith('ds-1', 'connected', {
      activeExecutions: 2,
      busy: true
    })
    subs[0]!.onEvent({
      args: [{ devServerId: 'ds-1', connected: false, activeExecutions: 0, busy: false }]
    })
    expect(updateMock).toHaveBeenLastCalledWith('ds-1', 'disconnected', {
      activeExecutions: 0,
      busy: false
    })
    subs[0]!.onEvent({ nothing: true })
    subs[0]!.onEvent(null)
    expect(updateMock).toHaveBeenCalledTimes(2)
    stop()
    expect(subs[0]!.unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('reads devServer.list once for the current state, but an event that arrived first wins', async () => {
    callMock.mockImplementation(async () => {
      subs[0]!.onEvent({ devServerId: 'ds-1', connected: true, activeExecutions: 1, busy: true })
      return [
        { id: 'ds-1', status: 'connected', busy: false, activeExecutions: 0 },
        { id: 'ds-2', status: 'connected', busy: true, activeExecutions: 3 },
        { id: 'ds-3', status: 'connected' }
      ]
    })
    startDevServerStatusStream()
    await flush()
    expect(callMock).toHaveBeenCalledWith(expect.anything(), 'devServer.list', null)
    expect(updateMock).toHaveBeenCalledWith('ds-1', 'connected', {
      activeExecutions: 1,
      busy: true
    })
    expect(updateMock).not.toHaveBeenCalledWith('ds-1', 'connected', {
      activeExecutions: 0,
      busy: false
    })
    expect(updateMock).toHaveBeenCalledWith('ds-2', 'connected', {
      activeExecutions: 3,
      busy: true
    })
    expect(updateMock.mock.calls.some((c) => c[0] === 'ds-3')).toBe(false)
  })

  it('reconnects after the stream closes, and not after stop()', async () => {
    const stop = startDevServerStatusStream()
    await flush()
    subs[0]!.options.onClose?.()
    await vi.advanceTimersByTimeAsync(1_000)
    expect(subs).toHaveLength(2)
    subs[1]!.options.onClose?.()
    stop()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(subs).toHaveLength(2)
  })

  it('clears the busy state when the channel does not exist, and gives up after repeated failures', async () => {
    subscribeMock.mockRejectedValue(Object.assign(new Error('x'), { code: 'method_not_found' }))
    startDevServerStatusStream()
    await flush()
    expect(updateMock).toHaveBeenCalledWith('ds-1', 'connected', {
      activeExecutions: undefined,
      busy: undefined
    })
    expect(subscribeMock).toHaveBeenCalledTimes(1)

    updateMock.mockClear()
    subscribeMock.mockReset().mockRejectedValue(new Error('network'))
    startDevServerStatusStream()
    await vi.advanceTimersByTimeAsync(10 * 60_000)
    expect(subscribeMock.mock.calls.length).toBeLessThanOrEqual(10)
    expect(updateMock).toHaveBeenCalledWith('ds-1', 'connected', {
      activeExecutions: undefined,
      busy: undefined
    })
  })

  it('does nothing on the desktop local target, and picks up the environment once settings load', async () => {
    targetMock.mockReturnValue({ kind: 'local' })
    startDevServerStatusStream()
    await flush()
    expect(subscribeMock).not.toHaveBeenCalled()
    targetMock.mockReturnValue({ kind: 'environment', environmentId: 'env-1' })
    await vi.advanceTimersByTimeAsync(2_000)
    expect(subscribeMock).toHaveBeenCalledTimes(1)
  })
})
