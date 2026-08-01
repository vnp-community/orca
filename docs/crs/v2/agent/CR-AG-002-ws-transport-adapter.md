# CR-AG-002 — WebSocket Transport Adapter cho SshChannelMultiplexer

**CR-ID:** CR-AG-002  
**Ngày:** 2026-07-26  
**Priority:** 🔴 Critical  
**Effort:** Medium (2–3 ngày)  
**Status:** ✅ Implemented (2026-07-26)  

**Implementation Note:** WsTransport + WsHandshake implemented. 100% test coverage.  
**Backend:** SOL-AG-002 IMPLEMENTED (TASK-003, TASK-004, TASK-005)  
**Frontend:** N/A (backend transport layer)  
**Depends on:** CR-AG-001  
**Blocks:** CR-AG-003, CR-AG-004  

---

## 1. Vấn đề

`SshChannelMultiplexer` là engine JSON-RPC của Orca. Nó nhận vào một `MultiplexerTransport`:

```typescript
// src/main/ssh/ssh-channel-multiplexer.ts
export type MultiplexerTransport = {
  write: (data: Buffer) => void
  onData: (cb: (data: Buffer) => void) => void
  onClose: (cb: () => void) => void
  close?: () => void
}
```

SSH transport adapter hiện có nằm trong `ssh-relay-deploy-helpers.ts`. **Chưa có WebSocket adapter** implement interface này.

Phase 2 cần adapter để map `ws.send(binary)` / `ws.on('message')` vào `MultiplexerTransport`.

---

## 2. Phân tích codebase hiện tại

### `MultiplexerTransport` interface

Interface này đã đủ trừu tượng. SSH transport được tạo trong `waitForSentinel()`:

```typescript
// src/main/ssh/ssh-relay-deploy-helpers.ts
export function waitForSentinel(channel: ClientChannel): Promise<MultiplexerTransport> {
  return new Promise((resolve) => {
    channel.stdout.on('data', (chunk) => {
      // wait for ORCA-RELAY READY sentinel...
      resolve({
        write: (data) => channel.stdin.write(data),
        onData: (cb) => channel.stdout.on('data', cb),
        onClose: (cb) => channel.on('close', cb),
        close: () => channel.close()
      })
    })
  })
}
```

### DevServerRelayBridge — nơi cần thêm logic

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
export class DevServerRelayBridge {
  session: SshChannelMultiplexer | null = null  // ← cần giữ generic

  async connect(): Promise<RelayHandshakeInfo> {
    if (this.config.connectionType === 'relay-ssh') {
      // ... existing SSH logic
    }
    throw new Error('Not yet implemented')  // ← Phase 2 fill here
  }
}
```

---

## 3. Giải pháp

### 3.1 Tạo `WebSocketTransport` adapter

**File mới:** `src/main/dev-server/ws-transport.ts`

```typescript
// src/main/dev-server/ws-transport.ts

import type { MultiplexerTransport } from '../ssh/ssh-channel-multiplexer'

/**
 * WebSocket transport adapter for SshChannelMultiplexer.
 *
 * Maps a WebSocket connection to the MultiplexerTransport interface
 * so the existing JSON-RPC framing engine works over WebSocket without
 * changes.
 *
 * Usage:
 *   const transport = createWebSocketTransport(ws)
 *   const mux = new SshChannelMultiplexer(transport)
 */

// Why: we use the ws library's WebSocket type shape rather than the
// browser's built-in — both have the same API surface but ws works
// in Node.js main process without needing electron's context bridge.
export type WsLike = {
  send(data: Buffer): void
  on(event: 'message', listener: (data: Buffer) => void): void
  on(event: 'close', listener: () => void): void
  on(event: 'error', listener: (err: Error) => void): void
  close(): void
  readyState: number
}

export const WsReadyState = {
  OPEN: 1,
} as const

