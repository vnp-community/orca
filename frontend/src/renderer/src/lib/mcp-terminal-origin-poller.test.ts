import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { emitMcpEvent } from './mcp-event-bus'
import {
  ACTIVE_POLL_MS,
  IDLE_POLL_MS,
  startMcpTerminalOriginPolling
} from './mcp-terminal-origin-poller'

const origin = { type: 'mcp', clientName: 'C', mcpSessionId: 's', userId: 'u' } as const

beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('window', {
    addEventListener: vi.fn(),
    removeEventListener: vi.fn()
  })
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('startMcpTerminalOriginPolling', () => {
  it('fetches immediately, backs off when no origins, and speeds up once seen', async () => {
    const fetchOrigins = vi.fn().mockResolvedValue({})
    const apply = vi.fn()
    const stop = startMcpTerminalOriginPolling({ fetchOrigins, apply })
    await vi.advanceTimersByTimeAsync(0)
    expect(fetchOrigins).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(ACTIVE_POLL_MS)
    expect(fetchOrigins).toHaveBeenCalledTimes(1)
    fetchOrigins.mockResolvedValue({ h: origin })
    await vi.advanceTimersByTimeAsync(IDLE_POLL_MS - ACTIVE_POLL_MS)
    expect(fetchOrigins).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(ACTIVE_POLL_MS)
    expect(fetchOrigins).toHaveBeenCalledTimes(3)
    expect(apply).toHaveBeenLastCalledWith({ h: origin })
    stop()
  })

  it('refreshes on session.closed and focus, and cleans every listener on teardown', async () => {
    const fetchOrigins = vi.fn().mockResolvedValue({})
    const stop = startMcpTerminalOriginPolling({ fetchOrigins, apply: vi.fn() })
    await vi.advanceTimersByTimeAsync(0)
    emitMcpEvent({ type: 'session.closed', sessionId: 's' } as never)
    expect(fetchOrigins).toHaveBeenCalledTimes(2)
    const add = (window.addEventListener as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(add[0]).toBe('focus')
    stop()
    expect(window.removeEventListener).toHaveBeenCalledWith('focus', add[1])
    emitMcpEvent({ type: 'session.closed', sessionId: 's' } as never)
    await vi.advanceTimersByTimeAsync(IDLE_POLL_MS * 2)
    expect(fetchOrigins).toHaveBeenCalledTimes(2)
  })

  it('keeps previous data on errors and drops responses that arrive after teardown', async () => {
    let resolve: (v: Record<string, never>) => void = () => {}
    const apply = vi.fn()
    const fetchOrigins = vi
      .fn()
      .mockRejectedValueOnce(new Error('net'))
      .mockImplementationOnce(() => new Promise((r) => (resolve = r)))
    const stop = startMcpTerminalOriginPolling({ fetchOrigins, apply })
    await vi.advanceTimersByTimeAsync(IDLE_POLL_MS)
    expect(apply).not.toHaveBeenCalled()
    stop()
    resolve({})
    await vi.advanceTimersByTimeAsync(0)
    expect(apply).not.toHaveBeenCalled()
  })
})
