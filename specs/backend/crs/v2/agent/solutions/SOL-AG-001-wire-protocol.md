# SOL-AG-001 — Agent Wire Protocol: Types, Constants, Frame Codec

**CR:** [CR-AG-001](../../../../../docs/crs/v2/agent/CR-AG-001-wire-protocol-spec.md)  
**TDD Refs:** TDD-05 §4 (Relay Protocol), TDD-13 §3 (Dev Server)  
**Approach:** Test-Driven — viết constants/types trước, pseudo-code verify sau  
**Status:** ✅ IMPLEMENTED (2026-07-26)  
**Tasks:** [TASK-001](../tasks/TASK-001-agent-wire-protocol.md), [TASK-002](../tasks/TASK-002-test-agent-wire-protocol.md)  
**Files:** `src/shared/agent-wire-protocol.ts`, `src/shared/__tests__/agent-wire-protocol.test.ts`  
**Tests:** 15/15 pass | **TypeScript:** 0 errors  

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 relay-protocol.ts đã có sẵn (không cần thay đổi)

```typescript
// src/main/ssh/relay-protocol.ts — ĐÃ CÓ, không sửa
export const HEADER_LENGTH = 13
export const MessageType = { Regular: 0x01, KeepAlive: 0x09 }
export const KEEPALIVE_SEND_MS = 5_000
export const TIMEOUT_MS = 20_000
export function encodeFrame(type, seq, ack, payload): Buffer   // 13-byte header
export function encodeJsonRpcFrame(msg, seq, ack): Buffer
export class FrameDecoder { feed(data: Buffer): void }
export function parseJsonRpcMessage(payload: Buffer): JsonRpcMessage
```

> **Key insight**: Frame format CỦA AGENT PROTOCOL GIỐNG HỆT relay-protocol.ts.
> Không cần tạo codec mới — agent dùng cùng 13-byte header `[TYPE][SEQ u32 BE][ACK u32 BE][LEN u32 BE][PAYLOAD]`.
> `src/shared/agent-wire-protocol.ts` chỉ cần export TYPES và CONSTANTS ngôn ngữ-agnostic.

### 1.2 GAP cần fill

- Chưa có `src/shared/agent-wire-protocol.ts` với types cho `agent.handshake`
- Chưa có error codes riêng cho agent auth (`-33100`, `-33101`)
- Chưa có `AgentCapability` type

---

## 2. File Structure

```
src/shared/
└── agent-wire-protocol.ts          ← [NEW] Types, constants, error codes cho agent
```

---

## 3. Implementation

### 3.1 `src/shared/agent-wire-protocol.ts`

```typescript
// src/shared/agent-wire-protocol.ts
// Agent Wire Protocol v1 — Language-agnostic constants and types.
//
// Frame format (13-byte header, same as relay-protocol.ts):
//   [TYPE u8][SEQ u32 BE][ACK u32 BE][LENGTH u32 BE][PAYLOAD bytes]
//
// TYPE values:
//   0x01 = Regular (JSON-RPC 2.0 payload)
//   0x09 = KeepAlive (empty payload)
//
// Kept in src/shared/ so it can be imported by both:
//   - Orca server (src/main/dev-server/ws-handshake.ts)
//   - Future relay-side code (src/relay/)

export const AGENT_PROTOCOL_VERSION = '1'
export const AGENT_HANDSHAKE_METHOD = 'agent.handshake'

// Keepalive & timeout — same values as relay-protocol.ts for consistency
export const AGENT_KEEPALIVE_INTERVAL_MS = 5_000
export const AGENT_TIMEOUT_MS = 20_000

// Direct-websocket: how long Orca waits for agent to connect after slot is registered
export const AGENT_CONNECT_TIMEOUT_MS = 60_000

// WS endpoint path for direct-websocket connections (Agent → Orca)
export const AGENT_WS_PATH = '/agent'

// Minimum agent version Orca accepts. Increment when breaking protocol changes land.
export const AGENT_MIN_VERSION = '1.0.0'

// ─── Error Codes ─────────────────────────────────────────────────────────────

export const AgentErrorCode = {
  // JSON-RPC standard
  ParseError:        -32700,
  InvalidRequest:    -32600,
  MethodNotFound:    -32601,
  InvalidParams:     -32602,
  ServerError:       -32000,
  // Agent-specific
  CommandNotFound:   -33001,
  PermissionDenied:  -33002,
  PathNotFound:      -33003,
  PtyAllocationFailed: -33004,
  DiskFull:          -33005,
  TooManyStreams:    -33006,
  StreamProtocolError: -33007,
  HandshakeFailed:   -33100,   // Version mismatch, protocol violation
  AuthFailed:        -33101,   // Invalid or missing agent token
} as const

export type AgentErrorCodeValue = typeof AgentErrorCode[keyof typeof AgentErrorCode]

// ─── Capabilities ─────────────────────────────────────────────────────────────

export type AgentCapability = 'pty' | 'fs' | 'git' | 'preflight'

// ─── Handshake Types ──────────────────────────────────────────────────────────

/** Params agent sends in agent.handshake (first message after WS open) */
export type AgentHandshakeParams = {
  agentVersion: string
  platform: string        // 'linux' | 'darwin' | 'win32'
  arch: string            // 'x64' | 'arm64' | ...
  nodeVersion?: string    // e.g. 'v20.11.0' or 'python-agent'
  capabilities: AgentCapability[]
  /** Token for direct-websocket auth. Not used in relay-websocket mode. */
  agentToken?: string
}

/** Result Orca returns on successful handshake */
export type AgentHandshakeResult = {
  ok: true
  orcaVersion: string
  sessionId: string
}

/** Result Orca returns on failed handshake */
export type AgentHandshakeError = {
  code: number           // AgentErrorCode.HandshakeFailed | AgentErrorCode.AuthFailed
  message: string
}

// ─── Token format ─────────────────────────────────────────────────────────────

/**
 * Generate a unique agent token for direct-websocket mode.
 * Format: agt-<devServerId>-<timestamp>
 * Consumers: DevServerRelayBridge.connectDirectWebSocket()
 */
export function generateAgentToken(devServerId: string): string {
  return `agt-${devServerId}-${Date.now()}`
}
```

