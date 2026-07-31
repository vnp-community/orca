# TASK-002: Tạo `src/shared/__tests__/agent-wire-protocol.test.ts`

> **Status:** ✅ DONE (2026-07-26)
> **File created:** `src/shared/__tests__/agent-wire-protocol.test.ts`
> **Tests:** 15/15 pass
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 1 — Protocol Foundation  
**Solution:** [SOL-AG-001](../solutions/SOL-AG-001-wire-protocol.md) §4  
**Depends on:** TASK-001  
**Blocks:** (không có)  

---

## Mục tiêu

Tạo unit tests cho `agent-wire-protocol.ts`. Tests verify constants có đúng giá trị
và `generateAgentToken()` hoạt động đúng format.

---

## File cần tạo

**Path:** `src/shared/__tests__/agent-wire-protocol.test.ts`

---

## Nội dung

```typescript
// src/shared/__tests__/agent-wire-protocol.test.ts
import { describe, it, expect } from 'vitest'
import {
  AGENT_PROTOCOL_VERSION,
  AGENT_HANDSHAKE_METHOD,
  AGENT_TIMEOUT_MS,
  AGENT_KEEPALIVE_INTERVAL_MS,
  AGENT_CONNECT_TIMEOUT_MS,
  AGENT_WS_PATH,
  AGENT_MIN_VERSION,
  AgentErrorCode,
  generateAgentToken,
} from '../agent-wire-protocol'

describe('agent-wire-protocol constants', () => {
  it('AGENT_PROTOCOL_VERSION is "1"', () => {
    expect(AGENT_PROTOCOL_VERSION).toBe('1')
  })

  it('AGENT_HANDSHAKE_METHOD is "agent.handshake"', () => {
    expect(AGENT_HANDSHAKE_METHOD).toBe('agent.handshake')
  })

  it('AGENT_TIMEOUT_MS is 20000 (matches relay-protocol TIMEOUT_MS)', () => {
    expect(AGENT_TIMEOUT_MS).toBe(20_000)
  })

  it('AGENT_KEEPALIVE_INTERVAL_MS is 5000 (matches relay-protocol KEEPALIVE_SEND_MS)', () => {
    expect(AGENT_KEEPALIVE_INTERVAL_MS).toBe(5_000)
  })

  it('AGENT_CONNECT_TIMEOUT_MS is 60000', () => {
    expect(AGENT_CONNECT_TIMEOUT_MS).toBe(60_000)
  })

  it('AGENT_WS_PATH is "/agent"', () => {
    expect(AGENT_WS_PATH).toBe('/agent')
  })

  it('AGENT_MIN_VERSION is "1.0.0"', () => {
    expect(AGENT_MIN_VERSION).toBe('1.0.0')
  })
})

describe('AgentErrorCode', () => {
  it('HandshakeFailed is -33100', () => {
    expect(AgentErrorCode.HandshakeFailed).toBe(-33100)
  })

  it('AuthFailed is -33101', () => {
    expect(AgentErrorCode.AuthFailed).toBe(-33101)
  })

  it('all error codes are unique', () => {
    const values = Object.values(AgentErrorCode)
    const unique = new Set(values)
    expect(unique.size).toBe(values.length)
  })

  it('standard JSON-RPC codes are correct', () => {
    expect(AgentErrorCode.ParseError).toBe(-32700)
    expect(AgentErrorCode.InvalidRequest).toBe(-32600)
    expect(AgentErrorCode.MethodNotFound).toBe(-32601)
    expect(AgentErrorCode.InvalidParams).toBe(-32602)
    expect(AgentErrorCode.ServerError).toBe(-32000)
  })
})

describe('generateAgentToken', () => {
  it('returns string with agt- prefix', () => {
    const token = generateAgentToken('ds-abc')
    expect(token).toMatch(/^agt-/)
  })

  it('includes devServerId in token', () => {
    const token = generateAgentToken('ds-xyz-123')
    expect(token).toContain('ds-xyz-123')
  })

  it('matches format agt-<devServerId>-<number>', () => {
    const token = generateAgentToken('my-server')
    expect(token).toMatch(/^agt-my-server-\d+$/)
  })

  it('tokens for same id differ over time (or at minimum are valid format)', () => {
    const t1 = generateAgentToken('ds-1')
    const t2 = generateAgentToken('ds-1')
    // Both should be valid format
    expect(t1).toMatch(/^agt-ds-1-\d+$/)
    expect(t2).toMatch(/^agt-ds-1-\d+$/)
  })
})
```

---

## Cách chạy test

```bash
pnpm vitest run src/shared/__tests__/agent-wire-protocol.test.ts
```

## Acceptance Criteria

- [x] File test tồn tại
- [x] Tất cả tests pass (15 test cases)
- [x] Không có TypeScript errors
