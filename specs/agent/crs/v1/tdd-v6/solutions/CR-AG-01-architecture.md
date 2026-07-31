# CR-AG-01: Architecture & Process Model — Entry Point Update

**CR:** CR-AG-01
**TDD:** [TDD-AG-01](../../tdd/v5/01-architecture.md)
**Ngày:** 2026-07-30
**Độ phức tạp:** Low — chỉ update metadata trong handshake

---

## 1. Phân tích Code Hiện Tại

### Code đã có ✅

[`src/relay/agent-entry.ts`](../../../../../src/relay/agent-entry.ts) — Entry point đầy đủ:
- `loadAgentConfig()` → typed config từ env vars
- `discoverTools()` → auto-detect binaries
- `createSession()` → handshake + keepalive + RPC dispatch
- Kết nối theo `config.mode` (direct-ws / relay-ws)

[`src/relay/agent-session.ts`](../../../../../src/relay/agent-session.ts) — Session management:
- Handshake frame gửi ngay khi open
- Keepalive interval (`AGENT_KEEPALIVE_INTERVAL_MS`)
- RPC dispatch gate behind handshake

### Gap so với TDD-AG-01

```diff
// agent-session.ts line 62-68 — capabilities list cần update:
- capabilities: ['fs', 'git', 'preflight'] as const,
+ capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'worktrees'] as const,

// agentVersion cần update:
- agentVersion: '2.1.0',
+ agentVersion: '5.0.0',
```

---

## 2. Solution

### 2.1 MODIFY: `src/relay/agent-session.ts`

**Thay đổi tối thiểu** — chỉ update 2 dòng trong `sendHandshake()`:

```typescript
// Before (line 60-68):
const rpc = {
  jsonrpc: '2.0' as const,
  id: 1,
  method: AGENT_HANDSHAKE_METHOD,
  params: {
    agentVersion:  '2.1.0',                                    // ← UPDATE
    platform:      process.platform,
    arch:          process.arch,
    nodeVersion:   process.version,
    capabilities:  ['fs', 'git', 'preflight'] as const,        // ← UPDATE
    ...(config.agentToken ? { agentToken: config.agentToken } : {}),
    devServerId:   config.devServerId,
    tools:         tools.map(t => t.name),
  },
}

// After:
const rpc = {
  jsonrpc: '2.0' as const,
  id: 1,
  method: AGENT_HANDSHAKE_METHOD,
  params: {
    agentVersion:  '5.0.0',                                    // ← UPDATED
    platform:      process.platform,
    arch:          process.arch,
    nodeVersion:   process.version,
    capabilities:  ['fs', 'git', 'preflight', 'ai.providers', 'worktrees'] as const, // ← UPDATED
    ...(config.agentToken ? { agentToken: config.agentToken } : {}),
    devServerId:   config.devServerId,
    tools:         tools.map(t => t.name),
  },
}
```

### 2.2 MODIFY: `src/relay/agent-config.ts`

Thêm `credentialDir` đã có ✅ — không cần thay đổi.

---

## 3. Tests

Existing tests trong `src/relay/__tests__/` cover handshake flow.
Chỉ cần update snapshot assertions cho `agentVersion` và `capabilities`:

```typescript
// src/relay/__tests__/agent-session.test.ts (update existing)
it('sends handshake with v5.0 capabilities', () => {
  const parsed = JSON.parse(capturedFrames[0])
  expect(parsed.params.agentVersion).toBe('5.0.0')
  expect(parsed.params.capabilities).toContain('ai.providers')
  expect(parsed.params.capabilities).toContain('worktrees')
})
```

---

## 4. Implementation Checklist

- [ ] `src/relay/agent-session.ts` — update `agentVersion: '5.0.0'`
- [ ] `src/relay/agent-session.ts` — update `capabilities` array thêm `'ai.providers'`, `'worktrees'`
- [ ] Test snapshot update
