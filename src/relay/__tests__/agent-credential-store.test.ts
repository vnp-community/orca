// src/relay/__tests__/agent-credential-store.test.ts
// Tests use real AES-256-GCM crypto (no mocks) with real tmpdir.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync, existsSync, statSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  handleWriteCredential,
  handleReadCredential,
  handleHealthCheck,
  handleDeleteCredential,
  readDecryptedKey,
} from '../agent-credential-store'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

let tmpDir: string
const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

const MASTER_KEY = 'test-master-key-for-aes256-minimum-32-chars!!'

beforeEach(() => {
  tmpDir = mkdtempSync(join(tmpdir(), 'cred-test-'))
  vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', MASTER_KEY)
})

afterEach(() => {
  vi.unstubAllEnvs()
  rmSync(tmpDir, { recursive: true, force: true })
})

function makeConfig(): AgentConfig {
  return { credentialDir: tmpDir } as unknown as AgentConfig
}

// ─── handleWriteCredential ────────────────────────────────────────────────────
describe('handleWriteCredential', () => {
  it('returns { ok: true } on success', async () => {
    const resp = await handleWriteCredential(1,
      { accountId: 'acc-1', encryptedBlob: 'blob123', iv: 'iv456', algorithm: 'AES-GCM' },
      makeConfig(), mockLog
    ) as any
    expect(resp.result.ok).toBe(true)
  })

  it('creates .enc file in credentialDir', async () => {
    await handleWriteCredential(1,
      { accountId: 'acc-2', encryptedBlob: 'b', iv: 'i' },
      makeConfig(), mockLog
    )
    expect(existsSync(join(tmpDir, 'acc-2.enc'))).toBe(true)
  })

  it('writes multiple accounts without collision', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'a1', encryptedBlob: 'b1', iv: 'i1' }, cfg, mockLog)
    await handleWriteCredential(2, { accountId: 'a2', encryptedBlob: 'b2', iv: 'i2' }, cfg, mockLog)
    expect(existsSync(join(tmpDir, 'a1.enc'))).toBe(true)
    expect(existsSync(join(tmpDir, 'a2.enc'))).toBe(true)
  })

  it('returns InvalidParams (-32602) when accountId is missing', async () => {
    const resp = await handleWriteCredential(1,
      { encryptedBlob: 'b', iv: 'i' },
      makeConfig(), mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams when encryptedBlob is missing', async () => {
    const resp = await handleWriteCredential(1,
      { accountId: 'acc', iv: 'i' },
      makeConfig(), mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams when iv is missing', async () => {
    const resp = await handleWriteCredential(1,
      { accountId: 'acc', encryptedBlob: 'b' },
      makeConfig(), mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns PermissionDenied (-33002) when ORCA_AI_CREDENTIAL_KEY not set', async () => {
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', '')
    const resp = await handleWriteCredential(1,
      { accountId: 'acc', encryptedBlob: 'b', iv: 'i' },
      makeConfig(), mockLog
    ) as any
    expect(resp.error.code).toBe(-33002)
  })

  it('rejects path-traversal accountId "../evil"', async () => {
    const resp = await handleWriteCredential(1,
      { accountId: '../evil', encryptedBlob: 'b', iv: 'i' },
      makeConfig(), mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
    expect(existsSync(join(tmpDir, '..', 'evil.enc'))).toBe(false)
  })

  it('rejects accountId with slash "/evil"', async () => {
    const resp = await handleWriteCredential(1,
      { accountId: '/evil', encryptedBlob: 'b', iv: 'i' },
      makeConfig(), mockLog
    ) as any
    expect(resp.error).toBeDefined()
  })

  it('rejects accountId with space "acc id"', async () => {
    const resp = await handleWriteCredential(1,
      { accountId: 'acc id', encryptedBlob: 'b', iv: 'i' },
      makeConfig(), mockLog
    ) as any
    expect(resp.error).toBeDefined()
  })
})

// ─── handleReadCredential ─────────────────────────────────────────────────────
describe('handleReadCredential', () => {
  it('round-trip: write then read returns same encryptedBlob + iv + algorithm', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1,
      { accountId: 'rt-acc', encryptedBlob: 'MY_ENCRYPTED_BLOB', iv: 'MY_IV', algorithm: 'AES-GCM' },
      cfg, mockLog
    )
    const resp = await handleReadCredential(2, { accountId: 'rt-acc' }, cfg, mockLog) as any
    expect(resp.result.encryptedBlob).toBe('MY_ENCRYPTED_BLOB')
    expect(resp.result.iv).toBe('MY_IV')
    expect(resp.result.algorithm).toBe('AES-GCM')
  })

  it('includes accountId in result', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'id-test', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    const resp = await handleReadCredential(2, { accountId: 'id-test' }, cfg, mockLog) as any
    expect(resp.result.accountId).toBe('id-test')
  })

  it('returns PathNotFound (-33003) for missing accountId', async () => {
    const resp = await handleReadCredential(1, { accountId: 'nonexistent' }, makeConfig(), mockLog) as any
    expect(resp.error.code).toBe(-33003)
  })

  it('returns error when ORCA_AI_CREDENTIAL_KEY changed (decryption fails)', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'change-key', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'different-key-that-wont-decrypt-previous-data!!x')
    const resp = await handleReadCredential(1, { accountId: 'change-key' }, cfg, mockLog) as any
    expect(resp.error).toBeDefined()
  })

  it('returns InvalidParams for missing accountId param', async () => {
    const resp = await handleReadCredential(1, {}, makeConfig(), mockLog) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('does not crash on corrupted .enc file', async () => {
    const { writeFileSync } = await import('node:fs')
    writeFileSync(join(tmpDir, 'corrupted.enc'), '{"version":1,"salt":"bad","iv2":"bad","authTag":"bad","data":"bad"}')
    const resp = await handleReadCredential(1, { accountId: 'corrupted' }, makeConfig(), mockLog) as any
    expect(resp.error).toBeDefined()
  })
})

