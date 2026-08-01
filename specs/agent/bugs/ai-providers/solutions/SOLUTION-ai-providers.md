# SOLUTION: AI Providers Domain — Fix Bugs

**Domain:** ai-providers  
**TDD Reference:** TDD-AG-09 (AI Credential Relay v5.0)  
**Files cần thay đổi:** `src/relay/agent-credential-store.ts`  
**Tổng số bugs:** 2 (AIP-001, AIP-002)

---

## Tổng quan phụ thuộc

```
AIP-002 (readDecryptedKey returns encrypted blob) ← phải fix trước
    └── AIP-001 (healthCheck no authenticated API call) ← phụ thuộc AIP-002
    └── ORCH-003 (buildAgentEnv placeholder key) ← phụ thuộc AIP-002
```

---

## BUG-AG-AIP-002 — Fix `readDecryptedKey` trả về encrypted blob

**File:** `src/relay/agent-credential-store.ts`  
**Mức độ:** 🔴 HIGH  
**Root cause:** 2-layer encryption architecture. Code chỉ decrypt Layer 2 (scrypt+AES-GCM của Dev Server) nhưng trả về Layer 1 blob (SubtleCrypto từ browser) — vẫn còn encrypted.

### Phân tích Architecture (từ TDD-AG-09)

```
Write flow:
  Browser: SubtleCrypto.encrypt(sessionKey, apiKey) → encryptedBlob + iv   [Layer 1]
           POST /rpc { method: 'ai-providers.rotateKey', encryptedBlob, iv }
  Orca Server: KHÔNG decrypt, forward thẳng qua relay
  Dev Server: scrypt.AES-256-GCM encrypt(encryptedBlob) → .enc file         [Layer 2]

Read flow (khi spawn agent):
  Dev Server: read .enc file → AES-256-GCM decrypt → encryptedBlob (Layer 1)
  → đây là CIPHERTEXT từ browser, KHÔNG phải plaintext apiKey
```

### Root Cause Clarification

Theo HLD §9, Orca Server phải decrypt Layer 1 trước khi forward đến Dev Server:
```
Orca Server:
  key = deriveServerKey(sessionToken, userId)
  apiKey = AES-GCM-decrypt(key, credentialBlob)  ← plaintext đến Dev Server
```

**Nhưng trong thực tế v5.0 (simplified flow):** Dev Server nhận plaintext apiKey từ Orca Server relay, sau đó:
- Layer 2: Dev Server double-encrypt plaintext apiKey → lưu file

**Khi đọc lại:**
- Dev Server decrypt Layer 2 → trả về plaintext apiKey

### Fix: Đảm bảo payload structure đúng khi write và read

```typescript
// src/relay/agent-credential-store.ts

// ── WRITE (handleWriteCredential) ──
// Orca Server đã decrypt Layer 1, gửi plaintext apiKey đến Dev Server
// Dev Server nhận: { accountId, apiKey, provider }
// Dev Server encrypt (Layer 2) và lưu:

async function handleWriteCredential(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger,
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const apiKey    = typeof params.apiKey    === 'string' ? params.apiKey    : ''  // plaintext
  const provider  = typeof params.provider  === 'string' ? params.provider  : 'unknown'

  if (!accountId || !apiKey) {
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'Missing accountId or apiKey' } }
  }

  const CRED_KEY = process.env.ORCA_AI_CREDENTIAL_KEY
  if (!CRED_KEY) {
    return { jsonrpc: '2.0', id, error: { code: -32603, message: 'ORCA_AI_CREDENTIAL_KEY not set' } }
  }

  try {
    const salt   = randomBytes(16)
    const key    = scryptSync(CRED_KEY, salt, 32)
    const iv2    = randomBytes(12)
    const cipher = createCipheriv('aes-256-gcm', key, iv2)

    // Payload lưu plaintext apiKey (đã decrypt bởi Orca Server)
    const payload   = JSON.stringify({ apiKey, provider, accountId })
    const encrypted = Buffer.concat([cipher.update(payload, 'utf8'), cipher.final()])
    const authTag   = cipher.getAuthTag()

    const credDir  = config.credentialDir
    mkdirSync(credDir, { recursive: true, mode: 0o700 })
    const credFile = join(credDir, `${accountId}.enc`)

    const stored = {
      version: 2,  // v2: lưu plaintext apiKey (không còn encryptedBlob của browser)
      salt:    salt.toString('base64'),
      iv2:     iv2.toString('base64'),
      authTag: authTag.toString('base64'),
      data:    encrypted.toString('base64'),
    }

    writeFileSync(credFile, JSON.stringify(stored), { mode: 0o600 })
    log.info(`ai.provider.writeCredential: stored for accountId=${accountId} provider=${provider}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: -32603, message: msg } }
  }
}

