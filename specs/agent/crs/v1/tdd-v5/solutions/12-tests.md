# SOL-12: Vitest Test Suite

**TDD Ref:** TDD-AG-01 đến TDD-AG-11 (Test Coverage sections)  
**Files:** `src/relay/__tests__/agent-*.test.ts` [NEW]  
**Mức độ:** 🔴 Phức tạp  
**Thời gian ước tính:** 4h

---

## Test Framework Setup

Agent tests chạy với **Vitest** cùng config `config/vitest.config.ts` — không cần config riêng.

```bash
# Run all agent tests
pnpm test -- --reporter verbose src/relay/__tests__/agent-

# Watch mode
pnpm exec vitest --watch src/relay/__tests__/

# Coverage
pnpm exec vitest run --coverage src/relay/__tests__/
```

---

## 1. agent-wire.test.ts (Target: ≥ 15 tests)

```typescript
// src/relay/__tests__/agent-wire.test.ts
import { describe, it, expect } from 'vitest'
import { createWireState, encodeDataFrame, encodeKeepaliveFrame, decodeFrame, parseJsonPayload, HEADER_SIZE } from '../agent-wire'
// Import MessageType từ relay-protocol (adjust path if needed)

describe('createWireState', () => {
  it('starts at seqCounter=0, highestAck=0', () => {
    const s = createWireState()
    expect(s.seqCounter).toBe(0)
    expect(s.highestAck).toBe(0)
  })
})

describe('encodeDataFrame', () => {
  it('TYPE byte = 0x01 (Regular)', () => { ... })
  it('increments seqCounter each call', () => { ... })
  it('ACK field = state.highestAck', () => { ... })
  it('payload encoded as UTF-8', () => { ... })
  it('total length = HEADER_SIZE + payload.length', () => { ... })
  it('accepts Buffer payload', () => { ... })
})

describe('encodeKeepaliveFrame', () => {
  it('TYPE byte = 0x09 (KeepAlive)', () => { ... })
  it('payload length = 0 (LENGTH field = 0)', () => { ... })
  it('total frame size = HEADER_SIZE (13)', () => { ... })
})

describe('decodeFrame', () => {
  it('updates state.highestAck from incoming seq', () => { ... })
  it('returns null for buffer < HEADER_SIZE', () => { ... })
  it('correctly parses type/seq/ack/length/payload', () => { ... })
  it('does NOT update highestAck if incoming seq < current highestAck', () => { ... })
})

describe('parseJsonPayload', () => {
  it('returns parsed object', () => { ... })
  it('returns null on malformed JSON', () => { ... })
  it('returns null on empty buffer', () => { ... })
})

describe('round-trip', () => {
  it('encodeDataFrame + decodeFrame returns same payload', () => { ... })
  it('multiple connections have independent state', () => { ... })
})
```

---

## 2. agent-config.test.ts (Target: ≥ 10 tests)

```typescript
// src/relay/__tests__/agent-config.test.ts
import { describe, it, expect, vi, afterEach } from 'vitest'
import { loadAgentConfig } from '../agent-config'

describe('loadAgentConfig', () => {
  afterEach(() => vi.unstubAllEnvs())

  it('defaults to direct-websocket', () => { ... })
  it('loads relay-websocket', () => { ... })
  it('throws on invalid MODE', () => { ... })
  it('AGENT_PORT parsed as number', () => { ... })
  it('workDir defaults to process.cwd() when AGENT_WORK_DIR empty', () => { ... })
  it('toolEnv.PATH contains ~/.local/bin', () => { ... })
  it('tlsRejectUnauthorized=false when NODE_TLS_REJECT_UNAUTHORIZED=0', () => { ... })
  it('tlsRejectUnauthorized=true by default', () => { ... })
  it('credentialDir = ~/.orca/credentials', () => { ... })
  it('agentToken empty string when not set', () => { ... })
})
```

---

## 3. agent-tool-registry.test.ts (Target: ≥ 20 tests)

```typescript
// src/relay/__tests__/agent-tool-registry.test.ts
import { describe, it, expect, vi } from 'vitest'
import { discoverTools, ALL_TOOL_DEFINITIONS } from '../agent-tool-registry'

// Mock fs.accessSync to control binary discovery
const mockConfig = {
  toolPath: '/usr/bin:/usr/local/bin',
  workDir: '/tmp',
  toolEnv: { PATH: '/usr/bin' },
  // ...
}

describe('discoverTools', () => {
  it('includes built-in tools (binary=null) always', async () => { ... })
  it('excludes tool when binary not found in toolPath', async () => { ... })
  it('includes tool when binary found', async () => { ... })
  it('reads_file handler: returns correct lines [start, end]', async () => { ... })
  it('read_file handler: file not found → exitCode=1', async () => { ... })
  it('list_dir handler: returns entries sorted', async () => { ... })
  it('shell handler: timeout capped at 600_000ms', async () => { ... })
  it('all tools have required fields: name, binary, description, inputSchema', () => {
    for (const tool of ALL_TOOL_DEFINITIONS) {
      expect(tool.name).toBeTruthy()
      expect(tool.description).toBeTruthy()
      expect(tool.inputSchema.type).toBe('object')
    }
  })
  it('no duplicate tool names', () => {
    const names = ALL_TOOL_DEFINITIONS.map(t => t.name)
    expect(new Set(names).size).toBe(names.length)
  })
  // ... handlers for gh, git, shell, gitnexus
})
```