// ─── handleHealthCheck ────────────────────────────────────────────────────────
describe('handleHealthCheck', () => {
  it('returns latencyMs when credential is readable', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'hc-acc', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    const resp = await handleHealthCheck(1, { accountId: 'hc-acc' }, cfg, mockLog) as any
    expect(typeof resp.result?.latencyMs).toBe('number')
    expect(resp.result?.latencyMs).toBeGreaterThanOrEqual(0)
  })

  it('returns note field (string) in result', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'hc-note', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    const resp = await handleHealthCheck(1, { accountId: 'hc-note', provider: 'anthropic' }, cfg, mockLog) as any
    // note can be 'reachable', 'unreachable', or 'local_provider' depending on network
    expect(typeof resp.result?.note).toBe('string')
    expect(resp.result?.note.length).toBeGreaterThan(0)
  })

  it('returns ok=true for local_provider (provider not in PROVIDER_HEALTH_URLS)', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'hc-local', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    const resp = await handleHealthCheck(1, { accountId: 'hc-local', provider: 'ollama' }, cfg, mockLog) as any
    expect(resp.result?.ok).toBe(true)
    expect(resp.result?.note).toBe('local_provider')
  })

  it('returns error when credential does not exist', async () => {
    const resp = await handleHealthCheck(1, { accountId: 'nope' }, makeConfig(), mockLog) as any
    expect(resp.error).toBeDefined()
  })
})

// ─── handleDeleteCredential ───────────────────────────────────────────────────
describe('handleDeleteCredential', () => {
  it('deletes an existing credential and returns { ok: true, deleted: true }', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'del-1', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    const resp = await handleDeleteCredential(2, { accountId: 'del-1' }, cfg, mockLog) as any
    expect(resp.result.ok).toBe(true)
    expect(resp.result.deleted).toBe(true)
  })

  it('credential is gone after delete (read returns error)', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'del-2', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    await handleDeleteCredential(2, { accountId: 'del-2' }, cfg, mockLog)
    const resp = await handleReadCredential(3, { accountId: 'del-2' }, cfg, mockLog) as any
    expect(resp.error).toBeDefined()
  })

  it('is idempotent: returns { ok: true, deleted: false } when not found', async () => {
    const resp = await handleDeleteCredential(1, { accountId: 'nonexistent-xxx' }, makeConfig(), mockLog) as any
    expect(resp.result.ok).toBe(true)
    expect(resp.result.deleted).toBe(false)
  })

  it('returns InvalidParams (-32602) when accountId is missing', async () => {
    const resp = await handleDeleteCredential(1, {}, makeConfig(), mockLog) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('.enc file is removed from disk after delete', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'del-3', encryptedBlob: 'b', iv: 'i' }, cfg, mockLog)
    expect(existsSync(join(tmpDir, 'del-3.enc'))).toBe(true)
    await handleDeleteCredential(2, { accountId: 'del-3' }, cfg, mockLog)
    expect(existsSync(join(tmpDir, 'del-3.enc'))).toBe(false)
  })
})

// ─── readDecryptedKey ─────────────────────────────────────────────────────────
describe('readDecryptedKey', () => {
  it('returns encryptedBlob for existing credential', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'rdk-1', encryptedBlob: 'my-api-key-blob', iv: 'iv1' }, cfg, mockLog)
    const key = await readDecryptedKey('rdk-1', cfg, mockLog)
    expect(key).toBe('my-api-key-blob')
  })

  it('returns null for non-existent credential', async () => {
    const key = await readDecryptedKey('does-not-exist-xyz', makeConfig(), mockLog)
    expect(key).toBeNull()
  })

  it('returns null when ORCA_AI_CREDENTIAL_KEY is wrong (decryption fails)', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'rdk-key', encryptedBlob: 'blob', iv: 'iv1' }, cfg, mockLog)
    vi.stubEnv('ORCA_AI_CREDENTIAL_KEY', 'wrong-master-key-that-will-fail-decryption!!!!')
    const key = await readDecryptedKey('rdk-key', cfg, mockLog)
    expect(key).toBeNull()
  })

  it('returns different blobs for different accountIds', async () => {
    const cfg = makeConfig()
    await handleWriteCredential(1, { accountId: 'rdk-a', encryptedBlob: 'blob-A', iv: 'iv1' }, cfg, mockLog)
    await handleWriteCredential(2, { accountId: 'rdk-b', encryptedBlob: 'blob-B', iv: 'iv2' }, cfg, mockLog)
    const keyA = await readDecryptedKey('rdk-a', cfg, mockLog)
    const keyB = await readDecryptedKey('rdk-b', cfg, mockLog)
    expect(keyA).toBe('blob-A')
    expect(keyB).toBe('blob-B')
    expect(keyA).not.toBe(keyB)
  })
})