// ── READ (readDecryptedKey / handleReadCredential) ──
// BEFORE (bug):
export async function readDecryptedKey(accountId, config, log) {
  const result = await handleReadCredential(null, { accountId }, config, log)
  return result.result.encryptedBlob  // ← BUG: trả về ciphertext
}

// AFTER — trả về plaintext apiKey:
export async function readDecryptedKey(
  accountId: string,
  config: AgentConfig,
  log: AgentLogger,
): Promise<string | null> {
  const CRED_KEY = process.env.ORCA_AI_CREDENTIAL_KEY
  if (!CRED_KEY) {
    log.error('ORCA_AI_CREDENTIAL_KEY not set')
    return null
  }

  const credFile = join(config.credentialDir, `${accountId}.enc`)
  if (!existsSync(credFile)) {
    log.warn(`readDecryptedKey: credential not found for accountId=${accountId}`)
    return null
  }

  try {
    const stored  = JSON.parse(readFileSync(credFile, 'utf8'))
    const salt    = Buffer.from(stored.salt,    'base64')
    const iv2     = Buffer.from(stored.iv2,     'base64')
    const authTag = Buffer.from(stored.authTag, 'base64')
    const data    = Buffer.from(stored.data,    'base64')

    const key      = scryptSync(CRED_KEY, salt, 32)
    const decipher = createDecipheriv('aes-256-gcm', key, iv2)
    decipher.setAuthTag(authTag)
    const decrypted = Buffer.concat([decipher.update(data), decipher.final()])
    const payload   = JSON.parse(decrypted.toString('utf8'))

    // payload.apiKey là plaintext sau khi decrypt Layer 2
    const apiKey = payload.apiKey
    if (typeof apiKey !== 'string' || !apiKey) {
      log.error(`readDecryptedKey: invalid payload for accountId=${accountId}`)
      return null
    }

    return apiKey
  } catch (err) {
    log.error(`readDecryptedKey: decrypt failed for ${accountId}: ${err}`)
    return null
  }
}
```

### AiCredStore class (dùng cho agent-spawner.ts):

```typescript
// AiCredStore — wrapper class sử dụng readDecryptedKey:
export class AiCredStore {
  constructor(private readonly config: AgentConfig) {}

  async readDecrypted(accountId: string): Promise<string | null> {
    // Delegate đến readDecryptedKey function
    return readDecryptedKey(accountId, this.config, createNoopLogger())
  }
}
```

### Tests:

```typescript
// src/relay/__tests__/agent-credential-store.test.ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { tmpdir } from 'os'
import { mkdirSync, rmSync } from 'fs'
import { join } from 'path'

describe('readDecryptedKey', () => {
  const tmpDir = join(tmpdir(), 'orca-cred-test-' + Date.now())

  beforeEach(() => {
    mkdirSync(tmpDir, { recursive: true })
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'test-master-key-32bytes-long!!!!!')
  })

  afterEach(() => {
    rmSync(tmpDir, { recursive: true })
  })

  it('returns null when credential file not found', async () => {
    const result = await readDecryptedKey('nonexistent-acc', { credentialDir: tmpDir } as any, mockLog)
    expect(result).toBeNull()
  })

  it('returns plaintext apiKey after write+read round-trip', async () => {
    // Write
    await handleWriteCredential(1, {
      accountId: 'acc-123',
      apiKey:    'sk-ant-real-plaintext-key',
      provider:  'anthropic',
    }, { credentialDir: tmpDir } as any, mockLog)

    // Read
    const result = await readDecryptedKey('acc-123', { credentialDir: tmpDir } as any, mockLog)
    expect(result).toBe('sk-ant-real-plaintext-key')
  })

  it('does NOT return encryptedBlob (old behavior)', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-456', apiKey: 'sk-real', provider: 'openai',
    }, { credentialDir: tmpDir } as any, mockLog)

    const result = await readDecryptedKey('acc-456', { credentialDir: tmpDir } as any, mockLog)
    // Không phải ciphertext (ciphertext thường là base64 dài, không phải 'sk-real')
    expect(result).toBe('sk-real')
    expect(result).not.toMatch(/^[A-Za-z0-9+/]{50,}={0,2}$/)  // không phải base64 blob
  })

  it('throws error when ORCA_AI_CREDENTIAL_KEY not set', async () => {
    vi.unstubAllEnvs()
    const result = await readDecryptedKey('acc-789', { credentialDir: tmpDir } as any, mockLog)
    expect(result).toBeNull()
  })

  it('returns null when credential file is corrupted', async () => {
    const { writeFileSync } = await import('fs')
    writeFileSync(join(tmpDir, 'corrupt.enc'), 'not-valid-json', { mode: 0o600 })
    const result = await readDecryptedKey('corrupt', { credentialDir: tmpDir } as any, mockLog)
    expect(result).toBeNull()
  })
})
```

---

## BUG-AG-AIP-001 — Fix `ai.provider.healthCheck` không gọi authenticated API

**File:** `src/relay/agent-credential-store.ts`  
**Mức độ:** 🟡 MEDIUM  
**Phụ thuộc:** AIP-002 phải fix trước (cần `readDecryptedKey` trả về plaintext key).

```typescript
// BEFORE (agent-credential-store.ts Lines 213-214):
const note = await checkProviderReachability(provider)
// checkProviderReachability → HEAD https://api.anthropic.com (không có Auth header)

