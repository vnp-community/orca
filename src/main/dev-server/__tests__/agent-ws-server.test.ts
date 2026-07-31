// src/main/dev-server/__tests__/agent-ws-server.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AgentWebSocketServer } from '../agent-ws-server'
import type { SshChannelMultiplexer } from '../../ssh/ssh-channel-multiplexer'
import type { WsHandshakeInfo } from '../ws-handshake'

// ─── Mocks ───────────────────────────────────────────────────────────────────

// vi.hoisted() runs before vi.mock() factories, making handshakeCtrl available
const handshakeCtrl = vi.hoisted(() => ({
  resolve: null as ((info: WsHandshakeInfo) => void) | null,
  reject: null as ((err: Error) => void) | null,
}))

vi.mock('../ws-handshake', () => ({
  runOrcaReceiverHandshake: vi.fn(
    (_ws: unknown, _validate: (t: string) => boolean, _ver: string) =>
      new Promise<WsHandshakeInfo>((res, rej) => {
        handshakeCtrl.resolve = res
        handshakeCtrl.reject = rej
      })
  ),
}))


vi.mock('../ws-transport', () => ({
  createWebSocketTransport: vi.fn(() => ({
    write: vi.fn(),
    onData: vi.fn(),
    onClose: vi.fn(),
    close: vi.fn(),
  })),
}))

vi.mock('../../ssh/ssh-channel-multiplexer', () => ({
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  SshChannelMultiplexer: vi.fn(function (this: any) {
    this._isMock = true
  }),
}))


// ─── Helpers ─────────────────────────────────────────────────────────────────

function makeHandshakeInfo(overrides: Partial<WsHandshakeInfo> = {}): WsHandshakeInfo {
  return {
    platform: 'linux',
    arch: 'x64',
    nodeVersion: 'v20.11.0',
    agentVersion: '1.0.0',
    sessionId: 'sess-test-123',
    agentToken: 'test-token-abc',
    ...overrides,
  }
}

const mockWs = { close: vi.fn() }

// ─── Tests ───────────────────────────────────────────────────────────────────

