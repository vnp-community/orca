# SOL-AG-002 — WebSocket Transport Adapter + Handshake

**CR:** [CR-AG-002](../../../../../docs/crs/v2/agent/CR-AG-002-ws-transport-adapter.md)  
**TDD Refs:** TDD-05 §4 (Relay Protocol + Multiplexer), TDD-13 §3 (Dev Server)  
**Depends on:** SOL-AG-001  
**Approach:** Test-Driven  
**Status:** ✅ IMPLEMENTED (2026-07-26)  
**Tasks:** [TASK-003](../tasks/TASK-003-ws-transport.md), [TASK-004](../tasks/TASK-004-ws-handshake.md), [TASK-005](../tasks/TASK-005-test-ws-transport-handshake.md)  
**Files:** `src/main/dev-server/ws-transport.ts`, `ws-handshake.ts`, `__tests__/ws-transport.test.ts`  
**Tests:** 21/21 pass | **TypeScript:** 0 errors  

---

## 1. Phân tích từ Code Hiện tại

### 1.1 MultiplexerTransport interface (src/main/ssh/ssh-channel-multiplexer.ts)

```typescript
export type MultiplexerTransport = {
  write: (data: Buffer) => void
  onData: (cb: (data: Buffer) => void) => void
  onClose: (cb: () => void) => void
  close?: () => void
}
```

SSH transport (trong `ssh-relay-deploy-helpers.ts`) implement interface này qua `channel.stdin/stdout`. WebSocket transport cần implement cùng interface qua `ws.send()` / `ws.on('message')`.

### 1.2 relay-protocol.ts exports cần dùng

```typescript
import { encodeJsonRpcFrame, FrameDecoder, parseJsonRpcMessage, MessageType } from '../ssh/relay-protocol'
```

Các hàm này đã export — không cần sửa `relay-protocol.ts`.

### 1.3 GAPs cần fill

- **GAP-2**: Chưa có WebSocket adapter implement `MultiplexerTransport`
- **ws npm package**: Check `package.json` xem `ws` đã có chưa

---

## 2. File Structure

```
src/main/dev-server/
├── ws-transport.ts           ← [NEW] WsLike type + createWebSocketTransport()
├── ws-handshake.ts           ← [NEW] runOrcaInitiatorHandshake() + runOrcaReceiverHandshake()
└── __tests__/
    └── ws-transport.test.ts  ← [NEW] Unit tests
```

---

## 3. Implementation

### 3.1 `src/main/dev-server/ws-transport.ts`

