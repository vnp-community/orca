// Regression test for TASK-FE-HLD-004 (BUG-FE-HLD-004): the direct-websocket
// slot-expiry instructions must read ORCA_HTTP_PORT instead of hardcoding 6768,
// so operators who override the port aren't told to configure the wrong URL.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { AgentWebSocketServer } from './agent-ws-server'

describe('AgentWebSocketServer — registerSlot expiry message', () => {
  const originalPort = process.env['ORCA_HTTP_PORT']

  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    if (originalPort === undefined) {
      delete process.env['ORCA_HTTP_PORT']
    } else {
      process.env['ORCA_HTTP_PORT'] = originalPort
    }
  })

  it('uses the default port 6768 when ORCA_HTTP_PORT is not set', () => {
    delete process.env['ORCA_HTTP_PORT']
    const server = new AgentWebSocketServer('1.0.0')
    const onExpired = vi.fn()
    server.registerSlot('token-abc', vi.fn(), onExpired)

    vi.runAllTimers()

    expect(onExpired).toHaveBeenCalledTimes(1)
    const message = onExpired.mock.calls[0]?.[0] as string
    expect(message).toContain('ws://<orca-host>:6768')
    expect(message).not.toContain(':undefined')
  })

  it('uses ORCA_HTTP_PORT when overridden, not the hardcoded 6768', () => {
    process.env['ORCA_HTTP_PORT'] = '9999'
    const server = new AgentWebSocketServer('1.0.0')
    const onExpired = vi.fn()
    server.registerSlot('token-abc', vi.fn(), onExpired)

    vi.runAllTimers()

    expect(onExpired).toHaveBeenCalledTimes(1)
    const message = onExpired.mock.calls[0]?.[0] as string
    expect(message).toContain('ws://<orca-host>:9999')
    expect(message).not.toContain(':6768')
  })
})
