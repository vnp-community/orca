# TASK-13: Write Vitest Tests — agent-wire + agent-config

**Phase:** 6  
**SOL Ref:** SOL-12  
**Estimated time:** 1.5h  
**Precondition:** TASK-03 (agent-config) và TASK-04 (agent-wire) hoàn thành và typecheck passes  

---

## File 1: `src/relay/__tests__/agent-wire.test.ts`

**Target: ≥ 15 tests**

```typescript
import { describe, it, expect } from 'vitest'
import {
  createWireState, encodeDataFrame, encodeKeepaliveFrame,
  decodeFrame, parseJsonPayload, HEADER_SIZE
} from '../agent-wire'

// MessageType values (from relay-protocol):
// Regular = 1, KeepAlive = 9

describe('createWireState', () => {
  it('initializes seqCounter=0 and highestAck=0', () => {
    const s = createWireState()
    expect(s.seqCounter).toBe(0)
    expect(s.highestAck).toBe(0)
  })
})

describe('encodeDataFrame', () => {
  it('sets TYPE byte to Regular (1)', () => {
    const s = createWireState()
    const f = encodeDataFrame(s, 'hello')
    expect(f.readUInt8(0)).toBe(1)
  })

  it('increments seqCounter on each call', () => {
    const s = createWireState()
    encodeDataFrame(s, 'a')
    encodeDataFrame(s, 'b')
    expect(s.seqCounter).toBe(2)
  })

  it('SEQ field = seqCounter after increment', () => {
    const s = createWireState()
    const f = encodeDataFrame(s, 'x')
    expect(f.readUInt32BE(1)).toBe(1)
  })

  it('ACK field = state.highestAck', () => {
    const s = createWireState()
    s.highestAck = 42
    const f = encodeDataFrame(s, 'x')
    expect(f.readUInt32BE(5)).toBe(42)
  })

  it('LENGTH field = payload byte length', () => {
    const s = createWireState()
    const payload = 'hello'
    const f = encodeDataFrame(s, payload)
    expect(f.readUInt32BE(9)).toBe(Buffer.byteLength(payload, 'utf8'))
  })

  it('total frame size = HEADER_SIZE + payload length', () => {
    const s = createWireState()
    const payload = 'test'
    const f = encodeDataFrame(s, payload)
    expect(f.length).toBe(HEADER_SIZE + Buffer.byteLength(payload))
  })

  it('accepts Buffer payload', () => {
    const s = createWireState()
    const f = encodeDataFrame(s, Buffer.from('buf'))
    expect(f.readUInt32BE(9)).toBe(3)
  })
})

describe('encodeKeepaliveFrame', () => {
  it('sets TYPE byte to KeepAlive (9)', () => {
    const s = createWireState()
    const f = encodeKeepaliveFrame(s)
    expect(f.readUInt8(0)).toBe(9)
  })

  it('LENGTH field = 0', () => {
    const s = createWireState()
    const f = encodeKeepaliveFrame(s)
    expect(f.readUInt32BE(9)).toBe(0)
  })

  it('total frame size = HEADER_SIZE (13)', () => {
    const s = createWireState()
    const f = encodeKeepaliveFrame(s)
    expect(f.length).toBe(HEADER_SIZE)
  })
})

describe('decodeFrame', () => {
  it('returns null for buffer shorter than HEADER_SIZE', () => {
    const s = createWireState()
    expect(decodeFrame(s, Buffer.alloc(5))).toBeNull()
  })

  it('updates state.highestAck from incoming seq', () => {
    const s = createWireState()
    const incoming = Buffer.allocUnsafe(HEADER_SIZE)
    incoming.writeUInt8(1, 0)
    incoming.writeUInt32BE(7, 1)   // seq=7
    incoming.writeUInt32BE(0, 5)
    incoming.writeUInt32BE(0, 9)
    decodeFrame(s, incoming)
    expect(s.highestAck).toBe(7)
  })

  it('does NOT lower highestAck if incoming seq < current', () => {
    const s = createWireState()
    s.highestAck = 10
    const incoming = Buffer.allocUnsafe(HEADER_SIZE)
    incoming.writeUInt8(1, 0)
    incoming.writeUInt32BE(3, 1)   // seq=3 < highestAck=10
    incoming.writeUInt32BE(0, 5)
    incoming.writeUInt32BE(0, 9)
    decodeFrame(s, incoming)
    expect(s.highestAck).toBe(10)  // unchanged
  })
})

describe('parseJsonPayload', () => {
  it('returns parsed object', () => {
    const buf = Buffer.from('{"ok":true}')
    expect(parseJsonPayload(buf)).toEqual({ ok: true })
  })

  it('returns null on malformed JSON', () => {
    expect(parseJsonPayload(Buffer.from('{bad}'))).toBeNull()
  })

  it('returns null on empty buffer', () => {
    expect(parseJsonPayload(Buffer.alloc(0))).toBeNull()
  })
})

describe('round-trip', () => {
  it('encode then decode returns same payload', () => {
    const s1 = createWireState()
    const s2 = createWireState()
    const payload = JSON.stringify({ method: 'test', id: 42 })
    const frame = encodeDataFrame(s1, payload)
    const decoded = decodeFrame(s2, frame)
    expect(decoded).not.toBeNull()
    expect(decoded!.payload.toString('utf8')).toBe(payload)
  })

  it('two connections have independent WireState', () => {
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

---

## File 2: `src/relay/__tests__/agent-config.test.ts`

**Target: ≥ 10 tests**

```typescript
import { describe, it, expect, vi, afterEach } from 'vitest'
import { loadAgentConfig } from '../agent-config'