```typescript
// src/main/dev-server/ws-transport.ts
// WebSocket transport adapter for SshChannelMultiplexer.
//
// Why: SshChannelMultiplexer uses MultiplexerTransport interface which was
// originally designed for SSH exec channels (stdin/stdout). This adapter
// maps a WebSocket connection to the same interface so the existing
// JSON-RPC framing engine works over WebSocket without modification.
//
// Deliberately does NOT import from 'electron' — must work in Node.js server mode.

import type { MultiplexerTransport } from '../ssh/ssh-channel-multiplexer'

/**
 * Minimal WebSocket interface accepted by createWebSocketTransport().
 * Compatible with both the `ws` npm library WebSocket and Node.js
 * built-in WebSocket (Node 22+).
 */
export type WsLike = {
  send(data: Buffer): void
  on(event: 'message', listener: (data: Buffer) => void): void
  on(event: 'close', listener: () => void): void
  on(event: 'error', listener: (err: Error) => void): void
  close(): void
  readyState: number
}

// WebSocket readyState constants (RFC 6455)
export const WS_READY_STATE = {
  CONNECTING: 0,
  OPEN: 1,
  CLOSING: 2,
  CLOSED: 3,
} as const

/**
 * Creates a MultiplexerTransport backed by a WebSocket connection.
 *
 * Usage:
 *   const transport = createWebSocketTransport(ws)
 *   const mux = new SshChannelMultiplexer(transport)
 *
 * The caller is responsible for:
 * - Performing handshake BEFORE calling this (use ws-handshake.ts)
 * - Closing the WebSocket when done
 */
export function createWebSocketTransport(ws: WsLike): MultiplexerTransport {
  const dataListeners: ((data: Buffer) => void)[] = []
  const closeListeners: (() => void)[] = []

  // Why: 'message' events from ws library deliver data as Buffer in binary mode.
  // The multiplexer's FrameDecoder expects Buffer input directly — no conversion needed.
  ws.on('message', (data: Buffer) => {
    for (const cb of dataListeners) {
      try {
        cb(data)
      } catch {
        // Isolate individual handler errors — don't crash the transport
      }
    }
  })

  // Why: ws errors always result in a close. Forward to the close path so the
  // multiplexer can dispose and trigger reconnect logic in DevServerManager.
  const notifyClose = () => {
    for (const cb of closeListeners) {
      try {
        cb()
      } catch {}
    }
  }

  ws.on('close', notifyClose)
  ws.on('error', notifyClose)

  return {
    write: (data: Buffer) => {
      // Why: guard against writes after WS is closed (would throw)
      if (ws.readyState === WS_READY_STATE.OPEN) {
        ws.send(data)
      }
    },
    onData: (cb) => {
      dataListeners.push(cb)
    },
    onClose: (cb) => {
      closeListeners.push(cb)
    },
    close: () => {
      ws.close()
    },
  }
}
```

### 3.2 `src/main/dev-server/ws-handshake.ts`