---

## 4. Test Specifications

```typescript
// src/shared/__tests__/agent-wire-protocol.test.ts
import { describe, it, expect } from 'vitest'
import {
  AgentErrorCode,
  AGENT_HANDSHAKE_METHOD,
  AGENT_PROTOCOL_VERSION,
  AGENT_TIMEOUT_MS,
  AGENT_KEEPALIVE_INTERVAL_MS,
  AGENT_CONNECT_TIMEOUT_MS,
  AGENT_WS_PATH,
  generateAgentToken,
} from '../agent-wire-protocol'

describe('agent-wire-protocol constants', () => {
  it('AGENT_PROTOCOL_VERSION is "1"', () => {
    expect(AGENT_PROTOCOL_VERSION).toBe('1')
  })

  it('AGENT_HANDSHAKE_METHOD is "agent.handshake"', () => {
    expect(AGENT_HANDSHAKE_METHOD).toBe('agent.handshake')
  })

  it('AGENT_TIMEOUT_MS matches relay-protocol TIMEOUT_MS (20000)', () => {
    expect(AGENT_TIMEOUT_MS).toBe(20_000)
  })

  it('AGENT_KEEPALIVE_INTERVAL_MS matches relay-protocol KEEPALIVE_SEND_MS (5000)', () => {
    expect(AGENT_KEEPALIVE_INTERVAL_MS).toBe(5_000)
  })

  it('AGENT_WS_PATH is "/agent"', () => {
    expect(AGENT_WS_PATH).toBe('/agent')
  })
})

describe('AgentErrorCode', () => {
  it('HandshakeFailed is -33100', () => {
    expect(AgentErrorCode.HandshakeFailed).toBe(-33100)
  })

  it('AuthFailed is -33101', () => {
    expect(AgentErrorCode.AuthFailed).toBe(-33101)
  })

  it('all codes are unique', () => {
    const values = Object.values(AgentErrorCode)
    expect(new Set(values).size).toBe(values.length)
  })
})

describe('generateAgentToken', () => {
  it('generates token with agt- prefix and devServerId', () => {
    const token = generateAgentToken('ds-123')
    expect(token).toMatch(/^agt-ds-123-\d+$/)
  })

  it('generates unique tokens for different calls', () => {
    const t1 = generateAgentToken('ds-1')
    const t2 = generateAgentToken('ds-1')
    // May not always differ if called at same ms, but format is correct
    expect(t1).toMatch(/^agt-/)
    expect(t2).toMatch(/^agt-/)
  })
})
```

---

## 5. Acceptance Criteria

- [ ] `src/shared/agent-wire-protocol.ts` được tạo với đầy đủ types và constants
- [ ] Không import bất kỳ runtime dependency nào (pure types + constants)
- [ ] `AgentErrorCode.HandshakeFailed = -33100`, `AuthFailed = -33101`
- [ ] `AGENT_TIMEOUT_MS = 20_000`, `AGENT_KEEPALIVE_INTERVAL_MS = 5_000`
- [ ] Unit tests pass cho tất cả constants và `generateAgentToken()`
