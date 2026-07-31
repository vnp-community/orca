# TASK-15: Write Vitest Tests — agent-session + agent-connection-*

**Phase:** 6  
**SOL Ref:** SOL-12  
**Estimated time:** 2h  
**Precondition:** TASK-07 (session) và TASK-08 (connections) hoàn thành  

---

## File 1: `src/relay/__tests__/agent-session.test.ts`

**Target: ≥ 15 tests**

**Pattern: MockWs** — EventEmitter + send/close mocks:

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { createSession } from '../agent-session'
import { HEADER_SIZE } from '../agent-wire'
import type { AgentConfig } from '../agent-config'
import type { ToolDefinition } from '../agent-tool-registry'
import type { AgentLogger } from '../agent-logger'

class MockWs extends EventEmitter {
  readyState = 1  // OPEN
  send = vi.fn()
  close = vi.fn()
}

// Mock WebSocket module
vi.mock('ws', () => ({ default: MockWs, WebSocket: MockWs }))

const mockConfig: AgentConfig = {
  mode: 'direct-websocket', orcaUrl: '', agentToken: 'tok-test',
  agentPort: 6799, devServerId: 'test', logLevel: 'info',
  workDir: '/tmp', toolPath: '/usr/bin', toolEnv: {},
  credentialDir: '/tmp/.creds', tlsRejectUnauthorized: true,
}

const mockTool: ToolDefinition = {
  name: 't1', binary: null, description: 'd',
  inputSchema: { type: 'object', properties: {} },
  async handler() { return { stdout: '', stderr: '', exitCode: 0 } },
}

const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

function extractJson(ws: MockWs, callIndex = 0): Record<string, unknown> {
  const buf = ws.send.mock.calls[callIndex][0] as Buffer
  return JSON.parse(buf.subarray(HEADER_SIZE).toString('utf8'))
}

describe('createSession().start()', () => {
  it('sends handshake immediately when ws is already OPEN (readyState=1)', () => {
    const ws = new MockWs()
    const session = createSession(mockConfig, [mockTool], mockLog)
    session.start(ws)
    expect(ws.send).toHaveBeenCalledOnce()
    const rpc = extractJson(ws, 0)
    expect(rpc.method).toBe('agent.handshake')
  })

  it('handshake params include devServerId and tools list', () => {
    const ws = new MockWs()
    createSession(mockConfig, [mockTool], mockLog).start(ws)
    const rpc = extractJson(ws, 0)
    expect((rpc.params as any).devServerId).toBe('test')
    expect((rpc.params as any).tools).toContain('t1')
  })

  it('handshake params include agentToken when config.agentToken non-empty', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws)
    const rpc = extractJson(ws, 0)
    expect((rpc.params as any).agentToken).toBe('tok-test')
  })

  it('handshake does NOT include agentToken when empty', () => {
    const ws = new MockWs()
    const cfg = { ...mockConfig, agentToken: '' }
    createSession(cfg, [], mockLog).start(ws)
    const rpc = extractJson(ws, 0)
    expect((rpc.params as any).agentToken).toBeUndefined()
  })
})

describe('keepalive', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('sends keepalive frame every 5000ms', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws)
    ws.send.mockClear()  // clear handshake call
    vi.advanceTimersByTime(5001)
    expect(ws.send).toHaveBeenCalledOnce()
    // KeepAlive frame: type=9, length=0, total=13 bytes
    const frame = ws.send.mock.calls[0][0] as Buffer
    expect(frame.length).toBe(13)
    expect(frame.readUInt8(0)).toBe(9)
  })

  it('stop() clears keepalive interval', () => {
    const ws = new MockWs()
    const session = createSession(mockConfig, [], mockLog)
    session.start(ws)
    session.stop()
    ws.send.mockClear()
    vi.advanceTimersByTime(10000)
    expect(ws.send).not.toHaveBeenCalled()
  })
})

describe('message handling', () => {
  it('fires onHandshakeOk callback on result.ok=true', () => {
    const ws = new MockWs()
    const session = createSession(mockConfig, [], mockLog)
    const cb = vi.fn()
    session.onHandshakeOk(cb)
    session.start(ws)

    // Simulate handshake response
    const response = { jsonrpc: '2.0', id: 1, result: { ok: true, sessionId: 'sess-1', orcaVersion: '1.0' } }
    const header = Buffer.allocUnsafe(13)
    const payload = Buffer.from(JSON.stringify(response))
    header.writeUInt8(1, 0); header.writeUInt32BE(1, 1); header.writeUInt32BE(0, 5); header.writeUInt32BE(payload.length, 9)
    ws.emit('message', Buffer.concat([header, payload]))

    expect(cb).toHaveBeenCalledOnce()
  })

  it('closes ws on handshake error', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws)

    const response = { jsonrpc: '2.0', id: 1, error: { code: -33101, message: 'AuthFailed' } }
    const header = Buffer.allocUnsafe(13)
    const payload = Buffer.from(JSON.stringify(response))
    header.writeUInt8(1, 0); header.writeUInt32BE(1, 1); header.writeUInt32BE(0, 5); header.writeUInt32BE(payload.length, 9)
    ws.emit('message', Buffer.concat([header, payload]))

    expect(ws.close).toHaveBeenCalledWith(1008, 'Handshake failed')
  })

  it('responds to KeepAlive frames with keepalive frame', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws)
    ws.send.mockClear()

    const kaFrame = Buffer.allocUnsafe(13)
    kaFrame.writeUInt8(9, 0); kaFrame.writeUInt32BE(5, 1); kaFrame.writeUInt32BE(0, 5); kaFrame.writeUInt32BE(0, 9)
    ws.emit('message', kaFrame)

    expect(ws.send).toHaveBeenCalledOnce()
    const resp = ws.send.mock.calls[0][0] as Buffer
    expect(resp.readUInt8(0)).toBe(9)  // KeepAlive type
  })

  it('ignores non-Buffer messages', () => {
    const ws = new MockWs()
    createSession(mockConfig, [], mockLog).start(ws)
    ws.send.mockClear()
    ws.emit('message', 'text message')
    expect(ws.send).not.toHaveBeenCalled()
  })
})
```

---

## File 2: `src/relay/__tests__/agent-connection-relay.test.ts`

**Target: ≥ 8 tests**

```typescript
// Test authenticate() logic in isolation
// Key tests:
// - correct token in ?token= querystring → accepted
// - correct token in Authorization: Bearer header → accepted
// - wrong token → ws.close(1008)
// - empty token → rejected
// - creates separate session per connection

import { describe, it, expect, vi } from 'vitest'
// Import authenticate if exported, or test via full listenRelay() integration
```

---

## Run Tests

```bash
pnpm test -- src/relay/__tests__/agent-session.test.ts src/relay/__tests__/agent-connection-relay.test.ts
```

## Definition of Done

- [x] `agent-session.test.ts` — ≥ 15 tests pass
- [x] `agent-connection-relay.test.ts` — ≥ 8 tests pass (focus on authenticate logic)
- [x] Fake timers used for keepalive tests (no real 5s waits)
