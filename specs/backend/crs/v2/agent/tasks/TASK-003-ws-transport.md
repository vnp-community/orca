# TASK-003: Tạo `src/main/dev-server/ws-transport.ts`

> **Status:** ✅ DONE (2026-07-26)
> **File created:** `src/main/dev-server/ws-transport.ts`
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 2 — Transport Layer  
**Solution:** [SOL-AG-002](../solutions/SOL-AG-002-ws-transport-adapter.md) §3.1  
**Depends on:** TASK-001  
**Blocks:** TASK-004, TASK-005, TASK-006, TASK-008  

---

## Mục tiêu

Tạo WebSocket transport adapter cho `SshChannelMultiplexer`. Adapter này map
`WebSocket` (ws npm) vào `MultiplexerTransport` interface nên toàn bộ JSON-RPC
framing engine hiện tại hoạt động qua WebSocket mà không cần thay đổi.

**Quan trọng:** Không import từ `electron` — phải work trong Node.js server mode.

---

## File cần tạo

**Path:** `src/main/dev-server/ws-transport.ts`

---

## Nội dung

```typescript
// src/main/dev-server/ws-transport.ts
// WebSocket transport adapter for SshChannelMultiplexer.
//
// Why: SshChannelMultiplexer.MultiplexerTransport was originally designed
// for SSH exec channels (stdin/stdout). This adapter maps a WebSocket
// connection to the same interface so the existing JSON-RPC framing engine
// works over WebSocket without any modification.
//
// Does NOT import from 'electron' — compatible with Node.js server mode.

import type { MultiplexerTransport } from '../ssh/ssh-channel-multiplexer'

/**
 * Minimal WebSocket interface accepted by createWebSocketTransport().
 *
 * Compatible with:
 *   - ws npm library WebSocket (ws ^8.x)
 *   - Node.js built-in WebSocket (Node 22+)
 *
 * Using WsLike instead of importing `ws` directly keeps this file
 * free of optional peer dependencies and easier to unit-test with mocks.
 */
export type WsLike = {
  send(data: Buffer): void
  on(event: 'message', listener: (data: Buffer) => void): void
  on(event: 'close', listener: () => void): void
  on(event: 'error', listener: (err: Error) => void): void
  close(): void
  readyState: number
}

/** WebSocket readyState constants per RFC 6455 */
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
 * Caller responsibilities:
 *   - Perform handshake BEFORE calling this (use ws-handshake.ts)
 *   - Close the WebSocket when done via transport.close() or ws.close()
 */
export function createWebSocketTransport(ws: WsLike): MultiplexerTransport {
  const dataListeners: ((data: Buffer) => void)[] = []
  const closeListeners: (() => void)[] = []

  // Why: ws library 'message' event delivers data as Buffer in binary mode.
  // The multiplexer's FrameDecoder expects Buffer input directly — no conversion.
  ws.on('message', (data: Buffer) => {
    for (const cb of dataListeners) {
      try {
        cb(data)
      } catch {
        // Isolate handler errors — one bad handler must not crash transport
      }
    }
  })

  // Why: WebSocket errors always result in a close. Forward to close path
  // so the multiplexer can dispose and trigger reconnect logic upstream.
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
      // Why: guard against writes after WS is closed (ws.send() throws if closed)
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

---

## Acceptance Criteria

- [x] File `src/main/dev-server/ws-transport.ts` tồn tại
- [x] Không có `import` từ `electron` hoặc `ws` npm
- [x] `WsLike` type export được
- [x] `WS_READY_STATE` export được với `OPEN = 1`
- [x] `createWebSocketTransport()` export được và return `MultiplexerTransport`
- [x] TypeScript compile không lỗi
