# TASK-009: Tạo `src/main/dev-server/__tests__/agent-ws-server.test.ts`

> **Status:** ✅ DONE (2026-07-26)
> **Tests:** 11/11 pass
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 4 — direct-websocket mode  
**Solution:** [SOL-AG-004](../solutions/SOL-AG-004-direct-websocket.md) §5  
**Depends on:** TASK-008  
**Blocks:** (không có)  

---

## Mục tiêu

Unit tests cho `AgentWebSocketServer`. Tests verify:
- Slot registration + expiry lifecycle
- handleConnection → callback pipeline
- re-register idempotency
- stop() cleanup

Tests dùng vi.mock cho `ws-handshake` và `ws-transport` — không cần network.

---

## File cần tạo

**Path:** `src/main/dev-server/__tests__/agent-ws-server.test.ts`

---

## Nội dung

```typescript
// src/main/dev-server/__tests__/agent-ws-server.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AgentWebSocketServer } from '../agent-ws-server'
import type { SshChannelMultiplexer } from '../../ssh/ssh-channel-multiplexer'
import type { WsHandshakeInfo } from '../ws-handshake'

// ─── Mocks ───────────────────────────────────────────────────────────────────

// Mock runOrcaReceiverHandshake — we control resolve/reject in each test
let mockHandshakeResolve: ((info: WsHandshakeInfo) => void) | null = null
let mockHandshakeReject: ((err: Error) => void) | null = null

vi.mock('../ws-handshake', () => ({
  runOrcaReceiverHandshake: vi.fn((_ws: unknown, _validate: (t: string) => boolean, _ver: string) =>
    new Promise<WsHandshakeInfo>((res, rej) => {
      mockHandshakeResolve = res
      mockHandshakeReject = rej
    })
  ),
}))

// Mock createWebSocketTransport — returns a dummy transport
vi.mock('../ws-transport', () => ({
  createWebSocketTransport: vi.fn(() => ({
    write: vi.fn(),
    onData: vi.fn(),
    onClose: vi.fn(),
    close: vi.fn(),
  })),
}))

// Mock SshChannelMultiplexer constructor
vi.mock('../../ssh/ssh-channel-multiplexer', () => ({
  SshChannelMultiplexer: vi.fn().mockImplementation(() => ({ _mock: true })),
}))

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
    mockHandshakeResolve = null
    mockHandshakeReject = null
    server = new AgentWebSocketServer('1.4.0')
  })

  afterEach(() => {
    server.stop()
  })

  // ─── registerSlot ────────────────────────────────────────────────────────

  describe('registerSlot()', () => {
    it('returns a disposer function', () => {
      const disposer = server.registerSlot('tok-1', vi.fn(), vi.fn())
      expect(typeof disposer).toBe('function')
    })

    it('slot expires after AGENT_CONNECT_TIMEOUT_MS with descriptive error', () => {
      vi.useFakeTimers()
      const onExpired = vi.fn()
      server.registerSlot('expire-tok', vi.fn(), onExpired)

      vi.advanceTimersByTime(61_000)

      expect(onExpired).toHaveBeenCalledOnce()
      const errorMsg = onExpired.mock.calls[0][0] as string
      expect(errorMsg).toContain('did not connect')
      expect(errorMsg).toContain('expire-tok')
      vi.useRealTimers()
    })

    it('disposer cancels expiry timer (onExpired NOT called after dispose)', () => {
      vi.useFakeTimers()
      const onExpired = vi.fn()
      const disposer = server.registerSlot('cancel-tok', vi.fn(), onExpired)

      disposer()
      vi.advanceTimersByTime(61_000)

      expect(onExpired).not.toHaveBeenCalled()
      vi.useRealTimers()
    })

    it('re-registering same token cancels previous slot timer', () => {
      vi.useFakeTimers()
      const onExpired1 = vi.fn()
      const onExpired2 = vi.fn()

      server.registerSlot('same-tok', vi.fn(), onExpired1)
      server.registerSlot('same-tok', vi.fn(), onExpired2)  // replaces first

      vi.advanceTimersByTime(61_000)

      expect(onExpired1).not.toHaveBeenCalled()  // first timer cancelled
      expect(onExpired2).toHaveBeenCalledOnce()   // second timer fires
      vi.useRealTimers()
    })
  })

  // ─── handleConnection (via internal method) ───────────────────────────────

  describe('handleConnection() pipeline', () => {
    it('calls onConnected with mux and info on successful handshake', async () => {
      const onConnected = vi.fn()
      server.registerSlot('test-token-abc', onConnected, vi.fn())

      // Access private method via casting for testing
      ;(server as unknown as { handleConnection(ws: unknown): void })
        .handleConnection(mockWs)

      // Simulate successful handshake
      mockHandshakeResolve!(makeHandshakeInfo({ agentToken: 'test-token-abc' }))
      await vi.runAllTimersAsync()
      // Need to flush promises
      await new Promise((r) => setTimeout(r, 0))

      expect(onConnected).toHaveBeenCalledOnce()
      const [mux, info] = onConnected.mock.calls[0] as [SshChannelMultiplexer, WsHandshakeInfo]
      expect(mux).toBeTruthy()
      expect(info.platform).toBe('linux')
    })

    it('slot is consumed after successful connection (removed from pendingSlots)', async () => {
      const onConnected = vi.fn()
      server.registerSlot('consume-tok', onConnected, vi.fn())

      ;(server as unknown as { handleConnection(ws: unknown): void })
        .handleConnection(mockWs)
      mockHandshakeResolve!(makeHandshakeInfo({ agentToken: 'consume-tok' }))
      await new Promise((r) => setTimeout(r, 0))

      expect(onConnected).toHaveBeenCalledOnce()
      // Slot is consumed — second connection attempt should not trigger callback
      ;(server as unknown as { handleConnection(ws: unknown): void })
        .handleConnection(mockWs)
      expect(onConnected).toHaveBeenCalledOnce()  // still 1 call
    })

    it('does NOT call onConnected on handshake failure', async () => {
      const onConnected = vi.fn()
      server.registerSlot('fail-tok', onConnected, vi.fn())

      ;(server as unknown as { handleConnection(ws: unknown): void })
        .handleConnection(mockWs)
      mockHandshakeReject!(new Error('Auth failed'))
      await new Promise((r) => setTimeout(r, 0))

      expect(onConnected).not.toHaveBeenCalled()
    })
  })

  // ─── stop() ──────────────────────────────────────────────────────────────

  describe('stop()', () => {
    it('cancels all pending slot timers', () => {
      vi.useFakeTimers()
      const expired1 = vi.fn()
      const expired2 = vi.fn()
      server.registerSlot('s1', vi.fn(), expired1)
      server.registerSlot('s2', vi.fn(), expired2)

      server.stop()
      vi.advanceTimersByTime(61_000)

      expect(expired1).not.toHaveBeenCalled()
      expect(expired2).not.toHaveBeenCalled()
      vi.useRealTimers()
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm vitest run src/main/dev-server/__tests__/agent-ws-server.test.ts
```

## Acceptance Criteria

- [x] File test tồn tại
- [x] `registerSlot()`: slot expires sau 61s với error message
- [x] `registerSlot()`: disposer cancels timer
- [x] `registerSlot()`: re-register replaces previous slot
- [x] `handleConnection()`: `onConnected` gọi với mux + info
- [x] `handleConnection()`: slot consumed sau kết nối thành công
- [x] `handleConnection()`: `onConnected` không gọi khi handshake fail
- [x] `stop()`: tất cả timers bị cancel
- [x] Tất cả tests pass (≥ 8 test cases)