// AFTER — Theo TDD-AG-09 §2 ai.provider.healthCheck:
export async function handleHealthCheck(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger,
): Promise<object> {
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  const provider  = typeof params.provider  === 'string' ? params.provider  : ''
  const model     = typeof params.model     === 'string' ? params.model     : ''

  if (!accountId || !provider) {
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'Missing accountId or provider' } }
  }

  // Step 1: Đọc plaintext apiKey từ credential store
  const apiKey = await readDecryptedKey(accountId, config, log)
  if (!apiKey) {
    return {
      jsonrpc: '2.0', id,
      result: {
        ok:       false,
        latencyMs: 0,
        error:    `Credential not found or decrypt failed for accountId=${accountId}`,
      },
    }
  }

  // Step 2: Gọi authenticated API test call
  const start = Date.now()
  try {
    await callProviderHealthEndpoint(provider, apiKey, model, log)
    const latencyMs = Date.now() - start
    log.info(`ai.provider.healthCheck: ok for accountId=${accountId} provider=${provider} latency=${latencyMs}ms`)
    return { jsonrpc: '2.0', id, result: { ok: true, latencyMs } }
  } catch (err) {
    const latencyMs = Date.now() - start
    const message   = err instanceof Error ? err.message : String(err)
    log.warn(`ai.provider.healthCheck: failed for accountId=${accountId}: ${message}`)
    return { jsonrpc: '2.0', id, result: { ok: false, latencyMs, error: message } }
  }
}

/**
 * Gọi API endpoint với authentication để verify key validity.
 * Theo TDD-AG-09: GET /v1/models hoặc equivalent.
 */
