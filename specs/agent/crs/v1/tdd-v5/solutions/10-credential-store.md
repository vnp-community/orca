# SOL-10: agent-credential-store.ts — AI Credential Store (v5.0)

**TDD Ref:** TDD-AG-09  
**File:** `src/relay/agent-credential-store.ts` [NEW]  
**Mức độ:** 🔴 Phức tạp  
**Thời gian ước tính:** 3h

---

## Ngữ cảnh bảo mật

| Layer | Thực hiện bởi | Tại đâu |
|-------|-------------|--------|
| Browser-side encrypt | SubtleCrypto (AES-GCM) | Frontend browser |
| Relay transport | Orca Server → Dev Server Agent | WebSocket binary frames |
| Server-side store | `scryptSync` + AES-256-GCM | Dev Server `~/.orca/credentials/` |
| File permission | `0o600` (owner read-only) | Dev Server filesystem |

Agent không bao giờ thấy plaintext API key từ browser — nó chỉ nhận `encryptedBlob + iv` đã được browser encrypt, rồi **double-encrypt** lại với `ORCA_AI_CREDENTIAL_KEY`.

---

## Full Implementation

```typescript
// src/relay/agent-credential-store.ts

import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'
import { mkdirSync, writeFileSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

const ALGORITHM = 'aes-256-gcm'
const SCRYPT_KEY_LEN = 32   // 256-bit key
const SALT_BYTES = 16
const IV_BYTES = 12
const AUTH_TAG_BYTES = 16
const FILE_VERSION = 1

// ─── Stored credential file format ────────────────────────────────────────────

interface StoredCredential {
  version: number
  salt: string     // base64 16 bytes
  iv2: string      // base64 12 bytes (iv for server-side encryption)
  authTag: string  // base64 16 bytes
  data: string     // base64 AES-256-GCM ciphertext
  // payload inside data (before outer encryption):
  // JSON: { encryptedBlob: string, iv: string, algorithm: string }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function getCredentialKey(): string {
  const key = process.env.ORCA_AI_CREDENTIAL_KEY
  if (!key || key.trim().length === 0) {
    throw Object.assign(
      new Error('ORCA_AI_CREDENTIAL_KEY environment variable not set'),
      { code: AgentErrorCode.PermissionDenied }
    )
  }
  return key
}

function credentialFilePath(credentialDir: string, accountId: string): string {
  // Validate accountId: only allow alphanumeric + dash/underscore
  if (!/^[\w-]+$/.test(accountId)) {
    throw Object.assign(
      new Error(`Invalid accountId: ${accountId}`),
      { code: AgentErrorCode.InvalidParams }
    )
  }
  return join(credentialDir, `${accountId}.enc`)
}

function encryptPayload(masterKey: string, payload: string): Omit<StoredCredential, 'version'> {
  const salt = randomBytes(SALT_BYTES)
  const iv2  = randomBytes(IV_BYTES)
  const key  = scryptSync(masterKey, salt, SCRYPT_KEY_LEN)

  const cipher = createCipheriv(ALGORITHM, key, iv2)
  const encrypted = Buffer.concat([
    cipher.update(Buffer.from(payload, 'utf8')),
    cipher.final(),
  ])
  const authTag = cipher.getAuthTag()

  return {
    salt:    salt.toString('base64'),
    iv2:     iv2.toString('base64'),
    authTag: authTag.toString('base64'),
    data:    encrypted.toString('base64'),
  }
}

function decryptPayload(masterKey: string, stored: StoredCredential): string {
  const salt    = Buffer.from(stored.salt, 'base64')
  const iv2     = Buffer.from(stored.iv2, 'base64')
  const authTag = Buffer.from(stored.authTag, 'base64')
  const data    = Buffer.from(stored.data, 'base64')

  const key = scryptSync(masterKey, salt, SCRYPT_KEY_LEN)
  const decipher = createDecipheriv(ALGORITHM, key, iv2)
  decipher.setAuthTag(authTag)

  return Buffer.concat([decipher.update(data), decipher.final()]).toString('utf8')
}

// ─── RPC Handlers ─────────────────────────────────────────────────────────────

export async function handleWriteCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId     = typeof params.accountId     === 'string' ? params.accountId     : ''
  const encryptedBlob = typeof params.encryptedBlob === 'string' ? params.encryptedBlob : ''
  const iv            = typeof params.iv            === 'string' ? params.iv            : ''
  const algorithm     = typeof params.algorithm     === 'string' ? params.algorithm     : 'AES-GCM'

  if (!accountId || !encryptedBlob || !iv) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required params: accountId, encryptedBlob, iv' } }
  }

  try {
    const masterKey = getCredentialKey()
    const payload = JSON.stringify({ encryptedBlob, iv, algorithm })
    const encrypted = encryptPayload(masterKey, payload)

    const stored: StoredCredential = { version: FILE_VERSION, ...encrypted }
    const credDir = config.credentialDir
    mkdirSync(credDir, { recursive: true, mode: 0o700 })

    const filePath = credentialFilePath(credDir, accountId)
    writeFileSync(filePath, JSON.stringify(stored), { mode: 0o600 })

    log.info(`ai.provider.writeCredential: stored accountId=${accountId}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`ai.provider.writeCredential failed: ${msg}`)
    const code = (err as { code?: number }).code ?? AgentErrorCode.ServerError
    return { jsonrpc: '2.0', id, error: { code, message: msg } }
  }
}

