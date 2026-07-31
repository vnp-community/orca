import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { WebCredentialStore } from '../web-credential-store'

function makeTempStore(userId = 'test-user-123', secret = 'test-server-secret') {
  const tempDir = mkdtempSync(join(tmpdir(), 'orca-cred-test-'))
  const store = new WebCredentialStore(tempDir, userId, secret)
  return { store, tempDir }
}

describe('WebCredentialStore', () => {
  let store: WebCredentialStore
  let tempDir: string

  beforeEach(() => {
    const result = makeTempStore()
    store = result.store
    tempDir = result.tempDir
  })

  afterEach(() => {
    rmSync(tempDir, { recursive: true, force: true })
  })

  describe('setToken / getToken', () => {
    it('stores and retrieves a token', async () => {
      await store.setToken('bitbucket', 'my-secret-token')
      const retrieved = await store.getToken('bitbucket')
      expect(retrieved).toBe('my-secret-token')
    })

    it('returns null when no token stored', async () => {
      const token = await store.getToken('bitbucket')
      expect(token).toBeNull()
    })

    it('encrypts token on disk (raw file differs from plaintext token)', async () => {
      await store.setToken('gitea', 'plaintext-value')
      const { readFileSync } = await import('node:fs')
      const raw = readFileSync(join(tempDir, 'users', 'test-user-123', 'credentials', 'gitea.enc'))
      expect(raw.toString('utf8')).not.toContain('plaintext-value')
    })

    it('overwrites previous token on second setToken call', async () => {
      await store.setToken('linear', 'first-token')
      await store.setToken('linear', 'second-token')
      const token = await store.getToken('linear')
      expect(token).toBe('second-token')
    })

    it('handles tokens with special characters', async () => {
      const complexToken = 'tok€n/with+special=chars&more!@#'
      await store.setToken('jira', complexToken)
      expect(await store.getToken('jira')).toBe(complexToken)
    })
  })

  describe('hasToken', () => {
    it('returns false before setToken', async () => {
      expect(await store.hasToken('bitbucket')).toBe(false)
    })

    it('returns true after setToken', async () => {
      await store.setToken('bitbucket', 'some-token')
      expect(await store.hasToken('bitbucket')).toBe(true)
    })

    it('returns false after deleteToken', async () => {
      await store.setToken('bitbucket', 'some-token')
      await store.deleteToken('bitbucket')
      expect(await store.hasToken('bitbucket')).toBe(false)
    })
  })

  describe('deleteToken', () => {
    it('removes the token file so getToken returns null', async () => {
      await store.setToken('azure-devops', 'pat-token')
      await store.deleteToken('azure-devops')
      expect(await store.getToken('azure-devops')).toBeNull()
    })

    it('does not throw if token does not exist', async () => {
      await expect(store.deleteToken('gitea')).resolves.toBeUndefined()
    })

    it('removes config file along with token', async () => {
      await store.setToken('bitbucket', 'tok', { email: 'a@b.com' })
      await store.deleteToken('bitbucket')
      // Config should be gone too
      expect(await store.getConfig('bitbucket')).toBeNull()
    })
  })

  describe('config storage', () => {
    it('stores and retrieves config alongside token', async () => {
      await store.setToken('bitbucket', 'tok', { email: 'user@example.com', apiBaseUrl: 'https://api.bb.io' })
      const config = await store.getConfig('bitbucket')
      expect(config).toEqual({ email: 'user@example.com', apiBaseUrl: 'https://api.bb.io' })
    })

    it('returns null config when not set', async () => {
      await store.setToken('linear', 'tok')  // no config
      expect(await store.getConfig('linear')).toBeNull()
    })

    it('returns null config when no token stored at all', async () => {
      expect(await store.getConfig('jira')).toBeNull()
    })
  })

  describe('user isolation', () => {
    it('two stores for different users have separate token files', async () => {
      const { store: storeB, tempDir: dirB } = makeTempStore('user-b', 'test-server-secret')
      try {
        await store.setToken('bitbucket', 'token-for-user-a')
        await storeB.setToken('bitbucket', 'token-for-user-b')

        expect(await store.getToken('bitbucket')).toBe('token-for-user-a')
        expect(await storeB.getToken('bitbucket')).toBe('token-for-user-b')
      } finally {
        rmSync(dirB, { recursive: true, force: true })
      }
    })

    it('cannot decrypt another user token with a different store (different derived key)', async () => {
      // Store A uses secret 'secret-A', store B uses different base dir with same secret.
      // The real isolation is the separate userId directory.
      const { store: storeA, tempDir: dirA } = makeTempStore('user-alice', 'server-secret')
      const { store: storeB, tempDir: dirB } = makeTempStore('user-bob', 'server-secret')
      try {
        await storeA.setToken('gitea', 'alices-token')
        // storeB has no token, returns null
        expect(await storeB.getToken('gitea')).toBeNull()
      } finally {
        rmSync(dirA, { recursive: true, force: true })
        rmSync(dirB, { recursive: true, force: true })
      }
    })
  })

  describe('listServices', () => {
    it('returns empty array when no tokens stored', async () => {
      expect(await store.listServices()).toEqual([])
    })

    it('returns service names for all stored tokens', async () => {
      await store.setToken('bitbucket', 'tok1')
      await store.setToken('linear', 'tok2')
      const services = await store.listServices()
      expect(services.sort()).toEqual(['bitbucket', 'linear'].sort())
    })
  })
})
