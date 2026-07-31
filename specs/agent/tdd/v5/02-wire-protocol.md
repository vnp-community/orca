# TDD-AG-02: Binary Wire Protocol (v2.1)

**Document:** TDD-AG-02
**Version:** 2.1 — Uses shared/agent-wire-protocol.ts
**Date:** 2026-07-28
**Domain:** 13-byte binary frame — typed enums from shared module
**Source files:**
- `src/shared/agent-wire-protocol.ts` ← constants & types (REUSE existing)
- `src/relay/agent-wire.ts` ← [NEW] encodeFrame/decodeFrame typed
**HLD Ref:** C3.8
**ADR:** ADR-005

---

## 1. Shared Constants (src/shared/agent-wire-protocol.ts)

**Đây là file đã có sẵn** — agent-wire.ts IMPORT từ đây, không duplicate:

```typescript
// src/shared/agent-wire-protocol.ts (existing — DO NOT MODIFY wire format)

export const AGENT_KEEPALIVE_INTERVAL_MS = 5_000
export const AGENT_TIMEOUT_MS = 20_000

export const AgentErrorCode = {
  ParseError:          -32700,
  InvalidRequest:      -32600,
  MethodNotFound:      -32601,
  InvalidParams:       -32602,
  ServerError:         -32000,
  CommandNotFound:     -33001,
  PermissionDenied:    -33002,
  PathNotFound:        -33003,
  HandshakeFailed:     -33100,
  AuthFailed:          -33101,
} as const

export type AgentCapability = 'pty' | 'fs' | 'git' | 'preflight'

export type AgentHandshakeParams = {
  agentVersion: string
  platform: string
  arch: string
  nodeVersion?: string
  capabilities: AgentCapability[]
  agentToken?: string
}

export type AgentHandshakeResult = {
  ok: true
  orcaVersion: string
  sessionId: string
}
```

---

## 2. MessageType (from relay-protocol.ts)

```typescript
// src/main/ssh/relay-protocol.ts (existing) — agent imports this
export const enum MessageType {
  Regular   = 0x01,   // JSON-RPC data frames + handshake
  KeepAlive = 0x09,   // Keepalive ping/pong
}
```

---

## 3. agent-wire.ts — Typed Frame Encoder/Decoder

```typescript
// src/relay/agent-wire.ts

import { MessageType } from '../main/ssh/relay-protocol'

export const HEADER_SIZE = 13  // 1 + 4 + 4 + 4 bytes

// Mutable state per connection — reset on each new connection
export interface WireState {
  seqCounter: number
  highestAck: number
}

export function createWireState(): WireState {
  return { seqCounter: 0, highestAck: 0 }
}

export function encodeDataFrame(state: WireState, payload: string | Buffer): Buffer {
  return encodeFrame(state, MessageType.Regular, payload)
}

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
  frame.writeUInt32BE(state.highestAck, 5)
  frame.writeUInt32BE(payloadBuf.length, 9)
  if (payloadBuf.length > 0) payloadBuf.copy(frame, HEADER_SIZE)
  return frame
}

export interface DecodedFrame {
  type: MessageType
  seq: number
  ack: number
  length: number
  payload: Buffer
}

export function decodeFrame(state: WireState, buf: Buffer): DecodedFrame | null {
  if (buf.length < HEADER_SIZE) return null
  const type   = buf.readUInt8(0) as MessageType
  const seq    = buf.readUInt32BE(1)
  const ack    = buf.readUInt32BE(5)
  const length = buf.readUInt32BE(9)
  const payload = buf.subarray(HEADER_SIZE, HEADER_SIZE + length)

  // Track highest SEQ from server to ACK in our next frame
  if (seq > state.highestAck) state.highestAck = seq

  return { type, seq, ack, length, payload }
}

export function parseJsonPayload<T = unknown>(payload: Buffer): T | null {
  try {
    return JSON.parse(payload.toString('utf8')) as T
  } catch {
    return null
  }
}
```

**Key improvements vs v1:**
- `WireState` object (không còn module-level `let seqCounter`) → testable, per-connection isolated
- `MessageType` enum từ relay-protocol.ts (không còn magic numbers `0x01`, `0x09`)
- `encodeDataFrame()` / `encodeKeepaliveFrame()` thay `encodeFrame(FRAME_TYPE.DATA, ...)` ambiguous
- Generic `parseJsonPayload<T>()` với proper null return

---

## 4. Test Coverage

```typescript
// src/relay/__tests__/agent-wire.test.ts
import { describe, it, expect } from 'vitest'
import { createWireState, encodeDataFrame, encodeKeepaliveFrame, decodeFrame, HEADER_SIZE } from '../agent-wire'
import { MessageType } from '../../main/ssh/relay-protocol'

describe('agent-wire', () => {
  it('encodeDataFrame: type=Regular(0x01), increments seqCounter', () => {
    const state = createWireState()
    const frame = encodeDataFrame(state, 'hello')
    expect(frame.readUInt8(0)).toBe(MessageType.Regular)   // 0x01
    expect(frame.readUInt32BE(1)).toBe(1)                   // seq=1
    expect(state.seqCounter).toBe(1)
  })

  it('encodeKeepaliveFrame: type=KeepAlive(0x09), payload length=0', () => {
    const state = createWireState()
    const frame = encodeKeepaliveFrame(state)
    expect(frame.readUInt8(0)).toBe(MessageType.KeepAlive)  // 0x09
    expect(frame.readUInt32BE(9)).toBe(0)                   // length=0
    expect(frame.length).toBe(HEADER_SIZE)
  })

  it('encodeDataFrame: ACK field = highestAck', () => {
    const state = createWireState()
    state.highestAck = 42
    const frame = encodeDataFrame(state, 'test')
    expect(frame.readUInt32BE(5)).toBe(42)                  // ack=42
  })

  it('decodeFrame: updates highestAck from incoming seq', () => {
    const state = createWireState()
    const incoming = Buffer.allocUnsafe(HEADER_SIZE)
    incoming.writeUInt8(MessageType.Regular, 0)
    incoming.writeUInt32BE(7, 1)   // seq=7
    incoming.writeUInt32BE(0, 5)   // ack=0
    incoming.writeUInt32BE(0, 9)   // length=0
    decodeFrame(state, incoming)
    expect(state.highestAck).toBe(7)
  })

  it('decodeFrame: returns null for buffer shorter than HEADER_SIZE', () => {
    const state = createWireState()
    expect(decodeFrame(state, Buffer.alloc(5))).toBeNull()
  })

  it('multiple connections have independent WireState', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    encodeDataFrame(s1, 'a')
    encodeDataFrame(s1, 'b')
    encodeDataFrame(s2, 'c')
    expect(s1.seqCounter).toBe(2)
    expect(s2.seqCounter).toBe(1)
  })
})
```

**Target:** ≥ 15 tests
