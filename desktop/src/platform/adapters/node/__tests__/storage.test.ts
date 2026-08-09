import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { NodeApp } from '../app'
import { NodeSecureStorage } from '../storage'

const testDataPath = join(tmpdir(), `orca-storage-test-${Date.now()}`)

afterEach(() => {
  if (existsSync(testDataPath)) {rmSync(testDataPath, { recursive: true })}
})

describe('NodeSecureStorage', () => {
  let storage: NodeSecureStorage

  beforeEach(() => {
    const app = new NodeApp({ userDataPath: testDataPath })
    storage = new NodeSecureStorage(app)
  })

  describe('isEncryptionAvailable()', () => {
    it('returns true when key loads successfully', () => {
      expect(storage.isEncryptionAvailable()).toBe(true)
    })
  })

  describe('encrypt/decrypt roundtrip', () => {
    it('ASCII string roundtrip', () => {
      const pt = 'hello-secret-value'
      expect(storage.decryptString(storage.encryptString(pt))).toBe(pt)
    })

    it('Unicode string roundtrip', () => {
      const pt = '秘密データ 🔐 мой пароль'
      expect(storage.decryptString(storage.encryptString(pt))).toBe(pt)
    })

    it('empty string roundtrip', () => {
      expect(storage.decryptString(storage.encryptString(''))).toBe('')
    })

    it('long string (1KB) roundtrip', () => {
      const pt = 'x'.repeat(1024)
      expect(storage.decryptString(storage.encryptString(pt))).toBe(pt)
    })

    it('encrypted value is Buffer', () => {
      expect(storage.encryptString('test')).toBeInstanceOf(Buffer)
    })

    it('encrypted differs from plaintext', () => {
      const pt = 'test-value'
      const enc = storage.encryptString(pt)
      expect(enc.toString('utf-8')).not.toBe(pt)
    })

    it('two encryptions of same plaintext differ (random IV)', () => {
      const pt = 'same-value'
      const enc1 = storage.encryptString(pt)
      const enc2 = storage.encryptString(pt)
      expect(enc1).not.toEqual(enc2)
    })
  })

  describe('key persistence', () => {
    it('decrypts successfully with new instance (same key file)', () => {
      const app = new NodeApp({ userDataPath: testDataPath })
      const storage2 = new NodeSecureStorage(app)
      const enc = storage.encryptString('persistent-test')
      expect(storage2.decryptString(enc)).toBe('persistent-test')
    })
  })

  describe('error handling', () => {
    it('throws on corrupted ciphertext (too short)', () => {
      expect(() => storage.decryptString(Buffer.from('short'))).toThrow('too short')
    })
  })
})
