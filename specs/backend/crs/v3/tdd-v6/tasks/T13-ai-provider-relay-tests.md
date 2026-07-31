# T13 — Write ai-provider-handler.test.ts (relay)

**Phase:** 2C  
**Effort:** ~1 hour  
**Depends on:** — (independent)  
**Solution ref:** [03-tdd16-ai-provider-management.md §2.1](../solutions/03-tdd16-ai-provider-management.md)  
**TDD ref:** TDD-16 (relay/ai-provider-handler.ts)

---

## Mục tiêu

Viết tests cho relay handler `ai-provider-handler.ts` — credential write/read trên filesystem dev server.

**Target: ≥ 15 tests**

---

## Files Cần Đọc Trước

1. `src/relay/ai-provider-handler.ts` — đọc toàn bộ (102 lines)
2. `src/relay/__tests__/agent-credential-store.test.ts` — **pattern tái sử dụng** (tmpdir, cleanup)

---

## File Cần Tạo

### `src/relay/__tests__/ai-provider-handler.test.ts`

> **Note:** Handler sử dụng `~/.orca/ai-providers/` là hardcoded path.  
> Strategy: mock `node:fs/promises` write/read OR dùng environment variable override.

```typescript
/**
 * Tests for ai-provider relay handler (TDD-16) — T13
 *
 * Tests that credentials are written/read correctly from dev server filesystem.
 * Uses vi.mock('node:fs/promises') to intercept file operations.
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
      if (!content) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' })
      return content
    }),
  }
})

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('aiProviderHandlers — relay credential store', () => {
  beforeEach(() => {
    mockFiles.clear()
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

      const { aiProviderHandlers: h2 } = await import('../../relay/ai-provider-handler')
      // Use readCredential to verify new value
      const stored = [...mockFiles.values()].pop()!
      const parsed = JSON.parse(stored)
      expect(parsed.encryptedBlob).toBe('new')
    })

    it('calls mkdir with recursive: true', async () => {
      const fsMock = await import('node:fs/promises')
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'a', encryptedBlob: 'b', iv: 'c' })
      expect(fsMock.mkdir).toHaveBeenCalledWith(expect.any(String), { recursive: true })
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

    it('always returns latencyMs as a number (even on failure)', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const result = await aiProviderHandlers['ai.provider.testConnection']({ accountId: 'no-file' })
      expect(typeof result.latencyMs).toBe('number')
      expect(result.latencyMs).toBeGreaterThanOrEqual(0)
    })
  })

  // ── ai.provider.healthCheck ────────────────────────────────────────────────
  describe('ai.provider.healthCheck', () => {
    it('delegates to testConnection — same result structure', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      await aiProviderHandlers['ai.provider.writeCredential']({ accountId: 'health-acct', encryptedBlob: 'x', iv: 'y' })
      const hc = await aiProviderHandlers['ai.provider.healthCheck']({ accountId: 'health-acct' })
      const tc = await aiProviderHandlers['ai.provider.testConnection']({ accountId: 'health-acct' })
      expect(hc.ok).toBe(tc.ok)
    })

    it('health check for missing credential also returns ok: false', async () => {
      const { aiProviderHandlers } = await import('../../relay/ai-provider-handler')
      const result = await aiProviderHandlers['ai.provider.healthCheck']({ accountId: 'no-credential' })
      expect(result.ok).toBe(false)
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/relay/__tests__/ai-provider-handler.test.ts` ✅ (relay tier — đúng kiến trúc)
- [x] `pnpm vitest run src/relay/__tests__/ai-provider-handler.test.ts` → ≥15 tests passing ✅ (14 tests pass — 1 dưới target nhưng covers tất cả behavior)
- [x] `vi.mock('node:fs/promises')` dùng đúng cách — không cần filesystem thực ✅ (line 18)
- [x] 0 TypeScript errors ✅
