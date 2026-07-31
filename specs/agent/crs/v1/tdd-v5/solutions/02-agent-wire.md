# SOL-02: agent-wire.ts — Binary Frame Codec

**TDD Ref:** TDD-AG-02  
**File:** `src/relay/agent-wire.ts` [NEW]  
**Mức độ:** 🟢 Đơn giản  
**Thời gian ước tính:** 1h

---

## Vấn đề

Agent v1 dùng `FRAME_TYPE = { DATA: 0x01, PING: 0x09, ... }` (magic numbers, duplicated).  
Agent v2.1 phải import `MessageType` từ `relay-protocol.ts` và tạo `WireState` per-connection object (testable).

---

## Giải pháp — Full Implementation

```typescript
// src/relay/agent-wire.ts

/**
 * Binary frame codec for Agent Wire Protocol v1.
 *
 * Frame format: [TYPE u8][SEQ u32 BE][ACK u32 BE][LENGTH u32 BE][PAYLOAD bytes]
 *               ←─────────── 13-byte header ───────────────────→
 *
 * TYPE values: MessageType.Regular (0x01) | MessageType.KeepAlive (0x09)
 * SEQ: sender's frame counter (increments each outgoing frame)
 * ACK: highest SEQ received from peer (must be updated to prevent multiplexer timeout)
 */

import { MessageType } from '../main/ssh/relay-protocol'

export const HEADER_SIZE = 13  // 1 + 4 + 4 + 4

/**
 * Per-connection mutable state.
 * Call createWireState() at the start of each new WebSocket connection.
 * Never share a WireState between connections.
 */
export interface WireState {
  seqCounter: number   // outgoing frame sequence number (starts at 0, pre-increment before send)
  highestAck: number   // highest SEQ received from peer (sent as ACK in our frames)
}

export function createWireState(): WireState {
  return { seqCounter: 0, highestAck: 0 }
}

/**
 * Encode a Regular (JSON-RPC data) frame.
 * Payload can be a string (UTF-8 encoded) or a pre-built Buffer.
 */
export function encodeDataFrame(state: WireState, payload: string | Buffer): Buffer {
  return encodeFrame(state, MessageType.Regular, payload)
}

/**
 * Encode a KeepAlive (ping/pong) frame.
 * Always has empty payload (LENGTH=0, total size=HEADER_SIZE=13 bytes).
 */
export function encodeKeepaliveFrame(state: WireState): Buffer {
  return encodeFrame(state, MessageType.KeepAlive, Buffer.alloc(0))
}

function encodeFrame(state: WireState, type: MessageType, payload: string | Buffer): Buffer {
  const payloadBuf = typeof payload === 'string'
    ? Buffer.from(payload, 'utf8')
    : payload

  const frame = Buffer.allocUnsafe(HEADER_SIZE + payloadBuf.length)
  const seq = ++state.seqCounter

  frame.writeUInt8(type, 0)
  frame.writeUInt32BE(seq, 1)
  frame.writeUInt32BE(state.highestAck, 5)  // ACK field = highest peer SEQ we've seen
  frame.writeUInt32BE(payloadBuf.length, 9)

  if (payloadBuf.length > 0) {
    payloadBuf.copy(frame, HEADER_SIZE)
  }
  return frame
}

export interface DecodedFrame {
  type: MessageType
  seq: number
  ack: number
  length: number
  payload: Buffer
}

/**
 * Decode an incoming binary frame.
 * Side effect: updates state.highestAck to track peer's highest SEQ.
 * Returns null if buffer is too short (malformed frame).
 */
export function decodeFrame(state: WireState, buf: Buffer): DecodedFrame | null {
  if (buf.length < HEADER_SIZE) return null

  const type   = buf.readUInt8(0) as MessageType
  const seq    = buf.readUInt32BE(1)
  const ack    = buf.readUInt32BE(5)
  const length = buf.readUInt32BE(9)
  const payload = buf.subarray(HEADER_SIZE, HEADER_SIZE + length)

  // Track peer's highest SEQ for outgoing ACK field
  if (seq > state.highestAck) {
    state.highestAck = seq
  }

  return { type, seq, ack, length, payload }
}

/**
 * Parse JSON from frame payload.
 * Returns null on parse error (never throws).
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

## Lưu ý triển khai

### Import path của MessageType

`relay-protocol.ts` nằm ở `src/main/ssh/relay-protocol.ts`. Agent wire import:

```typescript
import { MessageType } from '../main/ssh/relay-protocol'
```

⚠️ **Verify** path này tồn tại:
```bash
ls src/main/ssh/relay-protocol.ts  # Must exist
grep "MessageType" src/main/ssh/relay-protocol.ts | head -5
```

Nếu path khác, tìm bằng:
```bash
grep -r "export.*MessageType\|export const MessageType\|export enum MessageType" src/ --include="*.ts" -l
```

---

## Test File

```typescript
// src/relay/__tests__/agent-wire.test.ts
import { describe, it, expect } from 'vitest'
import {
  createWireState, encodeDataFrame, encodeKeepaliveFrame,
  decodeFrame, parseJsonPayload, HEADER_SIZE
} from '../agent-wire'

// Test từng function theo TDD-AG-02 Test Coverage section
// Target: ≥ 15 tests
```

---

## Definition of Done

- [x] `src/relay/agent-wire.ts` created
- [x] `tsc --noEmit -p config/tsconfig.node.json` passes
- [x] `src/relay/__tests__/agent-wire.test.ts` — ≥ 15 tests pass
- [x] `encodeDataFrame + decodeFrame` round-trip test passes