describe('loadAgentConfig', () => {
  afterEach(() => vi.unstubAllEnvs())

  it('defaults to direct-websocket when MODE not set', () => {
    vi.stubEnv('MODE', '')
    expect(loadAgentConfig().mode).toBe('direct-websocket')
  })

  it('loads relay-websocket when MODE=relay-websocket', () => {
    vi.stubEnv('MODE', 'relay-websocket')
    expect(loadAgentConfig().mode).toBe('relay-websocket')
  })

  it('throws on invalid MODE', () => {
    vi.stubEnv('MODE', 'ssh-tunnel')
    expect(() => loadAgentConfig()).toThrow('Invalid MODE')
  })

  it('parses AGENT_PORT as integer', () => {
    vi.stubEnv('AGENT_PORT', '7799')
    expect(loadAgentConfig().agentPort).toBe(7799)
  })

  it('workDir defaults to process.cwd() when AGENT_WORK_DIR empty', () => {
    vi.stubEnv('AGENT_WORK_DIR', '')
    expect(loadAgentConfig().workDir).toBe(process.cwd())
  })

  it('toolEnv.PATH contains ~/.local/bin', () => {
    expect(loadAgentConfig().toolEnv.PATH).toContain('.local/bin')
  })

  it('toolEnv.ANTHROPIC_API_KEY set from env', () => {
    vi.stubEnv('ANTHROPIC_API_KEY', 'sk-test')
    expect(loadAgentConfig().toolEnv.ANTHROPIC_API_KEY).toBe('sk-test')
  })

  it('tlsRejectUnauthorized=false when NODE_TLS_REJECT_UNAUTHORIZED=0', () => {
    vi.stubEnv('NODE_TLS_REJECT_UNAUTHORIZED', '0')
    expect(loadAgentConfig().tlsRejectUnauthorized).toBe(false)
  })

  it('tlsRejectUnauthorized=true by default', () => {
    vi.stubEnv('NODE_TLS_REJECT_UNAUTHORIZED', '')
    expect(loadAgentConfig().tlsRejectUnauthorized).toBe(true)
  })

  it('credentialDir ends with .orca/credentials', () => {
    expect(loadAgentConfig().credentialDir).toContain('.orca/credentials')
  })
})
```

---

## Run Tests

```bash
pnpm test -- src/relay/__tests__/agent-wire.test.ts src/relay/__tests__/agent-config.test.ts
```

## Definition of Done

- [x] `src/relay/__tests__/agent-wire.test.ts` created — ≥ 15 tests pass
- [x] `src/relay/__tests__/agent-config.test.ts` created — ≥ 10 tests pass
- [x] No test uses `any` to bypass TypeScript
- [x] `pnpm test` passes with 0 failures
