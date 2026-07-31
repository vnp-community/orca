# CR-AG-09: AI Credential Store — Extend với deleteCredential + healthCheck v2

**CR:** CR-AG-09
**TDD:** [TDD-AG-09](../../tdd/v5/09-ai-credential-relay.md)
**Ngày:** 2026-07-30
**Độ phức tạp:** Medium — extend file hiện tại
**ADR:** ADR-008
**HLD Ref:** C3.11a

---

## 1. Phân tích Code Hiện Tại

### Code đã có ✅ — [`src/relay/agent-credential-store.ts`](../../../../../src/relay/agent-credential-store.ts)

| Function | Trạng thái | Ghi chú |
|---------|-----------|---------|
| `handleWriteCredential()` | ✅ DONE | double-layer AES-256-GCM, scrypt key, chmod 0600 |
| `handleReadCredential()` | ✅ DONE | decrypt + return `encryptedBlob` |
| `handleHealthCheck()` | ⚠️ PARTIAL | chỉ verify credential readable, chưa test actual API |
| `handleDeleteCredential()` | ❌ MISSING | TDD yêu cầu |

### Code đã có ✅ — `agent-rpc-dispatch.ts`

Routes cho `ai.provider.writeCredential`, `ai.provider.readCredential`, `ai.provider.healthCheck` đã có.

### Gap so với TDD-AG-09

1. `ai.provider.deleteCredential` — chưa có handler
2. `ai.provider.healthCheck` — TODO comment `v5.1: make actual API call` chưa implement
3. `agent-rpc-dispatch.ts` — chưa route `ai.provider.deleteCredential`

---

## 2. Solution

### 2.1 EXTEND: `src/relay/agent-credential-store.ts`

Thêm `handleDeleteCredential()` vào cuối file (không sửa code hiện tại):

```typescript
// src/relay/agent-credential-store.ts — APPEND sau dòng 207

import { unlinkSync } from 'node:fs'    // ← thêm vào import block ở đầu file

// ─── ai.provider.deleteCredential ─────────────────────────────────────────────

export async function handleDeleteCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  if (!accountId) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: accountId' } }
  }

  try {
    const filePath = credentialFilePath(config.credentialDir, accountId)

    if (!existsSync(filePath)) {
      // Idempotent delete — nếu không tồn tại vẫn return ok
      log.info(`ai.provider.deleteCredential: not found (idempotent ok) accountId=${accountId}`)
      return { jsonrpc: '2.0', id, result: { ok: true, deleted: false } }
    }

    unlinkSync(filePath)
    log.info(`ai.provider.deleteCredential: deleted accountId=${accountId}`)
    return { jsonrpc: '2.0', id, result: { ok: true, deleted: true } }

  } catch (err: unknown) {
    log.error(`ai.provider.deleteCredential failed: ${err instanceof Error ? err.message : String(err)}`)
    return errorResponse(id, err)
  }
}
```

### 2.2 EXTEND: `src/relay/agent-credential-store.ts` — healthCheck v2

Thay thế TODO trong `handleHealthCheck()`:

```typescript
// Before (hiện tại — lines 196-206):
// TODO v5.1: Make an actual API call to verify the decrypted key is valid
log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} → credential_readable`)
return {
  jsonrpc: '2.0', id,
  result: { ok: true, latencyMs: Date.now() - start, note: 'credential_readable' },
}

// After:
// Verify credential readable trước
const credResult = await handleReadCredential(id, { accountId }, config, log) as { error?: unknown; result?: { encryptedBlob: string; iv: string; algorithm: string } }
if (credResult.error) return credResult

// v5.0: validate provider reachability với timeout 5s
const note = await checkProviderReachability(provider)

log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} → ${note}`)
return {
  jsonrpc: '2.0', id,
  result: { ok: note === 'reachable', latencyMs: Date.now() - start, note },
}
```

**Helper `checkProviderReachability()`:**

```typescript
// src/relay/agent-credential-store.ts — thêm helper

const PROVIDER_HEALTH_URLS: Record<string, string> = {
  anthropic: 'https://api.anthropic.com',
  openai:    'https://api.openai.com',
  gemini:    'https://generativelanguage.googleapis.com',
  // Ollama và vLLM: check localhost port
}

async function checkProviderReachability(provider: string): Promise<string> {
  const url = PROVIDER_HEALTH_URLS[provider]
  if (!url) return 'unknown_provider'   // local providers (Ollama/vLLM) không check

  try {
    // import fetch only if available (Node ≥ 18)
    const { default: fetch } = await import('node-fetch').catch(() => ({ default: globalThis.fetch }))
    const ctrl = new AbortController()
    const timer = setTimeout(() => ctrl.abort(), 5_000)
    const resp = await fetch(url, { method: 'HEAD', signal: ctrl.signal })
    clearTimeout(timer)
    // Any HTTP response (even 401/403) means server is reachable
    return resp.status < 500 ? 'reachable' : 'server_error'
  } catch {
    return 'unreachable'
  }
}
```

### 2.3 EXTEND: `src/relay/agent-rpc-dispatch.ts`

Thêm route cho `ai.provider.deleteCredential` sau dòng 213:

```typescript
// Thêm sau 'ai.provider.healthCheck' case (sau line 213):

// ── v5.0: ai.provider.deleteCredential ───────────────────────────────────
case 'ai.provider.deleteCredential': {
  try {
    const { handleDeleteCredential } = await import('./agent-credential-store')
    return (await handleDeleteCredential(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.deleteCredential unavailable: ${msg}`)
  }
}
```

---

## 3. Tests

Tạo `src/relay/__tests__/agent-credential-store.test.ts`:

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mkdirSync, writeFileSync, existsSync, unlinkSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import {
  handleWriteCredential,
  handleReadCredential,
  handleDeleteCredential,
  handleHealthCheck,
} from '../agent-credential-store'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

const mockLog: AgentLogger = { info: vi.fn(), error: vi.fn(), debug: vi.fn(), warn: vi.fn() }

function makeConfig(dir: string): AgentConfig {
  return {
    mode: 'direct-websocket', orcaUrl: '', agentToken: 'tok', agentPort: 6799,
    devServerId: 'test', logLevel: 'info',
    workDir: dir, toolPath: '/usr/bin', toolEnv: process.env,
    credentialDir: dir, tlsRejectUnauthorized: true,
  }
}

describe('handleWriteCredential', () => {
  let dir: string
  beforeEach(() => {
    dir = join(tmpdir(), `orca-cred-test-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'test-master-key-32byteslong!')
  })
  afterEach(() => vi.unstubAllEnvs())

  it('writes encrypted file for valid params', async () => {
    const cfg = makeConfig(dir)
    const res = await handleWriteCredential(null, {
      accountId: 'acc1', encryptedBlob: 'blob==', iv: 'iv==', algorithm: 'AES-GCM'
    }, cfg, mockLog) as { result: { ok: boolean } }
    expect(res.result.ok).toBe(true)
    expect(existsSync(join(dir, 'acc1.enc'))).toBe(true)
  })

  it('returns error if ORCA_AI_CREDENTIAL_KEY not set', async () => {
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', '')
    const cfg = makeConfig(dir)
    const res = await handleWriteCredential(null, {
      accountId: 'acc1', encryptedBlob: 'blob==', iv: 'iv=='
    }, cfg, mockLog) as { error: { code: number } }
    expect(res.error).toBeDefined()
  })

  it('returns error on missing params', async () => {
    const cfg = makeConfig(dir)
    const res = await handleWriteCredential(null, {}, cfg, mockLog) as { error: { code: number } }
    expect(res.error.code).toBe(-32602)  // InvalidParams
  })

  it('rejects accountId with path traversal', async () => {
    const cfg = makeConfig(dir)
    const res = await handleWriteCredential(null, {
      accountId: '../etc/passwd', encryptedBlob: 'b', iv: 'i'
    }, cfg, mockLog) as { error: { code: number } }
    expect(res.error).toBeDefined()
  })
})

describe('handleReadCredential', () => {
  let dir: string
  beforeEach(() => {
    dir = join(tmpdir(), `orca-cred-test-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'test-master-key-32byteslong!')
  })
  afterEach(() => vi.unstubAllEnvs())

  it('round-trips write→read', async () => {
    const cfg = makeConfig(dir)
    await handleWriteCredential(null, {
      accountId: 'acc2', encryptedBlob: 'payload==', iv: 'myiv==', algorithm: 'AES-GCM'
    }, cfg, mockLog)

    const res = await handleReadCredential(null, { accountId: 'acc2' }, cfg, mockLog) as {
      result: { accountId: string; encryptedBlob: string; iv: string }
    }
    expect(res.result.accountId).toBe('acc2')
    expect(res.result.encryptedBlob).toBe('payload==')
    expect(res.result.iv).toBe('myiv==')
  })

  it('returns PathNotFound for unknown accountId', async () => {
    const cfg = makeConfig(dir)
    const res = await handleReadCredential(null, { accountId: 'nonexistent' }, cfg, mockLog) as {
      error: { code: number }
    }
    expect(res.error.code).toBe(-32001)  // PathNotFound
  })
})

describe('handleDeleteCredential', () => {
  let dir: string
  beforeEach(() => {
    dir = join(tmpdir(), `orca-cred-test-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'test-master-key-32byteslong!')
  })
  afterEach(() => vi.unstubAllEnvs())

  it('deletes existing credential', async () => {
    const cfg = makeConfig(dir)
    await handleWriteCredential(null, {
      accountId: 'del1', encryptedBlob: 'b', iv: 'i'
    }, cfg, mockLog)

    const res = await handleDeleteCredential(null, { accountId: 'del1' }, cfg, mockLog) as {
      result: { ok: boolean; deleted: boolean }
    }
    expect(res.result.ok).toBe(true)
    expect(res.result.deleted).toBe(true)
    expect(existsSync(join(dir, 'del1.enc'))).toBe(false)
  })

  it('returns ok=true deleted=false for non-existent (idempotent)', async () => {
    const cfg = makeConfig(dir)
    const res = await handleDeleteCredential(null, { accountId: 'ghost' }, cfg, mockLog) as {
      result: { ok: boolean; deleted: boolean }
    }
    expect(res.result.ok).toBe(true)
    expect(res.result.deleted).toBe(false)
  })
})
```

**Target: ≥ 12 tests** cho module credential store

---

## 4. Implementation Checklist

- [ ] `src/relay/agent-credential-store.ts` — thêm `import { unlinkSync }` vào import block
- [ ] `src/relay/agent-credential-store.ts` — thêm `handleDeleteCredential()` function
- [ ] `src/relay/agent-credential-store.ts` — thêm `checkProviderReachability()` helper
- [ ] `src/relay/agent-credential-store.ts` — update `handleHealthCheck()` dùng `checkProviderReachability()`
- [ ] `src/relay/agent-rpc-dispatch.ts` — thêm `case 'ai.provider.deleteCredential'`
- [ ] `src/relay/__tests__/agent-credential-store.test.ts` — tạo test file mới