describe('AgentWebSocketServer', () => {
  let server: AgentWebSocketServer

  beforeEach(() => {
    vi.clearAllMocks()
    handshakeCtrl.resolve = null
    handshakeCtrl.reject = null
    server = new AgentWebSocketServer('1.4.0')
  })

  afterEach(() => {
    server.stop()
    vi.useRealTimers()
  })

  // ─── registerSlot() ──────────────────────────────────────────────────────

  describe('registerSlot()', () => {
    it('returns a disposer function', () => {
      const disposer = server.registerSlot('tok-1', vi.fn(), vi.fn())
      expect(typeof disposer).toBe('function')
    })

    it('slot expires after AGENT_CONNECT_TIMEOUT_MS with descriptive error message', () => {
      vi.useFakeTimers()
      const onExpired = vi.fn()
      server.registerSlot('expire-tok', vi.fn(), onExpired)

      vi.advanceTimersByTime(61_000)

      expect(onExpired).toHaveBeenCalledOnce()
      const errorMsg = onExpired.mock.calls[0][0] as string
      expect(errorMsg).toContain('did not connect')
      expect(errorMsg).toContain('expire-tok')
    })

    it('disposer cancels expiry timer (onExpired NOT called after dispose)', () => {
      vi.useFakeTimers()
      const onExpired = vi.fn()
      const disposer = server.registerSlot('cancel-tok', vi.fn(), onExpired)

      disposer()
      vi.advanceTimersByTime(61_000)

      expect(onExpired).not.toHaveBeenCalled()
    })

    it('re-registering same token cancels the previous slot timer', () => {
      vi.useFakeTimers()
      const onExpired1 = vi.fn()
      const onExpired2 = vi.fn()

      server.registerSlot('same-tok', vi.fn(), onExpired1)
      server.registerSlot('same-tok', vi.fn(), onExpired2) // replaces first

      vi.advanceTimersByTime(61_000)

      expect(onExpired1).not.toHaveBeenCalled() // first timer cancelled
      expect(onExpired2).toHaveBeenCalledOnce() // second timer fires
    })

    it('different tokens each have independent expiry', () => {
      vi.useFakeTimers()
      const onExpired1 = vi.fn()
      const onExpired2 = vi.fn()

      server.registerSlot('tok-a', vi.fn(), onExpired1)
      server.registerSlot('tok-b', vi.fn(), onExpired2)

      vi.advanceTimersByTime(61_000)

      expect(onExpired1).toHaveBeenCalledOnce()
      expect(onExpired2).toHaveBeenCalledOnce()
    })
  })

  // ─── handleConnection() ──────────────────────────────────────────────────

  describe('handleConnection() pipeline', () => {
    function triggerConnection() {
      ;(server as unknown as { handleConnection(ws: unknown): void }).handleConnection(mockWs)
    }

    it('calls onConnected with a SshChannelMultiplexer and WsHandshakeInfo', async () => {
      const onConnected = vi.fn()
      server.registerSlot('test-token-abc', onConnected, vi.fn())

      triggerConnection()
      handshakeCtrl.resolve!(makeHandshakeInfo({ agentToken: 'test-token-abc' }))
      await new Promise((r) => setTimeout(r, 0))

      expect(onConnected).toHaveBeenCalledOnce()
      const [mux, info] = onConnected.mock.calls[0] as [SshChannelMultiplexer, WsHandshakeInfo]
      expect(mux).toBeTruthy()
      expect(info.platform).toBe('linux')
      expect(info.agentToken).toBe('test-token-abc')
    })

    it('slot is consumed after successful connection (second connection rejected)', async () => {
      const onConnected = vi.fn()
      server.registerSlot('consume-tok', onConnected, vi.fn())

      // First connection
      triggerConnection()
      handshakeCtrl.resolve!(makeHandshakeInfo({ agentToken: 'consume-tok' }))
      await new Promise((r) => setTimeout(r, 0))
      expect(onConnected).toHaveBeenCalledOnce()

      // Second connection attempt — slot is gone
      vi.clearAllMocks()
      handshakeCtrl.resolve = null
      triggerConnection()
      handshakeCtrl.resolve?.(makeHandshakeInfo({ agentToken: 'consume-tok' }))
      await new Promise((r) => setTimeout(r, 0))
      expect(onConnected).not.toHaveBeenCalled()
    })

    it('does NOT call onConnected when handshake fails', async () => {
      const onConnected = vi.fn()
      server.registerSlot('fail-tok', onConnected, vi.fn())

      triggerConnection()
      handshakeCtrl.reject!(new Error('Auth failed — invalid token'))
      await new Promise((r) => setTimeout(r, 0))

      expect(onConnected).not.toHaveBeenCalled()
    })

    it('expiry timer is cancelled once slot is consumed by successful connect', async () => {
      vi.useFakeTimers()
      const onExpired = vi.fn()
      const onConnected = vi.fn()
      server.registerSlot('timer-cancel-tok', onConnected, onExpired)

      // Trigger connection in fake timer mode — flush microtasks manually
      ;(server as unknown as { handleConnection(ws: unknown): void }).handleConnection(mockWs)

      vi.useRealTimers() // switch to real timers for await
      handshakeCtrl.resolve!(makeHandshakeInfo({ agentToken: 'timer-cancel-tok' }))
      await new Promise((r) => setTimeout(r, 0))

      // If timer was not cancelled, advancing fake timers would fire onExpired
      vi.useFakeTimers()
      vi.advanceTimersByTime(61_000)
      expect(onExpired).not.toHaveBeenCalled()
    })
  })

  // ─── stop() ──────────────────────────────────────────────────────────────

  describe('stop()', () => {
    it('cancels all pending slot timers on stop()', () => {
      vi.useFakeTimers()
      const expired1 = vi.fn()
      const expired2 = vi.fn()
      server.registerSlot('s1', vi.fn(), expired1)
      server.registerSlot('s2', vi.fn(), expired2)

      server.stop()
      vi.advanceTimersByTime(61_000)

      expect(expired1).not.toHaveBeenCalled()
      expect(expired2).not.toHaveBeenCalled()
    })

    it('stop() is idempotent (can be called multiple times)', () => {
      expect(() => {
        server.stop()
        server.stop()
      }).not.toThrow()
    })
  })
})
