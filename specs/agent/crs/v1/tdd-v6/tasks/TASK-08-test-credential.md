# TASK-08: Write Tests — agent-credential-store

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:52
> 📝 **Note:** Extended existing test file (was 182 lines) — added `handleDeleteCredential` (5 tests) + `readDecryptedKey` (4 tests) + updated `handleHealthCheck` (added `note` and `local_provider` tests). Tests run via `pnpm test:server`.

**Phase:** 5
**File:** `src/relay/__tests__/agent-credential-store.test.ts` (NEW FILE)
**Operation:** CREATE
**Depends on:** TASK-02 phải hoàn thành
**Test framework:** Xác nhận framework từ package.json (vitest hoặc jest)

---

## Mục tiêu

Viết tests cho các functions trong `agent-credential-store.ts`:
- `handleWriteCredential()` — write + read roundtrip
- `handleReadCredential()` — missing file, invalid accountId
- `handleDeleteCredential()` — idempotent delete (NEW từ TASK-02)
- `handleHealthCheck()` — reachability check (NEW từ TASK-02)
- `readDecryptedKey()` — returns encryptedBlob (NEW từ TASK-02)

---

## Prerequisite check

```bash
# Xác định test framework
cat package.json | grep -E '"vitest|jest|@jest"'
# Tìm test script
cat package.json | grep '"test"'

# Xem existing tests để biết import pattern
ls src/relay/__tests__/ 2>/dev/null || echo "no test dir yet"
ls src/**/__tests__/ 2>/dev/null | head -5
```

---

## Test File

Tạo `src/relay/__tests__/agent-credential-store.test.ts`:

