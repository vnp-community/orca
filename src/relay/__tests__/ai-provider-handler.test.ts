/**
 * Tests for ai-provider relay handler (TDD-16) — T13
 *
 * Tests that credentials are written/read correctly from dev server filesystem.
 * Uses vi.mock('node:fs/promises') to intercept file operations.
 *
 * Actual export: aiProviderHandlers (object with handler functions)
 * Methods: 'ai.provider.writeCredential', 'ai.provider.readCredential',
 *          'ai.provider.testConnection', 'ai.provider.healthCheck'
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// ── Mock node:fs/promises ─────────────────────────────────────────────────────

const mockFiles = new Map<string, string>()

vi.mock('node:fs/promises', async (importOriginal) => {
  const orig = await importOriginal<typeof import('node:fs/promises')>()
  return {
    ...orig,
    mkdir: vi.fn().mockResolvedValue(undefined),
    writeFile: vi.fn().mockImplementation(async (path: string, content: string) => {
      mockFiles.set(String(path), content)
    }),
    readFile: vi.fn().mockImplementation(async (path: string) => {
      const content = mockFiles.get(String(path))
      if (!content) throw Object.assign(new Error('ENOENT: no such file'), { code: 'ENOENT' })
      return content
    }),
  }
})

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('aiProviderHandlers — relay credential store', () => {
  beforeEach(() => {
    mockFiles.clear()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  // ── ai.provider.writeCredential ────────────────────────────────────────────
  describe('ai.provider.writeCredential', () => {
    it('stores encrypted blob, iv, and updatedAt in JSON', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({
        accountId: 'acct-001',
        encryptedBlob: 'encrypted-data-here',
        iv: 'abc123iv',
      })
      const storedFile = [...mockFiles.values()][0]
      expect(storedFile).toBeDefined()
      const parsed = JSON.parse(storedFile)
      expect(parsed.encryptedBlob).toBe('encrypted-data-here')
      expect(parsed.iv).toBe('abc123iv')
      expect(parsed.updatedAt).toBeDefined()
    })

    it('returns { ok: true } on success', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const result = await aiProviderHandlers['ai.provider.writeCredential']({
        accountId: 'acct-002',
        encryptedBlob: 'blob',
        iv: 'iv',
      })
      expect(result).toEqual({ ok: true })
    })

    it('overwrites existing credential on repeated write (idempotent)', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'acct-003', encryptedBlob: 'old', iv: 'iv1' })
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'acct-003', encryptedBlob: 'new', iv: 'iv2' })

      // The last written value should be 'new'
      const allFiles = [...mockFiles.values()]
      const lastFile = allFiles[allFiles.length - 1]
      const parsed = JSON.parse(lastFile)
      expect(parsed.encryptedBlob).toBe('new')
    })

    it('calls mkdir with recursive: true', async () => {
      const fsMock = await import('node:fs/promises')
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'a', encryptedBlob: 'b', iv: 'c' })
      expect(fsMock.mkdir).toHaveBeenCalledWith(expect.any(String), { recursive: true })
    })

    it('stores updatedAt as ISO timestamp string', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const before = Date.now()
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'ts-test', encryptedBlob: 'b', iv: 'i' })
      const stored = JSON.parse([...mockFiles.values()][0])
      const updatedAt = new Date(stored.updatedAt).getTime()
      expect(updatedAt).toBeGreaterThanOrEqual(before)
    })
  })

  // ── ai.provider.readCredential ─────────────────────────────────────────────
  describe('ai.provider.readCredential', () => {
    it('reads and parses credential previously written', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({
        accountId: 'acct-read-001',
        encryptedBlob: 'secret-blob',
        iv: 'secret-iv',
      })
      const result = await aiProviderHandlers['ai.provider.readCredential']({ accountId: 'acct-read-001' })
      expect(result.encryptedBlob).toBe('secret-blob')
      expect(result.iv).toBe('secret-iv')
    })

    it('returns CredentialRecord with correct shape', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'shape-test', encryptedBlob: 'b', iv: 'i' })
      const result = await aiProviderHandlers['ai.provider.readCredential']({ accountId: 'shape-test' })
      expect(result).toHaveProperty('encryptedBlob')
      expect(result).toHaveProperty('iv')
      expect(result).toHaveProperty('updatedAt')
    })

    it('throws when credential file does not exist', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await expect(
        aiProviderHandlers['ai.provider.readCredential']({ accountId: 'nonexistent-acct' })
      ).rejects.toThrow('ENOENT')
    })
  })

  // ── ai.provider.testConnection ─────────────────────────────────────────────
  describe('ai.provider.testConnection', () => {
    it('returns { ok: true, latencyMs } when credential file exists', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'test-conn', encryptedBlob: 'x', iv: 'y' })
      const result = await aiProviderHandlers['ai.provider.testConnection']({ accountId: 'test-conn' })
      expect(result.ok).toBe(true)
      expect(typeof result.latencyMs).toBe('number')
      expect(result.error).toBeUndefined()
    })

    it('returns { ok: false, error } when credential file missing', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const result = await aiProviderHandlers['ai.provider.testConnection']({ accountId: 'no-cred-acct' })
      expect(result.ok).toBe(false)
      expect(result.error).toBeDefined()
      expect(typeof result.latencyMs).toBe('number')
    })

    it('always returns latencyMs as a non-negative number', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const result = await aiProviderHandlers['ai.provider.testConnection']({ accountId: 'no-file' })
      expect(typeof result.latencyMs).toBe('number')
      expect(result.latencyMs).toBeGreaterThanOrEqual(0)
    })
  })

  // ── ai.provider.healthCheck ────────────────────────────────────────────────
  describe('ai.provider.healthCheck', () => {
    it('delegates to testConnection — returns same ok status when cred exists', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'health-acct', encryptedBlob: 'x', iv: 'y' })
      const hc = await aiProviderHandlers['ai.provider.healthCheck']({ accountId: 'health-acct' })
      expect(hc.ok).toBe(true)
    })

    it('health check for missing credential returns ok: false', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const result = await aiProviderHandlers['ai.provider.healthCheck']({ accountId: 'no-credential' })
      expect(result.ok).toBe(false)
    })

    it('health check result has latencyMs field', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const result = await aiProviderHandlers['ai.provider.healthCheck']({ accountId: 'any-acct' })
      expect(typeof result.latencyMs).toBe('number')
    })
  })
})