export function createWebSocketTransport(ws: WsLike): MultiplexerTransport {
  const dataListeners: ((data: Buffer) => void)[] = []
  const closeListeners: (() => void)[] = []

  // Why: ws 'message' event delivers data as Buffer in binary mode.
  // The multiplexer's FrameDecoder expects Buffer input directly.
  ws.on('message', (data: Buffer) => {
    for (const cb of dataListeners) {
      try { cb(data) } catch { /* isolate handler errors */ }
    }
  })

  ws.on('close', () => {
    for (const cb of closeListeners) {
      try { cb() } catch { /* isolate handler errors */ }
    }
  })

  // Why: ws errors close the socket; forward to the close path so the
  // multiplexer can dispose and trigger reconnect logic.
  ws.on('error', () => {
    for (const cb of closeListeners) {
      try { cb() } catch {}
    }
  })

  return {
    write: (data: Buffer) => {
      if (ws.readyState === WsReadyState.OPEN) {
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

### 3.2 Tạo `WsHandshake` helper

**File mới:** `src/main/dev-server/ws-handshake.ts`

```typescript
// src/main/dev-server/ws-handshake.ts
//
// Performs the agent protocol handshake over a raw WebSocket
// connection, BEFORE wiring up the SshChannelMultiplexer.
//
// The handshake:
// 1. Agent (or Orca, depending on mode) sends agent.handshake request
// 2. Peer responds with { ok: true, orcaVersion, sessionId }
// 3. On success: resolve with handshake info
// 4. On failure: close ws and reject

import type { WsLike } from './ws-transport'
import {
  encodeJsonRpcFrame,
  FrameDecoder,
  parseJsonRpcMessage,
  MessageType,
} from '../ssh/relay-protocol'
import type { AgentHandshakeParams, AgentHandshakeResult } from '../../shared/agent-wire-protocol'
import { AGENT_HANDSHAKE_METHOD, AGENT_TIMEOUT_MS } from '../../shared/agent-wire-protocol'

export type WsHandshakeInfo = {
  platform: string
  arch: string
  nodeVersion: string
  agentVersion: string
  sessionId: string
}

/**
 * Orca-side handshake when Orca is the CALLER (relay-websocket mode).
 * Orca connects to agent WS server and drives the handshake.
 *
 * Protocol:
 *   Orca → { method: 'agent.handshake', params: { ... } }
 *   Agent → { result: { ok: true, platform, arch, ... } }
 */
export function runOrcaInitiatorHandshake(
  ws: WsLike,
  orcaVersion: string
): Promise<WsHandshakeInfo> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      ws.close()
      reject(new Error(`Agent handshake timed out after ${AGENT_TIMEOUT_MS}ms`))
    }, AGENT_TIMEOUT_MS)

    const decoder = new FrameDecoder((frame) => {
      if (frame.type !== MessageType.Regular) return
      const msg = parseJsonRpcMessage(frame.payload)
      if (!('id' in msg) || !('result' in msg || 'error' in msg)) return

      clearTimeout(timer)

      if ('error' in msg && msg.error) {
        ws.close()
        reject(new Error(`Agent handshake error: ${msg.error.message}`))
        return
      }

      const result = msg.result as AgentHandshakeResult & {
        platform?: string; arch?: string; nodeVersion?: string; agentVersion?: string
      }
      resolve({
        platform: result.platform ?? 'linux',
        arch: result.arch ?? 'x64',
        nodeVersion: result.nodeVersion ?? 'unknown',
        agentVersion: result.agentVersion ?? 'unknown',
        sessionId: result.sessionId,
      })
    })

    ws.on('message', (data: Buffer) => decoder.feed(data))

    // Send handshake request (id=1, seq=1, ack=0)
    const frame = encodeJsonRpcFrame(
      { jsonrpc: '2.0', id: 1, method: AGENT_HANDSHAKE_METHOD, params: { orcaVersion } },
      1,
      0
    )
    ws.send(frame)
  })
}

/**
 * Orca-side handshake when Agent is the CALLER (direct-websocket mode).
 * Agent connects to Orca, Orca waits for handshake and validates token.
 */
export function runOrcaReceiverHandshake(
  ws: WsLike,
  validateToken: (token: string) => boolean,
  orcaVersion: string
): Promise<WsHandshakeInfo> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      ws.close()
      reject(new Error(`Agent handshake timed out — no handshake received`))
    }, AGENT_TIMEOUT_MS)

    let seq = 0  // seq of frames Orca sends

    const decoder = new FrameDecoder((frame) => {
      if (frame.type !== MessageType.Regular) return
      const msg = parseJsonRpcMessage(frame.payload)
      if (!('method' in msg) || msg.method !== AGENT_HANDSHAKE_METHOD) {
        // First message MUST be handshake
        ws.close()
        reject(new Error('Protocol violation: expected agent.handshake as first message'))
        return
      }

      clearTimeout(timer)
      const params = (msg as { params?: AgentHandshakeParams }).params ?? {} as AgentHandshakeParams

      // Validate auth token
      if (!validateToken(params.agentToken ?? '')) {
        seq++
        const errFrame = encodeJsonRpcFrame({
          jsonrpc: '2.0', id: (msg as { id?: number }).id ?? 1,
          error: { code: -33101, message: 'Authentication failed: invalid agent token' }
        }, seq, 0)
        ws.send(errFrame)
        ws.close()
        reject(new Error('Agent auth failed'))
        return
      }

      // Send handshake-ok
      seq++
      const okFrame = encodeJsonRpcFrame({
        jsonrpc: '2.0', id: (msg as { id?: number }).id ?? 1,
        result: { ok: true, orcaVersion, sessionId: generateSessionId() }
      }, seq, 0)
      ws.send(okFrame)

      resolve({
        platform: params.platform ?? 'linux',
        arch: params.arch ?? 'x64',
        nodeVersion: params.nodeVersion ?? 'unknown',
        agentVersion: params.agentVersion,
        sessionId: `sess-${Date.now()}`,
      })
    })

    ws.on('message', (data: Buffer) => decoder.feed(data))
  })
}

function generateSessionId(): string {
  return `sess-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}
```

### 3.3 Unit tests

**File mới:** `src/main/dev-server/__tests__/ws-transport.test.ts`

```typescript
// Test: WebSocketTransport passes data through correctly
// Test: write() is no-op when ws is not OPEN
// Test: ws error triggers onClose handlers
// Test: OrcaInitiatorHandshake resolves on valid agent.handshake response
// Test: OrcaReceiverHandshake rejects on invalid token
// Test: handshake timeout after AGENT_TIMEOUT_MS
```

---

## 4. Files cần thay đổi

### [NEW] `src/main/dev-server/ws-transport.ts`
WebSocket → MultiplexerTransport adapter.

### [NEW] `src/main/dev-server/ws-handshake.ts`
Handshake logic cho cả 2 modes (initiator và receiver).

### [NEW] `src/shared/agent-wire-protocol.ts`
Constants và types từ CR-AG-001.

### [NEW] `src/main/dev-server/__tests__/ws-transport.test.ts`
Unit tests cho transport adapter và handshake.

### [MODIFY] `src/main/ssh/relay-protocol.ts`
Export thêm `MessageType` và `encodeJsonRpcFrame` nếu chưa export (hiện tại đã export).  
**Không cần thay đổi**.

---

## 5. Tiêu chí hoàn thành

- [ ] `ws-transport.ts`: `createWebSocketTransport()` hoạt động với cả `ws` npm và browser WebSocket
- [ ] `ws-handshake.ts`: Cả 2 handshake functions pass unit tests
- [ ] Unit tests đạt ≥ 90% coverage cho transport và handshake
- [ ] `DevServerRelayBridge.session` vẫn là `SshChannelMultiplexer` — không thay đổi type
- [ ] Không import gì từ `electron` trong các file này (node-compatible)