```typescript
// src/relay/__tests__/agent-credential-store.test.ts

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
// (hoặc jest nếu dự án dùng jest)
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import {
  handleWriteCredential,
  handleReadCredential,
  handleDeleteCredential,
  handleHealthCheck,
  readDecryptedKey,
} from '../agent-credential-store'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

// ─── Test Fixtures ─────────────────────────────────────────────────────────────

const TEST_MASTER_KEY = 'test-master-key-for-unit-tests-32chars!'

function makeConfig(credentialDir: string): AgentConfig {
  return {
    devServerId:   'test-server',
    agentToken:    '',
    workDir:       tmpdir(),
    credentialDir,
    toolPath:      '/usr/bin:/bin',
    toolEnv:       { PATH: '/usr/bin:/bin', ORCA_AI_CREDENTIAL_KEY: TEST_MASTER_KEY },
  } as unknown as AgentConfig
}

const MOCK_LOG: AgentLogger = {
  info:  () => {},
  warn:  () => {},
  error: () => {},
  debug: () => {},
}

// ─── Setup / Teardown ─────────────────────────────────────────────────────────

let tmpDir: string
let config: AgentConfig

beforeEach(() => {
  tmpDir = mkdtempSync(join(tmpdir(), 'orca-cred-test-'))
  config = makeConfig(tmpDir)
  process.env.ORCA_AI_CREDENTIAL_KEY = TEST_MASTER_KEY
})

afterEach(() => {
  rmSync(tmpDir, { recursive: true, force: true })
  delete process.env.ORCA_AI_CREDENTIAL_KEY
})

// ─── handleWriteCredential ────────────────────────────────────────────────────

describe('handleWriteCredential', () => {
  it('writes credential file and returns ok', async () => {
    const res = await handleWriteCredential(1, {
      accountId:     'acc-01',
      encryptedBlob: 'test-encrypted-api-key',
      iv:            'test-iv-base64',
      algorithm:     'AES-GCM',
    }, config, MOCK_LOG) as { result?: { ok: boolean } }

    expect(res.result?.ok).toBe(true)
  })

  it('returns InvalidParams for missing accountId', async () => {
    const res = await handleWriteCredential(1, {
      encryptedBlob: 'blob', iv: 'iv', algorithm: 'AES-GCM',
    }, config, MOCK_LOG) as { error?: { code: number } }

    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for missing encryptedBlob', async () => {
    const res = await handleWriteCredential(1, {
      accountId: 'acc-01', iv: 'iv', algorithm: 'AES-GCM',
    }, config, MOCK_LOG) as { error?: { code: number } }

    expect(res.error?.code).toBe(-32602)
  })

  it('rejects accountId with path traversal', async () => {
    const res = await handleWriteCredential(1, {
      accountId: '../../../etc/passwd', encryptedBlob: 'x', iv: 'y', algorithm: 'AES-GCM',
    }, config, MOCK_LOG) as { error?: { code: number } }

    expect(res.error).toBeDefined()
  })
})

// ─── handleReadCredential ─────────────────────────────────────────────────────

describe('handleReadCredential', () => {
  it('read returns same encryptedBlob that was written', async () => {
    const blob = 'my-encrypted-api-key-roundtrip'

    await handleWriteCredential(1, {
      accountId: 'acc-rw', encryptedBlob: blob, iv: 'iv-rw', algorithm: 'AES-GCM',
    }, config, MOCK_LOG)

    const res = await handleReadCredential(2, { accountId: 'acc-rw' }, config, MOCK_LOG) as {
      result?: { encryptedBlob: string; iv: string; algorithm: string }
    }

    expect(res.result?.encryptedBlob).toBe(blob)
    expect(res.result?.iv).toBe('iv-rw')
  })

  it('returns error when credential not found', async () => {
    const res = await handleReadCredential(1, { accountId: 'nonexistent-acc' }, config, MOCK_LOG) as {
      error?: { code: number }
    }
    expect(res.error).toBeDefined()
  })

  it('returns InvalidParams for missing accountId', async () => {
    const res = await handleReadCredential(1, {}, config, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

// ─── handleDeleteCredential ───────────────────────────────────────────────────

describe('handleDeleteCredential', () => {
  it('deletes an existing credential', async () => {
    // Write first
    await handleWriteCredential(1, {
      accountId: 'acc-del', encryptedBlob: 'blob', iv: 'iv', algorithm: 'AES-GCM',
    }, config, MOCK_LOG)

    // Delete
    const res = await handleDeleteCredential(2, { accountId: 'acc-del' }, config, MOCK_LOG) as {
      result?: { ok: boolean; deleted: boolean }
    }
    expect(res.result?.ok).toBe(true)
    expect(res.result?.deleted).toBe(true)
  })

  it('returns ok:true when credential does not exist (idempotent)', async () => {
    const res = await handleDeleteCredential(1, { accountId: 'nonexistent-acc' }, config, MOCK_LOG) as {
      result?: { ok: boolean; deleted: boolean }
    }
    expect(res.result?.ok).toBe(true)
    expect(res.result?.deleted).toBe(false)
  })

  it('credential not readable after delete', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-del2', encryptedBlob: 'blob', iv: 'iv', algorithm: 'AES-GCM',
    }, config, MOCK_LOG)

    await handleDeleteCredential(2, { accountId: 'acc-del2' }, config, MOCK_LOG)

    const res = await handleReadCredential(3, { accountId: 'acc-del2' }, config, MOCK_LOG) as {
      error?: unknown
    }
    expect(res.error).toBeDefined()
  })

  it('returns InvalidParams for missing accountId', async () => {
    const res = await handleDeleteCredential(1, {}, config, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

// ─── readDecryptedKey ─────────────────────────────────────────────────────────

describe('readDecryptedKey', () => {
  it('returns encryptedBlob for existing credential', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-key', encryptedBlob: 'my-api-key', iv: 'iv', algorithm: 'AES-GCM',
    }, config, MOCK_LOG)

    const key = await readDecryptedKey('acc-key', config, MOCK_LOG)
    expect(key).toBe('my-api-key')
  })

  it('returns null for non-existent credential', async () => {
    const key = await readDecryptedKey('nonexistent', config, MOCK_LOG)
    expect(key).toBeNull()
  })
})

// ─── handleHealthCheck ────────────────────────────────────────────────────────

describe('handleHealthCheck', () => {
  it('returns ok:false when credential not found', async () => {
    const res = await handleHealthCheck(1, {
      accountId: 'no-cred-acc', provider: 'anthropic',
    }, config, MOCK_LOG) as { error?: unknown }

    // Credential not found → error propagated
    expect(res.error).toBeDefined()
  })

  it('returns latencyMs in result', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-hc', encryptedBlob: 'blob', iv: 'iv', algorithm: 'AES-GCM',
    }, config, MOCK_LOG)

    const res = await handleHealthCheck(2, {
      accountId: 'acc-hc', provider: 'anthropic',
    }, config, MOCK_LOG) as { result?: { latencyMs: number; note: string } }

    expect(typeof res.result?.latencyMs).toBe('number')
    expect(res.result?.latencyMs).toBeGreaterThanOrEqual(0)
  })

  it('returns note field in result', async () => {
    await handleWriteCredential(1, {
      accountId: 'acc-hc2', encryptedBlob: 'blob', iv: 'iv', algorithm: 'AES-GCM',
    }, config, MOCK_LOG)

    const res = await handleHealthCheck(2, {
      accountId: 'acc-hc2', provider: 'anthropic',
    }, config, MOCK_LOG) as { result?: { note: string } }

    // Note can be 'reachable', 'unreachable', or 'local_provider'
    expect(typeof res.result?.note).toBe('string')
    expect(res.result?.note).toBeTruthy()
  })
})
```

---

## Verify

```bash
# Run tests (adjust command based on framework)
pnpm test src/relay/__tests__/agent-credential-store.test.ts
# hoặc: npx vitest run src/relay/__tests__/agent-credential-store.test.ts

# Expected: ≥ 15 tests, all pass
# (write: 4, read: 3, delete: 4, readDecryptedKey: 2, healthCheck: 3)
```

---

## Done criteria

- [ ] Test file tạo thành công
- [ ] Write/Read roundtrip test pass
- [ ] Delete idempotent test pass
- [ ] `readDecryptedKey` returns correct blob
- [ ] `handleHealthCheck` returns `latencyMs` và `note` fields
- [ ] Tất cả tests pass: ≥ 15 tests