```typescript
// src/main/dev-server/ws-handshake.ts
// Agent protocol handshake over raw WebSocket, BEFORE wiring SshChannelMultiplexer.
//
// Two functions:
//   runOrcaInitiatorHandshake()  — Orca is the WS CLIENT (relay-websocket mode)
//     Orca sends agent.handshake request, waits for agent's result response.
//
//   runOrcaReceiverHandshake()   — Orca is the WS SERVER (direct-websocket mode)
//     Orca waits for agent's agent.handshake request, validates token, sends ok.
//
// Both functions resolve with WsHandshakeInfo on success, reject on failure/timeout.
// Must complete BEFORE createWebSocketTransport() is called.

import type { WsLike } from './ws-transport'
import {
  encodeJsonRpcFrame,
  FrameDecoder,
  parseJsonRpcMessage,
  MessageType,
} from '../ssh/relay-protocol'
import type { AgentHandshakeParams } from '../../shared/agent-wire-protocol'
import {
  AGENT_HANDSHAKE_METHOD,
  AGENT_TIMEOUT_MS,
  AgentErrorCode,
} from '../../shared/agent-wire-protocol'

export type WsHandshakeInfo = {
  platform: string
  arch: string
  nodeVersion: string
  agentVersion: string
  sessionId: string
  agentToken?: string   // set by receiver handshake so caller can match slot
}

// ─── Initiator Handshake (relay-websocket) ────────────────────────────────────

/**
 * Used in relay-websocket mode: Orca connected to agent's WS server.
 * Orca sends agent.handshake, agent responds with platform/arch/etc.
 *
 * Protocol flow:
 *   Orca → { method: 'agent.handshake', params: { orcaVersion } }
 *   Agent → { result: { ok: true, platform, arch, nodeVersion, agentVersion, sessionId } }
 */
export function runOrcaInitiatorHandshake(
  ws: WsLike,
  orcaVersion: string
): Promise<WsHandshakeInfo> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      ws.close()
      reject(new Error(`Agent handshake timed out after ${AGENT_TIMEOUT_MS}ms — no response from agent`))
    }, AGENT_TIMEOUT_MS)

    const decoder = new FrameDecoder((frame) => {
      if (frame.type !== MessageType.Regular) return

      let msg: ReturnType<typeof parseJsonRpcMessage>
      try {
        msg = parseJsonRpcMessage(frame.payload)
      } catch {
        return  // not a valid JSON-RPC message, wait for next
      }

      // Only care about response to our handshake request (id=1)
      if (!('id' in msg)) return
      clearTimeout(timer)

      if ('error' in msg && msg.error) {
        ws.close()
        reject(new Error(`Agent handshake rejected: ${msg.error.message} (code: ${msg.error.code})`))
        return
      }

      const result = (msg as { result?: Record<string, unknown> }).result ?? {}
      resolve({
        platform: (result['platform'] as string) ?? 'linux',
        arch: (result['arch'] as string) ?? 'x64',
        nodeVersion: (result['nodeVersion'] as string) ?? 'unknown',
        agentVersion: (result['agentVersion'] as string) ?? 'unknown',
        sessionId: (result['sessionId'] as string) ?? `sess-${Date.now()}`,
      })
    })

    ws.on('message', (data: Buffer) => decoder.feed(data))

    // Send handshake request: seq=1, ack=0
    const frame = encodeJsonRpcFrame(
      {
        jsonrpc: '2.0',
        id: 1,
        method: AGENT_HANDSHAKE_METHOD,
        params: { orcaVersion },
      },
      1,
      0
    )
    ws.send(frame)
  })
}

// ─── Receiver Handshake (direct-websocket) ────────────────────────────────────

/**
 * Used in direct-websocket mode: Orca is the WS server, agent connected to us.
 * Orca waits for agent's handshake, validates agentToken, sends ok.
 *
 * Protocol flow:
 *   Agent → { method: 'agent.handshake', params: { agentToken, platform, ... } }
 *   Orca  → { result: { ok: true, orcaVersion, sessionId } }
 *         OR { error: { code: -33101, message: 'Auth failed' } }
 */
export function runOrcaReceiverHandshake(
  ws: WsLike,
  validateToken: (token: string) => boolean,
  orcaVersion: string
): Promise<WsHandshakeInfo> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      ws.close()
      reject(new Error(`Agent did not send handshake within ${AGENT_TIMEOUT_MS}ms`))
    }, AGENT_TIMEOUT_MS)

    let outSeq = 0

    const decoder = new FrameDecoder((frame) => {
      if (frame.type !== MessageType.Regular) return

      let msg: ReturnType<typeof parseJsonRpcMessage>
      try {
        msg = parseJsonRpcMessage(frame.payload)
      } catch {
        ws.close()
        reject(new Error('Protocol violation: received invalid JSON-RPC as first message'))
        return
      }

      // First message MUST be agent.handshake
      if (!('method' in msg) || msg.method !== AGENT_HANDSHAKE_METHOD) {
        clearTimeout(timer)
        ws.close()
        reject(new Error(`Protocol violation: expected '${AGENT_HANDSHAKE_METHOD}', got '${('method' in msg ? msg.method : 'response')}'`))
        return
      }

      clearTimeout(timer)
      const requestId = (msg as { id?: number }).id ?? 1
      const params = (msg as { params?: AgentHandshakeParams }).params

      // Validate auth token
      const agentToken = params?.agentToken ?? ''
      if (!validateToken(agentToken)) {
        outSeq++
        const errFrame = encodeJsonRpcFrame(
          {
            jsonrpc: '2.0',
            id: requestId,
            error: { code: AgentErrorCode.AuthFailed, message: 'Authentication failed: invalid agent token' },
          },
          outSeq,
          0
        )
        ws.send(errFrame)
        ws.close()
        reject(new Error('Agent authentication failed: invalid token'))
        return
      }

      // Send handshake-ok
      const sessionId = `sess-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
      outSeq++
      const okFrame = encodeJsonRpcFrame(
        {
          jsonrpc: '2.0',
          id: requestId,
          result: { ok: true, orcaVersion, sessionId },
        },
        outSeq,
        0
      )
      ws.send(okFrame)

      resolve({
        platform: params?.platform ?? 'linux',
        arch: params?.arch ?? 'x64',
        nodeVersion: params?.nodeVersion ?? 'unknown',
        agentVersion: params?.agentVersion ?? 'unknown',
        sessionId,
        agentToken,
      })
    })

    ws.on('message', (data: Buffer) => decoder.feed(data))
  })
}
```

---

## 4. Test Specifications

```typescript
// src/main/dev-server/__tests__/ws-transport.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createWebSocketTransport, WS_READY_STATE } from '../ws-transport'
import { runOrcaInitiatorHandshake, runOrcaReceiverHandshake } from '../ws-handshake'
import { encodeJsonRpcFrame } from '../../ssh/relay-protocol'
import { AGENT_HANDSHAKE_METHOD } from '../../../shared/agent-wire-protocol'

