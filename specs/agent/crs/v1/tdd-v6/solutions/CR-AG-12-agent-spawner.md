# CR-AG-12: ProfileAware Agent Spawner — AI Agent CLI Host

**CR:** CR-AG-12
**TDD:** [TDD-AG-12](../../tdd/v5/12-agent-spawner.md)
**Ngày:** 2026-07-30
**Độ phức tạp:** High — new module, node-pty integration, state machine
**ADR:** ADR-009
**HLD Ref:** C3.9, C3.11, §11 (dev-server-architecture.md)

---

## 1. Phân tích Code Hiện Tại

### Code đã có ✅ — có thể REUSE

| File | Reuse gì | Mức độ |
|------|---------|--------|
| `src/relay/agent-credential-store.ts` | `handleReadCredential()` → lấy `encryptedBlob` | 100% |
| `src/relay/agent-config.ts` | `AgentConfig` interface, `toolPath`, `workDir` | 100% |
| `src/relay/agent-logger.ts` | `AgentLogger` interface | 100% |
| `src/relay/agent-wire.ts` | `encodeDataFrame()`, `WireState` | 100% |
| `src/relay/agent-rpc-dispatch.ts` | Add `agent.spawn`/`agent.kill` cases | 90% |
| `src/shared/agent-wire-protocol.ts` | `AgentErrorCode` constants | 100% |

### Code chưa có ❌ — cần tạo mới

| File | Nội dung |
|------|---------|
| `src/relay/agent-spawner.ts` | ProfileAwareAgentSpawner full module |
| `src/relay/__tests__/agent-spawner.test.ts` | Tests |

### Dependency cần verify

```bash
# Check node-pty có trong package.json chưa:
cat package.json | grep node-pty

# Nếu chưa có:
pnpm add node-pty

# Check build-relay.mjs externals:
grep -n "external" build-relay.mjs
# node-pty phải là external (native addon)
```

---

## 2. Solution — New File: `src/relay/agent-spawner.ts`

### 2.1 Module structure

```typescript
// src/relay/agent-spawner.ts
import * as nodePty from 'node-pty'
import { homedir } from 'node:os'
import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { encodeDataFrame } from './agent-wire'
import type { WireState } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// --- Exports (all spec'd in TDD-AG-12) ---
export { AgentStateMachine }          // State machine (fully testable, pure class)
export { buildAgentEnv }              // Env builder (testable with mock credStore)
export { resolveAgentSpec }           // Model → binary spec resolver (pure, testable)
export { handleAgentSpawn }           // RPC handler
export { handleAgentKill }            // RPC handler
export type { AgentSpawnRequest, AgentLifecycleState, AgentStatusEvent }
```

### 2.2 PTY Registry (trong-process singleton)

```typescript
// Cần PTY registry để agent.kill có thể lookup ptyId:
const PTY_REGISTRY = new Map<string, {
  pty:    nodePty.IPty
  taskId: string
  userId: string
}>()

export function handleAgentKill(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  if (!ptyId) {
    return Promise.resolve({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } })
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    return Promise.resolve({ jsonrpc: '2.0', id, result: { ok: true, note: 'pty not found (already dead)' } })
  }

  entry.pty.kill('SIGTERM')
  PTY_REGISTRY.delete(ptyId)
  log.info(`agent.kill: ptyId=${ptyId} SIGTERM sent`)
  return Promise.resolve({ jsonrpc: '2.0', id, result: { ok: true } })
}
```

### 2.3 AiCredStore integration

```typescript
// REUSE handleReadCredential từ agent-credential-store.ts
// Nhưng cần decrypt layer thứ 2 để lấy plaintext key cho agent env

// Giải pháp: thêm export `readDecryptedKey()` vào agent-credential-store.ts:
export async function readDecryptedKey(
  accountId: string,
  config: AgentConfig,
  log: AgentLogger
): Promise<string | null> {
  // Read → decrypt outer layer → parse inner encryptedBlob
  // NOTE: Inner layer là browser-encrypted — Dev Server chỉ có outer key
  // Plaintext API key = decryptBrowserLayer(encryptedBlob, sessionKey)
  //                   ← sessionKey phải được inject by Admin setup script
  //
  // Cách 1 (đơn giản hơn): store plaintext (pre-decrypted) với outer encrypt
  // Cách 2 (full security): double-layer, cần sessionKey truyền qua
  //
  // v5.0: dùng Cách 1 — outer AES-256-GCM encrypt plaintext key
  // (Browser encrypt chỉ dùng khi transmit qua network)
  const result = await handleReadCredential(null, { accountId }, config, log) as {
    result?: { encryptedBlob: string }; error?: unknown
  }
  if (result.error || !result.result) return null
  // encryptedBlob trong v5.0 = outer-encrypted plaintext API key
  // (không phải browser-encrypted)
  return result.result.encryptedBlob  // đây là plaintext key sau decrypt
}
```

---

## 3. Extend: `src/relay/agent-rpc-dispatch.ts`

Thêm 2 cases vào switch (sau `preflight.check`):

```typescript
// agent.spawn
case 'agent.spawn': {
  try {
    const { handleAgentSpawn } = await import('./agent-spawner')
    // agent.spawn là streaming handler — không return final response ngay
    void handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)
    return { jsonrpc: '2.0', id: rpc.id, result: { type: 'spawn.accepted' } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`)
  }
}

// agent.kill
case 'agent.kill': {
  try {
    const { handleAgentKill } = await import('./agent-spawner')
    return (await handleAgentKill(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.kill unavailable: ${msg}`)
  }
}
```

---

## 4. Extend: `src/relay/agent-session.ts`

Cập nhật capabilities trong handshake (đã plan trong CR-AG-01):

```diff
- capabilities: ['fs', 'git', 'preflight'] as const,
+ capabilities: ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees'] as const,
```

---

## 5. Build Configuration

```javascript
// build-relay.mjs — ensure node-pty là external:
// (tìm đoạn external config)
external: [
  'node-pty',          // ← native addon, không bundle vào agent.js
  'fsevents',
  ...existingExternals
]
```

---

## 6. Tests (`src/relay/__tests__/agent-spawner.test.ts`)

Theo spec trong TDD-AG-12 §10:
- `AgentStateMachine` — 6 tests (pure class, không cần fs/pty)
- `resolveAgentSpec` — 4 tests (pure function)
- `buildAgentEnv` — 4 tests (mock credStore)
- `handleAgentSpawn` — validation tests (không cần real node-pty)
- `handleAgentKill` — 3 tests (mock PTY_REGISTRY)

---

## 7. Implementation Checklist

- [ ] `src/relay/agent-spawner.ts` — tạo file mới (~300 lines)
- [ ] `src/relay/agent-credential-store.ts` — thêm `readDecryptedKey()` export
- [ ] `src/relay/agent-rpc-dispatch.ts` — thêm `agent.spawn`, `agent.kill` cases
- [ ] `src/relay/agent-session.ts` — update capabilities list (CR-AG-01 sync)
- [ ] `build-relay.mjs` — add `node-pty` to externals
- [ ] `package.json` — verify/add `node-pty` dependency
- [ ] `src/relay/__tests__/agent-spawner.test.ts` — tạo test file (≥20 tests)