---

## 4. agent-rpc-dispatch.test.ts (Target: ≥ 20 tests)

```typescript
// src/relay/__tests__/agent-rpc-dispatch.test.ts

// Mock WebSocket
class MockWs {
  readyState = 1
  send = vi.fn()
}

const mockTools: ToolDefinition[] = [{
  name: 'echo_tool', binary: null,
  description: 'Test tool',
  inputSchema: { type: 'object', properties: { msg: { type: 'string', description: '' } }, required: ['msg'] },
  async handler(params) {
    return { stdout: String(params.msg), stderr: '', exitCode: 0 }
  },
}]

describe('createRpcDispatcher', () => {
  it('tools/list: returns all tools', async () => { ... })
  it('tools/call: found → handler called', async () => { ... })
  it('tools/call: unknown → MethodNotFound (-32601)', async () => { ... })
  it('tools/call: handler throws → ServerError (-32000)', async () => { ... })
  it('unknown method → MethodNotFound', async () => { ... })
  it('response sent via ws.send', async () => { ... })
  it('MCP format: content[0].type="text"', async () => { ... })
  it('MCP format: isError=false when exitCode=0', async () => { ... })
  it('MCP format: isError=true when exitCode≠0', async () => { ... })
  it('MCP format: stderr prepended with [stderr]', async () => { ... })
  it('ws.readyState≠OPEN → no send', async () => { ... })
})
```

---

## 5. agent-session.test.ts (Target: ≥ 15 tests)

```typescript
// src/relay/__tests__/agent-session.test.ts
// See SOL-06 for mock WebSocket pattern

describe('createSession', () => {
  it('sends handshake frame on start() when ws OPEN', () => { ... })
  it('sends handshake frame on ws.open event', () => { ... })
  it('starts keepalive interval on start()', () => { ... })
  it('sends keepalive frame on interval', () => { ... })
  it('stops keepalive interval on stop()', () => { ... })
  it('responds to KeepAlive frames', () => { ... })
  it('processes handshake result.ok=true → handshakeDone', () => { ... })
  it('closes ws on handshake error', () => { ... })
  it('dispatches RPC post-handshake', () => { ... })
  it('ignores non-Buffer messages', () => { ... })
  it('ignores malformed JSON frames', () => { ... })
  it('onHandshakeOk callback fires', () => { ... })
  it('handshake NOT dispatched post-handshake (id=1 reply discarded)', () => { ... })
})
```

---

## 6. git-handler.test.ts (Target: ≥ 20 tests)

```typescript
describe('validateGitArgs', () => {
  // 10 tests: each allowed subcommand, each disallowed, each metacharacter
})

describe('handleGitExec', () => {
  it('calls spawn with correct binary and args', () => { ... })
  it('returns stdout + stderr + exitCode', () => { ... })
  it('invalid args → error response (no crash)', () => { ... })
  it('timeout → error response', () => { ... })
})

describe('handleGitExecStream', () => {
  it('sends stream.chunk for each stdout line', () => { ... })
  it('sends stream.chunk with source=stderr for stderr lines', () => { ... })
  it('sends stream.end on close', () => { ... })
  it('invalid args → error frame immediately', () => { ... })
  it('ws closed mid-stream → no more sends', () => { ... })
})
```

---

## 7. agent-credential-store.test.ts (Target: ≥ 15 tests)

```typescript
// Use real tmpdir + real crypto (no mocks for crypto)
describe('credential store', () => {
  it('writeCredential + readCredential round-trip', async () => { ... })
  it('writeCredential creates file with mode 0600', async () => { ... })
  it('writeCredential: missing ORCA_AI_CREDENTIAL_KEY → error', async () => { ... })
  it('readCredential: wrong master key → decryption error', async () => { ... })
  it('readCredential: file not found → PathNotFound', async () => { ... })
  it('invalid accountId "../evil" → InvalidParams', async () => { ... })
  it('healthCheck: readable credential → ok=true', async () => { ... })
  // ... more edge cases
})
```

---

## Test Target Summary

| File | Tests | Target |
|------|-------|--------|
| agent-wire.test.ts | 15+ | ≥ 15 |
| agent-config.test.ts | 10+ | ≥ 10 |
| agent-tool-registry.test.ts | 20+ | ≥ 20 |
| agent-rpc-dispatch.test.ts | 20+ | ≥ 20 |
| agent-session.test.ts | 15+ | ≥ 15 |
| agent-connection-direct.test.ts | 8+ | ≥ 8 |
| agent-connection-relay.test.ts | 8+ | ≥ 8 |
| git-handler.test.ts | 20+ | ≥ 20 |
| agent-credential-store.test.ts | 15+ | ≥ 15 |
| fs-agent-extensions.test.ts | 20+ | ≥ 20 |
| **Total** | **≥ 151** | 🎯 |

---

## Definition of Done

- [x] Tất cả test files created
- [x] `pnpm test -- src/relay/__tests__/agent-` → 100% pass
- [x] Không có `any` type cast tắt TypeScript check trong test files
- [x] Không mock `node:crypto` — test crypto functions với real implementation
- [x] Coverage report: ≥ 80% branches cho agent modules