// ─── Mock WsLike ────────────────────────────────────────────────────────────

function createMockWs(readyState = WS_READY_STATE.OPEN) {
  const listeners: Record<string, ((data: Buffer) => void)[]> = {}
  const sent: Buffer[] = []
  return {
    readyState,
    send: vi.fn((data: Buffer) => sent.push(data)),
    close: vi.fn(),
    on: vi.fn((event: string, cb: (data: Buffer) => void) => {
      if (!listeners[event]) listeners[event] = []
      listeners[event].push(cb)
    }),
    // Test helpers
    _emit: (event: string, data?: Buffer) => {
      listeners[event]?.forEach((cb) => cb(data as Buffer))
    },
    _sent: sent,
  }
}

// ─── WebSocket Transport ─────────────────────────────────────────────────────

describe('createWebSocketTransport', () => {
  it('forwards incoming message to onData handlers', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const received: Buffer[] = []
    transport.onData((data) => received.push(data))

    const payload = Buffer.from('hello')
    ws._emit('message', payload)

    expect(received).toHaveLength(1)
    expect(received[0]).toEqual(payload)
  })

  it('write() sends to ws when OPEN', () => {
    const ws = createMockWs(WS_READY_STATE.OPEN)
    const transport = createWebSocketTransport(ws)
    const data = Buffer.from('test')
    transport.write(data)
    expect(ws.send).toHaveBeenCalledWith(data)
  })

  it('write() is a no-op when ws is not OPEN', () => {
    const ws = createMockWs(WS_READY_STATE.CLOSED)
    const transport = createWebSocketTransport(ws)
    transport.write(Buffer.from('should not send'))
    expect(ws.send).not.toHaveBeenCalled()
  })

  it('ws error triggers onClose handlers', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const onClose = vi.fn()
    transport.onClose(onClose)
    ws._emit('error', undefined)
    expect(onClose).toHaveBeenCalled()
  })

  it('ws close triggers onClose handlers', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    const onClose = vi.fn()
    transport.onClose(onClose)
    ws._emit('close', undefined)
    expect(onClose).toHaveBeenCalled()
  })

  it('close() calls ws.close()', () => {
    const ws = createMockWs()
    const transport = createWebSocketTransport(ws)
    transport.close?.()
    expect(ws.close).toHaveBeenCalled()
  })
})

// ─── Initiator Handshake ─────────────────────────────────────────────────────

