# TASK-04: Create src/relay/agent-wire.ts

**Phase:** 2  
**SOL Ref:** SOL-02  
**Estimated time:** 1h  
**Precondition:** TASK-01~03 hoàn thành  

---

## Tạo file mới: `src/relay/agent-wire.ts`

**QUAN TRỌNG — Import path chính xác:**  
`MessageType` nằm tại `src/main/ssh/relay-protocol.ts` với giá trị `{ Regular: 1, KeepAlive: 9 }`.  
Import từ agent-wire.ts: `import { MessageType } from '../main/ssh/relay-protocol'`

```typescript
// src/relay/agent-wire.ts
// Binary frame codec for Agent Wire Protocol v1.
//
// Frame format (13-byte header):
//   [TYPE u8][SEQ u32 BE][ACK u32 BE][LENGTH u32 BE][PAYLOAD bytes]
//
// TYPE: MessageType.Regular (1) = JSON-RPC data
//       MessageType.KeepAlive (9) = ping/pong, empty payload
//
// SEQ: outgoing frame counter (pre-incremented before each send)
// ACK: highest incoming SEQ seen from peer (updated by decodeFrame)
//
// Invariant: Never share a WireState between connections.

import { MessageType } from '../main/ssh/relay-protocol'

export const HEADER_SIZE = 13  // 1 (type) + 4 (seq) + 4 (ack) + 4 (length)

/**
 * Per-connection mutable state.
 * Create one per WebSocket connection via createWireState().
 */
export interface WireState {
  seqCounter: number   // incremented before each outgoing frame
  highestAck: number   // highest SEQ received from peer → sent as ACK
}

export function createWireState(): WireState {
  return { seqCounter: 0, highestAck: 0 }
}

/** Encode a JSON-RPC data frame (TYPE=Regular=1). */
export function encodeDataFrame(state: WireState, payload: string | Buffer): Buffer {
  return encodeFrame(state, MessageType.Regular, payload)
}

/** Encode a keepalive frame (TYPE=KeepAlive=9, LENGTH=0). */
export function encodeKeepaliveFrame(state: WireState): Buffer {
  return encodeFrame(state, MessageType.KeepAlive, Buffer.alloc(0))
}

function encodeFrame(state: WireState, type: number, payload: string | Buffer): Buffer {
  const payloadBuf = typeof payload === 'string'
    ? Buffer.from(payload, 'utf8')
    : payload
  const frame = Buffer.allocUnsafe(HEADER_SIZE + payloadBuf.length)
  const seq = ++state.seqCounter
  frame.writeUInt8(type, 0)
  frame.writeUInt32BE(seq, 1)
  frame.writeUInt32BE(state.highestAck, 5)
  frame.writeUInt32BE(payloadBuf.length, 9)
  if (payloadBuf.length > 0) payloadBuf.copy(frame, HEADER_SIZE)
  return frame
}

export interface DecodedFrame {
  type: number
  seq: number
  ack: number
  length: number
  payload: Buffer
}

/**
 * Decode an incoming binary frame.
 * Updates state.highestAck as a side effect.
 * Returns null if buf is shorter than HEADER_SIZE (malformed).
 */
export function decodeFrame(state: WireState, buf: Buffer): DecodedFrame | null {
  if (buf.length < HEADER_SIZE) return null
  const type   = buf.readUInt8(0)
  const seq    = buf.readUInt32BE(1)
  const ack    = buf.readUInt32BE(5)
  const length = buf.readUInt32BE(9)
  const payload = buf.subarray(HEADER_SIZE, HEADER_SIZE + length)
  if (seq > state.highestAck) state.highestAck = seq
  return { type, seq, ack, length, payload }
}

/**
 * Parse JSON from frame payload. Returns null on error (never throws).
 */
export function parseJsonPayload<T = unknown>(payload: Buffer): T | null {
  if (payload.length === 0) return null
  try {
    return JSON.parse(payload.toString('utf8')) as T
  } catch {
    return null
  }
}
```

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-wire" || echo "No errors for agent-wire"
```

## Definition of Done

- [x] `src/relay/agent-wire.ts` created
- [x] `pnpm run typecheck:node` passes (no new errors)
- [x] `MessageType` import from `'../main/ssh/relay-protocol'` resolves correctly
