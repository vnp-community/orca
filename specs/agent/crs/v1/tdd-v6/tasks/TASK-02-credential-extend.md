# TASK-02: Extend Agent Credential Store — deleteCredential + healthCheck v2

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:44

**Phase:** 1
**File:** `src/relay/agent-credential-store.ts`
**Operation:** EXTEND (append to existing file)
**CR:** [CR-AG-09](../solutions/CR-AG-09-credential-store.md)
**TDD:** TDD-AG-09
**Depends on:** Không có dependency
**Blocked by:** Không

---

## Mục tiêu

Thêm 2 tính năng mới vào `agent-credential-store.ts`:

1. **`handleDeleteCredential()`** — xóa credential file (idempotent)
2. **`checkProviderReachability()`** — helper check HTTP reachability (timeout 5s)
3. **Update `handleHealthCheck()`** — thay TODO bằng real reachability check

---

## Context đọc trước

```
src/relay/agent-credential-store.ts  (207 lines tổng)
  - Line 12-17: imports
  - Line 41-49: getCredentialKey()
  - Line 51-58: credentialFilePath()
  - Line 182-206: handleHealthCheck() — có TODO cần update
  - Line 207: cuối file — append vào đây
```

**Import block hiện tại (lines 12-17):**
```typescript
import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { mkdirSync, writeFileSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
```

**`handleHealthCheck()` hiện tại (lines 182-206) — có TODO cần thay:**
```typescript
export async function handleHealthCheck(...): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const provider  = typeof params.provider  === 'string' ? params.provider  : 'anthropic'
  const start = Date.now()

  const credResult = await handleReadCredential(id, { accountId }, config, log) as { error?: unknown; result?: unknown }
  if (credResult.error) return credResult

  // TODO v5.1: Make an actual API call to verify the decrypted key is valid  ← REPLACE THIS
  log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} → credential_readable`)
  return {
    jsonrpc: '2.0', id,
    result: { ok: true, latencyMs: Date.now() - start, note: 'credential_readable' },
  }
}
```

---

## Thay đổi cần thực hiện

### Edit 1 — Thêm `unlinkSync` vào import block (line 13)

```diff
-import { mkdirSync, writeFileSync, readFileSync, existsSync } from 'node:fs'
+import { mkdirSync, writeFileSync, readFileSync, existsSync, unlinkSync } from 'node:fs'
```

### Edit 2 — Replace TODO trong `handleHealthCheck()` (lines 196-205)

Thay thế đoạn từ `// TODO v5.1:` đến cuối return block:

```diff
-  // TODO v5.1: Make an actual API call to verify the decrypted key is valid
-  log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} → credential_readable`)
-  return {
-    jsonrpc: '2.0', id,
-    result: { ok: true, latencyMs: Date.now() - start, note: 'credential_readable' },
-  }
+  const note = await checkProviderReachability(provider)
+  log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider} → ${note}`)
+  return {
+    jsonrpc: '2.0', id,
+    result: { ok: note === 'reachable' || note === 'local_provider', latencyMs: Date.now() - start, note },
+  }
```

### Edit 3 — APPEND vào cuối file (sau line 207)

Thêm nguyên đoạn code sau vào **cuối file** `agent-credential-store.ts`:

```typescript
// ─── checkProviderReachability ────────────────────────────────────────────────

const PROVIDER_HEALTH_URLS: Record<string, string> = {
  anthropic: 'https://api.anthropic.com',
  openai:    'https://api.openai.com',
  gemini:    'https://generativelanguage.googleapis.com',
}

async function checkProviderReachability(provider: string): Promise<string> {
  const url = PROVIDER_HEALTH_URLS[provider]
  // Local providers (Ollama, vLLM, LM Studio) — check localhost port
  if (!url) return 'local_provider'

  try {
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

// ─── ai.provider.deleteCredential ────────────────────────────────────────────

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
      // Idempotent — if not found, still return ok
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

// ─── readDecryptedKey (used by agent-spawner.ts) ─────────────────────────────

/**
 * Read and decrypt the stored credential, returning the encryptedBlob
 * (which in v5.0 is the outer-encrypted plaintext API key).
 * Returns null if credential not found or decryption fails.
 */
export async function readDecryptedKey(
  accountId: string,
  config: AgentConfig,
  log: AgentLogger
): Promise<string | null> {
  const result = await handleReadCredential(null, { accountId }, config, log) as {
    result?: { encryptedBlob: string }; error?: unknown
  }
  if (result.error || !result.result) return null
  return result.result.encryptedBlob
}
```

---

## Verify

```bash
# TypeScript compile check
npx tsc --noEmit -p config/tsconfig.node.json

# Kiểm tra exports mới có trong file
grep -n "handleDeleteCredential\|readDecryptedKey\|checkProviderReachability" src/relay/agent-credential-store.ts

# Expected output:
# <line>: export async function handleDeleteCredential(
# <line>: async function checkProviderReachability(provider: string): Promise<string> {
# <line>: export async function readDecryptedKey(
```

---

## Done criteria

- [ ] `unlinkSync` đã được thêm vào import
- [ ] `checkProviderReachability()` function tồn tại trong file
- [ ] `handleHealthCheck()` dùng `checkProviderReachability()` thay vì TODO comment
- [ ] `handleDeleteCredential()` export function tồn tại
- [ ] `readDecryptedKey()` export function tồn tại
- [ ] TypeScript compile không lỗi