describe('runOrcaInitiatorHandshake', () => {
  it('resolves with agent info on valid handshake response', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    // Verify Orca sent handshake request
    expect(ws.send).toHaveBeenCalledOnce()

    // Simulate agent response
    const response = encodeJsonRpcFrame({
      jsonrpc: '2.0',
      id: 1,
      result: {
        ok: true,
        platform: 'linux',
        arch: 'arm64',
        nodeVersion: 'v20.11.0',
        agentVersion: '1.0.0',
        sessionId: 'sess-test-123',
      },
    }, 1, 0)
    ws._emit('message', response)

    const info = await promise
    expect(info.platform).toBe('linux')
    expect(info.arch).toBe('arm64')
    expect(info.sessionId).toBe('sess-test-123')
  })

  it('rejects on error response from agent', async () => {
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')

    const errorResponse = encodeJsonRpcFrame({
      jsonrpc: '2.0',
      id: 1,
      error: { code: -33100, message: 'Version too old' },
    }, 1, 0)
    ws._emit('message', errorResponse)

    await expect(promise).rejects.toThrow('Agent handshake rejected')
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects on timeout', async () => {
    vi.useFakeTimers()
    const ws = createMockWs()
    const promise = runOrcaInitiatorHandshake(ws, '1.4.0')
    vi.advanceTimersByTime(21_000)
    await expect(promise).rejects.toThrow('timed out')
    expect(ws.close).toHaveBeenCalled()
    vi.useRealTimers()
  })
})

// ─── Receiver Handshake ──────────────────────────────────────────────────────

describe('runOrcaReceiverHandshake', () => {
  const validToken = 'valid-token-123'
  const validateToken = (t: string) => t === validToken

  it('resolves on valid agent.handshake with correct token', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    const handshake = encodeJsonRpcFrame({
      jsonrpc: '2.0',
      id: 1,
      method: AGENT_HANDSHAKE_METHOD,
      params: {
        agentVersion: '1.0.0',
        agentToken: validToken,
        platform: 'linux',
        arch: 'x64',
        nodeVersion: 'v20.11.0',
        capabilities: ['fs', 'git'],
      },
    }, 1, 0)
    ws._emit('message', handshake)

    const info = await promise
    expect(info.platform).toBe('linux')
    expect(info.agentToken).toBe(validToken)
    expect(ws.send).toHaveBeenCalledOnce()   // sent handshake-ok
    expect(ws.close).not.toHaveBeenCalled()  // did NOT close
  })

  it('rejects on invalid token', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    const handshake = encodeJsonRpcFrame({
      jsonrpc: '2.0',
      id: 1,
      method: AGENT_HANDSHAKE_METHOD,
      params: { agentVersion: '1.0.0', agentToken: 'wrong-token', platform: 'linux', arch: 'x64', capabilities: [] },
    }, 1, 0)
    ws._emit('message', handshake)

    await expect(promise).rejects.toThrow('authentication failed')
    expect(ws.close).toHaveBeenCalled()
  })

  it('rejects if first message is not agent.handshake', async () => {
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')

    const wrongMsg = encodeJsonRpcFrame({
      jsonrpc: '2.0',
      id: 1,
      method: 'preflight.check',
      params: {},
    }, 1, 0)
    ws._emit('message', wrongMsg)

    await expect(promise).rejects.toThrow('Protocol violation')
  })

  it('rejects on timeout (no message)', async () => {
    vi.useFakeTimers()
    const ws = createMockWs()
    const promise = runOrcaReceiverHandshake(ws, validateToken, '1.4.0')
    vi.advanceTimersByTime(21_000)
    await expect(promise).rejects.toThrow('did not send handshake')
    vi.useRealTimers()
  })
})
```

---

## 5. Acceptance Criteria

- [ ] `createWebSocketTransport()` compiles without importing from `electron`
- [ ] `WsLike` interface compatible với `ws` npm WebSocket
- [ ] Transport: `write()` no-op khi `readyState !== OPEN`
- [ ] Transport: ws error → triggers all `onClose` handlers
- [ ] Initiator handshake: resolves với platform/arch/sessionId từ agent
- [ ] Initiator handshake: rejects khi agent trả error response
- [ ] Initiator handshake: rejects sau `AGENT_TIMEOUT_MS` (20s)
- [ ] Receiver handshake: resolves khi token hợp lệ, gửi handshake-ok
- [ ] Receiver handshake: rejects + close ws khi token sai
- [ ] Receiver handshake: rejects nếu first message không phải `agent.handshake`
- [ ] All unit tests pass (coverage ≥ 90%)