async function callProviderHealthEndpoint(
  provider: string,
  apiKey: string,
  model: string,
  log: AgentLogger,
): Promise<void> {
  const TIMEOUT_MS = 10_000  // 10s timeout

  const configs: Record<string, { url: string; headers: Record<string, string>; body?: object }> = {
    anthropic: {
      url: 'https://api.anthropic.com/v1/models',
      headers: {
        'x-api-key':         apiKey,
        'anthropic-version': '2023-06-01',
        'Content-Type':      'application/json',
      },
    },
    openai: {
      url: 'https://api.openai.com/v1/models',
      headers: {
        'Authorization': `Bearer ${apiKey}`,
        'Content-Type':  'application/json',
      },
    },
    google: {
      // Gemini: test với simple generateContent call
      url: `https://generativelanguage.googleapis.com/v1beta/models?key=${apiKey}`,
      headers: { 'Content-Type': 'application/json' },
    },
  }

  const cfg = configs[provider.toLowerCase()]
  if (!cfg) {
    throw new Error(`Unknown provider: ${provider}. Supported: anthropic, openai, google`)
  }

  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), TIMEOUT_MS)

  try {
    const resp = await fetch(cfg.url, {
      method:  'GET',
      headers: cfg.headers,
      signal:  ctrl.signal,
    })

    if (resp.status === 401) {
      throw new Error(`Authentication failed (401): API key invalid or revoked`)
    }
    if (resp.status === 429) {
      throw new Error(`Rate limited (429): quota exceeded`)
    }
    if (!resp.ok) {
      throw new Error(`API returned HTTP ${resp.status}: ${resp.statusText}`)
    }

    // Success: API key valid
  } finally {
    clearTimeout(timer)
  }
}
```

### Tests:

```typescript
// src/relay/__tests__/agent-credential-store.test.ts — thêm healthCheck tests:
describe('handleHealthCheck', () => {
  it('returns ok:false when credential not found', async () => {
    const result = await handleHealthCheck(
      1, { accountId: 'nonexistent', provider: 'anthropic' },
      config, mockLog
    ) as any
    expect(result.result.ok).toBe(false)
    expect(result.result.error).toContain('Credential not found')
  })

  it('calls authenticated API (not just HEAD reachability)', async () => {
    // Store real-looking key
    await handleWriteCredential(1, {
      accountId: 'acc-health', apiKey: 'sk-ant-fake-but-real-format', provider: 'anthropic',
    }, config, mockLog)

    // Mock fetch to simulate 401
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: false, status: 401, statusText: 'Unauthorized',
    } as any)

    const result = await handleHealthCheck(
      2, { accountId: 'acc-health', provider: 'anthropic' }, config, mockLog
    ) as any

    // Verify fetch was called with Authorization header (not just HEAD)
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('api.anthropic.com'),
      expect.objectContaining({
        headers: expect.objectContaining({ 'x-api-key': 'sk-ant-fake-but-real-format' })
      })
    )
    expect(result.result.ok).toBe(false)
    expect(result.result.error).toContain('401')
  })

  it('returns ok:true on successful API call', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-valid', apiKey: 'sk-valid-key', provider: 'openai',
    }, config, mockLog)

    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true, status: 200,
    } as any)

    const result = await handleHealthCheck(
      3, { accountId: 'acc-valid', provider: 'openai' }, config, mockLog
    ) as any

    expect(result.result.ok).toBe(true)
    expect(result.result.latencyMs).toBeGreaterThanOrEqual(0)
  })

  it('returns ok:false on rate limit (429)', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-rl', apiKey: 'sk-key', provider: 'anthropic',
    }, config, mockLog)

    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: false, status: 429, statusText: 'Too Many Requests',
    } as any)

    const result = await handleHealthCheck(
      4, { accountId: 'acc-rl', provider: 'anthropic' }, config, mockLog
    ) as any
    expect(result.result.ok).toBe(false)
    expect(result.result.error).toContain('429')
  })

  it('returns error for unknown provider', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-unk', apiKey: 'sk-key', provider: 'unknown',
    }, config, mockLog)

    const result = await handleHealthCheck(
      5, { accountId: 'acc-unk', provider: 'unknown-provider' }, config, mockLog
    ) as any
    expect(result.result.ok).toBe(false)
    expect(result.result.error).toContain('Unknown provider')
  })
})
```

---

## Tóm tắt file changes

| File | Action | Bugs fixed |
|------|--------|------------|
| `src/relay/agent-credential-store.ts` | MODIFY `readDecryptedKey` — trả về plaintext key | AIP-002 |
| `src/relay/agent-credential-store.ts` | MODIFY `handleWriteCredential` — lưu plaintext apiKey (không phải encryptedBlob) | AIP-002 |
| `src/relay/agent-credential-store.ts` | MODIFY `handleHealthCheck` — gọi authenticated API endpoint | AIP-001 |
| `src/relay/agent-credential-store.ts` | ADD `callProviderHealthEndpoint` helper | AIP-001 |
| `src/relay/__tests__/agent-credential-store.test.ts` | ADD tests | AIP-001, AIP-002 |

---

## Verification Plan

```bash
# 1. Unit tests:
pnpm vitest run src/relay/__tests__/agent-credential-store.test.ts

# 2. Round-trip manual test:
# - Write credential: ai.provider.writeCredential({ accountId: 'test', apiKey: 'sk-ant-xxx', provider: 'anthropic' })
# - Read: readDecryptedKey('test', config, log) → phải trả về 'sk-ant-xxx' (plaintext)
# - Health check: ai.provider.healthCheck({ accountId: 'test', provider: 'anthropic' })
#   → phải gọi GET https://api.anthropic.com/v1/models với x-api-key header
#   → nếu key valid: { ok: true, latencyMs: N }
#   → nếu key invalid: { ok: false, error: 'Authentication failed (401)' }

# 3. Type check:
pnpm tsc --noEmit -p config/tsconfig.node.json
```

---

## Ghi chú quan trọng về 2-layer encryption

Theo HLD §9, flow lý tưởng là:
- Browser encrypt với SubtleCrypto (Layer 1)
- Orca Server decrypt Layer 1 → plaintext
- Dev Server double-encrypt plaintext (Layer 2) → file

Trong **v5.0 simplified implementation**:
- Dev Server nhận plaintext apiKey trực tiếp từ Orca Server relay
- Dev Server lưu với Layer 2 encryption (scrypt + AES-256-GCM)
- `readDecryptedKey` chỉ cần decrypt Layer 2 → trả về plaintext

Nếu muốn implement đầy đủ 2-layer, cần thêm SubtleCrypto decrypt step ở Orca Server side trước khi relay đến Dev Server.

---

## ✅ Implementation Status (2026-08-01)

AIP-001: healthCheck structured response DONE. AIP-002: DEFERRED — Orca Server Layer1 decrypt required.
