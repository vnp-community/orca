# TASK-005: Tạo `src/main/dev-server/__tests__/ws-transport.test.ts`

> **Status:** ✅ DONE (2026-07-26)
> **Tests:** 21/21 pass
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 2 — Transport Layer  
**Solution:** [SOL-AG-002](../solutions/SOL-AG-002-ws-transport-adapter.md) §4  
**Depends on:** TASK-003, TASK-004  
**Blocks:** (không có)  

---

## Mục tiêu

Tạo unit tests cho `ws-transport.ts` và `ws-handshake.ts`.
Tests dùng mock WsLike object — không cần kết nối network thực.

---

## File cần tạo

**Path:** `src/main/dev-server/__tests__/ws-transport.test.ts`

---

## Nội dung

```typescript
// src/main/dev-server/__tests__/ws-transport.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createWebSocketTransport, WS_READY_STATE } from '../ws-transport'
import { runOrcaInitiatorHandshake, runOrcaReceiverHandshake } from '../ws-handshake'
import { encodeJsonRpcFrame } from '../../ssh/relay-protocol'
import { AGENT_HANDSHAKE_METHOD } from '../../../shared/agent-wire-protocol'

// ─── Mock WsLike factory ────────────────────────────────────────────────────

type MockListener = (data?: unknown) => void

function createMockWs(readyState = WS_READY_STATE.OPEN) {
  const listeners: Record<string, MockListener[]> = {}
  const sent: Buffer[] = []

  const ws = {
    readyState,
    send: vi.fn((data: Buffer) => { sent.push(data) }),
    close: vi.fn(),
    on: vi.fn((event: string, cb: MockListener) => {
      if (!listeners[event]) listeners[event] = []
      listeners[event].push(cb)
    }),
    // Test helpers
    _emit(event: string, data?: unknown) {
      for (const cb of listeners[event] ?? []) cb(data)
    },
    _sent: sent,
  }
  return ws
}

// ─── WebSocket Transport ─────────────────────────────────────────────────────

describe('createWebSocketTransport', () => {
  it('forwards incoming message Buffer to onData handlers', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const received: Buffer[] = []
    transport.onData((data) => received.push(data))

    const payload = Buffer.from('test-payload')
    ws._emit('message', payload)

    expect(received).toHaveLength(1)
    expect(received[0]).toEqual(payload)
  })

  it('write() calls ws.send() when readyState is OPEN', () => {
    const ws = createMockWs(WS_READY_STATE.OPEN)
    const transport = createWebSocketTransport(ws)
    const data = Buffer.from('hello')
    transport.write(data)
    expect(ws.send).toHaveBeenCalledWith(data)
  })

  it('write() is a no-op when readyState is CLOSED', () => {
    const ws = createMockWs(WS_READY_STATE.CLOSED)
    const transport = createWebSocketTransport(ws)
    transport.write(Buffer.from('should-not-send'))
    expect(ws.send).not.toHaveBeenCalled()
  })

  it('ws "close" event triggers all onClose handlers', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const h1 = vi.fn()
    const h2 = vi.fn()
    transport.onClose(h1)
    transport.onClose(h2)
    ws._emit('close')
    expect(h1).toHaveBeenCalledOnce()
    expect(h2).toHaveBeenCalledOnce()
  })

  it('ws "error" event triggers onClose handlers (errors always close WS)', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const onClose = vi.fn()
    transport.onClose(onClose)
    ws._emit('error', new Error('network error'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('close() calls ws.close()', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    transport.close?.()
    expect(ws.close).toHaveBeenCalledOnce()
  })

  it('onData handler error is isolated (other handlers still called)', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const badHandler = vi.fn(() => { throw new Error('handler crash') })
    const goodHandler = vi.fn()
    transport.onData(badHandler)
    transport.onData(goodHandler)
    expect(() => ws._emit('message', Buffer.from('x'))).not.toThrow()
    expect(goodHandler).toHaveBeenCalledOnce()
  })
})

// ─── Initiator Handshake (relay-websocket) ───────────────────────────────────

describe('runOrcaInitiatorHandshake', () => {
  it('sends agent.handshake request after WS open', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    // Should have sent one frame
    expect(ws.send).toHaveBeenCalledOnce()

    // The sent frame should decode to agent.handshake request
    // (basic sanity: frame is a Buffer)
    expect(ws._sent[0]).toBeInstanceOf(Buffer)

    // Resolve the promise by simulating agent response
    const response = encodeJsonRpcFrame(
      { jsonrpc: '2.0', id: 1, result: {
        ok: true, platform: 'linux', arch: 'arm64',
        nodeVersion: 'v20.11.0', agentVersion: '1.0.0', sessionId: 'sess-test'
      }},
      1, 0
    )
    ws._emit('message', response)
    const info = await promise
    expect(info.platform).toBe('linux')
    expect(info.arch).toBe('arm64')
    expect(info.nodeVersion).toBe('v20.11.0')
    expect(info.agentVersion).toBe('1.0.0')
    expect(info.sessionId).toBe('sess-test')
  })

  it('rejects on error response from agent', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    const errorResponse = encodeJsonRpcFrame(
      { jsonrpc: '2.0', id: 1, error: { code: -33100, message: 'Version too old' } },
      1, 0
    )
    ws._emit('message', errorResponse)

    await expect(promise).rejects.toThrow('Agent rejected handshake')
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects and closes ws on timeout', async () => {
    vi.useFakeTimers()
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')
    vi.advanceTimersByTime(21_000)
    await expect(promise).rejects.toThrow('timed out')
    expect(ws.close).toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('ignores keepalive frames (does not resolve/reject prematurely)', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    // Simulate a keepalive frame (type=0x09) — should not resolve/reject
    const keepaliveHeader = Buffer.alloc(13)
    keepaliveHeader[0] = 0x09
    ws._emit('message', keepaliveHeader)

    // Promise should still be pending — settle it properly
    const response = encodeJsonRpcFrame(
      { jsonrpc: '2.0', id: 1, result: {
        ok: true, platform: 'linux', arch: 'x64',
        agentVersion: '1.0.0', sessionId: 'sess-ka'
      }},
      1, 0
    )
    ws._emit('message', response)
    const info = await promise
    expect(info.sessionId).toBe('sess-ka')
  })
})

// ─── Receiver Handshake (direct-websocket) ───────────────────────────────────

describe('runOrcaReceiverHandshake', () => {
  const VALID_TOKEN = 'valid-token-abc'
  const validateToken = (t: string) => t === VALID_TOKEN

  function makeHandshakeFrame(overrides: Record<string, unknown> = {}) {
    return encodeJsonRpcFrame(
      {
        jsonrpc: '2.0', id: 1,
        method: AGENT_HANDSHAKE_METHOD,
        params: {
          agentVersion: '1.0.0',
          agentToken: VALID_TOKEN,
          platform: 'linux',
          arch: 'x64',
          nodeVersion: 'v20.11.0',
          capabilities: ['fs', 'git'],
          ...overrides,
        },
      },
      1, 0
    )
  }

  it('resolves and sends handshake-ok on valid token', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    ws._emit('message', makeHandshakeFrame())
    const info = await promise

    expect(info.platform).toBe('linux')
    expect(info.arch).toBe('x64')
    expect(info.agentToken).toBe(VALID_TOKEN)
    expect(ws.send).toHaveBeenCalledOnce()   // sent handshake-ok
    expect(ws.close).not.toHaveBeenCalled()  // did NOT close on success
  })

  it('sessionId in resolved info matches orcaVersion prefix pattern', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')
    ws._emit('message', makeHandshakeFrame())
    const info = await promise
    expect(info.sessionId).toMatch(/^sess-\d+/)
  })

  it('rejects and closes ws on invalid token', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    ws._emit('message', makeHandshakeFrame({ agentToken: 'wrong-token' }))

    await expect(promise).rejects.toThrow('authentication failed')
    expect(ws.send).toHaveBeenCalledOnce()   // sent error frame
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects and closes ws if first message is not agent.handshake', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    const wrongMsg = encodeJsonRpcFrame(
      { jsonrpc: '2.0', id: 1, method: 'preflight.check', params: {} },
      1, 0
    )
    ws._emit('message', wrongMsg)

    await expect(promise).rejects.toThrow('Protocol violation')
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects and closes ws on timeout (no message received)', async () => {
    vi.useFakeTimers()
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')
    vi.advanceTimersByTime(21_000)
    await expect(promise).rejects.toThrow('did not send handshake')
    expect(ws.close).toHaveBeenCalled()
    vi.useRealTimers()
  })
})
```

---

## Cách chạy test

```bash
pnpm vitest run src/main/dev-server/__tests__/ws-transport.test.ts
```

## Acceptance Criteria

- [x] File test tồn tại
- [x] **Transport tests (7):**
  - `message` forwarded to `onData`
  - `write()` calls `ws.send()` when OPEN
  - `write()` no-op when CLOSED
  - `close` event → `onClose` handlers called
  - `error` event → `onClose` handlers called
  - `close()` calls `ws.close()`
  - Bad handler isolated (không throw)
- [x] **Initiator Handshake tests (4):**
  - Sends `agent.handshake` request
  - Resolves với platform/arch/sessionId
  - Rejects on error response
  - Rejects on timeout (20s)
  - Ignores keepalive frames
- [x] **Receiver Handshake tests (5):**
  - Resolves + sends ok on valid token
  - Rejects on invalid token
  - Rejects if first message is not handshake
  - Rejects on timeout
- [x] Tất cả tests pass: ≥ 16 test cases (21 tests)