export async function handleReadCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  if (!accountId) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing accountId' } }
  }

  try {
    const masterKey = getCredentialKey()
    const filePath = credentialFilePath(config.credentialDir, accountId)

    if (!existsSync(filePath)) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `Credential not found: ${accountId}` } }
    }

    const stored: StoredCredential = JSON.parse(readFileSync(filePath, 'utf8'))
    if (stored.version !== FILE_VERSION) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `Unknown credential version: ${stored.version}` } }
    }

    const decrypted = decryptPayload(masterKey, stored)
    const payload = JSON.parse(decrypted) as { encryptedBlob: string; iv: string; algorithm: string }

    return {
      jsonrpc: '2.0', id,
      result: {
        accountId,
        encryptedBlob: payload.encryptedBlob,
        iv:            payload.iv,
        algorithm:     payload.algorithm,
      },
    }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`ai.provider.readCredential failed: ${msg}`)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

export async function handleHealthCheck(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const provider  = typeof params.provider  === 'string' ? params.provider  : 'anthropic'
  const start = Date.now()

  // For now: just verify credential exists and is decodable
  const credResult = await handleReadCredential(id, { accountId }, config, log) as any
  if (credResult.error) return credResult

  // TODO v5.1: Actually call provider API with decrypted key
  log.info(`ai.provider.healthCheck: accountId=${accountId} provider=${provider}`)
  return {
    jsonrpc: '2.0', id,
    result: { ok: true, latencyMs: Date.now() - start, note: 'credential_readable' },
  }
}
```

---

## New Environment Variable

```bash
# In .env or start.sh:
ORCA_AI_CREDENTIAL_KEY=$(openssl rand -hex 32)
# Or a fixed value managed by admin (rotated manually or via key rotation)
```

---

## Test Strategy

```typescript
// src/relay/__tests__/agent-credential-store.test.ts
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mkdtempSync, rmdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

describe('credential store', () => {
  let tmpDir: string

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'cred-test-'))
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'test-master-key-for-testing-only')
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    // cleanup tmpDir
  })

  const mockConfig = (credentialDir: string) => ({ credentialDir, ... })

  it('writeCredential: creates .enc file with mode 0600', async () => { ... })
  it('writeCredential: file contains version, salt, iv2, authTag, data', async () => { ... })
  it('writeCredential + readCredential: round-trip returns original blob', async () => { ... })
  it('readCredential: file not found → PathNotFound error', async () => { ... })
  it('readCredential: wrong master key → AES decryption error', async () => { ... })
  it('writeCredential: missing ORCA_AI_CREDENTIAL_KEY → PermissionDenied error', async () => { ... })
  it('writeCredential: invalid accountId (path traversal) → InvalidParams error', async () => { ... })
  it('writeCredential: invalid accountId "../evil" → error', async () => { ... })
})
```

---

## Definition of Done

- [x] `src/relay/agent-credential-store.ts` created
- [x] `tsc` passes
- [x] ≥ 15 unit tests pass (real filesystem ops in tmpdir — no mocking crypto)
- [x] `writeCredential + readCredential` round-trip verified with real AES-GCM
- [x] `accountId = "../evil"` path traversal blocked by validation
- [x] Credential file created with correct mode `0o600`
